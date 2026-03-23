package models

import (
	"database/sql"
	"encoding/json"
	"math"
	"time"
)

type Wertermittlung struct {
	ID                int                   `json:"id"`
	ParzelleID        int                   `json:"parzelle_id"`
	InspektionID      *int                  `json:"inspektion_id"`
	Datum             time.Time             `json:"datum"`
	LaubeWert         float64               `json:"laube_wert"`
	BaulichkeitenWert float64               `json:"baulichkeiten_wert"`
	GemuseWert        float64               `json:"gemuese_wert"`
	ObstWert          float64               `json:"obst_wert"`
	ZierWert          float64               `json:"zier_wert"`
	GesamtWert        float64               `json:"gesamt_wert"`
	Details           WertermittlungDetails `json:"details"`
	ErstelltAm        time.Time             `json:"erstellt_am"`
}

type WertermittlungDetails struct {
	Laube         LaubeDetails        `json:"laube"`
	Baulichkeiten []BaulichkeitDetail `json:"baulichkeiten"`
	Obst          []ObstDetail        `json:"obst"`
	Gemuese       []GemuseDetail      `json:"gemuese"`
	Zier          ZierDetail          `json:"zier"`
}

// ERWEITERTE LaubeDetails mit manueller Eingabe
type LaubeDetails struct {
	Bauklasse             string  `json:"bauklasse"`
	Erstellungsjahr       int     `json:"erstellungsjahr"`
	Grundflaeche          float64 `json:"grundflaeche"`
	HerstellungswertProQM float64 `json:"herstellungswert_pro_qm"`
	Bauindex              float64 `json:"bauindex"`
	AbschreibungProzent   float64 `json:"abschreibung_prozent"`
	AbschreibungJahre     int     `json:"abschreibung_jahre"`
	Zeitwert              float64 `json:"zeitwert"`

	// NEU: Manuelle Eingabe-Funktionen
	ManuellEingegeben bool    `json:"manuell_eingegeben"`
	RestwertProzent   float64 `json:"restwert_prozent"` // max 15%
	ManuellZeitwert   float64 `json:"manuell_zeitwert"`
	Begruendung       string  `json:"begruendung"` // Grund für manuelle Eingabe
}

type BaulichkeitDetail struct {
	Typ                 string  `json:"typ"`
	Anzahl              int     `json:"anzahl"`
	Flaeche             float64 `json:"flaeche"`
	Qualitaet           float64 `json:"qualitaet"`
	Baujahr             int     `json:"baujahr"`
	Aktiv               bool    `json:"aktiv"`
	Herstellungswert    float64 `json:"herstellungswert"`
	BelegVorhanden      bool    `json:"beleg_vorhanden"`
	Quelle              string  `json:"quelle"`
	Bewertungsgrund     string  `json:"bewertungsgrund"`
	AbschreibungProzent float64 `json:"abschreibung_prozent"`
	Restwert            float64 `json:"restwert"`
}

// KORRIGIERTE ObstDetail mit EinzelPreis
type ObstDetail struct {
	ID          int     `json:"id"`
	Kategorie   string  `json:"kategorie"`
	Art         string  `json:"art"`
	Anzahl      int     `json:"anzahl"`
	Einheit     string  `json:"einheit"`
	EinzelPreis float64 `json:"einzel_preis"` // KORRIGIERT: richtige Feldname
	GesamtWert  float64 `json:"gesamt_wert"`
	Zustand     string  `json:"zustand"`
}

type GemuseDetail struct {
	Art   string  `json:"art"`
	Menge float64 `json:"menge"`
	Wert  float64 `json:"wert"`
}

type ZierDetail struct {
	Flaeche      float64 `json:"flaeche"`
	Kategorie    string  `json:"kategorie"`
	EuroProQM    float64 `json:"euro_pro_qm"`
	Gesamtwert   float64 `json:"gesamtwert"`
	F1           float64 `json:"f1"`
	F2           float64 `json:"f2"`
	F3           float64 `json:"f3"`
	F4           float64 `json:"f4"`
	F8           float64 `json:"f8"`
	TeichFlaeche float64 `json:"teich_flaeche"`
	TeichTyp     string  `json:"teich_typ"`
}

