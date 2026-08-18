# Docker Deployment Guide für NAS

## Schnellstart (empfohlen: fertiges Image von GHCR)

Das NAS baut nichts selbst — es zieht das fertige Image aus der GitHub Container Registry. Alle Daten liegen in `./nas-data/` neben der Compose-Datei.

```bash
# 1. docker-compose.nas.yml in einen Ordner auf dem NAS kopieren,
#    z.B. /volume1/docker/gartenamt

# 2. Starten
docker compose -f docker-compose.nas.yml up -d

# 3. Logs ansehen (hier steht beim ersten Start das Admin-Passwort!)
docker logs -f gartenamt
```

Danach ist die App unter `https://<nas-ip>:8080` erreichbar (Zertifikatswarnung beim ersten Aufruf ist normal; `TLS_EXTRA_HOSTS` in einer `.env` neben der Compose-Datei vermeidet den Hostname-Mismatch).

**Update auf eine neue Version:**

```bash
docker compose -f docker-compose.nas.yml pull
docker compose -f docker-compose.nas.yml up -d
```

Datenbank-Migrationen laufen beim Start automatisch. Die Compose-Datei pinnt eine feste Version (`KLEINGARTEN_TAG`) statt eines beweglichen `latest`, damit ein Update reproduzierbar bleibt — für eine neue Version den Wert in der `.env` neben der Compose-Datei hochziehen.

### Einmalige Umstellung beim Update auf 1.1.0

Ab 1.1.0 läuft der Container nicht mehr als `root`, sondern unter UID/GID `10001`. Das Datenverzeichnis auf dem NAS gehört bisher `root` und muss deshalb einmalig übertragen werden — sonst startet die App mit einem Schreibfehler auf der Datenbank:

```bash
docker compose -f docker-compose.nas.yml down
sudo chown -R 10001:10001 ./nas-data
docker compose -f docker-compose.nas.yml pull
docker compose -f docker-compose.nas.yml up -d
```

Das TLS-Zertifikat liegt jetzt unter `/home/gartenamt/.gartenamt` statt `/root/.gartenamt`; die aktualisierte Compose-Datei mountet den Pfad bereits richtig. Beim ersten Start wird das Zertifikat einmalig neu erzeugt — im Browser erscheint dadurch einmal wieder die bekannte Warnung.

## Offline-Variante (NAS ohne Internetzugang)

Auf einem Rechner mit Docker das Image bauen und als Archiv transferieren:

```bash
# Build mit Version aus der VERSION-Datei (oder ./build-release.sh 0.2.0)
./build-release.sh

# Das erzeugte binary/gartenamt-docker-<version>.tar.gz aufs NAS
# übertragen und dort importieren:
docker load -i gartenamt-docker-<version>.tar.gz
docker run -d --name gartenamt -p 8080:8080 -v gartenamt-data:/data \
  -e DB_PATH=/data/kleingarten.db gartenamt:<version>
```

Hinweis: Beim erneuten Import desselben Versions-Tags zeigen NAS-Tools ggf. einen Konflikt — vor jedem Release die VERSION erhöhen.

## Troubleshooting

### Docker exits with code 1

**Check logs first:**
```bash
docker logs gartenamt
## or with more detail:
docker logs --follow --timestamps gartenamt
```

**Common issues:**

1. **Database permission error**
   - Der Container läuft als UID/GID `10001` und braucht Schreibrechte auf `/data`
   - Nach einem Update von einer Version vor 1.1.0: `sudo chown -R 10001:10001 ./nas-data`
   
2. **Port already in use**
   - Change port in compose: `8080:8080` → `8000:8080`
   
3. **Missing dependencies**
   - Container needs `sqlite-libs` (included in Alpine image)

### Check container status

```bash
# Is the container running?
docker ps

# See all containers (including stopped)
docker ps -a

# Inspect the container
docker inspect gartenamt

# View resource usage
docker stats gartenamt
```

### Rebuild locally on NAS

```bash
# Clone or pull latest
git clone https://github.com/bl0rb/Gartenamt.git
cd Gartenamt

# Build for your NAS architecture
docker build -t gartenamt:latest .

# Run with debug logging
docker run -it \
  -p 8080:8080 \
  -v /mnt/data/gartenamt:/data \
  -e DEBUG=1 \
  gartenamt
```

### Manual test without compose

```bash
# Build
docker build -t gartenamt:test .

# Run with interactive logging
docker run -it \
  --rm \
  -p 8080:8080 \
  -v kleingarten-test:/data \
  gartenamt

# View database
docker exec gartenamt ls -la /data/
```

## Data Persistence

- Database is stored in `/data/kleingarten.db`
- Mounted as a Docker volume: `gartenamt-data:/data`
- Persists even if container is stopped/removed

## Environment Variables

- `DB_PATH` - Database file path (default: `/data/kleingarten.db`)
- `--no-browser` - Don't try to open browser (used automatically in Docker)

## Performance

For NAS, consider resource limits in docker-compose:
```yaml
deploy:
  resources:
    limits:
      cpus: '1'
      memory: 512M
```
