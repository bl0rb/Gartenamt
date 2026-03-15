package handlers

import (
	"encoding/json"
	"kleingarten-verwaltung/models"
	"kleingarten-verwaltung/services"
	"net/http"
)

// API-Endpoint für Obstarten-Preise
func APIObstartenPreiseHandler(w http.ResponseWriter, r *http.Request) {
	berechnungsService := services.NewBerechnungsService(models.DB)

	preise, err := berechnungsService.GetAllObstartenPreise()
	if err != nil {
		http.Error(w, "Fehler beim Laden der Obstarten-Preise", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(preise)
}

// API-Endpoint für Zieranpflanzungen-Preise
func APIZieranpflanzungsPreiseHandler(w http.ResponseWriter, r *http.Request) {
	berechnungsService := services.NewBerechnungsService(models.DB)

	preise, err := berechnungsService.GetAllZieranpflanzungsPreise()
	if err != nil {
		http.Error(w, "Fehler beim Laden der Zieranpflanzungs-Preise", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(preise)
}

// API-Endpoint für Gemüse-Preise
func APIGemusePreiseHandler(w http.ResponseWriter, r *http.Request) {
	berechnungsService := services.NewBerechnungsService(models.DB)

	preise, err := berechnungsService.GetGemusePreise()
	if err != nil {
		http.Error(w, "Fehler beim Laden der Gemüse-Preise", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(preise)
}

// API-Endpoint für Bauindex
func APIBauindexHandler(w http.ResponseWriter, r *http.Request) {
	berechnungsService := services.NewBerechnungsService(models.DB)

	bauindex, err := berechnungsService.GetAktuellerBauindex()
	if err != nil {
		http.Error(w, "Fehler beim Laden des Bauindex", http.StatusInternalServerError)
		return
	}

	response := map[string]float64{"bauindex": bauindex}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// APIParzellenHandler returns all parzellen for admin invoice utilities and selection UIs.
func APIParzellenHandler(w http.ResponseWriter, r *http.Request) {
	parzellen, err := models.GetAllParzellen()
	if err != nil {
		http.Error(w, "Fehler beim Laden der Parzellen", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(parzellen)
}
