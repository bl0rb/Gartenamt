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

// Schwellwerte für die Login-Drosselung und die Session-Laufzeit
const (
	// maxFailedLoginsPerAccount schützt ein einzelnes Konto vor gezieltem
	// Raten. maxFailedLoginsPerIP liegt bewusst deutlich höher: hinter NAT
	// oder einem Reverse-Proxy teilen sich alle Mitglieder eine Adresse -
	// eine gleich strenge IP-Sperre würde dort nach fünf Tippfehlern den
	// gesamten Verein aussperren.
	maxFailedLoginsPerAccount = 5
	maxFailedLoginsPerIP      = 50
	loginLockoutTime          = 15 * time.Minute

	// maxThrottleEntries deckelt die Fehlversuchs-Tabelle. Ihre Schlüssel
	// enthalten den frei wählbaren Benutzernamen - ohne Obergrenze könnte ein
	// Skript darüber den Speicher füllen.
	maxThrottleEntries = 5000

	// sessionIdleTimeout beendet Sitzungen ohne Aktivität, sessionMaxLifetime
	// begrenzt sie unabhängig von der Aktivität. Ohne die zweite Grenze lässt
	// sich eine Sitzung durch regelmäßige Aufrufe unbegrenzt verlängern.
	sessionIdleTimeout = 24 * time.Hour
	sessionMaxLifetime = 7 * 24 * time.Hour
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
	// Zwei getrennte Zähler: einer pro Konto, einer pro Absender-IP. Eine
	// Sperre allein pro Kombination ließe sich umgehen, indem der Angreifer
	// die Quelladresse wechselt - im LAN ist das trivial.
	keys := throttleKeys(username, ipAddress)
	if err := as.checkLoginThrottle(keys); err != nil {
		return nil, err
	}

	// Benutzer aus Datenbank laden
	user, err := models.GetUserByUsername(username)
	if err != nil {
		// Ohne diesen Vergleich antwortet die Anmeldung bei unbekanntem
		// Benutzer sofort, bei bekanntem erst nach dem bcrypt-Durchlauf -
		// die Antwortzeit verrät damit, welche Konten existieren.
		models.ConsumeDummyPasswordTime(password)
		as.recordFailedLogin(keys)
		log.Printf("Login-Versuch fehlgeschlagen für Benutzer: %s - %v", username, err)
		return nil, errors.New("ungültige Anmeldedaten")
	}

	// Passwort validieren
	if !user.ValidatePassword(password) {
		as.recordFailedLogin(keys)
		log.Printf("Falsches Passwort für Benutzer: %s", username)
		return nil, errors.New("ungültige Anmeldedaten")
	}

	as.resetFailedLogins(keys)

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

	// Inaktivität und absolute Laufzeit prüfen
	if time.Since(session.LastSeen) > sessionIdleTimeout {
		as.InvalidateSession(sessionID)
		return nil, errors.New("session abgelaufen")
	}

	if time.Since(session.CreatedAt) > sessionMaxLifetime {
		as.InvalidateSession(sessionID)
		return nil, errors.New("session hat die maximale Laufzeit erreicht")
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

// throttleKey ist ein Zähler-Schlüssel mit eigener Schwelle.
type throttleKey struct {
	key   string
	limit int
}

// throttleKeys liefert die Zähler eines Anmeldeversuchs: einen für das Konto
// und einen für die Absender-IP, jeweils mit eigener Schwelle.
func throttleKeys(username, ipAddress string) []throttleKey {
	return []throttleKey{
		{"user:" + strings.ToLower(strings.TrimSpace(username)), maxFailedLoginsPerAccount},
		{"ip:" + ipAddress, maxFailedLoginsPerIP},
	}
}

// checkLoginThrottle blockiert Logins, solange eine der Sperren aktiv ist.
func (as *AuthService) checkLoginThrottle(keys []throttleKey) error {
	as.mutex.RLock()
	defer as.mutex.RUnlock()

	now := time.Now()
	for _, k := range keys {
		if state, exists := as.failedLogins[k.key]; exists && now.Before(state.LockedUntil) {
			return ErrLoginThrottled
		}
	}
	return nil
}

func (as *AuthService) recordFailedLogin(keys []throttleKey) {
	as.mutex.Lock()
	defer as.mutex.Unlock()

	now := time.Now()
	for _, k := range keys {
		state, exists := as.failedLogins[k.key]
		// Zähler nur neu beginnen, wenn der letzte Fehlversuch länger als das
		// Sperrfenster zurückliegt (und keine aktive Sperre mehr besteht).
		if !exists || (now.Sub(state.LastAttempt) > loginLockoutTime && now.After(state.LockedUntil)) {
			state = &failedLoginState{}
			as.failedLogins[k.key] = state
		}

		state.Count++
		state.LastAttempt = now
		if state.Count >= k.limit && now.After(state.LockedUntil) {
			state.LockedUntil = now.Add(loginLockoutTime)
			log.Printf("Login gesperrt für %s (%d Fehlversuche, Sperre bis %s)",
				k.key, state.Count, state.LockedUntil.Format("15:04:05"))
		}
	}

	as.pruneFailedLoginsLocked(now)
}

// pruneFailedLoginsLocked hält die Tabelle klein: zuerst fallen abgelaufene
// Einträge weg, danach - falls immer noch zu viele - die ältesten.
// Der Aufrufer hält bereits den Schreib-Lock.
func (as *AuthService) pruneFailedLoginsLocked(now time.Time) {
	if len(as.failedLogins) <= maxThrottleEntries {
		return
	}

	for key, state := range as.failedLogins {
		if now.Sub(state.LastAttempt) > loginLockoutTime && now.After(state.LockedUntil) {
			delete(as.failedLogins, key)
		}
	}

	for len(as.failedLogins) > maxThrottleEntries {
		oldestKey := ""
		var oldest time.Time
		for key, state := range as.failedLogins {
			if oldestKey == "" || state.LastAttempt.Before(oldest) {
				oldestKey, oldest = key, state.LastAttempt
			}
		}
		if oldestKey == "" {
			return
		}
		delete(as.failedLogins, oldestKey)
	}
}

func (as *AuthService) resetFailedLogins(keys []throttleKey) {
	as.mutex.Lock()
	defer as.mutex.Unlock()
	for _, k := range keys {
		delete(as.failedLogins, k.key)
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
			if now.Sub(session.LastSeen) > sessionIdleTimeout || now.Sub(session.CreatedAt) > sessionMaxLifetime {
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
		if err != nil {
			return nil
		}
		// Regenerieren, wenn die Passwortänderung noch aussteht - oder das
		// Konto noch nie benutzt wurde (Upgrade von Versionen, die das
		// Initialpasswort nur auf der Konsole ausgaben: es ist dann faktisch
		// verloren, z.B. beim Start als macOS-App ohne sichtbare Konsole).
		if !admin.MustChangePassword && admin.LastLogin != nil {
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
