package services

import (
	"encoding/csv"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bl0rb/gartenamt/models"
)

type CSVService struct {
	exportDir string
}

type CSVImportReport struct {
	Table        string
	TotalRows    int
	ImportedRows int
	SkippedRows  int
	ErrorRows    int
	Errors       []string
}

type CSVHealthReport struct {
	ForeignKeyOK  bool
	ForeignKeyMsg string
	Counts        map[string]int
}

// writeCSVRecord schreibt eine Datenzeile und entschaerft dabei jedes Feld.
func writeCSVRecord(writer *csv.Writer, record []string) {
	sanitized := make([]string, len(record))
	for i, value := range record {
		sanitized[i] = sanitizeCSVValue(value)
	}
	_ = writer.Write(sanitized)
}

// sanitizeCSVValue verhindert Formel-Injektion: Excel wertet Felder, die mit
// "=", "+", "-", "@" oder einem Steuerzeichen beginnen, beim Oeffnen als
// Formel aus. Ein vorangestelltes Apostroph macht daraus wieder Text. Die
// Exporte sind ausdruecklich fuer Excel gedacht (BOM, Semikolon als Trenner),
// deshalb ist genau dieser Weg vorgezeichnet.
func sanitizeCSVValue(value string) string {
	if value == "" {
		return value
	}

	switch value[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + value
	}

	return value
}

func NewCSVService() *CSVService {
	exportDir := "exports"
	os.MkdirAll(exportDir, 0755)
	return &CSVService{exportDir: exportDir}
}

// Export-Funktionen

func (s *CSVService) ExportAllData() (string, error) {
	// Alle CSV-Dateien erstellen
	exports := map[string]func() (string, error){
		"parzellen":         s.ExportParzellen,
		"wertermittlungen":  s.ExportWertermittlungen,
		"obstarten":         s.ExportObstarten,
		"zieranpflanzungen": s.ExportZieranpflanzungen,
		"bauindex":          s.ExportBauindex,
	}

	var exportedFiles []string
	for name, exportFunc := range exports {
		fileName, err := exportFunc()
		if err != nil {
			log.Printf("Fehler beim Export von %s: %v", name, err)
			continue
		}
		exportedFiles = append(exportedFiles, fileName)
	}

	// Erste verfügbare Datei zurückgeben (vereinfacht)
	if len(exportedFiles) > 0 {
		return exportedFiles[0], nil
	}

	return "", fmt.Errorf("keine Daten exportiert")
}

func (s *CSVService) ExportParzellen() (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	fileName := fmt.Sprintf("parzellen_%s.csv", timestamp)
	filePath := filepath.Join(s.exportDir, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// UTF-8 BOM für Excel-Kompatibilität
	file.WriteString("\uFEFF")

	writer := csv.NewWriter(file)
	writer.Comma = ';' // Semikolon für deutsche Excel-Version
	defer writer.Flush()

	// Header schreiben - ERWEITERT
	headers := []string{
		"ID", "Nummer", "Groesse", "Verein", "Paechter_Name",
		"Email", "Telefon", "Notizen", // NEU
		"Kuendigung_Datum", "Erstellt_Am",
	}
	writer.Write(headers)

	// Daten laden und schreiben - ERWEITERT
	query := `SELECT id, nummer, groesse, verein, paechter_name, email, telefon, notizen, kuendigung_datum, erstellt_am FROM parzellen ORDER BY nummer`
	rows, err := models.DB.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var nummer, verein, paechterName, email, telefon, notizen string
		var groesse float64
		var kuendigungDatum, erstelltAm *time.Time

		err := rows.Scan(&id, &nummer, &groesse, &verein, &paechterName,
			&email, &telefon, &notizen, &kuendigungDatum, &erstelltAm)
		if err != nil {
			continue
		}

		record := []string{
			strconv.Itoa(id),
			nummer,
			fmt.Sprintf("%.1f", groesse),
			verein,
			paechterName,
			email,   // NEU
			telefon, // NEU
			notizen, // NEU
			formatTimeForCSV(kuendigungDatum),
			formatTimeForCSV(erstelltAm),
		}
		writeCSVRecord(writer, record)
	}

	log.Printf("Parzellen exportiert: %s", fileName)
	return fileName, nil
}

