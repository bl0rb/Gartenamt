package models

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

// dbFilePath merkt sich den Pfad der geöffneten Datenbank (für Backup/Restore).
var dbFilePath = "kleingarten.db"

// databaseFilePath liefert den Pfad der aktuell verwendeten Datenbankdatei.
func databaseFilePath() string {
	return dbFilePath
}

// sqliteDSN aktiviert Foreign-Key-Constraints auf jeder Pool-Verbindung
// (ein einmaliges "PRAGMA foreign_keys" gilt in SQLite nur pro Verbindung).
func sqliteDSN(path string) string {
	return path + "?_foreign_keys=on"
}

func InitDB(dbPath string) (*sql.DB, error) {
	var err error
	DB, err = sql.Open("sqlite3", sqliteDSN(dbPath))
	if err != nil {
		return nil, err
	}
	dbFilePath = dbPath

	// Verbindung testen
	if err := DB.Ping(); err != nil {
		return nil, err
	}

	// Schema-Migrationen anwenden (bricht bei Fehlern ab)
	if err := applyPendingMigrations(); err != nil {
		return nil, err
	}

	// Stammdaten einfügen, falls noch keine vorhanden sind
	if err := insertDefaultData(); err != nil {
		return nil, err
	}

	log.Printf("📊 Datenbank erfolgreich initialisiert (Schema-Version %d)", SchemaVersion())
	return DB, nil
}

func insertDefaultData() error {
	// 1. Bauindex-Daten einfügen (falls nicht vorhanden)
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM bauindex_tabelle").Scan(&count)
	if count == 0 {
		bauindexData := []struct {
			Jahr     int
			Bauindex float64
		}{
			{2020, 39.8}, {2021, 40.5}, {2022, 41.2},
			{2023, 41.8}, {2024, 42.4}, {2025, 42.9}, {2026, 44.3},
		}

		for _, data := range bauindexData {
			_, err := DB.Exec("INSERT INTO bauindex_tabelle (jahr, bauindex) VALUES (?, ?)",
				data.Jahr, data.Bauindex)
			if err != nil {
				return err
			}
		}
		log.Println("📊 Bauindex-Daten eingefügt")
	}

	// 2. Standard-Obstarten einfügen (falls nicht vorhanden)
	DB.QueryRow("SELECT COUNT(*) FROM obstarten").Scan(&count)
	if count == 0 {
		standardObstarten := []struct {
			Name, Kategorie, Einheit, Beschreibung string
			Preis                                  float64
			MaxAnzahl                              int
		}{
			{"Apfel", "E1", "Stück", "Standardobstbaum", 25.00, 999},
			{"Birne", "E1", "Stück", "Standardobstbaum", 25.00, 999},
			{"Kirsche süß", "E1", "Stück", "Süßkirsche", 30.00, 999},
			{"Kirsche sauer", "E1", "Stück", "Sauerkirsche", 25.00, 999},
			{"Pflaume", "E1", "Stück", "Pflaumenbaum", 25.00, 999},
			{"Johannisbeere", "E2", "Stück", "Beerenstrauch", 8.00, 999},
			{"Stachelbeere", "E2", "Stück", "Beerenstrauch", 8.00, 999},
			{"Himbeere", "E10", "lfm", "Himbeeren", 6.00, 999},
			{"Erdbeere", "E8", "m²", "Erdbeerpflanzen", 5.00, 50},
			{"Rhabarber", "E7", "Stück", "Rhabarberpflanze", 5.00, 10},
		}

		for _, obstart := range standardObstarten {
			_, err := DB.Exec(`INSERT INTO obstarten (name, kategorie, einheit, standardpreis, max_anzahl, beschreibung) 
				VALUES (?, ?, ?, ?, ?, ?)`,
				obstart.Name, obstart.Kategorie, obstart.Einheit, obstart.Preis, obstart.MaxAnzahl, obstart.Beschreibung)
			if err != nil {
				return err
			}
		}
		log.Println("🍎 Standard-Obstarten eingefügt")
	}

	// 3. Standard-Zieranpflanzungen einfügen (falls nicht vorhanden)
	DB.QueryRow("SELECT COUNT(*) FROM zieranpflanzungen").Scan(&count)
	if count == 0 {
		standardZieranpflanzungen := []struct {
			Name, Kategorie, Beschreibung string
			PreisProQM                    float64
			MaxFlaeche                    *int
		}{
			{"Ziersträucher einzeln", "F1", "Einzelne Ziersträucher", 4.00, nil},
			{"Gehölze + Stauden", "F2", "Gemischte Gehölz-Stauden-Bepflanzung", 5.00, nil},
			{"Gemischte Bepflanzung", "F3", "Vielfältige Bepflanzung", 6.00, nil},
			{"Ökologisch wertvoll", "F4", "Naturnahe, ökologisch wertvolle Bepflanzung", 12.00, nil},
			{"Zier-/Fischteich", "F5", "Künstlich angelegter Gartenteich", 5.00, intPtr(15)},
			{"Naturnaher Teich", "F6", "Naturnaher Gartenteich", 7.00, intPtr(15)},
			{"Naturteich mit Uferzone", "F7", "Naturteich mit naturnaher Ufergestaltung", 15.00, intPtr(15)},
			{"Rasenflächen", "F8", "Gepflegte Rasenflächen", 0.50, nil},
		}

		for _, zier := range standardZieranpflanzungen {
			_, err := DB.Exec(`INSERT INTO zieranpflanzungen (name, kategorie, preis_pro_qm, beschreibung, max_flaeche) 
				VALUES (?, ?, ?, ?, ?)`,
				zier.Name, zier.Kategorie, zier.PreisProQM, zier.Beschreibung, zier.MaxFlaeche)
			if err != nil {
				return err
			}
		}
		log.Println("🌱 Standard-Zieranpflanzungen eingefügt")
	}

	return nil
}

// Hilfsfunktion für optionale int-Werte
func intPtr(i int) *int {
	return &i
}

// Cleanup-Funktionen
func CleanupExpiredSessions() error {
	// Sessions älter als 7 Tage löschen
	query := `DELETE FROM sessions WHERE last_seen < datetime('now', '-7 days')`
	_, err := DB.Exec(query)
	return err
}

// Audit-Log Funktionen
func LogAuditEvent(userID int, username, action, tableName string, recordID int, oldValues, newValues, ipAddress, userAgent string) error {
	query := `INSERT INTO audit_log 
		(user_id, username, action, table_name, record_id, old_values, new_values, ip_address, user_agent) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := DB.Exec(query, userID, username, action, tableName, recordID, oldValues, newValues, ipAddress, userAgent)
	return err
}
