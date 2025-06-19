# Configuration Guide

Armonite uses YAML configuration files to customize behavior for coordinators and agents. This document describes all available configuration options.

## Configuration File Locations

Armonite automatically searches for configuration files in the following order:

1. File specified with `--config` flag
2. `armonite.yaml` (current directory)
3. `armonite.yml` (current directory)
4. `config/armonite.yaml`
5. `config/armonite.yml`
6. `~/.armonite.yaml` (home directory)
7. `~/.armonite.yml` (home directory)

## Configuration Structure

```yaml
server:
  host: 0.0.0.0
  port: 4222
  http_port: 8080
  enable_ui: true

database:
  dsn: "./armonite.db"
  max_open: 25
  max_idle: 5
  max_lifetime: "1h"

logging:
  level: info
  format: text
  file: ""

output:
  directory: ./results
  formats:
    - json
    - csv
  filename: armonite-results

defaults:
  concurrency: 100
  duration: 1m
  broadcast_interval: 5s
  telemetry_interval: 5s
  keep_alive: true
  min_agents: 1
```

## Configuration Sections

### Server Configuration

Controls network and HTTP server settings.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `host` | string | `0.0.0.0` | Bind address for NATS server and HTTP API |
| `port` | int | `4222` | Port for internal NATS communication between coordinator and agents |
| `http_port` | int | `8080` | Port for HTTP API and web interface |
| `enable_ui` | bool | `true` | Enable the embedded React web UI |

**Examples:**
```yaml
server:
  host: 0.0.0.0        # Listen on all interfaces
  port: 4222           # NATS port for agent communication
  http_port: 8081      # Web UI at http://localhost:8081
  enable_ui: true      # Enable web interface
```

```yaml
server:
  host: 127.0.0.1      # Localhost only
  port: 4222
  http_port: 9000      # Custom HTTP port
  enable_ui: false     # API only, no web UI
```

### Database Configuration

SQLite database settings for persistent storage of test runs and results.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `dsn` | string | `"./armonite.db"` | SQLite database file path |
| `max_open` | int | `25` | Maximum number of open database connections |
| `max_idle` | int | `5` | Maximum number of idle connections in pool |
| `max_lifetime` | string | `"1h"` | Maximum lifetime of a database connection |

**Examples:**
```yaml
database:
  dsn: "./data/armonite.db"     # Custom database location
  max_open: 50                  # Higher connection limit
  max_idle: 10
  max_lifetime: "2h"
```

```yaml
database:
  dsn: "/var/lib/armonite/db"   # System-wide database
  max_open: 10                  # Conservative settings
  max_idle: 2
  max_lifetime: "30m"
```

**Supported Duration Formats:**
- `30s` - 30 seconds
- `5m` - 5 minutes
- `1h` - 1 hour
- `24h` - 24 hours

### Logging Configuration

Controls log output format, level, and destination.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `"info"` | Minimum log level to output |
| `format` | string | `"text"` | Log output format |
| `file` | string | `""` | Log file path (empty = stdout) |

**Log Levels (ascending verbosity):**
- `fatal` - Only fatal errors
- `error` - Errors and fatal
- `warn` - Warnings, errors, and fatal
- `info` - General info and above (default)
- `debug` - Detailed debugging information

**Log Formats:**
- `text` - Human-readable text format
- `json` - Structured JSON format (better for log aggregation)

**Examples:**
```yaml
logging:
  level: debug         # Verbose logging for troubleshooting
  format: json         # Structured logs
  file: ""             # Output to console
```

```yaml
logging:
  level: warn          # Only warnings and errors
  format: text         # Human-readable
  file: "/var/log/armonite.log"  # Log to file
```

### Output Configuration

Controls where and how test results are saved.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `directory` | string | `"./results"` | Directory to save test result files |
| `formats` | []string | `["json"]` | Output formats for test results |
| `filename` | string | `"armonite-results"` | Base filename for result files |

**Supported Output Formats:**
- `json` - JSON format (recommended)
- `csv` - Comma-separated values
- `xml` - XML format
- `yaml` - YAML format

**Examples:**
```yaml
output:
  directory: ./test-results      # Custom results directory
  formats:
    - json                       # Primary format
    - csv                        # Additional CSV export
  filename: load-test-results    # Custom filename prefix
```

```yaml
output:
  directory: /shared/results     # Network shared directory
  formats:
    - json
    - xml
    - yaml                       # Multiple formats
  filename: armonite-results
```

**Result File Naming:**
Files are named: `{filename}-{timestamp}.{format}`
Example: `armonite-results-20240618-143022.json`

### Defaults Configuration

