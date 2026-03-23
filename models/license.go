package models

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	FeatureWertermittlung = "wertermittlung"
	FeatureInspektion     = "inspektion"
	FeatureMailing        = "mailing"
	FeatureInvoicePrint   = "invoice_print"
)

type LicenseClaims struct {
	Plan      string   `json:"plan"`
	IssuedTo  string   `json:"issued_to"`
	IssuedAt  string   `json:"issued_at"`
	ExpiresAt string   `json:"expires_at"`
	Features  []string `json:"features"`
}

type LicenseStatus struct {
	Active      bool
	Plan        string
	IssuedTo    string
	ExpiresAt   *time.Time
	Features    map[string]bool
	ActivatedAt *time.Time
	KeyHash     string
}

func RequiredPremiumFeatures() []string {
	return []string{FeatureWertermittlung, FeatureInspektion, FeatureMailing, FeatureInvoicePrint}
}

func VerifyAndParseLicenseKey(licenseKey string) (*LicenseClaims, string, error) {
	key := strings.TrimSpace(licenseKey)
	parts := strings.Split(key, ".")
	if len(parts) != 3 || parts[0] != "KGV1" {
		return nil, "", errors.New("ungueltiges Lizenzformat")
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, "", errors.New("ungueltiger Lizenz-Payload")
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, "", errors.New("ungueltige Lizenz-Signatur")
	}

	publicKey, err := loadLicensePublicKey()
	if err != nil {
		return nil, "", err
	}

	if !ed25519.Verify(publicKey, payload, signature) {
		return nil, "", errors.New("signaturpruefung fehlgeschlagen")
	}

	claims := &LicenseClaims{}
	if err := json.Unmarshal(payload, claims); err != nil {
		return nil, "", errors.New("lizenzdaten konnten nicht gelesen werden")
	}

	if claims.Plan == "" {
		return nil, "", errors.New("lizenz ohne plan ist ungueltig")
	}

	if len(claims.Features) == 0 {
		return nil, "", errors.New("lizenz ohne features ist ungueltig")
	}

	if claims.ExpiresAt != "" {
		expiresAt, err := time.Parse(time.RFC3339, claims.ExpiresAt)
		if err != nil {
			return nil, "", errors.New("ungueltiges ablaufdatum in lizenz")
		}
		if time.Now().After(expiresAt) {
			return nil, "", errors.New("lizenz ist abgelaufen")
		}
	}

	keyHash := hashLicenseKey(key)
	return claims, keyHash, nil
}

func ActivateLicenseKey(licenseKey string) (*LicenseStatus, error) {
	claims, keyHash, err := VerifyAndParseLicenseKey(licenseKey)
	if err != nil {
		return nil, err
	}

	featuresJSON, err := json.Marshal(uniqueFeatures(claims.Features))
	if err != nil {
		return nil, err
	}

	var expiresAt interface{}
	if claims.ExpiresAt != "" {
		expiresAt = claims.ExpiresAt
	}

	query := `
	INSERT INTO license_state
	(id, is_active, plan, issued_to, key_hash, features_json, expires_at, activated_at, last_validation, updated_at)
	VALUES (1, 1, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO UPDATE SET
		is_active = 1,
		plan = excluded.plan,
		issued_to = excluded.issued_to,
		key_hash = excluded.key_hash,
		features_json = excluded.features_json,
		expires_at = excluded.expires_at,
		activated_at = CURRENT_TIMESTAMP,
		last_validation = CURRENT_TIMESTAMP,
		updated_at = CURRENT_TIMESTAMP
	`

	if _, err := DB.Exec(query, claims.Plan, claims.IssuedTo, keyHash, string(featuresJSON), expiresAt); err != nil {
		return nil, err
	}

	return GetLicenseStatus()
}

func DeactivateLicense() error {
	_, err := DB.Exec(`
	INSERT INTO license_state (id, is_active, updated_at)
	VALUES (1, 0, CURRENT_TIMESTAMP)
	ON CONFLICT(id) DO UPDATE SET
		is_active = 0,
		updated_at = CURRENT_TIMESTAMP
	`)
	return err
}

