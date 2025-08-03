package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"kleingarten-verwaltung/models"
	"kleingarten-verwaltung/services"
)

// AdminBackupHandler - Erweitert für CSV Export/Import
// AdminBackupHandler - Erweitert für CSV Export/Import
func AdminBackupHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		handleBackupPost(w, r) // ÄNDERUNG: "return" entfernt
		return                 // HINZUGEFÜGT: explizites return
	}

	// GET - Backup/CSV Interface anzeigen
	csvService := services.NewCSVService()

	// Bestehende Export-Dateien auflisten
	exportFiles, _ := filepath.Glob("exports/*.csv")
	backupFiles, _ := filepath.Glob("backups/*.db")

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/admin_backup.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":       "Backup & CSV Export/Import",
		"ExportFiles": exportFiles,
		"BackupFiles": backupFiles,
		"CSVService":  csvService,
	})
}

func handleBackupPost(w http.ResponseWriter, r *http.Request) {
	action := r.FormValue("action")

	switch action {
	case "csv_export_all":
		handleCSVExportAll(w, r)
	case "csv_export_single":
		handleCSVExportSingle(w, r)
	case "csv_import":
		handleCSVImport(w, r)
	case "database_backup":
		handleDatabaseBackup(w, r)
	default:
		http.Error(w, "Unbekannte Aktion", http.StatusBadRequest)
	}
}

func handleCSVExportAll(w http.ResponseWriter, r *http.Request) {
	csvService := services.NewCSVService()

	log.Println("Starte vollständigen CSV-Export...")
	fileName, err := csvService.ExportAllData()
	if err != nil {
		http.Error(w, "Fehler beim CSV-Export: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit-Log
	auditEntry := models.AuditLog{
		Aktion:       "CSV_EXPORT_KOMPLETT",
		Beschreibung: "Vollständiger CSV-Export erstellt: " + fileName,
		Zeitstempel:  time.Now(),
		IPAdresse:    r.RemoteAddr,
	}
	auditEntry.Save()

	// Erste Datei zum Download anbieten (normalerweise wäre das ein ZIP)
	filePath := filepath.Join("exports", fileName)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	http.ServeFile(w, r, filePath)
}

func handleCSVExportSingle(w http.ResponseWriter, r *http.Request) {
	tableName := r.FormValue("table")
	if tableName == "" {
		http.Error(w, "Tabellen-Name erforderlich", http.StatusBadRequest)
		return
	}

	csvService := services.NewCSVService()

	var fileName string
	var err error

	switch tableName {
	case "parzellen":
		fileName, err = csvService.ExportParzellen()
	case "wertermittlungen":
		fileName, err = csvService.ExportWertermittlungen()
	case "obstarten":
		fileName, err = csvService.ExportObstarten()
	case "zieranpflanzungen":
		fileName, err = csvService.ExportZieranpflanzungen()
	case "bauindex":
		fileName, err = csvService.ExportBauindex()
	default:
		http.Error(w, "Unbekannte Tabelle: "+tableName, http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, "Fehler beim CSV-Export: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit-Log
	auditEntry := models.AuditLog{
		Aktion:       "CSV_EXPORT_" + tableName,
		Beschreibung: fmt.Sprintf("CSV-Export für %s erstellt: %s", tableName, fileName),
		Zeitstempel:  time.Now(),
		IPAdresse:    r.RemoteAddr,
	}
	auditEntry.Save()

	// Datei zum Download anbieten
	filePath := filepath.Join("exports", fileName)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+fileName)
	http.ServeFile(w, r, filePath)
}

func handleCSVImport(w http.ResponseWriter, r *http.Request) {
	tableName := r.FormValue("table")
	if tableName == "" {
		http.Error(w, "Tabellen-Name erforderlich", http.StatusBadRequest)
		return
	}

	// File Upload verarbeiten
	file, fileHeader, err := r.FormFile("csv_file")
	if err != nil {
		http.Error(w, "Fehler beim Datei-Upload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Dateigröße prüfen (max 10MB)
	if fileHeader.Size > 10*1024*1024 {
		http.Error(w, "Datei zu groß (max. 10MB)", http.StatusBadRequest)
		return
	}

	// Dateiendung prüfen
	if filepath.Ext(fileHeader.Filename) != ".csv" {
		http.Error(w, "Nur CSV-Dateien erlaubt", http.StatusBadRequest)
		return
	}

	csvService := services.NewCSVService()

	log.Printf("Starte CSV-Import für Tabelle %s aus Datei %s", tableName, fileHeader.Filename)
	err = csvService.ImportFromFile(fileHeader, tableName)
	if err != nil {
		log.Printf("CSV-Import-Fehler: %v", err)
		http.Error(w, "Fehler beim CSV-Import: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit-Log
	auditEntry := models.AuditLog{
		Aktion:       "CSV_IMPORT_" + tableName,
		Beschreibung: fmt.Sprintf("CSV-Import für %s aus Datei %s erfolgreich", tableName, fileHeader.Filename),
		DatenVorher: map[string]interface{}{
			"dateiname":  fileHeader.Filename,
			"dateigröße": fileHeader.Size,
			"tabelle":    tableName,
		},
		Zeitstempel: time.Now(),
		IPAdresse:   r.RemoteAddr,
	}
	auditEntry.Save()

	// Erfolgs-Redirect
	http.Redirect(w, r, "/admin/backup?success=import&table="+tableName, http.StatusSeeOther)
}

func handleDatabaseBackup(w http.ResponseWriter, r *http.Request) {
	// Bestehende Datenbank-Backup-Funktionalität
	backupFile, err := models.CreateDatabaseBackup()
	if err != nil {
		http.Error(w, "Fehler beim Erstellen des Backups: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Audit-Log
	auditEntry := models.AuditLog{
		Aktion:       "BACKUP_ERSTELLT",
		Beschreibung: "Datenbank-Backup erstellt: " + backupFile,
		Zeitstempel:  time.Now(),
		IPAdresse:    r.RemoteAddr,
	}
	auditEntry.Save()

	// Backup zum Download anbieten
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename="+backupFile)
	http.ServeFile(w, r, "backups/"+backupFile)
}
