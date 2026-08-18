package models

import "testing"

// Die Rechtevergabe ist der Kern der Rollentrennung: hier wird festgehalten,
// welche Rolle welche Berechtigung besitzt. Insbesondere dürfen Kassenwart und
// Wertermittler die Benutzerverwaltung NICHT öffnen - sonst könnten sie sich
// über eine Rollenänderung selbst zum Admin machen.
func TestDefaultRolePermissions(t *testing.T) {
	expected := map[string]map[string]bool{
		"admin": {
			"dashboard.access": true, "stammdaten.manage": true, "parzellen.manage": true,
			"protokolle.manage": true, "invoices.manage": true, "backup.manage": true,
			"audit.view": true, "users.manage": true, "settings.manage": true, "system.settings": true,
		},
		"vorstand": {
			"dashboard.access": true, "stammdaten.manage": true, "parzellen.manage": true,
			"protokolle.manage": true, "invoices.manage": true, "backup.manage": true,
			"audit.view": true, "users.manage": true, "settings.manage": true, "system.settings": true,
		},
		"kassenwart": {
			"dashboard.access": true, "stammdaten.manage": true, "parzellen.manage": true,
			"protokolle.manage": false, "invoices.manage": true, "backup.manage": false,
			"audit.view": false, "users.manage": false, "settings.manage": false, "system.settings": false,
		},
		"wertermittler": {
			"dashboard.access": true, "stammdaten.manage": true, "parzellen.manage": true,
			"protokolle.manage": true, "invoices.manage": false, "backup.manage": false,
			"audit.view": false, "users.manage": false, "settings.manage": false, "system.settings": false,
		},
		"user": {
			"dashboard.access": false, "stammdaten.manage": false, "parzellen.manage": false,
			"protokolle.manage": false, "invoices.manage": false, "backup.manage": false,
			"audit.view": false, "users.manage": false, "settings.manage": false, "system.settings": false,
		},
	}

	for role, permissions := range expected {
		if len(permissions) != len(PermissionCatalog()) {
			t.Fatalf("Rolle %s: Testtabelle deckt %d von %d Berechtigungen ab - Katalog erweitert?",
				role, len(permissions), len(PermissionCatalog()))
		}
		for permission, want := range permissions {
			if got := RoleHasPermission(role, permission); got != want {
				t.Errorf("RoleHasPermission(%q, %q) = %v, erwartet %v", role, permission, got, want)
			}
		}
	}
}

// Eine unbekannte Rolle darf nicht versehentlich Rechte erben.
func TestUnknownRoleHasNoPermissions(t *testing.T) {
	for _, permission := range PermissionCatalog() {
		if RoleHasPermission("superadmin", permission.Key) {
			t.Errorf("unbekannte Rolle erhielt Berechtigung %q", permission.Key)
		}
	}
}

func TestCanManageRole(t *testing.T) {
	cases := []struct {
		actor, target string
		want          bool
	}{
		{"admin", "admin", true},
		{"admin", "vorstand", true},
		{"admin", "user", true},
		{"vorstand", "vorstand", true},
		{"vorstand", "kassenwart", true},
		// Ein Vorstand darf niemanden zum Admin machen.
		{"vorstand", "admin", false},
		// Backoffice ohne users.manage darf gar keine Rollen vergeben.
		{"kassenwart", "user", false},
		{"kassenwart", "admin", false},
		{"wertermittler", "wertermittler", false},
		{"user", "user", false},
	}

	for _, c := range cases {
		if got := CanManageRole(c.actor, c.target); got != c.want {
			t.Errorf("CanManageRole(%q, %q) = %v, erwartet %v", c.actor, c.target, got, c.want)
		}
	}
}

func TestRoleRankOrdering(t *testing.T) {
	if !(RoleRank("admin") > RoleRank("vorstand") &&
		RoleRank("vorstand") > RoleRank("kassenwart") &&
		RoleRank("kassenwart") == RoleRank("wertermittler") &&
		RoleRank("wertermittler") > RoleRank("user")) {
		t.Fatal("Rangfolge der Rollen stimmt nicht")
	}
}

func TestValidatePasswordForUser(t *testing.T) {
	cases := []struct {
		name, password, username string
		wantErr                  bool
	}{
		{"ausreichend lang", "Gruenkohl!2026", "vorstand", false},
		{"zu kurz", "kurz1234", "vorstand", true},
		{"genau zehn Zeichen", "abcde12345", "vorstand", false},
		{"aus Sperrliste", "passwort12", "vorstand", true},
		{"enthaelt Benutzernamen", "vorstand2026", "vorstand", true},
		{"Benutzername in anderer Schreibweise", "XxVorStand99", "vorstand", true},
		{"nur ein Zeichen wiederholt", "aaaaaaaaaaaa", "vorstand", true},
		{"ohne Benutzerbezug erlaubt", "vorstand2026", "", false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidatePasswordForUser(c.password, c.username)
			if (err != nil) != c.wantErr {
				t.Errorf("ValidatePasswordForUser(%q, %q) = %v, Fehler erwartet: %v",
					c.password, c.username, err, c.wantErr)
			}
		})
	}
}
