package models

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID                 int        `json:"id"`
	Username           string     `json:"username"`
	Email              string     `json:"email"`
	Password           string     `json:"-"` // Wird nicht in JSON serialisiert
	Role               string     `json:"role"`
	Active             bool       `json:"active"`
	CreatedAt          time.Time  `json:"created_at"`
	LastLogin          *time.Time `json:"last_login"`
	MustChangePassword bool       `json:"-"` // Erzwungene Passwortänderung beim ersten Login
}

// UserRole definiert Benutzerrollen
type UserRole string

const (
	RoleAdmin         UserRole = "admin"
	RoleVorstand      UserRole = "vorstand"
	RoleKassenwart    UserRole = "kassenwart"
	RoleWertermittler UserRole = "wertermittler"
	RoleUser          UserRole = "user"
)

// passwordHashCost liegt bewusst ueber bcrypt.DefaultCost (10). Bestehende
// Hashes mit niedrigerem Kostenfaktor bleiben gueltig und werden bei der
// naechsten Passwortaenderung angehoben.
const passwordHashCost = 12

type PermissionDefinition struct {
	Key      string
	Label    string
	Category string
}

func NormalizeRole(role string) string {
	switch role {
	case string(RoleAdmin), string(RoleVorstand), string(RoleKassenwart), string(RoleWertermittler), string(RoleUser):
		return role
	default:
		return string(RoleUser)
	}
}

func RoleDisplayName(role string) string {
	switch role {
	case string(RoleAdmin):
		return "Admin"
	case string(RoleVorstand):
		return "Vorstand"
	case string(RoleKassenwart):
		return "Kassenwart"
	case string(RoleWertermittler):
		return "Wertermittler"
	default:
		return "Benutzer"
	}
}

func RoleBadgeClass(role string) string {
	switch role {
	case string(RoleAdmin):
		return "bg-danger"
	case string(RoleVorstand):
		return "bg-dark"
	case string(RoleKassenwart):
		return "bg-warning text-dark"
	case string(RoleWertermittler):
		return "bg-info text-dark"
	default:
		return "bg-primary"
	}
}

func IsBackofficeRole(role string) bool {
	switch role {
	case string(RoleAdmin), string(RoleVorstand), string(RoleKassenwart), string(RoleWertermittler):
		return true
	default:
		return false
	}
}

func GetBackofficeRoles() []string {
	return []string{string(RoleWertermittler), string(RoleKassenwart), string(RoleVorstand), string(RoleAdmin)}
}

func GetAllRoles() []string {
	return []string{string(RoleUser), string(RoleWertermittler), string(RoleKassenwart), string(RoleVorstand), string(RoleAdmin)}
}

func PermissionCatalog() []PermissionDefinition {
	return []PermissionDefinition{
		{Key: "dashboard.access", Label: "Dashboard", Category: "Allgemein"},
		{Key: "stammdaten.manage", Label: "Stammdaten verwalten", Category: "Verwaltung"},
		{Key: "parzellen.manage", Label: "Parzellen & zugehoerige Daten", Category: "Verwaltung"},
		{Key: "protokolle.manage", Label: "Protokolle verwalten", Category: "Verwaltung"},
		{Key: "invoices.manage", Label: "Rechnungen & E-Mail", Category: "Finanzen"},
		{Key: "backup.manage", Label: "Backup & CSV", Category: "Sicherheit"},
		{Key: "audit.view", Label: "Audit-Log", Category: "Sicherheit"},
		{Key: "users.manage", Label: "Benutzer & Gruppen", Category: "Sicherheit"},
		{Key: "settings.manage", Label: "Vereins-/Maileinstellungen", Category: "Einstellungen"},
		{Key: "system.settings", Label: "Systemeinstellungen", Category: "Einstellungen"},
	}
}

// RoleRank bildet die Rollen auf eine Rangfolge ab. Sie entscheidet, wer wen
// verwalten darf: ein Konto kann nur Rollen vergeben und nur Benutzer
// bearbeiten, deren Rang den eigenen nicht uebersteigt. Damit kann sich
// niemand ueber die Benutzerverwaltung selbst hoeherstufen.
func RoleRank(role string) int {
	switch NormalizeRole(role) {
	case string(RoleAdmin):
		return 3
	case string(RoleVorstand):
		return 2
	case string(RoleKassenwart), string(RoleWertermittler):
		return 1
	default:
		return 0
	}
}

