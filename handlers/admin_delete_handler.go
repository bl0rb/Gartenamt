package handlers

import (
	"encoding/json"
	"fmt"
	"html/template"
	"kleingarten-verwaltung/models"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func AdminParzellenVerwaltungHandler(w http.ResponseWriter, r *http.Request) {

	parzellen, err := models.GetAllParzellenMitStatistiken()
	if err != nil {
		log.Printf("ERROR: Fehler beim Laden der Parzellen: %v", err)
		http.Error(w, "Fehler beim Laden der Parzellen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/admin_parzellen_verwalten.html"))
	err = tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":     "Parzellen verwalten",
		"Parzellen": parzellen,
	}))

	if err != nil {
		log.Printf("ERROR: Template-Ausführung fehlgeschlagen: %v", err)
		http.Error(w, "Template-Fehler: "+err.Error(), http.StatusInternalServerError)
		return
	}

}

func AdminProtokollVerwaltungHandler(w http.ResponseWriter, r *http.Request) {

	// Alle Inspektionen laden
	inspektionen, err := models.GetAllInspektionen()
	if err != nil {
		log.Printf("ERROR: Fehler beim Laden der Inspektionen: %v", err)
		http.Error(w, "Fehler beim Laden der Inspektionen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Alle Wertermittlungen laden
	wertermittlungen, err := models.GetAllWertermittlungen()
	if err != nil {
		log.Printf("ERROR: Fehler beim Laden der Wertermittlungen: %v", err)
		http.Error(w, "Fehler beim Laden der Wertermittlungen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/admin_protokolle_verwalten.html"))
	data := map[string]interface{}{
		"Title":            "Protokolle verwalten",
		"Inspektionen":     inspektionen,
		"Wertermittlungen": wertermittlungen,
	}
	tmpl.Execute(w, AddSessionToData(r, data))
}

// AdminInspektionLoeschenHandler - Einzelne Inspektion löschen
func AdminInspektionLoeschenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Nur POST erlaubt", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Ungültige Inspektions-ID", http.StatusBadRequest)
		return
	}

	// Sicherheitscheck: Bestätigung erforderlich
	if r.FormValue("bestaetigung") != "LOESCHEN" {
		http.Error(w, "Bestätigung 'LOESCHEN' erforderlich", http.StatusBadRequest)
		return
	}

	// Inspektion laden für Audit-Log
	inspektion, err := models.GetInspektionByID(id)
	if err != nil {
		http.Error(w, "Inspektion nicht gefunden", http.StatusNotFound)
		return
	}

	// Audit-Log erstellen
	auditEntry := models.AuditLog{
		Aktion:       "INSPEKTION_GELOESCHT",
		Beschreibung: fmt.Sprintf("Inspektion ID %d für Parzelle %d gelöscht", id, inspektion.ParzelleID),
		DatenVorher:  inspektion,
		Zeitstempel:  time.Now(),
		IPAdresse:    r.RemoteAddr,
	}

	// Abhängige Wertermittlungen prüfen
	wertermittlungen, err := models.GetWertermittlungenByInspektionID(id)
	if err == nil && len(wertermittlungen) > 0 {
		// Warnung: Abhängige Wertermittlungen vorhanden
		if r.FormValue("force_delete") != "true" {
			http.Error(w, fmt.Sprintf("Inspektion kann nicht gelöscht werden: %d abhängige Wertermittlungen vorhanden. Verwenden Sie 'Erzwingen' um trotzdem zu löschen.", len(wertermittlungen)), http.StatusConflict)
			return
		}
		// Abhängige Wertermittlungen ebenfalls löschen
		for _, wert := range wertermittlungen {
			if err := models.DeleteWertermittlung(wert.ID); err != nil {
				http.Error(w, fmt.Sprintf("Fehler beim Löschen der abhängigen Wertermittlung %d: %v", wert.ID, err), http.StatusInternalServerError)
				return
			}
		}
	}

	// Inspektion löschen
	if err := models.DeleteInspektion(id); err != nil {
		http.Error(w, "Fehler beim Löschen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit-Log speichern
	auditEntry.Save()

	http.Redirect(w, r, "/admin/protokolle?success=inspektion_geloescht", http.StatusSeeOther)
}

// AdminWertermittlungLoeschenHandler - Einzelne Wertermittlung löschen
func AdminWertermittlungLoeschenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Nur POST erlaubt", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Ungültige Wertermittlungs-ID", http.StatusBadRequest)
		return
	}

	// Sicherheitscheck: Bestätigung erforderlich
	if r.FormValue("bestaetigung") != "LOESCHEN" {
		http.Error(w, "Bestätigung 'LOESCHEN' erforderlich", http.StatusBadRequest)
		return
	}

	// Wertermittlung laden für Audit-Log
	wertermittlung, err := models.GetWertermittlungByID(id)
	if err != nil {
		http.Error(w, "Wertermittlung nicht gefunden", http.StatusNotFound)
		return
	}

	// Audit-Log erstellen
	auditEntry := models.AuditLog{
		Aktion:       "WERTERMITTLUNG_GELOESCHT",
		Beschreibung: fmt.Sprintf("Wertermittlung ID %d für Parzelle %d gelöscht (Wert: %.2f €)", id, wertermittlung.ParzelleID, wertermittlung.GesamtWert),
		DatenVorher:  wertermittlung,
		Zeitstempel:  time.Now(),
		IPAdresse:    r.RemoteAddr,
	}

	// Wertermittlung löschen
	if err := models.DeleteWertermittlung(id); err != nil {
		http.Error(w, "Fehler beim Löschen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit-Log speichern
	auditEntry.Save()

	http.Redirect(w, r, "/admin/protokolle?success=wertermittlung_geloescht", http.StatusSeeOther)
}

// AdminBulkDeleteHandler - Mehrere Einträge gleichzeitig löschen
func AdminBulkDeleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Nur POST erlaubt", http.StatusMethodNotAllowed)
		return
	}

	typ := r.FormValue("type")   // "parzellen", "inspektionen", "wertermittlungen"
	idsStr := r.FormValue("ids") // Comma-separated IDs

	// Sicherheitscheck: Bulk-Bestätigung erforderlich
	if r.FormValue("bulk_bestaetigung") != "BULK_LOESCHEN" {
		http.Error(w, "Bulk-Bestätigung 'BULK_LOESCHEN' erforderlich", http.StatusBadRequest)
		return
	}

	var ids []int
	if err := json.Unmarshal([]byte(idsStr), &ids); err != nil {
		http.Error(w, "Ungültige ID-Liste", http.StatusBadRequest)
		return
	}

	if len(ids) == 0 {
		http.Error(w, "Keine IDs zum Löschen ausgewählt", http.StatusBadRequest)
		return
	}

	// Bulk-Delete durchführen
	deletedCount := 0
	errors := []string{}

	for _, id := range ids {
		var err error
		switch typ {
		case "parzellen":
			err = loescheParzelleMitAbhaengigkeiten(id)
		case "inspektionen":
			err = models.DeleteInspektion(id)
		case "wertermittlungen":
			err = models.DeleteWertermittlung(id)
		default:
			err = fmt.Errorf("unbekannter Typ: %s", typ)
		}

		if err != nil {
			errors = append(errors, fmt.Sprintf("ID %d: %s", id, err.Error()))
		} else {
			deletedCount++
		}
	}

	// Audit-Log für Bulk-Delete
	auditEntry := models.AuditLog{
		Aktion:       "BULK_DELETE_" + typ,
		Beschreibung: fmt.Sprintf("Bulk-Delete: %d von %d Einträgen (%s) erfolgreich gelöscht", deletedCount, len(ids), typ),
		DatenVorher: map[string]interface{}{
			"ids":    ids,
			"typ":    typ,
			"errors": errors,
		},
		Zeitstempel: time.Now(),
		IPAdresse:   r.RemoteAddr,
	}
	auditEntry.Save()

	// Erfolgs-/Fehlermeldung
	if len(errors) > 0 {
		log.Printf("Bulk-Delete Fehler: %v", errors)
	}

	redirectUrl := fmt.Sprintf("/admin/%s?success=bulk_deleted&count=%d", typ, deletedCount)
	if len(errors) > 0 {
		redirectUrl += fmt.Sprintf("&errors=%d", len(errors))
	}

	http.Redirect(w, r, redirectUrl, http.StatusSeeOther)
}

