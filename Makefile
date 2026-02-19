BINARY     := zabbix-plugin-segi9
PLUGIN_DIR := /usr/local/lib/zabbix/plugins
CONF_DIR   := /etc/zabbix/zabbix_agent2.d

.PHONY: all setup build install uninstall clean test

all: build

## setup: download/update the Zabbix SDK dependency.
##        Run this ONCE before the first build (requires internet access).
##        Update COMMIT_HASH with the latest from:
##        https://git.zabbix.com/projects/AP/repos/plugin-support/commits?at=refs/heads/release/7.4
setup:
	@echo ">>> Fetching Zabbix SDK…"
	go get golang.zabbix.com/sdk@af85407
	go mod tidy
	@echo ">>> Done. You can now run: make build"

## build: compile the plugin binary.
build:
	go build -o $(BINARY) .
	@echo ">>> Built: ./$(BINARY)"

## install: build and install binary + conf file.
install: build
	install -d $(PLUGIN_DIR)
	install -m 0755 $(BINARY) $(PLUGIN_DIR)/$(BINARY)
	install -d $(CONF_DIR)
	install -m 0644 segi9.conf $(CONF_DIR)/segi9.conf
	@echo ">>> Installed to $(PLUGIN_DIR)/$(BINARY)"
	@echo ">>> Config  to  $(CONF_DIR)/segi9.conf"
	@echo ">>> Edit $(CONF_DIR)/segi9.conf and restart zabbix-agent2."

## uninstall: remove installed files.
uninstall:
	rm -f $(PLUGIN_DIR)/$(BINARY)
	rm -f $(CONF_DIR)/segi9.conf

## clean: remove the compiled binary.
clean:
	rm -f $(BINARY)

# ---------------------------------------------------------------------------
# Manual / test targets — do not require a running Zabbix agent.
# ---------------------------------------------------------------------------

## test: quick smoke test against a public IP echo service.
test: build
	@echo ">>> Testing with no auth…"
	./$(BINARY) -manual "https://api.ipify.org"

## test-basic: test Basic Auth (edit URL/credentials as needed).
test-basic: build
	./$(BINARY) -manual "http://httpbin.org/basic-auth/admin/secret" \
	             -auth basic -user admin -pass secret

## test-bearer: test Bearer token auth (edit URL/token as needed).
test-bearer: build
	./$(BINARY) -manual "https://httpbin.org/bearer" \
	             -auth bearer -user "meu-token-aqui"

## test-skipverify: test skipping TLS verification (self-signed cert).
test-skipverify: build
	./$(BINARY) -manual "https://self-signed.badssl.com/" \
	             -auth none

.PHONY: help
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
