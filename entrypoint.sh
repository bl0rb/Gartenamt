#!/bin/sh
set -e

# Entrypoint script with logging
echo "=================================="
echo "🚀 Kleingarten-Verwaltung Docker"
echo "=================================="
echo ""
echo "📍 Umgebung:"
echo "   Platform: $(uname -m)"
echo "   OS: $(uname -s)"
echo "   Go Version: $(go version 2>/dev/null || echo 'N/A')"
echo ""

echo "📂 Datenverzeichnis:"
ls -la /data/ || echo "   /data nicht vorhanden (wird erstellt)"
echo ""

echo "🔧 Starte Anwendung..."
echo "   Datenbank: /data/kleingarten.db"
echo "   Port: 8080"
echo ""

# Ensure /data directory is writable
mkdir -p /data
chmod 755 /data

# Also try to ensure permissions are correct
touch /data/.test-write 2>/dev/null && rm /data/.test-write || {
    echo "⚠️  WARNUNG: /data ist möglicherweise nicht beschreibbar!"
    echo "   Bitte überprüfen Sie die Docker-Volume-Berechtigungen."
}

# Run the app with no browser (since we're in a container)
echo "=================================="
echo "✅ Anwendung läuft..."
echo "=================================="
echo ""

exec /app/kleingarten-verwaltung --no-browser
