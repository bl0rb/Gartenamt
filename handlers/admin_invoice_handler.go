package handlers

import (
	"html/template"
	"log"
	"net/http"

	"github.com/bl0rb/gartenamt/middleware"
)

// AdminInvoiceManagementHandler displays the invoice management dashboard.
//
// Eigentümer-Entscheidung 26.08.2026 (ersetzt die Konsolidierung vom
// 23.08.2026): Abrechnung ist ein fachlicher Top-Level-Bereich (Route
// /abrechnung, main.go) statt einer Admin-Unterseite - der Handler setzt
// darum kein AdminSection mehr, die Sidebar entfällt für diese Seite
// (IsAdminPage wird in AddSessionToData rein aus dem URL-Pfad abgeleitet).
func AdminInvoiceManagementHandler(w http.ResponseWriter, r *http.Request) {
	if !middleware.HasPermission(r, "invoices.manage") {
		http.Error(w, "Zugriff verweigert - Berechtigung fehlt", http.StatusForbidden)
		return
	}

	log.Println("📋 Invoice management dashboard accessed")

	// Render the invoice management template
	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/invoice_management.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title": "Abrechnung",
	}))
}
