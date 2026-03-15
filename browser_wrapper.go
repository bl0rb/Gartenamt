package main

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"time"
)

// LaunchWebApp starts the HTTPS server and opens it in the system browser
func LaunchWebApp(addr string) {
	// Wait for server to be ready
	if !CheckServerWithTLS("8080", 5*time.Second) {
		log.Println("⚠️  Server did not start in time")
		return
	}

	log.Println("✅ Server ready, opening system browser...")
	openSystemBrowser("https://localhost:8080")
}

// CheckServerWithTLS checks if the HTTPS server is ready
func CheckServerWithTLS(port string, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		// Try to make a request to the server
		client := &http.Client{
			Timeout: 500 * time.Millisecond,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true, // Accept self-signed certs
				},
			},
		}
		resp, err := client.Get("https://localhost:" + port + "/login")
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// WaitForServerReady waits for TCP connection on the port
func WaitForServerReady(port string, timeout time.Duration) bool {
	start := time.Now()
	for time.Since(start) < timeout {
		conn, err := net.DialTimeout("tcp", "localhost:"+port, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}
