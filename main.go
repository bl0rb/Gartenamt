package main

import (
	"context"
	"embed"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/bl0rb/gartenamt/handlers"
	"github.com/bl0rb/gartenamt/middleware"
	"github.com/bl0rb/gartenamt/models"
	"github.com/bl0rb/gartenamt/services"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

//go:embed templates static
var embeddedFS embed.FS

// openBrowserApp öffnet die Anwendung im System-Browser
func openBrowserApp(url string) {
	openSystemBrowser(url)
}

// openSystemBrowser opens the URL in the system default browser
func openSystemBrowser(url string) {
	var err error
	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	}
	if err != nil {
		log.Printf("⚠️  Konnte Browser nicht automatisch öffnen: %v", err)
		log.Printf("   Bitte öffnen Sie manuell: %s", url)
	}
}

func loadEnvFiles() {
	candidates := []string{
		".env",
		".env.local",
	}

	var existing []string
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			existing = append(existing, candidate)
		}
	}

	if len(existing) == 0 {
		return
	}

	if err := godotenv.Load(existing...); err != nil {
		log.Printf("⚠️  Konnte Env-Dateien nicht laden: %v", err)
		return
	}

	log.Printf("📄 Env-Dateien geladen: %s", strings.Join(existing, ", "))
}

// ensureWritableWorkdir wechselt in ein benutzerspezifisches Datenverzeichnis,
// wenn das aktuelle Arbeitsverzeichnis nicht beschreibbar ist - z.B. beim Start
// als macOS-App-Bundle aus dem Finder (Arbeitsverzeichnis "/") oder als
// Windows-Exe unter "Programme". Datenbank, Exporte, Backups und .app_secret
// landen dann dort statt im Arbeitsverzeichnis. Gibt true zurück, wenn das
// Verzeichnis gewechselt wurde.
func ensureWritableWorkdir() bool {
	if os.Getenv("DB_PATH") != "" {
		return false // explizite Konfiguration hat Vorrang
	}

	if probe, err := os.CreateTemp(".", ".write-probe-*"); err == nil {
		probe.Close()
		os.Remove(probe.Name())
		return false
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		log.Printf("⚠️  Arbeitsverzeichnis nicht beschreibbar und kein Datenverzeichnis ermittelbar: %v", err)
		return false
	}

	dataDir := filepath.Join(configDir, "Gartenamt")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Printf("⚠️  Datenverzeichnis %s konnte nicht angelegt werden: %v", dataDir, err)
		return false
	}

	if err := os.Chdir(dataDir); err != nil {
		log.Printf("⚠️  Wechsel ins Datenverzeichnis %s fehlgeschlagen: %v", dataDir, err)
		return false
	}

	log.Printf("📁 Datenverzeichnis: %s", dataDir)
	return true
}

