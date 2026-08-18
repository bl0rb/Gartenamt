package middleware

import (
	"net/http"
	"strings"
)

const (
	// maxRequestBody deckelt gewoehnliche Formular-Requests.
	maxRequestBody = 4 << 20 // 4 MiB
	// maxUploadBody gilt fuer die Routen, die Dateien entgegennehmen
	// (CSV-Import, Backup-Restore und -Pruefung).
	maxUploadBody = 64 << 20 // 64 MiB
)

// uploadPaths sind die Routen, die per Multipart Dateien annehmen.
var uploadPaths = map[string]bool{
	"/admin/backup":  true,
	"/admin/backup/": true,
}

// LimitRequestBody begrenzt die Groesse eingehender Requests. Ohne diese
// Schranke nimmt die Anwendung beliebig grosse Uploads entgegen und schreibt
// sie auf die Platte, bevor irgendeine Groessenpruefung greift.
func LimitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch:
			limit := int64(maxRequestBody)
			if uploadPaths[r.URL.Path] {
				limit = maxUploadBody
			}
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeaders setzt die Schutz-Header fuer jede Antwort.
//
// Die CSP erlaubt 'unsafe-inline' fuer Skripte, weil die Templates mit
// Inline-Bloecken und onclick-Attributen arbeiten. Der Rest der Richtlinie
// greift trotzdem: keine fremden Quellen, keine Einbettung, kein <base>, und
// Formulare koennen nur an die eigene Anwendung gesendet werden. Werden die
// Inline-Handler spaeter ausgelagert, kann 'unsafe-inline' entfallen.
func SecurityHeaders(next http.Handler) http.Handler {
	const policy = "default-src 'self'; " +
		"script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; " +
		"img-src 'self' data:; " +
		"font-src 'self'; " +
		"connect-src 'self'; " +
		"object-src 'none'; " +
		"base-uri 'none'; " +
		"form-action 'self'; " +
		"frame-ancestors 'none'"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Content-Security-Policy", policy)
		header.Set("X-Frame-Options", "DENY")
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("Referrer-Policy", "same-origin")
		header.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")

		// HSTS nur im HTTPS-Server-Modus - im Desktop-Modus laeuft die App
		// bewusst per HTTP auf localhost.
		if secureCookies {
			header.Set("Strict-Transport-Security", "max-age=31536000")
		}

		// Alles ausser den eingebetteten Assets kann Paechterdaten oder
		// Geheimnisse enthalten - etwa der Backup-Schluessel auf der
		// Systeminfo-Seite. Solche Antworten gehoeren nicht in den Cache.
		if !strings.HasPrefix(r.URL.Path, "/static/") {
			header.Set("Cache-Control", "no-store, max-age=0")
			header.Set("Pragma", "no-cache")
		}

		next.ServeHTTP(w, r)
	})
}
