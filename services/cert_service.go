package services

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CertManager handles self-signed certificate generation and validation
type CertManager struct {
	CertDir  string
	CertFile string
	KeyFile  string
}

// NewCertManager creates a new certificate manager
func NewCertManager() *CertManager {
	certDir := filepath.Join(os.Getenv("HOME"), ".gartenamt", "certs")
	return &CertManager{
		CertDir:  certDir,
		CertFile: filepath.Join(certDir, "server.crt"),
		KeyFile:  filepath.Join(certDir, "server.key"),
	}
}

// EnsureCertificate checks if a valid certificate exists, creates one if needed
func (cm *CertManager) EnsureCertificate() error {
	// Create cert directory if it doesn't exist
	if err := os.MkdirAll(cm.CertDir, 0700); err != nil {
		return fmt.Errorf("failed to create certificate directory: %w", err)
	}

	// Check if certificates exist and are valid
	if cm.isCertificateValid() {
		log.Println("✅ Gültiges Zertifikat gefunden")
		return nil
	}

	log.Println("🔐 Erstelle neues selbstsigniertes Zertifikat...")
	return cm.generateSelfSignedCert()
}

// isCertificateValid checks if certificate exists and is not expired
func (cm *CertManager) isCertificateValid() bool {
	// Check if files exist
	if _, err := os.Stat(cm.CertFile); os.IsNotExist(err) {
		return false
	}
	if _, err := os.Stat(cm.KeyFile); os.IsNotExist(err) {
		return false
	}

	// Try to load and parse the certificate
	certPEM, err := os.ReadFile(cm.CertFile)
	if err != nil {
		return false
	}

	block, _ := pem.Decode(certPEM)
	if block == nil {
		return false
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false
	}

	// Check if certificate is still valid (with 1-day buffer)
	if cert.NotAfter.Before(time.Now().Add(24 * time.Hour)) {
		log.Printf("⚠️  Zertifikat läuft bald ab: %s", cert.NotAfter)
		return false
	}

	// Verify cert is for localhost
	hasLocalhost := false
	for _, dnsName := range cert.DNSNames {
		if dnsName == "localhost" || dnsName == "127.0.0.1" {
			hasLocalhost = true
			break
		}
	}

	return hasLocalhost
}

// generateSelfSignedCert creates a new self-signed certificate valid for 6 months
func (cm *CertManager) generateSelfSignedCert() error {
	// Generate RSA private key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate private key: %w", err)
	}

	// Create certificate template
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("failed to generate serial number: %w", err)
	}

	notBefore := time.Now()
	notAfter := notBefore.AddDate(0, 6, 0) // Valid for 6 months

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Gartenamt"},
			CommonName:   "localhost",
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,

		DNSNames: certDNSNames(),
		IPAddresses: []net.IP{
			net.ParseIP("127.0.0.1"),
			net.ParseIP("::1"),
		},
	}

	// Create certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	// Save certificate to file
	certOut, err := os.Create(cm.CertFile)
	if err != nil {
		return fmt.Errorf("failed to create certificate file: %w", err)
	}
	defer certOut.Close()

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to encode certificate: %w", err)
	}

	// Save private key to file - 0600, os.Create wuerde 0666 abzueglich umask
	// verwenden und den Schluessel damit lokal lesbar machen.
	keyOut, err := os.OpenFile(cm.KeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create key file: %w", err)
	}
	defer keyOut.Close()

	privBytes, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to encode private key: %w", err)
	}

	log.Printf("✅ Zertifikat erstellt: %s", cm.CertFile)
	log.Printf("   Gültig bis: %s", notAfter.Format("02.01.2006 15:04:05"))

	return nil
}

// certDNSNames liefert die SAN-Hostnamen für das selbstsignierte Zertifikat:
// localhost, der Rechnername sowie optionale Einträge aus TLS_EXTRA_HOSTS
// (kommagetrennt), damit LAN-Zugriffe keinen Hostname-Mismatch erzeugen.
func certDNSNames() []string {
	names := []string{"localhost"}

	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		names = append(names, hostname)
	}

	for _, extra := range strings.Split(os.Getenv("TLS_EXTRA_HOSTS"), ",") {
		if extra = strings.TrimSpace(extra); extra != "" {
			names = append(names, extra)
		}
	}

	return names
}

// GetTLSConfig returns the TLS configuration for the server
func (cm *CertManager) GetTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(cm.CertFile, cm.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load certificates: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
	}, nil
}