func GetLicenseStatus() (*LicenseStatus, error) {
	row := DB.QueryRow(`
	SELECT is_active, plan, issued_to, key_hash, features_json, expires_at, activated_at
	FROM license_state
	WHERE id = 1
	`)

	status := &LicenseStatus{Features: map[string]bool{}}
	var featuresJSON string
	var expiresAtRaw, activatedAtRaw interface{}

	if err := row.Scan(&status.Active, &status.Plan, &status.IssuedTo, &status.KeyHash, &featuresJSON, &expiresAtRaw, &activatedAtRaw); err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return status, nil
		}
		return nil, err
	}

	_ = json.Unmarshal([]byte(featuresJSON), &status.Features)

	if ts, ok := toTimePointer(expiresAtRaw); ok {
		status.ExpiresAt = ts
		if ts.Before(time.Now()) {
			status.Active = false
		}
	}

	if ts, ok := toTimePointer(activatedAtRaw); ok {
		status.ActivatedAt = ts
	}

	return status, nil
}

func HasPremiumFeature(feature string) bool {
	status, err := GetLicenseStatus()
	if err != nil || status == nil || !status.Active {
		return false
	}

	if status.ExpiresAt != nil && status.ExpiresAt.Before(time.Now()) {
		return false
	}

	for _, required := range RequiredPremiumFeatures() {
		if feature == required {
			return status.Features[feature]
		}
	}

	return false
}

func FeatureDisplayName(feature string) string {
	switch feature {
	case FeatureWertermittlung:
		return "Wertermittlung"
	case FeatureInspektion:
		return "Inspektion"
	case FeatureMailing:
		return "Mailing"
	case FeatureInvoicePrint:
		return "Rechnungsdruck"
	default:
		return feature
	}
}

func hashLicenseKey(licenseKey string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(licenseKey)))
	return hex.EncodeToString(sum[:])
}

func loadLicensePublicKey() (ed25519.PublicKey, error) {
	keyB64 := strings.TrimSpace(os.Getenv("LICENSE_PUBLIC_KEY"))
	if keyB64 == "" {
		keyB64 = strings.TrimSpace(fetchLicensePublicKeyFromServer())
	}
	if keyB64 == "" {
		return nil, errors.New("LICENSE_PUBLIC_KEY nicht gesetzt und kein Public Key vom License Server abrufbar")
	}

	keyBytes, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("LICENSE_PUBLIC_KEY ist ungueltig: %w", err)
	}

	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, errors.New("LICENSE_PUBLIC_KEY hat ungueltige laenge")
	}

	pk := ed25519.PublicKey(keyBytes)
	test := make([]byte, ed25519.PublicKeySize)
	if subtle.ConstantTimeCompare(pk, test) == 1 {
		return nil, errors.New("LICENSE_PUBLIC_KEY darf nicht leer sein")
	}

	return pk, nil
}

func fetchLicensePublicKeyFromServer() string {
	serverURL := strings.TrimSpace(os.Getenv("LICENSE_SERVER_URL"))
	if serverURL == "" {
		return ""
	}

	endpoint := strings.TrimRight(serverURL, "/") + "/v1/keys/public"
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(endpoint)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var response struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return ""
	}

	return strings.TrimSpace(response.PublicKey)
}

func uniqueFeatures(features []string) map[string]bool {
	result := map[string]bool{}
	for _, feature := range features {
		feature = strings.TrimSpace(strings.ToLower(feature))
		if feature == "" {
			continue
		}
		result[feature] = true
	}
	return result
}

func toTimePointer(value interface{}) (*time.Time, bool) {
	switch v := value.(type) {
	case string:
		if v == "" {
			return nil, false
		}
		if parsed, err := time.Parse(time.RFC3339, v); err == nil {
			return &parsed, true
		}
		if parsed, err := time.Parse("2006-01-02 15:04:05", v); err == nil {
			return &parsed, true
		}
	case []byte:
		return toTimePointer(string(v))
	case time.Time:
		copy := v
		return &copy, true
	}
	return nil, false
}
