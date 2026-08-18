# Build stage
FROM golang:1.26-bookworm AS builder

WORKDIR /build

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc libc6-dev libsqlite3-dev pkg-config && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

ARG TARGETARCH=amd64
COPY . .
RUN CGO_ENABLED=1 go build -a -o gartenamt .
RUN ls -lh /build/gartenamt && echo "✅ Binary ready"

# Runtime stage
FROM ubuntu:26.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-0 ca-certificates curl && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/gartenamt /usr/bin/gartenamt
RUN chmod +x /usr/bin/gartenamt && \
    ls -lh /usr/bin/gartenamt && \
    echo "✅ Binary installed"

# Unprivilegierter Benutzer mit fester UID/GID: /data enthält Datenbank,
# Backups, Exporte und den Backup-Schlüssel .app_secret - das braucht weder
# root noch die bisherigen 0777.
# Beim Update einer bestehenden Installation muss das Datenverzeichnis auf dem
# Host einmalig übertragen werden: chown -R 10001:10001 ./nas-data
RUN groupadd --gid 10001 gartenamt && \
    useradd --uid 10001 --gid 10001 --home-dir /home/gartenamt --create-home gartenamt && \
    mkdir -p /data && \
    chown -R gartenamt:gartenamt /data /home/gartenamt && \
    chmod 750 /data

# HOME steuert, wo das TLS-Zertifikat abgelegt wird (~/.gartenamt/certs).
ENV HOME=/home/gartenamt

USER gartenamt

WORKDIR /data
EXPOSE 8080
VOLUME ["/data"]

CMD ["/usr/bin/gartenamt", "--no-browser"]
