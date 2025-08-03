package handlers

import (
	"html/template"
	"net/http"
	"strconv"
	//"strings"
	"time"

	"github.com/gorilla/mux"
	"kleingarten-verwaltung/models"
)

func InspektionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	parzelleID, _ := strconv.Atoi(vars["parzelle_id"])

	if r.Method == "POST" {
		inspektion := &models.Inspektion{
			ParzelleID: parzelleID,
			Datum:      time.Now(),
		}

		// Prüfen ob Auflagen erfüllt sind
		if r.FormValue("auflagen_erfuellt") == "true" {
			inspektion.AuflagenErfuellt = true
		}

		// Frist setzen falls Mängel vorhanden
		if frist := r.FormValue("frist"); frist != "" {
			if t, err := time.Parse("2006-01-02", frist); err == nil {
				inspektion.Frist = &t
			}
		}

		// Mängel aus Formular extrahieren
		var maengel []models.Mangel
		for i := 1; i <= 32; i++ {
			checkboxName := "mangel_" + strconv.Itoa(i)
			if r.FormValue(checkboxName) == "true" {
				// Mangel aus vordefinierter Liste finden
				for _, mangel := range models.VordefinierteManagel {
					if mangel.Nr == i {
						mangel.Erfuellt = false // Neu festgestellter Mangel
						maengel = append(maengel, mangel)
						break
					}
				}
			}
		}

		// Weitere Auflagen/Hinweise
		weitereAuflagen := r.FormValue("weitere_auflagen")
		if weitereAuflagen != "" {
			mangel := models.Mangel{
				Nr:              99,
				Beschreibung:    weitereAuflagen,
				Rechtsgrundlage: "Sonstige",
				Erfuellt:        false,
			}
			maengel = append(maengel, mangel)
		}

		inspektion.Maengel = maengel

		if err := inspektion.Save(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/parzellen/"+strconv.Itoa(parzelleID), http.StatusSeeOther)
		return
	}

	// GET Request - Formular anzeigen
	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Bestehende Inspektion laden falls vorhanden
	inspektion, _ := models.GetInspektionByParzelleID(parzelleID)

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/inspektion.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":                "Inspektion - Parzelle " + parzelle.Nummer,
		"Parzelle":             parzelle,
		"Inspektion":           inspektion,
		"VordefinierteManagel": models.VordefinierteManagel,
	})
}
