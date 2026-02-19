package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.zabbix.com/sdk/conf"
	"golang.zabbix.com/sdk/errs"
	"golang.zabbix.com/sdk/plugin"
)

// ---------------------------------------------------------------------------
// Compile-time interface assertions.
// If any method signature is wrong the build will fail here with a clear error.
// ---------------------------------------------------------------------------
var (
	_ plugin.Exporter     = (*Plugin)(nil)
	_ plugin.Runner       = (*Plugin)(nil)
	_ plugin.Configurator = (*Plugin)(nil)
)

// ---------------------------------------------------------------------------
// Plugin struct
// ---------------------------------------------------------------------------

// Plugin is the main struct for the Segi9 HTTP plugin.
type Plugin struct {
	plugin.Base        // Provides the Logger field and base Accessor methods.
	mu          sync.RWMutex
	config      Config
}

// Config holds all configurable options for the plugin.
// These map to the Zabbix agent configuration file entries:
//
//	Plugins.Segi9.Timeout=<1..30>
//	Plugins.Segi9.SkipVerify=<true|false>
type Config struct {
	Timeout    int  `conf:"optional,range=1:30,default=10"`
	SkipVerify bool `conf:"optional,default=false"`
}

// ---------------------------------------------------------------------------
// Logging helpers — nil-safe wrappers around plugin.Base.Logger.
//
// When running in manual mode (no Zabbix agent), p.Logger is nil and calling
// the base methods directly would panic. These helpers fall back to the
// standard library log package.
// ---------------------------------------------------------------------------

func (p *Plugin) logInfof(format string, args ...interface{}) {
	if p.Logger != nil {
		p.Logger.Infof(format, args...)
	} else {
		log.Printf("[INFO]  "+format, args...)
	}
}

func (p *Plugin) logDebugf(format string, args ...interface{}) {
	if p.Logger != nil {
		p.Logger.Debugf(format, args...)
	} else {
		log.Printf("[DEBUG] "+format, args...)
	}
}

func (p *Plugin) logErrf(format string, args ...interface{}) {
	if p.Logger != nil {
		p.Logger.Errf(format, args...)
	} else {
		log.Printf("[ERROR] "+format, args...)
	}
}

// ---------------------------------------------------------------------------
// Runner interface — Start / Stop lifecycle hooks.
// ---------------------------------------------------------------------------

// Start is called once by the Zabbix agent when the plugin process initialises.
func (p *Plugin) Start() {
	p.logInfof("Segi9 HTTP plugin started")
}

// Stop is called by the Zabbix agent before the plugin process is terminated.
func (p *Plugin) Stop() {
	p.logInfof("Segi9 HTTP plugin stopped")
}

// ---------------------------------------------------------------------------
// Configurator interface — Configure / Validate.
// ---------------------------------------------------------------------------

// Configure is called by the Zabbix agent each time the configuration changes.
// global contains the agent-wide settings (e.g. global timeout).
// options contains the plugin-specific settings from the conf file.
func (p *Plugin) Configure(global *plugin.GlobalOptions, options interface{}) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Reset to built-in defaults before applying new values.
	p.config = Config{
		Timeout:    10,
		SkipVerify: false,
	}

	if options != nil {
		if err := conf.Unmarshal(options, &p.config); err != nil {
			p.logErrf("failed to parse plugin configuration: %v", err)
			// Keep the defaults already set above and carry on.
		}
	}

	// Use the global agent timeout as a fallback only if the plugin timeout
	// ended up at zero (which should not happen with the default=10 tag,
	// but we guard against it just in case).
	if p.config.Timeout == 0 && global != nil && global.Timeout > 0 {
		p.config.Timeout = global.Timeout
	}

	// Hard clamp — ensures the value is always in a sane range regardless of
	// what the conf tag parser returned.
	if p.config.Timeout < 1 {
		p.config.Timeout = 1
	}
	if p.config.Timeout > 30 {
		p.config.Timeout = 30
	}

	p.logInfof("configuration applied: Timeout=%ds SkipVerify=%v",
		p.config.Timeout, p.config.SkipVerify)
}

