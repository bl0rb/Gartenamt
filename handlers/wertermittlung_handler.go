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

	"github.com/jung-kurt/gofpdf"

	"kleingarten-verwaltung/models"
	"kleingarten-verwaltung/services"

	"github.com/gorilla/mux"
)

const lghOrgName = "Landesbund der Gartenfreunde in Hamburg e.V."
const stromManuellFestwert = 80.0
const stromMaxBetrag = 1900.0

func WertermittlungHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	parzelleID, err := strconv.Atoi(vars["parzelle_id"])
	if err != nil {
		http.Error(w, "Ungültige Parzellen-ID", http.StatusBadRequest)
		return
	}

	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		http.Error(w, "Parzelle nicht gefunden", http.StatusNotFound)
		return
	}

	if r.Method == "POST" {
		if err := handleWertermittlungPostMitService(w, r, parzelleID); err != nil {
			log.Printf("Fehler bei Wertermittlung POST: %v", err)
			http.Error(w, "Fehler beim Speichern der Wertermittlung: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/parzellen/"+strconv.Itoa(parzelleID), http.StatusSeeOther)
		return
	}

	handleWertermittlungGet(w, r, parzelle)
}

func handleWertermittlungPostMitService(w http.ResponseWriter, r *http.Request, parzelleID int) error {
	// Berechnungs-Service initialisieren
	berechnungsService := services.NewBerechnungsService(models.DB)

	wertermittlung := &models.Wertermittlung{
		ParzelleID: parzelleID,
		Datum:      time.Now(),
	}
	if existing, err := models.GetWertermittlungByParzelleID(parzelleID); err == nil && existing != nil {
		wertermittlung.ID = existing.ID
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
			aktuellerBauindex = 44.3 // Fallback 2026
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
	var wegeWert float64 = 0

	// Wege
	wegeFlaecheValue := parseFloatSafe(r.FormValue("wege_flaeche"))
	wegeQualitaetValue := parseFloatSafe(r.FormValue("wege_qualitaet"))
	if wegeFlaecheValue > 0 {
		wegeWert = wegeFlaecheValue * wegeQualitaetValue
		baulichkeitenWert += wegeWert
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

	// Strom- und Wasserversorgung
	var stromWert float64 = 0
	var wasserWert float64 = 0
	stromBaujahr := 0
	stromHerstellungswert := 0.0
	stromAbschreibungProzent := 3.0
	stromBelegVorhanden := false
	stromBewertungsgrund := ""

	if r.FormValue("strom_vorhanden") == "true" {
		stromVariante := strings.TrimSpace(r.FormValue("strom_variante"))
		if stromVariante == "" {
			stromVariante = "ohne_beleg"
		}
		switch stromVariante {
		case "manuell_80":
			stromWert = stromManuellFestwert
			stromHerstellungswert = stromManuellFestwert
			stromBewertungsgrund = "manuell_80"
		case "mit_beleg":
			belegWert := parseFloatSafe(r.FormValue("strom_rechnung_betrag"))
			if rechnungsJahr := strings.TrimSpace(r.FormValue("strom_rechnung_jahr")); rechnungsJahr != "" {
				if jahr, err := strconv.Atoi(rechnungsJahr); err == nil {
					stromBaujahr = jahr
				}
			}
			if stromBaujahr == 0 {
				stromBaujahr = time.Now().Year()
			}
			stromHerstellungswert = belegWert
			if stromHerstellungswert > stromMaxBetrag {
				stromHerstellungswert = stromMaxBetrag
			}
			stromAbschreibungProzent = 3.0
			stromBelegVorhanden = true
			stromWert = berechneStromRestwert(stromHerstellungswert, stromBaujahr, 3.0, 10.0)
			stromBewertungsgrund = "belegbasiert"
		default: // ohne_beleg
			stromHerstellungswert = stromMaxBetrag / 2 // 950.0
			laubeJahr := wertermittlung.Details.Laube.Erstellungsjahr
			if laubeJahr == 0 {
				laubeJahr = time.Now().Year()
			}
			stromBaujahr = laubeJahr
			laubeAbschreibung := wertermittlung.Details.Laube.AbschreibungProzent
			if laubeAbschreibung <= 0 {
				laubeAbschreibung = 3.0
			}
			stromAbschreibungProzent = laubeAbschreibung
			stromWert = berechneStromRestwert(stromHerstellungswert, stromBaujahr, laubeAbschreibung, 10.0)
			stromBewertungsgrund = "ohne_beleg"
		}
	}

	if r.FormValue("wasser_vorhanden") == "true" {
		wasserWert = 37.50
	}

	baulichkeitenWert += pforteWert + stromWert + wasserWert
	wertermittlung.Details.Baulichkeiten = []models.BaulichkeitDetail{
		{Typ: "wege", Flaeche: wegeFlaecheValue, Qualitaet: wegeQualitaetValue, Aktiv: wegeFlaecheValue > 0, Restwert: wegeWert},
		{Typ: "pforte", Herstellungswert: parseFloatSafe(r.FormValue("pforte_wert")), Aktiv: r.FormValue("pforte_vorhanden") == "true", Restwert: pforteWert},
		{Typ: "strom", Herstellungswert: stromHerstellungswert, Baujahr: stromBaujahr, Aktiv: r.FormValue("strom_vorhanden") == "true", BelegVorhanden: stromBelegVorhanden, Bewertungsgrund: stromBewertungsgrund, AbschreibungProzent: stromAbschreibungProzent, Restwert: stromWert},
		{Typ: "wasser", Herstellungswert: 37.50, Aktiv: r.FormValue("wasser_vorhanden") == "true", Restwert: wasserWert},
	}

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
	wertermittlung.Details.Gemuese = []models.GemuseDetail{
		{Art: "G1", Menge: float64(einzelpflanzen)},
		{Art: "G2", Menge: reihenMeter},
		{Art: "E9", Menge: kraeuterFlaeche},
	}

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
	wertermittlung.Details.Zier = models.ZierDetail{
		F1:           zierEintraege["F1"],
		F2:           zierEintraege["F2"],
		F3:           zierEintraege["F3"],
		F4:           zierEintraege["F4"],
		F8:           zierEintraege["F8"],
		TeichFlaeche: parseFloatSafe(r.FormValue("teich_flaeche")),
		TeichTyp:     r.FormValue("teich_typ"),
	}

	// GESAMTE WERTERMITTLUNG MIT SERVICE BERECHNEN
	berechnet, err := berechnungsService.BerechneGesamtWertermittlung(
		laubeWert, wegeWert, pforteWert, stromWert, wasserWert,
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
		"baulichkeitByTyp": func(items []models.BaulichkeitDetail, typ string) models.BaulichkeitDetail {
			for _, item := range items {
				if item.Typ == typ {
					return item
				}
			}
			return models.BaulichkeitDetail{Typ: typ}
		},
		"gemueseMenge": func(items []models.GemuseDetail, art string) float64 {
			for _, item := range items {
				if item.Art == art {
					return item.Menge
				}
			}
			return 0
		},
	}

	// Template laden und ausführen
	tmpl, err := LoadTemplateWithFuncs(funcMap, "templates/layout.html", "templates/wertermittlung.html")
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

func berechneStromRestwert(herstellungswert float64, baujahr int, abschreibungProzent float64, minRestwertProzent float64) float64 {
	if herstellungswert <= 0 {
		return 0
	}
	if baujahr <= 0 {
		return herstellungswert * 0.5
	}
	alter := time.Now().Year() - baujahr
	abschreibung := float64(alter) * abschreibungProzent / 100
	if abschreibung > 1.0 {
		abschreibung = 1.0
	}
	if abschreibung < 0 {
		abschreibung = 0
	}
	restwert := herstellungswert * (1 - abschreibung)
	if minRestwertProzent > 0 {
		minimum := herstellungswert * minRestwertProzent / 100
		if restwert < minimum {
			restwert = minimum
		}
	}
	return restwert
}

// Hilfsfunktion
func parseFloatSafe(s string) float64 {
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// Update the ProtokollHandler in wertermittlung_handler.go to handle both types
func ProtokollHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	typ := vars["typ"]
	id, _ := strconv.Atoi(vars["id"])

	switch typ {
	case "wertermittlung":
		generateWertermittlungPDF(w, id)
	case "inspektion":
		generateInspektionPDF(w, id)
	default:
		http.Error(w, "Unbekannter Protokoll-Typ", http.StatusBadRequest)
	}
}

func generateWertermittlungPDF(w http.ResponseWriter, id int) {
	wertermittlung, err := models.GetWertermittlungByID(id)
	if err != nil {
		http.Error(w, "Wertermittlung nicht gefunden", http.StatusNotFound)
		return
	}

	parzelle, err := models.GetParzelleByID(wertermittlung.ParzelleID)
	if err != nil {
		http.Error(w, "Parzelle nicht gefunden", http.StatusNotFound)
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	// Footer-Funktion registrieren
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Arial", "I", 8)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 5, tr(fmt.Sprintf("%s   |   Seite %d", lghOrgName, pdf.PageNo())), "", 0, "C", false, 0, "")
	})

	pdf.AddPage()

	pageW := 190.0
	leftX := 10.0

	// ── Header-Balken ──────────────────────────────────────────────
	pdf.SetFillColor(35, 84, 133)
	pdf.Rect(leftX, 10, pageW, 18, "F")
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 16)
	pdf.SetXY(leftX+2, 13)
	pdf.Cell(130, 9, tr("Wertermittlungsprotokoll"))
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(leftX+130, 14)
	pdf.Cell(58, 7, tr(wertermittlung.Datum.Format("02.01.2006")))

	// Unterzeile
	pdf.SetXY(leftX+2, 22)
	pdf.SetFont("Arial", "", 8)
	pdf.Cell(pageW-4, 5, tr(lghOrgName))

	// ── Stammdaten-Block (grau hinterlegt, 2-spaltig) ──────────────
	pdf.SetFillColor(240, 244, 248)
	pdf.SetDrawColor(180, 195, 210)
	pdf.Rect(leftX, 32, pageW, 20, "FD")
	pdf.SetTextColor(35, 35, 35)

	// Linke Spalte
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(leftX+3, 35)
	pdf.Cell(22, 5, tr("Parzelle:"))
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(65, 5, tr(parzelle.Nummer))

	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(leftX+3, 41)
	pdf.Cell(22, 5, tr("Pächter:"))
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(65, 5, tr(parzelle.PaechterName))

	// Rechte Spalte
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(leftX+97, 35)
	pdf.Cell(22, 5, tr("Verein:"))
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(70, 5, tr(parzelle.Verein))

	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(leftX+97, 41)
	pdf.Cell(22, 5, tr("Datum:"))
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(70, 5, tr(wertermittlung.Datum.Format("02.01.2006")))

	// ── Abschnittsüberschrift ──────────────────────────────────────
	pdf.SetXY(leftX, 56)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(35, 84, 133)
	pdf.Cell(pageW, 6, tr("Wertübersicht"))
	// Blaue Linie unter der Überschrift
	pdf.SetDrawColor(35, 84, 133)
	pdf.Line(leftX, 63, leftX+pageW, 63)
	pdf.SetDrawColor(180, 195, 210)

	// ── Tabellenkopf ──────────────────────────────────────────────
	pdf.SetXY(leftX, 65)
	pdf.SetFillColor(35, 84, 133)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(130, 8, tr("  Kategorie"), "1", 0, "L", true, 0, "")
	pdf.CellFormat(60, 8, tr("Wert (€)"), "1", 1, "R", true, 0, "")

	// ── Tabellenzeilen (abwechselnd weiß/hellblau) ─────────────────
	pdf.SetTextColor(35, 35, 35)
	pdf.SetFont("Arial", "", 10)
	kategorien := []struct {
		Name string
		Wert float64
	}{
		{tr("Laube"), wertermittlung.LaubeWert},
		{tr("Baulichkeiten"), wertermittlung.BaulichkeitenWert},
		{tr("Obstgehölze"), wertermittlung.ObstWert},
		{tr("Gemüse/Kräuter"), wertermittlung.GemuseWert},
		{tr("Ziergehölze"), wertermittlung.ZierWert},
	}
	for i, kat := range kategorien {
		if i%2 == 0 {
			pdf.SetFillColor(255, 255, 255)
		} else {
			pdf.SetFillColor(235, 244, 253)
		}
		pdf.CellFormat(130, 7, tr("  ")+kat.Name, "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 7, tr(fmt.Sprintf("%.2f €", kat.Wert)), "1", 1, "R", true, 0, "")
	}

	// Gesamtwert-Zeile
	pdf.SetFillColor(35, 84, 133)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetDrawColor(35, 84, 133)
	pdf.CellFormat(130, 9, tr("  Gesamtwert"), "1", 0, "L", true, 0, "")
	pdf.CellFormat(60, 9, tr(fmt.Sprintf("%.2f €", wertermittlung.GesamtWert)), "1", 1, "R", true, 0, "")
	pdf.SetDrawColor(180, 195, 210)
	pdf.SetTextColor(35, 35, 35)

	// ── Baulichkeiten-Details ──────────────────────────────────────
	hasBaulichkeiten := false
	for _, b := range wertermittlung.Details.Baulichkeiten {
		if b.Restwert > 0 {
			hasBaulichkeiten = true
			break
		}
	}
	if hasBaulichkeiten {
		pdf.Ln(5)
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(35, 84, 133)
		pdf.Cell(pageW, 6, tr("Baulichkeiten (Details)"))
		pdf.SetDrawColor(35, 84, 133)
		y := pdf.GetY() + 7
		pdf.Line(leftX, y, leftX+pageW, y)
		pdf.SetDrawColor(180, 195, 210)
		pdf.Ln(9)

		pdf.SetFillColor(35, 84, 133)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(130, 7, tr("  Position"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 7, tr("Wert (€)"), "1", 1, "R", true, 0, "")
		pdf.SetTextColor(35, 35, 35)
		pdf.SetFont("Arial", "", 10)

		rowIdx := 0
		baulichkeitLabels := map[string]string{
			"wege":   "Wege/Terrassen",
			"pforte": "Pforte",
			"strom":  "Elektroanschluss",
			"wasser": "Wasserversorgung",
		}
		for _, b := range wertermittlung.Details.Baulichkeiten {
			if b.Restwert <= 0 {
				continue
			}
			label := baulichkeitLabels[b.Typ]
			if label == "" {
				label = strings.Title(b.Typ)
			}
			if rowIdx%2 == 0 {
				pdf.SetFillColor(255, 255, 255)
			} else {
				pdf.SetFillColor(235, 244, 253)
			}
			pdf.CellFormat(130, 7, tr("  ")+tr(label), "1", 0, "L", true, 0, "")
			pdf.CellFormat(60, 7, tr(fmt.Sprintf("%.2f €", b.Restwert)), "1", 1, "R", true, 0, "")
			rowIdx++
		}
	}

	// ── Laube-Begründung ──────────────────────────────────────────
	if len(wertermittlung.Details.Laube.Begruendung) > 0 {
		pdf.Ln(5)
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(35, 84, 133)
		pdf.Cell(pageW, 6, tr("Begründung (manuelle Laube-Bewertung)"))
		y := pdf.GetY() + 7
		pdf.SetDrawColor(35, 84, 133)
		pdf.Line(leftX, y, leftX+pageW, y)
		pdf.SetDrawColor(180, 195, 210)
		pdf.Ln(9)
		pdf.SetFont("Arial", "", 10)
		pdf.SetTextColor(35, 35, 35)
		pdf.SetFillColor(248, 248, 248)
		pdf.MultiCell(pageW, 6, tr(wertermittlung.Details.Laube.Begruendung), "1", "", true)
	}

	// ── Obstgehölze-Tabelle ────────────────────────────────────────
	if len(wertermittlung.Details.Obst) > 0 {
		pdf.Ln(5)
		pdf.SetFont("Arial", "B", 11)
		pdf.SetTextColor(35, 84, 133)
		pdf.Cell(pageW, 6, tr("Obstgehölze"))
		y := pdf.GetY() + 7
		pdf.SetDrawColor(35, 84, 133)
		pdf.Line(leftX, y, leftX+pageW, y)
		pdf.SetDrawColor(180, 195, 210)
		pdf.Ln(9)

		pdf.SetFillColor(35, 84, 133)
		pdf.SetTextColor(255, 255, 255)
		pdf.SetFont("Arial", "B", 10)
		pdf.CellFormat(85, 7, tr("  Art"), "1", 0, "L", true, 0, "")
		pdf.CellFormat(35, 7, tr("Menge"), "1", 0, "C", true, 0, "")
		pdf.CellFormat(35, 7, tr("Einzelpreis"), "1", 0, "R", true, 0, "")
		pdf.CellFormat(35, 7, tr("Gesamt"), "1", 1, "R", true, 0, "")
		pdf.SetTextColor(35, 35, 35)
		pdf.SetFont("Arial", "", 10)

		for i, obst := range wertermittlung.Details.Obst {
			if i%2 == 0 {
				pdf.SetFillColor(255, 255, 255)
			} else {
				pdf.SetFillColor(235, 244, 253)
			}
			pdf.CellFormat(85, 7, tr("  ")+tr(obst.Art), "1", 0, "L", true, 0, "")
			pdf.CellFormat(35, 7, tr(fmt.Sprintf("%d %s", obst.Anzahl, obst.Einheit)), "1", 0, "C", true, 0, "")
			pdf.CellFormat(35, 7, tr(fmt.Sprintf("%.2f €", obst.EinzelPreis)), "1", 0, "R", true, 0, "")
			pdf.CellFormat(35, 7, tr(fmt.Sprintf("%.2f €", obst.GesamtWert)), "1", 1, "R", true, 0, "")
		}
	}

	// ── Unterschriften-Block ───────────────────────────────────────
	// Sicherstellen, dass Unterschriften auf der Seite Platz haben
	if pdf.GetY() > 230 {
		pdf.AddPage()
	}
	pdf.Ln(12)
	signY := pdf.GetY()
	pdf.SetDrawColor(80, 80, 80)
	// Linie links
	pdf.Line(leftX+5, signY, leftX+80, signY)
	// Linie rechts
	pdf.Line(leftX+105, signY, leftX+180, signY)
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(80, 80, 80)
	pdf.SetXY(leftX+5, signY+2)
	pdf.Cell(75, 5, tr("Unterschrift Vereinsvertreter"))
	pdf.SetXY(leftX+105, signY+2)
	pdf.Cell(75, 5, tr("Unterschrift Pächter"))

	// ── Ausgabe ────────────────────────────────────────────────────
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=wertermittlung_%d_%s.pdf",
			id, time.Now().Format("2006-01-02")))

	if err = pdf.Output(w); err != nil {
		http.Error(w, "Fehler beim Generieren des PDFs", http.StatusInternalServerError)
		return
	}
}

func addWertZeile(pdf *gofpdf.Fpdf, bezeichnung string, wert float64) {
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.CellFormat(115, 8, bezeichnung, "1", 0, "", false, 0, "")
	pdf.CellFormat(75, 8, tr(fmt.Sprintf("%.2f €", wert)), "1", 1, "R", false, 0, "")
}
