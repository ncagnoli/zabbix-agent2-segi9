package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"golang.zabbix.com/sdk/plugin"
)

// Plugin defines the structure of the plugin
type Plugin struct {
	plugin.Base
	config Config
}

// Config stores the configuration for the plugin
type Config struct {
	Timeout int `conf:"optional,range=1:30,default=10"`
}

var impl Plugin

// Export implements the Exporter interface
func (p *Plugin) Export(key string, params []string, ctx plugin.ContextProvider) (interface{}, error) {
	// Use p.Logger if available, falling back to log (stderr) if not (e.g. manual mode or not yet initialized)
	logMsg := func(format string, args ...interface{}) {
		if p.Logger != nil {
			p.Logger.Infof(format, args...)
		} else {
			log.Printf(format, args...)
		}
	}

	logMsg("Export called with key: %s, params count: %d", key, len(params))

	if len(params) < 1 {
		logMsg("Error: missing URL parameter")
		return nil, errors.New("missing URL parameter")
	}

	url := params[0]
	if url == "" {
		logMsg("Error: URL cannot be empty")
		return nil, errors.New("URL cannot be empty")
	}
	logMsg("URL: %s", url)

	authType := "none"
	if len(params) > 1 && params[1] != "" {
		authType = strings.ToLower(params[1])
		logMsg("AuthType: %s", authType)
	}

	var usernameOrToken, password string
	if len(params) > 2 {
		usernameOrToken = params[2]
		logMsg("Username/Token provided (masked)")
	}
	if len(params) > 3 {
		password = params[3]
		logMsg("Password provided (masked)")
	}

	// Create HTTP client with insecure skip verify
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	// Use configured timeout or default to 10s if 0
	timeout := time.Duration(p.config.Timeout) * time.Second
	if p.config.Timeout == 0 {
		timeout = 10 * time.Second
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	logMsg("Creating request to %s with timeout %v", url, timeout)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		logMsg("Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Handle Authentication
	switch authType {
	case "basic":
		req.SetBasicAuth(usernameOrToken, password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+usernameOrToken)
	case "none":
		// Do nothing
	default:
		logMsg("Unsupported auth type: %s", authType)
		return nil, fmt.Errorf("unsupported auth type: %s", authType)
	}

	logMsg("Sending %s request to %s...", req.Method, req.URL.String())
	resp, err := client.Do(req)
	if err != nil {
		logMsg("Request failed: %v", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	logMsg("Response received: Status %s, StatusCode %d", resp.Status, resp.StatusCode)
	logMsg("Reading response body...")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logMsg("Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	logMsg("Body read successfully (%d bytes)", len(body))

	// Return the raw JSON string regardless of status code, unless it's empty
	return string(body), nil
}

// Configure implements the Configurator interface
func (p *Plugin) Configure(global *plugin.GlobalOptions, privateOptions interface{}) {
	// Configure logging is tricky here because impl.Logger is set in main.
	// But p is the receiver.
	// p.Logger is available.

	logMsg := func(format string, args ...interface{}) {
		if p.Logger != nil {
			p.Logger.Infof(format, args...)
		} else {
			log.Printf(format, args...)
		}
	}

	logMsg("Configure called")
	if privateOptions != nil {
		p.config = *privateOptions.(*Config)
	}

	// If timeout is not set in private options, use global timeout
	if global != nil && p.config.Timeout == 0 && global.Timeout > 0 {
		p.config.Timeout = global.Timeout
	}
	logMsg("Configuration set: Timeout=%d", p.config.Timeout)
}

// Validate implements the Configurator interface
func (p *Plugin) Validate(privateOptions interface{}) error {
	// Validate is called before Logger might be set?
	// Usually Configure -> Validate -> Export.
	// But main sets Logger before Execute.
	// So Logger should be available.
	if p.Logger != nil {
		p.Logger.Infof("Validate called")
	} else {
		log.Println("Validate called")
	}
	return nil
}

// Name returns the plugin name
func (p *Plugin) Name() string {
	return "Segi9"
}

func init() {
	// Simple init log, likely goes to stderr or lost if log output not set yet
	// But since this runs before main, we can't rely on the file log yet.
	// We can use fmt.Println to stderr.
	// fmt.Fprintln(os.Stderr, "Segi9 plugin init") // Commented out to avoid noise if not needed

	plugin.RegisterMetrics(&impl, "Segi9",
		"segi9.http", "Make HTTP/HTTPS requests to any reachable service and return JSON status.")
}
