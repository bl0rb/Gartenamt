package handlers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bl0rb/gartenamt/middleware"
	"github.com/bl0rb/gartenamt/models"
	"github.com/bl0rb/gartenamt/services"

	"github.com/gorilla/mux"
)

type invoiceHistoryEntry struct {
	Month        int     `json:"month"`
	Year         int     `json:"year"`
	HasWasser    bool    `json:"has_wasser"`
	HasStrom     bool    `json:"has_strom"`
	WasserKosten float64 `json:"wasser_kosten"`
	StromKosten  float64 `json:"strom_kosten"`
	TotalKosten  float64 `json:"total_kosten"`
}

// WasserHandler lists and manages water records for a parzelle
func WasserHandler(w http.ResponseWriter, r *http.Request) {
	// Check admin privilege
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	parzelleID, err := strconv.Atoi(vars["parzelle_id"])
	if err != nil {
		http.Error(w, "Invalid parzelle ID", http.StatusBadRequest)
		return
	}

	if r.Method == "GET" {
		// List water records
		records, err := models.GetWasserByParzelle(parzelleID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	} else if r.Method == "POST" {
		// Add/update water record
		var w_record models.Wasser
		if err := json.NewDecoder(r.Body).Decode(&w_record); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w_record.ParzelleID = parzelleID
		if err := w_record.Save(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(w_record)
	}
}

// StromHandler lists and manages electricity records for a parzelle
func StromHandler(w http.ResponseWriter, r *http.Request) {
	// Check admin privilege
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	parzelleID, err := strconv.Atoi(vars["parzelle_id"])
	if err != nil {
		http.Error(w, "Invalid parzelle ID", http.StatusBadRequest)
		return
	}

	if r.Method == "GET" {
		// List electricity records
		records, err := models.GetStromByParzelle(parzelleID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(records)
	} else if r.Method == "POST" {
		// Add/update electricity record
		var s_record models.Strom
		if err := json.NewDecoder(r.Body).Decode(&s_record); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		s_record.ParzelleID = parzelleID
		if err := s_record.Save(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(s_record)
	}
}

// DeleteWasserHandler deletes a water record
func DeleteWasserHandler(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	recordID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid record ID", http.StatusBadRequest)
		return
	}

	if err := models.DeleteWasser(recordID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "deleted"}`))
}

// DeleteStromHandler deletes an electricity record
func DeleteStromHandler(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	recordID, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid record ID", http.StatusBadRequest)
		return
	}

	if err := models.DeleteStrom(recordID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "deleted"}`))
}

// OrganizationSettingsHandler manages the organization details
func OrganizationSettingsHandler(w http.ResponseWriter, r *http.Request) {
	// Only admins can access
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	if r.Method == "GET" {
		// Get current settings
		settings, err := models.GetOrganizationSettings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(settings)
	} else if r.Method == "POST" {
		// Update settings
		var settings models.OrganizationSettings
		if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := settings.Save(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("✅ Organization settings updated by admin")

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(settings)
	}
}

// InvoicePreviewHandler generates a preview of an invoice (for display before printing)
func InvoicePreviewHandler(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	parzelleID, err := strconv.Atoi(vars["parzelle_id"])
	if err != nil {
		http.Error(w, "Invalid parzelle ID", http.StatusBadRequest)
		return
	}

	month := r.URL.Query().Get("month")
	year := r.URL.Query().Get("year")
	invoiceType := r.URL.Query().Get("type") // "wasser", "strom", "both"

	// Get parzelle
	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		http.Error(w, "Parzelle not found", http.StatusNotFound)
		return
	}

	// Get organization settings
	org, err := models.GetOrganizationSettings()
	if err != nil {
		log.Printf("⚠️  Could not load organization settings: %v", err)
	}

	// Build invoice data
	invoiceData := map[string]interface{}{
		"parzelle":     parzelle,
		"organization": org,
		"month":        month,
		"year":         year,
		"invoiceType":  invoiceType,
		"createdAt":    time.Now().Format("02.01.2006"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoiceData)
}

// InvoiceHistoryHandler returns all available old invoices for a parzelle by month/year.
func InvoiceHistoryHandler(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	parzelleID, err := strconv.Atoi(vars["parzelle_id"])
	if err != nil {
		http.Error(w, "Invalid parzelle ID", http.StatusBadRequest)
		return
	}

	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		http.Error(w, "Parzelle not found", http.StatusNotFound)
		return
	}

	history, err := buildInvoiceHistory(parzelleID)
	if err != nil {
		http.Error(w, "Fehler beim Laden der Rechnungshistorie", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"parzelle": parzelle,
		"history":  history,
	})
}

// InvoicePDFDownloadHandler creates a PDF for a selected old invoice period and type.
func InvoicePDFDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	vars := mux.Vars(r)
	parzelleID, err := strconv.Atoi(vars["parzelle_id"])
	if err != nil {
		http.Error(w, "Invalid parzelle ID", http.StatusBadRequest)
		return
	}

	month, err := strconv.Atoi(r.URL.Query().Get("month"))
	if err != nil || month < 1 || month > 12 {
		http.Error(w, "Invalid month", http.StatusBadRequest)
		return
	}

	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil || year < 1900 || year > 2100 {
		http.Error(w, "Invalid year", http.StatusBadRequest)
		return
	}

	invoiceType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if invoiceType == "" {
		invoiceType = "both"
	}
	if invoiceType != "wasser" && invoiceType != "strom" && invoiceType != "both" {
		http.Error(w, "Invalid invoice type", http.StatusBadRequest)
		return
	}

	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		http.Error(w, "Parzelle not found", http.StatusNotFound)
		return
	}

	pdfData, err := services.LoadInvoiceDocumentData(parzelleID, month, year, invoiceType)
	if err != nil {
		http.Error(w, "Rechnungsdaten konnten nicht geladen werden", http.StatusInternalServerError)
		return
	}

	if (invoiceType == "wasser" && pdfData.Wasser == nil) ||
		(invoiceType == "strom" && pdfData.Strom == nil) ||
		(invoiceType == "both" && pdfData.Wasser == nil && pdfData.Strom == nil) {
		http.Error(w, "Keine Rechnungsdaten für diesen Zeitraum vorhanden", http.StatusNotFound)
		return
	}

	pdfBytes, err := services.GenerateInvoicePDF(pdfData)
	if err != nil {
		http.Error(w, "Fehler beim Erstellen der Rechnung", http.StatusInternalServerError)
		return
	}

	filename := fmt.Sprintf("rechnung_%s_%04d_%02d_%s.pdf", sanitizeFilePart(parzelle.Nummer), year, month, invoiceType)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(pdfBytes)
}