// Validate is called by the Zabbix agent before applying a new configuration
// to verify that the values are acceptable. Return a non-nil error to reject.
func (p *Plugin) Validate(options interface{}) error {
	cfg := Config{
		Timeout:    10, // safe default if options is nil
		SkipVerify: false,
	}

	if options != nil {
		if err := conf.Unmarshal(options, &cfg); err != nil {
			return errs.Wrap(err, "failed to parse plugin configuration")
		}
	}

	if cfg.Timeout < 1 || cfg.Timeout > 30 {
		return fmt.Errorf(
			"Plugins.Segi9.Timeout: value %d is out of the allowed range [1..30]",
			cfg.Timeout,
		)
	}

	return nil
}

// ---------------------------------------------------------------------------
// Exporter interface — Export (called on every Zabbix check cycle).
// ---------------------------------------------------------------------------

// Export is invoked by the Zabbix agent for every segi9.http[...] item.
//
// Item key signature:
//
//	segi9.http[<url>, <auth_type>, <user_or_token>, <password>]
//
// Parameters:
//
//	url          (required) – target URL, e.g. https://api.exemplo.com/status
//	auth_type    (optional) – none (default) | basic | bearer
//	user_or_token(optional) – username (basic) or bearer token (bearer)
//	password     (optional) – password (basic only)
//
// Returns the raw HTTP response body as a string.
func (p *Plugin) Export(key string, params []string, _ plugin.ContextProvider) (interface{}, error) {
	if key != "segi9.http" {
		return nil, errs.Errorf("unsupported key: %q", key)
	}

	// Validate mandatory URL parameter.
	if len(params) == 0 || strings.TrimSpace(params[0]) == "" {
		return nil, fmt.Errorf("segi9.http: the first parameter (url) is required and cannot be empty")
	}

	url := strings.TrimSpace(params[0])

	authType := "none"
	if len(params) > 1 && strings.TrimSpace(params[1]) != "" {
		authType = strings.TrimSpace(params[1])
	}

	var user, pass string
	if len(params) > 2 {
		user = params[2]
	}
	if len(params) > 3 {
		pass = params[3]
	}

	p.logDebugf("export: key=%s url=%q auth=%s", key, url, authType)

	return p.doRequest(url, authType, user, pass)
}

// ---------------------------------------------------------------------------
// Core HTTP logic — shared by Export() and runManual().
// ---------------------------------------------------------------------------

// doRequest performs an HTTP GET request with the given authentication and
// returns the full response body as a string.
func (p *Plugin) doRequest(url, authType, user, pass string) (string, error) {
	// Read config under a shared (read) lock so we don't block other goroutines.
	p.mu.RLock()
	timeout := time.Duration(p.config.Timeout) * time.Second
	skipVerify := p.config.SkipVerify
	p.mu.RUnlock()

	// Safety net in case we are called before Configure (e.g. in manual mode
	// with an uninitialised config — though runManual sets the config directly).
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	// Build the HTTP client.
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipVerify, //nolint:gosec // user-controlled via Plugins.Segi9.SkipVerify
			},
		},
	}

	// Build the request.
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build HTTP request for %q: %w", url, err)
	}

	req.Header.Set("User-Agent", "Zabbix-Plugin-Segi9/1.0")
	req.Header.Set("Accept", "*/*")

	// Apply authentication.
	switch strings.ToLower(authType) {

	case "basic":
		// user = username, pass = password
		req.SetBasicAuth(user, pass)

	case "bearer":
		// user = the bearer token (pass is ignored)
		if user == "" {
			return "", fmt.Errorf("auth_type 'bearer' requires a token in the third parameter (user)")
		}
		req.Header.Set("Authorization", "Bearer "+user)

	case "none", "":
		// No authentication — nothing to add.

	default:
		return "", fmt.Errorf(
			"unsupported auth_type %q; valid values are: none, basic, bearer",
			authType,
		)
	}

	p.logDebugf("→ GET %s (timeout=%v tls_skip=%v)", url, timeout, skipVerify)

	// Execute the request.
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request to %q failed: %w", url, err)
	}
	defer resp.Body.Close()

	// Read the entire response body.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body from %q: %w", url, err)
	}

	p.logDebugf("← %s (%d bytes)", resp.Status, len(body))

	// Return the raw body. Zabbix pre-processing rules on the item can parse
	// it further (JSONPath, regex, etc.) as needed.
	return string(body), nil
}
