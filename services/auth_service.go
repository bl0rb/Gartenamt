package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings" // NEU: für strings.Repeat
	"sync"
	"time"

	"kleingarten-verwaltung/models"
)

// Session repräsentiert eine Benutzersitzung
type Session struct {
	ID        string
	UserID    int
	Username  string
	Role      string
	CreatedAt time.Time
	LastSeen  time.Time
	IPAddress string
	UserAgent string
}

// AuthService verwaltet Authentifizierung und Sessions
type AuthService struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
}

// NewAuthService erstellt einen neuen Auth-Service
func NewAuthService() *AuthService {
	service := &AuthService{
		sessions: make(map[string]*Session),
	}

	// Cleanup-Routine für abgelaufene Sessions
	go service.cleanupExpiredSessions()

	return service
}

// Login authentifiziert einen Benutzer und erstellt eine Session
func (as *AuthService) Login(username, password, ipAddress, userAgent string) (*Session, error) {
	// Benutzer aus Datenbank laden
	user, err := models.GetUserByUsername(username)
	if err != nil {
		log.Printf("Login-Versuch fehlgeschlagen für Benutzer: %s - %v", username, err)
		return nil, errors.New("ungültige Anmeldedaten")
	}

	// Passwort validieren
	if !user.ValidatePassword(password) {
		log.Printf("Falsches Passwort für Benutzer: %s", username)
		return nil, errors.New("ungültige Anmeldedaten")
	}

	// Bestehende Sessions für diesen Benutzer löschen
	as.invalidateUserSessions(user.ID)

	// Neue Session erstellen
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		Username:  user.Username,
		Role:      user.Role,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
		IPAddress: ipAddress,
		UserAgent: userAgent,
	}

	// Session speichern
	as.mutex.Lock()
	as.sessions[sessionID] = session
	as.mutex.Unlock()

	// Last-Login in Datenbank aktualisieren
	user.UpdateLastLogin()

	log.Printf("Erfolgreicher Login: %s (Session: %s)", user.Username, sessionID[:8])
	return session, nil
}

// ValidateSession überprüft eine Session und verlängert sie
func (as *AuthService) ValidateSession(sessionID string) (*Session, error) {
	if sessionID == "" {
		return nil, errors.New("keine Session-ID")
	}

	as.mutex.RLock()
	session, exists := as.sessions[sessionID]
	as.mutex.RUnlock()

	if !exists {
		return nil, errors.New("session nicht gefunden")
	}

	// Session-Timeout prüfen (24 Stunden)
	if time.Since(session.LastSeen) > 24*time.Hour {
		as.InvalidateSession(sessionID)
		return nil, errors.New("session abgelaufen")
	}

	// LastSeen aktualisieren
	as.mutex.Lock()
	session.LastSeen = time.Now()
	as.mutex.Unlock()

	return session, nil
}

// InvalidateSession beendet eine Session
func (as *AuthService) InvalidateSession(sessionID string) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	if session, exists := as.sessions[sessionID]; exists {
		log.Printf("Session beendet: %s (Benutzer: %s)", sessionID[:8], session.Username)
		delete(as.sessions, sessionID)
	}
}

// Logout beendet die Benutzersitzung
func (as *AuthService) Logout(sessionID string) {
	as.InvalidateSession(sessionID)
}

// GetSessionInfo gibt Session-Informationen zurück
func (as *AuthService) GetSessionInfo(sessionID string) *Session {
	as.mutex.RLock()
	defer as.mutex.RUnlock()

	if session, exists := as.sessions[sessionID]; exists {
		return session
	}
	return nil
}

// GetActiveSessions gibt alle aktiven Sessions zurück (für Admin)
func (as *AuthService) GetActiveSessions() []*Session {
	as.mutex.RLock()
	defer as.mutex.RUnlock()

	var sessions []*Session
	for _, session := range as.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

// Private Hilfsfunktionen

func (as *AuthService) invalidateUserSessions(userID int) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	for sessionID, session := range as.sessions {
		if session.UserID == userID {
			delete(as.sessions, sessionID)
		}
	}
}

func (as *AuthService) cleanupExpiredSessions() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		as.mutex.Lock()
		now := time.Now()
		expired := make([]string, 0)

		for sessionID, session := range as.sessions {
			if now.Sub(session.LastSeen) > 24*time.Hour {
				expired = append(expired, sessionID)
			}
		}

		for _, sessionID := range expired {
			delete(as.sessions, sessionID)
		}

		if len(expired) > 0 {
			log.Printf("Abgelaufene Sessions bereinigt: %d", len(expired))
		}
		as.mutex.Unlock()
	}
}

func generateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// Globale Auth-Service Instanz
var GlobalAuth *AuthService

// InitAuth initialisiert den Auth-Service
func InitAuth() {
	GlobalAuth = NewAuthService()
	log.Println("🔐 Auth-Service initialisiert")
}

// CreateDefaultAdmin erstellt einen Standard-Administrator falls noch keiner existiert
func CreateDefaultAdmin() error {
	if models.CountUsers() > 0 {
		return nil // Bereits Benutzer vorhanden
	}

	// Standard-Admin erstellen
	defaultUsername := "admin"
	defaultPassword := "admin123" // MUSS nach erstem Login geändert werden!

	user, err := models.CreateUser(defaultUsername, "admin@kleingarten.local", defaultPassword, models.RoleAdmin)
	if err != nil {
		return err
	}

	// Warnung ausgeben - KORRIGIERT mit strings.Repeat
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🔧 STANDARD-ADMINISTRATOR ERSTELLT")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println("Da noch keine Benutzer vorhanden waren, wurde ein Standard-Administrator erstellt:")
	fmt.Println()
	fmt.Printf("Benutzername: %s\n", defaultUsername)
	fmt.Printf("Passwort:     %s\n", defaultPassword)
	fmt.Println()
	fmt.Println("⚠️  WICHTIGE SICHERHEITSHINWEISE:")
	fmt.Println("- Melden Sie sich SOFORT mit diesen Daten an")
	fmt.Println("- Ändern Sie das Passwort UNVERZÜGLICH")
	fmt.Println("- Erstellen Sie weitere Benutzer nach Bedarf")
	fmt.Println("- Löschen Sie diesen Hinweis aus der Konsole")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()

	log.Printf("Standard-Administrator erstellt: ID=%d, Username=%s", user.ID, user.Username)
	return nil
}

// Add this function to your existing services/auth_service.go

// InvalidateUserSessions invalidates all sessions for a specific user
func (as *AuthService) InvalidateUserSessions(userID int) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	var sessionsToDelete []string
	for sessionID, session := range as.sessions {
		if session.UserID == userID {
			sessionsToDelete = append(sessionsToDelete, sessionID)
		}
	}

	for _, sessionID := range sessionsToDelete {
		log.Printf("Session invalidiert: %s (Benutzer-ID: %d)", sessionID[:8], userID)
		delete(as.sessions, sessionID)
	}

	if len(sessionsToDelete) > 0 {
		log.Printf("Alle Sessions für Benutzer-ID %d invalidiert (%d Sessions)", userID, len(sessionsToDelete))
	}
}
