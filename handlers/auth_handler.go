package handlers

import (
	"errors"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/bl0rb/gartenamt/middleware"
	"github.com/bl0rb/gartenamt/models"
	"github.com/bl0rb/gartenamt/services"

	"github.com/gorilla/mux"
)

// LoginHandler zeigt Login-Formular oder verarbeitet Login
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Bereits eingeloggte Benutzer zur Hauptseite weiterleiten
	if middleware.IsAuthenticated(r) {
		session := middleware.GetSessionFromContext(r.Context())
		if session != nil && models.IsBackofficeRole(session.Role) {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/profile", http.StatusSeeOther)
		return
	}

	if r.Method == "POST" {
		username := strings.TrimSpace(r.FormValue("username"))
		password := r.FormValue("password")
		redirectURL := r.FormValue("redirect")

		// Validierung
		if username == "" || password == "" {
			showLoginWithError(w, r, "Benutzername und Passwort sind erforderlich", redirectURL)
			return
		}

		// IP und User-Agent für Session-Logging
		ipAddress := getClientIP(r)
		userAgent := r.Header.Get("User-Agent")

		// Login-Versuch
		session, err := services.GlobalAuth.Login(username, password, ipAddress, userAgent)
		if err != nil {
			log.Printf("Login-Fehler für Benutzer '%s' von IP %s: %v", username, ipAddress, err)
			if errors.Is(err, services.ErrLoginThrottled) {
				showLoginWithError(w, r, "Zu viele fehlgeschlagene Anmeldeversuche. Bitte versuchen Sie es in 15 Minuten erneut.", redirectURL)
			} else {
				showLoginWithError(w, r, "Ungültige Anmeldedaten", redirectURL)
			}
			return
		}

		// Session-Cookie setzen
		middleware.SetSessionCookie(w, session.ID)

		// Erfolgreiche Anmeldung
		log.Printf("Erfolgreiche Anmeldung: %s (Role: %s, IP: %s)", session.Username, session.Role, ipAddress)

		// Weiterleitung
		if redirectURL != "" && !strings.Contains(redirectURL, "login") {
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		} else {
			if models.IsBackofficeRole(session.Role) {
				http.Redirect(w, r, "/admin", http.StatusSeeOther)
			} else {
				http.Redirect(w, r, "/profile", http.StatusSeeOther)
			}
		}
		return
	}

	// GET Request - Login-Formular anzeigen
	redirectURL := r.URL.Query().Get("redirect")

	data := map[string]interface{}{
		"Title":       "Anmeldung",
		"RedirectURL": redirectURL,
	}
	addInitialCredentials(data)

	tmpl := template.Must(LoadTemplate("templates/login.html"))
	tmpl.Execute(w, data)
}

// addInitialCredentials ergänzt die Initial-Zugangsdaten des Standard-Admins,
// solange dessen Initialpasswort noch nicht geändert wurde.
func addInitialCredentials(data map[string]interface{}) {
	if username, password, ok := services.InitialAdminCredentials(); ok {
		data["InitialUsername"] = username
		data["InitialPassword"] = password
	}
}

// LogoutHandler meldet Benutzer ab
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Session-Cookie lesen
	cookie, err := r.Cookie("session_id")
	if err == nil {
		// Session invalidieren
		services.GlobalAuth.Logout(cookie.Value)
	}

	// Session-Cookie löschen
	middleware.ClearSessionCookie(w)

	// Zur Login-Seite weiterleiten
	http.Redirect(w, r, "/login?message=logout", http.StatusSeeOther)
}

// ProfileHandler zeigt Benutzerprofil
func ProfileHandler(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Nicht authentifiziert", http.StatusUnauthorized)
		return
	}

	// Success message handling
	successMsg := ""
	if r.URL.Query().Get("success") == "password_changed" {
		successMsg = "password_changed"
	}

	user, err := models.GetUserByID(session.UserID)
	if err != nil {
		http.Error(w, "Benutzer nicht gefunden", http.StatusNotFound)
		return
	}

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/profile.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":   "Profil",
		"User":    user,
		"Success": successMsg,
	}))
}

