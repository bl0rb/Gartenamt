package models

import (
	"database/sql"
	"fmt"
	"log"
)

// Versioniertes Migrationssystem:
//
// Jede Schemaänderung wird als neue migration mit fortlaufender Version an die
// Liste `migrations` angehängt. Angewendete Versionen werden in der Tabelle
// schema_migrations festgehalten; beim Start werden ausstehende Migrationen
// in einer Transaktion pro Migration ausgeführt. Schlägt eine Migration fehl,
// wird sie zurückgerollt und der Start abgebrochen - die App läuft nie mit
// einem halb migrierten Schema.
//
// Regeln für neue Migrationen:
//   - Version = höchste bestehende Version + 1, niemals bestehende ändern
//   - Nur vorwärts: eine einmal veröffentlichte Migration wird nicht mehr
//     angepasst, Korrekturen erfolgen als neue Migration
//   - Idempotenz ist nicht nötig (der Runner führt jede Version genau einmal
//     aus), schadet aber nicht

type migration struct {
	Version int
	Name    string
	Apply   func(tx *sql.Tx) error
}

var migrations = []migration{
	{Version: 1, Name: "baseline_schema", Apply: migrateBaselineSchema},
	{Version: 2, Name: "legacy_column_upgrades", Apply: migrateLegacyColumns},
	{Version: 3, Name: "users_must_change_password", Apply: migrateMustChangePassword},
	{Version: 4, Name: "audit_log_description", Apply: migrateAuditLogDescription},
}

// applyPendingMigrations führt alle noch nicht angewendeten Migrationen aus.
func applyPendingMigrations() error {
	if _, err := DB.Exec(`
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		applied_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);`); err != nil {
		return fmt.Errorf("schema_migrations-Tabelle konnte nicht erstellt werden: %w", err)
	}

	// Sanity-Check der Migrationsliste (Programmierfehler früh erkennen)
	for i, m := range migrations {
		if m.Version != i+1 {
			return fmt.Errorf("migrationsliste inkonsistent: position %d hat Version %d (erwartet %d)", i, m.Version, i+1)
		}
	}

	var current int
	if err := DB.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&current); err != nil {
		return fmt.Errorf("migrationsstand konnte nicht gelesen werden: %w", err)
	}

	if current > len(migrations) {
		return fmt.Errorf("datenbank hat Schema-Version %d, diese Programmversion kennt nur bis %d - bitte Anwendung aktualisieren", current, len(migrations))
	}

	for _, m := range migrations {
		if m.Version <= current {
			continue
		}

		tx, err := DB.Begin()
		if err != nil {
			return fmt.Errorf("migration %d (%s): Transaktion konnte nicht gestartet werden: %w", m.Version, m.Name, err)
		}

		if err := m.Apply(tx); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d (%s) fehlgeschlagen: %w", m.Version, m.Name, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations (version, name) VALUES (?, ?)`, m.Version, m.Name); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %d (%s) konnte nicht registriert werden: %w", m.Version, m.Name, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("migration %d (%s): Commit fehlgeschlagen: %w", m.Version, m.Name, err)
		}

		log.Printf("✅ Migration %d angewendet: %s", m.Version, m.Name)
	}

	return nil
}

// SchemaVersion liefert die aktuell angewendete Schema-Version (für System-Info).
func SchemaVersion() int {
	var v int
	if err := DB.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&v); err != nil {
		return 0
	}
	return v
}

