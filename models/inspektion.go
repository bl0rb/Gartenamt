package models

import (
	"database/sql"
	"encoding/json"
	"time"
)

type Inspektion struct {
	ID               int        `json:"id"`
	ParzelleID       int        `json:"parzelle_id"`
	Datum            time.Time  `json:"datum"`
	Maengel          []Mangel   `json:"maengel"`
	AuflagenErfuellt bool       `json:"auflagen_erfuellt"`
	Frist            *time.Time `json:"frist"`
	ErstelltAm       time.Time  `json:"erstellt_am"`
}

type Mangel struct {
	Nr              int    `json:"nr"`
	Beschreibung    string `json:"beschreibung"`
	Rechtsgrundlage string `json:"rechtsgrundlage"`
	Erfuellt        bool   `json:"erfuellt"`
}

// Vordefinierte Mängel basierend auf dem Inspektionsprotokoll
var VordefinierteManagel = []Mangel{
	{Nr: 1, Beschreibung: "Laube nicht erhaltungswürdig - Abriss einschließlich Fundament", Rechtsgrundlage: "Gartenordnung 6., BKleingG § 4"},
	{Nr: 2, Beschreibung: "Laube + Anbauten übergroß, auf 24 m² zurück bauen", Rechtsgrundlage: "Merkblatt 3.2.3.1, BKleingG § 4"},
	{Nr: 3, Beschreibung: "Laube durch feste Terrassenüberdachung über 24 m², Rückbau auf insgesamt 24 m²", Rechtsgrundlage: "BKleingG § 4"},
	{Nr: 4, Beschreibung: "Glas-/Kunststofffenster, Planen etc. über der Brüstung entfernen", Rechtsgrundlage: "Merkblatt 3.7.4"},
	{Nr: 5, Beschreibung: "Terrassenbrüstung zu hoch, auf 1 m Höhe zurückbauen", Rechtsgrundlage: "Merkblatt 3.7.5"},
	{Nr: 6, Beschreibung: "Pergolen-Zwischenwände entfernen (Zauncharakter)", Rechtsgrundlage: "Gartenordnung 5., Merkblatt 3.7.16"},
	{Nr: 7, Beschreibung: "Außenantenne entfernen", Rechtsgrundlage: "Merkblatt 3.2.3.5"},
	{Nr: 8, Beschreibung: "Wasseranschlüsse an und innerhalb der Laube entfernen", Rechtsgrundlage: "Gartenordnung 6.+9., Merkblatt 3.4.1 1."},
	{Nr: 9, Beschreibung: "WC/Dusche, Sickereinrichtung in/unter der Laube entfernen", Rechtsgrundlage: "Gartenordnung 6., Merkblatt 3.4.1 1."},
	{Nr: 10, Beschreibung: "Dach/Fußboden/Tür/Fenster, Außenwand etc. reparieren", Rechtsgrundlage: "Gartenordnung 6."},
	{Nr: 11, Beschreibung: "Ofenkonstruktionen in und an der Laube komplett entfernen", Rechtsgrundlage: "Merkblatt 3.2.3.4"},
	{Nr: 12, Beschreibung: "Laube 24 m² daher Schuppen nicht gestattet, entfernen", Rechtsgrundlage: "Merkblatt 3.7.8"},
	{Nr: 13, Beschreibung: "Geschüttete Betonkonstruktionen, z.B. Terrassen etc. entfernen", Rechtsgrundlage: "Merkblatt 3.7.3"},
	{Nr: 14, Beschreibung: "Kinderhaus zu groß, defekt entfernen", Rechtsgrundlage: "Merkblatt 3.7.9, BKleingG § 4"},
	{Nr: 15, Beschreibung: "Gewächshaus zu groß, defekt, etc. zurückbauen/entfernen", Rechtsgrundlage: "Merkblatt 3.7.7, BKleingG § 4"},
	{Nr: 16, Beschreibung: "Holzflechtzaunelemente und ähnliches entfernen", Rechtsgrundlage: "Gartenordnung 5., Merkblatt 3.7.15, 3.7.16, BKleingG § 4"},
	{Nr: 17, Beschreibung: "Zusätzliche, defekte, übergroße Pforte entfernen", Rechtsgrundlage: "Merkblatt 3.7.15"},
	{Nr: 18, Beschreibung: "Stacheldrahteinfriedigungen, Holzzäune, Wände etc. entfernen", Rechtsgrundlage: "Merkblatt 3.7.15"},
	{Nr: 19, Beschreibung: "Grillkamine, gemauert oder aus Betonfertigteilen entfernen", Rechtsgrundlage: "Merkblatt 3.11.1"},
	{Nr: 20, Beschreibung: "Überzähligen festen Wegebelag aufnehmen", Rechtsgrundlage: "Merkblatt 3.7.3"},
	{Nr: 21, Beschreibung: "Teich aus Beton, Glasfasermatten, defekte Teiche entfernen", Rechtsgrundlage: "Merkblatt 3.7.13"},
	{Nr: 22, Beschreibung: "Brenntonne, Bauschutt, Sperrmüll, Schnittgut etc. entfernen", Rechtsgrundlage: "Allgemein"},
	{Nr: 23, Beschreibung: "Entwässerungsgraben nach Anweisung instand setzen", Rechtsgrundlage: "Merkblatt 3.11.3"},
	{Nr: 24, Beschreibung: "Badebecken, Swimming-Quickpool, Großspielgerät entfernen", Rechtsgrundlage: "Allgemein"},
	{Nr: 25, Beschreibung: "Genutzte Flächen außerhalb der Parzelle komplett räumen", Rechtsgrundlage: "Merkblatt 3.10"},
	{Nr: 26, Beschreibung: "Vereinshecke auf 1 m Höhe zurück setzen, nicht vor dem 1. Oktober", Rechtsgrundlage: "Gartenordnung 5., Merkblatt 2.2.3"},
	{Nr: 27, Beschreibung: "Koniferen/Nadelgehölze mit Wurzel roden und entfernen, nicht vor dem 1. Oktober", Rechtsgrundlage: "Gartenordnung 5., Merkblatt 2.2.1"},
	{Nr: 28, Beschreibung: "Kranke Pflanzen/invasive Neophyten entfernen", Rechtsgrundlage: "Merkblatt 2.2.14"},
	{Nr: 29, Beschreibung: "Großgehölze (markiert) entfernen (mit Rodung), nicht vor dem 1. Oktober", Rechtsgrundlage: "Gartenordnung 5., Merkblatt 3.8"},
	{Nr: 30, Beschreibung: "Weitere Auflagen (siehe Zusatzblatt)", Rechtsgrundlage: "Diverses"},
	{Nr: 31, Beschreibung: "Reserviert für zusätzliche Auflagen", Rechtsgrundlage: "Diverses"},
	{Nr: 32, Beschreibung: "Reserviert für zusätzliche Auflagen", Rechtsgrundlage: "Diverses"},
}

