package handlers

import (
	"github.com/bl0rb/gartenamt/middleware"
	"github.com/bl0rb/gartenamt/models"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
)

// LoadTemplate loads a template from embedded files
func LoadTemplate(filenames ...string) (*template.Template, error) {
	return LoadTemplateWithFuncs(nil, filenames...)
}

// baseTemplateFuncMap liefert die Funcs, die in JEDEM Template-Set verfuegbar
// sein muessen (Sichtbarkeit=Berechtigung, ein Vokabular fuer Rollen). Sie
// spiegeln direkt die Berechtigungs-/Rollenlogik aus models/user.go, damit
// Templates nie eigene Kopien dieser Entscheidungen pflegen.
func baseTemplateFuncMap() template.FuncMap {
	return template.FuncMap{
		"hasPerm": func(role, permission string) bool {
			return models.RoleHasPermission(role, permission)
		},
		"roleLabel": func(role string) string {
			return models.RoleDisplayName(role)
		},
		"roleBadgeClass": func(role string) string {
			return models.RoleBadgeClass(role)
		},
	}
}

// LoadTemplateWithFuncs loads a template with custom functions. Die
// Basis-Funcs (hasPerm, roleLabel, roleBadgeClass) stehen dabei immer zur
// Verfuegung; ein per Handler uebergebenes funcMap wird obendrauf gemergt und
// kann bei Namensgleichheit die Basis-Funcs ueberschreiben.
func LoadTemplateWithFuncs(funcMap template.FuncMap, filenames ...string) (*template.Template, error) {
	if len(filenames) == 0 {
		return nil, nil
	}

	// Get the embedded filesystem from global context
	embeddedFS := GetEmbeddedFS()
	if embeddedFS == nil {
		return nil, nil
	}

	// Read the main file first
	mainFile := filenames[0]
	mainContent, err := fs.ReadFile(embeddedFS, mainFile)
	if err != nil {
		return nil, err
	}

	// Create template with the main file name and functions
	tmpl := template.New(filepath.Base(mainFile))
	merged := baseTemplateFuncMap()
	for name, fn := range funcMap {
		merged[name] = fn
	}
	tmpl = tmpl.Funcs(merged)

	// Parse the content
	tmpl, err = tmpl.Parse(string(mainContent))
	if err != nil {
		return nil, err
	}

	// Parse additional files
	for _, filename := range filenames[1:] {
		content, err := fs.ReadFile(embeddedFS, filename)
		if err != nil {
			return nil, err
		}
		tmpl, err = tmpl.Parse(string(content))
		if err != nil {
			return nil, err
		}
	}

	return tmpl, nil
}

// Get/Set embedded filesystem (set from main.go)
var embeddedFSGlobal fs.FS

func SetEmbeddedFS(fsys fs.FS) {
	embeddedFSGlobal = fsys
}

func GetEmbeddedFS() fs.FS {
	return embeddedFSGlobal
}

// GetEmbeddedStaticFS returns the embedded static filesystem
func GetEmbeddedStaticFS() fs.FS {
	fsys := GetEmbeddedFS()
	if fsys == nil {
		return nil
	}
	staticFS, err := fs.Sub(fsys, "static")
	if err != nil {
		return nil
	}
	return staticFS
}

// AddSessionToData copies session from context into your template data map.

func AddSessionToData(r *http.Request, data map[string]interface{}) map[string]interface{} {
	session := middleware.GetSessionFromContext(r.Context())
	data["Session"] = session

	if session != nil {
		data["IsBackoffice"] = models.IsBackofficeRole(session.Role)
		data["RoleLabel"] = models.RoleDisplayName(session.Role)
	}

	data["IsAdminPage"] = strings.HasPrefix(r.URL.Path, "/admin")

	section := ""
	switch {
	case strings.HasPrefix(r.URL.Path, "/admin/obstarten"):
		section = "obstarten"
	case strings.HasPrefix(r.URL.Path, "/admin/zieranpflanzungen"):
		section = "zieranpflanzungen"
	case strings.HasPrefix(r.URL.Path, "/admin/bauindex"):
		section = "bauindex"
	case strings.HasPrefix(r.URL.Path, "/admin/verwaltung"):
		section = "verwaltung"
	case strings.HasPrefix(r.URL.Path, "/admin/users"):
		section = "users"
	case strings.HasPrefix(r.URL.Path, "/admin/parzellenstruktur"):
		section = "parzellenstruktur"
	case strings.HasPrefix(r.URL.Path, "/admin/backup"):
		section = "backup"
	case strings.HasPrefix(r.URL.Path, "/admin/audit-log"):
		section = "audit"
	case strings.HasPrefix(r.URL.Path, "/admin/system-info"):
		section = "system"
	}

	data["AdminSection"] = section

	// NavSection steuert den aktiven Zustand in der Top-Nav (layout.html).
	// Eigentümer-Entscheidung 26.08.2026: die fachlichen Bereiche liegen
	// top-level, /admin ist nur noch technische + strukturelle Administration
	// - die Top-Nav braucht darum eine eigene, von AdminSection unabhängige
	// Ableitung aus dem URL-Pfad.
	navSection := ""
	switch {
	case strings.HasPrefix(r.URL.Path, "/dashboard"):
		navSection = "dashboard"
	case strings.HasPrefix(r.URL.Path, "/parzellen"),
		strings.HasPrefix(r.URL.Path, "/inspektion"),
		strings.HasPrefix(r.URL.Path, "/wertermittlung"),
		// "/protokoll/" mit Slash, nicht bloß "/protokoll" - sonst würde das
		// auch die eigenständige Übersicht /protokolle treffen (Punkt 10),
		// die kein Unterpfad von /parzellen ist und darum keinen aktiven
		// Nav-Zustand erzeugen soll.
		strings.HasPrefix(r.URL.Path, "/protokoll/"),
		strings.HasPrefix(r.URL.Path, "/wasser"),
		strings.HasPrefix(r.URL.Path, "/strom"):
		navSection = "parzellen"
	case strings.HasPrefix(r.URL.Path, "/abrechnung"):
		navSection = "abrechnung"
	case strings.HasPrefix(r.URL.Path, "/admin"):
		navSection = "admin"
	}

	data["NavSection"] = navSection
	return data
}