// KORRIGIERTE Bauindex-Tabelle mit aktuellen Daten bis 2026
var BauindexTabelle = map[int]float64{
	2026: 44.3, 2025: 42.9, 2024: 41.8, 2023: 38.4, 2022: 32.6, 2021: 30.7, 2020: 29.8, 2019: 28.0, 2018: 27.3, 2017: 26.6, 2016: 26.0,
	2015: 25.6, 2014: 25.2, 2013: 24.7, 2012: 24.1, 2011: 23.4, 2010: 23.2, 2009: 23.3, 2008: 22.5, 2007: 20.9, 2006: 20.6,
	2005: 20.4, 2004: 20.1, 2003: 20.2, 2002: 20.2, 2001: 20.2, 2000: 20.1, 1999: 20.3, 1998: 20.3, 1997: 20.5, 1996: 23.3,
	1995: 22.3, 1994: 22.8, 1993: 21.7, 1992: 20.7, 1991: 19.5, 1990: 18.2, 1989: 17.8, 1988: 17.6, 1987: 17.5, 1986: 17.3,
	1985: 17.3, 1984: 17.0, 1983: 16.5, 1982: 15.9, 1981: 15.1, 1980: 13.9, 1979: 12.8, 1978: 12.2, 1977: 11.7, 1976: 11.1,
	1975: 11.0, 1974: 9.9,
}

// HINZUGEFÜGTE ObstPreisTabelle Variable
var ObstPreisTabelle = map[string]ObstArt{
	// E1 - Obstbäume
	"Apfel": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 25.0,
		MaxAnzahl:     999,
	},
	"Birne": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 25.0,
		MaxAnzahl:     999,
	},
	"Kirsche süß": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 30.0,
		MaxAnzahl:     999,
	},
	"Kirsche sauer": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 25.0,
		MaxAnzahl:     999,
	},
	"Pflaume": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 25.0,
		MaxAnzahl:     999,
	},
	"Zwetschge": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 25.0,
		MaxAnzahl:     999,
	},
	"Quitte": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 20.0,
		MaxAnzahl:     999,
	},
	"Pfirsich": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 30.0,
		MaxAnzahl:     999,
	},
	"Aprikose": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 35.0,
		MaxAnzahl:     999,
	},
	"Nektarine": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 30.0,
		MaxAnzahl:     999,
	},
	"Walnuss": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 50.0,
		MaxAnzahl:     999,
	},
	"Haselnuss": {
		Kategorie:     "E1",
		Einheit:       "Stück",
		Standardpreis: 20.0,
		MaxAnzahl:     999,
	},

	// E2 - Beerensträucher
	"Johannisbeeren rot": {
		Kategorie:     "E2",
		Einheit:       "Stück",
		Standardpreis: 10.0,
		MaxAnzahl:     999,
	},
	"Johannisbeeren schwarz": {
		Kategorie:     "E2",
		Einheit:       "Stück",
		Standardpreis: 10.0,
		MaxAnzahl:     999,
	},
	"Stachelbeeren": {
		Kategorie:     "E2",
		Einheit:       "Stück",
		Standardpreis: 10.0,
		MaxAnzahl:     999,
	},
	"Josta": {
		Kategorie:     "E2",
		Einheit:       "Stück",
		Standardpreis: 10.0,
		MaxAnzahl:     999,
	},
	"Cranberry": {
		Kategorie:     "E2",
		Einheit:       "Stück",
		Standardpreis: 10.0,
		MaxAnzahl:     999,
	},

	// E3 - Hochstämme
	"Stachelbeeren Hochstamm": {
		Kategorie:     "E3",
		Einheit:       "Stück",
		Standardpreis: 20.0,
		MaxAnzahl:     999,
	},
	"Johannisbeeren Hochstamm": {
		Kategorie:     "E3",
		Einheit:       "Stück",
		Standardpreis: 20.0,
		MaxAnzahl:     999,
	},

	// E4 - Beeren-/Wildobst
	"Blaubeeren": {
		Kategorie:     "E4",
		Einheit:       "Stück",
		Standardpreis: 15.0,
		MaxAnzahl:     999,
	},
	"Heidelbeeren": {
		Kategorie:     "E4",
		Einheit:       "Stück",
		Standardpreis: 15.0,
		MaxAnzahl:     999,
	},
	"Aronia": {
		Kategorie:     "E4",
		Einheit:       "Stück",
		Standardpreis: 12.0,
		MaxAnzahl:     999,
	},
	"Sanddorn": {
		Kategorie:     "E4",
		Einheit:       "Stück",
		Standardpreis: 12.0,
		MaxAnzahl:     999,
	},
	"Holunder": {
		Kategorie:     "E4",
		Einheit:       "Stück",
		Standardpreis: 10.0,
		MaxAnzahl:     999,
	},

	// E5 - Kulturbrombeeren (max 5 Stück)
	"Brombeeren kultiviert": {
		Kategorie:     "E5",
		Einheit:       "Stück",
		Standardpreis: 10.0,
		MaxAnzahl:     5,
	},
	"Taybeeren": {
		Kategorie:     "E5",
		Einheit:       "Stück",
		Standardpreis: 10.0,
		MaxAnzahl:     5,
	},

	// E6 - Weinreben/Kiwi (max 5 Stück)
	"Weinreben": {
		Kategorie:     "E6",
		Einheit:       "Stück",
		Standardpreis: 20.0,
		MaxAnzahl:     5,
	},
	"Kiwi": {
		Kategorie:     "E6",
		Einheit:       "Stück",
		Standardpreis: 20.0,
		MaxAnzahl:     5,
	},
	"Mini-Kiwi": {
		Kategorie:     "E6",
		Einheit:       "Stück",
		Standardpreis: 18.0,
		MaxAnzahl:     5,
	},

	// E7 - Rhabarber (max 10 Stück)
	"Rhabarber": {
		Kategorie:     "E7",
		Einheit:       "Stück",
		Standardpreis: 5.0,
		MaxAnzahl:     10,
	},

	// E8 - Erdbeeren (max 15 m²)
	"Erdbeeren": {
		Kategorie:     "E8",
		Einheit:       "m²",
		Standardpreis: 5.0,
		MaxAnzahl:     15,
	},

	// E10 - Himbeeren (max 10 lfm)
	"Himbeeren": {
		Kategorie:     "E10",
		Einheit:       "lfm",
		Standardpreis: 8.0,
		MaxAnzahl:     10,
	},

	// E11 - Spargel (max 15 lfm)
	"Spargel": {
		Kategorie:     "E11",
		Einheit:       "lfm",
		Standardpreis: 10.0,
		MaxAnzahl:     15,
	},
}

