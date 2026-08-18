package services

import (
	"testing"
	"time"
)

// Die Kontosperre schützt ein einzelnes Konto vor gezieltem Raten. Die
// IP-Sperre darf dagegen nicht schon nach wenigen Fehlversuchen greifen:
// hinter NAT oder einem Reverse-Proxy teilen sich alle Mitglieder eine
// Adresse, und eine strenge IP-Sperre wäre dort ein Denial of Service.
func TestLoginThrottleThresholds(t *testing.T) {
	service := &AuthService{
		sessions:     map[string]*Session{},
		failedLogins: map[string]*failedLoginState{},
	}

	keys := throttleKeys("anna", "203.0.113.7")

	for i := 0; i < maxFailedLoginsPerAccount; i++ {
		if err := service.checkLoginThrottle(keys); err != nil {
			t.Fatalf("Sperre griff bereits nach %d Fehlversuchen", i)
		}
		service.recordFailedLogin(keys)
	}

	if err := service.checkLoginThrottle(keys); err == nil {
		t.Fatalf("Konto nach %d Fehlversuchen nicht gesperrt", maxFailedLoginsPerAccount)
	}

	// Ein anderes Konto von derselben IP muss sich weiterhin anmelden können.
	other := throttleKeys("bernd", "203.0.113.7")
	if err := service.checkLoginThrottle(other); err != nil {
		t.Fatal("Kontosperre hat die gesamte IP mitgesperrt")
	}
}

func TestLoginThrottleIPLocksOnlyAfterManyAttempts(t *testing.T) {
	service := &AuthService{
		sessions:     map[string]*Session{},
		failedLogins: map[string]*failedLoginState{},
	}

	// Viele verschiedene Konten von derselben IP: jedes bleibt unter seiner
	// eigenen Schwelle, die IP-Schwelle wird erst am Ende erreicht.
	ip := "198.51.100.4"
	for i := 0; i < maxFailedLoginsPerIP; i++ {
		keys := throttleKeys(string(rune('a'+i%26))+string(rune('0'+i/26)), ip)
		service.recordFailedLogin(keys)
	}

	if err := service.checkLoginThrottle(throttleKeys("neuer-nutzer", ip)); err == nil {
		t.Fatalf("IP nach %d Fehlversuchen nicht gesperrt", maxFailedLoginsPerIP)
	}

	if maxFailedLoginsPerIP <= maxFailedLoginsPerAccount {
		t.Fatal("IP-Schwelle muss über der Kontoschwelle liegen")
	}
}

// Ein erfolgreicher Login setzt beide Zähler zurück.
func TestResetFailedLogins(t *testing.T) {
	service := &AuthService{
		sessions:     map[string]*Session{},
		failedLogins: map[string]*failedLoginState{},
	}

	keys := throttleKeys("clara", "192.0.2.9")
	for i := 0; i < maxFailedLoginsPerAccount; i++ {
		service.recordFailedLogin(keys)
	}
	service.resetFailedLogins(keys)

	if err := service.checkLoginThrottle(keys); err != nil {
		t.Fatal("Zähler wurde nach erfolgreichem Login nicht zurückgesetzt")
	}
	if len(service.failedLogins) != 0 {
		t.Fatalf("Einträge blieben stehen: %d", len(service.failedLogins))
	}
}

// Die Tabelle darf nicht unbegrenzt wachsen - ihre Schlüssel enthalten den
// frei wählbaren Benutzernamen.
func TestFailedLoginTableIsBounded(t *testing.T) {
	service := &AuthService{
		sessions:     map[string]*Session{},
		failedLogins: map[string]*failedLoginState{},
	}

	for i := 0; i < maxThrottleEntries+500; i++ {
		service.recordFailedLogin([]throttleKey{{key: "user:muell" + time.Duration(i).String(), limit: 5}})
	}

	if len(service.failedLogins) > maxThrottleEntries {
		t.Fatalf("Tabelle wuchs auf %d Einträge (Grenze %d)", len(service.failedLogins), maxThrottleEntries)
	}
}
