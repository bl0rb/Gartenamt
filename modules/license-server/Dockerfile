FROM golang:1.21-bullseye AS builder

WORKDIR /build

RUN apt-get update && apt-get install -y --no-install-recommends \
    gcc libc6-dev libsqlite3-dev pkg-config && \
    rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -o license-server .

FROM ubuntu:22.04

RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-0 ca-certificates wget && \
    rm -rf /var/lib/apt/lists/*

COPY --from=builder /build/license-server /usr/bin/license-server
RUN chmod +x /usr/bin/license-server

RUN mkdir -p /data
WORKDIR /data
VOLUME ["/data"]
EXPOSE 8090

CMD ["/usr/bin/license-server"]
