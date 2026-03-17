package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

type claims struct {
	Plan      string   `json:"plan"`
	IssuedTo  string   `json:"issued_to"`
	IssuedAt  string   `json:"issued_at"`
	ExpiresAt string   `json:"expires_at,omitempty"`
	Features  []string `json:"features"`
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "gen-keypair":
		genKeypair()
	case "issue":
		issueLicense(os.Args[2:])
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("license_issuer usage:")
	fmt.Println("  go run ./tools/license_issuer gen-keypair")
	fmt.Println("  go run ./tools/license_issuer issue --private-key-base64 <BASE64> --plan premium --issued-to \"Verein\" --expires-at 2027-12-31T23:59:59Z --features wertermittlung,inspektion,mailing,invoice_print")
}

func genKeypair() {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fmt.Fprintf(os.Stderr, "key generation failed: %v\n", err)
		os.Exit(1)
	}

	publicKey := privateKey.Public().(ed25519.PublicKey)

	fmt.Println("Store private key securely and never in the app.")
	fmt.Println("LICENSE_PRIVATE_KEY_BASE64=" + base64.StdEncoding.EncodeToString(privateKey))
	fmt.Println("LICENSE_PUBLIC_KEY=" + base64.StdEncoding.EncodeToString(publicKey))
}

func issueLicense(args []string) {
	fs := flag.NewFlagSet("issue", flag.ExitOnError)
	privateKeyB64 := fs.String("private-key-base64", "", "Base64 encoded ed25519 private key (64 bytes)")
	plan := fs.String("plan", "premium", "License plan")
	issuedTo := fs.String("issued-to", "", "License owner")
	expiresAt := fs.String("expires-at", "", "RFC3339 expiry timestamp (optional)")
	featuresCSV := fs.String("features", "wertermittlung,inspektion,mailing,invoice_print", "Comma-separated features")

	_ = fs.Parse(args)

	if *privateKeyB64 == "" || *issuedTo == "" {
		fmt.Fprintln(os.Stderr, "private-key-base64 and issued-to are required")
		os.Exit(1)
	}

	privateKeyRaw, err := base64.StdEncoding.DecodeString(*privateKeyB64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid private key base64: %v\n", err)
		os.Exit(1)
	}
	if len(privateKeyRaw) != ed25519.PrivateKeySize {
		fmt.Fprintf(os.Stderr, "invalid private key length: expected %d, got %d\n", ed25519.PrivateKeySize, len(privateKeyRaw))
		os.Exit(1)
	}

	if *expiresAt != "" {
		if _, err := time.Parse(time.RFC3339, *expiresAt); err != nil {
			fmt.Fprintf(os.Stderr, "invalid expires-at, use RFC3339: %v\n", err)
			os.Exit(1)
		}
	}

	features := splitFeatures(*featuresCSV)
	if len(features) == 0 {
		fmt.Fprintln(os.Stderr, "at least one feature is required")
		os.Exit(1)
	}

	c := claims{
		Plan:      strings.TrimSpace(*plan),
		IssuedTo:  strings.TrimSpace(*issuedTo),
		IssuedAt:  time.Now().UTC().Format(time.RFC3339),
		ExpiresAt: strings.TrimSpace(*expiresAt),
		Features:  features,
	}

	payload, err := json.Marshal(c)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal claims: %v\n", err)
		os.Exit(1)
	}

	signature := ed25519.Sign(ed25519.PrivateKey(privateKeyRaw), payload)
	licenseKey := "KGV1." + base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(signature)

	fmt.Println("LICENSE_KEY=" + licenseKey)
}

func splitFeatures(input string) []string {
	parts := strings.Split(input, ",")
	features := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		feature := strings.TrimSpace(strings.ToLower(part))
		if feature == "" || seen[feature] {
			continue
		}
		seen[feature] = true
		features = append(features, feature)
	}
	return features
}