func main() {
	// Env-Dateien zuerst aus dem Startverzeichnis laden (z.B. .env neben der
	// Exe), dann ggf. ins Datenverzeichnis wechseln und dortige Env-Dateien
	// nachladen (godotenv überschreibt bereits gesetzte Werte nicht).
	loadEnvFiles()
	if ensureWritableWorkdir() {
		loadEnvFiles()
	}

	// 1. Initialize embedded filesystem
	handlers.SetEmbeddedFS(embeddedFS)

	// Desktop-Modus (Doppelklick, Standard): HTTP nur auf localhost mit
	// Menüleisten-/Tray-Symbol. Server-Modus (--no-browser, z.B. Docker/NAS):
	// HTTPS auf allen Interfaces.
	desktopMode := len(os.Args) == 1 || os.Args[1] != "--no-browser"
	middleware.SetSecureCookies(!desktopMode)

	// 1a. Certificate Manager initialisieren (nur im HTTPS-Server-Modus)
	var certManager *services.CertManager
	if !desktopMode {
		log.Println("🔒 Initialisiere HTTPS-Zertifikate...")
		certManager = services.NewCertManager()
		if err := certManager.EnsureCertificate(); err != nil {
			log.Fatal("Fehler beim Verwalten von Zertifikaten:", err)
		}
	}

	// 2. Auth-Service initialisieren (ZUERST!)
	log.Println("🔐 Initialisiere Auth-Service...")
	services.InitAuth()

	// 2. Datenbank initialisieren
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "kleingarten.db"
	}
	log.Println("📊 Initialisiere Datenbank...")
	db, err := models.InitDB(dbPath)
	if err != nil {
		log.Fatal("Fehler beim Initialisieren der Datenbank:", err)
	}
	defer db.Close()

	// 3. Standard-Admin erstellen (falls nötig)
	log.Println("👤 Prüfe Standard-Administrator...")
	if err := services.CreateDefaultAdmin(); err != nil {
		log.Printf("⚠️  Warnung beim Erstellen des Standard-Administrators: %v", err)
	}

	// Router erstellen
	r := mux.NewRouter()

	// Static files (UNGESCHÜTZT - nur eingebettete Assets, keine Nutzerdaten)
	staticFiles := http.StripPrefix("/static/", http.FileServer(http.FS(handlers.GetEmbeddedStaticFS())))
	r.PathPrefix("/static/").Handler(noDirectoryListing(staticFiles))

	// *** AUTHENTIFIZIERUNGS-ROUTEN (UNGESCHÜTZT) ***
	r.HandleFunc("/login", handlers.LoginHandler).Methods("GET", "POST")
	r.HandleFunc("/logout", handlers.LogoutHandler).Methods("POST")

	// *** GESCHÜTZTE HAUPT-ROUTEN (Benutzer + Admin) ***
	// Home-Route mit optionaler Auth (für Redirects)
	r.HandleFunc("/", middleware.CheckLoginStatus(handlers.HomeHandler)).Methods("GET")

	// Eigentümer-Entscheidung 26.08.2026 (ersetzt die Konsolidierung vom
	// 23.08.2026): Die App ist eine Arbeitsoberfläche für die
	// Vereinsverwaltung - die fachlichen Bereiche (Dashboard, Parzellen,
	// Abrechnung, Protokolle) liegen deshalb wieder top-level statt unter
	// /admin. /admin bleibt ausschließlich für technische und strukturelle
	// Administration (Benutzer, Stammdaten, Parzellenstruktur, System). Jede
	// fachliche Route trägt ihr eigenes Permission-Gate; die alten
	// /admin/*-Pendants bleiben als 301-Redirects (GET) bzw. Alias (POST,
	// siehe unten bei den Admin-Routen) für Deep-Links und Bookmarks
	// bestehen.
	r.HandleFunc("/dashboard", middleware.RequirePermission("dashboard.access", handlers.AdminDashboardHandler)).Methods("GET")

	// Parzellen: /neu und /{id}/edit müssen vor /{id} registriert werden -
	// gorilla/mux matcht in Registrierungsreihenfolge, sonst würde /{id} das
	// Literal "neu" als ID-Parameter schlucken.
	r.HandleFunc("/parzellen", middleware.RequirePermission("parzellen.manage", handlers.ParzellenListHandler)).Methods("GET")
	r.HandleFunc("/parzellen/neu", middleware.RequirePermission("parzellen.manage", handlers.ParzelleNeuHandler)).Methods("GET", "POST")
	r.HandleFunc("/parzellen/{id}/edit", middleware.RequirePermission("parzellen.manage", handlers.ParzelleEditHandler)).Methods("GET", "POST")
	r.HandleFunc("/parzellen/{id}", middleware.RequirePermission("parzellen.manage", handlers.ParzelleDetailHandler)).Methods("GET")

	// Wasser-/Stromablesungen je Parzelle
	r.HandleFunc("/parzellen/{parzelle_id}/wasser", middleware.RequirePermission("invoices.manage", handlers.WasserHandler)).Methods("GET", "POST")
	r.HandleFunc("/parzellen/{parzelle_id}/strom", middleware.RequirePermission("invoices.manage", handlers.StromHandler)).Methods("GET", "POST")

	// Rechnungsvorschau/-verlauf/-PDF sowie Mailversand je Parzelle
	r.HandleFunc("/parzellen/{parzelle_id}/invoice", middleware.RequirePermission("invoices.manage", handlers.InvoicePreviewHandler)).Methods("GET")
	r.HandleFunc("/parzellen/{parzelle_id}/invoice/history", middleware.RequirePermission("invoices.manage", handlers.InvoiceHistoryHandler)).Methods("GET")
	r.HandleFunc("/parzellen/{parzelle_id}/invoice/pdf", middleware.RequirePermission("invoices.manage", handlers.InvoicePDFDownloadHandler)).Methods("GET")
	r.HandleFunc("/parzellen/{parzelle_id}/email/send", middleware.RequirePermission("invoices.manage", handlers.SendParzelleEmailHandler)).Methods("POST")
	// Eigentümer-Entscheidung 23.08.2026: Mailversand/-verlauf der Parzellen-
	// Detailseite (Info/Verwarnung an Pächter) ist Vorstandsarbeit, nicht nur
	// Admin-Sache - daher eigene Permission "mail.send" (Rollen admin +
	// vorstand) statt der harten invoices.manage-Kopplung. Der Bulk-Rechnungs-
	// versand (email/send oben) bleibt bei invoices.manage, da fachlich
	// getrennt.
	r.HandleFunc("/parzellen/{parzelle_id}/email/info", middleware.RequirePermission("mail.send", handlers.SendParzelleInfoMailHandler)).Methods("POST")
	r.HandleFunc("/parzellen/{parzelle_id}/email/history", middleware.RequirePermission("mail.send", handlers.ParzelleEmailHistoryHandler)).Methods("GET")

	// Löschen von Wasser-/Stromablesungen (eigener Pfad, nicht unter /parzellen)
	r.HandleFunc("/wasser/{id}/delete", middleware.RequirePermission("invoices.manage", handlers.DeleteWasserHandler)).Methods("POST")
	r.HandleFunc("/strom/{id}/delete", middleware.RequirePermission("invoices.manage", handlers.DeleteStromHandler)).Methods("POST")

	// Abrechnung
	r.HandleFunc("/abrechnung", middleware.RequirePermission("invoices.manage", handlers.AdminInvoiceManagementHandler)).Methods("GET")
	r.HandleFunc("/abrechnung/export", middleware.RequirePermission("invoices.manage", handlers.AdminBulkInvoiceExportHandler)).Methods("GET")
	r.HandleFunc("/abrechnung/emails/send-bulk", middleware.RequirePermission("invoices.manage", handlers.SendBulkParzelleEmailHandler)).Methods("POST")

	// Protokoll-Verwaltung (Löschen von Inspektionen/Wertermittlungen)
	r.HandleFunc("/protokolle", middleware.RequirePermission("protokolle.manage", handlers.AdminProtokollVerwaltungHandler)).Methods("GET")
	r.HandleFunc("/protokolle/inspektionen/{id}/delete", middleware.RequirePermission("protokolle.manage", handlers.AdminInspektionLoeschenHandler)).Methods("POST")
	r.HandleFunc("/protokolle/wertermittlungen/{id}/delete", middleware.RequirePermission("protokolle.manage", handlers.AdminWertermittlungLoeschenHandler)).Methods("POST")

	// Inspektion/Wertermittlung (nur mit Protokoll-Berechtigung)
	r.HandleFunc("/inspektion/{parzelle_id}", middleware.RequirePermission("protokolle.manage", handlers.InspektionHandler)).Methods("GET", "POST")
	r.HandleFunc("/wertermittlung/{parzelle_id}", middleware.RequirePermission("protokolle.manage", handlers.WertermittlungHandler)).Methods("GET", "POST")
	r.HandleFunc("/protokoll/{typ}/{id}", middleware.RequirePermission("protokolle.manage", handlers.ProtokollHandler)).Methods("GET")

	// *** BENUTZER-PROFIL ROUTEN (für alle authentifizierten Benutzer) ***
	r.HandleFunc("/profile", middleware.RequireAuth(handlers.ProfileHandler)).Methods("GET")
	r.HandleFunc("/change-password", middleware.RequireAuth(handlers.ChangePasswordHandler)).Methods("POST")

	// *** ADMIN-ROUTEN (nur noch technische + strukturelle Administration,
	// Eigentümer-Entscheidung 26.08.2026) ***
	adminRoutes := r.PathPrefix("/admin").Subrouter()

	// / und /admin/ leiten auf die erste Admin-Seite weiter, für die die
	// Berechtigung vorliegt - das fachliche Dashboard liegt seit der
	// Entscheidung vom 26.08.2026 unter /dashboard, nicht mehr unter /admin.
	adminRoutes.HandleFunc("", middleware.RequireAuth(adminHomeHandler)).Methods("GET")
	adminRoutes.HandleFunc("/", middleware.RequireAuth(adminHomeHandler)).Methods("GET")
	adminRoutes.HandleFunc("/verwaltung", middleware.RequirePermission("settings.manage", handlers.AdminVerwaltungHandler)).Methods("GET")

	// Daten-Management
	adminRoutes.HandleFunc("/obstarten", middleware.RequirePermission("stammdaten.manage", handlers.AdminObstartenHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/obstarten/{id}/delete", middleware.RequirePermission("stammdaten.manage", handlers.AdminObstartenLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/zieranpflanzungen", middleware.RequirePermission("stammdaten.manage", handlers.AdminZieranpflanzungenHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/zieranpflanzungen/{id}/delete", middleware.RequirePermission("stammdaten.manage", handlers.AdminZieranpflanzungenLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/bauindex", middleware.RequirePermission("stammdaten.manage", handlers.AdminBauindexHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/bauindex/{jahr}/delete", middleware.RequirePermission("stammdaten.manage", handlers.AdminBauindexLoeschenHandler)).Methods("POST")

	// Parzellenstruktur (Datenverwaltung): früher "/admin/parzellen/verwalten",
	// jetzt eigener Name, da der URL-Raum /admin/parzellen* an die fachliche
	// Parzellenverwaltung (oben, top-level) zurückgegangen ist.
	adminRoutes.HandleFunc("/parzellenstruktur", middleware.RequirePermission("parzellen.manage", handlers.AdminParzellenVerwaltungHandler)).Methods("GET")
	adminRoutes.HandleFunc("/parzellenstruktur/{id}/delete", middleware.RequirePermission("parzellen.manage", handlers.AdminParzellenLoeschenHandler)).Methods("POST")
	// Alter POST-Pfad bleibt als Alias auf demselben Handler bestehen (kein
	// Redirect, da POST-Bodies bei 301 nicht zuverlässig erhalten bleiben).
	adminRoutes.HandleFunc("/parzellen/{id}/delete", middleware.RequirePermission("parzellen.manage", handlers.AdminParzellenLoeschenHandler)).Methods("POST")

	// Protokoll-Verwaltung (Löschen) - alte POST-Aliasse, fachliche GET-Seite
	// liegt jetzt unter /protokolle (oben, top-level).
	adminRoutes.HandleFunc("/inspektionen/{id}/delete", middleware.RequirePermission("protokolle.manage", handlers.AdminInspektionLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/wertermittlungen/{id}/delete", middleware.RequirePermission("protokolle.manage", handlers.AdminWertermittlungLoeschenHandler)).Methods("POST")
	// /admin/bulk-delete entfernt (23.08.2026, Eigentümer-Entscheidung): toter
	// Endpoint ohne UI-Anbindung. Der Handler-Code wird parallel entfernt.

	// System-Management
	adminRoutes.HandleFunc("/backup", middleware.RequirePermission("backup.manage", handlers.AdminBackupHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/exports/{filename}", middleware.RequirePermission("backup.manage", handlers.AdminExportDownloadHandler)).Methods("GET")
	// Download einzelner Backup-Dateien - dasselbe Gate wie die Backup-Seite.
	adminRoutes.HandleFunc("/backups/{filename}", middleware.RequirePermission("backup.manage", handlers.AdminBackupDownloadHandler)).Methods("GET")
	adminRoutes.HandleFunc("/audit-log", middleware.RequirePermission("audit.view", handlers.AdminAuditLogHandler)).Methods("GET")
	adminRoutes.HandleFunc("/system-info", middleware.RequirePermission("system.settings", handlers.AdminSystemInfoHandler)).Methods("GET", "POST")

	// *** BENUTZER-VERWALTUNG ***
	adminRoutes.HandleFunc("/users", middleware.RequirePermission("users.manage", handlers.AdminUsersHandlerEnhanced)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/users/{id}/edit", middleware.RequirePermission("users.manage", handlers.AdminUserEditHandler)).Methods("GET", "POST")

	// Organization settings for invoices
	adminRoutes.HandleFunc("/organization-settings", middleware.RequirePermission("settings.manage", handlers.OrganizationSettingsHandler)).Methods("GET", "POST")

	// *** ALTE POST-ENDPUNKTE UNTER /admin/parzellen* (Alias, kein Redirect) ***
	// Die fachlichen GET-Seiten liegen jetzt unter /parzellen* (oben,
	// top-level); diese POST-Endpunkte bleiben unter ihrer alten /admin-URL
	// zusätzlich erreichbar, damit bestehende Formulare nicht brechen.
	adminRoutes.HandleFunc("/wasser/{id}/delete", middleware.RequirePermission("invoices.manage", handlers.DeleteWasserHandler)).Methods("POST")
	adminRoutes.HandleFunc("/strom/{id}/delete", middleware.RequirePermission("invoices.manage", handlers.DeleteStromHandler)).Methods("POST")
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/email/send", middleware.RequirePermission("invoices.manage", handlers.SendParzelleEmailHandler)).Methods("POST")
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/email/info", middleware.RequirePermission("mail.send", handlers.SendParzelleInfoMailHandler)).Methods("POST")
	adminRoutes.HandleFunc("/emails/send-bulk", middleware.RequirePermission("invoices.manage", handlers.SendBulkParzelleEmailHandler)).Methods("POST")

	// *** ALTE GET-URLs -> 301 (Eigentümer-Entscheidung 26.08.2026) ***
	// Sonderfall zuerst: /admin/parzellen/verwalten ist keine Parzelle mit der
	// ID "verwalten", sondern die alte URL der Parzellenstruktur-Verwaltung.
	// Muss vor dem generischen /admin/parzellen*-Matcher registriert werden.
	r.HandleFunc("/admin/parzellen/verwalten", redirectTo("/admin/parzellenstruktur")).Methods("GET")
	// Generischer Fall: /admin/parzellen(/*) -> /parzellen(/*), Suffix und
	// Query-String erhalten. Bare Pfad und Pfade mit Suffix brauchen getrennte
	// Matcher: PathPrefix ohne abschließenden "/" prüft in gorilla/mux keine
	// Segmentgrenze und würde sonst auch /admin/parzellenstruktur treffen.
	r.HandleFunc("/admin/parzellen", redirectPrefix("/admin/parzellen", "/parzellen")).Methods("GET")
	r.PathPrefix("/admin/parzellen/").Handler(redirectPrefix("/admin/parzellen", "/parzellen")).Methods("GET")
	// /admin/invoices und /admin/invoices/export -> /abrechnung(/export). Wie
	// beim /admin/parzellen-Block oben: bare Pfad und Pfade mit Suffix brauchen
	// getrennte Matcher, sonst würde z.B. /admin/invoicesX (ohne Trennzeichen)
	// fälschlich auf /abrechnungX statt auf 404 matchen.
	r.HandleFunc("/admin/invoices", redirectPrefix("/admin/invoices", "/abrechnung")).Methods("GET")
	r.PathPrefix("/admin/invoices/").Handler(redirectPrefix("/admin/invoices", "/abrechnung")).Methods("GET")
	// /admin/protokolle -> /protokolle (gleiches Muster)
	r.HandleFunc("/admin/protokolle", redirectPrefix("/admin/protokolle", "/protokolle")).Methods("GET")
	r.PathPrefix("/admin/protokolle/").Handler(redirectPrefix("/admin/protokolle", "/protokolle")).Methods("GET")

	// *** API-ROUTEN ***
	// Stammdaten/Preise: für alle authentifizierten Benutzer
	r.HandleFunc("/api/obstarten/preise", middleware.RequireAuth(handlers.APIObstartenPreiseHandler)).Methods("GET")
	r.HandleFunc("/api/zieranpflanzungen/preise", middleware.RequireAuth(handlers.APIZieranpflanzungsPreiseHandler)).Methods("GET")
	r.HandleFunc("/api/gemuese/preise", middleware.RequireAuth(handlers.APIGemusePreiseHandler)).Methods("GET")
	r.HandleFunc("/api/bauindex", middleware.RequireAuth(handlers.APIBauindexHandler)).Methods("GET")
	// Parzellendaten enthalten Pächter-PII: nur mit Berechtigung
	r.HandleFunc("/api/parzellen", middleware.RequirePermission("parzellen.manage", handlers.APIParzellenHandler)).Methods("GET")

	// Server starten
	baseURL := "https://localhost:8080"
	if desktopMode {
		baseURL = desktopURL
	}
	log.Println("\n" + strings.Repeat("=", 80))
	if desktopMode {
		log.Println("🚀 GARTENAMT GESTARTET (Desktop-Modus, HTTP nur auf localhost)")
	} else {
		log.Println("🚀 GARTENAMT SERVER GESTARTET (HTTPS)")
	}
	log.Println(strings.Repeat("=", 80))
	log.Println("📍 URL: " + baseURL)
	log.Println("🔐 Login: " + baseURL + "/login")
	log.Println("👤 Standard-Admin: siehe Konsole oben")
	log.Println("📊 Dashboard: " + baseURL + "/dashboard")
	log.Println("🌱 Parzellen: " + baseURL + "/parzellen")
	log.Println("🧾 Abrechnung: " + baseURL + "/abrechnung")
	log.Println("📋 Admin-Interface: " + baseURL + "/admin")
	log.Println("👥 Benutzerverwaltung: " + baseURL + "/admin/users")
	log.Println("🔧 API-Endpoints: " + baseURL + "/api/")
	if !desktopMode {
		log.Println("🔒 Zertifikat: " + certManager.CertFile)
	}
	log.Println(strings.Repeat("=", 80))
	log.Println("✅ System bereit für Anmeldungen")
	log.Println("🔐 Vollständige Benutzerauthentifizierung aktiviert")
	if !desktopMode {
		log.Println("🛡️  HTTPS mit selbstsigniertem Zertifikat")
	}
	log.Println()

	// Middleware-Kette um den gesamten Router - so greifen Header und Limits
	// auch bei Anfragen, die auf keine Route passen.
	//   SecurityHeaders  -> Schutz-Header und no-store
	//   LimitRequestBody -> Obergrenze für Request-Körper
	//   CSRFProtect      -> Origin/Referer-Prüfung für schreibende Methoden
	handler := middleware.SecurityHeaders(
		middleware.LimitRequestBody(
			middleware.CSRFProtect(r)))

	// Ohne Timeouts hält jede halb geöffnete Verbindung unbegrenzt eine
	// Goroutine (Slowloris). ReadHeaderTimeout ist dabei die eigentliche
	// Bremse; die übrigen Fristen sind großzügig, damit Backup-Uploads und
	// PDF-Downloads im LAN nicht abbrechen.
	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadHeaderTimeout: 15 * time.Second,
		ReadTimeout:       5 * time.Minute,
		WriteTimeout:      5 * time.Minute,
		IdleTimeout:       2 * time.Minute,
		MaxHeaderBytes:    1 << 20,
	}

	// Signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if desktopMode {
		// HTTP nur auf dem Loopback-Interface: keine Zertifikatswarnung im
		// Browser, von außen ist der Server nicht erreichbar. Läuft bereits
		// eine Instanz (Port belegt), nur den Browser öffnen und beenden.
		listener, err := net.Listen("tcp", "127.0.0.1:8080")
		if err != nil {
			log.Println("⚠️  Port 8080 ist bereits belegt - vermutlich läuft Gartenamt schon. Öffne Browser...")
			openBrowserApp(desktopURL)
			return
		}

		go func() {
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				log.Printf("Server error: %v", err)
			}
		}()

		// Launch browser in goroutine
		go func() {
			time.Sleep(500 * time.Millisecond)
			log.Println("🌐 Öffne Browser...")
			openBrowserApp(desktopURL)
		}()

		// Blockiert bis "Beenden" im Menüleisten-/Tray-Menü oder Signal,
		// fährt dann den Server sauber herunter
		runDesktop(server, sigChan)
	} else {
		// Server-Modus: HTTPS in der Hauptgoroutine
		tlsConfig, err := certManager.GetTLSConfig()
		if err != nil {
			log.Fatal("Fehler beim Laden der TLS-Konfiguration:", err)
		}
		server.TLSConfig = tlsConfig

		go func() {
			sig := <-sigChan
			log.Printf("\n📛 Signal empfangen (%v), fahre herunter...", sig)
			shutdownServer(server)
		}()

		log.Fatal(server.ListenAndServeTLS("", ""))
	}
}

// adminHomeHandler leitet GET /admin und /admin/ per 303 auf die erste
// Admin-Seite weiter, für die die Session eine Berechtigung hat (Reihenfolge:
// Benutzer, Einstellungen, Obstarten, Parzellenstruktur, Backup, Audit-Log,
// System-Info). Ohne passende Berechtigung geht es auf /dashboard zurück.
// Eigentümer-Entscheidung 26.08.2026: /admin ist nur noch technische und
// strukturelle Administration - das fachliche Dashboard liegt unter
// /dashboard, weshalb es hier keinen entsprechenden Eintrag mehr gibt.
func adminHomeHandler(w http.ResponseWriter, r *http.Request) {
	targets := []struct {
		permission string
		path       string
	}{
		{"users.manage", "/admin/users"},
		{"settings.manage", "/admin/verwaltung"},
		{"stammdaten.manage", "/admin/obstarten"},
		{"parzellen.manage", "/admin/parzellenstruktur"},
		{"backup.manage", "/admin/backup"},
		{"audit.view", "/admin/audit-log"},
		{"system.settings", "/admin/system-info"},
	}

	for _, target := range targets {
		if middleware.HasPermission(r, target.permission) {
			http.Redirect(w, r, target.path, http.StatusSeeOther)
			return
		}
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// redirectTo liefert einen Handler, der 301 auf einen festen Zielpfad
// umleitet und dabei einen eventuellen Query-String erhält.
func redirectTo(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dest := target
		if r.URL.RawQuery != "" {
			dest += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, dest, http.StatusMovedPermanently)
	}
}

// redirectPrefix liefert einen Handler, der das Pfadpräfix oldPrefix durch
// newPrefix ersetzt und dabei Suffix sowie Query-String erhält (301).
// Eigentümer-Entscheidung 26.08.2026: die fachlichen Bereiche sind von /admin
// auf top-level-URLs zurückgezogen worden (Gegenrichtung zur Konsolidierung
// vom 23.08.2026) - alte /admin/*-Deep-Links und Bookmarks bleiben darüber
// erreichbar, ohne dass jede einzelne Unterseite einzeln registriert werden
// muss.
func redirectPrefix(oldPrefix, newPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := newPrefix + strings.TrimPrefix(r.URL.RequestURI(), oldPrefix)
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
}

// noDirectoryListing unterbindet die Verzeichnisauflistung des FileServers.
// Sie legt sonst offen, welche Assets in welcher Version eingebettet sind.
func noDirectoryListing(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// desktopURL ist die Adresse im Desktop-Modus (HTTP nur auf localhost).
const desktopURL = "http://localhost:8080"

// shutdownServer fährt den Server mit Timeout sauber herunter.
func shutdownServer(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("⚠️  Fehler beim Herunterfahren: %v", err)
	}
	log.Println("✅ Server heruntergefahren")
}
