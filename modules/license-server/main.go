package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type LicenseClaims struct {
	Plan      string   `json:"plan"`
	IssuedTo  string   `json:"issued_to"`
	IssuedAt  string   `json:"issued_at"`
	ExpiresAt string   `json:"expires_at,omitempty"`
	Features  []string `json:"features"`
}

type issueRequest struct {
	Plan      string   `json:"plan"`
	IssuedTo  string   `json:"issued_to"`
	ExpiresAt string   `json:"expires_at,omitempty"`
	Features  []string `json:"features"`
}

type validateRequest struct {
	LicenseKey string `json:"license_key"`
}

type revokeRequest struct {
	LicenseKey string `json:"license_key"`
}

type uiData struct {
	PublicKeyBase64   string
	PublicFingerprint string
	GeneratedAtBoot   bool
	IssueEnabled      bool
	Error             string
	Success           string
	IssuedLicense     string
	Plan              string
	IssuedTo          string
	ExpiresAt         string
	Features          string
}

type server struct {
	db                  *sql.DB
	publicKey           ed25519.PublicKey
	privateKey          ed25519.PrivateKey
	publicKeyBase64     string
	publicFingerprint   string
	generatedAtBoot     bool
	adminToken          string
	clientToken         string
	allowIssueAndRevoke bool
}

