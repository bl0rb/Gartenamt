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
		email TEXT,
		telefon TEXT,
		paechter_strasse TEXT,
		paechter_hausnr TEXT,
		paechter_plz TEXT,
		paechter_ort TEXT,
		paechter_adress TEXT,  -- NEU: Pächter address for invoices
		notizen TEXT,
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

	// 11. NEU: Wasser-Tabelle (Wasserverbrauch pro Parzelle)
	wasserSQL := `
	CREATE TABLE IF NOT EXISTS wasser (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		parzelle_id INTEGER NOT NULL,
		monat INTEGER NOT NULL,
		jahr INTEGER NOT NULL,
		verbrauch REAL NOT NULL,
		kosten REAL NOT NULL,
		noten TEXT,
		erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (parzelle_id) REFERENCES parzellen (id) ON DELETE CASCADE,
		UNIQUE(parzelle_id, monat, jahr)
	);`

	// 12. NEU: Strom-Tabelle (Stromverbrauch pro Parzelle)
	stromSQL := `
	CREATE TABLE IF NOT EXISTS strom (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		parzelle_id INTEGER NOT NULL,
		monat INTEGER NOT NULL,
		jahr INTEGER NOT NULL,
		verbrauch REAL NOT NULL,
		kosten REAL NOT NULL,
		noten TEXT,
		erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (parzelle_id) REFERENCES parzellen (id) ON DELETE CASCADE,
		UNIQUE(parzelle_id, monat, jahr)
	);`

	// 13. NEU: Organization-Settings-Tabelle (Kleingarten details für Rechnungen)
	organizationSettingsSQL := `
	CREATE TABLE IF NOT EXISTS organization_settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		strasse TEXT,
		hausnummer TEXT,
		plz TEXT,
		ort TEXT,
		telefon TEXT,
		email TEXT,
		website TEXT,
		iban TEXT,
		bic TEXT,
		kontoinhaber TEXT,
		smtp_host TEXT,
		smtp_port INTEGER DEFAULT 465,
		smtp_username TEXT,
		smtp_password_enc TEXT,
		smtp_from_address TEXT,
		smtp_from_name TEXT,
		smtp_tls_mode TEXT DEFAULT 'tls',
		imap_host TEXT,
		imap_port INTEGER DEFAULT 993,
		imap_username TEXT,
		imap_password_enc TEXT,
		imap_secure BOOLEAN DEFAULT 1,
		steuernummer TEXT,
		registernummer TEXT,
		logo_path TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	emailLogsSQL := `
	CREATE TABLE IF NOT EXISTS email_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		parzelle_id INTEGER,
		recipient TEXT NOT NULL,
		subject TEXT NOT NULL,
		message TEXT,
		attachment_types TEXT,
		status TEXT NOT NULL,
		error_message TEXT,
		created_by TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (parzelle_id) REFERENCES parzellen (id) ON DELETE SET NULL
	);`

	// Alle Tabellen erstellen
	tables := map[string]string{
		"parzellen":             parzellenSQL,
		"users":                 usersSQL,
		"inspektionen":          inspektionenSQL,
		"maengel":               maengelSQL,
		"wertermittlungen":      wertermittlungenSQL,
		"obstarten":             obstartenSQL,
		"zieranpflanzungen":     zieranpflanzungenSQL,
		"bauindex_tabelle":      bauindexSQL,
		"sessions":              sessionsSQL,
		"audit_log":             auditLogSQL,
		"wasser":                wasserSQL,
		"strom":                 stromSQL,
		"organization_settings": organizationSettingsSQL,
		"email_logs":            emailLogsSQL,
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
		"CREATE INDEX IF NOT EXISTS idx_wasser_parzelle ON wasser(parzelle_id);",
		"CREATE INDEX IF NOT EXISTS idx_wasser_jahr_monat ON wasser(jahr, monat);",
		"CREATE INDEX IF NOT EXISTS idx_strom_parzelle ON strom(parzelle_id);",
		"CREATE INDEX IF NOT EXISTS idx_strom_jahr_monat ON strom(jahr, monat);",
		"CREATE INDEX IF NOT EXISTS idx_email_logs_parzelle ON email_logs(parzelle_id);",
		"CREATE INDEX IF NOT EXISTS idx_email_logs_created_at ON email_logs(created_at);",
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

	// Datenbank-Migrationen
	if err := runMigrations(); err != nil {
		log.Printf("⚠️  Migration-Fehler: %v", err)
	}

	log.Println("🌱 Alle Tabellen und Indizes erfolgreich erstellt")
	return nil
}

// runMigrations handles database schema upgrades for existing installations
func runMigrations() error {
	// Add invoice address columns if they don't exist
	rows, err := DB.Query("PRAGMA table_info(parzellen)")
	if err != nil {
		return err
	}
	defer rows.Close()

	existingColumns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var typeStr string
		var notnull int
		var dflt_value interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dflt_value, &pk); err == nil {
			existingColumns[name] = true
		}
	}

	if !existingColumns["paechter_adress"] {
		_, err := DB.Exec("ALTER TABLE parzellen ADD COLUMN paechter_adress TEXT")
		if err != nil {
			log.Printf("⚠️  Could not add paechter_adress column: %v", err)
		} else {
			log.Println("✅ Added paechter_adress column to parzellen table")
		}
	}

	addressColumns := []struct {
		name string
		sql  string
	}{
		{name: "paechter_strasse", sql: "ALTER TABLE parzellen ADD COLUMN paechter_strasse TEXT"},
		{name: "paechter_hausnr", sql: "ALTER TABLE parzellen ADD COLUMN paechter_hausnr TEXT"},
		{name: "paechter_plz", sql: "ALTER TABLE parzellen ADD COLUMN paechter_plz TEXT"},
		{name: "paechter_ort", sql: "ALTER TABLE parzellen ADD COLUMN paechter_ort TEXT"},
	}

	for _, column := range addressColumns {
		if existingColumns[column.name] {
			continue
		}

		if _, err := DB.Exec(column.sql); err != nil {
			log.Printf("⚠️  Could not add %s column: %v", column.name, err)
		} else {
			log.Printf("✅ Added %s column to parzellen table", column.name)
		}
	}

	organizationColumns := []struct {
		name string
		sql  string
	}{
		{name: "smtp_host", sql: "ALTER TABLE organization_settings ADD COLUMN smtp_host TEXT"},
		{name: "smtp_port", sql: "ALTER TABLE organization_settings ADD COLUMN smtp_port INTEGER DEFAULT 465"},
		{name: "smtp_username", sql: "ALTER TABLE organization_settings ADD COLUMN smtp_username TEXT"},
		{name: "smtp_password_enc", sql: "ALTER TABLE organization_settings ADD COLUMN smtp_password_enc TEXT"},
		{name: "smtp_from_address", sql: "ALTER TABLE organization_settings ADD COLUMN smtp_from_address TEXT"},
		{name: "smtp_from_name", sql: "ALTER TABLE organization_settings ADD COLUMN smtp_from_name TEXT"},
		{name: "smtp_tls_mode", sql: "ALTER TABLE organization_settings ADD COLUMN smtp_tls_mode TEXT DEFAULT 'tls'"},
		{name: "imap_host", sql: "ALTER TABLE organization_settings ADD COLUMN imap_host TEXT"},
		{name: "imap_port", sql: "ALTER TABLE organization_settings ADD COLUMN imap_port INTEGER DEFAULT 993"},
		{name: "imap_username", sql: "ALTER TABLE organization_settings ADD COLUMN imap_username TEXT"},
		{name: "imap_password_enc", sql: "ALTER TABLE organization_settings ADD COLUMN imap_password_enc TEXT"},
		{name: "imap_secure", sql: "ALTER TABLE organization_settings ADD COLUMN imap_secure BOOLEAN DEFAULT 1"},
	}

	rows, err = DB.Query("PRAGMA table_info(organization_settings)")
	if err != nil {
		return err
	}
	defer rows.Close()

	existingOrganizationColumns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var typeStr string
		var notnull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dfltValue, &pk); err == nil {
			existingOrganizationColumns[name] = true
		}
	}

	for _, column := range organizationColumns {
		if existingOrganizationColumns[column.name] {
			continue
		}

		if _, err := DB.Exec(column.sql); err != nil {
			log.Printf("⚠️  Could not add %s column: %v", column.name, err)
		} else {
			log.Printf("✅ Added %s column to organization_settings table", column.name)
		}
	}

	rows, err = DB.Query("PRAGMA table_info(email_logs)")
	if err != nil {
		return err
	}
	defer rows.Close()

	existingEmailLogColumns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name string
		var typeStr string
		var notnull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dfltValue, &pk); err == nil {
			existingEmailLogColumns[name] = true
		}
	}

	if !existingEmailLogColumns["message"] {
		if _, err := DB.Exec("ALTER TABLE email_logs ADD COLUMN message TEXT"); err != nil {
			log.Printf("⚠️  Could not add message column to email_logs: %v", err)
		} else {
			log.Println("✅ Added message column to email_logs table")
		}
	}

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
