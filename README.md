# Docker Journald Plus

A Docker log driver plugin for systemd journald that improves upon the built-in
journald driver by adding multiline message merging and log priority detection.

Implemented as a Docker managed plugin (v2), installed via `docker plugin install`.

## Features

- **Multiline merging** -- consecutive log lines are merged into single journal
  entries based on configurable patterns
- **Priority detection** -- log priority is inferred from message content using
  sd-daemon `<N>` prefixes and configurable regex patterns
- **All built-in journald fields** -- writes the same container metadata fields
  as the built-in driver (CONTAINER_ID, CONTAINER_NAME, IMAGE_NAME, etc.)
- **Pure Go** -- no CGO required; writes to journald via the native socket protocol

## Installation

```bash
docker plugin install youruser/journald-plus:latest
```

## Usage

```bash
docker run --log-driver journald-plus \
  --log-opt multiline-regex="^\s" \
  --log-opt priority-prefix=true \
  --log-opt priority-match-warning="^WARN|^\[WARNING\]" \
  myimage
```

Or set as default in `/etc/docker/daemon.json`:

```json
{
  "log-driver": "journald-plus",
  "log-opts": {
    "multiline-regex": "^\\s",
    "priority-prefix": "true"
  }
}
```

## Configuration Options

### Options inherited from built-in journald driver

| Option | Description |
|--------|-------------|
| `tag` | Template for SYSLOG_IDENTIFIER. Default: first 12 chars of container ID. Supports Go templates (e.g. `{{.Name}}`). |
| `labels` | Comma-separated list of container label keys to include as journal fields. |
| `labels-regex` | Regex matching container label keys to include. |
| `env` | Comma-separated list of container env var keys to include as journal fields. |
| `env-regex` | Regex matching container env var keys to include. |

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
| `priority-match-crit` | *(none)* | Regex: if the first line matches, set priority to CRIT (2). |
| `priority-match-err` | *(none)* | Regex: if the first line matches, set priority to ERR (3). |
| `priority-match-warning` | *(none)* | Regex: if the first line matches, set priority to WARNING (4). |
| `priority-match-notice` | *(none)* | Regex: if the first line matches, set priority to NOTICE (5). |
| `priority-match-info` | *(none)* | Regex: if the first line matches, set priority to INFO (6). |
| `priority-match-debug` | *(none)* | Regex: if the first line matches, set priority to DEBUG (7). |

Priority is resolved in this order (first match wins):
1. `<N>` sd-daemon prefix (if `priority-prefix=true`)
2. `priority-match-*` regex patterns (checked from emerg to debug)
3. Default based on source (`priority-default-stdout` / `priority-default-stderr`)

### Priority names

The `priority-default-stdout` and `priority-default-stderr` options accept
these values: `emerg`, `alert`, `crit`, `err`, `warning`, `notice`, `info`, `debug`.

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

Plus any fields from `labels`, `labels-regex`, `env`, `env-regex` options.

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
journalctl CONTAINER_NAME=mycontainer
journalctl CONTAINER_ID=abc123def456
```

## Building

```bash
make build        # Build the Go binary
make plugin       # Build and create the Docker plugin
make enable       # Enable the plugin
make push         # Push to Docker Hub
```

## License

TBD
