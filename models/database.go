package models

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(dbPath string) (*sql.DB, error) {
	var err error
	DB, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Verbindung testen
	if err := DB.Ping(); err != nil {
		return nil, err
	}

	// Tabellen erstellen
	if err := createTables(); err != nil {
		return nil, err
	}

	log.Println("📊 Datenbank erfolgreich initialisiert")
	return DB, nil
}

func createTables() error {
	// BESTEHENDE TABELLEN (erweitert mit neuen Feldern)

	// 1. Parzellen-Tabelle (MIT NEUEN FELDERN)
	parzellenSQL := `
	CREATE TABLE IF NOT EXISTS parzellen (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		nummer TEXT NOT NULL UNIQUE,
		groesse REAL NOT NULL,
		verein TEXT NOT NULL,
		paechter_name TEXT,
		email TEXT,           -- NEU
		telefon TEXT,         -- NEU
		notizen TEXT,         -- NEU
		kuendigung_datum DATETIME,
		erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// 2. NEU: Users-Tabelle für Authentifizierung
	usersSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		email TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		active BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_login DATETIME
	);`

	// 3. Inspektionen-Tabelle
	inspektionenSQL := `
	CREATE TABLE IF NOT EXISTS inspektionen (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		parzelle_id INTEGER NOT NULL,
		datum DATE NOT NULL,
		maengel TEXT, -- JSON storage for maengel
		auflagen_erfuellt BOOLEAN NOT NULL DEFAULT 0,
		frist DATE,
		weitere_auflagen TEXT,
		keine_toiletten BOOLEAN NOT NULL DEFAULT 0,
		erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (parzelle_id) REFERENCES parzellen (id) ON DELETE CASCADE
	);`

	// 4. Mängel-Tabelle (für Inspektionen)
	maengelSQL := `
	CREATE TABLE IF NOT EXISTS maengel (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		inspektion_id INTEGER NOT NULL,
		nr INTEGER NOT NULL,
		beschreibung TEXT NOT NULL,
		rechtsgrundlage TEXT NOT NULL,
		erfuellt BOOLEAN NOT NULL DEFAULT 0,
		FOREIGN KEY (inspektion_id) REFERENCES inspektionen (id) ON DELETE CASCADE
	);`

	// 5. Wertermittlungen-Tabelle
	wertermittlungenSQL := `
	CREATE TABLE IF NOT EXISTS wertermittlungen (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		parzelle_id INTEGER NOT NULL,
		inspektion_id INTEGER,
		datum DATE NOT NULL,
		laube_wert REAL NOT NULL DEFAULT 0,
		baulichkeiten_wert REAL NOT NULL DEFAULT 0,
		obst_wert REAL NOT NULL DEFAULT 0,
		gemuese_wert REAL NOT NULL DEFAULT 0,
		zier_wert REAL NOT NULL DEFAULT 0,
		gesamt_wert REAL NOT NULL DEFAULT 0,
		details TEXT, -- JSON für komplexe Daten
		erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (parzelle_id) REFERENCES parzellen (id) ON DELETE CASCADE,
		FOREIGN KEY (inspektion_id) REFERENCES inspektionen (id) ON DELETE SET NULL
	);`

	// 6. Obstarten-Tabelle
	obstartenSQL := `
	CREATE TABLE IF NOT EXISTS obstarten (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		kategorie TEXT NOT NULL,
		einheit TEXT NOT NULL,
		standardpreis REAL NOT NULL,
		max_anzahl INTEGER DEFAULT 999,
		beschreibung TEXT,
		aktiv BOOLEAN NOT NULL DEFAULT 1,
		erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// 7. Zieranpflanzungen-Tabelle
	zieranpflanzungenSQL := `
	CREATE TABLE IF NOT EXISTS zieranpflanzungen (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		kategorie TEXT NOT NULL,
		preis_pro_qm REAL NOT NULL,
		beschreibung TEXT,
		max_flaeche INTEGER,
		aktiv BOOLEAN NOT NULL DEFAULT 1,
		erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// 8. Bauindex-Tabelle
	bauindexSQL := `
	CREATE TABLE IF NOT EXISTS bauindex_tabelle (
		jahr INTEGER PRIMARY KEY,
		bauindex REAL NOT NULL
	);`

	// 9. NEU: Sessions-Tabelle (Optional - für persistente Sessions)
	sessionsSQL := `
	CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
		ip_address TEXT,
		user_agent TEXT,
		FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
	);`

	// 10. NEU: Audit-Log-Tabelle
	auditLogSQL := `
	CREATE TABLE IF NOT EXISTS audit_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		username TEXT,
		action TEXT NOT NULL,
		table_name TEXT,
		record_id INTEGER,
		old_values TEXT, -- JSON
		new_values TEXT, -- JSON
		ip_address TEXT,
		user_agent TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL
	);`

	// Alle Tabellen erstellen
	tables := map[string]string{
		"parzellen":         parzellenSQL,
		"users":             usersSQL, // NEU
		"inspektionen":      inspektionenSQL,
		"maengel":           maengelSQL,
		"wertermittlungen":  wertermittlungenSQL,
		"obstarten":         obstartenSQL,
		"zieranpflanzungen": zieranpflanzungenSQL,
		"bauindex_tabelle":  bauindexSQL,
		"sessions":          sessionsSQL, // NEU
		"audit_log":         auditLogSQL, // NEU
	}

	for tableName, tableSQL := range tables {
		if _, err := DB.Exec(tableSQL); err != nil {
			return err
		}
		log.Printf("✅ Tabelle '%s' erstellt/validiert", tableName)
	}

	// Indizes erstellen für bessere Performance
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_parzellen_nummer ON parzellen(nummer);",
		"CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);",
		"CREATE INDEX IF NOT EXISTS idx_inspektionen_parzelle ON inspektionen(parzelle_id);",
		"CREATE INDEX IF NOT EXISTS idx_wertermittlungen_parzelle ON wertermittlungen(parzelle_id);",
		"CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_id);",
		"CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);",
	}

	for _, indexSQL := range indexes {
		if _, err := DB.Exec(indexSQL); err != nil {
			log.Printf("⚠️  Index-Erstellung übersprungen: %v", err)
		}
	}

	// Standard-Daten einfügen
	if err := insertDefaultData(); err != nil {
		return err
	}

	log.Println("🌱 Alle Tabellen und Indizes erfolgreich erstellt")
	return nil
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
			{2023, 41.8}, {2024, 42.4}, {2025, 42.9},
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
