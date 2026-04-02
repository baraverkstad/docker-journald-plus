# AGENTS.md
*Pure-Go Docker log driver for journald with multiline merging and priority parsing.*

## References
- [README.md] for features, installation, and usage documentation
- [DEVELOPMENT.md] for build, test, and release workflow
- [Makefile] — run `make` to list available targets
- [.github/workflows/] — GitHub Actions publishes to Docker Hub (`baraverkstad/journald-plus`)

## Goals & Ethos
- Radical brevity
- Minimal memory and storage requirements
- Minimal dependencies
- Robust error handling — log drivers cannot crash

## Code Style
- **Comments:** explain *why*, not *what*; only for exported APIs or non-obvious logic
- **Variables:** short but descriptive; 1–2 chars for loops and iteration

## Constraints
- **No CGO** — pure Go only
- **No `gogo/protobuf`** — use internal decoder
- **No `ReadLogs`** — users read via `journalctl`
- **No crash** — log drivers must survive all errors
- **Separate arch tags** — plugin limitations prevent true multi-arch manifests; publish `:latest-amd64`, `:latest-arm64` etc.

## Design Notes
- **Architecture**: Docker v2 managed plugin; writes structured log entries to host journald socket.
- **Protocol**: per-container FIFO with 4-byte big-endian length-prefixed protobuf; must drain FIFO before responding to `StopLogging`.
- **Tag**: defaults to container name (not short ID, unlike the built-in driver).
- **Pipeline**: Proto decode → Partial reassembly → Multiline merge → JSON/Regex extract → Priority detect → journald socket write
- **Multiline**: Buffer lines matching regex; flush on 10ms timeout
- **Priority**: Detect via JSON `level` key first, then regex matches on message
- **JSON extraction**: Zero overhead when disabled (single boolean check); invalid JSON falls back to raw text
- **Testing**: Inject `SendFunc` for isolated unit tests without a live journald socket

## Build & Test
```
make                    # show available targets
make build test         # build and test (works on macOS)
make publish            # cross-platform plugin build and push (Linux only)
```
