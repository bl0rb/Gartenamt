# Docker Deployment Guide for NAS

## Quick Start (NAS)

```bash
# Load the saved image (if transferring from Mac)
docker load -i kleingarten-verwaltung-amd64.tar

# Run with compose (recommended)
docker compose up -d

# View logs
docker logs -f kleingarten-verwaltung
```

## Image Versioning (important)

To avoid NAS image update conflicts with identical name+version:

1. Increase VERSION before each release (for example 0.1.1 -> 0.1.2).
2. Build and export again via ./build-release.sh.
3. Import the new tar.gz and deploy the matching tag.

Examples:

```bash
# Build with version from VERSION file
./build-release.sh

# Or override explicitly
./build-release.sh 0.2.0

# Run specific image tag
docker run -d --name kleingarten -p 8080:8080 -v kleingarten-data:/data kleingarten-verwaltung:0.2.0
```

If you try to import the same version tag again, NAS tools may show an image conflict or update error.

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
