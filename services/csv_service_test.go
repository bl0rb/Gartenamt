package services

import "testing"

// Die CSV-Exporte sind für Excel gedacht (BOM, Semikolon als Trenner). Excel
// wertet Felder, die mit "=", "+", "-" oder "@" beginnen, als Formel aus -
// deshalb bekommen sie ein voranstehendes Apostroph.
func TestSanitizeCSVValue(t *testing.T) {
	cases := []struct {
		name, input, want string
	}{
		{"gewöhnlicher Text", "Mustermann", "Mustermann"},
		{"leer", "", ""},
		{"Zahl", "42", "42"},
		{"Gleichheitszeichen", "=1+1", "'=1+1"},
		{"Formel mit Hyperlink", `=HYPERLINK("http://x","klick")`, `'=HYPERLINK("http://x","klick")`},
		{"Plus", "+49 170 1234567", "'+49 170 1234567"},
		{"Minus", "-5", "'-5"},
		{"At-Zeichen", "@SUM(A1)", "'@SUM(A1)"},
		{"Tabulator", "\tfoo", "'\tfoo"},
		{"Wagenrücklauf", "\rfoo", "'\rfoo"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := sanitizeCSVValue(c.input); got != c.want {
				t.Errorf("sanitizeCSVValue(%q) = %q, erwartet %q", c.input, got, c.want)
			}
		})
	}
}
