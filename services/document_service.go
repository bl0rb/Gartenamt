package services

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/bl0rb/gartenamt/models"

	"github.com/jung-kurt/gofpdf"
)

type InvoiceDocumentData struct {
	Parzelle  *models.Parzelle
	Org       *models.OrganizationSettings
	Month     int
	Year      int
	Type      string
	Wasser    *models.Wasser
	Strom     *models.Strom
	Generated time.Time
}

func LoadInvoiceDocumentData(parzelleID, month, year int, invoiceType string) (*InvoiceDocumentData, error) {
	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		return nil, err
	}

	org, err := models.GetOrganizationSettings()
	if err != nil {
		return nil, err
	}

	data := &InvoiceDocumentData{
		Parzelle:  parzelle,
		Org:       org,
		Month:     month,
		Year:      year,
		Type:      invoiceType,
		Generated: time.Now(),
	}

	wasserRecords, err := models.GetWasserByParzelle(parzelleID)
	if err == nil {
		for _, record := range wasserRecords {
			if record.Monat == month && record.Jahr == year {
				copy := record
				data.Wasser = &copy
				break
			}
		}
	}

	stromRecords, err := models.GetStromByParzelle(parzelleID)
	if err == nil {
		for _, record := range stromRecords {
			if record.Monat == month && record.Jahr == year {
				copy := record
				data.Strom = &copy
				break
			}
		}
	}

	return data, nil
}

