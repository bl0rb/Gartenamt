package handlers

import (
	"fmt"
	"html/template"
	"kleingarten-verwaltung/models"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/jung-kurt/gofpdf"
)

func InspektionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	parzelleID, _ := strconv.Atoi(vars["parzelle_id"])

	if r.Method == "POST" {
		inspektion := &models.Inspektion{
			ParzelleID: parzelleID,
			Datum:      time.Now(),
		}

		if r.FormValue("auflagen_erfuellt") == "true" {
			inspektion.AuflagenErfuellt = true
		}

		if frist := r.FormValue("frist"); frist != "" {
			if t, err := time.Parse("2006-01-02", frist); err == nil {
				inspektion.Frist = &t
			}
		}

		var maengel []models.Mangel
		for _, vm := range models.VordefinierteManagel {
			if r.FormValue(fmt.Sprintf("mangel_%d", vm.Nr)) == "true" {
				maengel = append(maengel, vm)
			}
		}

		inspektion.Maengel = maengel

		if err := inspektion.Save(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/parzellen/"+strconv.Itoa(parzelleID), http.StatusSeeOther)
		return
	}

	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	inspektion, _ := models.GetInspektionByParzelleID(parzelleID)

	tmpl := template.Must(template.ParseFiles("templates/layout.html", "templates/inspektion.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":                "Inspektion - Parzelle " + parzelle.Nummer,
		"Parzelle":             parzelle,
		"Inspektion":           inspektion,
		"VordefinierteManagel": models.VordefinierteManagel,
	}))
}

