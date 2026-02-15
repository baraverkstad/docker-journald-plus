# Docker Journald Plus

A Docker journald log driver plugin that adds multiline message merging and log
priority parsing.

Implemented as a Docker managed plugin (v2), installed via `docker plugin install`.

## Features

- **Multiline merging** -- consecutive log lines are merged into single journal
  entries based on configurable patterns
- **Priority detection** -- log priority is inferred from message content using
  sd-daemon `<N>` prefixes and configurable regex patterns
- **JSON log parsing** -- optional structured log parsing to extract level,
  message, and custom fields from JSON-formatted logs
- **All built-in journald fields** -- writes the same container metadata fields
  as the built-in driver (CONTAINER_ID, CONTAINER_NAME, IMAGE_NAME, etc.)
- **Pure Go** -- no CGO required; writes to journald via the native socket protocol

## Installation

```bash
docker plugin install baraverkstad/journald-plus:latest
```

## Usage

```bash
docker run --name myapp \
  --log-driver baraverkstad/journald-plus \
  --log-opt strip-timestamp=true \
  myimage
```

Or in `docker-compose.yml`:

```yaml
services:
  app:
    image: myapp:latest
    logging:
      driver: baraverkstad/journald-plus
      options:
        strip-timestamp: "true"
```

Or set as default in `/etc/docker/daemon.json`:

```json
{
  "log-driver": "baraverkstad/journald-plus",
  "log-opts": {
    "strip-timestamp": "true"
  }
}
```

## Configuration Options

### Options inherited from built-in journald driver

| Option | Description |
|--------|-------------|
| `tag` | Template for SYSLOG_IDENTIFIER. Default: `{{.Name}}` (container name). Supports Go templates -- see below. |
| `labels` | Comma-separated list of container label keys to include as journal fields. |
| `labels-regex` | Regex matching container label keys to include. |
| `env` | Comma-separated list of container env var keys to include as journal fields. |
| `env-regex` | Regex matching container env var keys to include. |

### Field extraction options

| Option | Description |
|--------|-------------|
| `field-FIELDNAME` | Extract data from log messages into a custom journald field. The option name specifies the field name (e.g., `field-REQUEST_ID`). The option value is a regex pattern with a capture group `(...)`. The first capture group's value is extracted. Multiple field extractors can be specified. |

**Tag template variables:**

| Variable | Description | Example |
|----------|-------------|---------|
| `{{.Name}}` | Container name | `mycontainer` |
| `{{.ID}}` | Short container ID (12 chars) | `abcdef123456` |
| `{{.FullID}}` | Full container ID | `abcdef123456...` |
| `{{.ImageName}}` | Image name | `nginx:latest` |
| `{{.ImageID}}` | Short image ID (12 chars) | `deadbeef1234` |
| `{{.ImageFullID}}` | Full image ID | `sha256:deadbeef...` |
| `{{.Command}}` | Entrypoint + args | `nginx -g daemon off` |
| `{{.DaemonName}}` | Docker daemon name | `docker` |

Example: `--log-opt tag="{{.ImageName}}/{{.Name}}"`

Note: the built-in journald driver defaults tag to `{{.ID}}` (short container ID).
This plugin defaults to `{{.Name}}` (container name), which is more useful with
`journalctl -t`.

**Field extraction examples:**

Extract request ID from logs:
```bash
--log-opt field-REQUEST_ID='request_id=([a-z0-9]+)'
```

Extract multiple fields:
```bash
--log-opt field-REQUEST_ID='request_id=([a-z0-9]+)' \
--log-opt field-USER_ID='user=(\d+)' \
--log-opt field-TRACE_ID='trace[:\s]+([a-f0-9]{32})'
```

Query with journalctl:
```bash
journalctl REQUEST_ID=abc123
journalctl USER_ID=42
```

In `/etc/docker/daemon.json`:
```json
{
  "log-driver": "baraverkstad/journald-plus",
  "log-opts": {
    "field-REQUEST_ID": "request_id=([a-z0-9]+)",
    "field-USER_ID": "user=(\\d+)"
  }
}
```

Note: In JSON, backslashes must be escaped (`\\d` instead of `\d`).

### Multiline options

| Option | Default | Description |
|--------|---------|-------------|
| `multiline-regex` | `^\s` | Regex matching **continuation** lines. Lines matching this pattern are appended to the previous message. Set to empty string to disable multiline merging. |
| `multiline-timeout` | `10ms` | Max time to wait for continuation lines before flushing the buffer. Parsed as a Go duration (e.g. `10ms`, `100ms`, `1s`). |
| `multiline-max-lines` | `100` | Maximum number of lines to merge into a single journal entry. Safety limit to prevent unbounded buffering. |
| `multiline-max-bytes` | `1048576` | Maximum total bytes of a merged message (default 1MB). |
| `multiline-separator` | `\n` | String inserted between merged lines. Default is newline. |

