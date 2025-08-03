package models

import (
	"time"
)

// Obstarten Model (erweitert)
type ObstArt struct {
	ID            int       `json:"id"`
	Name          string    `json:"name"`
	Kategorie     string    `json:"kategorie"`
	Einheit       string    `json:"einheit"`
	Standardpreis float64   `json:"standardpreis"`
	MaxAnzahl     int       `json:"max_anzahl"`
	Beschreibung  string    `json:"beschreibung"`
	Aktiv         bool      `json:"aktiv"`
	ErstelltAm    time.Time `json:"erstellt_am"`
}

// Zieranpflanzungen Model (erweitert)
type Zieranpflanzung struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	Kategorie    string    `json:"kategorie"`
	PreisProQM   float64   `json:"preis_pro_qm"`
	Beschreibung string    `json:"beschreibung"`
	MaxFlaeche   *int      `json:"max_flaeche"`
	Aktiv        bool      `json:"aktiv"`
	ErstelltAm   time.Time `json:"erstellt_am"`
}

// CRUD-Funktionen für Obstarten
func GetAllObstarten() ([]ObstArt, error) {
	query := `SELECT id, name, kategorie, einheit, standardpreis, max_anzahl, 
              COALESCE(beschreibung, ''), aktiv, erstellt_am 
              FROM obstarten WHERE aktiv = TRUE ORDER BY kategorie, name`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var obstarten []ObstArt
	for rows.Next() {
		var o ObstArt
		err := rows.Scan(&o.ID, &o.Name, &o.Kategorie, &o.Einheit,
			&o.Standardpreis, &o.MaxAnzahl, &o.Beschreibung, &o.Aktiv, &o.ErstelltAm)
		if err != nil {
			return nil, err
		}
		obstarten = append(obstarten, o)
	}
	return obstarten, nil
}

func GetObstartByID(id int) (*ObstArt, error) {
	query := `SELECT id, name, kategorie, einheit, standardpreis, max_anzahl, 
              COALESCE(beschreibung, ''), aktiv, erstellt_am 
              FROM obstarten WHERE id = ?`
	row := DB.QueryRow(query, id)

	var o ObstArt
	err := row.Scan(&o.ID, &o.Name, &o.Kategorie, &o.Einheit,
		&o.Standardpreis, &o.MaxAnzahl, &o.Beschreibung, &o.Aktiv, &o.ErstelltAm)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// CRUD-Funktionen für Zieranpflanzungen
func GetAllZieranpflanzungen() ([]Zieranpflanzung, error) {
	query := `SELECT id, name, kategorie, preis_pro_qm, 
              COALESCE(beschreibung, ''), max_flaeche, aktiv, erstellt_am 
              FROM zieranpflanzungen WHERE aktiv = TRUE ORDER BY kategorie, name`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var zieranpflanzungen []Zieranpflanzung
	for rows.Next() {
		var z Zieranpflanzung
		var maxFlaeche *int
		err := rows.Scan(&z.ID, &z.Name, &z.Kategorie, &z.PreisProQM,
			&z.Beschreibung, &maxFlaeche, &z.Aktiv, &z.ErstelltAm)
		if err != nil {
			return nil, err
		}
		z.MaxFlaeche = maxFlaeche
		zieranpflanzungen = append(zieranpflanzungen, z)
	}
	return zieranpflanzungen, nil
}

func GetZieranpflanzungByID(id int) (*Zieranpflanzung, error) {
	query := `SELECT id, name, kategorie, preis_pro_qm, 
              COALESCE(beschreibung, ''), max_flaeche, aktiv, erstellt_am 
              FROM zieranpflanzungen WHERE id = ?`
	row := DB.QueryRow(query, id)

	var z Zieranpflanzung
	var maxFlaeche *int
	err := row.Scan(&z.ID, &z.Name, &z.Kategorie, &z.PreisProQM,
		&z.Beschreibung, &maxFlaeche, &z.Aktiv, &z.ErstelltAm)
	if err != nil {
		return nil, err
	}
	z.MaxFlaeche = maxFlaeche
	return &z, nil
}
