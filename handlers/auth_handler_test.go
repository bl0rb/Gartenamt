package handlers

import "testing"

// safeRedirectPath ist der Schutz gegen den offenen Redirect nach dem Login:
// ein Link auf die echte Anwendung, der nach der Anmeldung auf eine fremde
// Seite weiterleitet, ist ein bequemer Phishing-Baustein.
func TestSafeRedirectPath(t *testing.T) {
	cases := []struct {
		name, input, want string
	}{
		{"interner Pfad", "/parzellen", "/parzellen"},
		{"interner Pfad mit Query", "/admin/users?success=x", "/admin/users?success=x"},
		{"leer", "", ""},
		{"absolute URL", "https://angreifer.tld", ""},
		{"absolute URL ohne Schema", "//angreifer.tld", ""},
		{"Backslash-Variante", "/\\angreifer.tld", ""},
		{"relativer Pfad ohne Slash", "parzellen", ""},
		{"Zeilenumbruch im Ziel", "/parzellen\r\nSet-Cookie: a=b", ""},
		{"Schleife auf die Loginseite", "/login?redirect=/admin", ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := safeRedirectPath(c.input); got != c.want {
				t.Errorf("safeRedirectPath(%q) = %q, erwartet %q", c.input, got, c.want)
			}
		})
	}
}