// AdminAuditLogHandler - Audit-Log anzeigen
func AdminAuditLogHandler(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	auditLogs, err := models.GetAuditLogs(limit)
	if err != nil {
		http.Error(w, "Fehler beim Laden des Audit-Logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/admin_audit_log.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":     "Audit-Log",
		"AuditLogs": auditLogs,
		"Limit":     limit,
	}))
}

// AdminSystemInfoHandler - System-Informationen und Statistiken
func AdminSystemInfoHandler(w http.ResponseWriter, r *http.Request) {
	revealedBackupKey := ""
	keySuccess := r.URL.Query().Get("key_success")
	keyError := r.URL.Query().Get("key_error")

	if r.Method == "POST" {
		action := r.FormValue("action")
		switch action {
		case "reveal_backup_key":
			fullKey, _, err := models.RevealBackupKey()
			if err != nil {
				keyError = "reveal_limit"
			} else {
				revealedBackupKey = fullKey
				keySuccess = "revealed"
			}
		case "ack_backup_key":
			if err := models.AcknowledgeBackupKeySaved(); err != nil {
				http.Redirect(w, r, "/admin/system-info?key_error=ack_failed", http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/admin/system-info?key_success=acknowledged", http.StatusSeeOther)
			return
		default:
			http.Redirect(w, r, "/admin/system-info?key_error=invalid_action", http.StatusSeeOther)
			return
		}
	}

	systemInfo := models.GetSystemInfo()
	appMeta := getApplicationMetadata()
	backupKeyState, _ := models.GetBackupKeyState()
	if backupKeyState == nil {
		backupKeyState = &models.BackupKeyState{}
	}

	funcMap := template.FuncMap{
		"divMB": func(size int64) float64 {
			return float64(size) / (1024 * 1024)
		},
	}

	tmpl := template.Must(LoadTemplateWithFuncs(funcMap, "templates/layout.html", "templates/admin_system_info.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":          "System-Information",
		"SystemInfo":     systemInfo,
		"AppMeta":        appMeta,
		"BackupKeyState": backupKeyState,
		"BackupKey":      revealedBackupKey,
		"KeySuccess":     keySuccess,
		"KeyError":       keyError,
	}))
}

func getApplicationMetadata() map[string]string {
	metadata := map[string]string{
		"Version":   readApplicationVersion(),
		"BuildTime": "Unbekannt",
		"Revision":  "Unbekannt",
		"GoVersion": runtime.Version(),
		"Platform":  runtime.GOOS + "/" + runtime.GOARCH,
	}

	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				if setting.Value != "" {
					metadata["Revision"] = setting.Value
				}
			case "vcs.time":
				if setting.Value != "" {
					metadata["BuildTime"] = setting.Value
				}
			}
		}
	}

	if metadata["BuildTime"] == "Unbekannt" {
		if executablePath, err := os.Executable(); err == nil {
			if executableInfo, statErr := os.Stat(executablePath); statErr == nil {
				metadata["BuildTime"] = executableInfo.ModTime().Format("02.01.2006 15:04:05")
			}
		}
	}

	return metadata
}