// CanManageRole prueft, ob eine Rolle Konten der Zielrolle anlegen, bearbeiten
// oder auf diese Rolle setzen darf.
func CanManageRole(actorRole, targetRole string) bool {
	return RoleHasPermission(actorRole, "users.manage") &&
		RoleRank(actorRole) >= RoleRank(targetRole)
}

// DefaultRolePermissions liefert die Berechtigungen einer Rolle. Die
// sicherheitsrelevanten Bereiche (Benutzerverwaltung, Backup, Audit-Log,
// Einstellungen) bleiben Admin und Vorstand vorbehalten - sonst koennte sich
// jedes Backoffice-Konto ueber die Benutzerverwaltung zum Admin machen.
func DefaultRolePermissions(role string) map[string]bool {
	permissions := map[string]bool{}

	for _, permission := range PermissionCatalog() {
		permissions[permission.Key] = false
	}

	grant := func(keys ...string) {
		for _, key := range keys {
			permissions[key] = true
		}
	}

	switch NormalizeRole(role) {
	case string(RoleAdmin), string(RoleVorstand):
		for _, permission := range PermissionCatalog() {
			permissions[permission.Key] = true
		}
	case string(RoleKassenwart):
		grant("dashboard.access", "stammdaten.manage", "parzellen.manage", "invoices.manage")
	case string(RoleWertermittler):
		grant("dashboard.access", "stammdaten.manage", "parzellen.manage", "protokolle.manage")
	}

	return permissions
}

func RoleHasPermission(role, permission string) bool {
	permissions := DefaultRolePermissions(role)
	return permissions[permission]
}

// CreateUser erstellt einen neuen Benutzer mit verschlüsseltem Passwort
func CreateUser(username, email, password string, role UserRole) (*User, error) {
	role = UserRole(NormalizeRole(string(role)))

	// Passwort hashen
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), passwordHashCost)
	if err != nil {
		return nil, err
	}

	user := &User{
		Username:  username,
		Email:     email,
		Password:  string(hashedPassword),
		Role:      string(role),
		Active:    true,
		CreatedAt: time.Now(),
	}

	// In Datenbank speichern
	query := `INSERT INTO users (username, email, password_hash, role, active, created_at) 
              VALUES (?, ?, ?, ?, ?, ?)`
	result, err := DB.Exec(query, user.Username, user.Email, user.Password, user.Role, user.Active, user.CreatedAt)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	user.ID = int(id)

	return user, nil
}

// GetUserByUsername findet einen Benutzer über den Benutzernamen
func GetUserByUsername(username string) (*User, error) {
	user := &User{}
	query := `SELECT id, username, email, password_hash, role, active, created_at, last_login, must_change_password
              FROM users WHERE username = ? AND active = 1`

	var lastLogin sql.NullTime
	err := DB.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.Password,
		&user.Role, &user.Active, &user.CreatedAt, &lastLogin, &user.MustChangePassword)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("benutzer nicht gefunden")
		}
		return nil, err
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// GetUserByID findet einen Benutzer über die ID
func GetUserByID(id int) (*User, error) {
	user := &User{}
	query := `SELECT id, username, email, password_hash, role, active, created_at, last_login, must_change_password
              FROM users WHERE id = ?`

	var lastLogin sql.NullTime
	err := DB.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.Password,
		&user.Role, &user.Active, &user.CreatedAt, &lastLogin, &user.MustChangePassword)

	if err != nil {
		return nil, err
	}

	if lastLogin.Valid {
		user.LastLogin = &lastLogin.Time
	}

	return user, nil
}

// ValidatePassword überprüft das Passwort
func (u *User) ValidatePassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	return err == nil
}

// UpdateLastLogin aktualisiert die letzte Login-Zeit
func (u *User) UpdateLastLogin() error {
	now := time.Now()
	query := `UPDATE users SET last_login = ? WHERE id = ?`
	_, err := DB.Exec(query, now, u.ID)
	if err == nil {
		u.LastLogin = &now
	}
	return err
}

// IsAdmin prüft ob der Benutzer Administrator ist
func (u *User) IsAdmin() bool {
	return u.Role == string(RoleAdmin)
}

// GetAllUsers gibt alle Benutzer zurück (für Admin)
func GetAllUsers() ([]User, error) {
	query := `SELECT id, username, email, role, active, created_at, last_login, must_change_password
              FROM users ORDER BY created_at DESC`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		var lastLogin sql.NullTime

		err := rows.Scan(&user.ID, &user.Username, &user.Email,
			&user.Role, &user.Active, &user.CreatedAt, &lastLogin, &user.MustChangePassword)
		if err != nil {
			continue
		}

		if lastLogin.Valid {
			user.LastLogin = &lastLogin.Time
		}

		users = append(users, user)
	}

	return users, nil
}

