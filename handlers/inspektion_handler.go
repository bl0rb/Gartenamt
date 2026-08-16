package handlers

import (
	"fmt"
	"github.com/bl0rb/gartenamt/models"
	"html/template"
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

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/inspektion.html"))
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

	// Footer
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
	pdf.Cell(130, 9, tr("Inspektionsprotokoll"))
	pdf.SetFont("Arial", "", 10)
	pdf.SetXY(leftX+130, 14)
	pdf.Cell(58, 7, tr(inspektion.Datum.Format("02.01.2006")))
	pdf.SetXY(leftX+2, 22)
	pdf.SetFont("Arial", "", 8)
	pdf.Cell(pageW-4, 5, tr(lghOrgName+" — Inspektion nach den Hamburger Richtlinien"))

	// ── Stammdaten-Block ────────────────────────────────────────────
	pdf.SetFillColor(240, 244, 248)
	pdf.SetDrawColor(180, 195, 210)
	blockH := 20.0
	if inspektion.Frist != nil {
		blockH = 26.0
	}
	pdf.Rect(leftX, 32, pageW, blockH, "FD")
	pdf.SetTextColor(35, 35, 35)

	// Linke Spalte
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(leftX+3, 35)
	pdf.Cell(22, 5, tr("Verein:"))
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(65, 5, tr(parzelle.Verein))

	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(leftX+3, 41)
	pdf.Cell(22, 5, tr("Parzelle:"))
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(65, 5, tr(parzelle.Nummer))

	if inspektion.Frist != nil {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetXY(leftX+3, 47)
		pdf.Cell(22, 5, tr("Pächter:"))
		pdf.SetFont("Arial", "", 9)
		pdf.Cell(65, 5, tr(parzelle.PaechterName))
	}

	// Rechte Spalte
	pdf.SetFont("Arial", "B", 9)
	pdf.SetXY(leftX+97, 35)
	pdf.Cell(22, 5, tr("Datum:"))
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(70, 5, tr(inspektion.Datum.Format("02.01.2006")))

	if inspektion.Frist != nil {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetXY(leftX+97, 41)
		pdf.Cell(22, 5, tr("Frist bis:"))
		pdf.SetFont("Arial", "", 9)
		pdf.Cell(70, 5, tr(inspektion.Frist.Format("02.01.2006")))
	}

	if inspektion.Frist == nil {
		pdf.SetFont("Arial", "B", 9)
		pdf.SetXY(leftX+3, 47)
		pdf.Cell(22, 5, tr("Pächter:"))
		pdf.SetFont("Arial", "", 9)
		pdf.Cell(65, 5, tr(parzelle.PaechterName))
	}

	// ── Mängelliste ─────────────────────────────────────────────────
	startY := 32 + blockH + 6
	pdf.SetXY(leftX, startY)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(35, 84, 133)
	pdf.Cell(pageW, 6, tr("Festgestellte Mängel"))
	pdf.SetDrawColor(35, 84, 133)
	lineY := startY + 7
	pdf.Line(leftX, lineY, leftX+pageW, lineY)
	pdf.SetDrawColor(180, 195, 210)
	pdf.SetXY(leftX, lineY+2)

	// Tabellenkopf
	pdf.SetFillColor(35, 84, 133)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(10, 7, tr("Nr."), "1", 0, "C", true, 0, "")
	pdf.CellFormat(8, 7, "", "1", 0, "C", true, 0, "")
	pdf.CellFormat(112, 7, tr("Beschreibung"), "1", 0, "L", true, 0, "")
	pdf.CellFormat(60, 7, tr("Rechtsgrundlage"), "1", 1, "L", true, 0, "")

	isMaengelSelected := func(nr int) bool {
		for _, m := range inspektion.Maengel {
			if m.Nr == nr {
				return true
			}
		}
		return false
	}

	pdf.SetTextColor(35, 35, 35)
	pdf.SetFont("Arial", "", 9)

	for i, mangel := range models.VordefinierteManagel {
		rowY := pdf.GetY()

		// Seitenumbruch-Prüfung
		if rowY > 260 {
			pdf.AddPage()
			rowY = pdf.GetY()
		}

		// Zeilenfarbe abwechselnd
		if i%2 == 0 {
			pdf.SetFillColor(255, 255, 255)
		} else {
			pdf.SetFillColor(235, 244, 253)
		}

		// Höhe der Beschreibungszelle berechnen (MultiCell)
		lineCount := pdf.SplitLines([]byte(tr(mangel.Beschreibung)), 112)
		cellH := float64(len(lineCount)) * 5.5
		if cellH < 6 {
			cellH = 6
		}

		// Nummer
		pdf.CellFormat(10, cellH, fmt.Sprintf("%d", mangel.Nr), "1", 0, "C", true, 0, "")

		// Checkbox
		chkX := pdf.GetX()
		pdf.CellFormat(8, cellH, "", "1", 0, "C", true, 0, "")
		if isMaengelSelected(mangel.Nr) {
			pdf.SetXY(chkX, rowY+(cellH-5)/2)
			pdf.SetFont("Arial", "B", 10)
			pdf.Cell(8, 5, "X")
			pdf.SetFont("Arial", "", 9)
			pdf.SetXY(chkX+8, rowY)
		}

		// Beschreibung
		descX := pdf.GetX()
		pdf.MultiCell(112, 5.5, tr(mangel.Beschreibung), "1", "L", true)
		newY := pdf.GetY()

		// Rechtsgrundlage
		pdf.SetXY(descX+112, rowY)
		pdf.CellFormat(60, newY-rowY, tr(mangel.Rechtsgrundlage), "1", 0, "L", true, 0, "")
		pdf.SetXY(leftX, newY)
	}

	// ── Auflagen-Status ────────────────────────────────────────────
	pdf.Ln(5)
	if pdf.GetY() > 245 {
		pdf.AddPage()
	}
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(35, 84, 133)
	pdf.Cell(pageW, 6, tr("Status der Auflagen"))
	auflagenLineY := pdf.GetY() + 7
	pdf.SetDrawColor(35, 84, 133)
	pdf.Line(leftX, auflagenLineY, leftX+pageW, auflagenLineY)
	pdf.SetDrawColor(180, 195, 210)
	pdf.Ln(9)

	pdf.SetFont("Arial", "", 10)
	pdf.SetTextColor(35, 35, 35)

	// Checkbox Auflagen erfüllt
	chkX := pdf.GetX()
	pdf.SetFillColor(255, 255, 255)
	pdf.CellFormat(7, 7, "", "1", 0, "C", true, 0, "")
	if inspektion.AuflagenErfuellt {
		pdf.SetXY(chkX, pdf.GetY())
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(7, 7, "X")
		pdf.SetFont("Arial", "", 10)
	}
	pdf.SetXY(chkX+8, pdf.GetY())
	pdf.Cell(pageW-8, 7, tr(" Alle Auflagen erfüllt"))
	pdf.Ln(8)

	// Checkbox Auflagen nicht erfüllt
	chkX = pdf.GetX()
	pdf.CellFormat(7, 7, "", "1", 0, "C", true, 0, "")
	if !inspektion.AuflagenErfuellt {
		pdf.SetXY(chkX, pdf.GetY())
		pdf.SetFont("Arial", "B", 10)
		pdf.Cell(7, 7, "X")
		pdf.SetFont("Arial", "", 10)
	}
	pdf.SetXY(chkX+8, pdf.GetY())
	pdf.Cell(pageW-8, 7, tr(" Auflagen nicht erfüllt"))
	pdf.Ln(8)

	// ── Unterschriften-Block ───────────────────────────────────────
	if pdf.GetY() > 240 {
		pdf.AddPage()
	}
	pdf.Ln(10)
	signY := pdf.GetY()
	pdf.SetDrawColor(80, 80, 80)
	pdf.Line(leftX+5, signY, leftX+80, signY)
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
		fmt.Sprintf("attachment; filename=inspektion_%s_%s.pdf",
			parzelle.Nummer, inspektion.Datum.Format("2006-01-02")))

	if err = pdf.Output(w); err != nil {
		http.Error(w, "Fehler beim Generieren des PDFs", http.StatusInternalServerError)
		return
	}
}
