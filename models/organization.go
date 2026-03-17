package models

import (
	"time"

	"kleingarten-verwaltung/securestore"
)

// OrganizationSettings stores the Kleingarten organization's details for invoices
type OrganizationSettings struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`         // Organization name
	Strasse        string    `json:"strasse"`      // Street
	Hausnummer     string    `json:"hausnummer"`   // House number
	PLZ            string    `json:"plz"`          // Postal code
	Ort            string    `json:"ort"`          // City
	Telefon        string    `json:"telefon"`      // Phone
	Email          string    `json:"email"`        // Email
	Website        string    `json:"website"`      // Website
	IBAN           string    `json:"iban"`         // Bank account IBAN
	BIC            string    `json:"bic"`          // Bank code (BIC)
	Kontoinhaber   string    `json:"kontoinhaber"` // Account owner name
	SMTPHost       string    `json:"smtp_host"`
	SMTPPort       int       `json:"smtp_port"`
	SMTPUsername   string    `json:"smtp_username"`
	SMTPPassword   string    `json:"smtp_password,omitempty"`
	SMTPConfigured bool      `json:"smtp_password_configured"`
	SMTPFromAddr   string    `json:"smtp_from_address"`
	SMTPFromName   string    `json:"smtp_from_name"`
	SMTPTLSMode    string    `json:"smtp_tls_mode"`
	IMAPHost       string    `json:"imap_host"`
	IMAPPort       int       `json:"imap_port"`
	IMAPUsername   string    `json:"imap_username"`
	IMAPPassword   string    `json:"imap_password,omitempty"`
	IMAPConfigured bool      `json:"imap_password_configured"`
	IMAPSecure     bool      `json:"imap_secure"`
	Steuernummer   string    `json:"steuernummer"`   // Tax number
	Registernummer string    `json:"registernummer"` // Registry number
	LogoPath       string    `json:"logo_path"`      // Path to logo for invoices
	UpdatedAt      time.Time `json:"updated_at"`
	CreatedAt      time.Time `json:"created_at"`
}

func getOrganizationSettings(includeSecrets bool) (*OrganizationSettings, error) {
	query := `SELECT id, name, strasse, hausnummer, plz, ort, telefon, email, website,
	                 iban, bic, kontoinhaber, smtp_host, smtp_port, smtp_username, smtp_password_enc,
	                 smtp_from_address, smtp_from_name, smtp_tls_mode,
	                 imap_host, imap_port, imap_username, imap_password_enc, imap_secure,
	                 steuernummer, registernummer, logo_path, updated_at, created_at
	          FROM organization_settings LIMIT 1`
	row := DB.QueryRow(query)

	var org OrganizationSettings
	var smtpPasswordEnc string
	var imapPasswordEnc string
	err := row.Scan(&org.ID, &org.Name, &org.Strasse, &org.Hausnummer, &org.PLZ, &org.Ort,
		&org.Telefon, &org.Email, &org.Website, &org.IBAN, &org.BIC, &org.Kontoinhaber,
		&org.SMTPHost, &org.SMTPPort, &org.SMTPUsername, &smtpPasswordEnc,
		&org.SMTPFromAddr, &org.SMTPFromName, &org.SMTPTLSMode,
		&org.IMAPHost, &org.IMAPPort, &org.IMAPUsername, &imapPasswordEnc, &org.IMAPSecure,
		&org.Steuernummer, &org.Registernummer, &org.LogoPath, &org.UpdatedAt, &org.CreatedAt)
	if err != nil {
		return &OrganizationSettings{}, nil
	}

	org.SMTPConfigured = smtpPasswordEnc != ""
	org.IMAPConfigured = imapPasswordEnc != ""
	if org.SMTPTLSMode == "" {
		org.SMTPTLSMode = "tls"
	}

	if includeSecrets {
		if smtpPasswordEnc != "" {
			org.SMTPPassword, err = securestore.DecryptString(smtpPasswordEnc)
			if err != nil {
				return nil, err
			}
		}
		if imapPasswordEnc != "" {
			org.IMAPPassword, err = securestore.DecryptString(imapPasswordEnc)
			if err != nil {
				return nil, err
			}
		}
	}

	return &org, nil
}

// GetOrganizationSettings retrieves the organization settings (usually only one record)
func GetOrganizationSettings() (*OrganizationSettings, error) {
	return getOrganizationSettings(false)
}

func GetOrganizationSettingsWithSecrets() (*OrganizationSettings, error) {
	return getOrganizationSettings(true)
}

// SaveOrganizationSettings saves or updates the organization settings
func (org *OrganizationSettings) Save() error {
	// Check if settings exist
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM organization_settings").Scan(&count)

	var existingID int
	var existingSMTPPasswordEnc string
	var existingIMAPPasswordEnc string
	if count > 0 {
		_ = DB.QueryRow("SELECT id, smtp_password_enc, imap_password_enc FROM organization_settings LIMIT 1").Scan(&existingID, &existingSMTPPasswordEnc, &existingIMAPPasswordEnc)
	}

	smtpPasswordEnc := existingSMTPPasswordEnc
	if org.SMTPPassword != "" {
		encrypted, err := securestore.EncryptString(org.SMTPPassword)
		if err != nil {
			return err
		}
		smtpPasswordEnc = encrypted
	}

	imapPasswordEnc := existingIMAPPasswordEnc
	if org.IMAPPassword != "" {
		encrypted, err := securestore.EncryptString(org.IMAPPassword)
		if err != nil {
			return err
		}
		imapPasswordEnc = encrypted
	}

	if org.SMTPTLSMode == "" {
		org.SMTPTLSMode = "tls"
	}

	if count == 0 {
		// Insert new settings
		query := `INSERT INTO organization_settings 
                  (name, strasse, hausnummer, plz, ort, telefon, email, website, 
	                   iban, bic, kontoinhaber, smtp_host, smtp_port, smtp_username, smtp_password_enc,
	                   smtp_from_address, smtp_from_name, smtp_tls_mode,
	                   imap_host, imap_port, imap_username, imap_password_enc, imap_secure,
	                   steuernummer, registernummer, logo_path) 
	                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		result, err := DB.Exec(query, org.Name, org.Strasse, org.Hausnummer, org.PLZ, org.Ort,
			org.Telefon, org.Email, org.Website, org.IBAN, org.BIC, org.Kontoinhaber,
			org.SMTPHost, org.SMTPPort, org.SMTPUsername, smtpPasswordEnc,
			org.SMTPFromAddr, org.SMTPFromName, org.SMTPTLSMode,
			org.IMAPHost, org.IMAPPort, org.IMAPUsername, imapPasswordEnc, org.IMAPSecure,
			org.Steuernummer, org.Registernummer, org.LogoPath)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		org.ID = int(id)
	} else {
		org.ID = existingID
		// Update existing settings
		query := `UPDATE organization_settings 
                  SET name=?, strasse=?, hausnummer=?, plz=?, ort=?, telefon=?, email=?, website=?, 
	                      iban=?, bic=?, kontoinhaber=?, smtp_host=?, smtp_port=?, smtp_username=?, smtp_password_enc=?,
	                      smtp_from_address=?, smtp_from_name=?, smtp_tls_mode=?,
	                      imap_host=?, imap_port=?, imap_username=?, imap_password_enc=?, imap_secure=?,
	                      steuernummer=?, registernummer=?, logo_path=?, updated_at=CURRENT_TIMESTAMP 
                  WHERE id=?`
		_, err := DB.Exec(query, org.Name, org.Strasse, org.Hausnummer, org.PLZ, org.Ort,
			org.Telefon, org.Email, org.Website, org.IBAN, org.BIC, org.Kontoinhaber,
			org.SMTPHost, org.SMTPPort, org.SMTPUsername, smtpPasswordEnc,
			org.SMTPFromAddr, org.SMTPFromName, org.SMTPTLSMode,
			org.IMAPHost, org.IMAPPort, org.IMAPUsername, imapPasswordEnc, org.IMAPSecure,
			org.Steuernummer, org.Registernummer, org.LogoPath, org.ID)
		return err
	}

	org.SMTPPassword = ""
	org.IMAPPassword = ""
	org.SMTPConfigured = smtpPasswordEnc != ""
	org.IMAPConfigured = imapPasswordEnc != ""
	return nil
}
