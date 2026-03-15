package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"kleingarten-verwaltung/middleware"
	"kleingarten-verwaltung/models"

	"github.com/gorilla/mux"
)

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

	// For water invoice
	if invoiceType == "wasser" || invoiceType == "both" {
		// Load wasser data from database
		// This would be used by the template
	}

	// For electricity invoice
	if invoiceType == "strom" || invoiceType == "both" {
		// Load strom data from database
		// This would be used by the template
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoiceData)
}
