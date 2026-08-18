package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bl0rb/gartenamt/services"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestSecurityHeadersSetsProtectiveHeaders(t *testing.T) {
	SetSecureCookies(true)
	defer SetSecureCookies(true)

	recorder := httptest.NewRecorder()
	SecurityHeaders(okHandler()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/admin/system-info", nil))

	expected := map[string]string{
		"X-Frame-Options":           "DENY",
		"X-Content-Type-Options":    "nosniff",
		"Referrer-Policy":           "same-origin",
		"Strict-Transport-Security": "max-age=31536000",
	}
	for header, want := range expected {
		if got := recorder.Header().Get(header); got != want {
			t.Errorf("Header %s = %q, erwartet %q", header, got, want)
		}
	}

	policy := recorder.Header().Get("Content-Security-Policy")
	for _, directive := range []string{"default-src 'self'", "frame-ancestors 'none'", "base-uri 'none'", "form-action 'self'"} {
		if !strings.Contains(policy, directive) {
			t.Errorf("CSP enthält %q nicht: %s", directive, policy)
		}
	}
}

// Im Desktop-Modus läuft die App bewusst per HTTP auf localhost - HSTS würde
// den Browser dauerhaft auf HTTPS festnageln.
func TestSecurityHeadersOmitsHSTSWithoutTLS(t *testing.T) {
	SetSecureCookies(false)
	defer SetSecureCookies(true)

	recorder := httptest.NewRecorder()
	SecurityHeaders(okHandler()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := recorder.Header().Get("Strict-Transport-Security"); got != "" {
		t.Errorf("HSTS im Desktop-Modus gesetzt: %q", got)
	}
}

// Der Backup-Schlüssel und die Pächterdaten dürfen nicht im Browser-Cache
// landen; die eingebetteten Assets dagegen schon.
func TestSecurityHeadersNoStoreExceptStatic(t *testing.T) {
	cases := map[string]bool{
		"/admin/system-info":    true,
		"/parzellen/1":          true,
		"/login":                true,
		"/static/style.css":     false,
		"/static/vendor/foo.js": false,
	}

	for path, wantNoStore := range cases {
		recorder := httptest.NewRecorder()
		SecurityHeaders(okHandler()).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))

		got := recorder.Header().Get("Cache-Control")
		if wantNoStore && !strings.Contains(got, "no-store") {
			t.Errorf("%s: Cache-Control = %q, no-store erwartet", path, got)
		}
		if !wantNoStore && strings.Contains(got, "no-store") {
			t.Errorf("%s: Cache-Control = %q, kein no-store erwartet", path, got)
		}
	}
}

func TestCSRFProtect(t *testing.T) {
	cases := []struct {
		name        string
		method      string
		origin      string
		referer     string
		withCookie  bool
		wantAllowed bool
	}{
		{name: "GET immer erlaubt", method: http.MethodGet, wantAllowed: true},
		{name: "POST mit eigenem Origin", method: http.MethodPost, origin: "https://gartenamt.local", wantAllowed: true},
		{name: "POST mit fremdem Origin", method: http.MethodPost, origin: "https://angreifer.tld", wantAllowed: false},
		{name: "POST mit Origin null", method: http.MethodPost, origin: "null", wantAllowed: false},
		{name: "POST mit eigenem Referer", method: http.MethodPost, referer: "https://gartenamt.local/parzellen", wantAllowed: true},
		{name: "POST mit fremdem Referer", method: http.MethodPost, referer: "https://angreifer.tld/x", wantAllowed: false},
		{name: "POST ohne Header und ohne Cookie", method: http.MethodPost, wantAllowed: true},
		// Fail-closed: mit Session-Cookie, aber ohne Origin/Referer kann die
		// Anfrage nicht aus einem Browser-Formular der Anwendung stammen.
		{name: "POST ohne Header mit Session-Cookie", method: http.MethodPost, withCookie: true, wantAllowed: false},
		{name: "DELETE mit fremdem Origin", method: http.MethodDelete, origin: "https://angreifer.tld", wantAllowed: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			request := httptest.NewRequest(c.method, "https://gartenamt.local/admin/users", nil)
			request.Host = "gartenamt.local"
			if c.origin != "" {
				request.Header.Set("Origin", c.origin)
			}
			if c.referer != "" {
				request.Header.Set("Referer", c.referer)
			}
			if c.withCookie {
				request.AddCookie(&http.Cookie{Name: "session_id", Value: "abc"})
			}

			recorder := httptest.NewRecorder()
			CSRFProtect(okHandler()).ServeHTTP(recorder, request)

			allowed := recorder.Code == http.StatusOK
			if allowed != c.wantAllowed {
				t.Errorf("Status %d, durchgelassen = %v, erwartet %v", recorder.Code, allowed, c.wantAllowed)
			}
		})
	}
}

func TestLimitRequestBody(t *testing.T) {
	readAll := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		for {
			if _, err := r.Body.Read(buf); err != nil {
				if err.Error() == "EOF" {
					w.WriteHeader(http.StatusOK)
					return
				}
				w.WriteHeader(http.StatusRequestEntityTooLarge)
				return
			}
		}
	})

	oversized := strings.NewReader(strings.Repeat("x", maxRequestBody+1024))
	request := httptest.NewRequest(http.MethodPost, "/parzellen/neu", oversized)
	recorder := httptest.NewRecorder()
	LimitRequestBody(readAll).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("übergroßer Request wurde nicht begrenzt: Status %d", recorder.Code)
	}

	// Die Upload-Route darf mehr annehmen als das allgemeine Limit.
	upload := strings.NewReader(strings.Repeat("x", maxRequestBody+1024))
	uploadRequest := httptest.NewRequest(http.MethodPost, "/admin/backup", upload)
	uploadRecorder := httptest.NewRecorder()
	LimitRequestBody(readAll).ServeHTTP(uploadRecorder, uploadRequest)

	if uploadRecorder.Code != http.StatusOK {
		t.Errorf("Upload-Route zu früh begrenzt: Status %d", uploadRecorder.Code)
	}
}

func TestHasPermissionWithoutSession(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	if HasPermission(request, "users.manage") {
		t.Error("ohne Session wurde eine Berechtigung bestätigt")
	}
}

// HasPermission muss dieselbe Antwort geben wie die Route-Middleware - sonst
// laufen Route und Handler auseinander.
func TestHasPermissionFollowsRole(t *testing.T) {
	cases := []struct {
		role, permission string
		want             bool
	}{
		{"admin", "users.manage", true},
		{"vorstand", "users.manage", true},
		{"kassenwart", "users.manage", false},
		{"kassenwart", "invoices.manage", true},
		{"wertermittler", "invoices.manage", false},
		{"wertermittler", "protokolle.manage", true},
		{"user", "dashboard.access", false},
	}

	for _, c := range cases {
		request := httptest.NewRequest(http.MethodGet, "/admin", nil)
		ctx := context.WithValue(request.Context(), SessionContextKey, &services.Session{
			ID: "test", UserID: 1, Username: "test", Role: c.role,
		})

		if got := HasPermission(request.WithContext(ctx), c.permission); got != c.want {
			t.Errorf("HasPermission(%q, %q) = %v, erwartet %v", c.role, c.permission, got, c.want)
		}
	}
}
