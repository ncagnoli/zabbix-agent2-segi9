package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.zabbix.com/sdk/conf"
	"golang.zabbix.com/sdk/plugin"
)

// Plugin defines the structure of the plugin
type Plugin struct {
	plugin.Base
	config Config
	mu     sync.RWMutex
}

// Config stores the configuration for the plugin
type Config struct {
	Timeout    int  `conf:"optional,range=1:30,default=10"`
	SkipVerify bool `conf:"optional,default=false"`
}

var impl Plugin

// Start implements the Starter interface
func (p *Plugin) Start() {
	if p.Logger != nil {
		p.Logger.Infof("Segi9 plugin started")
	} else {
		log.Println("Segi9 plugin started")
	}
}

// Stop implements the Stopper interface
func (p *Plugin) Stop() {
	if p.Logger != nil {
		p.Logger.Infof("Segi9 plugin stopped")
	} else {
		log.Println("Segi9 plugin stopped")
	}
}

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

	// Capture config with read lock
	p.mu.RLock()
	timeoutVal := p.config.Timeout
	skipVerify := p.config.SkipVerify
	p.mu.RUnlock()

	// Create HTTP client
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: skipVerify},
	}

	// Use configured timeout or default to 10s if 0
	timeout := time.Duration(timeoutVal) * time.Second
	if timeoutVal == 0 {
		timeout = 10 * time.Second
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   timeout,
	}

	logMsg("Creating request to %s with timeout %v, SkipVerify: %v", url, timeout, skipVerify)

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
	logMsg := func(format string, args ...interface{}) {
		if p.Logger != nil {
			p.Logger.Infof(format, args...)
		} else {
			log.Printf(format, args...)
		}
	}

	logMsg("Configure called")

	p.mu.Lock()
	defer p.mu.Unlock()

	// Initialize with defaults first
	if err := conf.Unmarshal(nil, &p.config); err != nil {
		logMsg("Failed to set default config: %v", err)
	}

	if privateOptions != nil {
		if config, ok := privateOptions.(*Config); ok {
			p.config = *config
		} else if privateMap, ok := privateOptions.(map[string]interface{}); ok {
			logMsg("Configuration passed as map: %v", privateMap)
			// Marshal map to JSON
			jsonBytes, err := json.Marshal(privateMap)
			if err != nil {
				logMsg("Failed to marshal config map: %v", err)
			} else {
				// Unmarshal JSON to struct.
				// Note: This overwrites only fields present in the map.
				if err = json.Unmarshal(jsonBytes, &p.config); err != nil {
					logMsg("Failed to unmarshal JSON config: %v", err)
				}
			}
		} else {
			logMsg("Unknown configuration type: %T", privateOptions)
		}
	}

	// Apply global timeout if local timeout is invalid (0) or if we wanted to support inheritance.
	// Since default is 10, p.config.Timeout is usually >= 1.
	// If the user explicitly set 0 (which Validate should catch, but let's be safe), use global.
	if global != nil && global.Timeout > 0 {
		if p.config.Timeout == 0 {
			p.config.Timeout = global.Timeout
		}
	}

	// Final safeguard: ensure timeout is at least 1 second
	if p.config.Timeout < 1 {
		p.config.Timeout = 1
		logMsg("Warning: Timeout corrected to minimum 1s")
	}

	logMsg("Configuration set: Timeout=%d, SkipVerify=%v", p.config.Timeout, p.config.SkipVerify)
}

// Validate implements the Configurator interface
func (p *Plugin) Validate(privateOptions interface{}) error {
	logMsg := func(format string, args ...interface{}) {
		if p.Logger != nil {
			p.Logger.Debugf(format, args...)
		} else {
			log.Printf(format, args...)
		}
	}

	logMsg("Validate called")

	var cfg Config
	// Initialize with defaults first
	if err := conf.Unmarshal(nil, &cfg); err != nil {
		return fmt.Errorf("failed to set default config: %w", err)
	}

	if privateOptions != nil {
		if config, ok := privateOptions.(*Config); ok {
			cfg = *config
		} else if privateMap, ok := privateOptions.(map[string]interface{}); ok {
			jsonBytes, err := json.Marshal(privateMap)
			if err != nil {
				return fmt.Errorf("failed to marshal config map: %w", err)
			}
			if err = json.Unmarshal(jsonBytes, &cfg); err != nil {
				return fmt.Errorf("failed to unmarshal JSON config: %w", err)
			}
		}
	}

	// Manual validation
	if cfg.Timeout < 1 || cfg.Timeout > 30 {
		return fmt.Errorf("invalid timeout: %d (must be between 1 and 30)", cfg.Timeout)
	}

	logMsg("Validation successful: Timeout=%d, SkipVerify=%v", cfg.Timeout, cfg.SkipVerify)
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
