package handlers

import (
	"encoding/json"
	"fmt"
	htmltemplate "html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/bl0rb/gartenamt/middleware"
	"github.com/bl0rb/gartenamt/models"
	"github.com/bl0rb/gartenamt/services"

	"github.com/gorilla/mux"
)

type sendParzelleEmailRequest struct {
	Month           int      `json:"month"`
	Year            int      `json:"year"`
	Subject         string   `json:"subject"`
	Message         string   `json:"message"`
	AttachmentTypes []string `json:"attachment_types"`
}

type sendParzelleInfoRequest struct {
	Subject string `json:"subject"`
	Message string `json:"message"`
}

type bulkSendParzelleEmailRequest struct {
	ParzelleIDs     []int    `json:"parzelle_ids"`
	Month           int      `json:"month"`
	Year            int      `json:"year"`
	Subject         string   `json:"subject"`
	Message         string   `json:"message"`
	AttachmentTypes []string `json:"attachment_types"`
}

type parzelleEmailResult struct {
	ParzelleID int    `json:"parzelle_id"`
	Recipient  string `json:"recipient"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
}

func SendParzelleEmailHandler(w http.ResponseWriter, r *http.Request) {
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

	var request sendParzelleEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result := sendParzelleMail(parzelleID, request.Month, request.Year, request.Subject, request.Message, request.AttachmentTypes, currentUsername(r))
	if result.Error != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(result)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func SendBulkParzelleEmailHandler(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	var request bulkSendParzelleEmailRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	results := make([]parzelleEmailResult, 0, len(request.ParzelleIDs))
	successCount := 0
	for _, parzelleID := range request.ParzelleIDs {
		result := sendParzelleMail(parzelleID, request.Month, request.Year, request.Subject, request.Message, request.AttachmentTypes, currentUsername(r))
		if result.Error == "" {
			successCount++
		}
		results = append(results, result)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results":       results,
		"success_count": successCount,
		"total_count":   len(results),
	})
}

func SendParzelleInfoMailHandler(w http.ResponseWriter, r *http.Request) {
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

	var request sendParzelleInfoRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	request.Subject = strings.TrimSpace(request.Subject)
	request.Message = strings.TrimSpace(request.Message)
	if request.Subject == "" || request.Message == "" {
		http.Error(w, "subject und message sind erforderlich", http.StatusBadRequest)
		return
	}

	result := sendParzelleInfoMail(parzelleID, request.Subject, request.Message, currentUsername(r))
	if result.Error != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(result)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func ParzelleEmailHistoryHandler(w http.ResponseWriter, r *http.Request) {
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

	logs, err := models.GetEmailLogsByParzelleID(parzelleID)
	if err != nil {
		http.Error(w, "Fehler beim Laden des Mailverlaufs", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logs,
	})
}

func sendParzelleMail(parzelleID, month, year int, subject, message string, attachmentTypes []string, createdBy string) parzelleEmailResult {
	result := parzelleEmailResult{ParzelleID: parzelleID, Status: "failed"}

	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		result.Error = "Parzelle nicht gefunden"
		return result
	}
	result.Recipient = parzelle.Email

	if parzelle.Email == "" {
		result.Error = "Keine E-Mail-Adresse für Parzelle hinterlegt"
		_ = logEmail(parzelleID, "", subject, message, attachmentTypes, "failed", result.Error, createdBy)
		return result
	}

	attachmentTypes = normalizeAttachmentTypes(attachmentTypes)
	if len(attachmentTypes) == 0 {
		result.Error = "Keine Anhänge ausgewählt"
		_ = logEmail(parzelleID, parzelle.Email, subject, message, attachmentTypes, "failed", result.Error, createdBy)
		return result
	}

	settings, err := models.GetOrganizationSettingsWithSecrets()
	if err != nil {
		result.Error = "Mail-Konfiguration konnte nicht geladen werden"
		_ = logEmail(parzelleID, parzelle.Email, subject, message, attachmentTypes, "failed", result.Error, createdBy)
		return result
	}

	attachments, attachmentLabels, err := buildAttachments(parzelleID, month, year, subject, message, attachmentTypes)
	if err != nil {
		result.Error = err.Error()
		_ = logEmail(parzelleID, parzelle.Email, subject, message, attachmentTypes, "failed", result.Error, createdBy)
		return result
	}

	htmlBody, textBody := buildMailBody(parzelle, settings, subject, message, month, year, attachmentLabels)
	mailService := services.NewEmailService()
	err = mailService.SendMail(settings, services.MailRequest{
		To:          []string{parzelle.Email},
		Subject:     subject,
		HTMLBody:    htmlBody,
		TextBody:    textBody,
		Attachments: attachments,
	})
	if err != nil {
		result.Error = err.Error()
		_ = logEmail(parzelleID, parzelle.Email, subject, message, attachmentTypes, "failed", result.Error, createdBy)
		return result
	}

	result.Status = "sent"
	_ = logEmail(parzelleID, parzelle.Email, subject, message, attachmentTypes, "sent", "", createdBy)
	return result
}

func sendParzelleInfoMail(parzelleID int, subject, message, createdBy string) parzelleEmailResult {
	result := parzelleEmailResult{ParzelleID: parzelleID, Status: "failed"}

	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		result.Error = "Parzelle nicht gefunden"
		return result
	}
	result.Recipient = parzelle.Email

	if parzelle.Email == "" {
		result.Error = "Keine E-Mail-Adresse für Parzelle hinterlegt"
		_ = logEmail(parzelleID, "", subject, message, []string{}, "failed", result.Error, createdBy)
		return result
	}

	settings, err := models.GetOrganizationSettingsWithSecrets()
	if err != nil {
		result.Error = "Mail-Konfiguration konnte nicht geladen werden"
		_ = logEmail(parzelleID, parzelle.Email, subject, message, []string{}, "failed", result.Error, createdBy)
		return result
	}

	htmlBody, textBody := buildInfoMailBody(parzelle, settings, subject, message)
	mailService := services.NewEmailService()
	err = mailService.SendMail(settings, services.MailRequest{
		To:       []string{parzelle.Email},
		Subject:  subject,
		HTMLBody: htmlBody,
		TextBody: textBody,
	})
	if err != nil {
		result.Error = err.Error()
		_ = logEmail(parzelleID, parzelle.Email, subject, message, []string{}, "failed", result.Error, createdBy)
		return result
	}

	result.Status = "sent"
	_ = logEmail(parzelleID, parzelle.Email, subject, message, []string{}, "sent", "", createdBy)
	return result
}

func buildAttachments(parzelleID, month, year int, subject, message string, attachmentTypes []string) ([]services.MailAttachment, []string, error) {
	parzelle, err := models.GetParzelleByID(parzelleID)
	if err != nil {
		return nil, nil, err
	}
	org, err := models.GetOrganizationSettings()
	if err != nil {
		return nil, nil, err
	}

	attachments := make([]services.MailAttachment, 0, len(attachmentTypes))
	labels := make([]string, 0, len(attachmentTypes))

	for _, attachmentType := range attachmentTypes {
		switch attachmentType {
		case "info":
			pdfBytes, err := services.GenerateInfoPDF(parzelle, org, subject, message)
			if err != nil {
				return nil, nil, err
			}
			attachments = append(attachments, services.MailAttachment{Filename: fmt.Sprintf("info_parzelle_%s.pdf", parzelle.Nummer), ContentType: "application/pdf", Data: pdfBytes})
			labels = append(labels, "Info")
		case "wasser":
			data, err := services.LoadInvoiceDocumentData(parzelleID, month, year, "wasser")
			if err != nil {
				return nil, nil, err
			}
			if data.Wasser == nil {
				return nil, nil, fmt.Errorf("keine Wasserrechnung für %02d/%d vorhanden", month, year)
			}
			pdfBytes, err := services.GenerateInvoicePDF(data)
			if err != nil {
				return nil, nil, err
			}
			attachments = append(attachments, services.MailAttachment{Filename: fmt.Sprintf("wasserrechnung_%s_%02d_%d.pdf", parzelle.Nummer, month, year), ContentType: "application/pdf", Data: pdfBytes})
			labels = append(labels, "Wasserrechnung")
		case "strom":
			data, err := services.LoadInvoiceDocumentData(parzelleID, month, year, "strom")
			if err != nil {
				return nil, nil, err
			}
			if data.Strom == nil {
				return nil, nil, fmt.Errorf("keine Stromrechnung für %02d/%d vorhanden", month, year)
			}
			pdfBytes, err := services.GenerateInvoicePDF(data)
			if err != nil {
				return nil, nil, err
			}
			attachments = append(attachments, services.MailAttachment{Filename: fmt.Sprintf("stromrechnung_%s_%02d_%d.pdf", parzelle.Nummer, month, year), ContentType: "application/pdf", Data: pdfBytes})
			labels = append(labels, "Stromrechnung")
		case "inspektion":
			inspektion, err := models.GetInspektionByParzelleID(parzelleID)
			if err != nil {
				return nil, nil, fmt.Errorf("kein Inspektionsprotokoll vorhanden")
			}
			pdfBytes, err := services.GenerateInspektionPDFBytes(inspektion.ID)
			if err != nil {
				return nil, nil, err
			}
			attachments = append(attachments, services.MailAttachment{Filename: fmt.Sprintf("inspektion_%s.pdf", parzelle.Nummer), ContentType: "application/pdf", Data: pdfBytes})
			labels = append(labels, "Inspektion")
		case "wertermittlung":
			wertermittlung, err := models.GetWertermittlungByParzelleID(parzelleID)
			if err != nil {
				return nil, nil, fmt.Errorf("kein Wertermittlungsprotokoll vorhanden")
			}
			pdfBytes, err := services.GenerateWertermittlungPDFBytes(wertermittlung.ID)
			if err != nil {
				return nil, nil, err
			}
			attachments = append(attachments, services.MailAttachment{Filename: fmt.Sprintf("wertermittlung_%s.pdf", parzelle.Nummer), ContentType: "application/pdf", Data: pdfBytes})
			labels = append(labels, "Wertermittlung")
		}
	}

	return attachments, labels, nil
}

func buildMailBody(parzelle *models.Parzelle, org *models.OrganizationSettings, subject, message string, month, year int, labels []string) (string, string) {
	escapedMessage := strings.ReplaceAll(htmltemplate.HTMLEscapeString(message), "\n", "<br>")
	attachmentList := ""
	for _, label := range labels {
		attachmentList += fmt.Sprintf("<li>%s</li>", htmltemplate.HTMLEscapeString(label))
	}

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="de">
<body style="font-family: Arial, sans-serif; line-height: 1.5; color: #1f2937;">
  <h2>%s</h2>
  <p>Hallo %s,</p>
  <p>für Ihre Parzelle %s wurden neue Unterlagen bereitgestellt.</p>
  <p><strong>Zeitraum:</strong> %02d/%d</p>
  <p>%s</p>
  <p><strong>Anhänge:</strong></p>
  <ul>%s</ul>
  <p>Mit freundlichen Grüßen<br>%s</p>
</body>
</html>`, htmltemplate.HTMLEscapeString(subject), htmltemplate.HTMLEscapeString(parzelle.PaechterName), htmltemplate.HTMLEscapeString(parzelle.Nummer), month, year, escapedMessage, attachmentList, htmltemplate.HTMLEscapeString(org.Name))

	textBody := fmt.Sprintf("%s\n\nHallo %s,\n\nfür Ihre Parzelle %s wurden neue Unterlagen bereitgestellt.\nZeitraum: %02d/%d\n\n%s\n\nAnhänge: %s\n\nMit freundlichen Grüßen\n%s",
		subject, parzelle.PaechterName, parzelle.Nummer, month, year, message, strings.Join(labels, ", "), org.Name)

	return htmlBody, textBody
}

