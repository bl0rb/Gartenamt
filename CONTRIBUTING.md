# Mitwirken an der Kleingarten-Verwaltung

Danke für dein Interesse! Beiträge in Form von Bug-Reports, Verbesserungsvorschlägen und Pull Requests sind willkommen.

## Bug melden / Feature vorschlagen

- Bitte zuerst prüfen, ob es bereits ein passendes Issue gibt.
- Für Bugs: Schritte zur Reproduktion, erwartetes und tatsächliches Verhalten, ggf. Log-Ausgabe angeben.
- Sicherheitslücken **nicht** als öffentliches Issue melden — siehe [SECURITY.md](SECURITY.md).

## Entwicklungsumgebung

Voraussetzungen: Go 1.21+ und ein C-Compiler (SQLite via cgo).

```bash
git clone https://github.com/bl0rb/kleingarten-verwaltung.git
cd kleingarten-verwaltung
go run .
```

Vor jedem Pull Request:

```bash
go build ./...
go vet ./...
```

## Pull Requests

- Kleine, fokussierte PRs sind leichter zu prüfen als große Sammel-PRs.
- Beschreibe im PR, *was* geändert wurde und *warum*.
- Neue Texte in der Oberfläche bitte auf Deutsch (Zielgruppe sind deutsche Kleingartenvereine); Code-Bezeichner und Commit-Messages können deutsch oder englisch sein — bleib konsistent mit dem umliegenden Code.
- Keine externen CDN-Einbindungen: alle Frontend-Assets werden lokal aus `static/` ausgeliefert (Datenschutz).

## Lizenz deiner Beiträge

Mit dem Einreichen eines Pull Requests erklärst du dich einverstanden, dass dein Beitrag unter der [AGPL-3.0](LICENSE) veröffentlicht wird.
