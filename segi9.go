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

// Helper to log info messages safely (handles nil Logger in manual mode)
func (p *Plugin) Infof(format string, args ...interface{}) {
	if p.Logger != nil {
		p.Logger.Infof(format, args...)
	} else {
		log.Printf(format, args...)
	}
}

// Helper to log debug messages safely
func (p *Plugin) Debugf(format string, args ...interface{}) {
	if p.Logger != nil {
		p.Logger.Debugf(format, args...)
	} else {
		// In manual mode, we might want to see debug logs too if verbose?
		// For now, treat as standard log.
		log.Printf("[DEBUG] "+format, args...)
	}
}

// Helper to log error messages safely
func (p *Plugin) Errorf(format string, args ...interface{}) {
	if p.Logger != nil {
		p.Logger.Errf(format, args...)
	} else {
		log.Printf("[ERROR] "+format, args...)
	}
}

// Start implements the Starter interface
func (p *Plugin) Start() {
	p.Infof("Segi9 plugin started")
}

// Stop implements the Stopper interface
func (p *Plugin) Stop() {
	p.Infof("Segi9 plugin stopped")
}

// Export implements the Exporter interface
func (p *Plugin) Export(key string, params []string, ctx plugin.ContextProvider) (interface{}, error) {
	p.Debugf("Export called with key: %s, params count: %d", key, len(params))

	if len(params) < 1 {
		p.Errorf("Error: missing URL parameter")
		return nil, errors.New("missing URL parameter")
	}

	url := params[0]
	if url == "" {
		p.Errorf("Error: URL cannot be empty")
		return nil, errors.New("URL cannot be empty")
	}
	p.Debugf("URL: %s", url)

	authType := "none"
	if len(params) > 1 && params[1] != "" {
		authType = strings.ToLower(params[1])
		p.Debugf("AuthType: %s", authType)
	}

	var usernameOrToken, password string
	if len(params) > 2 {
		usernameOrToken = params[2]
		p.Debugf("Username/Token provided (masked)")
	}
	if len(params) > 3 {
		password = params[3]
		p.Debugf("Password provided (masked)")
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

	p.Debugf("Creating request to %s with timeout %v, SkipVerify: %v", url, timeout, skipVerify)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		p.Errorf("Failed to create request: %v", err)
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
		p.Errorf("Unsupported auth type: %s", authType)
		return nil, fmt.Errorf("unsupported auth type: %s", authType)
	}

	p.Debugf("Sending %s request to %s...", req.Method, req.URL.String())
	resp, err := client.Do(req)
	if err != nil {
		p.Errorf("Request failed: %v", err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	p.Debugf("Response received: Status %s, StatusCode %d", resp.Status, resp.StatusCode)
	p.Debugf("Reading response body...")
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		p.Errorf("Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	p.Debugf("Body read successfully (%d bytes)", len(body))

	// Return the raw JSON string regardless of status code, unless it's empty
	return string(body), nil
}

// Configure implements the Configurator interface
func (p *Plugin) Configure(global *plugin.GlobalOptions, privateOptions interface{}) {
	p.Debugf("Configure called")

	p.mu.Lock()
	defer p.mu.Unlock()

	// Initialize with defaults first
	if err := conf.Unmarshal(nil, &p.config); err != nil {
		p.Errorf("Failed to set default config: %v", err)
	}

	if privateOptions != nil {
		if config, ok := privateOptions.(*Config); ok {
			p.config = *config
		} else if privateMap, ok := privateOptions.(map[string]interface{}); ok {
			p.Debugf("Configuration passed as map: %v", privateMap)
			// Marshal map to JSON
			jsonBytes, err := json.Marshal(privateMap)
			if err != nil {
				p.Errorf("Failed to marshal config map: %v", err)
			} else {
				// Unmarshal JSON to struct.
				// Note: This overwrites only fields present in the map.
				if err = json.Unmarshal(jsonBytes, &p.config); err != nil {
					p.Errorf("Failed to unmarshal JSON config: %v", err)
				}
			}
		} else {
			p.Debugf("Unknown configuration type: %T", privateOptions)
		}
	}

	// Apply global timeout if local timeout is invalid (0) or if we wanted to support inheritance.
	// Since default is 10, p.config.Timeout is usually >= 1.
	if global != nil && global.Timeout > 0 {
		if p.config.Timeout == 0 {
			p.config.Timeout = global.Timeout
		}
	}

	// Final safeguard: ensure timeout is at least 1 second
	if p.config.Timeout < 1 {
		p.config.Timeout = 1
		p.Infof("Warning: Timeout corrected to minimum 1s")
	}

	p.Infof("Configuration set: Timeout=%d, SkipVerify=%v", p.config.Timeout, p.config.SkipVerify)
}

// Validate implements the Configurator interface
func (p *Plugin) Validate(privateOptions interface{}) error {
	// We can't rely on p.Logger here easily because Validate might be called before Configure?
	// Actually, Zabbix calls Validate via RPC. p.Logger should be available if connection is up.
	// But let's be safe.
	p.Debugf("Validate called")

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

	p.Debugf("Validation successful: Timeout=%d, SkipVerify=%v", cfg.Timeout, cfg.SkipVerify)
	return nil
}

// Name returns the plugin name
func (p *Plugin) Name() string {
	return "Segi9"
}

func init() {
	plugin.RegisterMetrics(&impl, "Segi9",
		"segi9.http", "Make HTTP/HTTPS requests to any reachable service and return JSON status.")
}
