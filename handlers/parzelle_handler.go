package handlers

import (
	"errors"
	"html/template"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/bl0rb/gartenamt/middleware"
	"github.com/bl0rb/gartenamt/models"

	"github.com/gorilla/mux"
	"github.com/mattn/go-sqlite3"
)

var plzPattern = regexp.MustCompile(`^\d+$`)
var errInvalidPLZ = errors.New("invalid plz")
var errInvalidEmail = errors.New("invalid email")

// init registriert RenderErrorPage bei der middleware, damit dortige
// http.Error-Pfade (403 bei fehlenden Berechtigungen) über dieselbe
// Fehlerseite im Layout laufen. Die Zuweisung passiert hier statt in
// middleware, weil middleware bereits von handlers importiert wird - ein
// Rückimport middleware -> handlers wäre ein Zyklus.
func init() {
	middleware.ErrorRenderer = RenderErrorPage
}

// RenderErrorPage rendert templates/error.html im Standard-Layout (Titel,
// Meldung, Zurück-Link) - für 403/404/500 und alle sonstigen Fehlerpfade, die
// keinen rohen Plaintext mehr ausgeben sollen. AddSessionToData füllt Session/
// IsBackoffice korrekt, auch wenn kein Nutzer eingeloggt ist (Session=nil).
func RenderErrorPage(w http.ResponseWriter, r *http.Request, status int, title, message, backURL string) {
	if backURL == "" {
		backURL = "/"
	}

	tmpl, err := LoadTemplate("templates/layout.html", "templates/error.html")
	if err != nil || tmpl == nil {
		// Fallback, falls die Templates nicht geladen werden können - besser
		// eine Plaintext-Meldung als eine leere/kaputte Antwort.
		http.Error(w, message, status)
		return
	}

	w.WriteHeader(status)
	_ = tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":    title,
		"ErrTitle": title,
		"Message":  message,
		"BackURL":  backURL,
	}))
}

// isUniqueConstraintViolation erkennt eine SQLite-UNIQUE-Verletzung (z.B.
// doppelte Parzellennummer), damit der Handler dafür eine sprechende Meldung
// statt der rohen DB-Fehlermeldung zeigen kann.
func isUniqueConstraintViolation(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code == sqlite3.ErrConstraint && sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
	}
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}

// parzelleSaveErrorMessage übersetzt einen Speicherfehler in eine für die
// Oberfläche verständliche Meldung, ohne die rohe DB-Fehlermeldung zu zeigen.
func parzelleSaveErrorMessage(err error) string {
	if isUniqueConstraintViolation(err) {
		return "Parzellennummer bereits vergeben."
	}
	return "Die Parzelle konnte nicht gespeichert werden. Bitte versuchen Sie es erneut."
}

// parzelleFormErrorMessage übersetzt die Validierungsfehler des Formulars in
// eine Meldung für die Oberfläche.
func parzelleFormErrorMessage(err error) string {
	switch {
	case errors.Is(err, errInvalidEmail):
		return "Die E-Mail-Adresse ist ungültig."
	case errors.Is(err, errInvalidPLZ):
		return "Die PLZ darf nur aus Zahlen bestehen."
	default:
		return "Die Eingaben sind unvollständig oder ungültig."
	}
}