func main() {
	dbPath := envOrDefault("LICENSE_DB_PATH", "/data/licenses.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := initDB(db); err != nil {
		log.Fatal(err)
	}

	publicKey, privateKey, publicKeyBase64, generatedAtBoot, err := bootstrapKeyMaterial(db)
	if err != nil {
		log.Fatal(err)
	}
	publicFingerprint := fingerprintFromPublicKey(publicKey)

	srv := &server{
		db:                  db,
		publicKey:           publicKey,
		privateKey:          privateKey,
		publicKeyBase64:     publicKeyBase64,
		publicFingerprint:   publicFingerprint,
		generatedAtBoot:     generatedAtBoot,
		adminToken:          os.Getenv("LICENSE_SERVER_ADMIN_TOKEN"),
		clientToken:         os.Getenv("LICENSE_SERVER_CLIENT_TOKEN"),
		allowIssueAndRevoke: len(privateKey) == ed25519.PrivateKeySize,
	}

	if generatedAtBoot {
		log.Printf("generated initial keypair, public fingerprint: %s", publicFingerprint)
	} else {
		log.Printf("loaded keypair, public fingerprint: %s", publicFingerprint)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", srv.webIndex)
	mux.HandleFunc("/ui/issue", srv.webIssue)
	mux.HandleFunc("/health", srv.health)
	mux.HandleFunc("/v1/keys/public", srv.publicKeyInfo)
	mux.HandleFunc("/v1/licenses/validate", srv.validate)
	mux.HandleFunc("/v1/licenses/issue", srv.issue)
	mux.HandleFunc("/v1/licenses/revoke", srv.revoke)

	addr := envOrDefault("LICENSE_SERVER_ADDR", ":8090")
	log.Printf("license server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func initDB(db *sql.DB) error {
	query := `
	CREATE TABLE IF NOT EXISTS licenses (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key_hash TEXT NOT NULL UNIQUE,
		issued_to TEXT NOT NULL,
		plan TEXT NOT NULL,
		features_json TEXT NOT NULL,
		expires_at DATETIME,
		revoked BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		revoked_at DATETIME
	);
	CREATE TABLE IF NOT EXISTS server_keys (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		public_key_base64 TEXT NOT NULL,
		private_key_base64 TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err := db.Exec(query)
	return err
}

func (s *server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok":                   true,
		"issue_revoke_enabled": s.allowIssueAndRevoke,
		"public_fingerprint":   s.publicFingerprint,
		"generated_at_boot":    s.generatedAtBoot,
	})
}

func (s *server) publicKeyInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"public_key":         s.publicKeyBase64,
		"public_fingerprint": s.publicFingerprint,
		"generated_at_boot":  s.generatedAtBoot,
	})
}

func (s *server) issue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	if !s.authorizeAdmin(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if !s.allowIssueAndRevoke {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "private_key_not_configured"})
		return
	}

	var req issueRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}

	req.Plan = strings.TrimSpace(req.Plan)
	req.IssuedTo = strings.TrimSpace(req.IssuedTo)
	if req.Plan == "" || req.IssuedTo == "" || len(req.Features) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_required_fields"})
		return
	}

	claims, licenseKey, keyHash, err := s.createLicense(req.Plan, req.IssuedTo, req.ExpiresAt, req.Features)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"license_key": licenseKey,
		"key_hash":    keyHash,
		"claims":      claims,
	})
}

func (s *server) webIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	data := uiData{
		PublicKeyBase64:   s.publicKeyBase64,
		PublicFingerprint: s.publicFingerprint,
		GeneratedAtBoot:   s.generatedAtBoot,
		IssueEnabled:      s.allowIssueAndRevoke,
		Features:          "wertermittlung,inspektion,mailing,invoice_print",
		Plan:              "premium",
	}

	renderUI(w, data)
}

func (s *server) webIssue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	_ = r.ParseForm()
	data := uiData{
		PublicKeyBase64:   s.publicKeyBase64,
		PublicFingerprint: s.publicFingerprint,
		GeneratedAtBoot:   s.generatedAtBoot,
		IssueEnabled:      s.allowIssueAndRevoke,
		Plan:              strings.TrimSpace(r.FormValue("plan")),
		IssuedTo:          strings.TrimSpace(r.FormValue("issued_to")),
		ExpiresAt:         strings.TrimSpace(r.FormValue("expires_at")),
		Features:          strings.TrimSpace(r.FormValue("features")),
	}

	if !s.allowIssueAndRevoke {
		data.Error = "Issue-Funktion ist deaktiviert: kein privater Schluessel vorhanden"
		renderUI(w, data)
		return
	}

	adminTokenInput := strings.TrimSpace(r.FormValue("admin_token"))
	if strings.TrimSpace(s.adminToken) == "" || adminTokenInput != s.adminToken {
		data.Error = "Ungueltiger Admin-Token"
		renderUI(w, data)
		return
	}

	features := splitFeaturesCSV(data.Features)
	_, licenseKey, _, err := s.createLicense(data.Plan, data.IssuedTo, data.ExpiresAt, features)
	if err != nil {
		data.Error = err.Error()
		renderUI(w, data)
		return
	}

	data.Success = "Lizenz erfolgreich erstellt"
	data.IssuedLicense = licenseKey
	renderUI(w, data)
}

func (s *server) validate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	if !s.authorizeClient(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req validateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}

	claims, keyHash, err := verifyLicenseKey(s.publicKey, req.LicenseKey)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"valid": false, "reason": err.Error()})
		return
	}

	var revoked bool
	err = s.db.QueryRow("SELECT revoked FROM licenses WHERE key_hash = ?", keyHash).Scan(&revoked)
	if err == nil && revoked {
		writeJSON(w, http.StatusOK, map[string]interface{}{"valid": false, "reason": "revoked"})
		return
	}

	if claims.ExpiresAt != "" {
		expiresAt, parseErr := time.Parse(time.RFC3339, claims.ExpiresAt)
		if parseErr == nil && time.Now().After(expiresAt) {
			writeJSON(w, http.StatusOK, map[string]interface{}{"valid": false, "reason": "expired"})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"valid":  true,
		"claims": claims,
	})
}

func (s *server) revoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method_not_allowed"})
		return
	}
	if !s.authorizeAdmin(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if !s.allowIssueAndRevoke {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "private_key_not_configured"})
		return
	}

	var req revokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	if strings.TrimSpace(req.LicenseKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing_license_key"})
		return
	}

	_, keyHash, err := verifyLicenseKey(s.publicKey, req.LicenseKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_license_key"})
		return
	}

	result, err := s.db.Exec("UPDATE licenses SET revoked = 1, revoked_at = CURRENT_TIMESTAMP WHERE key_hash = ?", keyHash)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "db_write_failed"})
		return
	}
	rows, _ := result.RowsAffected()

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"revoked":  rows > 0,
		"key_hash": keyHash,
	})
}

func (s *server) authorizeAdmin(r *http.Request) bool {
	if strings.TrimSpace(s.adminToken) == "" {
		return false
	}
	return bearerToken(r) == s.adminToken
}

func (s *server) authorizeClient(r *http.Request) bool {
	if strings.TrimSpace(s.clientToken) == "" {
		return true
	}
	return bearerToken(r) == s.clientToken
}

func bearerToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
}

func verifyLicenseKey(publicKey ed25519.PublicKey, licenseKey string) (*LicenseClaims, string, error) {
	parts := strings.Split(strings.TrimSpace(licenseKey), ".")
	if len(parts) != 3 || parts[0] != "KGV1" {
		return nil, "", errors.New("invalid_format")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "", errors.New("invalid_payload")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, "", errors.New("invalid_signature")
	}

	if !ed25519.Verify(publicKey, payload, signature) {
		return nil, "", errors.New("signature_invalid")
	}

	claims := &LicenseClaims{}
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, "", errors.New("claims_invalid")
	}
	if claims.Plan == "" || len(claims.Features) == 0 {
		return nil, "", errors.New("claims_incomplete")
	}

	return claims, hashLicenseKey(licenseKey), nil
}

func (s *server) createLicense(plan, issuedTo, expiresAt string, features []string) (*LicenseClaims, string, string, error) {
	plan = strings.TrimSpace(plan)
	issuedTo = strings.TrimSpace(issuedTo)
	features = normalizeFeatures(features)
	if plan == "" || issuedTo == "" || len(features) == 0 {
		return nil, "", "", errors.New("missing_required_fields")
	}

	if expiresAt != "" {
		if _, err := time.Parse(time.RFC3339, expiresAt); err != nil {
			return nil, "", "", errors.New("invalid_expires_at")
		}
	}

	claims := &LicenseClaims{
		Plan:      plan,
		IssuedTo:  issuedTo,
		IssuedAt:  time.Now().UTC().Format(time.RFC3339),
		ExpiresAt: expiresAt,
		Features:  features,
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return nil, "", "", errors.New("marshal_failed")
	}

	signature := ed25519.Sign(s.privateKey, payload)
	licenseKey := "KGV1." + base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature)
	keyHash := hashLicenseKey(licenseKey)

	featuresJSON, _ := json.Marshal(claims.Features)
	_, err = s.db.Exec(`
		INSERT OR REPLACE INTO licenses (key_hash, issued_to, plan, features_json, expires_at, revoked, revoked_at)
		VALUES (?, ?, ?, ?, ?, 0, NULL)
	`, keyHash, claims.IssuedTo, claims.Plan, string(featuresJSON), nullOrValue(claims.ExpiresAt))
	if err != nil {
		return nil, "", "", errors.New("db_write_failed")
	}

	return claims, licenseKey, keyHash, nil
}

func normalizeFeatures(features []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(features))
	for _, feature := range features {
		normalized := strings.TrimSpace(strings.ToLower(feature))
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		result = append(result, normalized)
	}
	return result
}

func hashLicenseKey(licenseKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(licenseKey)))
	return hex.EncodeToString(sum[:])
}

func bootstrapKeyMaterial(db *sql.DB) (ed25519.PublicKey, ed25519.PrivateKey, string, bool, error) {
	envPub := strings.TrimSpace(os.Getenv("LICENSE_PUBLIC_KEY"))
	envPriv := strings.TrimSpace(os.Getenv("LICENSE_PRIVATE_KEY_BASE64"))
	if (envPub == "" && envPriv != "") || (envPub != "" && envPriv == "") {
		return nil, nil, "", false, errors.New("set both LICENSE_PUBLIC_KEY and LICENSE_PRIVATE_KEY_BASE64, or neither")
	}

	if envPub != "" && envPriv != "" {
		pub, priv, err := decodeKeyPair(envPub, envPriv)
		if err != nil {
			return nil, nil, "", false, err
		}
		_, err = db.Exec(`
			INSERT INTO server_keys (id, public_key_base64, private_key_base64, updated_at)
			VALUES (1, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET
				public_key_base64 = excluded.public_key_base64,
				private_key_base64 = excluded.private_key_base64,
				updated_at = CURRENT_TIMESTAMP
		`, envPub, envPriv)
		if err != nil {
			return nil, nil, "", false, err
		}
		return pub, priv, envPub, false, nil
	}

	var storedPub, storedPriv string
	err := db.QueryRow("SELECT public_key_base64, private_key_base64 FROM server_keys WHERE id = 1").Scan(&storedPub, &storedPriv)
	if err == nil {
		pub, priv, decodeErr := decodeKeyPair(storedPub, storedPriv)
		if decodeErr != nil {
			return nil, nil, "", false, decodeErr
		}
		return pub, priv, storedPub, false, nil
	}

	if err != sql.ErrNoRows {
		return nil, nil, "", false, err
	}

	pub, priv, genErr := ed25519.GenerateKey(rand.Reader)
	if genErr != nil {
		return nil, nil, "", false, genErr
	}

	pubB64 := base64.StdEncoding.EncodeToString(pub)
	privB64 := base64.StdEncoding.EncodeToString(priv)
	_, execErr := db.Exec(
		"INSERT INTO server_keys (id, public_key_base64, private_key_base64) VALUES (1, ?, ?)",
		pubB64,
		privB64,
	)
	if execErr != nil {
		return nil, nil, "", false, execErr
	}

	return pub, priv, pubB64, true, nil
}

func decodeKeyPair(publicKeyB64, privateKeyB64 string) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	pubDecoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(publicKeyB64))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid public key: %w", err)
	}
	privDecoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(privateKeyB64))
	if err != nil {
		return nil, nil, fmt.Errorf("invalid private key: %w", err)
	}
	if len(pubDecoded) != ed25519.PublicKeySize {
		return nil, nil, errors.New("public key has invalid length")
	}
	if len(privDecoded) != ed25519.PrivateKeySize {
		return nil, nil, errors.New("private key has invalid length")
	}
	return ed25519.PublicKey(pubDecoded), ed25519.PrivateKey(privDecoded), nil
}

func fingerprintFromPublicKey(publicKey ed25519.PublicKey) string {
	sum := sha256.Sum256(publicKey)
	return hex.EncodeToString(sum[:8])
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func nullOrValue(value string) interface{} {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return trimmed
}

func splitFeaturesCSV(csv string) []string {
	parts := strings.Split(csv, ",")
	features := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		features = append(features, trimmed)
	}
	return features
}

func renderUI(w http.ResponseWriter, data uiData) {
	const page = `<!doctype html>
<html lang="de">
<head>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>License Server</title>
	<style>
		body { font-family: Georgia, serif; background: #f5f6f2; margin: 0; color: #1f2a1f; }
		.wrap { max-width: 920px; margin: 0 auto; padding: 24px; }
		.card { background: #fff; border-radius: 12px; box-shadow: 0 6px 18px rgba(0,0,0,.08); padding: 20px; margin-bottom: 18px; }
		label { display: block; font-weight: 600; margin-top: 8px; }
		input, textarea { width: 100%; box-sizing: border-box; padding: 10px; border: 1px solid #cfd6cc; border-radius: 8px; margin-top: 4px; }
		button { margin-top: 12px; background: #2d6a4f; color: #fff; border: 0; border-radius: 8px; padding: 10px 14px; cursor: pointer; }
		.muted { color: #5e6a60; font-size: 14px; }
		.ok { background: #d8f3dc; padding: 10px; border-radius: 8px; }
		.err { background: #ffd8d8; padding: 10px; border-radius: 8px; }
		code { word-break: break-all; }
	</style>
</head>
<body>
	<div class="wrap">
		<div class="card">
			<h1>License Server</h1>
			<p class="muted">Dieser Server startet zuerst, erzeugt initiale Schluessel (falls nicht vorhanden) und stellt die Public Key Information sowie Lizenz-Erstellung bereit.</p>
			<p><strong>Public Key Fingerprint:</strong> {{.PublicFingerprint}}</p>
			<p><strong>Beim Start erzeugt:</strong> {{if .GeneratedAtBoot}}ja{{else}}nein{{end}}</p>
			<p><strong>Issue aktiv:</strong> {{if .IssueEnabled}}ja{{else}}nein{{end}}</p>
			<label>Public Key (base64)</label>
			<textarea rows="3" readonly>{{.PublicKeyBase64}}</textarea>
		</div>

		<div class="card">
			<h2>Neue Lizenz erzeugen</h2>
			{{if .Success}}<p class="ok">{{.Success}}</p>{{end}}
			{{if .Error}}<p class="err">{{.Error}}</p>{{end}}
			<form method="POST" action="/ui/issue">
				<label>Admin Token</label>
				<input type="password" name="admin_token" required>

				<label>Plan</label>
				<input name="plan" value="{{.Plan}}" required>

				<label>Issued To</label>
				<input name="issued_to" value="{{.IssuedTo}}" required>

				<label>Expires At (RFC3339, optional)</label>
				<input name="expires_at" value="{{.ExpiresAt}}" placeholder="2027-12-31T23:59:59Z">

				<label>Features (comma-separated)</label>
				<input name="features" value="{{.Features}}" required>

				<button type="submit">Lizenz erstellen</button>
			</form>
			{{if .IssuedLicense}}
			<label>Generierter Lizenzcode</label>
			<textarea rows="4" readonly>{{.IssuedLicense}}</textarea>
			{{end}}
		</div>
	</div>
</body>
</html>`

	tmpl := template.Must(template.New("page").Parse(page))
	_ = tmpl.Execute(w, data)
}