// migrateBaselineSchema erstellt das vollständige Ausgangsschema. Alle
// Statements sind idempotent (IF NOT EXISTS), damit Installationen aus der
// Zeit vor dem Migrationssystem verlustfrei auf Version 1 konvergieren.
func migrateBaselineSchema(tx *sql.Tx) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS parzellen (
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
			paechter_adress TEXT,
			notizen TEXT,
			kuendigung_datum DATETIME,
			erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			email TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			active BOOLEAN NOT NULL DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_login DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS inspektionen (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			parzelle_id INTEGER NOT NULL,
			datum DATE NOT NULL,
			maengel TEXT,
			auflagen_erfuellt BOOLEAN NOT NULL DEFAULT 0,
			frist DATE,
			weitere_auflagen TEXT,
			keine_toiletten BOOLEAN NOT NULL DEFAULT 0,
			erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (parzelle_id) REFERENCES parzellen (id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS maengel (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			inspektion_id INTEGER NOT NULL,
			nr INTEGER NOT NULL,
			beschreibung TEXT NOT NULL,
			rechtsgrundlage TEXT NOT NULL,
			erfuellt BOOLEAN NOT NULL DEFAULT 0,
			FOREIGN KEY (inspektion_id) REFERENCES inspektionen (id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS wertermittlungen (
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
			details TEXT,
			erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (parzelle_id) REFERENCES parzellen (id) ON DELETE CASCADE,
			FOREIGN KEY (inspektion_id) REFERENCES inspektionen (id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS obstarten (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			kategorie TEXT NOT NULL,
			einheit TEXT NOT NULL,
			standardpreis REAL NOT NULL,
			max_anzahl INTEGER DEFAULT 999,
			beschreibung TEXT,
			aktiv BOOLEAN NOT NULL DEFAULT 1,
			erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS zieranpflanzungen (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			kategorie TEXT NOT NULL,
			preis_pro_qm REAL NOT NULL,
			beschreibung TEXT,
			max_flaeche INTEGER,
			aktiv BOOLEAN NOT NULL DEFAULT 1,
			erstellt_am DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS bauindex_tabelle (
			jahr INTEGER PRIMARY KEY,
			bauindex REAL NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			ip_address TEXT,
			user_agent TEXT,
			FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS audit_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			username TEXT,
			action TEXT NOT NULL,
			description TEXT,
			table_name TEXT,
			record_id INTEGER,
			old_values TEXT,
			new_values TEXT,
			ip_address TEXT,
			user_agent TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE SET NULL
		)`,
		`CREATE TABLE IF NOT EXISTS wasser (
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
		)`,
		`CREATE TABLE IF NOT EXISTS strom (
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
		)`,
		`CREATE TABLE IF NOT EXISTS organization_settings (
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
		)`,
		`CREATE TABLE IF NOT EXISTS email_logs (
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
		)`,
		`CREATE TABLE IF NOT EXISTS app_security_state (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			key_reveal_count INTEGER NOT NULL DEFAULT 0,
			key_acknowledged BOOLEAN NOT NULL DEFAULT 0,
			key_last_reveal_at DATETIME,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS restore_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			backup_file TEXT NOT NULL,
			backup_checksum TEXT,
			status TEXT NOT NULL,
			details TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_parzellen_nummer ON parzellen(nummer)`,
		`CREATE INDEX IF NOT EXISTS idx_users_username ON users(username)`,
		`CREATE INDEX IF NOT EXISTS idx_inspektionen_parzelle ON inspektionen(parzelle_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wertermittlungen_parzelle ON wertermittlungen(parzelle_id)`,
		`CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_user ON audit_log(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp)`,
		`CREATE INDEX IF NOT EXISTS idx_wasser_parzelle ON wasser(parzelle_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wasser_jahr_monat ON wasser(jahr, monat)`,
		`CREATE INDEX IF NOT EXISTS idx_strom_parzelle ON strom(parzelle_id)`,
		`CREATE INDEX IF NOT EXISTS idx_strom_jahr_monat ON strom(jahr, monat)`,
		`CREATE INDEX IF NOT EXISTS idx_email_logs_parzelle ON email_logs(parzelle_id)`,
		`CREATE INDEX IF NOT EXISTS idx_email_logs_created_at ON email_logs(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_restore_history_created_at ON restore_history(created_at)`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("statement fehlgeschlagen (%.60s...): %w", stmt, err)
		}
	}
	return nil
}

// migrateLegacyColumns ergänzt Spalten, die in sehr alten Installationen
// (vor dem Migrationssystem) fehlen können. Auf frischen Datenbanken sind
// alle Spalten bereits im Baseline-Schema enthalten und dies ist ein No-Op.
func migrateLegacyColumns(tx *sql.Tx) error {
	legacyColumns := []struct {
		table  string
		column string
		ddl    string
	}{
		{"parzellen", "paechter_strasse", "ALTER TABLE parzellen ADD COLUMN paechter_strasse TEXT"},
		{"parzellen", "paechter_hausnr", "ALTER TABLE parzellen ADD COLUMN paechter_hausnr TEXT"},
		{"parzellen", "paechter_plz", "ALTER TABLE parzellen ADD COLUMN paechter_plz TEXT"},
		{"parzellen", "paechter_ort", "ALTER TABLE parzellen ADD COLUMN paechter_ort TEXT"},
		{"parzellen", "paechter_adress", "ALTER TABLE parzellen ADD COLUMN paechter_adress TEXT"},
		{"organization_settings", "smtp_host", "ALTER TABLE organization_settings ADD COLUMN smtp_host TEXT"},
		{"organization_settings", "smtp_port", "ALTER TABLE organization_settings ADD COLUMN smtp_port INTEGER DEFAULT 465"},
		{"organization_settings", "smtp_username", "ALTER TABLE organization_settings ADD COLUMN smtp_username TEXT"},
		{"organization_settings", "smtp_password_enc", "ALTER TABLE organization_settings ADD COLUMN smtp_password_enc TEXT"},
		{"organization_settings", "smtp_from_address", "ALTER TABLE organization_settings ADD COLUMN smtp_from_address TEXT"},
		{"organization_settings", "smtp_from_name", "ALTER TABLE organization_settings ADD COLUMN smtp_from_name TEXT"},
		{"organization_settings", "smtp_tls_mode", "ALTER TABLE organization_settings ADD COLUMN smtp_tls_mode TEXT DEFAULT 'tls'"},
		{"organization_settings", "imap_host", "ALTER TABLE organization_settings ADD COLUMN imap_host TEXT"},
		{"organization_settings", "imap_port", "ALTER TABLE organization_settings ADD COLUMN imap_port INTEGER DEFAULT 993"},
		{"organization_settings", "imap_username", "ALTER TABLE organization_settings ADD COLUMN imap_username TEXT"},
		{"organization_settings", "imap_password_enc", "ALTER TABLE organization_settings ADD COLUMN imap_password_enc TEXT"},
		{"organization_settings", "imap_secure", "ALTER TABLE organization_settings ADD COLUMN imap_secure BOOLEAN DEFAULT 1"},
		{"email_logs", "message", "ALTER TABLE email_logs ADD COLUMN message TEXT"},
		{"app_security_state", "key_reveal_count", "ALTER TABLE app_security_state ADD COLUMN key_reveal_count INTEGER NOT NULL DEFAULT 0"},
		{"app_security_state", "key_acknowledged", "ALTER TABLE app_security_state ADD COLUMN key_acknowledged BOOLEAN NOT NULL DEFAULT 0"},
		{"app_security_state", "key_last_reveal_at", "ALTER TABLE app_security_state ADD COLUMN key_last_reveal_at DATETIME"},
		{"app_security_state", "updated_at", "ALTER TABLE app_security_state ADD COLUMN updated_at DATETIME DEFAULT CURRENT_TIMESTAMP"},
		{"app_security_state", "created_at", "ALTER TABLE app_security_state ADD COLUMN created_at DATETIME DEFAULT CURRENT_TIMESTAMP"},
	}

	for _, lc := range legacyColumns {
		exists, err := columnExists(tx, lc.table, lc.column)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		if _, err := tx.Exec(lc.ddl); err != nil {
			return fmt.Errorf("spalte %s.%s konnte nicht ergänzt werden: %w", lc.table, lc.column, err)
		}
		log.Printf("✅ Spalte %s.%s ergänzt", lc.table, lc.column)
	}
	return nil
}

// migrateMustChangePassword ergänzt das Flag für die erzwungene Passwortänderung
// nach dem ersten Login (gesetzt für den automatisch erzeugten Standard-Admin).
func migrateMustChangePassword(tx *sql.Tx) error {
	exists, err := columnExists(tx, "users", "must_change_password")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	if _, err := tx.Exec(`ALTER TABLE users ADD COLUMN must_change_password BOOLEAN NOT NULL DEFAULT 0`); err != nil {
		return fmt.Errorf("spalte users.must_change_password konnte nicht ergänzt werden: %w", err)
	}
	return nil
}

// columnExists prüft per PRAGMA table_info, ob eine Spalte existiert.
// migrateAuditLogDescription ergänzt die Freitext-Spalte des Audit-Logs.
// Der Go-Code schrieb bisher deutsche Spaltennamen (aktion, beschreibung, ...),
// die Tabelle trägt aber englische - dadurch schlug jeder INSERT fehl und das
// Audit-Log blieb leer. Der Schreib-/Lesepfad ist jetzt auf das tatsächliche
// Schema umgestellt; dort fehlte nur eine Spalte für die Beschreibung.
func migrateAuditLogDescription(tx *sql.Tx) error {
	exists, err := columnExists(tx, "audit_log", "description")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	_, err = tx.Exec("ALTER TABLE audit_log ADD COLUMN description TEXT")
	return err
}

func columnExists(tx *sql.Tx, table, column string) (bool, error) {
	rows, err := tx.Query(fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, typeStr string
		var notnull, pk int
		var dfltValue interface{}
		if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
