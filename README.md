# Kleingarten-Verwaltung

Verwaltungssoftware für Kleingartenvereine: Parzellen, Pächter, Inspektionen, Wertermittlungen, Wasser-/Stromabrechnung und Rechnungsversand — als einzelne Go-Anwendung mit SQLite, ohne externe Dienste selbst zu hosten.

*Management software for German allotment garden associations (Kleingartenvereine), built with Go and SQLite. The application and its documentation are in German, as its workflows follow German allotment-garden regulations.*

## Funktionen

- **Parzellenverwaltung** — Parzellen mit Pächterdaten anlegen, bearbeiten und verwalten
- **Inspektionen** — Begehungsprotokolle mit Mängelerfassung
- **Wertermittlung** — Wertermittlungsprotokolle mit PDF-Export (angelehnt an die Richtlinie des Landesbundes der Gartenfreunde in Hamburg e.V.)
- **Abrechnung** — Wasser-/Stromzählerstände, Rechnungserstellung als PDF, Sammelexport
- **E-Mail-Versand** — Rechnungen und Infos direkt an Pächter senden (SMTP)
- **Benutzer & Rollen** — Rollenmodell (Admin, Vorstand, Kassenwart, Wertermittler, Benutzer) mit feingranularen Berechtigungen
- **Backup** — verschlüsselte Datenbank-Backups mit Wiederherstellung, CSV-Export/-Import
- **Audit-Log** — Nachverfolgung administrativer Aktionen

Die Weboberfläche läuft komplett lokal (HTTPS mit selbstsigniertem Zertifikat); es werden keine externen Dienste oder CDNs eingebunden.

## Schnellstart (lokal)

Voraussetzung: Go 1.21+ und ein C-Compiler (für SQLite/cgo).

```bash
git clone https://github.com/bl0rb/kleingarten-verwaltung.git
cd kleingarten-verwaltung
go run .
```

Die App startet unter `https://localhost:8080` und öffnet den Browser automatisch (unterdrückbar mit `--no-browser`). Beim ersten Start wird ein Administrator-Konto **admin** mit einem **zufällig generierten Passwort** angelegt — das Passwort wird einmalig auf der Konsole ausgegeben.

> Hinweis: Das TLS-Zertifikat ist selbstsigniert; die Browser-Warnung beim ersten Aufruf ist erwartbar. Zusätzliche Hostnamen für das Zertifikat können über `TLS_EXTRA_HOSTS` konfiguriert werden.

## Schnellstart (Docker)

```bash
docker compose up -d
```

Die Datenbank liegt im Volume `kleingarten-data` (`/data/kleingarten.db`). Details und NAS-Hinweise: [DOCKER_DEPLOYMENT.md](DOCKER_DEPLOYMENT.md).

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

Die Wertermittlung orientiert sich an der Richtlinie zur Wertermittlung des [Landesbundes der Gartenfreunde in Hamburg e.V.](https://www.gartenfreunde-hh.de/) Die offiziellen Richtlinien- und Formulardokumente sind urheberrechtlich geschützt und daher nicht Teil dieses Repositories — sie sind beim Landesbund erhältlich. Für andere Landesverbände lassen sich die Bewertungsgrundlagen (Obstgehölze, Zieranpflanzungen, Bauindex) in der App unter *Admin → Stammdaten* anpassen.

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

- `kleingarten-verwaltung-X.Y.Z-windows-amd64.exe`
- `kleingarten-verwaltung-X.Y.Z-linux-amd64`
- `kleingarten-verwaltung-X.Y.Z-macos-universal.dmg` (Apple Silicon + Intel)

**Hinweis für macOS:** Das DMG ist nicht mit einem Apple-Developer-Zertifikat signiert, daher blockiert macOS die heruntergeladene App beim ersten Start. Je nach macOS-Version erscheint entweder „Entwickler kann nicht verifiziert werden" — dann über *Systemeinstellungen → Datenschutz & Sicherheit → „Dennoch öffnen"* freigeben — oder die Meldung „ist beschädigt und kann nicht geöffnet werden". Letztere behebt ein Terminal-Befehl, der das Quarantäne-Attribut des Downloads entfernt:

```bash
xattr -cr "/Applications/Kleingarten Verwaltung.app"
```

Beim Start aus dem Finder legt die App ihre Daten (Datenbank, Backups, Exporte) unter `~/Library/Application Support/Kleingarten-Verwaltung/` ab.

**Datenbank-Updates sind automatisch und sicher:** Die App verwaltet ihr SQLite-Schema über versionierte Migrationen (Tabelle `schema_migrations`). Beim ersten Start einer neuen Version werden ausstehende Migrationen einzeln in Transaktionen angewendet — schlägt eine fehl, wird sie zurückgerollt und die App startet nicht mit halbem Schema. Ein Downgrade auf eine ältere Programmversion mit neuerer Datenbank wird erkannt und mit klarer Fehlermeldung abgelehnt. Vor größeren Updates empfiehlt sich trotzdem ein Backup (*Admin → Backup*).

Abhängigkeits-Updates (Go-Module, GitHub Actions, Docker-Basis-Images) werden wöchentlich per Dependabot vorgeschlagen.

## Mitwirken

Beiträge sind willkommen — siehe [CONTRIBUTING.md](CONTRIBUTING.md). Sicherheitslücken bitte nicht öffentlich melden, sondern wie in [SECURITY.md](SECURITY.md) beschrieben.

## Lizenz

Dieses Projekt steht unter der [GNU Affero General Public License v3.0](LICENSE) (AGPL-3.0). Wer die Software verändert und als Dienst betreibt, muss den Quellcode der veränderten Version den Nutzern zugänglich machen.
