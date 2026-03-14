package services

import (
	"database/sql"
	"kleingarten-verwaltung/models"
)

type BerechnungsService struct {
	db *sql.DB
}

type PreisInfo struct {
	ID            int     `json:"id"`
	Name          string  `json:"name"`
	Kategorie     string  `json:"kategorie"`
	Einheit       string  `json:"einheit"`
	Standardpreis float64 `json:"standardpreis"`
	MaxAnzahl     int     `json:"max_anzahl"`
	MaxFlaeche    *int    `json:"max_flaeche,omitempty"`
}

func NewBerechnungsService(db *sql.DB) *BerechnungsService {
	return &BerechnungsService{db: db}
}

// Obstarten-Berechnungen
func (s *BerechnungsService) GetAllObstartenPreise() (map[string]PreisInfo, error) {
	query := `SELECT id, name, kategorie, einheit, standardpreis, max_anzahl 
              FROM obstarten WHERE aktiv = TRUE ORDER BY kategorie, name`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	preise := make(map[string]PreisInfo)
	for rows.Next() {
		var p PreisInfo
		err := rows.Scan(&p.ID, &p.Name, &p.Kategorie, &p.Einheit, &p.Standardpreis, &p.MaxAnzahl)
		if err != nil {
			return nil, err
		}
		preise[p.Name] = p
	}
	return preise, nil
}

func (s *BerechnungsService) GetObstartPreis(obstart string) (PreisInfo, error) {
	query := `SELECT id, name, kategorie, einheit, standardpreis, max_anzahl 
              FROM obstarten WHERE name = ? AND aktiv = TRUE`
	row := s.db.QueryRow(query, obstart)

	var p PreisInfo
	err := row.Scan(&p.ID, &p.Name, &p.Kategorie, &p.Einheit, &p.Standardpreis, &p.MaxAnzahl)
	if err != nil {
		return PreisInfo{}, err
	}
	return p, nil
}

func (s *BerechnungsService) BerechneObstWert(obstEintraege []models.ObstDetail) (float64, error) {
	var gesamtwert float64

	for _, eintrag := range obstEintraege {
		preisInfo, err := s.GetObstartPreis(eintrag.Art)
		if err != nil {
			// Fallback auf eingegebenen Preis falls nicht in DB gefunden
			gesamtwert += float64(eintrag.Anzahl) * eintrag.EinzelPreis
			continue
		}

		// Verwende DB-Preis, aber erlaube Überschreibung
		preis := preisInfo.Standardpreis
		if eintrag.EinzelPreis > 0 {
			preis = eintrag.EinzelPreis
		}

		gesamtwert += float64(eintrag.Anzahl) * preis
	}

	return gesamtwert, nil
}

// Zieranpflanzungen-Berechnungen
func (s *BerechnungsService) GetAllZieranpflanzungsPreise() (map[string]PreisInfo, error) {
	query := `SELECT id, name, kategorie, preis_pro_qm, max_flaeche 
              FROM zieranpflanzungen WHERE aktiv = TRUE ORDER BY kategorie, name`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	preise := make(map[string]PreisInfo)
	for rows.Next() {
		var p PreisInfo
		var maxFlaeche sql.NullInt64
		err := rows.Scan(&p.ID, &p.Name, &p.Kategorie, &p.Standardpreis, &maxFlaeche)
		if err != nil {
			return nil, err
		}

		p.Einheit = "m²"
		if maxFlaeche.Valid {
			val := int(maxFlaeche.Int64)
			p.MaxFlaeche = &val
		}

		preise[p.Kategorie] = p
	}
	return preise, nil
}

func (s *BerechnungsService) GetZieranpflanzungPreis(kategorie string) (float64, error) {
	query := `SELECT preis_pro_qm FROM zieranpflanzungen WHERE kategorie = ? AND aktiv = TRUE`
	row := s.db.QueryRow(query, kategorie)

	var preis float64
	err := row.Scan(&preis)
	if err != nil {
		// Fallback auf hardcoded Werte falls nicht in DB
		fallbackPreise := map[string]float64{
			"F1": 4.00, "F2": 5.00, "F3": 6.00, "F4": 12.00,
			"F5": 5.00, "F6": 7.00, "F7": 15.00, "F8": 0.50,
		}
		if p, ok := fallbackPreise[kategorie]; ok {
			return p, nil
		}
		return 0, err
	}
	return preis, nil
}

func (s *BerechnungsService) BerechneZierWert(zierEintraege map[string]float64) (float64, error) {
	var gesamtwert float64

	for kategorie, flaeche := range zierEintraege {
		if flaeche <= 0 {
			continue
		}

		preis, err := s.GetZieranpflanzungPreis(kategorie)
		if err != nil {
			continue // Überspringe unbekannte Kategorien
		}

		gesamtwert += flaeche * preis
	}

	return gesamtwert, nil
}