// AdminBulkInvoiceExportHandler exports all available invoices grouped by parzelle as ZIP.
func AdminBulkInvoiceExportHandler(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	parzellen, err := resolveParzellenForBulkExport(r.URL.Query().Get("parzelle_ids"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)

	addedFiles := 0
	reportLines := []string{"Bulk-Rechnungsexport", ""}
	reportLines = append(reportLines, "Erstellt am: "+time.Now().Format("02.01.2006 15:04:05"))
	reportLines = append(reportLines, fmt.Sprintf("Parzellen im Export: %d", len(parzellen)))
	reportLines = append(reportLines, "")

	for _, parzelle := range parzellen {
		history, historyErr := buildInvoiceHistory(parzelle.ID)
		if historyErr != nil {
			reportLines = append(reportLines, fmt.Sprintf("Parzelle %s: Fehler beim Laden der Historie: %v", parzelle.Nummer, historyErr))
			continue
		}

		for _, entry := range history {
			if entry.HasWasser {
				if err := addInvoiceToZip(zipWriter, parzelle, entry.Month, entry.Year, "wasser"); err != nil {
					reportLines = append(reportLines, fmt.Sprintf("Parzelle %s %02d/%d Wasser: %v", parzelle.Nummer, entry.Month, entry.Year, err))
				} else {
					addedFiles++
				}
			}

			if entry.HasStrom {
				if err := addInvoiceToZip(zipWriter, parzelle, entry.Month, entry.Year, "strom"); err != nil {
					reportLines = append(reportLines, fmt.Sprintf("Parzelle %s %02d/%d Strom: %v", parzelle.Nummer, entry.Month, entry.Year, err))
				} else {
					addedFiles++
				}
			}
		}
	}

	reportLines = append(reportLines, "")
	reportLines = append(reportLines, fmt.Sprintf("Erfolgreich hinzugefuegte Rechnungsdateien: %d", addedFiles))

	reportFile, err := zipWriter.Create("export_report.txt")
	if err == nil {
		_, _ = reportFile.Write([]byte(strings.Join(reportLines, "\n")))
	}

	if err := zipWriter.Close(); err != nil {
		http.Error(w, "Fehler beim Erstellen des ZIP-Exports", http.StatusInternalServerError)
		return
	}

	if addedFiles == 0 {
		http.Error(w, "Keine Rechnungen fuer den Export gefunden", http.StatusNotFound)
		return
	}

	filename := fmt.Sprintf("rechnungen_export_%s.zip", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Write(buffer.Bytes())
}

func buildInvoiceHistory(parzelleID int) ([]invoiceHistoryEntry, error) {
	wasser, err := models.GetWasserByParzelle(parzelleID)
	if err != nil {
		return nil, err
	}

	strom, err := models.GetStromByParzelle(parzelleID)
	if err != nil {
		return nil, err
	}

	index := make(map[string]*invoiceHistoryEntry)

	for _, record := range wasser {
		key := fmt.Sprintf("%04d-%02d", record.Jahr, record.Monat)
		entry, ok := index[key]
		if !ok {
			entry = &invoiceHistoryEntry{Month: record.Monat, Year: record.Jahr}
			index[key] = entry
		}
		entry.HasWasser = true
		entry.WasserKosten = record.Kosten
		entry.TotalKosten = entry.WasserKosten + entry.StromKosten
	}

	for _, record := range strom {
		key := fmt.Sprintf("%04d-%02d", record.Jahr, record.Monat)
		entry, ok := index[key]
		if !ok {
			entry = &invoiceHistoryEntry{Month: record.Monat, Year: record.Jahr}
			index[key] = entry
		}
		entry.HasStrom = true
		entry.StromKosten = record.Kosten
		entry.TotalKosten = entry.WasserKosten + entry.StromKosten
	}

	result := make([]invoiceHistoryEntry, 0, len(index))
	for _, entry := range index {
		result = append(result, *entry)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Year != result[j].Year {
			return result[i].Year > result[j].Year
		}
		return result[i].Month > result[j].Month
	})

	return result, nil
}

func resolveParzellenForBulkExport(rawIDs string) ([]models.Parzelle, error) {
	allParzellen, err := models.GetAllParzellen()
	if err != nil {
		return nil, fmt.Errorf("fehler beim Laden der Parzellen")
	}

	rawIDs = strings.TrimSpace(rawIDs)
	if rawIDs == "" {
		return allParzellen, nil
	}

	selected := make(map[int]bool)
	for _, value := range strings.Split(rawIDs, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		id, convErr := strconv.Atoi(value)
		if convErr != nil {
			return nil, fmt.Errorf("ungueltige parzelle_id: %s", value)
		}
		selected[id] = true
	}

	filtered := make([]models.Parzelle, 0, len(selected))
	for _, parzelle := range allParzellen {
		if selected[parzelle.ID] {
			filtered = append(filtered, parzelle)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("keine passenden Parzellen fuer den Export gefunden")
	}

	return filtered, nil
}

func addInvoiceToZip(zipWriter *zip.Writer, parzelle models.Parzelle, month, year int, invoiceType string) error {
	pdfData, err := services.LoadInvoiceDocumentData(parzelle.ID, month, year, invoiceType)
	if err != nil {
		return err
	}

	if (invoiceType == "wasser" && pdfData.Wasser == nil) || (invoiceType == "strom" && pdfData.Strom == nil) {
		return fmt.Errorf("keine Rechnungsdaten verfuegbar")
	}

	pdfBytes, err := services.GenerateInvoicePDF(pdfData)
	if err != nil {
		return err
	}

	entryPath := fmt.Sprintf("parzelle_%s/%04d/%02d_%s.pdf", sanitizeFilePart(parzelle.Nummer), year, month, invoiceType)
	fileWriter, err := zipWriter.Create(entryPath)
	if err != nil {
		return err
	}

	_, err = fileWriter.Write(pdfBytes)
	return err
}

func sanitizeFilePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unbekannt"
	}

	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		if r == ' ' || r == '.' || r == '/' || r == '\\' {
			b.WriteRune('_')
		}
	}

	result := strings.Trim(b.String(), "_")
	if result == "" {
		return "unbekannt"
	}

	return result
}
