package models

import (
	"time"
)

// OrganizationSettings stores the Kleingarten organization's details for invoices
type OrganizationSettings struct {
	ID             int       `json:"id"`
	Name           string    `json:"name"`           // Organization name
	Strasse        string    `json:"strasse"`        // Street
	Hausnummer     string    `json:"hausnummer"`     // House number
	PLZ            string    `json:"plz"`            // Postal code
	Ort            string    `json:"ort"`            // City
	Telefon        string    `json:"telefon"`        // Phone
	Email          string    `json:"email"`          // Email
	Website        string    `json:"website"`        // Website
	IBAN           string    `json:"iban"`           // Bank account IBAN
	BIC            string    `json:"bic"`            // Bank code (BIC)
	Kontoinhaber   string    `json:"kontoinhaber"`   // Account owner name
	Steuernummer   string    `json:"steuernummer"`   // Tax number
	Registernummer string    `json:"registernummer"` // Registry number
	LogoPath       string    `json:"logo_path"`      // Path to logo for invoices
	UpdatedAt      time.Time `json:"updated_at"`
	CreatedAt      time.Time `json:"created_at"`
}

// GetOrganizationSettings retrieves the organization settings (usually only one record)
func GetOrganizationSettings() (*OrganizationSettings, error) {
	query := `SELECT id, name, strasse, hausnummer, plz, ort, telefon, email, website, 
                     iban, bic, kontoinhaber, steuernummer, registernummer, logo_path, updated_at, created_at 
              FROM organization_settings LIMIT 1`
	row := DB.QueryRow(query)

	var org OrganizationSettings
	err := row.Scan(&org.ID, &org.Name, &org.Strasse, &org.Hausnummer, &org.PLZ, &org.Ort,
		&org.Telefon, &org.Email, &org.Website, &org.IBAN, &org.BIC, &org.Kontoinhaber,
		&org.Steuernummer, &org.Registernummer, &org.LogoPath, &org.UpdatedAt, &org.CreatedAt)
	if err != nil {
		// Return empty settings if not found instead of error
		return &OrganizationSettings{}, nil
	}
	return &org, nil
}

// SaveOrganizationSettings saves or updates the organization settings
func (org *OrganizationSettings) Save() error {
	// Check if settings exist
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM organization_settings").Scan(&count)

	if count == 0 {
		// Insert new settings
		query := `INSERT INTO organization_settings 
                  (name, strasse, hausnummer, plz, ort, telefon, email, website, 
                   iban, bic, kontoinhaber, steuernummer, registernummer, logo_path) 
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		result, err := DB.Exec(query, org.Name, org.Strasse, org.Hausnummer, org.PLZ, org.Ort,
			org.Telefon, org.Email, org.Website, org.IBAN, org.BIC, org.Kontoinhaber,
			org.Steuernummer, org.Registernummer, org.LogoPath)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		org.ID = int(id)
	} else {
		// Update existing settings
		query := `UPDATE organization_settings 
                  SET name=?, strasse=?, hausnummer=?, plz=?, ort=?, telefon=?, email=?, website=?, 
                      iban=?, bic=?, kontoinhaber=?, steuernummer=?, registernummer=?, logo_path=?, updated_at=CURRENT_TIMESTAMP 
                  WHERE id=?`
		_, err := DB.Exec(query, org.Name, org.Strasse, org.Hausnummer, org.PLZ, org.Ort,
			org.Telefon, org.Email, org.Website, org.IBAN, org.BIC, org.Kontoinhaber,
			org.Steuernummer, org.Registernummer, org.LogoPath, org.ID)
		return err
	}
	return nil
}
