package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.zabbix.com/sdk/plugin/container"
)

func main() {
	// First check for Plugin Mode (socket path passed as first arg)
	// This must be fast and side-effect free.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		runPlugin()
		return
	}

	// Manual Mode
	runManual()
}

func runPlugin() {
	// Initialize logging for plugin mode.
	// We default to stderr (which Zabbix captures).
	// Only use file logging if explicitly requested via env var.
	setupPluginLogging()

	// Handle socket cleanup if necessary
	socket := os.Args[1]
	cleanupSocket(socket)

	// Create and execute the handler
	h, err := container.NewHandler(impl.Name())
	if err != nil {
		log.Printf("Failed to create plugin handler: %s", err)
		os.Exit(1)
	}

	// Set the logger for the implementation
	impl.Logger = h

	// Execute the handler. This blocks.
	if err = h.Execute(); err != nil {
		log.Printf("Failed to execute plugin handler: %s", err)
		os.Exit(1)
	}
}

func runManual() {
	// Define flags here to avoid polluting global scope
	var (
		manualURL = flag.String("manual", "", "Execute manually with the given URL")
		authType  = flag.String("auth", "none", "Authentication type (none, basic, bearer)")
		username  = flag.String("user", "", "Username or token")
		password  = flag.String("pass", "", "Password")
	)

	flag.Parse()

	if *manualURL == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Logging for manual mode: simply stderr
	log.SetOutput(os.Stderr)

	// Disable logger interface for manual mode so it falls back to log.Printf
	impl.Logger = nil

	log.Printf("Running in manual mode. URL: %s", *manualURL)

	params := []string{*manualURL}
	if *authType != "none" {
		params = append(params, *authType, *username, *password)
	}

	// Configure default timeouts for manual mode
	impl.config.Timeout = 10
	impl.config.SkipVerify = true

	res, err := impl.Export("segi9.http", params, nil)
	if err != nil {
		log.Printf("Export failed: %v", err)
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(res.(string))
}

func setupPluginLogging() {
	log.SetOutput(os.Stderr)

	logPath := os.Getenv("SEGI9_LOG_FILE")
	if logPath == "" {
		return
	}

	// Try to open the log file. If it fails or blocks, we fallback to stderr.
	// We do this synchronously but with a quick check if possible?
	// Standard os.OpenFile is blocking. We accept this risk but log errors.
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open log file %s: %v. Logging to stderr.", logPath, err)
		return
	}

	log.SetOutput(f)
	// We rely on OS to close the file on exit
}

func cleanupSocket(socket string) {
	// Attempt to remove the socket file if it exists.
	// This prevents "address already in use" errors.
	info, err := os.Stat(socket)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Warning: Failed to stat socket %s: %v", socket, err)
		}
		return
	}

	if !info.IsDir() {
		if err := os.Remove(socket); err != nil {
			log.Printf("Warning: Failed to remove stale socket %s: %v", socket, err)
		}
	}
}
