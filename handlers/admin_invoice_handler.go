package handlers

import (
	"html/template"
	"log"
	"net/http"

	"github.com/bl0rb/gartenamt/middleware"
)

// AdminInvoiceManagementHandler displays the invoice management dashboard
func AdminInvoiceManagementHandler(w http.ResponseWriter, r *http.Request) {
	if !middleware.IsAdmin(r) {
		http.Error(w, "Unauthorized", http.StatusForbidden)
		return
	}

	log.Println("📋 Invoice management dashboard accessed")

	// Render the invoice management template
	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/invoice_management.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title": "Rechnung & Gebührenverwaltung",
	}))
}
