# AI_README - Kleingarten-Verwaltung (Developer Handoff Guide)

## 🎯 Project Overview

**Kleingarten-Verwaltung** is a web-based management system for allotment garden associations built with Go. It manages plots (Parzellen), inspections, property valuations (Wertermittlungen), and administrative functions.

- **Status:** Active development, core features complete
- **Language:** Go 1.21
- **Database:** SQLite3
- **License Model:** Freemium (planned - see section below)
- **Git User:** werkm@bl0rb.de

---

## 🚀 Quick Start

### Build & Run
```bash
cd /Users/matze/Github/kleingarten-verwaltung
go build -o app main.go
./app
```

App runs on: `http://localhost:8080`

### Default Credentials
- Username: `admin`
- Password: `admin123`
- ⚠️ **Change immediately in production!**

---

## 🏗️ Architecture

### Project Structure
```
.
├── main.go                         # Entry point (routes & initialization)
├── handlers/                       # HTTP request handlers
│   ├── auth_handler.go            # Login/logout/user auth (✓ Complete)
│   ├── admin_handler.go           # Admin dashboard & reference data CRUD
│   ├── admin_csv_handler.go       # Backup & CSV export/import
│   ├── admin_delete_handler.go    # Bulk delete & audit operations
│   ├── parzelle_handler.go        # Plot management (✓ Complete)
│   ├── inspektion_handler.go      # Inspections (✓ Complete)
│   ├── wertermittlung_handler.go  # Valuations (✓ Complete)
│   ├── api_handler.go             # JSON API endpoints
│   └── template_helpers.go        # Template utilities (AddSessionToData)

├── middleware/
│   └── auth_middleware.go         # RequireAuth, RequireAdmin, GetSessionFromContext

├── models/
│   ├── database.go                # DB initialization, schema creation
│   ├── user.go                    # User model & auth logic
│   ├── parzelle.go                # Plot model
│   ├── inspektion.go              # Inspection model
│   ├── wertermittlung.go          # Valuation model
│   └── admin_models.go            # Admin reference data (ObstArt, Zieranpflanzung, Bauindex)

├── services/
│   ├── auth_service.go            # Session management, authentication
│   ├── berechnungs_service.go     # Valuation calculations
│   └── csv_service.go             # CSV import/export

├── templates/                      # Go HTML templates (✓ Bootstrap 5, German UI)
│   ├── layout.html                # Base template with navigation
│   ├── admin_*.html               # Admin pages
│   ├── parzelle*.html             # Plot pages
│   ├── inspektion.html            # Inspection page
│   ├── wertermittlung.html        # Valuation page
│   └── ...

├── static/
│   └── style.css                  # Additional CSS
```

### Database Schema
```sql
-- Core Tables
users (id, username, email, hashed_password, role, erstellt_am)
parzellen (id, nummer, besitzer, flaeche, lage, gesamtwert, ...)
inspektionen (id, parzelle_id, datum, mangelcodes, notes, ...)
wertermittlungen (id, parzelle_id, obst_wert, zier_wert, gemüse_wert, ...)

-- Admin Reference Data
obstarten (id, name, kategorie, einheit, standardpreis, max_anzahl, aktiv)
zieranpflanzungen (id, name, kategorie, preis_pro_qm, max_flaeche, aktiv)
bauindex_tabelle (jahr, bauindex)

-- System Tables
audit_log (id, benutzer_id, aktion, beschreibung, zeitstempel, ip_adresse)
sessions (id, user_id, created_at, last_seen) - loaded on startup
```

---

## 🔄 Recent Changes & What's Done

### ✅ Completed Features
1. **User Authentication** - bcrypt password hashing, session-based auth
2. **Role-Based Access Control** - Admin vs. User roles with middleware
3. **Parzellen Management** - CRUD operations with statistics
4. **Inspections** - Defect tracking (Mängel) with PDF export
5. **Wertermittlung** - Full valuation calculation engine
6. **Admin Dashboard** - Statistics, management interfaces
7. **Reference Data Management:**
   - Obstarten (Fruit types) - CRUD with prices
   - Zieranpflanzungen (Ornamental plants) - CRUD with prices
   - **Bauindex (Building index)** - NEW: Year-based flexible management