Default values for test runs and agent behavior.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `concurrency` | int | `100` | Default number of concurrent requests per agent |
| `duration` | string | `"1m"` | Default test duration |
| `broadcast_interval` | string | `"5s"` | How often coordinator broadcasts status |
| `telemetry_interval` | string | `"5s"` | How often agents send telemetry |
| `keep_alive` | bool | `true` | Keep HTTP connections alive between requests |
| `min_agents` | int | `1` | Minimum agents required to start a test |

**Examples:**
```yaml
defaults:
  concurrency: 200            # Higher default load
  duration: 5m                # Longer default tests
  broadcast_interval: 10s     # Less frequent broadcasts
  telemetry_interval: 2s      # More frequent telemetry
  keep_alive: false           # Disable connection reuse
  min_agents: 3               # Require multiple agents
```

```yaml
defaults:
  concurrency: 50             # Conservative load
  duration: 30s               # Quick tests
  broadcast_interval: 1s      # Frequent updates
  telemetry_interval: 1s      # Real-time telemetry
  keep_alive: true            # Efficient connections
  min_agents: 1               # Single agent OK
```

## CLI Flag Overrides

Most configuration options can be overridden with command-line flags:

```bash
# Override server settings
armonite coordinator --host 192.168.1.100 --port 4223 --http-port 9000

# Override logging
armonite coordinator --log-level debug --log-format json

# Override agent defaults
armonite agent --concurrency 200 --keep-alive=false
```

## Environment Variables

Some settings can be controlled via environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `ARMONITE_CONFIG` | Configuration file path | `/etc/armonite/config.yaml` |
| `ARMONITE_LOG_LEVEL` | Log level override | `debug` |
| `ARMONITE_DATABASE_DSN` | Database connection override | `./data/test.db` |

## Configuration Examples

### High-Performance Setup
```yaml
server:
  host: 0.0.0.0
  port: 4222
  http_port: 8080
  enable_ui: true

database:
  dsn: "./armonite.db"
  max_open: 100           # High connection limit
  max_idle: 25
  max_lifetime: "30m"

logging:
  level: warn             # Reduce log noise
  format: json
  file: "/var/log/armonite.log"

defaults:
  concurrency: 500        # High default load
  duration: 10m
  broadcast_interval: 10s # Less frequent updates
  telemetry_interval: 5s
  keep_alive: true
  min_agents: 2
```

### Development Setup
```yaml
server:
  host: 127.0.0.1        # Localhost only
  port: 4222
  http_port: 8081
  enable_ui: true

database:
  dsn: "./dev-armonite.db"
  max_open: 10
  max_idle: 3
  max_lifetime: "1h"

logging:
  level: debug           # Verbose logging
  format: text           # Human-readable
  file: ""               # Console output

output:
  directory: ./dev-results
  formats:
    - json
    - csv
  filename: dev-test

defaults:
  concurrency: 10        # Light load
  duration: 30s          # Quick tests
  broadcast_interval: 1s # Frequent updates
  telemetry_interval: 1s
  keep_alive: true
  min_agents: 1
```

### Production Setup
```yaml
server:
  host: 0.0.0.0
  port: 4222
  http_port: 80
  enable_ui: false       # API only

database:
  dsn: "/var/lib/armonite/production.db"
  max_open: 50
  max_idle: 10
  max_lifetime: "2h"

logging:
  level: info
  format: json           # Structured logging
  file: "/var/log/armonite/armonite.log"

output:
  directory: /var/lib/armonite/results
  formats:
    - json
  filename: production-results

defaults:
  concurrency: 1000      # High load capacity
  duration: 15m
  broadcast_interval: 30s # Minimal coordinator overhead
  telemetry_interval: 10s
  keep_alive: true
  min_agents: 5          # Require distributed load
```

## Configuration Validation

Armonite validates configuration on startup and will report errors for:

- Invalid port numbers (must be 1-65535)
- Invalid log levels or formats
- Invalid output formats
- Invalid duration formats
- Missing or invalid directories
- Invalid concurrency values (must be ≥ 1)

## Generating Configuration

Create a default configuration file:

```bash
armonite config generate > armonite.yaml
```

This creates a fully commented configuration with all default values.

## Best Practices

1. **Use version control** for configuration files
2. **Set appropriate log levels** (debug for development, info/warn for production)
3. **Choose output directory carefully** (ensure disk space and permissions)
4. **Test configuration changes** in development first
5. **Monitor database size** and set up rotation if needed
6. **Use JSON logging format** for production (better for log aggregation)
7. **Adjust connection pools** based on expected load and system resources
8. **Set min_agents appropriately** for distributed testing requirements