# Kleingarten-Verwaltung

A web-based management system for allotment gardens (Kleingartenanlagen) built with Go.


## 🚀 Quick Start (Local)

### Requirements
- Go 1.21+
- SQLite3

### Run Locally
```bash
git clone https://github.com/bl0rb/kleingarten-verwaltung.git
cd kleingarten-verwaltung
go mod download
go build -o kleingarten-verwaltung main.go
./kleingarten-verwaltung
```

Open `https://localhost:8080`

**Default Credentials:**
- Username: `admin`
- Password: `admin123`
- ⚠️ Change immediately in production!

---

## 🐳 Docker (NAS/Server)

Import and run the AMD64 Docker image:
```bash
docker load -i kleingarten-verwaltung-amd64.tar
docker run -d -p 8080:8080 -v kleingarten-data:/data kleingarten-verwaltung:amd64
```

Or use compose:
```bash
docker-compose up -d
```

Database persists in `/data` volume.

---

## ✨ Features

### 🔐 User Management
- Authentication with encrypted passwords
- Role-based access (Admin/User)
- Session management
- Activity logging

### 🏡 Plot Management (Parzellen)
- Create/manage allotment plots
- Track owners & contact info
- Link inspections & valuations

### 🔍 Inspections
- Record plot inspections & defects
- PDF protocol generation
- Inspection history

### 💰 Valuations (Wertermittlung)
- Automatic valuation calculation
- Fruit trees, ornamental plants, structures
- Building index adjustment
- PDF generation

### ⚡ Utilities
- Water (Wasser) & Electricity (Strom) records
- Invoice generation
- Organization payment settings (IBAN, address)

### 📊 Admin Tools
- CSV import/export
- Database backups
- Audit logging
- Reference data management

---

## 📁 Project Structure

```
.
├── main.go                      # Entry point
├── Dockerfile                   # Docker build
├── docker-compose.yml           # Compose config
├── build-release.sh             # Build all platforms
├── handlers/                    # HTTP handlers
├── middleware/                  # HTTP middleware
├── models/                      # Database models
├── services/                    # Business logic
├── templates/                   # HTML templates
├── static/                      # CSS, JS
└── binary/                      # Build outputs (ignored)


---

## 🔧 Build All Platforms

```bash
bash build-release.sh
```

Generates in `binary/`:
- Linux (amd64)
- Windows (amd64)
- macOS Intel (amd64)
- macOS ARM (arm64)
- Checksums

---

## 📋 Technology Stack

- **Language:** Go 1.21
- **Web:** Gorilla Mux (routing)
- **Database:** SQLite3
- **PDF:** gofpdf
- **Frontend:** HTML5, CSS3, Bootstrap 5
- **Container:** Docker (Ubuntu 22.04)

---

## 🛡️ Security

### Current Implementation
- Password hashing (bcrypt)
- Session authentication with HTTPOnly cookies
- Role-based access control
- Parameterized SQL queries

### Recommendations for Production
1. **Change default admin password immediately**
2. **Enable HTTPS** (app supports TLS)
3. **Set secure cookie flags** (Secure, SameSite)
4. **Implement CSRF protection**
5. **Rate limit login attempts**
6. **Use persistent session storage** (for multi-instance)

See [SECURITY_AUDIT.md](SECURITY_AUDIT.md) for detailed findings.
