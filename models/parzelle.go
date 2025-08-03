package models

import (
	"database/sql"
	"time"
)

type Parzelle struct {
	ID              int        `json:"id"`
	Nummer          string     `json:"nummer"`
	Groesse         float64    `json:"groesse"`
	Verein          string     `json:"verein"`
	PaechterName    string     `json:"paechter_name"`
	Email           string     `json:"email"`   // NEU
	Telefon         string     `json:"telefon"` // NEU
	Notizen         string     `json:"notizen"` // NEU
	KuendigungDatum *time.Time `json:"kuendigung_datum"`
	ErstelltAm      time.Time  `json:"erstellt_am"`
}

func (p *Parzelle) Save() error {
	if p.ID == 0 {
		query := `INSERT INTO parzellen (nummer, groesse, verein, paechter_name, email, telefon, notizen, kuendigung_datum) 
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
		result, err := DB.Exec(query, p.Nummer, p.Groesse, p.Verein, p.PaechterName, p.Email, p.Telefon, p.Notizen, p.KuendigungDatum)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		p.ID = int(id)
	} else {
		query := `UPDATE parzellen SET nummer=?, groesse=?, verein=?, paechter_name=?, email=?, telefon=?, notizen=?, kuendigung_datum=? WHERE id=?`
		_, err := DB.Exec(query, p.Nummer, p.Groesse, p.Verein, p.PaechterName, p.Email, p.Telefon, p.Notizen, p.KuendigungDatum, p.ID)
		return err
	}
	return nil
}

func GetAllParzellen() ([]Parzelle, error) {
	query := `SELECT id, nummer, groesse, verein, paechter_name, email, telefon, notizen, kuendigung_datum, erstellt_am FROM parzellen ORDER BY nummer`
	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var parzellen []Parzelle
	for rows.Next() {
		var p Parzelle
		var kuendigungDatum sql.NullTime
		err := rows.Scan(&p.ID, &p.Nummer, &p.Groesse, &p.Verein, &p.PaechterName,
			&p.Email, &p.Telefon, &p.Notizen, &kuendigungDatum, &p.ErstelltAm)
		if err != nil {
			return nil, err
		}
		if kuendigungDatum.Valid {
			p.KuendigungDatum = &kuendigungDatum.Time
		}
		parzellen = append(parzellen, p)
	}
	return parzellen, nil
}

func GetParzelleByID(id int) (*Parzelle, error) {
	query := `SELECT id, nummer, groesse, verein, paechter_name, email, telefon, notizen, kuendigung_datum, erstellt_am FROM parzellen WHERE id=?`
	row := DB.QueryRow(query, id)

	var p Parzelle
	var kuendigungDatum sql.NullTime
	err := row.Scan(&p.ID, &p.Nummer, &p.Groesse, &p.Verein, &p.PaechterName,
		&p.Email, &p.Telefon, &p.Notizen, &kuendigungDatum, &p.ErstelltAm)
	if err != nil {
		return nil, err
	}
	if kuendigungDatum.Valid {
		p.KuendigungDatum = &kuendigungDatum.Time
	}
	return &p, nil
}
