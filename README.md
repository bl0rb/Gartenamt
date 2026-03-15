# Kleingarten-Verwaltung

A web-based management system for allotment gardens (Kleingartenanlagen) built with Go.

## 🎯 Overview

- **Web App:** Browser-based, no installation required
- **Database:** SQLite3, auto-initialized
- **Platforms:** macOS, Windows, Linux, Docker (NAS)
- **Architecture:** Multi-platform amd64 and ARM64 support


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

## � Build Releases

Create all platform binaries and Docker image with one command:

```bash
./build-release.sh
```

**Generated Artifacts** (in `/binary/`):
- `kleingarten-verwaltung-linux-amd64` - Linux binary
- `kleingarten-verwaltung-windows-amd64.exe` - Windows executable
- `kleingarten-verwaltung-macos-amd64` & `.app` - macOS Intel
- `kleingarten-verwaltung-macos-arm64` & `.app` - macOS Apple Silicon
- `kleingarten-verwaltung-docker-VERSION.tar.gz` - Docker image
- `CHECKSUMS.txt` - SHA256 verification for all artifacts

**Usage:**
- **Linux:** `./kleingarten-verwaltung-linux-amd64`
- **Windows:** Double-click `kleingarten-verwaltung.exe`
- **macOS:** Open `kleingarten-verwaltung-macos-*.app`
- **Docker:** `docker load < kleingarten-verwaltung-docker-*.tar.gz && docker run -p 8080:8080 -v data:/data kleingarten-verwaltung`

---

## 🐳 Docker Deployment

### On NAS via Docker GUI

1. **Transfer image to NAS:**
   ```bash
   scp binary/kleingarten-verwaltung-docker-*.tar.gz user@nas:/path/
   ```

2. **Load image** (via NAS Docker interface):
   - Select "Import Image"
   - Choose the `.tar.gz` file
   - Wait for import to complete

3. **Create container**:
   - Image: `kleingarten-verwaltung:latest`
   - Port mapping: `8080:8080`
   - Volume mount: `/data` (for database persistence)
   - Start container

Access at: `http://nas-ip:8080`

### Command Line
```bash
docker load < kleingarten-verwaltung-docker-VERSION.tar.gz
docker run -d --name kleingarten -p 8080:8080 -v kleingarten-data:/data kleingarten-verwaltung:VERSION
```

### Versioning for Builds

Release builds now use an explicit project version from the VERSION file.

1. Initial version is 0.1.1 (see VERSION file).
2. Before each release, increase VERSION (for example 0.1.2 or 0.2.0).
3. Run build script:

```bash
./build-release.sh
```

Optional one-off override:

```bash
./build-release.sh 0.2.0
```

Docker images are tagged with both VERSION and latest.

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
├── Dockerfile                   # Docker build (linux/amd64)
├── build-release.sh             # Build all platforms & Docker
├── go.mod / go.sum              # Go dependencies
├── handlers/                    # HTTP request handlers
├── middleware/                  # Auth middleware
├── models/                      # Database models & queries
├── services/                    # Business logic
├── templates/                   # HTML templates
├── static/                      # CSS, JavaScript
├── binary/                      # Build outputs (generated, .gitignored)
└── .github/workflows/           # CI/CD pipelines
```

---

## 🔧 Development

**Prerequisites:**
- Go 1.21+
- SQLite3
- Docker (for containerization)

**Environment Setup:**
```bash
git clone https://github.com/bl0rb/kleingarten-verwaltung.git
cd kleingarten-verwaltung
go mod download
```

**Run Development Server:**
```bash
go run main.go
```
App opens in browser at `http://localhost:8080`

**Build for Current Platform:**
```bash
go build -o kleingarten-verwaltung .
```

**Build Release Package:**
```bash
./build-release.sh     # Creates all platform binaries + Docker
```

---

## 📋 Technology Stack

- **Language:** Go 1.21
- **Router:** Gorilla Mux
- **Database:** SQLite3 (auto-initialized)
- **PDF Generation:** gofpdf
- **Frontend:** HTML5, CSS3, Bootstrap 5
- **Container:** Docker with Ubuntu 22.04 (amd64)
- **CI/CD:** GitHub Actions

---

## 🛡️ Security

### Implementation
- Password hashing with bcrypt
- Session authentication (HTTPOnly cookies)
- Role-based access control (Admin/User)
- Parameterized SQL queries
- IP logging for audit trail

### Production Checklist
- ✅ Change default admin password immediately
- ✅ Enable HTTPS/TLS
- ✅ Set secure cookie flags (Secure, SameSite=Strict)
- ✅ Implement rate limiting on login
- ✅ Regular database backups
- ✅ Monitor audit logs

See [SECURITY_AUDIT.md](SECURITY_AUDIT.md) for full audit report.

---

## 🐛 Troubleshooting

**Application won't start:**
- Check port 8080 is available: `lsof -i :8080`
- Verify Go version: `go version` (needs 1.21+)
- Check permissions: `chmod +x kleingarten-verwaltung`

**Database issues:**
- SQLite file: `kleingarten.db` (created on first run in app directory)
- Reset database: Delete `kleingarten.db` and restart
- Backup database: `cp kleingarten.db kleingarten.db.backup`

**Docker on NAS:**
- Verify image loaded: `docker images | grep kleingarten`
- Check container logs: `docker logs <container-id>`
- Ensure volume persists: `docker volume ls | grep kleingarten`

**Browser access issues:**
- Try `http://localhost:8080` instead of `https://`
- Clear browser cache/cookies
- Check firewall rules for port 8080

---

## 📝 License

MIT

---

## 👤 Development

Built with ❤️ for community garden management.