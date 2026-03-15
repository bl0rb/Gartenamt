# AI_README - Developer Handoff Guide

Quick reference for developers continuing/modifying this project.

## 🎯 Project Overview

**Kleingarten-Verwaltung** - Allotment garden management system (Go + SQLite + Bootstrap 5).

- **Language:** Go 1.21
- **Database:** SQLite3
- **Status:** Production-ready, core features complete
- **Port:** 8080 (HTTPS)

---

## 🚀 Quick Start (Development)

```bash
git clone https://github.com/bl0rb/kleingarten-verwaltung.git
cd kleingarten-verwaltung
go mod download
go build -o app main.go
./app
```

**Credentials:** `admin` / `admin123` (change in production!)

---

## 📂 Architecture

### Directory Structure
```
handlers/        # HTTP handlers (auth, admin, parzelle, inspektion, wertermittlung)
middleware/      # auth_middleware.go (RequireAuth, RequireAdmin, GetSessionFromContext)
models/          # Database models + CRUD operations
services/        # Business logic (auth, calculations, CSV)
templates/       # HTML templates (Bootstrap 5, German)
static/          # CSS/JS
main.go          # Routes & initialization
```

### Core Models
- `User` - Authentication + roles
- `Parzelle` - Plots
- `Inspektion` - Inspections (defects, PDF export)
- `Wertermittlung` - Valuations (auto-calculated)
- `ObstArt`, `Zieranpflanzung`, `Bauindex` - Reference data

### Database Tables
```
users, parzellen, inspektionen, wertermittlungen
obstarten, zieranpflanzungen, bauindex_tabelle
audit_log, sessions
```

---

## 🔄 Common Development Patterns

### Adding New Admin Reference Data

**Example: New "Beerenarten" type**

1. Add model in `models/admin_models.go`
2. Add CRUD functions (GetAll, Create, Update, Delete)
3. Create handler in `handlers/admin_handler.go` (copy from admin_obstarten pattern)
4. Add routes in `main.go` (under `adminRoutes`)
5. Create template `templates/admin_beeren.html`
6. Add dashboard link in `templates/admin_dashboard.html`
7. Add CSV export if needed in `services/csv_service.go`

### Adding New Valuation Calculation

1. Add method to `BerechnungsService` in `services/berechnungs_service.go`
2. Call from `wertermittlung_handler.go` in form processing
3. Add result to template data map
4. Update `templates/wertermittlung.html` to display

### Adding New Route

```go
// main.go in appropriate section
adminRoutes.HandleFunc("/newfeature", 
    middleware.RequireAdmin(handlers.NewFeatureHandler)).Methods("GET", "POST")

// handlers/newfeature_handler.go
func NewFeatureHandler(w http.ResponseWriter, r *http.Request) {
    session := middleware.GetSessionFromContext(r.Context())
    data := make(map[string]interface{})
    handlers.AddSessionToData(r, data) // ⚠️ Always include this
    
    if r.Method == "POST" {
        // Process form
        // Redirect back
    }
    // GET: Render template
}
```

### Template Pattern

All templates extend `layout.html`:

```html
{{define "content"}}
<div>Content here</div>
{{end}}
```

**Critical:** Always call `AddSessionToData(r, data)` before template execution!

---

## ✅ What's Complete

- ✅ Authentication (bcrypt + sessions)
- ✅ RBAC (Admin/User roles)
- ✅ Parzellen management
- ✅ Inspections + PDF export
- ✅ Valuation calculations
- ✅ Reference data admin (Obstarten, Zieranpflanzungen, Bauindex)
- ✅ CSV import/export
- ✅ Audit logging
- ✅ Utilities (Strom/Wasser)
- ✅ Docker support (AMD64)

---

## ⚠️ Missing/TODO

1. **CSRF Protection** - Add to middleware
2. **Rate Limiting** - Login attempts + API
3. **Session Persistence** - Currently in-memory (lost on restart)
4. **Email** - Password reset, notifications
5. **API Tokens** - For mobile/external access

---

## 🛡️ Security Checklist

- ✅ SQL parameterized queries (injection safe)
- ✅ Password hashing (bcrypt)
- ✅ Session auth with HTTPOnly cookies
- ⚠️ Missing: CSRF tokens
- ⚠️ Missing: Rate limiting
- ⚠️ Missing: Secure cookie flags (set for production)
- ⚠️ Missing: Input size limits
- ⚠️ Missing: File upload validation

See [SECURITY_AUDIT.md](SECURITY_AUDIT.md) for details.

---

## 🔍 Debugging

### Database
```bash
sqlite3 kleingarten.db ".schema"
sqlite3 kleingarten.db "SELECT * FROM users;"
```

### Build & Test
```bash
go build -o app main.go
./app
# Visit https://localhost:8080
```

### Common Issues
- **Template not rendering?** - Check `AddSessionToData(r, data)` is called
- **Auth failing?** - Check `middleware/auth_middleware.go` and session context
- **Calculations wrong?** - Check `services/berechnungs_service.go`

---

## 📋 Key Files

| File | Purpose |
|------|---------|
| `main.go` | Routes + initialization |
| `middleware/auth_middleware.go` | Auth/permission checks |
| `services/berechnungs_service.go` | Valuation calculations |
| `models/database.go` | Schema + DB init |
| `handlers/template_helpers.go` | `AddSessionToData()` |
| `templates/layout.html` | Base template + nav |

---

## 🚀 Deployment

### Local
```bash
go build && ./app
```

### Docker
```bash
docker build -t kleingarten:latest .
docker run -d -p 8080:8080 -v data:/data kleingarten:latest
```

### Production Tips
- Change admin password first
- Enable HTTPS (set TLS cert path)
- Set secure cookie flags in production code
- Use persistent DB (not in-memory)
- Implement CSRF protection
- Add rate limiting

---

## 📝 Before Committing

- [ ] Run build: `go build`
- [ ] No unused imports (`goimports -w`)
- [ ] All templates have `AddSessionToData(r, data)`
- [ ] All DB queries are parameterized
- [ ] Changes logged/documented
- [ ] Tests pass (if applicable)

---

**Last Updated:** 2026-03-15  
**For Questions:** Check code comments and SECURITY_AUDIT.md
