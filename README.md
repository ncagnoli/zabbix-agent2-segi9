# Zabbix Agent 2 Plugin: Segi9

This is a loadable plugin for Zabbix Agent 2, designed to make HTTP/HTTPS requests to any reachable service (localhost or remote) and return the raw JSON status response.

## Features

- **Metric**: `segi9.http`
- **Dynamic Parameters**: URL, Auth Type, Username/Token, Password.
- **TLS Security**: Automatically skips SSL/TLS verification (`InsecureSkipVerify: true`) to support self-signed certificates, unless configured otherwise.
- **Authentication**: Supports `Basic` and `Bearer` authentication.
- **Output**: Returns the raw JSON body as a string.

## Requirements

- Zabbix Agent 2 (version 6.0+)
- Go (version 1.21+ recommended for building)

## Installation

### 1. Build the Plugin

```bash
# Clone the repository
git clone <repository-url>
cd zabbix-agent2-segi9

# Build the binary
go build -o zabbix-agent2-segi9
```

### 2. Deploy the Plugin

Move the compiled binary to a directory accessible by Zabbix Agent 2.

```bash
# Example directory
mkdir -p /usr/local/zabbix/go/plugins/segi9
mv zabbix-agent2-segi9 /usr/local/zabbix/go/plugins/segi9/
```

### 3. Configure Zabbix Agent 2

Copy the configuration file `segi9.conf` to the Zabbix Agent 2 plugin configuration directory (typically `/etc/zabbix/zabbix_agent2.d/plugins.d/`).

```bash
sudo cp segi9.conf /etc/zabbix/zabbix_agent2.d/plugins.d/
```

Edit the configuration file to ensure the `Path` is correct:

```ini
Plugins.Segi9.System.Path=/usr/local/zabbix/go/plugins/segi9/zabbix-agent2-segi9
```

### 4. Restart Zabbix Agent 2

```bash
sudo systemctl restart zabbix-agent2
```

## Usage

### Key Format

`segi9.http[<url>, <auth_type>, <username_or_token>, <password>]`

- `url`: (Required) The URL to request (e.g., `https://127.0.0.1:9200/_cluster/health`).
- `auth_type`: (Optional) `none` (default), `basic`, or `bearer`.
- `username_or_token`: (Optional) Username for Basic Auth or Token for Bearer Auth.
- `password`: (Optional) Password for Basic Auth.

### Examples

**1. Simple Request (No Auth):**
```
segi9.http[https://127.0.0.1:9200/_cluster/health]
```

**2. Basic Authentication:**
```
segi9.http[https://remote-host:9200/_cluster/health,basic,myuser,mypassword]
```

**3. Bearer Token Authentication:**
```
segi9.http[https://api.example.com/v1/status,bearer,my-secret-token]
```

## Configuration Options

You can configure the plugin in `segi9.conf`:

```ini
# Timeout for HTTP requests (default: 10, range: 1-30)
Plugins.Segi9.Timeout=10

# Skip TLS certificate verification (default: 0)
Plugins.Segi9.SkipVerify=0
```

## Troubleshooting & Debugging

If the Zabbix Agent 2 service fails to start or reports connection errors with the plugin:

### 1. Enable Debug Logging

Since the plugin output is handled by Zabbix Agent, startup errors might be missed. You can force the plugin to log to a file by setting the `SEGI9_LOG_FILE` environment variable in the Zabbix Agent 2 service environment.

**Method A: Edit systemd service**

1.  Run `systemctl edit zabbix-agent2`
2.  Add the following:
    ```ini
    [Service]
    Environment="SEGI9_LOG_FILE=/tmp/segi9_debug.log"
    ```
3.  Save and exit.
4.  Restart the service: `systemctl restart zabbix-agent2`
5.  Check the log file: `cat /tmp/segi9_debug.log`

### 2. Manual Testing

You can run the plugin manually to verify it works in isolation:

```bash
# Test with a URL (Manual Mode)
./zabbix-agent2-segi9 -manual https://google.com
```

### 3. Common Errors

- **`failed to create connection`**:
    -   This usually means the plugin failed to start or crashed immediately.
    -   Check permissions: Ensure the user running `zabbix_agent2` (usually `zabbix`) has execute permissions on the plugin binary.
    -   Check `SEGI9_LOG_FILE` for crash logs.
    -   Ensure `Plugins.Segi9.System.Path` in `segi9.conf` points to the correct location.

- **`invalid timeout`**:
    -   Ensure `Plugins.Segi9.Timeout` is between 1 and 30.

## License

MIT
