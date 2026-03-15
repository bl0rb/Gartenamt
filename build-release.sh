#!/bin/bash

# Build script for cross-platform releases
# Creates native app bundles for macOS and Windows executables

set -e

PROJECT_NAME="kleingarten-verwaltung"
VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
BINARY_DIR="binary"

echo "🔨 Building $PROJECT_NAME v$VERSION"

# Create binary directory
mkdir -p "$BINARY_DIR"

# Build binaries
echo "📦 Building Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o "$BINARY_DIR/app-linux" main.go

echo "📦 Building Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -o "$BINARY_DIR/app.exe" main.go

echo "📦 Building macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -o "$BINARY_DIR/app-macos-intel" main.go

echo "📦 Building macOS (arm64)..."
GOOS=darwin GOARCH=arm64 go build -o "$BINARY_DIR/app-macos-arm64" main.go

# Create macOS .app bundle for Intel
echo "📦 Creating macOS Intel app bundle..."
MACOS_APP_INTEL="$BINARY_DIR/$PROJECT_NAME-macos-intel.app"
mkdir -p "$MACOS_APP_INTEL/Contents/MacOS"
mkdir -p "$MACOS_APP_INTEL/Contents/Resources"
cp "$BINARY_DIR/app-macos-intel" "$MACOS_APP_INTEL/Contents/MacOS/$PROJECT_NAME"
chmod +x "$MACOS_APP_INTEL/Contents/MacOS/$PROJECT_NAME"

# Create Info.plist for macOS Intel
cat > "$MACOS_APP_INTEL/Contents/Info.plist" << 'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleDevelopmentRegion</key>
    <string>de_DE</string>
    <key>CFBundleExecutable</key>
    <string>kleingarten-verwaltung</string>
    <key>CFBundleIdentifier</key>
    <string>de.bl0rb.kleingarten-verwaltung</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundleName</key>
    <string>Kleingarten Verwaltung</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>NSHumanReadableCopyright</key>
    <string>© 2026 Community Garden Management</string>
    <key>NSRequiresIPhoneOS</key>
    <false/>
    <key>NSHighResolutionCapable</key>
    <true/>
</dict>
</plist>
EOF

# Create macOS .app bundle for ARM
echo "📦 Creating macOS ARM64 app bundle..."
MACOS_APP_ARM="$BINARY_DIR/$PROJECT_NAME-macos-arm64.app"
mkdir -p "$MACOS_APP_ARM/Contents/MacOS"
mkdir -p "$MACOS_APP_ARM/Contents/Resources"
cp "$BINARY_DIR/app-macos-arm64" "$MACOS_APP_ARM/Contents/MacOS/$PROJECT_NAME"
chmod +x "$MACOS_APP_ARM/Contents/MacOS/$PROJECT_NAME"

# Create Info.plist for macOS ARM
cp "$MACOS_APP_INTEL/Contents/Info.plist" "$MACOS_APP_ARM/Contents/Info.plist"

# Copy assets to both macOS bundles
for APP_BUNDLE in "$MACOS_APP_INTEL" "$MACOS_APP_ARM"; do
    if [ -d "static" ]; then
        cp -r static "$APP_BUNDLE/Contents/Resources/"
    fi
    if [ -d "templates" ]; then
        cp -r templates "$APP_BUNDLE/Contents/Resources/"
    fi
done

# Copy Windows exe to binary folder
cp "$BINARY_DIR/app.exe" "$BINARY_DIR/kleingarten-verwaltung.exe"

# Create Windows launcher
echo "📦 Creating Windows launcher..."
cat > "$BINARY_DIR/kleingarten-verwaltung.bat" << 'EOF'
@echo off
cd /d "%~dp0"
start "" "kleingarten-verwaltung.exe"
EOF

# Create archives
echo "📦 Creating archives..."
cd "$BINARY_DIR"

# macOS Intel
ditto -c -k --sequesterRsrc "$PROJECT_NAME-macos-intel.app" "$PROJECT_NAME-macos-intel.zip"
echo "✅ Created: $PROJECT_NAME-macos-intel.zip"

# macOS ARM
ditto -c -k --sequesterRsrc "$PROJECT_NAME-macos-arm64.app" "$PROJECT_NAME-macos-arm64.zip"
echo "✅ Created: $PROJECT_NAME-macos-arm64.zip"

cd ..

# Create checksums
echo "📦 Creating checksum..."
cd "$BINARY_DIR"
shasum -a 256 *.zip *.exe *.bat > CHECKSUMS.txt 2>/dev/null || true
cd ..

echo ""
echo "🎉 Build complete!"
echo ""
echo "📦 Release artifacts in: $BINARY_DIR/"
ls -lh "$BINARY_DIR/"
echo ""
echo "Usage:"
echo "  macOS Intel: Double-click or run: open $BINARY_DIR/$PROJECT_NAME-macos-intel.app"
echo "  macOS ARM:   Double-click or run: open $BINARY_DIR/$PROJECT_NAME-macos-arm64.app"
echo "  Windows:     Double-click $BINARY_DIR/kleingarten-verwaltung.exe or .bat"
