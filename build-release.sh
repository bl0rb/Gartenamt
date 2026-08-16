#!/bin/bash

# Lokales Build-Skript: natives Binary + Docker-Image-Export (z.B. für NAS).
#
# Plattform-Releases (Windows-exe, Linux-Binary, macOS-DMG) baut der
# GitHub-Actions-Workflow .github/workflows/release.yml bei einem Git-Tag
# vX.Y.Z - Cross-Kompilieren mit CGO/SQLite funktioniert lokal nicht
# zuverlässig, daher baut dieses Skript nur noch für die eigene Plattform.

set -e

PROJECT_NAME="kleingarten-verwaltung"
VERSION_FILE="VERSION"
if [ -n "$1" ]; then
    VERSION="$1"
elif [ -f "$VERSION_FILE" ]; then
    VERSION=$(tr -d '[:space:]' < "$VERSION_FILE")
else
    VERSION="0.1.2"
    echo "$VERSION" > "$VERSION_FILE"
fi

if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z]+)*$ ]]; then
    echo "❌ Invalid version format: $VERSION"
    echo "   Expected format like: 0.1.1, 0.2.0, 1.0.0-rc1"
    exit 1
fi

# Keep VERSION file aligned when version was passed as argument.
echo "$VERSION" > "$VERSION_FILE"

BINARY_DIR="binary"
LDFLAGS="-s -w -X kleingarten-verwaltung/handlers.AppVersion=${VERSION}"

echo "🔨 Building $PROJECT_NAME v$VERSION"
mkdir -p "$BINARY_DIR"

NATIVE_SUFFIX="$(go env GOOS)-$(go env GOARCH)"
echo "📦 Building native binary ($NATIVE_SUFFIX)..."
CGO_ENABLED=1 go build -ldflags "$LDFLAGS" -o "$BINARY_DIR/$PROJECT_NAME-$NATIVE_SUFFIX" .

# Build Docker image for amd64
echo "🐳 Building Docker image (linux/amd64)..."
DOCKER_BUILD_EXIT=0
docker build --platform linux/amd64 -t "$PROJECT_NAME:$VERSION" -t "$PROJECT_NAME:latest" . > /tmp/docker_build.log 2>&1
DOCKER_BUILD_EXIT=$?

if [ $DOCKER_BUILD_EXIT -eq 0 ]; then
    echo "✅ Docker image built: $PROJECT_NAME:$VERSION"

    # Save Docker image for NAS import
    echo "💾 Exporting Docker image to tar.gz..."
    DOCKER_EXPORT_FILE="$BINARY_DIR/$PROJECT_NAME-docker-$VERSION.tar.gz"
    docker save "$PROJECT_NAME:latest" | gzip > "$DOCKER_EXPORT_FILE" 2>/dev/null
    DOCKER_EXPORT_EXIT=$?

    if [ $DOCKER_EXPORT_EXIT -eq 0 ] && [ -f "$DOCKER_EXPORT_FILE" ]; then
        DOCKER_SIZE=$(ls -lh "$DOCKER_EXPORT_FILE" | awk '{print $5}')
        echo "✅ Docker image exported: $PROJECT_NAME-docker-$VERSION.tar.gz ($DOCKER_SIZE)"
    else
        echo "⚠️  Failed to export Docker image (exit code: $DOCKER_EXPORT_EXIT)"
    fi
else
    echo "⚠️  Failed to build Docker image (exit code: $DOCKER_BUILD_EXIT)"
    tail -20 /tmp/docker_build.log
fi

# Create checksums
echo "📦 Creating checksums..."
cd "$BINARY_DIR"
shasum -a 256 "$PROJECT_NAME-$NATIVE_SUFFIX" *.tar.gz 2>/dev/null > CHECKSUMS.txt || true
cd ..

echo ""
echo "🎉 Build complete!"
echo ""
echo "📦 Artifacts in: $BINARY_DIR/"
ls -lh "$BINARY_DIR/"
echo ""
echo "Usage:"
echo "  Native binary:  ./$BINARY_DIR/$PROJECT_NAME-$NATIVE_SUFFIX"
echo "  Docker (NAS):   1. Transfer $PROJECT_NAME-docker-$VERSION.tar.gz to NAS"
echo "                  2. Load via NAS Docker GUI: docker load < $PROJECT_NAME-docker-$VERSION.tar.gz"
echo "                  3. Create container with volume mount /data"
echo "  Plattform-Releases (exe/DMG/Linux): git tag v$VERSION && git push --tags"