// ChangePasswordHandler handles password changes for current user
func ChangePasswordHandler(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		http.Error(w, "Nicht authentifiziert", http.StatusUnauthorized)
		return
	}

	if r.Method == "POST" {
		currentPassword := r.FormValue("current_password")
		newPassword := r.FormValue("new_password")
		confirmPassword := r.FormValue("confirm_password")

		// Validate input
		if currentPassword == "" || newPassword == "" || confirmPassword == "" {
			showProfileWithError(w, r, "Alle Felder sind erforderlich", session)
			return
		}

		if newPassword != confirmPassword {
			showProfileWithError(w, r, "Neue Passwörter stimmen nicht überein", session)
			return
		}

		if err := models.ValidatePassword(newPassword); err != nil {
			showProfileWithError(w, r, err.Error(), session)
			return
		}

		// Load current user
		user, err := models.GetUserByID(session.UserID)
		if err != nil {
			http.Error(w, "Benutzer nicht gefunden", http.StatusNotFound)
			return
		}

		// Verify current password
		if !user.ValidatePassword(currentPassword) {
			showProfileWithError(w, r, "Aktuelles Passwort ist falsch", session)
			return
		}

		wasInitialPassword := user.MustChangePassword

		// Change password
		if err := user.ChangePassword(newPassword); err != nil {
			showProfileWithError(w, r, "Fehler beim Ändern des Passworts", session)
			return
		}

		// Initial-Zugangsdaten nicht länger auf der Login-Seite anzeigen
		if wasInitialPassword {
			services.ClearInitialAdminCredentials()
			services.GlobalAuth.ClearMustChangePassword(session.ID)
		}

		// Invalidate all other sessions for this user
		services.GlobalAuth.InvalidateOtherUserSessions(user.ID, session.ID)

		log.Printf("Passwort geändert für Benutzer: %s", user.Username)

		// Redirect with success message
		http.Redirect(w, r, "/profile?success=password_changed", http.StatusSeeOther)
		return
	}

	// Should not reach here as profile page handles GET
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

