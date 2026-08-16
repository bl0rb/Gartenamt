//go:build !darwin && !windows

package main

import (
	"log"
	"net/http"
	"os"
)

// runDesktop wartet auf ein Signal und fährt den Server dann sauber herunter
// (Linux-Desktop: kein Tray-Symbol, Verhalten wie bisher).
func runDesktop(server *http.Server, sigChan chan os.Signal) {
	sig := <-sigChan
	log.Printf("\n📛 Signal empfangen (%v), fahre herunter...", sig)
	shutdownServer(server)
}