// Gemüse-Berechnungen
func (s *BerechnungsService) GetGemusePreise() (map[string]float64, error) {
	query := `SELECT kategorie, preis_pro_einheit FROM gemuese_arten WHERE aktiv = TRUE`
	rows, err := s.db.Query(query)
	if err != nil {
		// Fallback auf hardcoded Werte
		return map[string]float64{
			"G1": 0.75, // Einzelpflanzen
			"G2": 1.00, // Reihenpflanzungen
			"E9": 5.00, // Kräuter
		}, nil
	}
	defer rows.Close()

	preise := make(map[string]float64)
	for rows.Next() {
		var kategorie string
		var preis float64
		err := rows.Scan(&kategorie, &preis)
		if err != nil {
			continue
		}
		preise[kategorie] = preis
	}
	return preise, nil
}

func (s *BerechnungsService) BerechneGemuseWert(einzelpflanzen int, reihenMeter, kraeuterFlaeche float64) (float64, error) {
	preise, err := s.GetGemusePreise()
	if err != nil {
		return 0, err
	}

	gesamtwert := 0.0

	// G1 - Einzelpflanzen
	if preis, ok := preise["G1"]; ok {
		gesamtwert += float64(einzelpflanzen) * preis
	}

	// G2 - Reihenpflanzungen
	if preis, ok := preise["G2"]; ok {
		gesamtwert += reihenMeter * preis
	}

	// E9 - Kräuter
	if preis, ok := preise["E9"]; ok {
		gesamtwert += kraeuterFlaeche * preis
	}

	return gesamtwert, nil
}

// Baulichkeiten-Berechnungen
type BaulichkeitInfo struct {
	Name                string  `json:"name"`
	Kategorie           string  `json:"kategorie"`
	Standardpreis       float64 `json:"standardpreis"`
	Einheit             string  `json:"einheit"`
	AbschreibungProzent float64 `json:"abschreibung_prozent"`
	LebensdauerJahre    int     `json:"lebensdauer_jahre"`
}

func (s *BerechnungsService) GetBaulichkeitenPreise() (map[string]BaulichkeitInfo, error) {
	query := `SELECT name, kategorie, standardpreis, einheit, abschreibung_prozent, lebensdauer_jahre 
              FROM baulichkeiten_arten WHERE aktiv = TRUE`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	infos := make(map[string]BaulichkeitInfo)
	for rows.Next() {
		var info BaulichkeitInfo
		err := rows.Scan(&info.Name, &info.Kategorie, &info.Standardpreis,
			&info.Einheit, &info.AbschreibungProzent, &info.LebensdauerJahre)
		if err != nil {
			continue
		}
		infos[info.Kategorie] = info
	}
	return infos, nil
}

// Bauindex-Berechnungen
func (s *BerechnungsService) GetAktuellerBauindex() (float64, error) {
	query := `SELECT bauindex FROM bauindex_tabelle ORDER BY jahr DESC LIMIT 1`
	row := s.db.QueryRow(query)

	var bauindex float64
	err := row.Scan(&bauindex)
	if err != nil {
		return 44.3, nil // Fallback auf aktuellen Wert 2025
	}
	return bauindex, nil
}

func (s *BerechnungsService) GetBauindexByJahr(jahr int) (float64, error) {
	query := `SELECT bauindex FROM bauindex_tabelle WHERE jahr = ?`
	row := s.db.QueryRow(query, jahr)

	var bauindex float64
	err := row.Scan(&bauindex)
	if err != nil {
		// Fallback auf aktuellen Bauindex
		return s.GetAktuellerBauindex()
	}
	return bauindex, nil
}

// Vollständige Wertermittlungs-Berechnung
func (s *BerechnungsService) BerechneGesamtWertermittlung(
	laubeWert, wegeWert, pforteWert, stromWert, wasserWert float64,
	obstEintraege []models.ObstDetail,
	einzelpflanzen int, reihenMeter, kraeuterFlaeche float64,
	zierEintraege map[string]float64) (models.Wertermittlung, error) {

	var wertermittlung models.Wertermittlung

	// Baulichkeiten
	wertermittlung.BaulichkeitenWert = wegeWert + pforteWert + stromWert + wasserWert

	// Obst
	obstWert, err := s.BerechneObstWert(obstEintraege)
	if err != nil {
		return wertermittlung, err
	}
	wertermittlung.ObstWert = obstWert

	// Gemüse
	gemuseWert, err := s.BerechneGemuseWert(einzelpflanzen, reihenMeter, kraeuterFlaeche)
	if err != nil {
		return wertermittlung, err
	}
	wertermittlung.GemuseWert = gemuseWert

	// Zier
	zierWert, err := s.BerechneZierWert(zierEintraege)
	if err != nil {
		return wertermittlung, err
	}
	wertermittlung.ZierWert = zierWert

	// Laube (wird separat berechnet)
	wertermittlung.LaubeWert = laubeWert

	// Gesamtwert
	wertermittlung.BerechneGesamtwert()

	return wertermittlung, nil
}
