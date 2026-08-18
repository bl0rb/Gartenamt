# Gartenamt

[![Build Check](https://img.shields.io/github/actions/workflow/status/bl0rb/Gartenamt/docker-build.yml?branch=main&label=Build%20Check&logo=githubactions&logoColor=white&style=flat-square)](https://github.com/bl0rb/Gartenamt/actions/workflows/docker-build.yml)
[![Release](https://img.shields.io/github/v/release/bl0rb/Gartenamt?label=Release&logo=github&color=25503C&style=flat-square)](https://github.com/bl0rb/Gartenamt/releases/latest)
[![Lizenz](https://img.shields.io/badge/Lizenz-PolyForm%20Noncommercial%201.0.0-6E7970?style=flat-square)](LICENSE)

[![Go](https://img.shields.io/badge/Go-1.26.6-00ADD8?logo=go&logoColor=white&style=flat-square)](go.mod)
[![SQLite](https://img.shields.io/badge/SQLite-eingebettet-003B57?logo=sqlite&logoColor=white&style=flat-square)](#repository-struktur)
[![macOS](https://img.shields.io/badge/macOS-universal-000000?logo=apple&logoColor=white&style=flat-square)](https://github.com/bl0rb/Gartenamt/releases/latest)
[![Windows](https://img.shields.io/badge/Windows-amd64-0078D4?logo=windows&logoColor=white&style=flat-square)](https://github.com/bl0rb/Gartenamt/releases/latest)
[![Linux](https://img.shields.io/badge/Linux-amd64-FCC624?logo=linux&logoColor=black&style=flat-square)](https://github.com/bl0rb/Gartenamt/releases/latest)
[![Docker](https://img.shields.io/badge/Docker-ubuntu%2026.04-2496ED?logo=docker&logoColor=white&style=flat-square)](DOCKER_DEPLOYMENT.md)

[![govulncheck](https://img.shields.io/badge/govulncheck-0%20Befunde-2E6A44?logo=go&logoColor=white&style=flat-square)](.github/workflows/docker-build.yml)
[![Audit & Pentest](https://img.shields.io/badge/Audit%20%26%20Pentest-Claude%20Opus%205-D97757?logo=anthropic&logoColor=white&style=flat-square)](#sicherheit)
[![Container](https://img.shields.io/badge/Container-non--root%20uid%2010001-0DB7ED?logo=docker&logoColor=white&style=flat-square)](Dockerfile)
[![HTTP-Header](https://img.shields.io/badge/HTTP-CSP%20%C2%B7%20HSTS%20%C2%B7%20SameSite-3F6382?style=flat-square)](middleware/security.go)

Verwaltungssoftware für Kleingartenvereine: Parzellen, Pächter, Inspektionen, Wertermittlungen, Wasser-/Stromabrechnung und Rechnungsversand — als einzelne Go-Anwendung mit SQLite, ohne externe Dienste selbst zu hosten.

*Management software for German allotment garden associations (Kleingartenvereine), built with Go and SQLite. The application and its documentation are in German, as its workflows follow German allotment-garden regulations.*

## Funktionen

- **Parzellenverwaltung** — Parzellen mit Pächterdaten anlegen, bearbeiten und verwalten
- **Inspektionen** — Begehungsprotokolle mit Mängelerfassung (im Standard nach den Hamburger Richtlinien)
- **Wertermittlung** — Wertermittlungsprotokolle mit PDF-Export (im Standard nach der Richtlinie des Landesbundes der Gartenfreunde in Hamburg e.V.)
- **Abrechnung** — Wasser-/Stromzählerstände, Rechnungserstellung als PDF, Sammelexport
- **E-Mail-Versand** — Rechnungen und Infos direkt an Pächter senden (SMTP)
- **Benutzer & Rollen** — Rollenmodell (Admin, Vorstand, Kassenwart, Wertermittler, Benutzer) mit feingranularen Berechtigungen
- **Backup** — verschlüsselte Datenbank-Backups mit Wiederherstellung, CSV-Export/-Import
- **Audit-Log** — Nachverfolgung administrativer Aktionen

Die Weboberfläche läuft komplett lokal (HTTPS mit selbstsigniertem Zertifikat); es werden keine externen Dienste oder CDNs eingebunden.

## Schnellstart (lokal)

Voraussetzung: Go 1.26.6+ und ein C-Compiler (für SQLite/cgo).

```bash
git clone https://github.com/bl0rb/Gartenamt.git
cd Gartenamt
go run .
```

Die App kennt zwei Betriebsmodi:

- **Desktop-Modus** (Standard, z.B. Doppelklick auf App/Exe): HTTP unter `http://localhost:8080`, gebunden nur an das Loopback-Interface — keine Zertifikatswarnung, von außen nicht erreichbar. Der Browser öffnet sich automatisch, und in der Menüleiste (macOS) bzw. im System-Tray (Windows) erscheint ein Symbol mit *Gartenamt öffnen* und *Beenden*.
- **Server-Modus** (`--no-browser`, z.B. Docker/NAS): HTTPS unter `https://localhost:8080` auf allen Interfaces. Das TLS-Zertifikat ist selbstsigniert; die Browser-Warnung beim ersten Aufruf ist erwartbar. Zusätzliche Hostnamen für das Zertifikat können über `TLS_EXTRA_HOSTS` konfiguriert werden.

Beim ersten Start wird ein Administrator-Konto **admin** mit einem **zufällig generierten Passwort** angelegt. Die Zugangsdaten werden auf der Konsole ausgegeben und **direkt auf der Login-Seite angezeigt**, bis das Passwort beim ersten Login geändert wurde (die Änderung wird erzwungen). Wird die App vor dem ersten Login neu gestartet, wird das Initialpasswort neu generiert.

## Schnellstart (Docker)

```bash
docker compose up -d
```

Die Datenbank liegt im Volume `gartenamt-data` (`/data/kleingarten.db`).

**NAS (Synology & Co.):** [docker-compose.nas.yml](docker-compose.nas.yml) zieht das fertige Image von GHCR (`ghcr.io/bl0rb/gartenamt`) — nichts bauen, Daten liegen in `./nas-data/` neben der Compose-Datei, Update per `docker compose pull`. Details: [DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md).

Optional gibt es ein modulares Setup mit zusätzlicher öffentlicher Vereins-Webseite (Nginx):

```bash
docker compose -f docker-compose.modular.yml up --build
```

## Konfiguration

Alle Einstellungen sind optional und werden über Umgebungsvariablen (oder eine `.env`-Datei, siehe [.env.example](.env.example)) gesetzt:

| Variable | Zweck |
|---|---|
| `APP_SECRET_KEY` | Schlüssel für Backup-Verschlüsselung (32 Bytes, base64). Wird sonst beim ersten Start erzeugt. |
| `APP_SECRET_KEY_FILE` | Alternativ: Pfad zur Schlüsseldatei (Default: `.app_secret`) |
| `DB_PATH` | Pfad zur SQLite-Datenbank (Default: `kleingarten.db`) |
| `TLS_EXTRA_HOSTS` | Zusätzliche Hostnamen für das selbstsignierte Zertifikat (kommagetrennt) |
| `TRUSTED_PROXY_IPS` | IPs vertrauenswürdiger Reverse-Proxys für die korrekte Client-IP-Ermittlung |

SMTP-Zugangsdaten für den E-Mail-Versand werden in der App unter *Admin → Vereinseinstellungen* hinterlegt (verschlüsselt gespeichert).

## Wertermittlung nach LGH-Richtlinie

Wertermittlung und Inspektion folgen im Standard den Hamburger Richtlinien: der Richtlinie für die Inspektion und Wertermittlung von Kleingärten bei Pächterwechsel des [Landesbundes der Gartenfreunde in Hamburg e.V.](https://www.gartenfreunde-hh.de/) Die offiziellen Richtlinien- und Formulardokumente sind urheberrechtlich geschützt und daher nicht Teil dieses Repositories — sie sind beim Landesbund erhältlich. Für andere Landesverbände lassen sich die Bewertungsgrundlagen (Obstgehölze, Zieranpflanzungen, Bauindex) in der App unter *Admin → Stammdaten* anpassen.

## Repository-Struktur

- `main.go`, `handlers/`, `models/`, `services/`, `middleware/` — die Verwaltungs-App
- `templates/`, `static/` — Weboberfläche (eingebettet ins Binary, inkl. lokal ausgelieferter Bootstrap-/Font-Awesome-Assets)
- `securestore/` — Verschlüsselung für gespeicherte Zugangsdaten und Backups
- `modules/public-web/` — optionale öffentliche Vereins-Webseite (statisch, Nginx)
- `build-release.sh` — Release-Build inkl. Docker-Image-Export

## Build

```bash
go build ./...        # kompilieren
go vet ./...          # statische Prüfung
./build-release.sh    # lokaler Release-Build (Docker-Image, Versionierung über VERSION-Datei)
```

## Releases & Updates

Ein Git-Tag der Form `vX.Y.Z` löst den Release-Workflow aus, der fertige Binaries als GitHub-Release-Assets baut (inkl. SHA256-Prüfsummen):

- `gartenamt-X.Y.Z-windows-amd64.exe`
- `gartenamt-X.Y.Z-linux-amd64`
- `gartenamt-X.Y.Z-macos-universal.dmg` (Apple Silicon + Intel)

**Hinweis für macOS:** Das DMG ist nicht mit einem Apple-Developer-Zertifikat signiert, daher blockiert macOS die heruntergeladene App beim ersten Start. Je nach macOS-Version erscheint entweder „Entwickler kann nicht verifiziert werden" — dann über *Systemeinstellungen → Datenschutz & Sicherheit → „Dennoch öffnen"* freigeben — oder die Meldung „ist beschädigt und kann nicht geöffnet werden". Letztere behebt ein Terminal-Befehl, der das Quarantäne-Attribut des Downloads entfernt:

```bash
xattr -cr "/Applications/Gartenamt.app"
```

Beim Start aus dem Finder legt die App ihre Daten (Datenbank, Backups, Exporte) unter `~/Library/Application Support/Gartenamt/` ab.

Die App läuft ohne Dock-Symbol, zeigt aber ein Symbol in der Menüleiste: Darüber lässt sich Gartenamt jederzeit wieder im Browser öffnen und der Server sauber **beenden**. Ein erneutes Öffnen der App startet keinen zweiten Server, sondern öffnet nur wieder den Browser.

**Datenbank-Updates sind automatisch und sicher:** Die App verwaltet ihr SQLite-Schema über versionierte Migrationen (Tabelle `schema_migrations`). Beim ersten Start einer neuen Version werden ausstehende Migrationen einzeln in Transaktionen angewendet — schlägt eine fehl, wird sie zurückgerollt und die App startet nicht mit halbem Schema. Ein Downgrade auf eine ältere Programmversion mit neuerer Datenbank wird erkannt und mit klarer Fehlermeldung abgelehnt. Vor größeren Updates empfiehlt sich trotzdem ein Backup (*Admin → Backup*).

Abhängigkeits-Updates (Go-Module, GitHub Actions, Docker-Basis-Images) werden wöchentlich per Dependabot vorgeschlagen.

## Sicherheit

Die Anwendung ist für den Betrieb im **lokalen Vereinsnetz** gedacht; für den Zugriff über das Internet gehört ein Reverse-Proxy mit echtem Zertifikat davor. Details und der Meldeweg für Schwachstellen stehen in [SECURITY.md](SECURITY.md).

Was fest eingebaut ist:

| Bereich | Umsetzung |
| --- | --- |
| Passwörter | bcrypt mit Kostenfaktor 12, Mindestlänge 10, Sperrliste, kein Benutzername im Passwort |
| Sitzungen | 256-Bit-IDs aus `crypto/rand`, `HttpOnly` · `Secure` · `SameSite=Strict`, 24 h Inaktivität und 7 Tage Höchstdauer |
| Anmeldung | Sperre nach 5 Fehlversuchen je Konto und 50 je IP, gleiche Antwortzeit für bekannte und unbekannte Konten |
| Rollen | Benutzerverwaltung, Backup, Audit-Log und Einstellungen nur für Admin und Vorstand; niemand vergibt eine Rolle über der eigenen |
| HTTP | CSP, `X-Frame-Options`, `nosniff`, `Referrer-Policy`, HSTS im Server-Modus, `no-store` außerhalb von `/static/` |
| Daten | Backups und SMTP-Passwörter mit AES-256-GCM, parametrisierte SQL-Abfragen, automatisches Escaping in allen Templates |
| Betrieb | Container als UID/GID 10001 ohne zusätzliche Kernel-Rechte, Server-Timeouts, Größenlimits für Anfragen |

**Prüfstand:** Der Code wurde von **Claude Opus 5** einem Sicherheitsaudit und anschließend einem Penetrationstest gegen eine laufende Instanz unterzogen (Autorisierungsmatrix über alle Routen und Rollen, Injection, XSS, Pfad-Traversal, CSRF, Sitzungsangriffe, Uploads und Geschäftslogik). 23 der 24 Befunde sind behoben und in v1.1.1 veröffentlicht.

Bewusst nicht geändert: Die Initial-Zugangsdaten des Administrators werden bis zur ersten Passwortänderung auf der Login-Seite angezeigt, damit sie beim Start als App-Bundle ohne sichtbare Konsole auffindbar bleiben. **Diese Änderung gehört deshalb in die Inbetriebnahme, nicht auf die Liste danach** — bis dahin kann jeder mit Netzzugriff das Konto übernehmen.

`govulncheck` und die Testsuite laufen bei jedem Push in der CI.

## Mitwirken

Beiträge sind willkommen — siehe [CONTRIBUTING.md](CONTRIBUTING.md). Sicherheitslücken bitte nicht öffentlich melden, sondern wie in [SECURITY.md](SECURITY.md) beschrieben.

## Lizenz

Dieses Projekt steht unter der [PolyForm Noncommercial License 1.0.0](LICENSE): Der Quellcode ist offen, die Nutzung ist für **nicht-kommerzielle Zwecke** frei — also insbesondere für Kleingartenvereine, Privatpersonen, Bildungseinrichtungen und Behörden. **Kommerzielle Nutzung ist nicht gestattet**, etwa der Verkauf der Software oder das entgeltliche Anbieten als Hosting-Dienst. Für eine kommerzielle Lizenz bitte Kontakt über GitHub aufnehmen.

*This project is source-available under the PolyForm Noncommercial License 1.0.0 — free for noncommercial use, commercial use is not permitted.*
