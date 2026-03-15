#!/bin/sh
# Entrypoint script for Kleingarten-Verwaltung Docker container

echo "=================================="
echo "🚀 Kleingarten-Verwaltung - Docker"
echo "=================================="
echo ""
echo "📍 System Info:"
echo "   Platform: $(uname -m)"
echo "   OS: $(uname -s)"
echo ""

echo "📂 Data Directory:"
echo "   Location: /data"

if [ -d /data ]; then
    echo "   Status: EXISTS"
    ls -la /data/ 2>/dev/null | head -3 || true
else
    echo "   Status: CREATING"
    mkdir -p /data
fi

echo "   Checking write permissions..."
if touch /data/.write-test 2>/dev/null; then
    rm -f /data/.write-test
    echo "   Permissions: ✅ OK"
else
    echo "   Permissions: ⚠️  FAILED - May cause database issues"
fi

echo ""
echo "🔧 Application Configuration:"
echo "   Database: /data/kleingarten.db"
echo "   Port: 8080 (HTTPS)"
echo "   Mode: --no-browser"
echo ""

echo "=================================="
echo "✅ Starting application..."
echo "=================================="
echo ""

# Change to /data so database is created there
cd /data

# Run the binary (no --no-browser, let main.go detect we're in a container)
exec /app/kleingarten-verwaltung --no-browser