8. **CSV Export/Import** - All data exportable, reference data importable
9. **Audit Logging** - Track all admin actions
10. **Template Fixes** - Fixed "contains" function and Session passing to all templates
11. **Admin Warning** - Auto-fade warning on admin dashboard (15s)

### 🔧 Recent Implementation Details
- **Session Context Passing:** All template handlers now use `AddSessionToData(r, data)` helper
- **FuncMap for Templates:** `contains()` function added to template FuncEnv for browser detection
- **Bauindex Flexibility:** Changed from hardcoded value to database-driven with year support
- **Admin Dashboard Enhancement:** Added management cards for Obstarten, Zieranpflanzungen, Bauindex

### 📋 TODO/Known Issues
1. **CSRF Protection** - Not implemented (security concern)
2. **Rate Limiting** - No login attempt rate limiting
3. **Session Persistence** - Sessions lost on restart (in-memory only)
4. **Email Verification** - Not implemented
5. **Password Reset** - Manual admin password reset only
6. **Input Validation** - Basic server-side validation exists, could be more thorough
7. **API Security** - No API token/key system yet

---

## 💳 License Model Planning

### Current Status: **NOT IMPLEMENTED**
User wants Freemium model. Implementation needed:

```go
// Database tables to add:
license_tiers (id, name, max_plots, max_users, price_per_month, features)
subscriptions (id, user_id, tier_id, start_date, end_date, payment_status)
license_keys (key, tier, expires_at, is_active) // for self-hosted option
```

### Recommended Implementation Path:
1. Add license middleware checking subscription status
2. Create PayPal/Stripe integration for payments
3. Feature gating based on tier
4. Admin panel for subscription management
5. Trial period logic (14 days free?)

### Key Decision Pending:
- **SaaS only** vs. **Self-hosted with license keys** vs. **Both**?
- **Pricing tiers** need definition
- **Free tier limits** (10 plots? 1 user?)

---

## 🔐 Security Status

### ✅ Implemented
- bcrypt password hashing
- SQL parameterized queries (no SQL injection)
- Session-based auth with HttpOnly cookies
- Role-based middleware checks

### ⚠️ Missing/TODO
- CSRF token validation (handlers/middleware/auth_middleware.go)
- Rate limiting on login (services/auth_service.go)
- Input size limits on file uploads
- Secure cookie flags (Secure, SameSite in production)
- HTTPS enforcement in production

See [SECURITY_AUDIT.md](SECURITY_AUDIT.md) for full details.

---

## 🛠️ Common Patterns Used

### 1. Adding a New Admin Reference Data Type

**Example: Adding new "Beerenarten" (berry types)**

```go
// 1. Add model in models/admin_models.go
type Beerenart struct {
    ID            int     `json:"id"`
    Name          string  `json:"name"`
    Preis         float64 `json:"preis"`
    Aktiv         bool    `json:"aktiv"`
}

// 2. Add CRUD functions in models/admin_models.go
func GetAllBeerenarten() ([]Beerenart, error) { ... }

// 3. Create handler in handlers/admin_handler.go
func AdminBeerenHandler(w http.ResponseWriter, r *http.Request) {
    // Copy pattern from AdminObstartenHandler
    // GET: load data, execute template
    // POST: handle handleBeerenPost(r)
}

// 4. Add delete handler
func AdminBeerenLoeschenHandler(w http.ResponseWriter, r *http.Request) { ... }

// 5. Add routes in main.go
adminRoutes.HandleFunc("/beeren", middleware.RequireAdmin(...)).Methods("GET", "POST")
adminRoutes.HandleFunc("/beeren/{id}/delete", middleware.RequireAdmin(...)).Methods("POST")

// 6. Create template templates/admin_beeren.html
// Copy from admin_obstarten.html, adjust fields

// 7. Add dashboard link
// In templates/admin_dashboard.html, add card to Verwaltungen section
```

### 2. Adding New Calculation to Wertermittlung

Handler: `handlers/wertermittlung_handler.go` → calls `services/berechnungs_service.go`

