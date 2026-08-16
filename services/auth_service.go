package services

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/bl0rb/gartenamt/models"
)

// Schwellwerte für die Login-Drosselung
const (
	maxFailedLogins  = 5
	loginLockoutTime = 15 * time.Minute
)

type failedLoginState struct {
	Count       int
	LastAttempt time.Time
	LockedUntil time.Time
}

// ErrLoginThrottled signalisiert eine aktive Login-Sperre nach zu vielen Fehlversuchen.
var ErrLoginThrottled = errors.New("zu viele fehlgeschlagene Anmeldeversuche - bitte später erneut versuchen")

// Session repräsentiert eine Benutzersitzung
type Session struct {
	ID                 string
	UserID             int
	Username           string
	Role               string
	CreatedAt          time.Time
	LastSeen           time.Time
	IPAddress          string
	UserAgent          string
	MustChangePassword bool
}

// AuthService verwaltet Authentifizierung und Sessions
type AuthService struct {
	sessions     map[string]*Session
	failedLogins map[string]*failedLoginState
	mutex        sync.RWMutex
}

// NewAuthService erstellt einen neuen Auth-Service
func NewAuthService() *AuthService {
	service := &AuthService{
		sessions:     make(map[string]*Session),
		failedLogins: make(map[string]*failedLoginState),
	}

	// Cleanup-Routine für abgelaufene Sessions
	go service.cleanupExpiredSessions()

	return service
}

// Login authentifiziert einen Benutzer und erstellt eine Session
func (as *AuthService) Login(username, password, ipAddress, userAgent string) (*Session, error) {
	throttleKey := strings.ToLower(strings.TrimSpace(username)) + "|" + ipAddress
	if err := as.checkLoginThrottle(throttleKey); err != nil {
		return nil, err
	}

	// Benutzer aus Datenbank laden
	user, err := models.GetUserByUsername(username)
	if err != nil {
		as.recordFailedLogin(throttleKey)
		log.Printf("Login-Versuch fehlgeschlagen für Benutzer: %s - %v", username, err)
		return nil, errors.New("ungültige Anmeldedaten")
	}

	// Passwort validieren
	if !user.ValidatePassword(password) {
		as.recordFailedLogin(throttleKey)
		log.Printf("Falsches Passwort für Benutzer: %s", username)
		return nil, errors.New("ungültige Anmeldedaten")
	}

	as.resetFailedLogins(throttleKey)

	// Bestehende Sessions für diesen Benutzer löschen
	as.InvalidateUserSessions(user.ID)

	// Neue Session erstellen
	sessionID, err := generateSessionID()
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:                 sessionID,
		UserID:             user.ID,
		Username:           user.Username,
		Role:               user.Role,
		CreatedAt:          time.Now(),
		LastSeen:           time.Now(),
		IPAddress:          ipAddress,
		UserAgent:          userAgent,
		MustChangePassword: user.MustChangePassword,
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

// ClearMustChangePassword hebt die erzwungene Passwortänderung für eine
// laufende Session auf (nach erfolgreicher Passwortänderung).
func (as *AuthService) ClearMustChangePassword(sessionID string) {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	if session, exists := as.sessions[sessionID]; exists {
		session.MustChangePassword = false
	}
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

// InvalidateUserSessions beendet alle Sessions eines Benutzers.
func (as *AuthService) InvalidateUserSessions(userID int) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	for sessionID, session := range as.sessions {
		if session.UserID == userID {
			log.Printf("Session invalidiert: %s (Benutzer-ID: %d)", sessionID[:8], userID)
			delete(as.sessions, sessionID)
		}
	}
}

// InvalidateOtherUserSessions beendet alle Sessions eines Benutzers außer der
// angegebenen (z.B. nach einer Passwortänderung die aktuelle Sitzung behalten).
func (as *AuthService) InvalidateOtherUserSessions(userID int, keepSessionID string) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	for sessionID, session := range as.sessions {
		if session.UserID == userID && sessionID != keepSessionID {
			log.Printf("Session invalidiert: %s (Benutzer-ID: %d)", sessionID[:8], userID)
			delete(as.sessions, sessionID)
		}
	}
}

// Private Hilfsfunktionen

// checkLoginThrottle blockiert Logins, solange die Sperre nach zu vielen
// Fehlversuchen aktiv ist.
func (as *AuthService) checkLoginThrottle(key string) error {
	as.mutex.RLock()
	var lockedUntil time.Time
	if state, exists := as.failedLogins[key]; exists {
		lockedUntil = state.LockedUntil
	}
	as.mutex.RUnlock()

	if time.Now().Before(lockedUntil) {
		return ErrLoginThrottled
	}
	return nil
}

func (as *AuthService) recordFailedLogin(key string) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	now := time.Now()
	state, exists := as.failedLogins[key]
	// Zähler nur neu beginnen, wenn der letzte Fehlversuch länger als das
	// Sperrfenster zurückliegt (und keine aktive Sperre mehr besteht).
	if !exists || (now.Sub(state.LastAttempt) > loginLockoutTime && now.After(state.LockedUntil)) {
		state = &failedLoginState{}
		as.failedLogins[key] = state
	}

	state.Count++
	state.LastAttempt = now
	if state.Count >= maxFailedLogins && now.After(state.LockedUntil) {
		state.LockedUntil = now.Add(loginLockoutTime)
		log.Printf("Login gesperrt für %s (%d Fehlversuche, Sperre bis %s)",
			key, state.Count, state.LockedUntil.Format("15:04:05"))
	}
}

