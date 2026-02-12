package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"git.zabbix.com/ap/plugin-support/plugin"
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
	if len(params) < 1 {
		return nil, errors.New("missing URL parameter")
	}

	url := params[0]
	if url == "" {
		return nil, errors.New("URL cannot be empty")
	}

	authType := "none"
	if len(params) > 1 && params[1] != "" {
		authType = strings.ToLower(params[1])
	}

	var usernameOrToken, password string
	if len(params) > 2 {
		usernameOrToken = params[2]
	}
	if len(params) > 3 {
		password = params[3]
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

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
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
		return nil, fmt.Errorf("unsupported auth type: %s", authType)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Return the raw JSON string regardless of status code, unless it's empty
	return string(body), nil
}

// Configure implements the Configurator interface
func (p *Plugin) Configure(global *plugin.GlobalOptions, privateOptions interface{}) {
	if privateOptions != nil {
		p.config = *privateOptions.(*Config)
	}

	// If timeout is not set in private options, use global timeout
	if p.config.Timeout == 0 && global.Timeout > 0 {
		p.config.Timeout = global.Timeout
	}
}

// Validate implements the Configurator interface
func (p *Plugin) Validate(privateOptions interface{}) error {
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
