package handlers

import (
	"html/template"
	"io/fs"
	"kleingarten-verwaltung/middleware"
	"net/http"
	"path/filepath"
	"strings"
)

// LoadTemplate loads a template from embedded files
func LoadTemplate(filenames ...string) (*template.Template, error) {
	return LoadTemplateWithFuncs(nil, filenames...)
}

// LoadTemplateWithFuncs loads a template with custom functions
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
	if funcMap != nil {
		tmpl = tmpl.Funcs(funcMap)
	}

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
	data["Session"] = middleware.GetSessionFromContext(r.Context())
	data["IsAdminPage"] = strings.HasPrefix(r.URL.Path, "/admin")

	section := ""
	switch {
	case r.URL.Path == "/admin":
		section = "dashboard"
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
	case strings.HasPrefix(r.URL.Path, "/admin/invoices"):
		section = "invoices"
	case strings.HasPrefix(r.URL.Path, "/admin/parzellen"):
		section = "parzellen"
	case strings.HasPrefix(r.URL.Path, "/admin/protokolle"):
		section = "protokolle"
	case strings.HasPrefix(r.URL.Path, "/admin/backup"):
		section = "backup"
	case strings.HasPrefix(r.URL.Path, "/admin/audit-log"):
		section = "audit"
	case strings.HasPrefix(r.URL.Path, "/admin/system-info"):
		section = "system"
	}

	data["AdminSection"] = section
	return data
}
