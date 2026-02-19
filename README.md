# zabbix-plugin-segi9

Loadable plugin for **Zabbix Agent 2** that acts as an HTTP/HTTPS proxy.
The agent executes the request **locally** (on the host where it is installed) and returns the full response body to the Zabbix server.

---

## Metric Key

```
segi9.http[<url>, <auth_type>, <user_or_token>, <password>]
```

| Parameter       | Req. | Description                                                 |
|-----------------|------|-------------------------------------------------------------|
| `url`           | ✓    | Target URL, e.g.: `https://api.example.com/status`          |
| `auth_type`     |      | `none` (default) · `basic` · `bearer`                       |
| `user_or_token` |      | Username (`basic`) or token (`bearer`)                      |
| `password`      |      | Password (`basic` only; ignored for `bearer`)               |

### Zabbix Item Key Examples

```
# No authentication
segi9.http[https://api.example.com/status]

# Basic Auth
segi9.http[https://api.internal.com/metrics,basic,admin,s3cr3t]

# Bearer Token
segi9.http[https://api.external.com/data,bearer,eyJhbGciOiJSUzI1NiJ9...]
```

---

## Configuration (`segi9.conf`)

```ini
# Path to the plugin binary (REQUIRED)
Plugins.Segi9.System.Path=/usr/local/lib/zabbix/plugins/zabbix-plugin-segi9

# HTTP request timeout in seconds (1–30, default: 10)
# Plugins.Segi9.Timeout=10

# Ignore TLS/SSL certificate errors (default: false)
# Plugins.Segi9.SkipVerify=false
```

---

## Build and Installation

### Prerequisites

- Go 1.21+
- Access to `git.zabbix.com` (to download the Zabbix SDK)

### 1 – Get the SDK

```bash
go get golang.zabbix.com/sdk@<COMMIT_HASH>
go mod tidy
```

> Find the latest hash for the `release/7.4` branch at:
> https://git.zabbix.com/projects/AP/repos/plugin-support/commits?at=refs%2Fheads%2Frelease%2F7.4

Or use the Makefile shortcut:

```bash
make setup
```

### 2 – Compile

```bash
make build
# or directly:
go build -o zabbix-plugin-segi9 .
```

### 3 – Install

```bash
sudo make install
```

This copies the binary to `/usr/local/lib/zabbix/plugins/` and `segi9.conf` to `/etc/zabbix/zabbix_agent2.d/`.

Edit `segi9.conf` as needed and restart the agent:

```bash
sudo systemctl restart zabbix-agent2
```

---

## Manual Mode (Test without Agent)

The plugin can be executed directly in the terminal for quick debugging:

```bash
# No authentication
./zabbix-plugin-segi9 -manual "https://api.ipify.org"

# Basic Auth
./zabbix-plugin-segi9 -manual "http://httpbin.org/basic-auth/admin/secret" \
                      -auth basic -user admin -pass secret

# Bearer Token
./zabbix-plugin-segi9 -manual "https://httpbin.org/bearer" \
                      -auth bearer -user "my-token"
```

Result is printed directly to `stdout`. Errors go to `stderr`.

---

## Project Structure

```
.
├── main.go       ← entry point: plugin mode (Zabbix) and manual mode (-manual)
├── plugin.go     ← HTTP logic, Exporter / Runner / Configurator interfaces
├── go.mod
├── segi9.conf    ← configuration template for the agent
├── Makefile
└── README.md
```

---

## Zabbix Preprocessing (Optional)

Since the plugin returns the **raw body** of the response, you can use Zabbix preprocessing rules to extract specific fields:

| Type           | Example Expression                           |
|----------------|---------------------------------------------|
| JSONPath        | `$.status`                                  |
| Regex          | `uptime: (\d+)`                             |
| JavaScript     | `return JSON.parse(value).metrics.cpu_pct;` |

---

## Security

- `SkipVerify=false` (default): TLS certificates **are** verified.
- Use `SkipVerify=true` only in controlled environments (e.g., internal monitoring with self-signed certificates).
- Credentials passed in the key parameters are visible in the Zabbix database. For sensitive environments, consider using Zabbix secret macros.