func GenerateInfoPDF(parzelle *models.Parzelle, org *models.OrganizationSettings, subject, message string) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(190, 10, tr(subject))
	pdf.Ln(12)

	pdf.SetFont("Arial", "", 11)
	if org != nil && org.Name != "" {
		pdf.Cell(190, 6, tr(org.Name))
		pdf.Ln(6)
	}
	pdf.Cell(190, 6, tr(fmt.Sprintf("Parzelle: %s", parzelle.Nummer)))
	pdf.Ln(6)
	pdf.Cell(190, 6, tr(fmt.Sprintf("Pächter: %s", parzelle.PaechterName)))
	pdf.Ln(10)
	pdf.MultiCell(190, 6, tr(message), "", "", false)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func GenerateInvoicePDF(data *InvoiceDocumentData) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(190, 10, tr("Rechnung"))
	pdf.Ln(12)

	pdf.SetFont("Arial", "", 11)
	if data.Org != nil {
		pdf.Cell(190, 6, tr(data.Org.Name))
		pdf.Ln(6)
	}
	pdf.Cell(190, 6, tr(fmt.Sprintf("Parzelle: %s", data.Parzelle.Nummer)))
	pdf.Ln(6)
	pdf.Cell(190, 6, tr(fmt.Sprintf("Pächter: %s", data.Parzelle.PaechterName)))
	pdf.Ln(6)
	pdf.Cell(190, 6, tr(fmt.Sprintf("Zeitraum: %02d/%d", data.Month, data.Year)))
	pdf.Ln(10)

	if address := formatParzelleAddress(data.Parzelle); address != "" {
		pdf.MultiCell(190, 6, tr("Rechnungsadresse: "+address), "", "", false)
		pdf.Ln(4)
	}

	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(90, 8, tr("Position"), "1", 0, "", false, 0, "")
	pdf.CellFormat(40, 8, tr("Verbrauch"), "1", 0, "", false, 0, "")
	pdf.CellFormat(40, 8, tr("Kosten"), "1", 1, "", false, 0, "")

	pdf.SetFont("Arial", "", 11)
	total := 0.0
	if (data.Type == "wasser" || data.Type == "both") && data.Wasser != nil {
		pdf.CellFormat(90, 8, tr("Wasser"), "1", 0, "", false, 0, "")
		pdf.CellFormat(40, 8, tr(fmt.Sprintf("%.2f m³", data.Wasser.Verbrauch)), "1", 0, "", false, 0, "")
		pdf.CellFormat(40, 8, tr(fmt.Sprintf("%.2f €", data.Wasser.Kosten)), "1", 1, "", false, 0, "")
		total += data.Wasser.Kosten
	}
	if (data.Type == "strom" || data.Type == "both") && data.Strom != nil {
		pdf.CellFormat(90, 8, tr("Strom"), "1", 0, "", false, 0, "")
		pdf.CellFormat(40, 8, tr(fmt.Sprintf("%.2f kWh", data.Strom.Verbrauch)), "1", 0, "", false, 0, "")
		pdf.CellFormat(40, 8, tr(fmt.Sprintf("%.2f €", data.Strom.Kosten)), "1", 1, "", false, 0, "")
		total += data.Strom.Kosten
	}

	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(130, 8, tr("Gesamt"), "1", 0, "R", false, 0, "")
	pdf.CellFormat(40, 8, tr(fmt.Sprintf("%.2f €", total)), "1", 1, "", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func GenerateInspektionPDFBytes(id int) ([]byte, error) {
	inspektion, err := models.GetInspektionByID(id)
	if err != nil {
		return nil, err
	}

	parzelle, err := models.GetParzelleByID(inspektion.ParzelleID)
	if err != nil {
		return nil, err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(190, 10, tr("Inspektionsprotokoll"))
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 11)
	pdf.Cell(190, 6, tr(fmt.Sprintf("Parzelle: %s", parzelle.Nummer)))
	pdf.Ln(6)
	pdf.Cell(190, 6, tr(fmt.Sprintf("Pächter: %s", parzelle.PaechterName)))
	pdf.Ln(6)
	pdf.Cell(190, 6, tr(fmt.Sprintf("Datum: %s", inspektion.Datum.Format("02.01.2006"))))
	pdf.Ln(10)
	for _, mangel := range inspektion.Maengel {
		pdf.MultiCell(190, 6, tr(fmt.Sprintf("%d. %s", mangel.Nr, mangel.Beschreibung)), "", "", false)
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func GenerateWertermittlungPDFBytes(id int) ([]byte, error) {
	wertermittlung, err := models.GetWertermittlungByID(id)
	if err != nil {
		return nil, err
	}

	parzelle, err := models.GetParzelleByID(wertermittlung.ParzelleID)
	if err != nil {
		return nil, err
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(190, 10, tr("Wertermittlungsprotokoll"))
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 11)
	pdf.Cell(190, 6, tr(fmt.Sprintf("Parzelle: %s", parzelle.Nummer)))
	pdf.Ln(6)
	pdf.Cell(190, 6, tr(fmt.Sprintf("Pächter: %s", parzelle.PaechterName)))
	pdf.Ln(6)
	pdf.Cell(190, 6, tr(fmt.Sprintf("Datum: %s", wertermittlung.Datum.Format("02.01.2006"))))
	pdf.Ln(10)
	addLine := func(label string, value float64) {
		pdf.CellFormat(80, 8, tr(label), "1", 0, "", false, 0, "")
		pdf.CellFormat(50, 8, tr(fmt.Sprintf("%.2f €", value)), "1", 1, "", false, 0, "")
	}
	addLine("Laube", wertermittlung.LaubeWert)
	addLine("Baulichkeiten", wertermittlung.BaulichkeitenWert)
	addLine("Obstgehölze", wertermittlung.ObstWert)
	addLine("Gemüse/Kräuter", wertermittlung.GemuseWert)
	addLine("Ziergehölze", wertermittlung.ZierWert)
	addLine("Gesamt", wertermittlung.GesamtWert)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func formatParzelleAddress(parzelle *models.Parzelle) string {
	streetLine := strings.TrimSpace(strings.TrimSpace(parzelle.PaechterStrasse) + " " + strings.TrimSpace(parzelle.PaechterHausnr))
	cityLine := strings.TrimSpace(strings.TrimSpace(parzelle.PaechterPLZ) + " " + strings.TrimSpace(parzelle.PaechterOrt))
	parts := make([]string, 0, 2)
	if streetLine != "" {
		parts = append(parts, streetLine)
	}
	if cityLine != "" {
		parts = append(parts, cityLine)
	}
	if len(parts) > 0 {
		return strings.Join(parts, ", ")
	}
	return parzelle.PaechterAdress
}