### Priority options

| Option | Default | Description |
|--------|---------|-------------|
| `priority-prefix` | `true` | Parse sd-daemon `<N>` prefix at start of log lines (where N is 0-7). The prefix is stripped from the message before writing to journal. See [sd-daemon(3)](https://www.freedesktop.org/software/systemd/man/latest/sd-daemon.html). |
| `priority-default-stdout` | `info` | Default priority for stdout messages. |
| `priority-default-stderr` | `err` | Default priority for stderr messages. |
| `priority-match-emerg` | *(none)* | Regex: if the first line of a message matches, set priority to EMERG (0). |
| `priority-match-alert` | *(none)* | Regex: if the first line matches, set priority to ALERT (1). |
| `priority-match-crit` | `^.{0,30}(CRITICAL\|\[Critical\])` | Regex: if the first line matches, set priority to CRIT (2). Allows up to 30 chars prefix. |
| `priority-match-err` | `^.{0,30}(ERROR\|FATAL\|\[ERROR\]\|\[Fatal\])` | Regex: if the first line matches, set priority to ERR (3). Allows up to 30 chars prefix. |
| `priority-match-warning` | `^.{0,30}(WARN\|WARNING\|\[Warning\])` | Regex: if the first line matches, set priority to WARNING (4). Allows up to 30 chars prefix. |
| `priority-match-notice` | `^.{0,30}\[Note\]` | Regex: if the first line matches, set priority to NOTICE (5). Allows up to 30 chars prefix. |
| `priority-match-info` | *(none)* | Regex: if the first line matches, set priority to INFO (6). |
| `priority-match-debug` | `^.{0,30}(DEBUG\|\[Debug\])` | Regex: if the first line matches, set priority to DEBUG (7). Allows up to 30 chars prefix. |

Priority is resolved in this order (first match wins):
1. `<N>` sd-daemon prefix (if `priority-prefix=true`)
2. `priority-match-*` regex patterns (checked from emerg to debug)
3. Default based on source (`priority-default-stdout` / `priority-default-stderr`)

### Priority names

The `priority-default-stdout` and `priority-default-stderr` options accept
these values: `emerg`, `alert`, `crit`, `err`, `warning`, `notice`, `info`, `debug`.

### Timestamp stripping (experimental)

| Option | Default | Description |
|--------|---------|-------------|
| `strip-timestamp` | `false` | Strip leading timestamps from log messages. Since journald records its own timestamps, application-level timestamps are often redundant. |
| `strip-timestamp-regex` | *(built-in)* | Override the built-in timestamp patterns with a custom regex. Only used when `strip-timestamp=true`. |

When enabled, timestamps are stripped **before** priority detection. The default
priority patterns allow up to 30 characters prefix, which handles cases where
timestamp stripping leaves behind other prefixes. For example, MariaDB logs like
`2026-02-15 15:15:16 0 [Note] InnoDB:...` become ` 0 [Note] InnoDB:...` after
timestamp stripping, and the `[Note]` pattern will still match.

Built-in patterns recognize these formats:

| Format | Example |
|--------|---------|
| ISO 8601 | `2024-01-15T10:30:45.123Z`, `2024-01-15 10:30:45,123 UTC` |
| Go log | `2024/01/15 10:30:45` |
| Syslog | `Jan 15 10:30:45` |
| Apache/nginx CLF | `15/Oct/2024:10:30:45 +0200` |
| Log4j DATE | `14 Nov 2017 20:30:20,434` |
| Apache error | `Wed Oct 15 19:41:46.123456 2019` |

Trailing separators (whitespace, `-`, `|`, `:`) after the timestamp are also
stripped. Timezone abbreviations are limited to Z/UTC/GMT to avoid accidentally
matching log level words like ERROR or WARN.

### JSON log parsing (experimental)

| Option | Default | Description |
|--------|---------|-------------|
| `parse-json` | `false` | Parse log lines as JSON objects and extract structured fields. |
| `json-level-keys` | `level,severity,log_level` | Comma-separated list of JSON keys to check for log level/priority (first match wins). |
| `json-message-keys` | `message,msg,log` | Comma-separated list of JSON keys to extract as the message body (first match wins). |

When `parse-json=true`, the driver attempts to parse each log line as a JSON object:

1. **Level extraction** -- Checks `json-level-keys` (in order) and maps the value to a syslog priority
2. **Message extraction** -- Checks `json-message-keys` (in order) and uses the value as MESSAGE
3. **Field flattening** -- Remaining fields are added to journald with `JSON_` prefix
4. **Graceful fallback** -- If parsing fails or no message key is found, the original line is used

**Supported level mappings:**

| JSON Level | Syslog Priority |
|------------|-----------------|
| `debug`, `trace` | DEBUG (7) |
| `info`, `information` | INFO (6) |
| `notice` | NOTICE (5) |
| `warn`, `warning` | WARNING (4) |
| `error`, `err` | ERR (3) |
| `fatal`, `critical`, `crit` | CRIT (2) |
| `panic`, `alert` | ALERT (1) |
| `emerg`, `emergency` | EMERG (0) |

Level strings are case-insensitive.

**JSON parsing examples:**

Basic usage with default keys:
```bash
docker run --log-driver baraverkstad/journald-plus \
  --log-opt parse-json=true \
  myapp
```

Your application logs JSON:
```json
{"level":"error","message":"database connection failed","request_id":"abc123","retry_count":3}
```

Results in journald fields:
- `MESSAGE=database connection failed`
- `PRIORITY=3` (ERR)
- `JSON_REQUEST_ID=abc123`
- `JSON_RETRY_COUNT=3`

Query with journalctl:
```bash
journalctl JSON_REQUEST_ID=abc123
journalctl -p err  # Show all ERROR and above
```

Custom key names for non-standard JSON formats:
```bash
docker run --log-driver baraverkstad/journald-plus \
  --log-opt parse-json=true \
  --log-opt json-level-keys='lvl,severity' \
  --log-opt json-message-keys='text,body' \
  myapp
```

With structured logging frameworks (e.g., OpenTelemetry):
```json
{"severity":"INFO","body":"Request processed","trace_id":"a1b2c3","span_id":"x9y8z7","duration_ms":42.5}
```

Results in:
- `MESSAGE=Request processed`
- `PRIORITY=6` (INFO)
- `JSON_TRACE_ID=a1b2c3`
- `JSON_SPAN_ID=x9y8z7`
- `JSON_DURATION_MS=42.5`

**Notes:**
- Field names are sanitized for journald compatibility (uppercase, special chars replaced with `_`)
- Nested JSON objects/arrays are serialized as JSON strings
- Null values are omitted
- If JSON parsing fails, the original line is logged as-is (no data loss)
- Zero overhead when disabled (single boolean check)

## Journal Fields

Each log entry is written to journald with the following fields:

| Field | Description |
|-------|-------------|
| `MESSAGE` | The log message content (after multiline merge and prefix stripping) |
| `PRIORITY` | Numeric syslog priority (0-7) |
| `SYSLOG_IDENTIFIER` | The tag value |
| `SYSLOG_TIMESTAMP` | RFC 3339 timestamp from Docker |
| `CONTAINER_ID` | Short (12-char) container ID |
| `CONTAINER_ID_FULL` | Full container ID |
| `CONTAINER_NAME` | Container name |
| `CONTAINER_TAG` | Formatted tag |
| `IMAGE_NAME` | Container image name |

Plus any fields from:
- `labels`, `labels-regex` options (container labels)
- `env`, `env-regex` options (environment variables)
- `field-*` options (extracted from log messages via regex)
- `parse-json` option (JSON fields with `JSON_` prefix)

## Architecture

This is a Docker managed plugin (v2). It communicates with the Docker daemon
over the standard log driver plugin protocol:

1. Docker creates a FIFO per container and calls `StartLogging` with the FIFO path
2. The plugin reads protobuf-encoded `LogEntry` messages from the FIFO
3. Partial messages (lines >16KB) are reassembled
4. Multiline merging is applied based on the continuation regex and timeout
5. Priority is determined from message content
6. The merged, prioritized message is written to journald via the native socket

The plugin requires the host's journald socket to be mounted into its rootfs
(`/run/systemd/journal/socket`).

`docker logs` is **not** supported -- use `journalctl` to read logs:

```bash
journalctl -t myapp                   # by tag (container name or custom tag)
journalctl -t myapp -p warning        # only warnings and above
journalctl -t myapp -f                # follow (like tail -f)
journalctl CONTAINER_ID=abc123def456  # by container ID
```

## Development

See **DEVELOPMENT.md** for build instructions, testing, and development workflow.

## License

This project is licensed under the **MIT License**. See the [LICENSE](LICENSE) for details.

Copyright (c) 2026 Per Cederberg
