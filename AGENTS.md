# AGENTS.md
*Pure-Go Docker log driver for journald with multiline merging and priority parsing.*

## References
- [README.md] for features, installation, and usage documentation
- [DEVELOPMENT.md] for build, test, and release workflow
- [Makefile] — run `make` to list available targets
- [.github/workflows/] — GitHub Actions publishes to Docker Hub (`baraverkstad/journald-plus`)

## Goals & Ethos
- Minimal memory and storage requirements
- Minimal dependencies
- Robust error handling — log drivers cannot crash

## Code Style
- Radically brief: short names, minimal ceremony
- No comments unless API/javadoc or genuinely non-obvious logic
- Short variable names (1–2 chars) for loops and iteration

## Constraints
- **No CGO** — pure Go only
- **No `gogo/protobuf`** — use internal decoder
- **No `ReadLogs`** — users read via `journalctl`
- **No crash** — log drivers must survive all errors
- **Separate arch tags** — plugin limitations prevent true multi-arch manifests; publish `:latest-amd64`, `:latest-arm64` etc.

## Design Notes
- **Architecture**: Docker v2 managed plugin; writes structured log entries to host journald socket.
- **Pipeline**: Proto decode → Partial reassembly → Multiline merge → JSON/Regex extract → Priority detect → journald socket write
- **Multiline**: Buffer lines matching regex; flush on 10ms timeout
- **Priority**: Detect via JSON `level` key first, then regex matches on message
- **JSON extraction**: Zero overhead when disabled (single boolean check); invalid JSON falls back to raw text
- **Build**: `make build` works on macOS (fast iteration); `make plugin` requires Linux
- **Testing**: Inject `SendFunc` for isolated unit tests without a live journald socket
