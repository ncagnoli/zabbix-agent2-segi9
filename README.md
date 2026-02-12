# Zabbix Agent 2 Plugin: Segi9

This is a loadable plugin for Zabbix Agent 2, designed to make HTTP/HTTPS requests to localhost services (like Elasticsearch, NATS, etc.) and return the raw JSON status response.

## Features

- **Metric**: `segi9.local.http`
- **Dynamic Parameters**: URL, Auth Type, Username/Token, Password.
- **TLS Security**: Automatically skips SSL/TLS verification (`InsecureSkipVerify: true`) to support self-signed certificates on localhost.
- **Authentication**: Supports `Basic` and `Bearer` authentication.
- **Output**: Returns the raw JSON body as a string (parsing is handled by Zabbix Server via JSONPath/LLD).

## Requirements

- Zabbix Agent 2 (version 6.0+)
- Go (version 1.17+ for building)

## Installation

### 1. Build the Plugin

```bash
# Clone the repository
git clone <repository-url>
cd zabbix-agent2-segi9

# Download dependencies
go mod tidy

# Build the binary
go build -o zabbix-agent2-segi9
```

### 2. Deploy the Plugin

Move the compiled binary to a directory accessible by Zabbix Agent 2 (e.g., `/usr/local/zabbix/go/plugins/segi9/`).

```bash
mkdir -p /usr/local/zabbix/go/plugins/segi9
mv zabbix-agent2-segi9 /usr/local/zabbix/go/plugins/segi9/
```

### 3. Configure Zabbix Agent 2

Copy the configuration file `segi9.conf` to the Zabbix Agent 2 plugin configuration directory (typically `/etc/zabbix/zabbix_agent2.d/plugins.d/`).

```bash
sudo cp segi9.conf /etc/zabbix/zabbix_agent2.d/plugins.d/
```

Edit the configuration file to ensure the path is correct:

```ini
Plugins.Segi9.System.Path=/usr/local/zabbix/go/plugins/segi9/zabbix-agent2-segi9
```

### 4. Restart Zabbix Agent 2

```bash
sudo systemctl restart zabbix-agent2
```

## Usage

### Key Format

`segi9.local.http[<url>, <auth_type>, <username_or_token>, <password>]`

- `url`: (Required) The URL to request (e.g., `https://127.0.0.1:9200/_cluster/health`).
- `auth_type`: (Optional) `none` (default), `basic`, or `bearer`.
- `username_or_token`: (Optional) Username for Basic Auth or Token for Bearer Auth.
- `password`: (Optional) Password for Basic Auth.

### Examples

**1. Simple Request (No Auth):**
```
segi9.local.http[https://127.0.0.1:9200/_cluster/health]
```

**2. Basic Authentication:**
```
segi9.local.http[https://127.0.0.1:9200/_cluster/health,basic,myuser,mypassword]
```

**3. Bearer Token Authentication:**
```
segi9.local.http[https://127.0.0.1:8222/varz,bearer,my-secret-token]
```

## Troubleshooting

You can test the plugin manually using the `zabbix_agent2` command line:

```bash
zabbix_agent2 -t segi9.local.http[https://google.com]
```

Or by running the plugin binary directly (though it communicates via stdin/stdout protocol):

```bash
./zabbix-agent2-segi9
```
(Note: Running directly without the agent wrapper will just start the handler and wait for instructions).

## Configuration Options

You can configure the global timeout in `segi9.conf`:

```ini
Plugins.Segi9.Timeout=10
```

## License

MIT
