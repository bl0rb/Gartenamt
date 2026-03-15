package main

import (
	"context"
	"embed"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"kleingarten-verwaltung/handlers"
	"kleingarten-verwaltung/middleware"
	"kleingarten-verwaltung/models"
	"kleingarten-verwaltung/services"

	"github.com/gorilla/mux"
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

func main() {
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
	log.Println("📊 Initialisiere Datenbank...")
	db, err := models.InitDB("kleingarten.db")
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

	// Static files (UNGESCHÜTZT)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.FS(handlers.GetEmbeddedStaticFS()))))
	r.PathPrefix("/exports/").Handler(http.StripPrefix("/exports/", http.FileServer(http.Dir("exports/"))))

	// *** AUTHENTIFIZIERUNGS-ROUTEN (UNGESCHÜTZT) ***
	r.HandleFunc("/login", handlers.LoginHandler).Methods("GET", "POST")
	r.HandleFunc("/logout", handlers.LogoutHandler).Methods("POST", "GET")

	// *** GESCHÜTZTE HAUPT-ROUTEN (Benutzer + Admin) ***
	// Home-Route mit optionaler Auth (für Redirects)
	r.HandleFunc("/", middleware.CheckLoginStatus(handlers.HomeHandler)).Methods("GET")

	// Parzellen-Management (für alle authentifizierten Benutzer)
	r.HandleFunc("/parzellen", middleware.RequireAuth(handlers.ParzellenListHandler)).Methods("GET")
	r.HandleFunc("/parzellen/neu", middleware.RequireAuth(handlers.ParzelleNeuHandler)).Methods("GET", "POST")
	r.HandleFunc("/parzellen/{id}", middleware.RequireAuth(handlers.ParzelleDetailHandler)).Methods("GET")

	// Inspektion/Wertermittlung (für alle authentifizierten Benutzer)
	r.HandleFunc("/inspektion/{parzelle_id}", middleware.RequireAuth(handlers.InspektionHandler)).Methods("GET", "POST")
	r.HandleFunc("/wertermittlung/{parzelle_id}", middleware.RequireAuth(handlers.WertermittlungHandler)).Methods("GET", "POST")
	r.HandleFunc("/protokoll/{typ}/{id}", middleware.RequireAuth(handlers.ProtokollHandler)).Methods("GET")

	// *** BENUTZER-PROFIL ROUTEN (für alle authentifizierten Benutzer) ***
	r.HandleFunc("/profile", middleware.RequireAuth(handlers.ProfileHandler)).Methods("GET")
	r.HandleFunc("/change-password", middleware.RequireAuth(handlers.ChangePasswordHandler)).Methods("POST")

	// *** ADMIN-ROUTEN (NUR ADMINISTRATOREN) ***
	adminRoutes := r.PathPrefix("/admin").Subrouter()

	// Dashboard
	adminRoutes.HandleFunc("", middleware.RequireAdmin(handlers.AdminDashboardHandler)).Methods("GET")
	adminRoutes.HandleFunc("/", middleware.RequireAdmin(handlers.AdminDashboardHandler)).Methods("GET")

	// Daten-Management
	adminRoutes.HandleFunc("/obstarten", middleware.RequireAdmin(handlers.AdminObstartenHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/obstarten/{id}/delete", middleware.RequireAdmin(handlers.AdminObstartenLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/zieranpflanzungen", middleware.RequireAdmin(handlers.AdminZieranpflanzungenHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/zieranpflanzungen/{id}/delete", middleware.RequireAdmin(handlers.AdminZieranpflanzungenLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/bauindex", middleware.RequireAdmin(handlers.AdminBauindexHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/bauindex/{jahr}/delete", middleware.RequireAdmin(handlers.AdminBauindexLoeschenHandler)).Methods("POST")

	// Parzellen-Verwaltung (Löschen)
	adminRoutes.HandleFunc("/parzellen", middleware.RequireAdmin(handlers.AdminParzellenVerwaltungHandler)).Methods("GET")
	adminRoutes.HandleFunc("/parzellen/{id}/delete", middleware.RequireAdmin(handlers.AdminParzellenLoeschenHandler)).Methods("GET", "POST")

	// Protokoll-Verwaltung (Löschen)
	adminRoutes.HandleFunc("/protokolle", middleware.RequireAdmin(handlers.AdminProtokollVerwaltungHandler)).Methods("GET")
	adminRoutes.HandleFunc("/inspektionen/{id}/delete", middleware.RequireAdmin(handlers.AdminInspektionLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/wertermittlungen/{id}/delete", middleware.RequireAdmin(handlers.AdminWertermittlungLoeschenHandler)).Methods("POST")
	adminRoutes.HandleFunc("/bulk-delete", middleware.RequireAdmin(handlers.AdminBulkDeleteHandler)).Methods("POST")

	// System-Management
	adminRoutes.HandleFunc("/backup", middleware.RequireAdmin(handlers.AdminBackupHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/audit-log", middleware.RequireAdmin(handlers.AdminAuditLogHandler)).Methods("GET")
	adminRoutes.HandleFunc("/system-info", middleware.RequireAdmin(handlers.AdminSystemInfoHandler)).Methods("GET")

	// *** BENUTZER-VERWALTUNG (NUR ADMINISTRATOREN) ***
	adminRoutes.HandleFunc("/users", middleware.RequireAdmin(handlers.AdminUsersHandlerEnhanced)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/users/{id}/edit", middleware.RequireAdmin(handlers.AdminUserEditHandler)).Methods("GET", "POST")

	// *** INVOICE-/RECHNUNGS-VERWALTUNG (NUR ADMINISTRATOREN) ***
	// Invoice management dashboard
	adminRoutes.HandleFunc("/invoices", middleware.RequireAdmin(handlers.AdminInvoiceManagementHandler)).Methods("GET")

	// Organization settings for invoices
	adminRoutes.HandleFunc("/organization-settings", middleware.RequireAdmin(handlers.OrganizationSettingsHandler)).Methods("GET", "POST")

	// Water (Wasser) records per parzelle
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/wasser", middleware.RequireAdmin(handlers.WasserHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/wasser/{id}/delete", middleware.RequireAdmin(handlers.DeleteWasserHandler)).Methods("POST")

	// Electricity (Strom) records per parzelle
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/strom", middleware.RequireAdmin(handlers.StromHandler)).Methods("GET", "POST")
	adminRoutes.HandleFunc("/strom/{id}/delete", middleware.RequireAdmin(handlers.DeleteStromHandler)).Methods("POST")

	// Invoice preview and generation
	adminRoutes.HandleFunc("/parzellen/{parzelle_id}/invoice", middleware.RequireAdmin(handlers.InvoicePreviewHandler)).Methods("GET")

	// *** API-ROUTEN (für authentifizierte Benutzer) ***
	r.HandleFunc("/api/obstarten/preise", middleware.RequireAuth(handlers.APIObstartenPreiseHandler)).Methods("GET")
	r.HandleFunc("/api/zieranpflanzungen/preise", middleware.RequireAuth(handlers.APIZieranpflanzungsPreiseHandler)).Methods("GET")
	r.HandleFunc("/api/gemuese/preise", middleware.RequireAuth(handlers.APIGemusePreiseHandler)).Methods("GET")
	r.HandleFunc("/api/bauindex", middleware.RequireAuth(handlers.APIBauindexHandler)).Methods("GET")

	// Server starten
	log.Println("\n" + strings.Repeat("=", 80))
	log.Println("🚀 KLEINGARTEN-VERWALTUNG SERVER GESTARTET (HTTPS)")
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
		go func() {
			if err := server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
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