// UserExists prüft ob ein Benutzer bereits existiert
func UserExists(username string) bool {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE username = ?`
	DB.QueryRow(query, username).Scan(&count)
	return count > 0
}

// CountUsers zählt alle aktiven Benutzer
func CountUsers() int {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE active = 1`
	DB.QueryRow(query).Scan(&count)
	return count
}

// ChangePassword ändert das Passwort eines Benutzers und hebt eine
// ggf. gesetzte erzwungene Passwortänderung auf.
func (u *User) ChangePassword(newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), passwordHashCost)
	if err != nil {
		return err
	}

	query := `UPDATE users SET password_hash = ?, must_change_password = 0 WHERE id = ?`
	_, err = DB.Exec(query, string(hashedPassword), u.ID)
	if err != nil {
		return err
	}

	u.Password = string(hashedPassword)
	u.MustChangePassword = false
	return nil
}

// SetMustChangePassword setzt oder löscht das Flag für die erzwungene Passwortänderung.
func SetMustChangePassword(userID int, value bool) error {
	_, err := DB.Exec(`UPDATE users SET must_change_password = ? WHERE id = ?`, value, userID)
	return err
}

// SetInitialPassword setzt ein neues Initialpasswort und markiert den Benutzer
// für die erzwungene Passwortänderung beim nächsten Login.
func SetInitialPassword(userID int, password string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), passwordHashCost)
	if err != nil {
		return err
	}
	_, err = DB.Exec(`UPDATE users SET password_hash = ?, must_change_password = 1 WHERE id = ?`, string(hashedPassword), userID)
	return err
}

// UpdateUser updates user information (admin only)
func UpdateUser(id int, username, email, role string, active bool) error {
	role = NormalizeRole(role)

	query := `UPDATE users SET username = ?, email = ?, role = ?, active = ? WHERE id = ?`
	_, err := DB.Exec(query, username, email, role, active, id)
	return err
}

// DeactivateUser deactivates a user account
func DeactivateUser(id int) error {
	query := `UPDATE users SET active = 0 WHERE id = ?`
	_, err := DB.Exec(query, id)
	return err
}

// ReactivateUser reactivates a user account
func ReactivateUser(id int) error {
	query := `UPDATE users SET active = 1 WHERE id = ?`
	_, err := DB.Exec(query, id)
	return err
}

// weakPasswords sind offensichtliche Kandidaten, die trotz ausreichender
// Laenge nicht zugelassen werden.
var weakPasswords = map[string]bool{
	"passwort12":    true,
	"passwort123":   true,
	"password12":    true,
	"password123":   true,
	"passwort1234":  true,
	"kleingarten":   true,
	"gartenamt1":    true,
	"gartenamt12":   true,
	"1234567890":    true,
	"0123456789":    true,
	"qwertzuiop":    true,
	"qwertyuiop":    true,
	"administrator": true,
	"geheim1234":    true,
	"willkommen1":   true,
}

// ValidatePassword prueft die Passwortregeln ohne Benutzerbezug.
func ValidatePassword(password string) error {
	return ValidatePasswordForUser(password, "")
}

// ValidatePasswordForUser prueft die Passwortregeln und schliesst zusaetzlich
// Passwoerter aus, die den Benutzernamen enthalten.
func ValidatePasswordForUser(password, username string) error {
	if len([]rune(password)) < 10 {
		return errors.New("passwort muss mindestens 10 Zeichen lang sein")
	}

	lower := strings.ToLower(password)

	if weakPasswords[lower] {
		return errors.New("passwort ist zu leicht zu erraten - bitte ein anderes waehlen")
	}

	if username != "" {
		if user := strings.ToLower(strings.TrimSpace(username)); user != "" && strings.Contains(lower, user) {
			return errors.New("passwort darf den Benutzernamen nicht enthalten")
		}
	}

	if isSingleRepeatedRune(password) {
		return errors.New("passwort darf nicht aus einem einzigen wiederholten Zeichen bestehen")
	}

	return nil
}

func isSingleRepeatedRune(password string) bool {
	runes := []rune(password)
	for _, r := range runes[1:] {
		if r != runes[0] {
			return false
		}
	}
	return true
}