func readApplicationVersion() string {
	content, err := os.ReadFile("VERSION")
	if err != nil {
		return "Unbekannt"
	}

	version := strings.TrimSpace(string(content))
	if version == "" {
		return "Unbekannt"
	}

	return version
}

func loescheParzelleMitAbhaengigkeiten(parzelleID int) error {
	// In einer Transaktion alle abhängigen Daten löschen
	tx, err := models.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Wertermittlungen löschen
	_, err = tx.Exec("DELETE FROM wertermittlungen WHERE parzelle_id = ?", parzelleID)
	if err != nil {
		return err
	}

	// Inspektionen löschen
	_, err = tx.Exec("DELETE FROM inspektionen WHERE parzelle_id = ?", parzelleID)
	if err != nil {
		return err
	}

	// Parzelle löschen
	_, err = tx.Exec("DELETE FROM parzellen WHERE id = ?", parzelleID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// AdminParzellenLoeschenHandler - Parzellen mit Sicherheitsabfrage löschen (nur POST,
// die Bestätigung erfolgt über den Modal-Dialog in der Parzellen-Verwaltung)
func AdminParzellenLoeschenHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Ungültige Parzellen-ID", http.StatusBadRequest)
		return
	}

	// Sicherheitscheck: Bestätigung erforderlich
	if r.FormValue("bestaetigung") != "LOESCHEN" {
		http.Error(w, "Bestätigung erforderlich", http.StatusBadRequest)
		return
	}

	// Parzelle und alle abhängigen Daten löschen
	if err := loescheParzelleMitAbhaengigkeiten(id); err != nil {
		http.Error(w, "Fehler beim Löschen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin?success=parzelle_geloescht", http.StatusSeeOther)
}