// ERWEITERTE Lauben-Berechnung mit manueller Option
func (w *Wertermittlung) BerechneLaubeWert(laube LaubeDetails, aktuellerBauindex float64) float64 {
	// Falls manuell eingegeben, verwende manuellen Wert
	if laube.ManuellEingegeben {
		// Validierung: Max 15% Restwert
		if laube.RestwertProzent > 15.0 {
			laube.RestwertProzent = 15.0
		}
		if laube.RestwertProzent < 0.0 {
			laube.RestwertProzent = 0.0
		}

		// Manueller Zeitwert direkt verwenden falls angegeben
		if laube.ManuellZeitwert > 0 {
			return laube.ManuellZeitwert
		}

		// Oder aus Restwert-Prozent berechnen
		grundwert := laube.HerstellungswertProQM * laube.Grundflaeche * aktuellerBauindex
		return grundwert * laube.RestwertProzent / 100
	}

	// AUTOMATISCHE Berechnung (bestehende Logik, korrigiert)
	grundwert := laube.HerstellungswertProQM * laube.Grundflaeche * aktuellerBauindex

	aktuellesJahr := time.Now().Year()

	// KORRIGIERT: Abschreibung beginnt im Jahr NACH Erstellung
	abschreibungsjahre := aktuellesJahr - laube.Erstellungsjahr - 1
	if abschreibungsjahre < 0 {
		abschreibungsjahre = 0
	}

	// Erwartete Lebensdauer nach Material
	var maxLebensdauer int
	var minRestwertProzent float64 = 15.0

	if laube.AbschreibungProzent <= 2.0 { // Steinlaube
		maxLebensdauer = 50
	} else { // Holzlaube
		maxLebensdauer = 33
	}

	// Abschreibung berechnen
	gesamtabschreibung := float64(abschreibungsjahre) * laube.AbschreibungProzent

	// Nach max. Lebensdauer: max. 15% Restwert
	if abschreibungsjahre >= maxLebensdauer {
		gesamtabschreibung = 85.0
	} else if gesamtabschreibung > 85.0 {
		gesamtabschreibung = 85.0
	}

	abschreibungsbetrag := grundwert * gesamtabschreibung / 100
	zeitwert := grundwert - abschreibungsbetrag

	// Mindestens 15% Restwert
	mindestRestwert := grundwert * minRestwertProzent / 100

	return math.Max(zeitwert, mindestRestwert)
}

