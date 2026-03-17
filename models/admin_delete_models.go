package models

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var appStartedAt = time.Now()

// ParzelleStatistik für Admin-Übersicht
type ParzelleStatistik struct {
	Parzelle               *Parzelle  `json:"parzelle"`
	AnzahlInspektionen     int        `json:"anzahl_inspektionen"`
	AnzahlWertermittlungen int        `json:"anzahl_wertermittlungen"`
	LetzteAktivitaet       *time.Time `json:"letzte_aktivitaet"`
	GesamtWert             float64    `json:"gesamt_wert"`
}

// AuditLog für Nachverfolgung von Admin-Aktionen
type AuditLog struct {
	ID           int         `json:"id"`
	Aktion       string      `json:"aktion"`
	Beschreibung string      `json:"beschreibung"`
	DatenVorher  interface{} `json:"daten_vorher"`
	DatenNachher interface{} `json:"daten_nachher"`
	Zeitstempel  time.Time   `json:"zeitstempel"`
	IPAdresse    string      `json:"ip_adresse"`
	BenutzerID   *int        `json:"benutzer_id,omitempty"`
}

// SystemInfo für Admin-Dashboard
type SystemInfo struct {
	DatabaseSize            int64      `json:"database_size"`
	AnzahlParzellen         int        `json:"anzahl_parzellen"`
	AnzahlInspektionen      int        `json:"anzahl_inspektionen"`
	AnzahlWertermittlungen  int        `json:"anzahl_wertermittlungen"`
	AnzahlObstarten         int        `json:"anzahl_obstarten"`
	AnzahlZieranpflanzungen int        `json:"anzahl_zieranpflanzungen"`
	LetztesBackup           *time.Time `json:"letztes_backup"`
	SystemStart             time.Time  `json:"system_start"`
}

// Erweiterte Parzellen-Funktionen
// Erweiterte Parzellen-Funktionen
func GetAllParzellenMitStatistiken() ([]ParzelleStatistik, error) {
	query := `
		SELECT p.id, p.nummer, p.groesse, p.verein, p.paechter_name, p.kuendigung_datum, p.erstellt_am,
		       COUNT(DISTINCT i.id) as inspektionen_count,
		       COUNT(DISTINCT w.id) as wertermittlungen_count,
		       MAX(COALESCE(w.datum, i.datum)) as letzte_aktivitaet,
		       COALESCE(MAX(w.gesamt_wert), 0) as letzter_gesamtwert
		FROM parzellen p
		LEFT JOIN inspektionen i ON p.id = i.parzelle_id
		LEFT JOIN wertermittlungen w ON p.id = w.parzelle_id
		GROUP BY p.id, p.nummer, p.groesse, p.verein, p.paechter_name, p.kuendigung_datum, p.erstellt_am
		ORDER BY p.nummer`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parzellen []ParzelleStatistik
	for rows.Next() {
		var ps ParzelleStatistik
		var p Parzelle
		var kuendigungDatum sql.NullTime
		var letzteAktivitaetStr sql.NullString // GEÄNDERT: als NullString scannen

		err := rows.Scan(&p.ID, &p.Nummer, &p.Groesse, &p.Verein, &p.PaechterName,
			&kuendigungDatum, &p.ErstelltAm, &ps.AnzahlInspektionen,
			&ps.AnzahlWertermittlungen, &letzteAktivitaetStr, &ps.GesamtWert)
		if err != nil {
			return nil, err
		}

		if kuendigungDatum.Valid {
			p.KuendigungDatum = &kuendigungDatum.Time
		}

		// GEÄNDERT: String zu time.Time parsen
		if letzteAktivitaetStr.Valid && letzteAktivitaetStr.String != "" {
			// Verschiedene Datumsformate versuchen
			timeFormats := []string{
				"2006-01-02 15:04:05",
				"2006-01-02T15:04:05Z",
				"2006-01-02",
				time.RFC3339,
			}

			for _, format := range timeFormats {
				if t, err := time.Parse(format, letzteAktivitaetStr.String); err == nil {
					ps.LetzteAktivitaet = &t
					break
				}
			}
		}

		ps.Parzelle = &p
		parzellen = append(parzellen, ps)
	}
	return parzellen, nil
}

// Inspektionen-Funktionen erweitern
func GetAllInspektionen() ([]struct {
	Inspektion     *Inspektion `json:"inspektion"`
	ParzelleNummer string      `json:"parzelle_nummer"`
}, error) {
	query := `
		SELECT i.id, i.parzelle_id, i.datum, i.maengel, i.auflagen_erfuellt, i.frist, i.erstellt_am,
		       p.nummer as parzelle_nummer
		FROM inspektionen i
		JOIN parzellen p ON i.parzelle_id = p.id
		ORDER BY i.datum DESC`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var inspektionen []struct {
		Inspektion     *Inspektion `json:"inspektion"`
		ParzelleNummer string      `json:"parzelle_nummer"`
	}

	for rows.Next() {
		var i Inspektion
		var maengelJSON string
		var frist sql.NullTime
		var parzelleNummer string

		err := rows.Scan(&i.ID, &i.ParzelleID, &i.Datum, &maengelJSON,
			&i.AuflagenErfuellt, &frist, &i.ErstelltAm, &parzelleNummer)
		if err != nil {
			return nil, err
		}

		json.Unmarshal([]byte(maengelJSON), &i.Maengel)
		if frist.Valid {
			i.Frist = &frist.Time
		}

		inspektionen = append(inspektionen, struct {
			Inspektion     *Inspektion `json:"inspektion"`
			ParzelleNummer string      `json:"parzelle_nummer"`
		}{
			Inspektion:     &i,
			ParzelleNummer: parzelleNummer,
		})
	}
	return inspektionen, nil
}