func (i *Inspektion) Save() error {
	maengelJSON, _ := json.Marshal(i.Maengel)

	if i.ID == 0 {
		query := `INSERT INTO inspektionen (parzelle_id, datum, maengel, auflagen_erfuellt, frist) 
                  VALUES (?, ?, ?, ?, ?)`
		result, err := DB.Exec(query, i.ParzelleID, i.Datum, string(maengelJSON), i.AuflagenErfuellt, i.Frist)
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		i.ID = int(id)
	} else {
		query := `UPDATE inspektionen SET datum=?, maengel=?, auflagen_erfuellt=?, frist=? WHERE id=?`
		_, err := DB.Exec(query, i.Datum, string(maengelJSON), i.AuflagenErfuellt, i.Frist, i.ID)
		return err
	}
	return nil
}

func GetInspektionByParzelleID(parzelleID int) (*Inspektion, error) {
	query := `SELECT id, parzelle_id, datum, maengel, auflagen_erfuellt, frist, erstellt_am 
              FROM inspektionen WHERE parzelle_id=? ORDER BY datum DESC LIMIT 1`
	row := DB.QueryRow(query, parzelleID)

	var i Inspektion
	var maengelJSON string
	var frist sql.NullTime

	err := row.Scan(&i.ID, &i.ParzelleID, &i.Datum, &maengelJSON, &i.AuflagenErfuellt, &frist, &i.ErstelltAm)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(maengelJSON), &i.Maengel)
	if frist.Valid {
		i.Frist = &frist.Time
	}

	return &i, nil
}
