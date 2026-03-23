package handlers

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"kleingarten-verwaltung/models"

	"github.com/gorilla/mux"
)

// AdminDashboardHandler - Übersicht aller Admin-Funktionen
func AdminDashboardHandler(w http.ResponseWriter, r *http.Request) {
	// Statistiken für Dashboard laden
	stats := getAdminStatistiken()

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/admin_dashboard.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title": "Verwaltung",
		"Stats": stats,
	}))
}

// AdminVerwaltungHandler shows central organization and mail server settings.
func AdminVerwaltungHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/admin_verwaltung.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title": "Vereins- und Maileinstellungen",
	}))
}

// AdminObstartenHandler - CRUD für Obstarten
func AdminObstartenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Neue Obstart hinzufügen/bearbeiten
		if err := handleObstartenPost(r); err != nil {
			http.Error(w, "Fehler beim Speichern: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/obstarten", http.StatusSeeOther)
		return
	}

	// GET - Alle Obstarten anzeigen
	obstarten, err := models.GetAllObstarten()
	if err != nil {
		http.Error(w, "Fehler beim Laden der Obstarten: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/admin_obstarten.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":     "Obstarten verwalten",
		"Obstarten": obstarten,
	}))
}

// AdminZieranpflanzungenHandler - CRUD für Zieranpflanzungen
func AdminZieranpflanzungenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Neue Zieranpflanzung hinzufügen/bearbeiten
		if err := handleZieranpflanzungenPost(r); err != nil {
			http.Error(w, "Fehler beim Speichern: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/zieranpflanzungen", http.StatusSeeOther)
		return
	}

	// GET - Alle Zieranpflanzungen anzeigen
	zieranpflanzungen, err := models.GetAllZieranpflanzungen()
	if err != nil {
		http.Error(w, "Fehler beim Laden der Zieranpflanzungen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/admin_zieranpflanzungen.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":             "Zieranpflanzungen verwalten",
		"Zieranpflanzungen": zieranpflanzungen,
	}))
}

// AdminObstartenLoeschenHandler - Einzelne Obstart löschen
func AdminObstartenLoeschenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Nur POST erlaubt", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Ungültige ID", http.StatusBadRequest)
		return
	}

	// Obstart deaktivieren (soft delete)
	query := `UPDATE obstarten SET aktiv = FALSE WHERE id = ?`
	_, err = models.DB.Exec(query, id)
	if err != nil {
		http.Error(w, "Fehler beim Löschen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/obstarten?success=deleted", http.StatusSeeOther)
}

// AdminZieranpflanzungenLoeschenHandler - Einzelne Zieranpflanzung löschen
func AdminZieranpflanzungenLoeschenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Nur POST erlaubt", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Ungültige ID", http.StatusBadRequest)
		return
	}

	// Zieranpflanzung deaktivieren (soft delete)
	query := `UPDATE zieranpflanzungen SET aktiv = FALSE WHERE id = ?`
	_, err = models.DB.Exec(query, id)
	if err != nil {
		http.Error(w, "Fehler beim Löschen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/zieranpflanzungen?success=deleted", http.StatusSeeOther)
}

// Hilfsfunktionen

func getAdminStatistiken() map[string]interface{} {
	stats := make(map[string]interface{})

	// Anzahl Obstarten
	var anzahlObstarten int
	models.DB.QueryRow("SELECT COUNT(*) FROM obstarten WHERE aktiv = TRUE").Scan(&anzahlObstarten)
	stats["AnzahlObstarten"] = anzahlObstarten

	// Anzahl Zieranpflanzungen
	var anzahlZieranpflanzungen int
	models.DB.QueryRow("SELECT COUNT(*) FROM zieranpflanzungen WHERE aktiv = TRUE").Scan(&anzahlZieranpflanzungen)
	stats["AnzahlZieranpflanzungen"] = anzahlZieranpflanzungen

	// Anzahl Parzellen
	var anzahlParzellen int
	models.DB.QueryRow("SELECT COUNT(*) FROM parzellen").Scan(&anzahlParzellen)
	stats["AnzahlParzellen"] = anzahlParzellen

	// Anzahl Wertermittlungen
	var anzahlWertermittlungen int
	models.DB.QueryRow("SELECT COUNT(*) FROM wertermittlungen").Scan(&anzahlWertermittlungen)
	stats["AnzahlWertermittlungen"] = anzahlWertermittlungen

	return stats
}

