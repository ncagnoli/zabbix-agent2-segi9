package main

import (
	"flag"
	"fmt"
	"os"

	"golang.zabbix.com/sdk/errs"
	"golang.zabbix.com/sdk/plugin"
	"golang.zabbix.com/sdk/plugin/container"
)

func main() {
	// ----- Manual/test mode flags -----
	var (
		manualURL = flag.String("manual", "", "URL to request in manual/test mode (bypasses Zabbix agent communication)")
		authType  = flag.String("auth", "none", "Authentication type: none | basic | bearer")
		user      = flag.String("user", "", "Username (basic) or Bearer token (bearer)")
		pass      = flag.String("pass", "", "Password for basic auth")
	)
	flag.Parse()

	// If -manual flag is present, run in standalone test mode and exit.
	if *manualURL != "" {
		runManual(*manualURL, *authType, *user, *pass)
		return
	}

	// Otherwise run as a Zabbix loadable plugin (communicates via Unix socket with agent 2).
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// run registers metrics, sets up the IPC handler and blocks until the agent shuts the plugin down.
func run() error {
	p := &Plugin{}

	err := plugin.RegisterMetrics(
		p,
		"Segi9", // Plugin name — must match Plugins.Segi9.* in conf files
		"segi9.http",
		"Performs an HTTP/HTTPS GET request from the agent host and returns the full response body.",
	)
	if err != nil {
		return errs.Wrap(err, "failed to register metrics")
	}

	h, err := container.NewHandler("Segi9")
	if err != nil {
		return errs.Wrap(err, "failed to create plugin handler")
	}

	// Wire the SDK handler as the Logger so that p.logInfof / Debugf / Errf
	// forward to the Zabbix agent log.
	p.Logger = h

	// Execute blocks until the agent sends a termination signal.
	return errs.Wrap(h.Execute(), "failed to execute plugin handler")
}

// runManual performs a single HTTP request and prints the result to stdout.
// Useful for quick testing without a running Zabbix agent.
//
// Usage:
//
//	./zabbix-plugin-segi9 -manual "https://api.exemplo.com/status"
//	./zabbix-plugin-segi9 -manual "https://api.exemplo.com/secure" -auth basic -user "admin" -pass "secret"
//	./zabbix-plugin-segi9 -manual "https://api.exemplo.com/token"  -auth bearer -user "eyJhbGci..."
func runManual(url, authType, user, pass string) {
	p := &Plugin{}

	// Use safe defaults for manual / test mode.
	p.config = Config{
		Timeout:    10,
		SkipVerify: true, // convenient for testing self-signed certs locally
	}

	fmt.Fprintf(os.Stderr, "[manual] url=%s auth=%s\n", url, authType)

	result, err := p.doRequest(url, authType, user, pass)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result)
}
