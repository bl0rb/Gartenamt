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

	// 1a. Certificate Manager initialisieren
	log.Println("🔒 Initialisiere HTTPS-Zertifikate...")
	certManager := services.NewCertManager()
	if err := certManager.EnsureCertificate(); err != nil {
		log.Fatal("Fehler beim Verwalten von Zertifikaten:", err)
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

	// CSRF-Schutz für alle state-ändernden Requests (Origin/Referer-Prüfung)
	r.Use(middleware.CSRFProtect)

	// Static files (UNGESCHÜTZT - nur eingebettete Assets, keine Nutzerdaten)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(handlers.GetEmbeddedStaticFS()))))

	// *** AUTHENTIFIZIERUNGS-ROUTEN (UNGESCHÜTZT) ***
	r.HandleFunc("/login", handlers.LoginHandler).Methods("GET", "POST")
	r.HandleFunc("/logout", handlers.LogoutHandler).Methods("POST")

	// *** GESCHÜTZTE HAUPT-ROUTEN (Benutzer + Admin) ***
	// Home-Route mit optionaler Auth (für Redirects)
	r.HandleFunc("/", middleware.CheckLoginStatus(handlers.HomeHandler)).Methods("GET")

	// Parzellen-Management (enthält Pächterdaten - nur mit Berechtigung)
	r.HandleFunc("/parzellen", middleware.RequirePermission("parzellen.manage", handlers.ParzellenListHandler)).Methods("GET")
	r.HandleFunc("/parzellen/neu", middleware.RequirePermission("parzellen.manage", handlers.ParzelleNeuHandler)).Methods("GET", "POST")
	r.HandleFunc("/parzellen/{id}/edit", middleware.RequirePermission("parzellen.manage", handlers.ParzelleEditHandler)).Methods("GET", "POST")
	r.HandleFunc("/parzellen/{id}", middleware.RequirePermission("parzellen.manage", handlers.ParzelleDetailHandler)).Methods("GET")

	// Inspektion/Wertermittlung (nur mit Protokoll-Berechtigung)
	r.HandleFunc("/inspektion/{parzelle_id}", middleware.RequirePermission("protokolle.manage", handlers.InspektionHandler)).Methods("GET", "POST")
	r.HandleFunc("/wertermittlung/{parzelle_id}", middleware.RequirePermission("protokolle.manage", handlers.WertermittlungHandler)).Methods("GET", "POST")
	r.HandleFunc("/protokoll/{typ}/{id}", middleware.RequirePermission("protokolle.manage", handlers.ProtokollHandler)).Methods("GET")

	// *** BENUTZER-PROFIL ROUTEN (für alle authentifizierten Benutzer) ***
	r.HandleFunc("/profile", middleware.RequireAuth(handlers.ProfileHandler)).Methods("GET")
	r.HandleFunc("/change-password", middleware.RequireAuth(handlers.ChangePasswordHandler)).Methods("POST")

	// *** ADMIN-ROUTEN (BACKOFFICE + BERECHTIGUNGEN) ***
	adminRoutes := r.PathPrefix("/admin").Subrouter()

	// Dashboard
	adminRoutes.HandleFunc("", middleware.RequirePermission("dashboard.access", handlers.AdminDashboardHandler)).Methods("GET")
	adminRoutes.HandleFunc("/", middleware.RequirePermission("dashboard.access", handlers.AdminDashboardHandler)).Methods("GET")
	adminRoutes.HandleFunc("/verwaltung", middleware.RequirePermission("settings.manage", handlers.AdminVerwaltungHandler)).Methods("GET")

	// Daten-Management
	adminRoutes.HandleFunc("/obstarten", middleware.RequirePermission("stammdaten.manage", handlers.AdminObstartenHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/obstarten/{id}/delete", middleware.RequirePermission("stammdaten.manage", handlers.AdminObstartenLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/zieranpflanzungen", middleware.RequirePermission("stammdaten.manage", handlers.AdminZieranpflanzungenHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/zieranpflanzungen/{id}/delete", middleware.RequirePermission("stammdaten.manage", handlers.AdminZieranpflanzungenLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/bauindex", middleware.RequirePermission("stammdaten.manage", handlers.AdminBauindexHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/bauindex/{jahr}/delete", middleware.RequirePermission("stammdaten.manage", handlers.AdminBauindexLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/parzellen", middleware.RequirePermission("parzellen.manage", handlers.ParzellenListHandler)).Methods("GET")
	adminRoutes.HandleFunc("/parzellen/neu", middleware.RequirePermission("parzellen.manage", handlers.ParzelleNeuHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/parzellen/{id}/edit", middleware.RequirePermission("parzellen.manage", handlers.ParzelleEditHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/parzellen/{id}", middleware.RequirePermission("parzellen.manage", handlers.ParzelleDetailHandler)).Methods("GET")

	// Parzellen-Verwaltung (Löschen)
	adminRoutes.HandleFunc("/parzellen/verwalten", middleware.RequirePermission("parzellen.manage", handlers.AdminParzellenVerwaltungHandler)).Methods("GET")
	adminRoutes.HandleFunc("/parzellen/{id}/delete", middleware.RequirePermission("parzellen.manage", handlers.AdminParzellenLoeschenHandler)).Methods("POST")

	// Protokoll-Verwaltung (Löschen)
	adminRoutes.HandleFunc("/protokolle", middleware.RequirePermission("protokolle.manage", handlers.AdminProtokollVerwaltungHandler)).Methods("GET")
	adminRoutes.HandleFunc("/inspektionen/{id}/delete", middleware.RequirePermission("protokolle.manage", handlers.AdminInspektionLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/wertermittlungen/{id}/delete", middleware.RequirePermission("protokolle.manage", handlers.AdminWertermittlungLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/bulk-delete", middleware.RequirePermission("protokolle.manage", handlers.AdminBulkDeleteHandler)).Methods("POST")

	// System-Management
	adminRoutes.HandleFunc("/backup", middleware.RequirePermission("backup.manage", handlers.AdminBackupHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/exports/{filename}", middleware.RequirePermission("backup.manage", handlers.AdminExportDownloadHandler)).Methods("GET")
	adminRoutes.HandleFunc("/audit-log", middleware.RequirePermission("audit.view", handlers.AdminAuditLogHandler)).Methods("GET")
	adminRoutes.HandleFunc("/system-info", middleware.RequirePermission("system.settings", handlers.AdminSystemInfoHandler)).Methods("GET", "POST")

	// *** BENUTZER-VERWALTUNG ***
	adminRoutes.HandleFunc("/users", middleware.RequirePermission("users.manage", handlers.AdminUsersHandlerEnhanced)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/users/{id}/edit", middleware.RequirePermission("users.manage", handlers.AdminUserEditHandler)).Methods("GET", "POST")

	// *** INVOICE-/RECHNUNGS-VERWALTUNG ***
	// Invoice management dashboard
	adminRoutes.HandleFunc("/invoices", middleware.RequirePermission("invoices.manage", handlers.AdminInvoiceManagementHandler)).Methods("GET")

	// Organization settings for invoices
	adminRoutes.HandleFunc("/organization-settings", middleware.RequirePermission("settings.manage", handlers.OrganizationSettingsHandler)).Methods("GET", "POST")

	// Water (Wasser) records per parzelle
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/wasser", middleware.RequirePermission("invoices.manage", handlers.WasserHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/wasser/{id}/delete", middleware.RequirePermission("invoices.manage", handlers.DeleteWasserHandler)).Methods("POST")

	// Electricity (Strom) records per parzelle
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/strom", middleware.RequirePermission("invoices.manage", handlers.StromHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/strom/{id}/delete", middleware.RequirePermission("invoices.manage", handlers.DeleteStromHandler)).Methods("POST")

	// Invoice preview and generation
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/invoice", middleware.RequirePermission("invoices.manage", handlers.InvoicePreviewHandler)).Methods("GET")
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/invoice/history", middleware.RequirePermission("invoices.manage", handlers.InvoiceHistoryHandler)).Methods("GET")
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/invoice/pdf", middleware.RequirePermission("invoices.manage", handlers.InvoicePDFDownloadHandler)).Methods("GET")
	adminRoutes.HandleFunc("/invoices/export", middleware.RequirePermission("invoices.manage", handlers.AdminBulkInvoiceExportHandler)).Methods("GET")
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/email/send", middleware.RequirePermission("invoices.manage", handlers.SendParzelleEmailHandler)).Methods("POST")
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/email/info", middleware.RequirePermission("invoices.manage", handlers.SendParzelleInfoMailHandler)).Methods("POST")
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/email/history", middleware.RequirePermission("invoices.manage", handlers.ParzelleEmailHistoryHandler)).Methods("GET")
	adminRoutes.HandleFunc("/emails/send-bulk", middleware.RequirePermission("invoices.manage", handlers.SendBulkParzelleEmailHandler)).Methods("POST")

	// *** API-ROUTEN ***
	// Stammdaten/Preise: für alle authentifizierten Benutzer
	r.HandleFunc("/api/obstarten/preise", middleware.RequireAuth(handlers.APIObstartenPreiseHandler)).Methods("GET")
	r.HandleFunc("/api/zieranpflanzungen/preise", middleware.RequireAuth(handlers.APIZieranpflanzungsPreiseHandler)).Methods("GET")
	r.HandleFunc("/api/gemuese/preise", middleware.RequireAuth(handlers.APIGemusePreiseHandler)).Methods("GET")
	r.HandleFunc("/api/bauindex", middleware.RequireAuth(handlers.APIBauindexHandler)).Methods("GET")
	// Parzellendaten enthalten Pächter-PII: nur mit Berechtigung
	r.HandleFunc("/api/parzellen", middleware.RequirePermission("parzellen.manage", handlers.APIParzellenHandler)).Methods("GET")

	// Server starten
	log.Println("\n" + strings.Repeat("=", 80))
	log.Println("🚀 GARTENAMT SERVER GESTARTET (HTTPS)")
	log.Println(strings.Repeat("=", 80))
	log.Println("📍 URL: https://localhost:8080")
	log.Println("🔐 Login: https://localhost:8080/login")
	log.Println("👤 Standard-Admin: siehe Konsole oben")
	log.Println("📋 Admin-Interface: https://localhost:8080/admin")
	log.Println("👥 Benutzerverwaltung: https://localhost:8080/admin/users")
	log.Println("🔧 API-Endpoints: https://localhost:8080/api/")
	log.Println("🔒 Zertifikat: " + certManager.CertFile)
	log.Println(strings.Repeat("=", 80))
	log.Println("✅ System bereit für Anmeldungen")
	log.Println("🔐 Vollständige Benutzerauthentifizierung aktiviert")
	log.Println("🛡️  HTTPS mit selbstsigniertem Zertifikat")
	log.Println()

	// Get TLS configuration
	tlsConfig, err := certManager.GetTLSConfig()
	if err != nil {
		log.Fatal("Fehler beim Laden der TLS-Konfiguration:", err)
	}

	// Create HTTPS server
	server := &http.Server{
		Addr:      ":8080",
		Handler:   r,
		TLSConfig: tlsConfig,
	}

	// Signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine if using browser
	if len(os.Args) == 1 || (len(os.Args) > 1 && os.Args[1] != "--no-browser") {
		// Läuft bereits eine Instanz (Port belegt), nur den Browser öffnen und
		// beenden - sonst bliebe bei jedem Doppelklick ein Prozess ohne Server zurück
		listener, err := net.Listen("tcp", server.Addr)
		if err != nil {
			log.Println("⚠️  Port 8080 ist bereits belegt - vermutlich läuft Gartenamt schon. Öffne Browser...")
			openBrowserApp("https://localhost:8080")
			return
		}

		go func() {
			if err := server.ServeTLS(listener, "", ""); err != nil && err != http.ErrServerClosed {
				log.Printf("Server error: %v", err)
			}
		}()

		// Launch browser in goroutine
		go func() {
			time.Sleep(500 * time.Millisecond)
			log.Println("🌐 Öffne Browser...")
			openBrowserApp("https://localhost:8080")
		}()

		// Wait for signal to shutdown
		sig := <-sigChan
		log.Printf("\n📛 Signal empfangen (%v), fahre herunter...", sig)

		// Graceful shutdown with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("⚠️  Fehler beim Herunterfahren: %v", err)
		}
		log.Println("✅ Server heruntergefahren")
	} else {
		// Terminal-only mode: run server in main thread
		go func() {
			sig := <-sigChan
			log.Printf("\n📛 Signal empfangen (%v), fahre herunter...", sig)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			if err := server.Shutdown(ctx); err != nil {
				log.Printf("⚠️  Fehler beim Herunterfahren: %v", err)
			}
		}()

		log.Fatal(server.ListenAndServeTLS("", ""))
	}
}
