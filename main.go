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
	// Check if running as a plugin (arguments present and first argument is not a flag)
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

	// Configure logging for manual mode (optional, useful for debugging)
	configureLogging()

	if *manualURL != "" {
		runManualMode(*manualURL, *authType, *username, *password)
	} else {
		flag.Usage()
		os.Exit(1)
	}
}

func configureLogging() {
	// Default log output is stderr.
	// log.SetOutput(os.Stderr) // Removed to avoid potential blocking issues in plugin mode, defaults to stderr anyway

	if logPath := os.Getenv("SEGI9_LOG_FILE"); logPath != "" {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open log file %s: %v. Logging to stderr.", logPath, err)
		} else {
			// Note: We can't defer f.Close() here because it would close immediately.
			// Ideally, we should keep it open. For now, we rely on OS to close it on exit.
			log.SetOutput(f)
		}
	}
}

func runPluginMode() {
	configureLogging()

	log.Printf("Starting Segi9 plugin. Args: %v", os.Args)

	h, err := container.NewHandler(impl.Name())
	if err != nil {
		errMsg := fmt.Sprintf("failed to create plugin handler %s", err.Error())
		log.Println(errMsg)
		panic(errMsg)
	}
	impl.Logger = h

	log.Println("Handler created, executing...")

	err = h.Execute()
	if err != nil {
		errMsg := fmt.Sprintf("failed to execute plugin handler %s", err.Error())
		log.Println(errMsg)
		panic(errMsg)
	}
	log.Println("Plugin execution finished")
}

func runManualMode(url, authType, username, password string) {
	// Force Logger to nil so Export uses standard log (stderr)
	impl.Logger = nil

	params := []string{url}
	if authType != "none" {
		params = append(params, authType, username, password)
	}

	// Passing nil for context is safe here as Export doesn't use it.
	res, err := impl.Export("segi9.http", params, nil)
	if err != nil {
		log.Printf("Export failed: %v", err)
		os.Exit(1)
	}
	fmt.Println(res.(string))
}
