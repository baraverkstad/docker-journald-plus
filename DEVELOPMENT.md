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

- `main.go`: Plugin API entrypoint
- `driver/`: Core logic
- `config.json`: Plugin manifest
- `tmp/`: Build artifacts

## Local Plugin Testing

For testing on the same machine where you build:

```bash
# 1. Build plugin rootfs
make build test
mkdir -p tmp/local/rootfs/usr/bin
cp tmp/build/journald-plus tmp/local/rootfs/usr/bin/
cp config.json tmp/local/

# 2. Create and enable the plugin
docker plugin create baraverkstad/journald-plus:latest tmp/local
docker plugin enable baraverkstad/journald-plus:latest

# 3. Test with a container
docker run --rm --log-driver baraverkstad/journald-plus:latest --log-opt tag=test alpine \
  sh -c 'echo "Line 1"; echo "  continuation"; echo "ERROR: failing task"'

# 4. Check journald
journalctl -t test -o json-pretty -n 1

# 5. Cleanup when done
docker plugin disable baraverkstad/journald-plus:latest
docker plugin rm baraverkstad/journald-plus:latest
```

## Integration Testing

To verify end-to-end logging:
```bash
# 1. Install the plugin (use :latest or :latest-arm64)
docker plugin install baraverkstad/journald-plus

# 2. Start a container using the plugin
docker run --rm --log-driver baraverkstad/journald-plus --log-opt tag=test alpine \
  sh -c 'echo "Line 1"; echo "  continuation"; echo "ERROR: failing task"'

# 3. Check journald for the results
journalctl -t test -o json-pretty -n 1
```
