# Docker Journald Plus

A Docker journald log driver plugin that adds multiline message merging and log
priority parsing.

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
docker plugin install baraverkstad/journald-plus:latest
```

## Usage

```bash
docker run --name myapp \
  --log-driver journald-plus \
  --log-opt strip-timestamp=true \
  myimage
```

Or set as default in `/etc/docker/daemon.json`:

```json
{
  "log-driver": "journald-plus",
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
| `priority-match-crit` | `^CRITICAL\|^\[Critical\]` | Regex: if the first line matches, set priority to CRIT (2). |
| `priority-match-err` | `^ERROR\|^FATAL\|^\[ERROR\]\|^\[Fatal\]` | Regex: if the first line matches, set priority to ERR (3). |
| `priority-match-warning` | `^WARN\|^WARNING\|^\[Warning\]` | Regex: if the first line matches, set priority to WARNING (4). |
| `priority-match-notice` | `^\[Note\]` | Regex: if the first line matches, set priority to NOTICE (5). |
| `priority-match-info` | *(none)* | Regex: if the first line matches, set priority to INFO (6). |
| `priority-match-debug` | `^DEBUG\|^\[Debug\]` | Regex: if the first line matches, set priority to DEBUG (7). |

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

When enabled, timestamps are stripped **before** priority detection, so patterns
like `^ERROR` work even when the original line was `2024-01-15 10:30:45 ERROR ...`.

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
