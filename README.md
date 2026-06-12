# Docker Journald Plus

A Docker journald log driver plugin that adds multiline message merging and log
priority parsing.

Implemented as a Docker managed plugin (v2), installed via
`docker plugin install`. Available on
[Docker Hub](https://hub.docker.com/r/baraverkstad/journald-plus).

Read more: https://github.com/baraverkstad/docker-journald-plus

## Features

- **Multiline merging** — consecutive log lines are merged into single journal
  entries based on configurable patterns
- **Priority detection** — log priority is inferred from message content using
  sd-daemon `<N>` prefixes and configurable regex patterns
- **JSON log parsing** — optional structured log parsing to extract level,
  message, and custom fields from JSON-formatted logs
- **All built-in journald fields** — writes the same container metadata fields
  as the built-in driver (CONTAINER_ID, CONTAINER_NAME, IMAGE_NAME, etc.)
- **Minimal footprint** — plugin image is 7.4MB on disk (1.7MB compressed)

### journald-plus vs built-in journald driver

| Feature            | Built-in journald  | journald-plus               |
| ------------------ | ------------------ | --------------------------- |
| Metadata fields    | Yes                | Yes                         |
| Tag default        | `{{.ID}}`          | `{{.Name}}`                 |
| Multiline merging  | No                 | Yes                         |
| Priority detection | No                 | Yes                         |
| JSON log parsing   | No                 | Yes                         |
| `docker logs`      | Yes (via journald) | Yes (via local)<sup>1</sup> |

**1:** Docker automatically enables a limited
[dual logging](https://docs.docker.com/engine/logging/dual-logging/) for plugins
without API support for reading logs. It has been omitted in `journald-plus` to
achieve a minimal footprint. Use `journalctl` to access processed log output.

## Output

Each log entry is written to journald with the following fields:

| Field               | Description                                    |
| ------------------- | ---------------------------------------------- |
| `MESSAGE`           | Log message (after merge and prefix stripping) |
| `PRIORITY`          | Numeric syslog priority (0-7)                  |
| `SYSLOG_IDENTIFIER` | The tag value                                  |
| `SYSLOG_TIMESTAMP`  | RFC 3339 timestamp from Docker                 |
| `CONTAINER_ID`      | Short (12-char) container ID                   |
| `CONTAINER_ID_FULL` | Full container ID                              |
| `CONTAINER_NAME`    | Container name                                 |
| `CONTAINER_TAG`     | Formatted tag                                  |
| `IMAGE_NAME`        | Container image name                           |

Plus any fields from:

- `labels`, `labels-regex` — see [Inherited options](#inherited-options)
- `env`, `env-regex` — see [Inherited options](#inherited-options)
- `field-*` — see [Field extraction](#field-extraction)
- `json-parse` — see [JSON log parsing](#json-log-parsing)

## Usage

```bash
docker run --name myapp \
  --log-driver baraverkstad/journald-plus:[VERSION] \
  --log-opt json-parse=true \
  myimage
```

Or in `docker-compose.yml`:

```yaml
services:
  app:
    image: myapp:latest
    logging:
      driver: baraverkstad/journald-plus:[VERSION]
      options:
        json-parse: "true"
```

Or set as default in `/etc/docker/daemon.json`:

```json
{
  "log-driver": "baraverkstad/journald-plus:[VERSION]",
  "log-opts": {
    "json-parse": "true"
  }
}
```

> 👉 See [Configuration](#configuration) for all options.

### Reading logs

`docker logs` reads from a
[local cache](#journald-plus-vs-built-in-journald-driver) and shows raw output
(no merging or parsing). For full processed entries, use `journalctl`:

```bash
journalctl -t myapp -f                # follow (like tail -f)
journalctl -t myapp -p warning        # warnings and above
journalctl -t myapp --since -1h       # last hour
journalctl CONTAINER_ID=abc123def456  # filter by container ID
journalctl REQUEST_ID=abc123          # custom field from JSON or field-*
```

### Examples

#### JSON log parsing

Basic usage with default keys:

```bash
docker run --log-driver baraverkstad/journald-plus:[VERSION] \
  --log-opt json-parse=true \
  myapp
```

Your application logs JSON:

```json
{
  "level": "error",
  "message": "database connection failed",
  "request_id": "abc123",
  "retry_count": 3
}
```

Results in:

- `MESSAGE=database connection failed`
- `PRIORITY=3` (ERR)
- `JSON_REQUEST_ID=abc123`
- `JSON_RETRY_COUNT=3`

Custom key names for non-standard formats (e.g. OpenTelemetry):

```bash
--log-opt json-level-keys='severity,lvl' \
--log-opt json-message-keys='body,text'
```

Skip high-cardinality fields and keep remaining fields inline in the message:

```bash
--log-opt json-skip-keys='ts,time,@timestamp' \
--log-opt json-extra=inline
```

#### Field extraction

Extract journal fields from log messages using regex capture groups:

```bash
docker run --log-driver baraverkstad/journald-plus:[VERSION] \
  --log-opt field-REQUEST_ID='request_id=([a-z0-9]+)' \
  --log-opt field-USER_ID='user=(\d+)' \
  --log-opt field-TRACE_ID='trace[:\s]+([a-f0-9]{32})' \
  myapp
```

Per-service in `docker-compose.yml`:

```yaml
services:
  api:
    image: myapi:latest
    logging:
      driver: baraverkstad/journald-plus:[VERSION]
      options:
        field-REQUEST_ID: "request_id=([a-z0-9]+)"
        field-USER_ID: 'user=(\d+)'
  worker:
    image: myworker:latest
    logging:
      driver: baraverkstad/journald-plus:[VERSION]
      options:
        field-JOB_ID: "job=([0-9]+)"
```

In `/etc/docker/daemon.json` (backslashes must be escaped):

```json
{
  "log-driver": "baraverkstad/journald-plus:[VERSION]",
  "log-opts": {
    "field-REQUEST_ID": "request_id=([a-z0-9]+)",
    "field-USER_ID": "user=(\\d+)"
  }
}
```

## Installation

```bash
ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
docker plugin install --alias baraverkstad/journald-plus:[VERSION] \
  baraverkstad/journald-plus:[VERSION]-$ARCH
```

The alias (`baraverkstad/journald-plus:[VERSION]`) is what you reference in
`daemon.json`, `compose.yml`, or `--log-driver`. Never reference the
arch-specific tag directly in config -- only the alias is portable across
machines.

> 👉 No `latest` tag is published. Use `latest-amd64` or `latest-arm64` if you
> want the latest build without pinning a version. But be aware that doing so
> makes upgrading difficult.

### Upgrading

A plugin in use by running containers cannot be disabled or upgraded in-place.
The versioned alias pattern lets two plugin versions coexist for zero-downtime
migration:

1. Install the new version under a new alias:
   ```bash
   ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
   docker plugin install --alias baraverkstad/journald-plus:[NEW-VERSION] \
     baraverkstad/journald-plus:[NEW-VERSION]-$ARCH
   ```
2. Update `daemon.json` or `compose.yml` to reference the new alias.
3. If using `daemon.json`, restart `dockerd` to pick up the new default driver.
4. Recreate services to switch them to the new alias one at a time.
5. Once no containers reference the old alias, remove it:
   ```bash
   docker plugin disable baraverkstad/journald-plus:[OLD-VERSION]
   docker plugin rm baraverkstad/journald-plus:[OLD-VERSION]
   ```

## Configuration

| Option                    | Default                    | Section                                     |
| ------------------------- | -------------------------- | ------------------------------------------- |
| `tag`                     | `{{.Name}}`                | [Inherited options](#inherited-options)     |
| `labels`                  | _(none)_                   | [Inherited options](#inherited-options)     |
| `labels-regex`            | _(none)_                   | [Inherited options](#inherited-options)     |
| `env`                     | _(none)_                   | [Inherited options](#inherited-options)     |
| `env-regex`               | _(none)_                   | [Inherited options](#inherited-options)     |
| `field-FIELDNAME`         | _(none)_                   | [Field extraction](#field-extraction)       |
| `multiline-regex`         | `^\s`                      | [Multiline](#multiline)                     |
| `multiline-timeout`       | `10ms`                     | [Multiline](#multiline)                     |
| `multiline-max-lines`     | `100`                      | [Multiline](#multiline)                     |
| `multiline-max-bytes`     | `1048576`                  | [Multiline](#multiline)                     |
| `multiline-separator`     | `\n`                       | [Multiline](#multiline)                     |
| `priority-prefix`         | `true`                     | [Priority](#priority)                       |
| `priority-default-stdout` | `info`                     | [Priority](#priority)                       |
| `priority-default-stderr` | `err`                      | [Priority](#priority)                       |
| `priority-match-emerg`    | _(none)_                   | [Priority](#priority)                       |
| `priority-match-alert`    | _(none)_                   | [Priority](#priority)                       |
| `priority-match-crit`     | _(regex)_                  | [Priority](#priority)                       |
| `priority-match-err`      | _(regex)_                  | [Priority](#priority)                       |
| `priority-match-warning`  | _(regex)_                  | [Priority](#priority)                       |
| `priority-match-notice`   | _(none)_                   | [Priority](#priority)                       |
| `priority-match-info`     | _(none)_                   | [Priority](#priority)                       |
| `priority-match-debug`    | _(regex)_                  | [Priority](#priority)                       |
| `strip-timestamp`         | `false`                    | [Timestamp stripping](#timestamp-stripping) |
| `strip-timestamp-regex`   | _(regex)_                  | [Timestamp stripping](#timestamp-stripping) |
| `strip-priority`          | `false`                    | [Priority stripping](#priority-stripping)   |
| `strip-priority-regex`    | _(regex)_                  | [Priority stripping](#priority-stripping)   |
| `normalize-whitespace`    | `false`                    | [Whitespace](#whitespace)                   |
| `json-parse`              | `false`                    | [JSON log parsing](#json-log-parsing)       |
| `json-level-keys`         | `level,severity,log_level` | [JSON log parsing](#json-log-parsing)       |
| `json-message-keys`       | `message,msg,log`          | [JSON log parsing](#json-log-parsing)       |
| `json-skip-keys`          | _(none)_                   | [JSON log parsing](#json-log-parsing)       |
| `json-extra`              | `fields`                   | [JSON log parsing](#json-log-parsing)       |

### Inherited options

| Option         | Default     | Description                                              |
| -------------- | ----------- | -------------------------------------------------------- |
| `tag`          | `{{.Name}}` | Template for `SYSLOG_IDENTIFIER`; supports Go templates. |
| `labels`       | _(none)_    | Container label keys to include as journal fields.       |
| `labels-regex` | _(none)_    | Container label keys to include (regex).                 |
| `env`          | _(none)_    | Container env var keys to include as journal fields.     |
| `env-regex`    | _(none)_    | Container env var keys to include (regex).               |

**Tag template variables:**

| Variable           | Description                   | Example               |
| ------------------ | ----------------------------- | --------------------- |
| `{{.Name}}`        | Container name                | `mycontainer`         |
| `{{.ID}}`          | Short container ID (12 chars) | `abcdef123456`        |
| `{{.FullID}}`      | Full container ID             | `abcdef123456...`     |
| `{{.ImageName}}`   | Image name                    | `nginx:latest`        |
| `{{.ImageID}}`     | Short image ID (12 chars)     | `deadbeef1234`        |
| `{{.ImageFullID}}` | Full image ID                 | `sha256:deadbeef...`  |
| `{{.Command}}`     | Entrypoint + args             | `nginx -g daemon off` |
| `{{.DaemonName}}`  | Docker daemon name            | `docker`              |

Example: `--log-opt tag="{{.ImageName}}/{{.Name}}"`

> 👉 The built-in journald driver defaults tag to `{{.ID}}` (short container
> ID). This plugin defaults to `{{.Name}}` (container name), which is more
> useful with `journalctl -t`.

**Field name sanitization:**

Label and environment variable names are converted to journald-compatible field
names: lowercase letters become uppercase, non-alphanumeric characters become
`_`, leading digits and underscores are trimmed (journald rejects them), and
names are capped at 64 characters. For example, a label `com.example.my-app`
becomes the journal field `COM_EXAMPLE_MY_APP`.

### Field extraction

| Option            | Description                                                  |
| ----------------- | ------------------------------------------------------------ |
| `field-FIELDNAME` | Extract a journal field via regex capture group.<sup>1</sup> |

**1:** The option name is the field name (e.g. `field-REQUEST_ID`); the value is
a regex with one capture group `(...)`. Multiple `field-*` options can be used.
Names are uppercased automatically and must use only letters, digits and `_`
(max 64 chars, no leading digit or `_`). Reserved names (`MESSAGE`, `PRIORITY`,
`IMAGE_NAME`, `SYSLOG_*`, `CONTAINER_*`) are rejected.

> 👉 See [Examples](#examples) for usage.

### Multiline

| Option                | Default   | Description                                      |
| --------------------- | --------- | ------------------------------------------------ |
| `multiline-regex`     | `^\s`     | Regex matching continuation lines.<sup>1</sup>   |
| `multiline-timeout`   | `10ms`    | Max wait before flush (Go duration).<sup>2</sup> |
| `multiline-max-lines` | `100`     | Max lines to merge into one entry.               |
| `multiline-max-bytes` | `1048576` | Max bytes for a merged entry (1 MB).             |
| `multiline-separator` | `\n`      | String inserted between merged lines.            |

**1:** Lines matching this pattern are appended to the previous message. Set to
empty string to disable multiline merging. \
**2:** Accepts Go duration format: `10ms`, `100ms`, `1s`.

The default `^\s` pattern matches any line beginning with whitespace, which
handles Java stack traces (`at com.example.Foo` continuation lines are indented)
and Python tracebacks (frame lines start with spaces) out of the box.

### Priority

| Option                    | Default               | Description                               |
| ------------------------- | --------------------- | ----------------------------------------- |
| `priority-prefix`         | `true`                | sd-daemon `<N>` prefix (0-7).<sup>1</sup> |
| `priority-default-stdout` | `info`                | Default priority for stdout.              |
| `priority-default-stderr` | `err`                 | Default priority for stderr.              |
| `priority-match-emerg`    | _(none)_              | Match first line -> EMERG (0).            |
| `priority-match-alert`    | _(none)_              | Match first line -> ALERT (1).            |
| `priority-match-crit`     | \_(regex)<sup>2</sup> | Match first line -> CRIT (2).             |
| `priority-match-err`      | \_(regex)<sup>2</sup> | Match first line -> ERR (3).              |
| `priority-match-warning`  | \_(regex)<sup>2</sup> | Match first line -> WARNING (4).          |
| `priority-match-notice`   | _(none)_              | Match first line -> NOTICE (5).           |
| `priority-match-info`     | _(none)_              | Match first line -> INFO (6).             |
| `priority-match-debug`    | \_(regex)<sup>2</sup> | Match first line -> DEBUG (7).            |

**1:** The `<N>` prefix is stripped from MESSAGE before writing to journal. See
[sd-daemon(3)](https://www.freedesktop.org/software/systemd/man/latest/sd-daemon.html).
\
**2:** Default regex patterns (all allow up to 30 chars of prefix before
keyword):

- `priority-match-crit`: `^.{0,30}(CRITICAL|\[Critical\])`
- `priority-match-err`: `^.{0,30}(ERROR|FATAL|\[ERROR\]|\[Fatal\])`
- `priority-match-warning`: `^.{0,30}(WARN|WARNING|\[Warning\])`
- `priority-match-debug`: `^.{0,30}(DEBUG|\[Debug\])`

Priority is resolved in this order (first match wins):

1. `<N>` sd-daemon prefix (if `priority-prefix=true`)
2. `priority-match-*` regex patterns (checked from emerg to debug)
3. Default: `priority-default-stdout` or `priority-default-stderr`

### Priority names

The `priority-default-stdout` and `priority-default-stderr` options accept these
values: `emerg`, `alert`, `crit`, `err`, `warning`, `notice`, `info`, `debug`.

### Timestamp stripping

| Option                  | Default   | Description                           |
| ----------------------- | --------- | ------------------------------------- |
| `strip-timestamp`       | `false`   | Strip leading timestamps.<sup>1</sup> |
| `strip-timestamp-regex` | _(regex)_ | Override built-in regex.<sup>2</sup>  |

**1:** Journald records its own timestamps; application-level ones are
redundant. \
**2:** Only used when `strip-timestamp=true`.

When enabled, timestamps are stripped **before** priority detection. The default
priority patterns allow up to 30 characters prefix, which handles cases where
timestamp stripping leaves behind other prefixes. For example, MariaDB logs like
`2026-02-15 15:15:16 0 [Warning] InnoDB:...` become `0 [Warning] InnoDB:...`
after timestamp stripping, and the `[Warning]` pattern will still match.

Built-in patterns recognize these formats:

| Format           | Example                                           |
| ---------------- | ------------------------------------------------- |
| ISO 8601         | `2024-01-15T10:30:45Z`, `2024-01-15 10:30:45 UTC` |
| Go log           | `2024/01/15 10:30:45`                             |
| Syslog           | `Jan 15 10:30:45`                                 |
| Apache/nginx CLF | `15/Oct/2024:10:30:45 +0200`                      |
| Log4j DATE       | `14 Nov 2017 20:30:20,434`                        |
| Apache error     | `Wed Oct 15 19:41:46.123456 2019`                 |

Trailing separators (whitespace, `-`, `|`, `:`) after the timestamp are also
stripped. Timezone abbreviations are limited to Z/UTC/GMT to avoid accidentally
matching log level words like ERROR or WARN.

### Priority stripping

| Option                 | Default   | Description                          |
| ---------------------- | --------- | ------------------------------------ |
| `strip-priority`       | `false`   | Strip leading log level.<sup>1</sup> |
| `strip-priority-regex` | _(regex)_ | Override built-in regex.<sup>2</sup> |

**1:** Having log level in both the PRIORITY field and MESSAGE is redundant. \
**2:** Only used when `strip-priority=true`.

When enabled, the leading log level keyword is stripped **after** priority
detection. The default regex matches common level keywords at the start of the
message, optionally bracketed:

```
(?i)^\[?(trace|debug|info|notice|note|warning|warn|critical|error|fatal|alert|emerg)\]?
```

Trailing separators (whitespace, `-`, `|`, `:`) after the match are also
stripped. Because the regex is anchored to `^`, only the first line of a
multiline-merged message is affected; continuation lines are never touched.

| Input                        | After stripping      |
| ---------------------------- | -------------------- |
| `INFO request completed`     | `request completed`  |
| `[Error] connection refused` | `connection refused` |
| `WARN: disk space low`       | `disk space low`     |

### Whitespace

| Option                 | Default | Description                                     |
| ---------------------- | ------- | ----------------------------------------------- |
| `normalize-whitespace` | `false` | Normalize tabs and repeated spaces.<sup>1</sup> |

**1:** Any sequence of one or more tabs (or two or more consecutive spaces) is
replaced with a single space. Applied last in the pipeline, after timestamp
stripping and priority detection/stripping.

| Input                       | After normalization        |
| --------------------------- | -------------------------- |
| `INFO\t\tStarting server`   | `INFO Starting server`     |
| `ERROR  connection refused` | `ERROR connection refused` |
| `[WARN]\t  disk space low`  | `[WARN] disk space low`    |

### JSON log parsing

| Option              | Default                    | Description                                 |
| ------------------- | -------------------------- | ------------------------------------------- |
| `json-parse`        | `false`                    | Parse log lines as JSON.                    |
| `json-level-keys`   | `level,severity,log_level` | JSON keys for log level (first match).      |
| `json-message-keys` | `message,msg,log`          | JSON keys for message body (first match).   |
| `json-skip-keys`    | _(none)_                   | JSON keys to ignore entirely.               |
| `json-extra`        | `fields`                   | How to handle remaining fields.<sup>1</sup> |

**1:** `fields` (default): stored as `JSON_*` journal fields; `inline`: appended
to MESSAGE.

When `json-parse=true`, the driver attempts to parse each log line as a JSON
object:

1. **Level extraction** — Checks `json-level-keys` (in order) and maps the value
   to a syslog priority
2. **Message extraction** — Checks `json-message-keys` (in order) and uses the
   value as MESSAGE
3. **Skip keys** — Keys in `json-skip-keys` are discarded
4. **Remaining fields** — Stored as `JSON_*` journal fields
   (`json-extra=fields`) or appended to the message as JSON
   (`json-extra=inline`); ignored if empty
5. **Graceful fallback** — If parsing fails or no message key is found, the
   original line is used

**Supported level mappings:**

| JSON Level                  | Syslog Priority |
| --------------------------- | --------------- |
| `debug`, `trace`            | DEBUG (7)       |
| `info`, `information`       | INFO (6)        |
| `notice`                    | NOTICE (5)      |
| `warn`, `warning`           | WARNING (4)     |
| `error`, `err`              | ERR (3)         |
| `fatal`, `critical`, `crit` | CRIT (2)        |
| `panic`, `alert`            | ALERT (1)       |
| `emerg`, `emergency`        | EMERG (0)       |

Level strings are case-insensitive.

> 👉 See [Examples](#examples) for usage.

**Behavior:**

- Field names are sanitized for journald compatibility (uppercase, special chars
  replaced with `_`)
- Nested JSON objects/arrays are serialized as JSON strings
- Null values are omitted
- If JSON parsing fails, the original line is logged as-is (no data loss)
- Zero overhead when disabled (single boolean check)

## Contributing

Ideas, bug reports, and pull requests are welcome. See
[DEVELOPMENT.md](DEVELOPMENT.md) for build instructions, testing, and
development workflow.

## License

Docker Journald Plus is licensed under the [MIT License](LICENSE).

Copyright (c) 2026 Per Cederberg