// AdminUserEditHandler handles user editing (admin only)
func AdminUserEditHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Ungültige Benutzer-ID", http.StatusBadRequest)
		return
	}

	user, err := models.GetUserByID(userID)
	if err != nil {
		http.Error(w, "Benutzer nicht gefunden", http.StatusNotFound)
		return
	}

	if r.Method == "POST" {
		action := r.FormValue("action")

		switch action {
		case "update":
			username := strings.TrimSpace(r.FormValue("username"))
			email := strings.TrimSpace(r.FormValue("email"))
			role := r.FormValue("role")
			active := r.FormValue("active") == "1"

			if username == "" || email == "" {
				showUserEditWithError(w, r, user, "Benutzername und E-Mail sind erforderlich")
				return
			}

			// Check if username is taken by another user
			if username != user.Username && models.UserExists(username) {
				showUserEditWithError(w, r, user, "Benutzername bereits vergeben")
				return
			}

			if err := models.UpdateUser(userID, username, email, role, active); err != nil {
				showUserEditWithError(w, r, user, "Fehler beim Aktualisieren des Benutzers")
				return
			}

			log.Printf("Benutzer aktualisiert: %s (Role: %s, Active: %v)", username, role, active)
			http.Redirect(w, r, "/admin/users?success=user_updated", http.StatusSeeOther)
			return

		case "reset_password":
			newPassword := r.FormValue("new_password")
			confirmPassword := r.FormValue("confirm_password")

			if newPassword == "" || confirmPassword == "" {
				showUserEditWithError(w, r, user, "Neue Passwörter sind erforderlich")
				return
			}

			if newPassword != confirmPassword {
				showUserEditWithError(w, r, user, "Passwörter stimmen nicht überein")
				return
			}

			if err := models.ValidatePassword(newPassword); err != nil {
				showUserEditWithError(w, r, user, err.Error())
				return
			}

			// Vom Admin vergebenes Passwort ist nur temporär: der Benutzer muss
			// beim nächsten Login ein eigenes Passwort setzen
			if err := models.SetInitialPassword(user.ID, newPassword); err != nil {
				showUserEditWithError(w, r, user, "Fehler beim Ändern des Passworts")
				return
			}

			// Invalidate all sessions for this user
			services.GlobalAuth.InvalidateUserSessions(userID)

			log.Printf("Passwort zurückgesetzt für Benutzer: %s", user.Username)
			http.Redirect(w, r, "/admin/users?success=password_reset", http.StatusSeeOther)
			return

		case "deactivate":
			if err := models.DeactivateUser(userID); err != nil {
				showUserEditWithError(w, r, user, "Fehler beim Deaktivieren des Benutzers")
				return
			}

			// Invalidate all sessions for this user
			services.GlobalAuth.InvalidateUserSessions(userID)

			log.Printf("Benutzer deaktiviert: %s", user.Username)
			http.Redirect(w, r, "/admin/users?success=user_deactivated", http.StatusSeeOther)
			return

		case "reactivate":
			if err := models.ReactivateUser(userID); err != nil {
				showUserEditWithError(w, r, user, "Fehler beim Reaktivieren des Benutzers")
				return
			}

			log.Printf("Benutzer reaktiviert: %s", user.Username)
			http.Redirect(w, r, "/admin/users?success=user_reactivated", http.StatusSeeOther)
			return
		}
	}

	// GET request - show edit form
	tmpl := template.Must(LoadTemplateWithFuncs(adminUsersTemplateFuncMap(), "templates/layout.html", "templates/admin_user_edit.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":             "Benutzer bearbeiten",
		"User":              user,
		"RoleOptions":       models.GetAllRoles(),
		"PermissionCatalog": models.PermissionCatalog(),
	}))
}

// AdminUsersHandlerEnhanced shows enhanced user management
func AdminUsersHandlerEnhanced(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := strings.TrimSpace(r.FormValue("username"))
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")
		role := r.FormValue("role")

		if username == "" || password == "" || email == "" {
			showUsersListWithError(w, r, "Alle Felder sind erforderlich")
			return
		}

		if models.UserExists(username) {
			showUsersListWithError(w, r, "Benutzername bereits vergeben")
			return
		}

		if err := models.ValidatePassword(password); err != nil {
			showUsersListWithError(w, r, err.Error())
			return
		}

		userRole := models.UserRole(models.NormalizeRole(role))

		_, err := models.CreateUser(username, email, password, userRole)
		if err != nil {
			showUsersListWithError(w, r, "Fehler beim Erstellen des Benutzers: "+err.Error())
			return
		}

		log.Printf("Neuer Benutzer erstellt: %s (Role: %s)", username, userRole)
		http.Redirect(w, r, "/admin/users?success=user_created", http.StatusSeeOther)
		return
	}

	// GET Request - User-Liste anzeigen
	users, err := models.GetAllUsers()
	if err != nil {
		http.Error(w, "Fehler beim Laden der Benutzer", http.StatusInternalServerError)
		return
	}

	// Aktive Sessions für zusätzliche Info laden
	activeSessions := services.GlobalAuth.GetActiveSessions()

	// Success message from URL parameter
	successMsg := ""
	switch r.URL.Query().Get("success") {
	case "user_created":
		successMsg = "Benutzer erfolgreich erstellt"
	case "user_updated":
		successMsg = "Benutzer erfolgreich aktualisiert"
	case "password_reset":
		successMsg = "Passwort erfolgreich zurückgesetzt"
	case "user_deactivated":
		successMsg = "Benutzer erfolgreich deaktiviert"
	case "user_reactivated":
		successMsg = "Benutzer erfolgreich reaktiviert"
	}

	tmpl := template.Must(LoadTemplateWithFuncs(adminUsersTemplateFuncMap(), "templates/layout.html", "templates/admin_users.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":             "Benutzerverwaltung",
		"Users":             users,
		"ActiveSessions":    activeSessions,
		"Success":           successMsg,
		"RoleOptions":       models.GetAllRoles(),
		"PermissionCatalog": models.PermissionCatalog(),
	}))
}