func handleObstartenPost(r *http.Request) error {
	// Formulardaten verarbeiten
	name := r.FormValue("name")
	kategorie := r.FormValue("kategorie")
	einheit := r.FormValue("einheit")
	standardpreis, _ := strconv.ParseFloat(r.FormValue("standardpreis"), 64)
	maxAnzahl, _ := strconv.Atoi(r.FormValue("max_anzahl"))
	beschreibung := r.FormValue("beschreibung")

	if r.FormValue("id") != "" {
		// Update bestehende Obstart
		id, _ := strconv.Atoi(r.FormValue("id"))
		query := `UPDATE obstarten SET name=?, kategorie=?, einheit=?, standardpreis=?, 
                  max_anzahl=?, beschreibung=? WHERE id=?`
		_, err := models.DB.Exec(query, name, kategorie, einheit, standardpreis, maxAnzahl, beschreibung, id)
		return err
	} else {
		// Neue Obstart erstellen
		query := `INSERT INTO obstarten (name, kategorie, einheit, standardpreis, max_anzahl, beschreibung) 
                  VALUES (?, ?, ?, ?, ?, ?)`
		_, err := models.DB.Exec(query, name, kategorie, einheit, standardpreis, maxAnzahl, beschreibung)
		return err
	}
}

func handleZieranpflanzungenPost(r *http.Request) error {
	// Formulardaten verarbeiten
	name := r.FormValue("name")
	kategorie := r.FormValue("kategorie")
	preisProQM, _ := strconv.ParseFloat(r.FormValue("preis_pro_qm"), 64)
	beschreibung := r.FormValue("beschreibung")

	var maxFlaeche *int
	if maxFlaecheStr := r.FormValue("max_flaeche"); maxFlaecheStr != "" {
		if mf, err := strconv.Atoi(maxFlaecheStr); err == nil {
			maxFlaeche = &mf
		}
	}

	if r.FormValue("id") != "" {
		// Update bestehende Zieranpflanzung
		id, _ := strconv.Atoi(r.FormValue("id"))
		query := `UPDATE zieranpflanzungen SET name=?, kategorie=?, preis_pro_qm=?, 
                  beschreibung=?, max_flaeche=? WHERE id=?`
		_, err := models.DB.Exec(query, name, kategorie, preisProQM, beschreibung, maxFlaeche, id)
		return err
	} else {
		// Neue Zieranpflanzung erstellen
		query := `INSERT INTO zieranpflanzungen (name, kategorie, preis_pro_qm, beschreibung, max_flaeche) 
                  VALUES (?, ?, ?, ?, ?)`
		_, err := models.DB.Exec(query, name, kategorie, preisProQM, beschreibung, maxFlaeche)
		return err
	}
}

// AdminBauindexHandler - CRUD für Bauindex
func AdminBauindexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Neue/aktualisierte Bauindex hinzufügen/bearbeiten
		if err := handleBauindexPost(r); err != nil {
			http.Error(w, "Fehler beim Speichern: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/bauindex", http.StatusSeeOther)
		return
	}

	// GET - Alle Bauindex-Einträge anzeigen
	bauindexEintraege, err := models.GetAllBauindexEintraege()
	if err != nil {
		http.Error(w, "Fehler beim Laden der Bauindex-Einträge: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/admin_bauindex.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":             "Bauindex verwalten",
		"BauindexEintraege": bauindexEintraege,
	}))
}

// AdminBauindexLoeschenHandler - Einzelnen Bauindex-Eintrag löschen
func AdminBauindexLoeschenHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Nur POST erlaubt", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	jahr, err := strconv.Atoi(vars["jahr"])
	if err != nil {
		http.Error(w, "Ungültige Jahr", http.StatusBadRequest)
		return
	}

	// Bauindex-Eintrag löschen
	err = models.DeleteBauindex(jahr)
	if err != nil {
		http.Error(w, "Fehler beim Löschen: "+err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/admin/bauindex?success=deleted", http.StatusSeeOther)
}

func handleBauindexPost(r *http.Request) error {
	// Formulardaten verarbeiten
	jahr, err := strconv.Atoi(strings.TrimSpace(r.FormValue("jahr")))
	if err != nil || jahr <= 0 {
		return errors.New("ungueltiges Jahr")
	}

	bauindex, err := parseLocalizedFloat(r.FormValue("bauindex"))
	if err != nil || bauindex <= 0 {
		return errors.New("ungueltiger Bauindex")
	}

	// Bauindex erstellen oder aktualisieren
	return models.CreateBauindex(jahr, bauindex)
}

func parseLocalizedFloat(raw string) (float64, error) {
	normalized := strings.TrimSpace(raw)
	normalized = strings.ReplaceAll(normalized, ",", ".")
	return strconv.ParseFloat(normalized, 64)
}
