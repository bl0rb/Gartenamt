package handlers

import (
	"errors"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kleingarten-verwaltung/middleware"
	"kleingarten-verwaltung/models"

	"github.com/gorilla/mux"
)

var plzPattern = regexp.MustCompile(`^\d+$`)
var errInvalidPLZ = errors.New("invalid plz")

func isAdminParzellenScope(r *http.Request) bool {
	return strings.HasPrefix(r.URL.Path, "/admin/parzellen")
}

func parzellenBasePath(r *http.Request) string {
	if isAdminParzellenScope(r) {
		return "/admin/parzellen"
	}
	return "/parzellen"
}

func populateParzelleFromForm(parzelle *models.Parzelle, r *http.Request) error {
	parzelle.Nummer = strings.TrimSpace(r.FormValue("nummer"))
	parzelle.Verein = strings.TrimSpace(r.FormValue("verein"))
	parzelle.PaechterName = strings.TrimSpace(r.FormValue("paechter_name"))
	parzelle.Email = strings.TrimSpace(r.FormValue("email"))
	parzelle.Telefon = strings.TrimSpace(r.FormValue("telefon"))
	parzelle.PaechterStrasse = strings.TrimSpace(r.FormValue("paechter_strasse"))
	parzelle.PaechterHausnr = strings.TrimSpace(r.FormValue("paechter_hausnr"))
	parzelle.PaechterPLZ = strings.TrimSpace(r.FormValue("paechter_plz"))
	parzelle.PaechterOrt = strings.TrimSpace(r.FormValue("paechter_ort"))
	parzelle.Notizen = strings.TrimSpace(r.FormValue("notizen"))

	if parzelle.PaechterPLZ != "" && !plzPattern.MatchString(parzelle.PaechterPLZ) {
		return errInvalidPLZ
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
	} else {
		parzelle.KuendigungDatum = nil
	}

	return nil
}

func getConfiguredVerein() (string, error) {
	org, err := models.GetOrganizationSettings()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(org.Name), nil
}

func renderParzelleForm(w http.ResponseWriter, r *http.Request, title string, parzelle models.Parzelle, isEdit bool, errMsg string) {
	organizationName, _ := getConfiguredVerein()
	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/parzelle_neu.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":             title,
		"Parzelle":          parzelle,
		"IsEdit":            isEdit,
		"Error":             errMsg,
		"OrganizationName":  organizationName,
		"ParzellenBasePath": parzellenBasePath(r),
	}))
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromContext(r.Context())
	if session != nil && models.IsBackofficeRole(session.Role) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/profile", http.StatusSeeOther)
}

func ParzellenListHandler(w http.ResponseWriter, r *http.Request) {
	parzellen, err := models.GetAllParzellen()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/parzellen.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":             "Parzellenverwaltung",
		"Parzellen":         parzellen,
		"ParzellenBasePath": parzellenBasePath(r),
		"IsAdminParzellen":  isAdminParzellenScope(r),
	}))
}

func ParzelleNeuHandler(w http.ResponseWriter, r *http.Request) {
	vereinName, err := getConfiguredVerein()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if vereinName == "" {
		renderParzelleForm(w, r, "Neue Parzelle", models.Parzelle{}, false, "Bitte zuerst in der Verwaltung einen Vereinsnamen speichern.")
		return
	}

	if r.Method == "POST" {
		parzelle := models.Parzelle{}
		if err := populateParzelleFromForm(&parzelle, r); err != nil {
			renderParzelleForm(w, r, "Neue Parzelle", parzelle, false, "Die PLZ darf nur aus Zahlen bestehen.")
			return
		}

		parzelle.Verein = vereinName

		if err := parzelle.Save(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, parzellenBasePath(r), http.StatusSeeOther)
		return
	}

	renderParzelleForm(w, r, "Neue Parzelle", models.Parzelle{Verein: vereinName}, false, "")
}

func ParzelleEditHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	parzelle, err := models.GetParzelleByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if r.Method == "POST" {
		if err := populateParzelleFromForm(parzelle, r); err != nil {
			renderParzelleForm(w, r, "Parzelle bearbeiten", *parzelle, true, "Die PLZ darf nur aus Zahlen bestehen.")
			return
		}

		if strings.TrimSpace(parzelle.Verein) == "" {
			if vereinName, cfgErr := getConfiguredVerein(); cfgErr == nil && vereinName != "" {
				parzelle.Verein = vereinName
			}
		}

		if saveErr := parzelle.Save(); saveErr != nil {
			http.Error(w, saveErr.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, parzellenBasePath(r)+"/"+strconv.Itoa(parzelle.ID), http.StatusSeeOther)
		return
	}

	renderParzelleForm(w, r, "Parzelle bearbeiten", *parzelle, true, "")
}

func ParzelleDetailHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, _ := strconv.Atoi(vars["id"])

	parzelle, err := models.GetParzelleByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

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
	err = tmpl.ExecuteTemplate(w, "layout.html", AddSessionToData(r, map[string]interface{}{
		"Title":             "Parzelle " + parzelle.Nummer,
		"Parzelle":          parzelle,
		"Inspektion":        inspektion,
		"Wertermittlung":    wertermittlung,
		"ParzellenBasePath": parzellenBasePath(r),
	}))

	if err != nil {
		http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
