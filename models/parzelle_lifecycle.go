package models

// parzelle_lifecycle.go bündelt Abfragen zum Lebenszyklus einer Parzelle
// (Pachtvertrag -> Kündigung -> Inspektion -> Wertermittlung -> Abrechnung).
// Eigentümer-Entscheidung 26.08.2026: Dieser Lebenszyklus ist der zentrale
// Workflow der Vereinsverwaltung - die Parzellenliste (Spalte "Nächste
// Aktion") und die Parzellen-Akte (Karte "Prozess") leiten daraus jeweils den
// nächsten Schritt ab. Für die Liste reicht dafür je eine Query für alle
// Parzellen (statt einer Abfrage pro Zeile / N+1).

// GetParzellenIDsMitInspektion liefert die IDs aller Parzellen, für die
// mindestens eine Inspektion vorliegt.
func GetParzellenIDsMitInspektion() (map[int]bool, error) {
	rows, err := DB.Query(`SELECT DISTINCT parzelle_id FROM inspektionen`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[int]bool)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// GetParzellenIDsMitWertermittlung liefert die IDs aller Parzellen, für die
// mindestens eine Wertermittlung vorliegt (siehe GetParzellenIDsMitInspektion).
func GetParzellenIDsMitWertermittlung() (map[int]bool, error) {
	rows, err := DB.Query(`SELECT DISTINCT parzelle_id FROM wertermittlungen`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[int]bool)
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}

// HatRechnungshistorie prüft für eine einzelne Parzelle (die Prozess-
// Checkliste der Akte in parzelle_detail.html), ob mindestens ein Wasser-
// oder Stromeintrag existiert. Anders als GetWasserByParzelle/
// GetStromByParzelle müssen die Datensätze dafür nicht geladen werden - die
// Existenzfrage genügt.
func HatRechnungshistorie(parzelleID int) (bool, error) {
	var exists bool
	err := DB.QueryRow(`
		SELECT EXISTS(
			SELECT 1 FROM wasser WHERE parzelle_id = ?
			UNION
			SELECT 1 FROM strom WHERE parzelle_id = ?
		)
	`, parzelleID, parzelleID).Scan(&exists)
	return exists, err
}
