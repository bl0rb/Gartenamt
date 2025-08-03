package models

import (
	"database/sql"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        int        `json:"id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	Password  string     `json:"-"` // Wird nicht in JSON serialisiert
	Role      string     `json:"role"`
	Active    bool       `json:"active"`
	CreatedAt time.Time  `json:"created_at"`
	LastLogin *time.Time `json:"last_login"`
}

// UserRole definiert Benutzerrollen
type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

// CreateUser erstellt einen neuen Benutzer mit verschlüsseltem Passwort
func CreateUser(username, email, password string, role UserRole) (*User, error) {
	// Passwort hashen
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
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
	query := `SELECT id, username, email, password_hash, role, active, created_at, last_login 
              FROM users WHERE username = ? AND active = 1`

	var lastLogin sql.NullTime
	err := DB.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.Password,
		&user.Role, &user.Active, &user.CreatedAt, &lastLogin)

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
	query := `SELECT id, username, email, password_hash, role, active, created_at, last_login 
              FROM users WHERE id = ?`

	var lastLogin sql.NullTime
	err := DB.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.Password,
		&user.Role, &user.Active, &user.CreatedAt, &lastLogin)

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
	query := `SELECT id, username, email, role, active, created_at, last_login 
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
			&user.Role, &user.Active, &user.CreatedAt, &lastLogin)
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

// Add these functions to your existing models/user.go

// ChangePassword ändert das Passwort eines Benutzers
func (u *User) ChangePassword(newPassword string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	query := `UPDATE users SET password_hash = ? WHERE id = ?`
	_, err = DB.Exec(query, string(hashedPassword), u.ID)
	if err != nil {
		return err
	}

	u.Password = string(hashedPassword)
	return nil
}

// UpdateUser updates user information (admin only)
func UpdateUser(id int, username, email, role string, active bool) error {
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

// ValidatePassword validates password requirements
func ValidatePassword(password string) error {
	if len(password) < 6 {
		return errors.New("passwort muss mindestens 6 Zeichen lang sein")
	}

	// Additional password requirements can be added here
	// hasUpper := strings.ContainsAny(password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	// hasLower := strings.ContainsAny(password, "abcdefghijklmnopqrstuvwxyz")
	// hasDigit := strings.ContainsAny(password, "0123456789")

	return nil
}
