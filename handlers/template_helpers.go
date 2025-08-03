package handlers

import (
	"kleingarten-verwaltung/middleware"
	"net/http"
)

// AddSessionToData copies session from context into your template data map.
func AddSessionToData(r *http.Request, data map[string]interface{}) map[string]interface{} {
	data["Session"] = middleware.GetSessionFromContext(r.Context())
	return data
}