// parzellenBasePath liefert den Basis-Pfad für Parzellen-Links im Template.
// Eigentümer-Entscheidung 23.08.2026: Der Parzellen-URL-Raum ist auf
// /admin/parzellen konsolidiert - die alten /parzellen*-Routen sind in main.go
// nur noch 301-Redirects auf ihr /admin/parzellen-Pendant. Es gibt damit keinen
// Nicht-Admin-Scope mehr zu unterscheiden; der Parameter bleibt erhalten, damit
// die bestehenden Aufrufer (hier und in inspektion_handler.go,
// wertermittlung_handler.go) unverändert bleiben.
func parzellenBasePath(r *http.Request) string {
	return "/admin/parzellen"
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

	// Die Adresse wird später ungeprüft als SMTP-Empfänger verwendet - bislang
	// wurde die PLZ strenger geprüft als die E-Mail.
	if parzelle.Email != "" {
		address, err := mail.ParseAddress(parzelle.Email)
		if err != nil || address.Address != parzelle.Email {
			return errInvalidEmail
		}
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
		RenderErrorPage(w, r, http.StatusInternalServerError, "Es ist ein Fehler aufgetreten", "Die Parzellen konnten nicht geladen werden.", "/")
		return
	}

	tmpl := template.Must(LoadTemplate("templates/layout.html", "templates/parzellen.html"))
	tmpl.Execute(w, AddSessionToData(r, map[string]interface{}{
		"Title":             "Parzellenverwaltung",
		"Parzellen":         parzellen,
		"ParzellenBasePath": parzellenBasePath(r),
		// Eigentümer-Entscheidung 23.08.2026: Seit der URL-Konsolidierung auf
		// /admin/parzellen gibt es keinen eigenen Admin-Scope mehr zu erkennen
		// (isAdminParzellenScope wäre jetzt immer wahr). Der Feldname bleibt aus
		// Kompatibilität mit templates/parzellen.html erhalten, gated aber jetzt
		// direkt auf die Permission der Zielroute /admin/parzellen/verwalten
		// ("parzellen.manage") statt auf den (nicht mehr existenten) URL-Scope.
		"IsAdminParzellen": middleware.HasPermission(r, "parzellen.manage"),
	}))
}

func ParzelleNeuHandler(w http.ResponseWriter, r *http.Request) {
	vereinName, err := getConfiguredVerein()
	if err != nil {
		RenderErrorPage(w, r, http.StatusInternalServerError, "Es ist ein Fehler aufgetreten", "Die Vereinseinstellungen konnten nicht geladen werden.", parzellenBasePath(r))
		return
	}

	if vereinName == "" {
		renderParzelleForm(w, r, "Neue Parzelle", models.Parzelle{}, false, "Bitte zuerst in der Verwaltung einen Vereinsnamen speichern.")
		return
	}

	if r.Method == "POST" {
		parzelle := models.Parzelle{}
		if err := populateParzelleFromForm(&parzelle, r); err != nil {
			renderParzelleForm(w, r, "Neue Parzelle", parzelle, false, parzelleFormErrorMessage(err))
			return
		}

		parzelle.Verein = vereinName

		if err := parzelle.Save(); err != nil {
			// DB-Fehler (z.B. doppelte Parzellennummer) darf das Formular nicht
			// mit http.Error zerstören - stattdessen erneut mit Meldung und
			// allen eingegebenen Werten rendern.
			renderParzelleForm(w, r, "Neue Parzelle", parzelle, false, parzelleSaveErrorMessage(err))
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
		RenderErrorPage(w, r, http.StatusNotFound, "Nicht gefunden", "Diese Parzelle wurde nicht gefunden.", parzellenBasePath(r))
		return
	}

	if r.Method == "POST" {
		if err := populateParzelleFromForm(parzelle, r); err != nil {
			renderParzelleForm(w, r, "Parzelle bearbeiten", *parzelle, true, parzelleFormErrorMessage(err))
			return
		}

		if strings.TrimSpace(parzelle.Verein) == "" {
			if vereinName, cfgErr := getConfiguredVerein(); cfgErr == nil && vereinName != "" {
				parzelle.Verein = vereinName
			}
		}

		if saveErr := parzelle.Save(); saveErr != nil {
			// DB-Fehler (z.B. doppelte Parzellennummer) darf das Formular nicht
			// mit http.Error zerstören - stattdessen erneut mit Meldung und
			// allen eingegebenen Werten rendern.
			renderParzelleForm(w, r, "Parzelle bearbeiten", *parzelle, true, parzelleSaveErrorMessage(saveErr))
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
		RenderErrorPage(w, r, http.StatusNotFound, "Nicht gefunden", "Diese Parzelle wurde nicht gefunden.", parzellenBasePath(r))
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
		RenderErrorPage(w, r, http.StatusInternalServerError, "Es ist ein Fehler aufgetreten", "Die Seite konnte nicht dargestellt werden.", parzellenBasePath(r))
		return
	}
}