func generateInspektionPDF(w http.ResponseWriter, id int) {
	inspektion, err := models.GetInspektionByID(id)
	if err != nil {
		http.Error(w, "Inspektion nicht gefunden", http.StatusNotFound)
		return
	}

	parzelle, err := models.GetParzelleByID(inspektion.ParzelleID)
	if err != nil {
		http.Error(w, "Parzelle nicht gefunden", http.StatusNotFound)
		return
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(190, 10, tr("Inspektionsprotokoll"))
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(190, 6, tr("Landesbund der Gartenfreunde in Hamburg e.V."))
	pdf.Ln(10)

	pdf.Line(10, pdf.GetY(), 200, pdf.GetY())
	pdf.Ln(5)

	// Basis-Informationen
	pdf.SetFont("Arial", "", 11)
	y := pdf.GetY()

	// Linke Spalte
	pdf.CellFormat(30, 6, tr("Verein:"), "", 0, "", false, 0, "")
	pdf.CellFormat(60, 6, tr(parzelle.Verein), "", 1, "", false, 0, "")
	pdf.CellFormat(30, 6, tr("Parzelle:"), "", 0, "", false, 0, "")
	pdf.CellFormat(60, 6, tr(parzelle.Nummer), "", 1, "", false, 0, "")
	pdf.CellFormat(30, 6, tr("Pächter:"), "", 0, "", false, 0, "")
	pdf.CellFormat(60, 6, tr(parzelle.PaechterName), "", 1, "", false, 0, "")

	// Rechte Spalte
	pdf.SetXY(110, y)
	pdf.CellFormat(30, 6, tr("Datum:"), "", 0, "", false, 0, "")
	pdf.CellFormat(60, 6, tr(inspektion.Datum.Format("02.01.2006")), "", 1, "", false, 0, "")
	if inspektion.Frist != nil {
		pdf.SetX(110)
		pdf.CellFormat(30, 6, tr("Frist bis:"), "", 0, "", false, 0, "")
		pdf.CellFormat(60, 6, tr(inspektion.Frist.Format("02.01.2006")), "", 1, "", false, 0, "")
	}
	pdf.Ln(15)

	// Mängelliste Header
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(190, 8, tr("Festgestellte Mängel:"))
	pdf.Ln(10)

	// Tabellenkopf
	pdf.SetFont("Arial", "", 11)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(10, 8, tr("Nr."), "1", 0, "", true, 0, "")
	pdf.CellFormat(8, 8, "", "1", 0, "", true, 0, "") // Checkbox column
	pdf.CellFormat(112, 8, tr("Mangel"), "1", 0, "", true, 0, "")
	pdf.CellFormat(60, 8, tr("Rechtsgrundlage"), "1", 1, "", true, 0, "")

	// Helper function to check if a Mangel is selected
	isMaengelSelected := func(nr int) bool {
		for _, m := range inspektion.Maengel {
			if m.Nr == nr {
				return true
			}
		}
		return false
	}

	// Alle vordefinierten Mängel auflisten
	for _, mangel := range models.VordefinierteManagel {
		//startY := pdf.GetY()

		// Nummer
		pdf.CellFormat(10, 6, fmt.Sprintf("%d", mangel.Nr), "1", 0, "", false, 0, "")

		// Checkbox
		checkboxX := pdf.GetX()
		//checkboxY := pdf.GetY()
		pdf.CellFormat(8, 6, "", "1", 0, "", false, 0, "")

		// Draw checkbox mark if selected
		if isMaengelSelected(mangel.Nr) {
			pdf.SetX(checkboxX)
			pdf.CellFormat(8, 6, "X", "", 0, "C", false, 0, "")
		}

		// Beschreibung (mehrzeilig)
		x := pdf.GetX()
		y := pdf.GetY()
		pdf.MultiCell(112, 6, tr(mangel.Beschreibung), "1", "", false)

		// Höhe der Beschreibungszelle ermitteln
		newY := pdf.GetY()
		height := newY - y

		// Rechtsgrundlage
		pdf.SetXY(x+112, y)
		pdf.CellFormat(60, height, tr(mangel.Rechtsgrundlage), "1", 1, "", false, 0, "")
	}

	pdf.Ln(10)

	// Status der Auflagen
	pdf.SetFont("Arial", "B", 11)
	pdf.Cell(190, 8, tr("Status der Auflagen:"))
	pdf.Ln(8)

	pdf.SetFont("Arial", "", 11)

	// Checkbox für "Auflagen erfüllt"
	checkboxX := pdf.GetX()
	pdf.CellFormat(8, 6, "", "1", 0, "", false, 0, "")
	if inspektion.AuflagenErfuellt {
		pdf.SetX(checkboxX)
		pdf.CellFormat(8, 6, "X", "", 0, "C", false, 0, "")
	}
	pdf.CellFormat(182, 6, tr(" Auflagen erfüllt"), "", 1, "", false, 0, "")

	// Checkbox für "Auflagen nicht erfüllt"
	checkboxX = pdf.GetX()
	pdf.CellFormat(8, 6, "", "1", 0, "", false, 0, "")
	if !inspektion.AuflagenErfuellt {
		pdf.SetX(checkboxX)
		pdf.CellFormat(8, 6, "X", "", 0, "C", false, 0, "")
	}
	pdf.CellFormat(182, 6, tr(" Auflagen nicht erfüllt"), "", 1, "", false, 0, "")

	pdf.Ln(15)

	// Unterschriften
	pdf.SetY(pdf.GetY() + 10)
	pdf.Line(20, pdf.GetY(), 90, pdf.GetY())
	pdf.Line(110, pdf.GetY(), 180, pdf.GetY())

	pdf.SetY(pdf.GetY() + 5)
	pdf.SetFont("Arial", "", 10)
	pdf.SetX(20)
	pdf.Cell(70, 5, tr("Unterschrift Vereinsvertreter"))
	pdf.SetX(110)
	pdf.Cell(70, 5, tr("Unterschrift Pächter"))

	// Fußzeile
	pdf.SetY(275)
	pdf.SetFont("Arial", "I", 8)
	pdf.Cell(190, 5, tr("Landesbund der Gartenfreunde in Hamburg e.V. - "+time.Now().Format("02.01.2006")))

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition",
		fmt.Sprintf("attachment; filename=inspektion_%s_%s.pdf",
			parzelle.Nummer, inspektion.Datum.Format("2006-01-02")))

	err = pdf.Output(w)
	if err != nil {
		http.Error(w, "Fehler beim Generieren des PDFs", http.StatusInternalServerError)
		return
	}
}