func buildInfoMailBody(parzelle *models.Parzelle, org *models.OrganizationSettings, subject, message string) (string, string) {
	escapedMessage := strings.ReplaceAll(htmltemplate.HTMLEscapeString(message), "\n", "<br>")

	htmlBody := fmt.Sprintf(`<!DOCTYPE html>
<html lang="de">
<body style="font-family: Arial, sans-serif; line-height: 1.5; color: #1f2937;">
  <h2>%s</h2>
  <p>Hallo %s,</p>
  <p>%s</p>
  <p>Mit freundlichen Grüßen<br>%s</p>
</body>
</html>`, htmltemplate.HTMLEscapeString(subject), htmltemplate.HTMLEscapeString(parzelle.PaechterName), escapedMessage, htmltemplate.HTMLEscapeString(org.Name))

	textBody := fmt.Sprintf("%s\n\nHallo %s,\n\n%s\n\nMit freundlichen Grüßen\n%s", subject, parzelle.PaechterName, message, org.Name)

	return htmlBody, textBody
}

func normalizeAttachmentTypes(values []string) []string {
	allowed := map[string]bool{"info": true, "wasser": true, "strom": true, "inspektion": true, "wertermittlung": true}
	seen := make(map[string]bool)
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if !allowed[value] || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func currentUsername(r *http.Request) string {
	session := middleware.GetSessionFromContext(r.Context())
	if session == nil {
		return "system"
	}
	return session.Username
}

func logEmail(parzelleID int, recipient, subject, message string, attachmentTypes []string, status, errorMessage, createdBy string) error {
	log := models.EmailLog{
		ParzelleID:      parzelleID,
		Recipient:       recipient,
		Subject:         subject,
		Message:         message,
		AttachmentTypes: strings.Join(attachmentTypes, ","),
		Status:          status,
		ErrorMessage:    errorMessage,
		CreatedBy:       createdBy,
	}
	return log.Save()
}