func (s *CSVService) ExportWertermittlungen() (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	fileName := fmt.Sprintf("wertermittlungen_%s.csv", timestamp)
	filePath := filepath.Join(s.exportDir, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// UTF-8 BOM für Excel-Kompatibilität
	file.WriteString("\uFEFF")

	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	// Header schreiben
	headers := []string{
		"ID", "Parzelle_Nummer", "Datum", "Laube_Wert", "Baulichkeiten_Wert",
		"Obst_Wert", "Gemuese_Wert", "Zier_Wert", "Gesamt_Wert",
		"Manuell_Laube", "Begruendung_Laube", "Erstellt_Am",
	}
	writer.Write(headers)

	// Daten mit JOIN laden
	query := `
		SELECT w.id, p.nummer, w.datum, w.laube_wert, w.baulichkeiten_wert,
		       w.obst_wert, w.gemuese_wert, w.zier_wert, w.gesamt_wert, 
		       w.details, w.erstellt_am
		FROM wertermittlungen w
		JOIN parzellen p ON w.parzelle_id = p.id
		ORDER BY w.datum DESC`

	rows, err := models.DB.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var parzelleNummer string
		var datum, erstelltAm time.Time
		var laubeWert, baulichkeitenWert, obstWert, gemuseWert, zierWert, gesamtWert float64
		var detailsJSON string

		err := rows.Scan(&id, &parzelleNummer, &datum, &laubeWert, &baulichkeitenWert,
			&obstWert, &gemuseWert, &zierWert, &gesamtWert, &detailsJSON, &erstelltAm)
		if err != nil {
			continue
		}

		// Details parsen für manuelle Laube-Info
		manuellLaube := "Nein"
		begruendung := ""

		// Vereinfachte Prüfung für manuelle Eingabe
		if strings.Contains(detailsJSON, `"manuell_eingegeben":true`) {
			manuellLaube = "Ja"
		}

		record := []string{
			strconv.Itoa(id),
			parzelleNummer,
			datum.Format("2006-01-02"),
			fmt.Sprintf("%.2f", laubeWert),
			fmt.Sprintf("%.2f", baulichkeitenWert),
			fmt.Sprintf("%.2f", obstWert),
			fmt.Sprintf("%.2f", gemuseWert),
			fmt.Sprintf("%.2f", zierWert),
			fmt.Sprintf("%.2f", gesamtWert),
			manuellLaube,
			begruendung,
			erstelltAm.Format("2006-01-02 15:04:05"),
		}
		writeCSVRecord(writer, record)
	}

	log.Printf("Wertermittlungen exportiert: %s", fileName)
	return fileName, nil
}

func (s *CSVService) ExportObstarten() (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	fileName := fmt.Sprintf("obstarten_%s.csv", timestamp)
	filePath := filepath.Join(s.exportDir, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	file.WriteString("\uFEFF")
	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	headers := []string{
		"ID", "Name", "Kategorie", "Einheit", "Standardpreis",
		"Max_Anzahl", "Beschreibung", "Aktiv",
	}
	writer.Write(headers)

	query := `SELECT id, name, kategorie, einheit, standardpreis, max_anzahl, beschreibung, aktiv FROM obstarten ORDER BY kategorie, name`
	rows, err := models.DB.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var id, maxAnzahl int
		var name, kategorie, einheit, beschreibung string
		var standardpreis float64
		var aktiv bool

		err := rows.Scan(&id, &name, &kategorie, &einheit, &standardpreis, &maxAnzahl, &beschreibung, &aktiv)
		if err != nil {
			continue
		}

		record := []string{
			strconv.Itoa(id),
			name,
			kategorie,
			einheit,
			fmt.Sprintf("%.2f", standardpreis),
			strconv.Itoa(maxAnzahl),
			beschreibung,
			boolToString(aktiv),
		}
		writeCSVRecord(writer, record)
	}

	log.Printf("Obstarten exportiert: %s", fileName)
	return fileName, nil
}

