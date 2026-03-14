# Kleingarten-Verwaltung (Allotment Garden Management System)

A web-based management system for allotment gardens (Kleingartenanlagen) built with Go, designed to manage plots, inspections, valuations, and user administration.

## 🎯 Features

### 👤 Benutzerverwaltung
- Benutzerauthentifizierung und Authentifizierung mit verschlüsselten Passwörtern
- Rollenbasierte Zugriffskontrolle (Admin, Benutzer)
- Benutzerprofilmanagement mit Passwortänderung
- Admin-Benutzerverwaltungsschnittstelle
- Sitzungsverwaltung und Aktivitätsverfolgung
- Session-basierte Authentifizierung mit HttpOnly-Cookies

### 🏡 Parzellenverwaltung
- Anlage, Anzeige und Verwaltung von Kleingartenparzellen
- Verfolgung von Parzellendetails (Größe, Besitzer, Kontaktinformationen)
- Schnelle Suche und Filterung nach Parzellenummer
- Parzellenhistorie und Aktivitätslog
- Verknüpfung mit Inspektionen und Wertermittlungen

### 🔍 Inspektionen
- Durchführung von Inspektionen an Parzellen mit Mangelerfassung
- Dokumentation von Mängeln und Schäden
- PDF-Protokoll-Generierung für Inspektionen
- Inspektionshistorie und Nachverfolgung
- Berichterstellung zerstörter Inspektionen

### 💰 Wertermittlung
- Automatische Berechnung von Parzellenwertwertermittlungen
- Berücksichtigung von:
  - Obstbeständen (Obstarten)
  - Zieranpflanzungen
  - Gemüsebeet-Anlagen
  - Baulichkeiten (Lauben, Wege, Pforte, Strom, Wasser)
- Bauindex-gestützte Preisanpassung
- PDF-Wertermittlungs-Protokoll
- Historische Wertermittlungen verfolgbar

### 📚 Referenzdatenverwaltung
- **Obstarten-Verwaltung:** Anlage, Bearbeitung und Löschung von Obstarten mit:
  - Kategorie (E1-E11)
  - Einheit (Stück, m², lfm)
  - Standardpreis
  - Maximale Anzahl
  
- **Zieranpflanzungen-Verwaltung:** Verwaltung von Zierplanzungen mit:
  - Kategorien (F1-F8)
  - Preis pro m²
  - Maximale Fläche
  
- **Bauindex-Verwaltung:** Flexible Verwaltung von Bauindizes:
  - Hinzufügen von Bauindizes für verschiedene Jahre
  - Automatische Verwendung des aktuellen Bauindex
  - Historische Bauindex-Einträge verfolgbar
  - Bearbeitung und Löschung ermöglicht

### 📊 Datenmanagement
- CSV-Export aller relevanten Daten:
  - Parzellen-Export
  - Wertermittlungs-Export
  - Obstarten-Export
  - Zieranpflanzungen-Export
  - Bauindex-Export
  
- CSV-Import für Referenzdaten
- Datenbankbackup-Funktionalität
- Bulk-Operationen (Löschen, Verwaltung)

### 🛡️ Admin-Dashboard & Sicherheit
- Übersicht mit System-Statistiken:
  - Anzahl Obstarten
  - Anzahl Zieranpflanzungen
  - Anzahl Parzellen
  - Anzahl Wertermittlungen
  
- **Audit-Logging:** Vollständige Verfolgung aller administrativen Aktionen:
  - Benutzer-Aktivitäten
  - Datenänderungen
  - Zeitstempel und IP-Adresse
  
- **System-Informationen:** Detaillierte System- und Datenbankstatistiken
- Backup- und Wiederherstellungsfunktionen
- Verwaltungsoberfläche mit Rollen-Controlling

### 📈 Berechnungslogik
- Automatische Wertberechnung für Obstbestände
- Automatische Wertberechnung für Zieranpflanzungen
- Gemüsebeet-Berechnung (Einzelpflanzen, Reihenanbau, Kräuter)
- Baulichkeiten-Abschreibung
- Flexible Preisanpassung über Bauindex
- Fallback-Mechanismen für fehlende Daten

### 🌐 Benutzeroberfläche
- Responsive Bootstrap 5 Design
- Deutsche Benutzeroberfläche
- Intuitive Navigation
- Die wichtigsten Funktionen sind direkt vom Admin-Dashboard erreichbar
- Kontextabhängige Hilfetexte
- Warnsystem für kritische Aktionen

## 📋 Technology Stack

- **Language:** Go 1.21
- **Web Framework:** Gorilla Mux (routing)
- **Database:** SQLite3
- **PDF Generation:** gofpdf
- **Security:** crypto/x509, golang.org/x/crypto (password hashing)
- **Frontend:** HTML5, CSS3, JavaScript (vanilla)

## 🚀 Getting Started