```go
// 1. Add method to BerechnungsService
func (s *BerechnungsService) BerechneBeerenWert(...) (float64, error) {
    // Query database or use fallback
}

// 2. Call from handleWertermittlungGet()
// 3. Add to map[string]interface{} passed to template
```

### 3. Template Pattern

All templates use `{{define "content"}}` and extend `layout.html`:

```html
{{define "content"}}
<div class="row">
    <div class="col-12">
        <h2>{{.Title}}</h2>
        {{if .Session}}
            <!-- Show user content -->
        {{else}}
            <!-- Shouldn't happen, middleware prevents this -->
        {{end}}
    </div>
</div>
{{end}}
```

**Important:** Always pass `AddSessionToData(r, data)` in handler!

### 4. Adding New Route

```go
// In main.go, inside adminRoutes section:
adminRoutes.HandleFunc("/newfeature", middleware.RequireAdmin(handlers.NewFeatureHandler)).Methods("GET", "POST")

// Handler pattern:
func NewFeatureHandler(w http.ResponseWriter, r *http.Request) {
    // Check request method
    // If POST: parse form, save to DB, redirect with ?success=...
    // If GET: load data, execute template with AddSessionToData(r, data)
}
```

---

## 🚢 How to Deploy/Continue Development

### Adding New Feature Checklist

- [ ] Create model in `models/`
- [ ] Create handler(s) in `handlers/`
- [ ] Create template(s) in `templates/`
- [ ] Add routes in `main.go`
- [ ] Add `AddSessionToData(r, ...)` to all template handler calls
- [ ] Test with `go build && ./app`
- [ ] Check database queries for SQL injection
- [ ] Verify authentication/authorization on sensitive routes
- [ ] Update `README.md` features section
- [ ] Test all forms and redirects

### Testing Checklist
```bash
# Build
go build -o app main.go

# Run
./app

# Test in browser
# Login: admin / admin123
# Check:
# - Can create/edit/delete parzelle
# - Can create inspection & export PDF
# - Admin features work (reference data management)
# - Audit log records actions
# - CSV export works
```

---

## 📚 Important Files to Know

| File | Purpose | Edit When |
|------|---------|-----------|
| `main.go` | Route definitions | Adding new routes/features |
| `middleware/auth_middleware.go` | Auth checks | Changing permission logic |
| `services/berechnungs_service.go` | Valuation calculations | Changing pricing/calculations |
| `models/database.go` | Schema creation | Adding database tables |
| `handlers/template_helpers.go` | `AddSessionToData()` | Improving template context |
| `templates/layout.html` | Navigation & base layout | Global UI changes |

---

## 🐛 Debugging Tips

### Database Issues
```bash
# Check DB schema
sqlite3 kleingarten.db ".schema"
sqlite3 kleingarten.db "SELECT * FROM users LIMIT 5;"
```

### Template Not Rendering
- Check `AddSessionToData(r, data)` is used
- Verify template file exists in `templates/`
- Check for typos in `{{.FieldName}}`

### Session/Auth Issues
- Check `middleware/auth_middleware.go` - `GetSessionFromContext()`
- Verify `services/auth_service.go` - session management
- Look at `handlers/auth_handler.go` - login logic

---

## 📞 Next Steps for Developer

### Immediate Priorities (if continuing):

1. **Implement License Model**
   - Add subscription tables
   - Create payment integration
   - Add feature gating middleware

2. **Security Hardening**
   - Add CSRF protection
   - Implement rate limiting
   - Set secure cookie flags

3. **Email Functionality**
   - Password reset via email
   - Notification system

4. **API Expansion**
   - RESTful API with authentication tokens
   - Mobile app support

---

## 📝 Git Info
- **Author:** werkm@bl0rb.de (as of 2026-03-15)
- **Platform:** macOS
- **Go Version:** 1.21+
- **SQLite3:** Latest

---

## 🔗 Related Files
- [README.md](README.md) - User documentation
- [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - Security findings
- [go.mod](go.mod) - Dependencies

---

*Last Updated: 2026-03-15*
*For AI Assistance: Feel free to reference this guide when making changes to the project.*
