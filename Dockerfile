# Multi-stage build for kleingarten-verwaltung
# Stage 1: Builder
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install required build dependencies for sqlite3 CGO
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -o kleingarten-verwaltung .

# Stage 2: Runtime
FROM alpine:latest

# Install runtime dependencies
RUN apk add --no-cache ca-certificates sqlite-libs

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/kleingarten-verwaltung .

# Create data directory for sqlite database
RUN mkdir -p /data

# Expose port
EXPOSE 8080

# Volume mount point for persistent data (database)
VOLUME ["/data"]

# Set environment for database location
ENV DB_PATH=/data/kleingarten.db

# Run the application
CMD ["./kleingarten-verwaltung"]
