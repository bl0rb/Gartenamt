package models

import (
	"time"
)

// Wasser - Water consumption record per Parzelle
type Wasser struct {
	ID         int       `json:"id"`
	ParzelleID int       `json:"parzelle_id"`
	Monat      int       `json:"monat"` // 1-12
	Jahr       int       `json:"jahr"`
	Verbrauch  float64   `json:"verbrauch"` // in m³
	Kosten     float64   `json:"kosten"`    // in EUR
	Noten      string    `json:"noten"`
	ErstelltAm time.Time `json:"erstellt_am"`
}

// Strom - Electricity consumption record per Parzelle
type Strom struct {
	ID         int       `json:"id"`
	ParzelleID int       `json:"parzelle_id"`
	Monat      int       `json:"monat"` // 1-12
	Jahr       int       `json:"jahr"`
	Verbrauch  float64   `json:"verbrauch"` // in kWh
	Kosten     float64   `json:"kosten"`    // in EUR
	Noten      string    `json:"noten"`
	ErstelltAm time.Time `json:"erstellt_am"`
}

// SaveWasser saves a water consumption record
func (w *Wasser) Save() error {
	if w.ID == 0 {
		query := `INSERT INTO wasser (parzelle_id, monat, jahr, verbrauch, kosten, noten) 
                  VALUES (?, ?, ?, ?, ?, ?)`
		result, err := DB.Exec(query, w.ParzelleID, w.Monat, w.Jahr, w.Verbrauch, w.Kosten, w.Noten)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		w.ID = int(id)
	} else {
		query := `UPDATE wasser SET parzelle_id=?, monat=?, jahr=?, verbrauch=?, kosten=?, noten=? WHERE id=?`
		_, err := DB.Exec(query, w.ParzelleID, w.Monat, w.Jahr, w.Verbrauch, w.Kosten, w.Noten, w.ID)
		return err
	}
	return nil
}

// SaveStrom saves an electricity consumption record
func (s *Strom) Save() error {
	if s.ID == 0 {
		query := `INSERT INTO strom (parzelle_id, monat, jahr, verbrauch, kosten, noten) 
                  VALUES (?, ?, ?, ?, ?, ?)`
		result, err := DB.Exec(query, s.ParzelleID, s.Monat, s.Jahr, s.Verbrauch, s.Kosten, s.Noten)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		s.ID = int(id)
	} else {
		query := `UPDATE strom SET parzelle_id=?, monat=?, jahr=?, verbrauch=?, kosten=?, noten=? WHERE id=?`
		_, err := DB.Exec(query, s.ParzelleID, s.Monat, s.Jahr, s.Verbrauch, s.Kosten, s.Noten, s.ID)
		return err
	}
	return nil
}

// GetWasserByParzelle gets all water records for a parzelle
func GetWasserByParzelle(parzelleID int) ([]Wasser, error) {
	query := `SELECT id, parzelle_id, monat, jahr, verbrauch, kosten, noten, erstellt_am 
              FROM wasser WHERE parzelle_id=? ORDER BY jahr DESC, monat DESC`
	rows, err := DB.Query(query, parzelleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Wasser
	for rows.Next() {
		var w Wasser
		err := rows.Scan(&w.ID, &w.ParzelleID, &w.Monat, &w.Jahr, &w.Verbrauch, &w.Kosten, &w.Noten, &w.ErstelltAm)
		if err != nil {
			return nil, err
		}
		records = append(records, w)
	}
	return records, nil
}

// GetStromByParzelle gets all electricity records for a parzelle
func GetStromByParzelle(parzelleID int) ([]Strom, error) {
	query := `SELECT id, parzelle_id, monat, jahr, verbrauch, kosten, noten, erstellt_am 
              FROM strom WHERE parzelle_id=? ORDER BY jahr DESC, monat DESC`
	rows, err := DB.Query(query, parzelleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []Strom
	for rows.Next() {
		var s Strom
		err := rows.Scan(&s.ID, &s.ParzelleID, &s.Monat, &s.Jahr, &s.Verbrauch, &s.Kosten, &s.Noten, &s.ErstelltAm)
		if err != nil {
			return nil, err
		}
		records = append(records, s)
	}
	return records, nil
}

// GetWasserByID retrieves a single water record
func GetWasserByID(id int) (*Wasser, error) {
	query := `SELECT id, parzelle_id, monat, jahr, verbrauch, kosten, noten, erstellt_am FROM wasser WHERE id=?`
	row := DB.QueryRow(query, id)

	var w Wasser
	err := row.Scan(&w.ID, &w.ParzelleID, &w.Monat, &w.Jahr, &w.Verbrauch, &w.Kosten, &w.Noten, &w.ErstelltAm)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

// GetStromByID retrieves a single electricity record
func GetStromByID(id int) (*Strom, error) {
	query := `SELECT id, parzelle_id, monat, jahr, verbrauch, kosten, noten, erstellt_am FROM strom WHERE id=?`
	row := DB.QueryRow(query, id)

	var s Strom
	err := row.Scan(&s.ID, &s.ParzelleID, &s.Monat, &s.Jahr, &s.Verbrauch, &s.Kosten, &s.Noten, &s.ErstelltAm)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// DeleteWasser deletes a water record
func DeleteWasser(id int) error {
	_, err := DB.Exec("DELETE FROM wasser WHERE id=?", id)
	return err
}

// DeleteStrom deletes an electricity record
func DeleteStrom(id int) error {
	_, err := DB.Exec("DELETE FROM strom WHERE id=?", id)
	return err
}
