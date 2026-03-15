# Build stage
FROM golang:1.21-bullseye AS builder

WORKDIR /build

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc libc6-dev libsqlite3-dev pkg-config && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -a -o kleingarten-verwaltung .
RUN ls -lh /build/kleingarten-verwaltung && echo "✅ Binary ready"

# Runtime stage
FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-0 ca-certificates && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/kleingarten-verwaltung /usr/bin/kleingarten-verwaltung
RUN chmod +x /usr/bin/kleingarten-verwaltung && \
    ls -lh /usr/bin/kleingarten-verwaltung && \
    echo "✅ Binary installed"

RUN mkdir -p /data && chmod 777 /data

WORKDIR /data
EXPOSE 8080
VOLUME ["/data"]

CMD ["/usr/bin/kleingarten-verwaltung", "--no-browser"]