// NEU: Validierungsfunktion für manuelle Eingabe
func (l *LaubeDetails) ValidiereManuellEingabe() []string {
	var fehler []string

	if l.ManuellEingegeben {
		if l.RestwertProzent > 15.0 {
			fehler = append(fehler, "Restwert darf maximal 15% betragen")
		}
		if l.RestwertProzent < 0.0 {
			fehler = append(fehler, "Restwert kann nicht negativ sein")
		}
		if l.ManuellZeitwert < 0.0 {
			fehler = append(fehler, "Manueller Zeitwert kann nicht negativ sein")
		}
		if l.Begruendung == "" {
			fehler = append(fehler, "Begründung für manuelle Eingabe ist erforderlich")
		}
	}

	return fehler
}

func (w *Wertermittlung) BerechneGesamtwert() {
	w.GesamtWert = w.LaubeWert + w.BaulichkeitenWert + w.GemuseWert + w.ObstWert + w.ZierWert
}

func (w *Wertermittlung) Save() error {
	w.BerechneGesamtwert()
	detailsJSON, _ := json.Marshal(w.Details)

	if w.ID == 0 {
		query := `INSERT INTO wertermittlungen (parzelle_id, inspektion_id, datum, laube_wert, 
                  baulichkeiten_wert, gemuese_wert, obst_wert, zier_wert, gesamt_wert, details) 
                  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		result, err := DB.Exec(query, w.ParzelleID, w.InspektionID, w.Datum, w.LaubeWert,
			w.BaulichkeitenWert, w.GemuseWert, w.ObstWert, w.ZierWert, w.GesamtWert, string(detailsJSON))
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		w.ID = int(id)
	} else {
		query := `UPDATE wertermittlungen SET datum=?, laube_wert=?, baulichkeiten_wert=?, 
                  gemuese_wert=?, obst_wert=?, zier_wert=?, gesamt_wert=?, details=? WHERE id=?`
		_, err := DB.Exec(query, w.Datum, w.LaubeWert, w.BaulichkeitenWert, w.GemuseWert,
			w.ObstWert, w.ZierWert, w.GesamtWert, string(detailsJSON), w.ID)
		return err
	}
	return nil
}

func GetWertermittlungByParzelleID(parzelleID int) (*Wertermittlung, error) {
	query := `SELECT id, parzelle_id, inspektion_id, datum, laube_wert, baulichkeiten_wert, 
              gemuese_wert, obst_wert, zier_wert, gesamt_wert, details, erstellt_am 
              FROM wertermittlungen WHERE parzelle_id=? ORDER BY datum DESC LIMIT 1`
	row := DB.QueryRow(query, parzelleID)

	var w Wertermittlung
	var detailsJSON string
	var inspektionID sql.NullInt64

	err := row.Scan(&w.ID, &w.ParzelleID, &inspektionID, &w.Datum, &w.LaubeWert,
		&w.BaulichkeitenWert, &w.GemuseWert, &w.ObstWert, &w.ZierWert, &w.GesamtWert,
		&detailsJSON, &w.ErstelltAm)
	if err != nil {
		return nil, err
	}

	if inspektionID.Valid {
		id := int(inspektionID.Int64)
		w.InspektionID = &id
	}
	json.Unmarshal([]byte(detailsJSON), &w.Details)

	return &w, nil
}
