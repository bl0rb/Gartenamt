# Build stage
FROM golang:1.21-bullseye AS builder

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

RUN mkdir -p /data && chmod 777 /data

WORKDIR /data
EXPOSE 8080
VOLUME ["/data"]

CMD ["/usr/bin/gartenamt", "--no-browser"]
