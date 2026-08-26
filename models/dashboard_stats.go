package models

import "time"

// DashboardAktivitaet ist ein Eintrag im "Letzte Aktivitäten"-Block des
// Dashboards - siehe GetLetzteAktivitaeten.
type DashboardAktivitaet struct {
	Typ            string // "inspektion" | "wertermittlung" | "parzelle"
	Zeit           time.Time
	ParzelleID     int
	ParzelleNummer string
}

// Ereignis liefert den Anzeigetext für den Aktivitätstyp.
func (a DashboardAktivitaet) Ereignis() string {
	switch a.Typ {
	case "inspektion":
		return "Inspektion durchgeführt"
	case "wertermittlung":
		return "Wertermittlung durchgeführt"
	case "parzelle":
		return "Parzelle angelegt"
	default:
		return "Ereignis"
	}
}

// IconClass liefert die FontAwesome-Klasse für den Aktivitätstyp.
func (a DashboardAktivitaet) IconClass() string {
	switch a.Typ {
	case "inspektion":
		return "fa-clipboard-check"
	case "wertermittlung":
		return "fa-calculator"
	case "parzelle":
		return "fa-map"
	default:
		return "fa-circle"
	}
}

// GetLetzteAktivitaeten lädt die letzten Ereignisse für den dritten
// Dashboard-Block ("Letzte Aktivitäten", nach Handlungsbedarf und
// Kennzahlen). Analog zu GetDashboardHandlungsbedarf per UNION über die
// erstellt_am-Spalten der beteiligten Tabellen, absteigend sortiert.
func GetLetzteAktivitaeten(limit int) []DashboardAktivitaet {
	var aktivitaeten []DashboardAktivitaet

	rows, err := DB.Query(`
		SELECT 'inspektion' AS typ, i.erstellt_am AS zeit, i.parzelle_id AS parzelle_id, p.nummer AS nummer
		FROM inspektionen i
		JOIN parzellen p ON p.id = i.parzelle_id
		UNION ALL
		SELECT 'wertermittlung' AS typ, w.erstellt_am AS zeit, w.parzelle_id AS parzelle_id, p.nummer AS nummer
		FROM wertermittlungen w
		JOIN parzellen p ON p.id = w.parzelle_id
		UNION ALL
		SELECT 'parzelle' AS typ, p.erstellt_am AS zeit, p.id AS parzelle_id, p.nummer AS nummer
		FROM parzellen p
		ORDER BY zeit DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return aktivitaeten
	}
	defer rows.Close()

	for rows.Next() {
		var a DashboardAktivitaet
		if err := rows.Scan(&a.Typ, &a.Zeit, &a.ParzelleID, &a.ParzelleNummer); err != nil {
			continue
		}
		aktivitaeten = append(aktivitaeten, a)
	}

	return aktivitaeten
}

// DashboardHandlungsbedarf fasst die "Handlungsbedarf"-Kennzahlen für das
// Dashboard (/dashboard) zusammen. Es werden ausschließlich Zustände gezählt,
// die sich direkt aus vorhandenen Modellfeldern ableiten lassen (keine neuen
// Fachkonzepte):
//   - überschrittene Inspektionsfristen (frist < heute, auflagen_erfuellt = 0)
//   - gekündigte Parzellen ohne jede Inspektion
//   - abgeschlossene Inspektionen (auflagen_erfuellt = 1), deren Parzelle noch
//     keine Wertermittlung hat
type DashboardHandlungsbedarf struct {
	FristenUeberschritten          int
	GekuendigtOhneInspektion       int
	InspektionenOhneWertermittlung int
	Gesamt                         int
}

// GetDashboardHandlungsbedarf lädt die Handlungsbedarf-Kennzahlen für den
// obersten Block des Dashboards.
func GetDashboardHandlungsbedarf() DashboardHandlungsbedarf {
	var h DashboardHandlungsbedarf

	DB.QueryRow(`
		SELECT COUNT(*) FROM inspektionen
		WHERE frist IS NOT NULL AND frist < date('now') AND auflagen_erfuellt = 0
	`).Scan(&h.FristenUeberschritten)

	DB.QueryRow(`
		SELECT COUNT(*) FROM parzellen p
		WHERE p.kuendigung_datum IS NOT NULL
		AND NOT EXISTS (SELECT 1 FROM inspektionen i WHERE i.parzelle_id = p.id)
	`).Scan(&h.GekuendigtOhneInspektion)

	DB.QueryRow(`
		SELECT COUNT(*) FROM inspektionen i
		WHERE i.auflagen_erfuellt = 1
		AND NOT EXISTS (SELECT 1 FROM wertermittlungen w WHERE w.parzelle_id = i.parzelle_id)
	`).Scan(&h.InspektionenOhneWertermittlung)

	h.Gesamt = h.FristenUeberschritten + h.GekuendigtOhneInspektion + h.InspektionenOhneWertermittlung
	return h
}

// DashboardStats fasst die Kennzahlen-Kacheln des Dashboards zusammen.
type DashboardStats struct {
	AnzahlParzellen    int
	AnzahlPaechter     int
	OffeneKuendigungen int
	OffeneRechnungen   int
}

// GetDashboardStats lädt die Kennzahlen für die Dashboard-Kacheln.
//
// "Offene Rechnungen" zählt besetzte, nicht gekündigte Parzellen (ein Pächter
// ist eingetragen, kuendigung_datum ist leer), für die im laufenden
// Kalenderjahr noch keine einzige Wasser- oder Stromerfassung vorliegt - es
// gibt in den Daten keinen eigenständigen "Rechnung erstellt/bezahlt"-Status,
// darum ist das Fehlen jeder Verbrauchserfassung im laufenden Jahr die
// ehrlichste verfügbare Näherung für "hier steht noch eine Abrechnung aus".
func GetDashboardStats() DashboardStats {
	var s DashboardStats

	DB.QueryRow(`SELECT COUNT(*) FROM parzellen`).Scan(&s.AnzahlParzellen)

	DB.QueryRow(`
		SELECT COUNT(*) FROM parzellen
		WHERE paechter_name IS NOT NULL AND paechter_name != ''
	`).Scan(&s.AnzahlPaechter)

	DB.QueryRow(`
		SELECT COUNT(*) FROM parzellen WHERE kuendigung_datum IS NOT NULL
	`).Scan(&s.OffeneKuendigungen)

	jahr := time.Now().Year()
	DB.QueryRow(`
		SELECT COUNT(*) FROM parzellen p
		WHERE p.kuendigung_datum IS NULL
		AND p.paechter_name IS NOT NULL AND p.paechter_name != ''
		AND NOT EXISTS (SELECT 1 FROM wasser w WHERE w.parzelle_id = p.id AND w.jahr = ?)
		AND NOT EXISTS (SELECT 1 FROM strom st WHERE st.parzelle_id = p.id AND st.jahr = ?)
	`, jahr, jahr).Scan(&s.OffeneRechnungen)

	return s
}
