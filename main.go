package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"git.zabbix.com/ap/plugin-support/plugin/container"
)

func main() {
	var (
		manualURL = flag.String("manual", "", "Execute manually with the given URL")
		authType  = flag.String("auth", "none", "Authentication type (none, basic, bearer)")
		username  = flag.String("user", "", "Username or token")
		password  = flag.String("pass", "", "Password")
	)
	flag.Parse()

	// Default log output is stderr.
	log.SetOutput(os.Stderr)

	if logPath := os.Getenv("SEGI9_LOG_FILE"); logPath != "" {
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Printf("Failed to open log file %s: %v. Logging to stderr.", logPath, err)
		} else {
			defer f.Close()
			log.SetOutput(f)
		}
	}

	if *manualURL != "" {
		runManualMode(*manualURL, *authType, *username, *password)
		return
	}

	log.Printf("Starting Segi9 plugin. Args: %v", os.Args)

	h, err := container.NewHandler(impl.Name())
	if err != nil {
		errMsg := fmt.Sprintf("failed to create plugin handler %s", err.Error())
		log.Println(errMsg)
		panic(errMsg)
	}
	impl.Logger = &h

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
