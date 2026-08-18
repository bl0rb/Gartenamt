package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/bl0rb/gartenamt/middleware"
	"github.com/bl0rb/gartenamt/models"
)

// writeAudit schreibt einen Audit-Eintrag und traegt dabei Akteur und
// Client-IP aus dem Request ein. Ohne den Akteur beantwortet das Log nur
// "was" und "wann", aber nie "wer". Die IP kommt aus getClientIP und
// beruecksichtigt damit TRUSTED_PROXY_IPS statt blind r.RemoteAddr zu nehmen.
func writeAudit(r *http.Request, aktion, beschreibung string, davor, danach interface{}) {
	entry := models.AuditLog{
		Aktion:       aktion,
		Beschreibung: beschreibung,
		DatenVorher:  davor,
		DatenNachher: danach,
		Zeitstempel:  time.Now(),
		IPAdresse:    getClientIP(r),
		UserAgent:    r.Header.Get("User-Agent"),
	}

	if session := middleware.GetSessionFromContext(r.Context()); session != nil {
		userID := session.UserID
		entry.BenutzerID = &userID
		entry.Benutzer = session.Username
	}

	if err := entry.Save(); err != nil {
		log.Printf("⚠️  Audit-Eintrag %s konnte nicht gespeichert werden: %v", aktion, err)
	}
}