// Wertermittlungen-Funktionen erweitern
func GetAllWertermittlungen() ([]struct {
	Wertermittlung *Wertermittlung `json:"wertermittlung"`
	ParzelleNummer string          `json:"parzelle_nummer"`
}, error) {
	query := `
		SELECT w.id, w.parzelle_id, w.inspektion_id, w.datum, w.laube_wert, w.baulichkeiten_wert,
		       w.gemuese_wert, w.obst_wert, w.zier_wert, w.gesamt_wert, w.details, w.erstellt_am,
		       p.nummer as parzelle_nummer
		FROM wertermittlungen w
		JOIN parzellen p ON w.parzelle_id = p.id
		ORDER BY w.datum DESC`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wertermittlungen []struct {
		Wertermittlung *Wertermittlung `json:"wertermittlung"`
		ParzelleNummer string          `json:"parzelle_nummer"`
	}

	for rows.Next() {
		var w Wertermittlung
		var detailsJSON string
		var inspektionID sql.NullInt64
		var parzelleNummer string

		err := rows.Scan(&w.ID, &w.ParzelleID, &inspektionID, &w.Datum, &w.LaubeWert,
			&w.BaulichkeitenWert, &w.GemuseWert, &w.ObstWert, &w.ZierWert, &w.GesamtWert,
			&detailsJSON, &w.ErstelltAm, &parzelleNummer)
		if err != nil {
			return nil, err
		}

		if inspektionID.Valid {
			id := int(inspektionID.Int64)
			w.InspektionID = &id
		}
		json.Unmarshal([]byte(detailsJSON), &w.Details)

		wertermittlungen = append(wertermittlungen, struct {
			Wertermittlung *Wertermittlung `json:"wertermittlung"`
			ParzelleNummer string          `json:"parzelle_nummer"`
		}{
			Wertermittlung: &w,
			ParzelleNummer: parzelleNummer,
		})
	}
	return wertermittlungen, nil
}

// Lösch-Funktionen
func GetInspektionByID(id int) (*Inspektion, error) {
	query := `SELECT id, parzelle_id, datum, maengel, auflagen_erfuellt, frist, erstellt_am 
              FROM inspektionen WHERE id = ?`
	row := DB.QueryRow(query, id)

	var i Inspektion
	var maengelJSON string
	var frist sql.NullTime

	err := row.Scan(&i.ID, &i.ParzelleID, &i.Datum, &maengelJSON, &i.AuflagenErfuellt, &frist, &i.ErstelltAm)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(maengelJSON), &i.Maengel)
	if frist.Valid {
		i.Frist = &frist.Time
	}

	return &i, nil
}

func GetWertermittlungByID(id int) (*Wertermittlung, error) {
	query := `SELECT id, parzelle_id, inspektion_id, datum, laube_wert, baulichkeiten_wert, 
              gemuese_wert, obst_wert, zier_wert, gesamt_wert, details, erstellt_am 
              FROM wertermittlungen WHERE id = ?`
	row := DB.QueryRow(query, id)

	var w Wertermittlung
	var detailsJSON string
	var inspektionID sql.NullInt64

	err := row.Scan(&w.ID, &w.ParzelleID, &inspektionID, &w.Datum, &w.LaubeWert,
		&w.BaulichkeitenWert, &w.GemuseWert, &w.ObstWert, &w.ZierWert, &w.GesamtWert,
		&detailsJSON, &w.ErstelltAm)
	if err != nil {
		return nil, err
	}

	if inspektionID.Valid {
		id := int(inspektionID.Int64)
		w.InspektionID = &id
	}
	json.Unmarshal([]byte(detailsJSON), &w.Details)

	return &w, nil
}