func (s *CSVService) ExportZieranpflanzungen() (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	fileName := fmt.Sprintf("zieranpflanzungen_%s.csv", timestamp)
	filePath := filepath.Join(s.exportDir, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	file.WriteString("\uFEFF")
	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	headers := []string{
		"ID", "Name", "Kategorie", "Preis_Pro_QM",
		"Beschreibung", "Max_Flaeche", "Aktiv",
	}
	writer.Write(headers)

	query := `SELECT id, name, kategorie, preis_pro_qm, beschreibung, max_flaeche, aktiv FROM zieranpflanzungen ORDER BY kategorie, name`
	rows, err := models.DB.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var name, kategorie, beschreibung string
		var preisProQM float64
		var maxFlaeche *int
		var aktiv bool

		err := rows.Scan(&id, &name, &kategorie, &preisProQM, &beschreibung, &maxFlaeche, &aktiv)
		if err != nil {
			continue
		}

		maxFlaecheStr := ""
		if maxFlaeche != nil {
			maxFlaecheStr = strconv.Itoa(*maxFlaeche)
		}

		record := []string{
			strconv.Itoa(id),
			name,
			kategorie,
			fmt.Sprintf("%.2f", preisProQM),
			beschreibung,
			maxFlaecheStr,
			boolToString(aktiv),
		}
		writeCSVRecord(writer, record)
	}

	log.Printf("Zieranpflanzungen exportiert: %s", fileName)
	return fileName, nil
}

func (s *CSVService) ExportBauindex() (string, error) {
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	fileName := fmt.Sprintf("bauindex_%s.csv", timestamp)
	filePath := filepath.Join(s.exportDir, fileName)

	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	file.WriteString("\uFEFF")
	writer := csv.NewWriter(file)
	writer.Comma = ';'
	defer writer.Flush()

	headers := []string{"Jahr", "Bauindex"}
	writer.Write(headers)

	query := `SELECT jahr, bauindex FROM bauindex_tabelle ORDER BY jahr DESC`
	rows, err := models.DB.Query(query)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var jahr int
		var bauindex float64

		err := rows.Scan(&jahr, &bauindex)
		if err != nil {
			continue
		}

		record := []string{
			strconv.Itoa(jahr),
			fmt.Sprintf("%.1f", bauindex),
		}
		writeCSVRecord(writer, record)
	}

	log.Printf("Bauindex exportiert: %s", fileName)
	return fileName, nil
}

// Import-Funktionen

func (s *CSVService) ImportFromFile(fileHeader *multipart.FileHeader, tableName string) (*CSVImportReport, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ';' // Deutsche CSV-Konvention
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("CSV-Parsing-Fehler: %v", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV-Datei muss mindestens Header und eine Datenzeile enthalten")
	}

	// Header validieren und Import durchführen
	switch tableName {
	case "parzellen":
		return s.importParzellen(records)
	case "obstarten":
		return s.importObstarten(records)
	case "zieranpflanzungen":
		return s.importZieranpflanzungen(records)
	case "bauindex":
		return s.importBauindex(records)
	default:
		return nil, fmt.Errorf("unbekannte Tabelle: %s", tableName)
	}
}