func (as *AuthService) resetFailedLogins(key string) {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	delete(as.failedLogins, key)
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

		// Veraltete Fehlversuchs-Einträge aufräumen (letzter Versuch älter als
		// das Sperrfenster und keine aktive Sperre mehr)
		for key, state := range as.failedLogins {
			if now.Sub(state.LastAttempt) > loginLockoutTime && now.After(state.LockedUntil) {
				delete(as.failedLogins, key)
			}
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

// Initial-Zugangsdaten des automatisch erzeugten Standard-Admins. Solange das
// Initialpasswort noch nicht geändert wurde, werden sie auf der Login-Seite
// angezeigt (die Konsolenausgabe ist z.B. beim Start als macOS-App unsichtbar).
var (
	initialCredsMutex    sync.RWMutex
	initialAdminUsername string
	initialAdminPassword string
)

// InitialAdminCredentials liefert die Initial-Zugangsdaten für die Anzeige auf
// der Login-Seite, solange das Initialpasswort noch nicht geändert wurde.
func InitialAdminCredentials() (username, password string, ok bool) {
	initialCredsMutex.RLock()
	defer initialCredsMutex.RUnlock()
	return initialAdminUsername, initialAdminPassword, initialAdminPassword != ""
}

// ClearInitialAdminCredentials entfernt die Initial-Zugangsdaten aus der Anzeige
// (aufzurufen, sobald das Initialpasswort geändert wurde).
func ClearInitialAdminCredentials() {
	initialCredsMutex.Lock()
	defer initialCredsMutex.Unlock()
	initialAdminUsername = ""
	initialAdminPassword = ""
}

func setInitialAdminCredentials(username, password string) {
	initialCredsMutex.Lock()
	defer initialCredsMutex.Unlock()
	initialAdminUsername = username
	initialAdminPassword = password
}

// CreateDefaultAdmin erstellt einen Standard-Administrator falls noch keiner
// existiert. Das Passwort wird zufällig generiert, auf der Konsole ausgegeben
// und bis zur ersten Passwortänderung auf der Login-Seite angezeigt. Wurde die
// App neu gestartet, bevor das Initialpasswort geändert wurde, wird es neu
// generiert, damit die Anzeige auf der Login-Seite gültig bleibt.
func CreateDefaultAdmin() error {
	const defaultUsername = "admin"

	if count := models.CountUsers(); count > 0 {
		// Initialpasswort noch nie geändert? Dann neu generieren, damit es
		// auf der Login-Seite angezeigt werden kann. Nur solange der
		// automatisch erzeugte Admin der einzige Benutzer ist - ein per
		// Admin-Reset gesetztes Flag darf keine Anzeige auslösen.
		if count > 1 {
			return nil
		}
		admin, err := models.GetUserByUsername(defaultUsername)
		if err != nil || !admin.MustChangePassword {
			return nil
		}

		password, err := generateInitialPassword()
		if err != nil {
			return fmt.Errorf("konnte kein Initial-Passwort generieren: %w", err)
		}
		if err := models.SetInitialPassword(admin.ID, password); err != nil {
			return fmt.Errorf("initial-Passwort konnte nicht gesetzt werden: %w", err)
		}
		setInitialAdminCredentials(defaultUsername, password)
		printInitialCredentials(defaultUsername, password, "Das Initialpasswort wurde noch nicht geändert und daher neu generiert:")
		return nil
	}

	defaultPassword, err := generateInitialPassword()
	if err != nil {
		return fmt.Errorf("konnte kein Initial-Passwort generieren: %w", err)
	}

	user, err := models.CreateUser(defaultUsername, "admin@kleingarten.local", defaultPassword, models.RoleAdmin)
	if err != nil {
		return err
	}
	if err := models.SetMustChangePassword(user.ID, true); err != nil {
		return err
	}
	setInitialAdminCredentials(defaultUsername, defaultPassword)
	printInitialCredentials(defaultUsername, defaultPassword, "Da noch keine Benutzer vorhanden waren, wurde ein Standard-Administrator erstellt:")

	log.Printf("Standard-Administrator erstellt: ID=%d, Username=%s", user.ID, user.Username)
	return nil
}

func printInitialCredentials(username, password, reason string) {
	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("🔧 STANDARD-ADMINISTRATOR")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println(reason)
	fmt.Println()
	fmt.Printf("Benutzername: %s\n", username)
	fmt.Printf("Passwort:     %s\n", password)
	fmt.Println()
	fmt.Println("⚠️  WICHTIGE SICHERHEITSHINWEISE:")
	fmt.Println("- Die Zugangsdaten werden bis zur ersten Passwortänderung auch auf der Login-Seite angezeigt")
	fmt.Println("- Nach dem ersten Login muss das Passwort geändert werden")
	fmt.Println("- Erstellen Sie weitere Benutzer nach Bedarf")
	fmt.Println(strings.Repeat("=", 80))
	fmt.Println()
}

// generateInitialPassword erzeugt ein zufälliges, URL-sicheres Passwort.
func generateInitialPassword() (string, error) {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
