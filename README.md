# Kleingarten-Verwaltung

Kleingarten-Verwaltung is a modular system with three deployable parts:

- Verwaltung App (Go + SQLite): internal member/admin workflows
- License Server (Go + SQLite): license issuing, validation, revocation
- Public Webpage (Nginx static site): public-facing content

## Repository Layout

- `main.go`, `handlers/`, `models/`, `services/`: Verwaltung App
- `modules/license-server/`: dedicated license server module
- `modules/public-web/`: public website module
- `docker-compose.modular.yml`: multi-module docker setup

## Quick Start (Modular Docker)

1. Copy environment template:

```bash
cp .env.modular.example .env
```

2. Fill required values in `.env`:
- `LICENSE_PUBLIC_KEY`
- `LICENSE_PRIVATE_KEY_BASE64` (required for issue/revoke in license server)
- `LICENSE_SERVER_ADMIN_TOKEN`
- `LICENSE_SERVER_CLIENT_TOKEN`

3. Start all services:

```bash
docker compose -f docker-compose.modular.yml up --build
```

## Service Endpoints

- Verwaltung App: http://localhost:8080
- License Server Health: http://localhost:8090/health
- Public Webpage: http://localhost:8081

## License Flow

- License Server signs/validates license keys.
- Verwaltung App uses premium feature gating and can call License Server.
- Public Webpage only links to the app login and does not include admin logic.

## Build (Root App)

```bash
go build ./...
```

## Step-by-Step Guide (NAS/Server)

1. Prepare certificates for public HTTPS.

Create these files:
- `modules/public-web/certs/fullchain.pem`
- `modules/public-web/certs/privkey.pem`

2. Create your environment file.

```bash
cp .env.modular.example .env
```

3. Set required secrets in `.env`.

Minimum values:
- `LICENSE_SERVER_ADMIN_TOKEN=your-admin-token`
- `LICENSE_SERVER_CLIENT_TOKEN=your-client-token`

Optional key bootstrap modes:
- Mode A (automatic first start): leave `LICENSE_PUBLIC_KEY` and `LICENSE_PRIVATE_KEY_BASE64` empty.
- Mode B (fixed keys): set both values explicitly.

4. Start the full stack.

```bash
docker compose -f docker-compose.modular.yml up -d --build
```

5. Open the license server first.

- UI: `http://<server>:8090/`
- On first start it auto-generates keys if not provided.
- Copy and store the displayed public key/fingerprint.

6. Generate first app license code.

In the license server UI:
- Enter `Admin Token`
- Plan: `premium`
- Issued To: your organization
- Features: `wertermittlung,inspektion,mailing,invoice_print`
- Click `Lizenz erstellen`

7. Activate the license in the app.

- Open Verwaltung App: `http://<server>:8080`
- Go to Admin -> System-Info -> Lizenzverwaltung
- Paste generated `KGV1...` key and activate

8. Verify public webpage via HTTPS.

- HTTPS endpoint: `https://<server>:8443`
- HTTP endpoint (`:8081`) redirects to HTTPS

9. Validate service health.

```bash
docker compose -f docker-compose.modular.yml ps
curl -k https://<server>:8443/
curl http://<server>:8090/health
```

10. Update production ports (optional).

For a real domain on NAS, map public web ports to 80/443 in `docker-compose.modular.yml`.
