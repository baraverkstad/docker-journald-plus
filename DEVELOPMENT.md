# Development Guide

## Quick Start

```bash
make              # List available targets
make clean        # Cleanup build artifacts
make build        # Compile Go binary
make test         # Run tests & code style checks
make publish      # Build plugin and push to Docker Hub
```

## Local Plugin Testing

For testing on the same machine where you build:

```bash
# 1. Build plugin rootfs
make build-docker

# 2. Create and enable the plugin
docker plugin create baraverkstad/journald-plus:latest tmp/plugin
docker plugin enable baraverkstad/journald-plus:latest

# 3. Test with a container
docker run --rm --log-driver baraverkstad/journald-plus --log-opt tag=test alpine \
  sh -c 'echo "Line 1"; echo "  continuation"; echo "ERROR: failing task"'

# 4. Check journald
journalctl -t test -o json-pretty -n 1

# 5. Cleanup when done
docker plugin disable baraverkstad/journald-plus:latest
docker plugin rm baraverkstad/journald-plus:latest
```

## Plugin Lifecycle

The plugin is a Docker v2 managed plugin. The build process follows:
1. **Build**: `make build-docker` creates the rootfs and `config.json` in `tmp/plugin/`.
2. **Create**: `docker plugin create` packages it as a plugin.
3. **Enable**: `docker plugin enable` makes it available to Docker containers.
4. **Verify**: Run a container with `--log-driver baraverkstad/journald-plus` and check `journalctl`.

## Integration Testing

To verify end-to-end logging:
```bash
# 1. Start a container using the plugin
docker run --rm --log-driver baraverkstad/journald-plus --log-opt tag=test alpine \
  sh -c 'echo "Line 1"; echo "  continuation"; echo "ERROR: failing task"'

# 2. Check journald for the results
journalctl -t test -o json-pretty -n 1
```

## Debugging

- **Plugin logs**: The plugin's stderr is captured by the Docker daemon:
  `journalctl -u docker.service | grep journald-plus`
- **Host Socket**: Ensure `/run/systemd/journal/socket` is accessible; the plugin will fail to enable without it.
- **Protocol**: Logs are read from per-container FIFOs; the driver processes these in goroutines (one per container).

## Project Layout

- `main.go`: Docker plugin API entrypoint.
- `driver/`: Core logic (Decode -> Reassemble -> Merge -> Strip -> Priority -> Socket).
- `config.json`: Plugin manifest (mounts, capabilities).
- `tmp/`: Build artifacts (gitignored).
  - `tmp/build/`: Compiled Go binary.
  - `tmp/plugin/`: Plugin rootfs and manifest for Docker plugin creation.
