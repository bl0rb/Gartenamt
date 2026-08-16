package models

import (
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"kleingarten-verwaltung/securestore"

	_ "github.com/mattn/go-sqlite3"
)

type BackupManifest struct {
	Version        int    `json:"version"`
	CreatedAt      string `json:"created_at"`
	Algorithm      string `json:"algorithm"`
	PlainSHA256    string `json:"plain_sha256"`
	KeyFingerprint string `json:"key_fingerprint"`
	AppVersion     string `json:"app_version"`
}

type EncryptedBackupFile struct {
	Manifest   BackupManifest `json:"manifest"`
	Ciphertext string         `json:"ciphertext"`
}

type BackupVerificationResult struct {
	Valid          bool
	BackupFile     string
	Checksum       string
	KeyFingerprint string
	Details        string
}

func CreateEncryptedDatabaseBackup() (string, error) {
	if err := os.MkdirAll("backups", 0755); err != nil {
		return "", err
	}

	plainData, err := os.ReadFile(databaseFilePath())
	if err != nil {
		return "", err
	}

	cipherPayload, err := securestore.EncryptBytes(plainData)
	if err != nil {
		return "", err
	}

	fingerprint, err := securestore.KeyFingerprint()
	if err != nil {
		return "", err
	}

	manifest := BackupManifest{
		Version:        1,
		CreatedAt:      time.Now().UTC().Format(time.RFC3339),
		Algorithm:      "AES-256-GCM",
		PlainSHA256:    sha256Hex(plainData),
		KeyFingerprint: fingerprint,
		AppVersion:     readVersionSafe(),
	}

	backupDoc := EncryptedBackupFile{
		Manifest:   manifest,
		Ciphertext: base64.StdEncoding.EncodeToString(cipherPayload),
	}

	content, err := json.MarshalIndent(backupDoc, "", "  ")
	if err != nil {
		return "", err
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	backupFile := fmt.Sprintf("kleingarten_backup_%s.kgbak", timestamp)
	backupPath := filepath.Join("backups", backupFile)

	if err := os.WriteFile(backupPath, content, 0600); err != nil {
		return "", err
	}

	return backupFile, nil
}

func VerifyEncryptedBackup(path string) (*BackupVerificationResult, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	backupDoc := EncryptedBackupFile{}
	if err := json.Unmarshal(content, &backupDoc); err != nil {
		return nil, errors.New("ungueltiges Backup-Format")
	}

	cipherPayload, err := base64.StdEncoding.DecodeString(backupDoc.Ciphertext)
	if err != nil {
		return nil, errors.New("ungueltiger Ciphertext im Backup")
	}

	plainData, err := securestore.DecryptBytes(cipherPayload)
	if err != nil {
		return nil, errors.New("backup konnte mit aktuellem Schluessel nicht entschluesselt werden")
	}

	checksum := sha256Hex(plainData)
	if checksum != backupDoc.Manifest.PlainSHA256 {
		return nil, errors.New("Backup-Pruefsumme stimmt nicht ueberein")
	}

	if err := verifySQLiteIntegrity(plainData); err != nil {
		return nil, err
	}

	return &BackupVerificationResult{
		Valid:          true,
		BackupFile:     filepath.Base(path),
		Checksum:       checksum,
		KeyFingerprint: backupDoc.Manifest.KeyFingerprint,
		Details:        "Backup ist gueltig und Datenbank-Integritaet ist OK",
	}, nil
}

func RestoreEncryptedDatabaseBackup(path string) error {
	verification, err := VerifyEncryptedBackup(path)
	if err != nil {
		recordRestoreHistory(filepath.Base(path), "failed", "verify_failed: "+err.Error(), "")
		return err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	backupDoc := EncryptedBackupFile{}
	if err := json.Unmarshal(content, &backupDoc); err != nil {
		return err
	}

	cipherPayload, err := base64.StdEncoding.DecodeString(backupDoc.Ciphertext)
	if err != nil {
		return err
	}

	plainData, err := securestore.DecryptBytes(cipherPayload)
	if err != nil {
		return err
	}

	// Safety snapshot before replace.
	_, _ = CreateEncryptedDatabaseBackup()

	tmpPath := "kleingarten.restore.tmp"
	if err := os.WriteFile(tmpPath, plainData, 0600); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	if err := verifySQLiteFileIntegrity(tmpPath); err != nil {
		return err
	}

	if DB != nil {
		_ = DB.Close()
	}

	originalPath := databaseFilePath()
	backupOriginalPath := originalPath + ".pre_restore"
	_ = os.Remove(backupOriginalPath)

	// reopenOriginalDB versucht nach einem fehlgeschlagenen Restore die
	// ursprüngliche Datenbankdatei wieder zu öffnen und die package-level
	// DB-Variable neu zu setzen, damit die Anwendung weiterhin funktioniert.
	reopenOriginalDB := func() {
		reopened, reopenErr := sql.Open("sqlite3", originalPath)
		if reopenErr != nil {
			log.Printf("KRITISCH: Datenbank konnte nach fehlgeschlagenem Restore nicht wieder geoeffnet werden: %v", reopenErr)
			return
		}

		if _, reopenErr := reopened.Exec("PRAGMA foreign_keys = ON"); reopenErr != nil {
			log.Printf("KRITISCH: Datenbank konnte nach fehlgeschlagenem Restore nicht initialisiert werden: %v", reopenErr)
			_ = reopened.Close()
			return
		}

		if reopenErr := reopened.Ping(); reopenErr != nil {
			log.Printf("KRITISCH: Datenbank antwortet nach fehlgeschlagenem Restore nicht: %v", reopenErr)
			_ = reopened.Close()
			return
		}

		DB = reopened
	}

	if err := os.Rename(originalPath, backupOriginalPath); err != nil {
		reopenOriginalDB()
		return err
	}

	if err := os.Rename(tmpPath, originalPath); err != nil {
		_ = os.Rename(backupOriginalPath, originalPath)
		reopenOriginalDB()
		return err
	}

	newDB, err := sql.Open("sqlite3", originalPath)
	if err != nil {
		_ = os.Rename(backupOriginalPath, originalPath)
		reopenOriginalDB()
		return err
	}

	if _, err := newDB.Exec("PRAGMA foreign_keys = ON"); err != nil {
		_ = newDB.Close()
		_ = os.Rename(backupOriginalPath, originalPath)
		reopenOriginalDB()
		return err
	}

	if err := newDB.Ping(); err != nil {
		_ = newDB.Close()
		_ = os.Rename(backupOriginalPath, originalPath)
		reopenOriginalDB()
		return err
	}

	DB = newDB
	recordRestoreHistory(filepath.Base(path), "success", verification.Details, verification.Checksum)
	return nil
}

func recordRestoreHistory(fileName, status, details, checksum string) {
	_, _ = DB.Exec(`INSERT INTO restore_history (backup_file, backup_checksum, status, details) VALUES (?, ?, ?, ?)`, fileName, checksum, status, details)
}

func verifySQLiteIntegrity(plainData []byte) error {
	tmpFile, err := os.CreateTemp("", "kgv-backup-verify-*.db")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	if err := os.WriteFile(tmpPath, plainData, 0600); err != nil {
		return err
	}

	return verifySQLiteFileIntegrity(tmpPath)
}

func verifySQLiteFileIntegrity(path string) error {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return err
	}
	defer db.Close()

	var result string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("sqlite integrity_check fehlgeschlagen: %s", result)
	}

	return nil
}

func sha256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func readVersionSafe() string {
	content, err := os.ReadFile("VERSION")
	if err != nil {
		return "unknown"
	}
	value := string(content)
	if len(value) == 0 {
		return "unknown"
	}
	return value
}
