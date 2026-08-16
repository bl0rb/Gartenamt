//go:build darwin || windows

package main

import (
	_ "embed"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"

	"fyne.io/systray"
)

//go:embed static/favicon-32.png
var trayIconPNG []byte

//go:embed packaging/windows/icon.ico
var trayIconICO []byte

// runDesktop zeigt das Menüleisten- (macOS) bzw. Tray-Symbol (Windows) und
// blockiert, bis der Benutzer "Beenden" wählt oder ein Signal eintrifft.
// Der Server wird dabei sauber heruntergefahren - direkt im Quit-Pfad, nicht
// erst im onExit-Callback von systray (der läuft nicht zuverlässig zu Ende).
// Muss auf der Hauptgoroutine laufen (Cocoa-Event-Loop).
func runDesktop(server *http.Server, sigChan chan os.Signal) {
	var once sync.Once
	stop := func() { once.Do(func() { shutdownServer(server) }) }
	systray.Run(func() { trayReady(sigChan, stop) }, stop)
}

func trayReady(sigChan chan os.Signal, stop func()) {
	if runtime.GOOS == "windows" {
		systray.SetIcon(trayIconICO)
	} else {
		// Template-Icon: macOS färbt es passend zu hellem/dunklem Menübalken ein
		systray.SetTemplateIcon(trayIconPNG, trayIconPNG)
	}
	systray.SetTooltip("Gartenamt läuft - " + desktopURL)

	mOpen := systray.AddMenuItem("Gartenamt öffnen", "Öffnet Gartenamt im Browser")
	systray.AddSeparator()
	mQuit := systray.AddMenuItem("Beenden", "Beendet den Gartenamt-Server")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				openBrowserApp(desktopURL)
			case <-mQuit.ClickedCh:
				log.Println("📛 Beenden über das Menü, fahre herunter...")
				stop()
				systray.Quit()
				return
			case sig := <-sigChan:
				log.Printf("\n📛 Signal empfangen (%v), fahre herunter...", sig)
				stop()
				systray.Quit()
				return
			}
		}
	}()
}