func (s *CSVService) importParzellen(records [][]string) (*CSVImportReport, error) {
	headers := records[0]
	// ERWEITERTE Header mit neuen Feldern
	expectedHeaders := []string{"ID", "Nummer", "Groesse", "Verein", "Paechter_Name", "Email", "Telefon", "Notizen", "Kuendigung_Datum", "Erstellt_Am"}
	report := &CSVImportReport{Table: "parzellen", TotalRows: len(records) - 1}

	if !validateHeaders(headers, expectedHeaders) {
		return nil, fmt.Errorf("ungültige Header. Erwartet: %v", expectedHeaders)
	}

	// Transaktion für Bulk-Import
	tx, err := models.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// ERWEITERTE Query mit neuen Feldern
	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO parzellen (id, nummer, groesse, verein, paechter_name, email, telefon, notizen, kuendigung_datum, erstellt_am) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	for lineNum, record := range records[1:] { // Skip header
		if len(record) < len(expectedHeaders) {
			log.Printf("Zeile %d übersprungen: zu wenige Spalten", lineNum+2)
			report.SkippedRows++
			report.ErrorRows++
			report.Errors = append(report.Errors, fmt.Sprintf("Zeile %d: zu wenige Spalten", lineNum+2))
			continue
		}

		id, _ := strconv.Atoi(record[0])
		groesse, _ := strconv.ParseFloat(record[2], 64)
		kuendigungDatum := parseTimeFromCSV(record[8]) // Index angepasst
		erstelltAm := parseTimeFromCSV(record[9])      // Index angepasst

		// ERWEITERTE Parameter mit neuen Feldern
		_, err := stmt.Exec(id, record[1], groesse, record[3], record[4],
			record[5], record[6], record[7], kuendigungDatum, erstelltAm)
		if err != nil {
			log.Printf("Fehler beim Import von Zeile %d: %v", lineNum+2, err)
			report.ErrorRows++
			report.Errors = append(report.Errors, fmt.Sprintf("Zeile %d: %v", lineNum+2, err))
			continue
		}
		report.ImportedRows++
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	report.SkippedRows += report.TotalRows - report.ImportedRows - report.ErrorRows
	log.Printf("Parzellen-Import erfolgreich: %d Datensätze", report.ImportedRows)
	return report, nil
}

func (s *CSVService) importObstarten(records [][]string) (*CSVImportReport, error) {
	headers := records[0]
	expectedHeaders := []string{"ID", "Name", "Kategorie", "Einheit", "Standardpreis", "Max_Anzahl", "Beschreibung", "Aktiv"}
	report := &CSVImportReport{Table: "obstarten", TotalRows: len(records) - 1}

	if !validateHeaders(headers, expectedHeaders) {
		return nil, fmt.Errorf("ungültige Header. Erwartet: %v", expectedHeaders)
	}

	tx, err := models.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO obstarten (id, name, kategorie, einheit, standardpreis, max_anzahl, beschreibung, aktiv) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	for lineNum, record := range records[1:] {
		if len(record) < len(expectedHeaders) {
			report.SkippedRows++
			report.ErrorRows++
			report.Errors = append(report.Errors, fmt.Sprintf("Zeile %d: zu wenige Spalten", lineNum+2))
			continue
		}

		id, _ := strconv.Atoi(record[0])
		standardpreis, _ := strconv.ParseFloat(record[4], 64)
		maxAnzahl, _ := strconv.Atoi(record[5])
		aktiv := stringToBool(record[7])

		_, err := stmt.Exec(id, record[1], record[2], record[3], standardpreis, maxAnzahl, record[6], aktiv)
		if err != nil {
			log.Printf("Fehler beim Import von Obstart %s: %v", record[1], err)
			report.ErrorRows++
			report.Errors = append(report.Errors, fmt.Sprintf("Zeile %d: %v", lineNum+2, err))
			continue
		}
		report.ImportedRows++
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	report.SkippedRows += report.TotalRows - report.ImportedRows - report.ErrorRows
	log.Printf("Obstarten-Import erfolgreich: %d Datensätze", report.ImportedRows)
	return report, nil
}

func (s *CSVService) importZieranpflanzungen(records [][]string) (*CSVImportReport, error) {
	headers := records[0]
	expectedHeaders := []string{"ID", "Name", "Kategorie", "Preis_Pro_QM", "Beschreibung", "Max_Flaeche", "Aktiv"}
	report := &CSVImportReport{Table: "zieranpflanzungen", TotalRows: len(records) - 1}

	if !validateHeaders(headers, expectedHeaders) {
		return nil, fmt.Errorf("ungültige Header. Erwartet: %v", expectedHeaders)
	}

	tx, err := models.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO zieranpflanzungen (id, name, kategorie, preis_pro_qm, beschreibung, max_flaeche, aktiv) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	for lineNum, record := range records[1:] {
		if len(record) < len(expectedHeaders) {
			report.SkippedRows++
			report.ErrorRows++
			report.Errors = append(report.Errors, fmt.Sprintf("Zeile %d: zu wenige Spalten", lineNum+2))
			continue
		}

		id, _ := strconv.Atoi(record[0])
		preisProQM, _ := strconv.ParseFloat(record[3], 64)

		var maxFlaeche *int
		if record[5] != "" {
			if mf, err := strconv.Atoi(record[5]); err == nil {
				maxFlaeche = &mf
			}
		}

		aktiv := stringToBool(record[6])

		_, err := stmt.Exec(id, record[1], record[2], preisProQM, record[4], maxFlaeche, aktiv)
		if err != nil {
			log.Printf("Fehler beim Import von Zieranpflanzung %s: %v", record[1], err)
			report.ErrorRows++
			report.Errors = append(report.Errors, fmt.Sprintf("Zeile %d: %v", lineNum+2, err))
			continue
		}
		report.ImportedRows++
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	report.SkippedRows += report.TotalRows - report.ImportedRows - report.ErrorRows
	log.Printf("Zieranpflanzungen-Import erfolgreich: %d Datensätze", report.ImportedRows)
	return report, nil
}

func (s *CSVService) importBauindex(records [][]string) (*CSVImportReport, error) {
	headers := records[0]
	expectedHeaders := []string{"Jahr", "Bauindex"}
	report := &CSVImportReport{Table: "bauindex", TotalRows: len(records) - 1}

	if !validateHeaders(headers, expectedHeaders) {
		return nil, fmt.Errorf("ungültige Header. Erwartet: %v", expectedHeaders)
	}

	tx, err := models.DB.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT OR REPLACE INTO bauindex_tabelle (jahr, bauindex) VALUES (?, ?)`)
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	for lineNum, record := range records[1:] {
		if len(record) < 2 {
			report.SkippedRows++
			report.ErrorRows++
			report.Errors = append(report.Errors, fmt.Sprintf("Zeile %d: zu wenige Spalten", lineNum+2))
			continue
		}

		jahr, _ := strconv.Atoi(record[0])
		bauindex, _ := strconv.ParseFloat(record[1], 64)

		_, err := stmt.Exec(jahr, bauindex)
		if err != nil {
			log.Printf("Fehler beim Import von Bauindex %d: %v", jahr, err)
			report.ErrorRows++
			report.Errors = append(report.Errors, fmt.Sprintf("Zeile %d: %v", lineNum+2, err))
			continue
		}
		report.ImportedRows++
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	report.SkippedRows += report.TotalRows - report.ImportedRows - report.ErrorRows
	log.Printf("Bauindex-Import erfolgreich: %d Datensätze", report.ImportedRows)
	return report, nil
}

func (s *CSVService) VerifyCSVHealth() (*CSVHealthReport, error) {
	report := &CSVHealthReport{
		Counts: map[string]int{},
	}

	countTables := []string{"parzellen", "inspektionen", "wertermittlungen", "obstarten", "zieranpflanzungen", "bauindex_tabelle"}
	for _, table := range countTables {
		var count int
		if err := models.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count); err != nil {
			return nil, err
		}
		report.Counts[table] = count
	}

	rows, err := models.DB.Query("PRAGMA foreign_key_check")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if rows.Next() {
		report.ForeignKeyOK = false
		report.ForeignKeyMsg = "Foreign-Key-Verletzungen gefunden"
		return report, nil
	}

	report.ForeignKeyOK = true
	report.ForeignKeyMsg = "Keine Foreign-Key-Verletzungen"
	return report, nil
}

// Hilfsfunktionen

func formatTimeForCSV(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02 15:04:05")
}

func parseTimeFromCSV(s string) *time.Time {
	if s == "" {
		return nil
	}

	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02.01.2006",
		"02.01.2006 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return &t
		}
	}
	return nil
}

func boolToString(b bool) string {
	if b {
		return "Ja"
	}
	return "Nein"
}

func stringToBool(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return s == "ja" || s == "true" || s == "1" || s == "wahr"
}

func validateHeaders(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}

	for i, header := range expected {
		if i >= len(actual) || strings.TrimSpace(actual[i]) != header {
			return false
		}
	}
	return true
}
