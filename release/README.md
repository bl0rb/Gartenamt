# Kleingarten Verwaltung - Release Binaries

This directory contains pre-compiled, ready-to-use GUI applications for **Kleingarten Verwaltung** (Community Garden Management System) across multiple platforms.

## 📦 Available Releases

### macOS
- **kleingarten-verwaltung-macos-intel.zip** - For Intel/x86-64 Macs (Intel processors)
- **kleingarten-verwaltung-macos-arm64.zip** - For Apple Silicon Macs (M1, M2, M3, etc.)

**Installation:**
1. Download the appropriate .zip file for your Mac
2. Double-click to extract the .app bundle
3. Double-click **kleingarten-verwaltung.app** to launch
4. Your default browser will automatically open to `http://localhost:8080`

> **Note:** macOS may show a security warning. Click "Open" or right-click and select "Open" to proceed.

### Windows
- **app.exe** - Standalone Windows executable (64-bit)
- **kleingarten-verwaltung.bat** - Quick launcher batch file

**Installation:**
1. Download **app.exe** (or use the .bat launcher)
2. Double-click to launch
3. Your default browser will automatically open to `http://localhost:8080`

> **Note:** Windows may show a Windows Defender SmartScreen warning. Click "More info" → "Run anyway" to proceed.

## 🌐 Web Interface

Once launched, the application opens automatically in your default browser at:
- **Main App:** http://localhost:8080
- **Login:** http://localhost:8080/login
- **Admin Panel:** http://localhost:8080/admin

### Default Admin Credentials
See the console output when the app starts for the default admin login credentials.

## ⚙️ Starting the App

### From GUI
- **macOS:** Double-click the .app bundle
- **Windows:** Double-click app.exe or kleingarten-verwaltung.bat

### From Terminal
```bash
# macOS
./kleingarten-verwaltung-macos-intel.app/Contents/MacOS/kleingarten-verwaltung

# macOS (disable auto-browser)
./kleingarten-verwaltung-macos-intel.app/Contents/MacOS/kleingarten-verwaltung --no-browser

# Windows
app.exe

# Windows (disable auto-browser)
app.exe --no-browser
```

## 🔐 Database

The application creates and manages a SQLite database file `kleingarten.db` in its working directory:
- **macOS:** In the directory where you launched the .app
- **Windows:** In the same directory as app.exe

**Data Persistence:**
- All user data, plots (Parzellen), inspections (Inspektionen), and valuations (Wertermittlungen) are saved in the database
- The database file should be backed up regularly using the Admin → Backup function

## 🔍 Verification

To verify the integrity of downloaded files, check the **CHECKSUMS.txt** file:
```bash
shasum -a 256 -c CHECKSUMS.txt
```

## 📋 Requirements

- **macOS:** 10.12 or later (Intel or Apple Silicon)
- **Windows:** Windows 7 or later (64-bit)
- **Browser:** Any modern browser for the web interface

## 🐛 Troubleshooting

### Browser doesn't open automatically
The server is still running. Manually open `http://localhost:8080` in your browser.

### Can't connect to the application
- Ensure port 8080 is not in use by another application
- Check firewall settings
- Try accessing `http://127.0.0.1:8080`

### macOS "can't be opened because it is from an unidentified developer"
1. Open System Preferences → Security & Privacy
2. Click "Open Anyway" next to the app
3. Alternatively, right-click the .app → Open

### Windows "This app can't run on your PC"
Ensure you're running 64-bit Windows. Download the correct version.

## 📱 Features

- 👥 Multi-user management with role-based access
- 🍎 Garden plot inventory tracking (Obstarten, Gemüse, Zieranpflanzungen)
- 📊 Property valuation system (Wertermittlung)
- 📋 Inspection protocols (Inspektionen)
- 👨‍💼 Admin panel for system configuration
- 🔐 Secure session-based authentication
- 💾 Automatic database backups

## 📞 Support

For issues or questions, please refer to the main project repository or contact the project maintainer.

---
**Version:** See git tag or commit hash  
**Last Updated:** March 2026
