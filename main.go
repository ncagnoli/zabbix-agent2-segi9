package main

import (
	"errors"
	stdflag "flag"
	"fmt"
	"io"
	"os"

	"golang.zabbix.com/sdk/errs"
	"golang.zabbix.com/sdk/plugin"
	"golang.zabbix.com/sdk/plugin/container"
	zbxflag "golang.zabbix.com/sdk/plugin/flag"
	"golang.zabbix.com/sdk/zbxerr"
)

const (
	PluginName      = "Segi9"
	PluginCopyright = "Copyright 2024 Zabbix SIA"
	PluginVersionRC = ""
	PluginVersion   = 1
	PluginMinor     = 0
	PluginPatch     = 0
)

func main() {
	// 1. Try to parse manual mode flags using a separate FlagSet.
	// We do this first because if the user passes -manual, we want to intercept it
	// before the Zabbix SDK handles flags (which might reject unknown flags).
	manualFlagSet := stdflag.NewFlagSet("manual", stdflag.ContinueOnError)
	manualFlagSet.SetOutput(io.Discard) // Suppress errors for unknown flags (like -V)

	var (
		manualURL = manualFlagSet.String("manual", "", "URL to request in manual/test mode (bypasses Zabbix agent communication)")
		authType  = manualFlagSet.String("auth", "none", "Authentication type: none | basic | bearer")
		user      = manualFlagSet.String("user", "", "Username (basic) or Bearer token (bearer)")
		pass      = manualFlagSet.String("pass", "", "Password for basic auth")
	)

	// Attempt to parse. If it fails (e.g. unknown flag -V), we just ignore and move to plugin mode.
	_ = manualFlagSet.Parse(os.Args[1:])

	if *manualURL != "" {
		runManual(*manualURL, *authType, *user, *pass)
		return
	}

	// 2. Handle standard Zabbix flags (-V, -h) using the SDK.
	err := zbxflag.HandleFlags(
		PluginName,
		os.Args[0],
		PluginCopyright,
		PluginVersionRC,
		PluginVersion,
		PluginMinor,
		PluginPatch,
	)
	if err != nil {
		// zbxflag.HandleFlags returns ErrorOSExitZero if help/version was printed successfully.
		if errors.Is(err, zbxerr.ErrorOSExitZero) {
			return
		}
		panic(err)
	}

	// 3. Run as a Zabbix loadable plugin (communicates via Unix socket with agent 2).
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
		PluginName,
		"segi9.http",
		"Performs an HTTP/HTTPS GET request from the agent host and returns the full response body.",
	)
	if err != nil {
		return errs.Wrap(err, "failed to register metrics")
	}

	h, err := container.NewHandler(PluginName)
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
