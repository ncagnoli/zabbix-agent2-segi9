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
	// Configure logging immediately to catch startup errors
	configureLogging()

	// Check if running as a plugin (arguments present and first argument is not a flag)
	// Zabbix Agent 2 passes the socket path as the first argument.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		runPluginMode()
		return
	}

	// Manual mode configuration
	var (
		manualURL = flag.String("manual", "", "Execute manually with the given URL")
		authType  = flag.String("auth", "none", "Authentication type (none, basic, bearer)")
		username  = flag.String("user", "", "Username or token")
		password  = flag.String("pass", "", "Password")
	)
	flag.Parse()

	if *manualURL != "" {
		runManualMode(*manualURL, *authType, *username, *password)
	} else {
		flag.Usage()
		os.Exit(1)
	}
}

func configureLogging() {
	// Default log output is stderr.
	log.SetOutput(os.Stderr)

	if logPath := os.Getenv("SEGI9_LOG_FILE"); logPath != "" {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open log file %s: %v. Logging to stderr.", logPath, err)
		} else {
			// We don't close f here; rely on OS to close on exit
			log.SetOutput(f)
			log.Printf("Logging initialized to file: %s", logPath)
		}
	}
}

func runPluginMode() {
	log.Printf("Starting Segi9 plugin. Args: %v", os.Args)

	// Ensure cleanup of the socket file on exit
	if len(os.Args) > 1 {
		socket := os.Args[1]

		// Try to remove socket in case it was left over from a crash
		// This is critical because net.Listen("unix", ...) fails if the file exists.
		if info, err := os.Stat(socket); err == nil {
			if !info.IsDir() {
				log.Printf("Removing existing socket file: %s", socket)
				if err := os.Remove(socket); err != nil {
					log.Printf("Warning: Failed to remove stale socket %s: %v", socket, err)
				}
			} else {
				log.Printf("Warning: Socket path %s is a directory!", socket)
			}
		}

		defer func() {
			if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
				log.Printf("Failed to remove socket %s: %v", socket, err)
			}
		}()
	}

	// Create the handler
	h, err := container.NewHandler(impl.Name())
	if err != nil {
		errMsg := fmt.Sprintf("failed to create plugin handler %s", err.Error())
		log.Println(errMsg)
		panic(errMsg)
	}

	// Assign the handler to the plugin's Logger
	impl.Logger = h

	log.Println("Handler created, executing...")

	// Execute the handler - this blocks until connection is closed or signal received
	err = h.Execute()
	if err != nil {
		errMsg := fmt.Sprintf("failed to execute plugin handler %s", err.Error())
		log.Println(errMsg)
		panic(errMsg)
	}
	log.Println("Plugin execution finished")
}

func runManualMode(url, authType, username, password string) {
	// Force Logger to nil so Export uses standard log (stderr/file)
	impl.Logger = nil

	log.Printf("Running in manual mode. URL: %s", url)

	params := []string{url}
	if authType != "none" {
		params = append(params, authType, username, password)
	}

	// Configure default timeouts for manual mode since Configure isn't called
	impl.config.Timeout = 10
	impl.config.SkipVerify = true // Default for manual mode often useful

	// Passing nil for context is safe here as Export doesn't use it.
	res, err := impl.Export("segi9.http", params, nil)
	if err != nil {
		log.Printf("Export failed: %v", err)
		fmt.Printf("Error: %v\n", err) // Ensure error is printed to stdout too
		os.Exit(1)
	}
	fmt.Println(res.(string))
}
