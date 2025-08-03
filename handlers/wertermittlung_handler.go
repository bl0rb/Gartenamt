package handlers

import (
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"kleingarten-verwaltung/models"
	"kleingarten-verwaltung/services"
)

func WertermittlungHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	parzelleID, err := strconv.Atoi(vars["parzelle_id"])
	if err != nil {
		http.Error(w, "Ungültige Parzellen-ID", http.StatusBadRequest)
		return
	}

	// Parzelle laden
	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		http.Error(w, "Parzelle nicht gefunden", http.StatusNotFound)
		return
	}

	if r.Method == "POST" {
		// POST-Verarbeitung mit Service
		if err := handleWertermittlungPostMitService(w, r, parzelleID); err != nil {
			log.Printf("Fehler bei Wertermittlung POST: %v", err)
			http.Error(w, "Fehler beim Speichern der Wertermittlung: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// Nach erfolgreichem POST -> Redirect
		http.Redirect(w, r, "/parzellen/"+strconv.Itoa(parzelleID), http.StatusSeeOther)
		return
	}

	// GET-Request: Template anzeigen
	handleWertermittlungGet(w, r, parzelle)
}

func handleWertermittlungPostMitService(w http.ResponseWriter, r *http.Request, parzelleID int) error {
	// Berechnungs-Service initialisieren
	berechnungsService := services.NewBerechnungsService(models.DB)

	wertermittlung := &models.Wertermittlung{
		ParzelleID: parzelleID,
		Datum:      time.Now(),
	}

	var laubeWert float64 = 0

	// A. LAUBE - mit Service-Integration für Bauindex
	if r.FormValue("laube_vorhanden") == "true" {
		laubeDetails := models.LaubeDetails{}

		// Grundlegende Laube-Daten
		if erstellungsjahr := r.FormValue("laube_erstellungsjahr"); erstellungsjahr != "" {
			if jahr, err := strconv.Atoi(erstellungsjahr); err == nil {
				laubeDetails.Erstellungsjahr = jahr
			}
		}

		if grundflaeche := r.FormValue("laube_grundflaeche"); grundflaeche != "" {
			if gf, err := strconv.ParseFloat(grundflaeche, 64); err == nil {
				laubeDetails.Grundflaeche = gf
			}
		}

		if herstellungswert := r.FormValue("laube_herstellungswert"); herstellungswert != "" {
			if hw, err := strconv.ParseFloat(herstellungswert, 64); err == nil {
				laubeDetails.HerstellungswertProQM = hw
			}
		}

		laubeDetails.Bauklasse = r.FormValue("laube_bauklasse")

		// Material und Abschreibung
		laubeDetails.AbschreibungProzent = 3.0 // Standard für Holzlauben
		if r.FormValue("laube_material") == "stein" {
			laubeDetails.AbschreibungProzent = 2.0
		}

		// DATENBANKBASIERTER Bauindex
		aktuellerBauindex, err := berechnungsService.GetAktuellerBauindex()
		if err != nil {
			aktuellerBauindex = 42.9 // Fallback
		}
		laubeDetails.Bauindex = aktuellerBauindex

		// Manuelle Eingabe prüfen
		if r.FormValue("laube_manuell") == "true" {
			laubeDetails.ManuellEingegeben = true
			laubeDetails.Begruendung = r.FormValue("laube_begruendung")

			// Restwert-Prozent verarbeiten
			if restwertProzent := r.FormValue("laube_restwert_prozent"); restwertProzent != "" {
				if rp, err := strconv.ParseFloat(restwertProzent, 64); err == nil {
					if rp > 15.0 {
						return errors.New("Restwert darf maximal 15% betragen")
					}
					laubeDetails.RestwertProzent = rp
				}
			}

			// Alternativer manueller Zeitwert
			if manuellZeitwert := r.FormValue("laube_manuell_zeitwert"); manuellZeitwert != "" {
				if mz, err := strconv.ParseFloat(manuellZeitwert, 64); err == nil {
					laubeDetails.ManuellZeitwert = mz
				}
			}

			// Validierung der manuellen Eingabe
			if fehler := laubeDetails.ValidiereManuellEingabe(); len(fehler) > 0 {
				return errors.New("Validierungsfehler: " + strings.Join(fehler, ", "))
			}
		}

		// Laube-Wert berechnen (automatisch oder manuell)
		laubeWert = wertermittlung.BerechneLaubeWert(laubeDetails, aktuellerBauindex)
		wertermittlung.Details.Laube = laubeDetails
	}

	// B. SONSTIGE BAULICHKEITEN - vereinfacht
	var baulichkeitenWert float64 = 0

	// Wege
	if wegeFlaeche := r.FormValue("wege_flaeche"); wegeFlaeche != "" {
		if flaeche, err := strconv.ParseFloat(wegeFlaeche, 64); err == nil && flaeche > 0 {
			qualitaet, _ := strconv.ParseFloat(r.FormValue("wege_qualitaet"), 64)
			baulichkeitenWert += flaeche * qualitaet
		}
	}

	// Pforte
	var pforteWert float64 = 0
	if r.FormValue("pforte_vorhanden") == "true" {
		if pforteWertStr := r.FormValue("pforte_wert"); pforteWertStr != "" {
			if wert, err := strconv.ParseFloat(pforteWertStr, 64); err == nil {
				pforteWert = wert
			}
		}
	}

	// Strom- und Wasserversorgung (vereinfacht)
	var stromWert float64 = 0
	var wasserWert float64 = 0

	if r.FormValue("strom_vorhanden") == "true" {
		if stromWertStr := r.FormValue("strom_typ"); stromWertStr != "" {
			if wert, err := strconv.ParseFloat(stromWertStr, 64); err == nil {
				// Vereinfachte Abschreibung: 3% pro Jahr
				if baujahr := r.FormValue("strom_baujahr"); baujahr != "" {
					if jahr, err := strconv.Atoi(baujahr); err == nil {
						alter := time.Now().Year() - jahr
						abschreibung := float64(alter) * 3.0 / 100
						if abschreibung > 90 {
							abschreibung = 90
						}
						stromWert = wert * (1 - abschreibung/100)
					}
				} else {
					stromWert = wert * 0.5
				}
			}
		}
	}

	if r.FormValue("wasser_vorhanden") == "true" {
		wasserWert = 37.50
	}

	baulichkeitenWert += pforteWert + stromWert + wasserWert

	// D. OBSTGEHÖLZE - MIT SERVICE
	var obstDetails []models.ObstDetail
	obstID := 1

	if err := r.ParseForm(); err == nil {
		obstArten := r.Form["obst_art[]"]
		obstAnzahlen := r.Form["obst_anzahl[]"]
		obstEinheiten := r.Form["obst_einheit[]"]
		obstEinzelpreise := r.Form["obst_einzelpreis[]"]

		for i := 0; i < len(obstArten) && i < len(obstAnzahlen); i++ {
			if obstArten[i] == "" {
				continue
			}

			anzahl, _ := strconv.ParseFloat(obstAnzahlen[i], 64)
			einzelpreis, _ := strconv.ParseFloat(obstEinzelpreise[i], 64)

			if anzahl > 0 && einzelpreis > 0 {
				// DATENBANKBASIERTE Kategorie-Ermittlung
				preisInfo, err := berechnungsService.GetObstartPreis(obstArten[i])
				kategorie := "E1" // Default
				if err == nil {
					kategorie = preisInfo.Kategorie
				}

				einheit := "Stück"
				if i < len(obstEinheiten) {
					einheit = obstEinheiten[i]
				}

				obstDetail := models.ObstDetail{
					ID:          obstID,
					Kategorie:   kategorie,
					Art:         obstArten[i],
					Anzahl:      int(anzahl),
					Einheit:     einheit,
					EinzelPreis: einzelpreis,
					GesamtWert:  anzahl * einzelpreis,
				}

				obstDetails = append(obstDetails, obstDetail)
				obstID++
			}
		}
	}

	wertermittlung.Details.Obst = obstDetails

	// E. GEMÜSE - MIT SERVICE
	einzelpflanzen, _ := strconv.Atoi(r.FormValue("gemuese_einzelpflanzen_anzahl"))
	reihenMeter, _ := strconv.ParseFloat(r.FormValue("gemuese_reihen_meter"), 64)
	kraeuterFlaeche, _ := strconv.ParseFloat(r.FormValue("kraeuter_flaeche"), 64)

	// F. ZIERANPFLANZUNGEN - MIT SERVICE
	zierEintraege := map[string]float64{
		"F1": parseFloatSafe(r.FormValue("zier_f1")),
		"F2": parseFloatSafe(r.FormValue("zier_f2")),
		"F3": parseFloatSafe(r.FormValue("zier_f3")),
		"F4": parseFloatSafe(r.FormValue("zier_f4")),
		"F8": parseFloatSafe(r.FormValue("rasen")),
	}

	// Gartenteich
	if teichFlaeche := parseFloatSafe(r.FormValue("teich_flaeche")); teichFlaeche > 0 {
		if teichFlaeche > 15 {
			teichFlaeche = 15 // Max 15 m²
		}
		teichTyp := r.FormValue("teich_typ")
		zierEintraege[teichTyp] = teichFlaeche
	}

	// GESAMTE WERTERMITTLUNG MIT SERVICE BERECHNEN
	berechnet, err := berechnungsService.BerechneGesamtWertermittlung(
		laubeWert, baulichkeitenWert-pforteWert-stromWert-wasserWert, pforteWert, stromWert, wasserWert,
		obstDetails,
		einzelpflanzen, reihenMeter, kraeuterFlaeche,
		zierEintraege)

	if err != nil {
		return err
	}

	// Werte in finale Wertermittlung übertragen
	wertermittlung.LaubeWert = berechnet.LaubeWert
	wertermittlung.BaulichkeitenWert = berechnet.BaulichkeitenWert
	wertermittlung.ObstWert = berechnet.ObstWert
	wertermittlung.GemuseWert = berechnet.GemuseWert
	wertermittlung.ZierWert = berechnet.ZierWert
	wertermittlung.GesamtWert = berechnet.GesamtWert

	// Speichern
	return wertermittlung.Save()
}

func handleWertermittlungGet(w http.ResponseWriter, r *http.Request, parzelle *models.Parzelle) {
	// Bestehende Wertermittlung laden falls vorhanden
	wertermittlung, _ := models.GetWertermittlungByParzelleID(parzelle.ID)

	// Template-Funktionen definieren
	funcMap := template.FuncMap{
		"div": func(a, b float64) float64 {
			if b == 0 {
				return 0
			}
			return a / b
		},
		"add": func(a, b float64) float64 {
			return a + b
		},
		"sub": func(a, b float64) float64 {
			return a - b
		},
		"printf": func(format string, args ...interface{}) string {
			return fmt.Sprintf(format, args...)
		},
	}

	// Template laden und ausführen
	tmpl, err := template.New("").Funcs(funcMap).ParseFiles("templates/layout.html", "templates/wertermittlung.html")
	if err != nil {
		log.Printf("Template Parse Error: %v", err)
		http.Error(w, "Template-Fehler", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":           "Wertermittlung - Parzelle " + parzelle.Nummer,
		"Parzelle":        parzelle,
		"Wertermittlung":  wertermittlung,
		"BauindexTabelle": models.BauindexTabelle,
	}

	err = tmpl.ExecuteTemplate(w, "layout.html", data)
	if err != nil {
		log.Printf("Template Execute Error: %v", err)
		http.Error(w, "Template-Rendering-Fehler: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

// Hilfsfunktion
func parseFloatSafe(s string) float64 {
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

func ProtokollHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	typ := vars["typ"]
	id, _ := strconv.Atoi(vars["id"])

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "attachment; filename="+typ+"_protokoll_"+strconv.Itoa(id)+".pdf")

	// Hier würde PDF-Generierung implementiert
	// Für jetzt nur Platzhalter
	w.Write([]byte("PDF Protokoll für " + typ + " ID: " + strconv.Itoa(id)))
}