// Helper functions

func showLoginWithError(w http.ResponseWriter, r *http.Request, errorMsg, redirectURL string) {
	data := map[string]interface{}{
		"Title":       "Anmeldung",
		"Error":       errorMsg,
		"RedirectURL": redirectURL,
		"Username":    r.FormValue("username"), // Benutzername beibehalten
	}
	addInitialCredentials(data)

	tmpl := template.Must(LoadTemplate("templates/login.html"))
	tmpl.Execute(w, data)
}

func showProfileWithError(w http.ResponseWriter, r *http.Request, errorMsg string, session *services.Session) {
	user, _ := models.GetUserByID(session.UserID)
	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/profile.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title": "Profil",
		"User":  user,
		"Error": errorMsg,
	}))
}

func showUserEditWithError(w http.ResponseWriter, r *http.Request, user *models.User, errorMsg string) {
	tmpl := template.Must(LoadTemplateWithFuncs(adminUsersTemplateFuncMap(), "templates/layout.html", "templates/admin_user_edit.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":             "Benutzer bearbeiten",
		"User":              user,
		"Error":             errorMsg,
		"RoleOptions":       models.GetAllRoles(),
		"PermissionCatalog": models.PermissionCatalog(),
	}))
}

func showUsersListWithError(w http.ResponseWriter, r *http.Request, errorMsg string) {
	users, _ := models.GetAllUsers()
	activeSessions := services.GlobalAuth.GetActiveSessions()

	tmpl := template.Must(LoadTemplateWithFuncs(adminUsersTemplateFuncMap(), "templates/layout.html", "templates/admin_users.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":             "Benutzerverwaltung",
		"Users":             users,
		"ActiveSessions":    activeSessions,
		"Error":             errorMsg,
		"RoleOptions":       models.GetAllRoles(),
		"PermissionCatalog": models.PermissionCatalog(),
	}))
}

func adminUsersTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		"add": func(a, b int) int {
			return a + b
		},
		"roleLabel": func(role string) string {
			return models.RoleDisplayName(role)
		},
		"roleBadgeClass": func(role string) string {
			return models.RoleBadgeClass(role)
		},
		"hasPermission": func(role, permission string) bool {
			return models.RoleHasPermission(role, permission)
		},
		"isBackoffice": func(role string) bool {
			return models.IsBackofficeRole(role)
		},
	}
}

// trustedProxyIPs wird erst beim ersten Request ausgewertet, damit auch
// Werte aus einer per loadEnvFiles() geladenen .env-Datei greifen.
var trustedProxyIPs = sync.OnceValue(loadTrustedProxyIPs)

func loadTrustedProxyIPs() map[string]bool {
	trusted := make(map[string]bool)
	raw := os.Getenv("TRUSTED_PROXY_IPS")
	if raw == "" {
		return trusted
	}
	for _, ip := range strings.Split(raw, ",") {
		ip = strings.TrimSpace(ip)
		if ip != "" {
			trusted[ip] = true
		}
	}
	return trusted
}

// remoteHost extrahiert den Host-Anteil aus RemoteAddr (ohne Port).
func remoteHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}

func getClientIP(r *http.Request) string {
	directIP := remoteHost(r.RemoteAddr)

	// X-Forwarded-For / X-Real-IP nur vertrauen, wenn der direkte Peer ein
	// vertrauenswürdiger Proxy ist (via TRUSTED_PROXY_IPS konfiguriert).
	if trustedProxyIPs()[directIP] {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			return strings.TrimSpace(strings.Split(xff, ",")[0])
		}

		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return strings.TrimSpace(xri)
		}
	}

	// Standard RemoteAddr verwenden
	return directIP
}
