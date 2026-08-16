package middleware

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/bl0rb/gartenamt/models"
	"github.com/bl0rb/gartenamt/services"
)

// ContextKey für Session-Daten
type ContextKey string

const (
	SessionContextKey ContextKey = "session"
	UserContextKey    ContextKey = "user"
)

// RequireAuth Middleware für geschützte Routen
func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Session-Cookie lesen
		cookie, err := r.Cookie("session_id")
		if err != nil {
			// Kein Cookie = redirect zu Login
			redirectToLogin(w, r)
			return
		}

		// Session validieren
		session, err := services.GlobalAuth.ValidateSession(cookie.Value)
		if err != nil {
			// Ungültige Session = Cookie löschen und redirect
			ClearSessionCookie(w)
			redirectToLogin(w, r)
			return
		}

		// Session in Context einbetten
		ctx := context.WithValue(r.Context(), SessionContextKey, session)

		// Request mit Context weiterleiten
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// RequireAdmin Middleware für Admin-Routen
func RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		session := GetSessionFromContext(r.Context())
		if session == nil || session.Role != "admin" {
			http.Error(w, "Zugriff verweigert - Administrator-Rechte erforderlich", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireBackoffice erlaubt Zugriff für Rollen mit Verwaltungszugriff.
func RequireBackoffice(next http.HandlerFunc) http.HandlerFunc {
	return RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		session := GetSessionFromContext(r.Context())
		if session == nil || !models.IsBackofficeRole(session.Role) {
			http.Error(w, "Zugriff verweigert - Verwaltungsrechte erforderlich", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequirePermission erlaubt Zugriff nur bei expliziter Berechtigung.
func RequirePermission(permission string, next http.HandlerFunc) http.HandlerFunc {
	return RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		session := GetSessionFromContext(r.Context())
		if session == nil || !models.RoleHasPermission(session.Role, permission) {
			http.Error(w, "Zugriff verweigert - Berechtigung fehlt", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CSRFProtect weist state-ändernde Requests ab, deren Origin/Referer nicht zum
// eigenen Host passt. Requests ohne beide Header (z.B. curl) werden durchgelassen,
// da CSRF nur über automatisch mitgesendete Browser-Cookies funktioniert.
func CSRFProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			if !sameOriginRequest(r) {
				http.Error(w, "Zugriff verweigert - Cross-Origin-Anfrage blockiert", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func sameOriginRequest(r *http.Request) bool {
	check := func(raw string) bool {
		parsed, err := url.Parse(raw)
		if err != nil {
			return false
		}
		return strings.EqualFold(parsed.Host, r.Host)
	}

	if origin := r.Header.Get("Origin"); origin != "" {
		// "Origin: null" (z.B. sandboxed iframe) ist nie same-origin
		if origin == "null" {
			return false
		}
		return check(origin)
	}
	if referer := r.Header.Get("Referer"); referer != "" {
		return check(referer)
	}
	return true
}

// OptionalAuth Middleware für Routen die sowohl authenticated als auch anonymous funktionieren
func OptionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Session-Cookie lesen (ohne Fehler bei fehlendem Cookie)
		cookie, err := r.Cookie("session_id")
		if err == nil {
			// Session validieren
			session, err := services.GlobalAuth.ValidateSession(cookie.Value)
			if err == nil {
				// Gültige Session in Context einbetten
				ctx := context.WithValue(r.Context(), SessionContextKey, session)
				r = r.WithContext(ctx)
			} else {
				// Ungültige Session = Cookie löschen
				ClearSessionCookie(w)
			}
		}

		next.ServeHTTP(w, r)
	}
}

// CheckLoginStatus prüft ob der Benutzer eingeloggt ist (für Templates)
func CheckLoginStatus(next http.HandlerFunc) http.HandlerFunc {
	return OptionalAuth(next)
}

// Hilfsfunktionen

// GetSessionFromContext extrahiert die Session aus dem Context
func GetSessionFromContext(ctx context.Context) *services.Session {
	if session, ok := ctx.Value(SessionContextKey).(*services.Session); ok {
		return session
	}
	return nil
}

// IsAuthenticated prüft ob eine Anfrage authentifiziert ist
func IsAuthenticated(r *http.Request) bool {
	return GetSessionFromContext(r.Context()) != nil
}

// IsAdmin prüft ob der aktuelle Benutzer Administrator ist
func IsAdmin(r *http.Request) bool {
	session := GetSessionFromContext(r.Context())
	return session != nil && session.Role == "admin"
}

// IsBackoffice prüft ob die aktuelle Rolle Verwaltungszugriff hat.
func IsBackoffice(r *http.Request) bool {
	session := GetSessionFromContext(r.Context())
	return session != nil && models.IsBackofficeRole(session.Role)
}

// GetCurrentUser gibt den aktuellen Benutzer zurück
func GetCurrentUser(r *http.Request) *services.Session {
	return GetSessionFromContext(r.Context())
}

func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	// AJAX-Requests erhalten JSON-Response
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "Authentication required", "redirect": "/login"}`))
		return
	}

	// Normale Requests werden zu Login weitergeleitet
	http.Redirect(w, r, "/login?redirect="+r.URL.Path, http.StatusSeeOther)
}

// ClearSessionCookie löscht das Session-Cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}

// SetSessionCookie setzt das Session-Cookie
func SetSessionCookie(w http.ResponseWriter, sessionID string) {
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400, // 24 Stunden
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}
