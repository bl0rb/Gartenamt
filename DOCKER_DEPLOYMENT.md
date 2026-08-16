# Docker Deployment Guide für NAS

## Schnellstart (empfohlen: fertiges Image von GHCR)

Das NAS baut nichts selbst — es zieht das fertige Image aus der GitHub Container Registry. Alle Daten liegen in `./nas-data/` neben der Compose-Datei.

```bash
# 1. docker-compose.nas.yml in einen Ordner auf dem NAS kopieren,
#    z.B. /volume1/docker/kleingarten

# 2. Starten
docker compose -f docker-compose.nas.yml up -d

# 3. Logs ansehen (hier steht beim ersten Start das Admin-Passwort!)
docker logs -f kleingarten-verwaltung
```

Danach ist die App unter `https://<nas-ip>:8080` erreichbar (Zertifikatswarnung beim ersten Aufruf ist normal; `TLS_EXTRA_HOSTS` in einer `.env` neben der Compose-Datei vermeidet den Hostname-Mismatch).

**Update auf eine neue Version:**

```bash
docker compose -f docker-compose.nas.yml pull
docker compose -f docker-compose.nas.yml up -d
```

Datenbank-Migrationen laufen beim Start automatisch. Eine feste Version statt `latest` pinnt `KLEINGARTEN_TAG=0.2.0` in der `.env`.

## Offline-Variante (NAS ohne Internetzugang)

Auf einem Rechner mit Docker das Image bauen und als Archiv transferieren:

```bash
# Build mit Version aus der VERSION-Datei (oder ./build-release.sh 0.2.0)
./build-release.sh

# Das erzeugte binary/kleingarten-verwaltung-docker-<version>.tar.gz aufs NAS
# übertragen und dort importieren:
docker load -i kleingarten-verwaltung-docker-<version>.tar.gz
docker run -d --name kleingarten -p 8080:8080 -v kleingarten-data:/data \
  -e DB_PATH=/data/kleingarten.db kleingarten-verwaltung:<version>
```

Hinweis: Beim erneuten Import desselben Versions-Tags zeigen NAS-Tools ggf. einen Konflikt — vor jedem Release die VERSION erhöhen.

## Troubleshooting

### Docker exits with code 1

**Check logs first:**
```bash
docker logs kleingarten-verwaltung
## or with more detail:
docker logs --follow --timestamps kleingarten-verwaltung
```

**Common issues:**

1. **Database permission error**
   - Make sure `/data` volume has proper permissions
   - The container user needs write access
   
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
docker inspect kleingarten-verwaltung

# View resource usage
docker stats kleingarten-verwaltung
```

### Rebuild locally on NAS

```bash
# Clone or pull latest
git clone <repo>
cd kleingarten-verwaltung
git checkout feature/docker-nas

# Build for your NAS architecture
docker build -t kleingarten-verwaltung:0.1.1 -t kleingarten-verwaltung:latest .

# Run with debug logging
docker run -it \
  -p 8080:8080 \
  -v /mnt/data/kleingarten:/data \
  -e DEBUG=1 \
  kleingarten-verwaltung
```

### Manual test without compose

```bash
# Build
docker build -t kleingarten-verwaltung:test .

# Run with interactive logging
docker run -it \
  --rm \
  -p 8080:8080 \
  -v kleingarten-test:/data \
  kleingarten-verwaltung

# View database
docker exec kleingarten-verwaltung ls -la /data/
```

## Data Persistence

- Database is stored in `/data/kleingarten.db`
- Mounted as a Docker volume: `kleingarten-data:/data`
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
