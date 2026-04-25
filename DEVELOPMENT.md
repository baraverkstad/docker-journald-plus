# Development Guide

## Quick Start

```bash
make              # List available targets
make clean        # Cleanup build artifacts
make build        # Compile Go binary
make test         # Run tests & code style checks
make publish      # Build multi-arch plugins and push to Docker Hub
```

## Project Layout

| File | Responsibility |
|------|----------------|
| `main.go` | Plugin API entrypoint |
| `driver/http.go` | Minimal HTTP/1.1 server (no `net/http` dependency) |
| `driver/driver.go` | Request handlers, consumer lifecycle, orchestration |
| `driver/config.go` | Parse and validate log-opt options into `Config` struct |
| `driver/journal_metadata.go` | Build journald field list and write entries |
| `driver/journal_socket.go` | Low-level journald socket protocol (pure Go) |
| `driver/multiline.go` | Continuation-line merging with configurable regex/timeout |
| `driver/partial.go` | Reassemble split protobuf log entries |
| `driver/priority.go` | Priority detection (prefix, regex, default) |
| `driver/timestamp.go` | Strip leading timestamps from log lines |
| `driver/json.go` | Parse JSON log lines, map level strings to priorities |
| `driver/proto.go` | Decode protobuf `LogEntry` from FIFO |
| `test/` | Integration test scripts |
| `config.json` | Plugin manifest |
| `tmp/` | Build artifacts |

## Architecture

The plugin implements Docker's managed plugin (v2) log driver interface:

1. Docker creates a FIFO per container and calls `StartLogging` with the FIFO path
2. The plugin reads protobuf-encoded `LogEntry` messages from the FIFO
3. Partial messages (lines >16KB) are reassembled into complete log lines
4. Multiline merging is applied based on the continuation regex and timeout
5. Priority is determined from message content (prefix, regex patterns, default)
6. The merged, prioritized message is written to journald via the native socket protocol

The plugin requires the host's journald socket (`/run/systemd/journal/socket`)
mounted into its rootfs. The consumer per container drains its FIFO fully before
responding to `StopLogging`, ensuring no log lines are dropped on container stop.

## Plugin Testing (on Linux)

Install the plugin from a local build or directly from Docker Hub:

```bash
# From local build
make build test
mkdir -p tmp/local/rootfs/usr/bin
cp tmp/build/journald-plus tmp/local/rootfs/usr/bin/
cp config.json tmp/local/
docker plugin create baraverkstad/journald-plus:latest tmp/local
docker plugin enable baraverkstad/journald-plus:latest

# From Docker Hub (use :<ver>-amd64 or :<ver>-arm64)
docker plugin install baraverkstad/journald-plus:latest-arm64
```

Then run the test scenarios and verify the journald output:

```bash
docker run --rm -i --log-driver baraverkstad/journald-plus:latest --log-opt tag=test alpine:latest sh < test/test-logging.sh
sh test/verify-journal.sh
```

Cleanup when done:

```bash
docker plugin disable baraverkstad/journald-plus:latest
docker plugin rm baraverkstad/journald-plus:latest
```

## Plugin Testing (on macOS via OrbStack)

[OrbStack](https://orbstack.dev) runs a Linux VM that hosts its own Docker daemon and
journald instance. The build step and all commands above must run inside OrbStack so that
Docker and journald share the same environment. Install Go once if needed, then use
`orb run` as a prefix throughout:

```bash
orb run sudo apt install golang-go  # one-time setup
orb run make build test
orb run docker plugin create ...
orb run docker run ...
orb run sh test/verify-journal.sh
```
