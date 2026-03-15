package handlers

import (
	"html/template"
	"net/http"
	"strconv"
	"time"

	"kleingarten-verwaltung/middleware"
	"kleingarten-verwaltung/models"

	"github.com/gorilla/mux"
)

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/parzellen", http.StatusSeeOther)
}

func ParzellenListHandler(w http.ResponseWriter, r *http.Request) {
	parzellen, err := models.GetAllParzellen()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Session aus Context laden
	session := middleware.GetSessionFromContext(r.Context())

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/parzellen.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":     "Parzellenverwaltung",
		"Parzellen": parzellen,
		"Session":   session, // NEU: Session für Layout
	})
}

func ParzelleNeuHandler(w http.ResponseWriter, r *http.Request) {
	// Session aus Context laden
	session := middleware.GetSessionFromContext(r.Context())

	if r.Method == "POST" {
		parzelle := models.Parzelle{
			Nummer:       r.FormValue("nummer"),
			Verein:       r.FormValue("verein"),
			PaechterName: r.FormValue("paechter_name"),
			Email:        r.FormValue("email"),   // NEU
			Telefon:      r.FormValue("telefon"), // NEU
			Notizen:      r.FormValue("notizen"), // NEU
		}

		if groesse := r.FormValue("groesse"); groesse != "" {
			if g, err := strconv.ParseFloat(groesse, 64); err == nil {
				parzelle.Groesse = g
			}
		}

		if kuendigung := r.FormValue("kuendigung_datum"); kuendigung != "" {
			if t, err := time.Parse("2006-01-02", kuendigung); err == nil {
				parzelle.KuendigungDatum = &t
			}
		}

		if err := parzelle.Save(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/parzellen", http.StatusSeeOther)
		return
	}

	// GET Request - Formular anzeigen
	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/parzelle_neu.html"))
	tmpl.Execute(w, map[string]interface{}{
		"Title":   "Neue Parzelle",
		"Session": session, // NEU: Session für Layout
	})
}

func ParzelleDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	parzelle, err := models.GetParzelleByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Session aus Context laden
	session := middleware.GetSessionFromContext(r.Context())

	// Aktuelle Inspektion und Wertermittlung laden
	inspektion, _ := models.GetInspektionByParzelleID(id)
	wertermittlung, _ := models.GetWertermittlungByParzelleID(id)

	// Template-Funktionen definieren (für parzelle_detail.html)
	funcMap := template.FuncMap{
		"add": func(a, b float64) float64 {
			return a + b
		},
		"sub": func(a, b float64) float64 {
			return a - b
		},
	}

	tmpl := template.Must(LoadTemplateWithFuncs(funcMap, "templates/layout.html", "templates/parzelle_detail.html"))
	err = tmpl.ExecuteTemplate(w, "layout.html", map[string]interface{}{
		"Title":          "Parzelle " + parzelle.Nummer,
		"Parzelle":       parzelle,
		"Inspektion":     inspektion,
		"Wertermittlung": wertermittlung,
		"Session":        session, // NEU: Session für Layout
	})

	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
