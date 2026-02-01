# Development Guide

## Quick Start

```bash
make              # List available targets
make test         # Run unit tests
make plugin       # Build plugin rootfs and manifest
make enable       # Install and enable plugin locally
```

## Plugin Lifecycle

The plugin is a Docker v2 managed plugin. The build process follows:
1. **Build**: `make plugin` creates the rootfs and `config.json` in `plugin-dir/`.
2. **Install**: `make enable` removes any existing version, creates the plugin from the rootfs, and enables it.
3. **Verify**: Run a container with `--log-driver journald-plus` and check `journalctl`.

## Integration Testing

To verify end-to-end logging:
```bash
# 1. Start a container using the plugin
docker run --rm --log-driver journald-plus --log-opt tag=test alpine \
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
- `test/`: End-to-end integration scripts.