func GetWertermittlungenByInspektionID(inspektionID int) ([]Wertermittlung, error) {
	query := `SELECT id, parzelle_id, inspektion_id, datum, laube_wert, baulichkeiten_wert, 
              gemuese_wert, obst_wert, zier_wert, gesamt_wert, details, erstellt_am 
              FROM wertermittlungen WHERE inspektion_id = ?`
	rows, err := DB.Query(query, inspektionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var wertermittlungen []Wertermittlung
	for rows.Next() {
		var w Wertermittlung
		var detailsJSON string
		var inspektionIDNull sql.NullInt64

		err := rows.Scan(&w.ID, &w.ParzelleID, &inspektionIDNull, &w.Datum, &w.LaubeWert,
			&w.BaulichkeitenWert, &w.GemuseWert, &w.ObstWert, &w.ZierWert, &w.GesamtWert,
			&detailsJSON, &w.ErstelltAm)
		if err != nil {
			return nil, err
		}

		if inspektionIDNull.Valid {
			id := int(inspektionIDNull.Int64)
			w.InspektionID = &id
		}
		json.Unmarshal([]byte(detailsJSON), &w.Details)
		wertermittlungen = append(wertermittlungen, w)
	}
	return wertermittlungen, nil
}

func DeleteInspektion(id int) error {
	query := `DELETE FROM inspektionen WHERE id = ?`
	_, err := DB.Exec(query, id)
	return err
}

func DeleteWertermittlung(id int) error {
	query := `DELETE FROM wertermittlungen WHERE id = ?`
	_, err := DB.Exec(query, id)
	return err
}

// Audit-Log Funktionen
func (a *AuditLog) Save() error {
	datenVorherJSON, _ := json.Marshal(a.DatenVorher)
	datenNachherJSON, _ := json.Marshal(a.DatenNachher)

	query := `INSERT INTO audit_log (aktion, beschreibung, daten_vorher, daten_nachher, zeitstempel, ip_adresse, benutzer_id) 
              VALUES (?, ?, ?, ?, ?, ?, ?)`
	result, err := DB.Exec(query, a.Aktion, a.Beschreibung, string(datenVorherJSON),
		string(datenNachherJSON), a.Zeitstempel, a.IPAdresse, a.BenutzerID)
	if err != nil {
		return err
	}
	id, _ := result.LastInsertId()
	a.ID = int(id)
	return nil
}

func GetAuditLogs(limit int) ([]AuditLog, error) {
	query := `SELECT id, aktion, beschreibung, daten_vorher, daten_nachher, zeitstempel, ip_adresse, benutzer_id 
              FROM audit_log ORDER BY zeitstempel DESC LIMIT ?`
	rows, err := DB.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var auditLogs []AuditLog
	for rows.Next() {
		var a AuditLog
		var datenVorherJSON, datenNachherJSON string
		var benutzerID sql.NullInt64

		err := rows.Scan(&a.ID, &a.Aktion, &a.Beschreibung, &datenVorherJSON,
			&datenNachherJSON, &a.Zeitstempel, &a.IPAdresse, &benutzerID)
		if err != nil {
			return nil, err
		}

		json.Unmarshal([]byte(datenVorherJSON), &a.DatenVorher)
		json.Unmarshal([]byte(datenNachherJSON), &a.DatenNachher)
		if benutzerID.Valid {
			id := int(benutzerID.Int64)
			a.BenutzerID = &id
		}

		auditLogs = append(auditLogs, a)
	}
	return auditLogs, nil
}

// Backup-Funktionen
func CreateDatabaseBackup() (string, error) {
	// Backup-Verzeichnis erstellen falls nicht vorhanden
	backupDir := "backups"
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", err
	}

	// Backup-Dateiname mit Zeitstempel
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFile := fmt.Sprintf("kleingarten_backup_%s.db", timestamp)
	backupPath := filepath.Join(backupDir, backupFile)

	// SQLite-Datenbank kopieren
	sourceFile, err := os.Open("kleingarten.db")
	if err != nil {
		return "", err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(backupPath)
	if err != nil {
		return "", err
	}
	defer destFile.Close()

	// Datei kopieren
	_, err = destFile.ReadFrom(sourceFile)
	if err != nil {
		return "", err
	}

	log.Printf("Backup erstellt: %s", backupPath)
	return backupFile, nil
}

// System-Informations-Funktionen
func GetSystemInfo() SystemInfo {
	var info SystemInfo
	info.SystemStart = appStartedAt

	// Datenbank-Größe
	if stat, err := os.Stat("kleingarten.db"); err == nil {
		info.DatabaseSize = stat.Size()
	}

	// Anzahl Einträge zählen
	DB.QueryRow("SELECT COUNT(*) FROM parzellen").Scan(&info.AnzahlParzellen)
	DB.QueryRow("SELECT COUNT(*) FROM inspektionen").Scan(&info.AnzahlInspektionen)
	DB.QueryRow("SELECT COUNT(*) FROM wertermittlungen").Scan(&info.AnzahlWertermittlungen)
	DB.QueryRow("SELECT COUNT(*) FROM obstarten WHERE aktiv = TRUE").Scan(&info.AnzahlObstarten)
	DB.QueryRow("SELECT COUNT(*) FROM zieranpflanzungen WHERE aktiv = TRUE").Scan(&info.AnzahlZieranpflanzungen)

	// Letztes Backup prüfen
	if files, err := filepath.Glob("backups/kleingarten_backup_*.db"); err == nil && len(files) > 0 {
		// Neueste Backup-Datei finden
		var neuesteDatei string
		var neuesteZeit time.Time
		for _, file := range files {
			if stat, err := os.Stat(file); err == nil {
				if stat.ModTime().After(neuesteZeit) {
					neuesteZeit = stat.ModTime()
					neuesteDatei = file
				}
			}
		}
		if neuesteDatei != "" {
			info.LetztesBackup = &neuesteZeit
		}
	}

	return info
}
