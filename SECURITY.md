# Sicherheitsrichtlinie / Security Policy

## Sicherheitslücke melden

Bitte melde Sicherheitslücken **nicht** über öffentliche GitHub-Issues.

Nutze stattdessen [GitHub Security Advisories](https://github.com/bl0rb/kleingarten-verwaltung/security/advisories/new) („Report a vulnerability"), damit die Lücke vertraulich gemeldet und behoben werden kann, bevor sie öffentlich wird.

Bitte gib an:

- Betroffene Version bzw. Commit
- Schritte zur Reproduktion
- Mögliche Auswirkungen

## Unterstützte Versionen

Sicherheitskorrekturen erfolgen auf dem `main`-Branch. Es werden keine älteren Release-Stände gepflegt — bitte immer die aktuelle Version einsetzen.

## Hinweise zum Betrieb

- Die App ist für den Betrieb im **lokalen Vereinsnetz** gedacht. Wer sie über das Internet erreichbar macht, sollte einen Reverse-Proxy mit echtem TLS-Zertifikat vorschalten und `TRUSTED_PROXY_IPS` konfigurieren.
- Das beim ersten Start generierte Admin-Passwort nach dem ersten Login ändern.
- Regelmäßige verschlüsselte Backups anlegen (Admin → Backup) und den Backup-Schlüssel sicher verwahren.