### Prerequisites
- Go 1.21 or later
- SQLite3 (included with most systems)

### Installation

1. Clone the repository:
```bash
git clone https://github.com/matze/kleingarten-verwaltung.git
cd kleingarten-verwaltung
```

2. Download dependencies:
```bash
go mod download
```

3. Run the application:
```bash
go run main.go
```

The application will start on `http://localhost:8080` by default.

4. Login with default credentials:
   - **Username:** admin
   - **Password:** admin123
   
   ⚠️ **IMPORTANT:** Change the default admin password immediately in production!

## 📁 Project Structure

```
.
├── main.go                    # Application entry point and routing
├── go.mod                     # Go module definition
├── handlers/                  # HTTP request handlers
│   ├── auth_handler.go        # Login/logout handlers
│   ├── admin_*.go             # Admin dashboard and management
│   ├── parzelle_handler.go    # Plot management
│   ├── inspektion_handler.go  # Inspection tracking
│   ├── wertermittlung_handler.go # Valuation handling
│   ├── api_handler.go         # API endpoints
│   └── template_helpers.go    # Template utility functions
├── middleware/                # HTTP middleware
│   └── auth_middleware.go     # Authentication/authorization
├── models/                    # Data models and database access
│   ├── database.go            # Database initialization
│   ├── user.go                # User model
│   ├── parzelle.go            # Plot model
│   ├── inspektion.go          # Inspection model
│   ├── wertermittlung.go      # Valuation model
│   └── admin_*.go             # Admin-specific models
├── services/                  # Business logic
│   ├── auth_service.go        # Authentication service
│   ├── berechnungs_service.go # Calculation logic
│   └── csv_service.go         # CSV import/export
├── templates/                 # HTML templates
├── static/                    # Static files (CSS, JS, images)
└── exports/                   # Generated CSV/backup files
```

## 🔐 Security Notes

### Current Implementation
- Password hashing using bcrypt
- Session-based authentication with HTTPOnly cookies
- Role-based access control (RBAC)
- SQL parameterized queries (SQL injection protection)

### ⚠️ Security Recommendations (High Priority)

1. **Change Default Admin Password**
   - Immediately change the default `admin123` password
   - Implement a first-run setup wizard

2. **Enable CSRF Protection**
   - Add CSRF token validation on POST endpoints
   - Consider using `github.com/gorilla/csrf` middleware

3. **Session Security**
   - Enable `Secure` flag on cookies in production (HTTPS only)
   - Reduce session timeout from 24 hours to 2 hours
   - Implement rate limiting on login attempts

4. **Input Validation**
   - Validate all form inputs server-side
   - Validate file uploads (content-type, size limits)
   - Sanitize user-provided URLs to prevent open redirects

5. **Database**
   - Use database migrations instead of inline table creation
   - Configure SQLite connection pooling appropriately
   - Consider persistent session storage for multi-instance deployments

6. **HTTPS/TLS**
   - Always use HTTPS in production
   - Set `Secure` cookie flag and `SameSite` attributes

See [SECURITY_AUDIT.md](SECURITY_AUDIT.md) for detailed audit findings.

## 🔧 Configuration

The application uses the following defaults:
- **Port:** 8080
- **Database:** `kleingarten.db` (SQLite)
- **Session Timeout:** 24 hours
- **Exports Directory:** `exports/`
- **Backups Directory:** `backups/`

To modify, edit `main.go` and relevant service files.

## 📊 Database Schema

The application uses SQLite3 with the following main tables:
- `users` - User accounts and authentication
- `parzellen` - Allotment plots
- `inspektionen` - Inspection records
- `wertermittlungen` - Property valuations
- `obstarten` - Fruit tree reference data
- `zieranpflanzungen` - Ornamental plant reference data
- `audit_log` - Administrative action logging

Database is auto-initialized on first run with `kleingarten.db`.

## 🧪 Testing

To build the project:
```bash
go build -o kleingarten-verwaltung
```

To run with verbose logging:
```bash
go run main.go 2>&1 | grep -E "^(🔐|📊|👤|⚠️)"
```

## 📝 License

[Specify your license here]

## 👥 Authors

- [Your Name/Organization]

## 🐛 Known Issues & Limitations

1. **In-Memory Sessions** - Sessions are stored in memory and lost on restart
2. **Single Instance** - Not designed for distributed deployment
3. **SQLite Concurrency** - Limited concurrent write capability
4. **Session Persistence** - No persistent login across restarts
5. **File Uploads** - Only filename extension validated, not actual content

See [SECURITY_AUDIT.md](SECURITY_AUDIT.md) for detailed findings and recommendations.

## 🤝 Contributing

Please ensure:
- All error handling is explicit (no ignored `_` assignments)
- Input validation on all user-provided data
- Security review before adding authentication-related features

## 📞 Support

For issues or questions, please [specify support channel].
