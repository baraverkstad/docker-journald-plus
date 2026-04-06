# AGENTS.md
*Pure-Go Docker log driver for journald with multiline merging and priority parsing.*

## References
- [README.md] — features, installation, documentation
- [CHANGELOG.md] — user-facing changes by version
- [DEVELOPMENT.md] — detailed build, test, and release workflow
- [Makefile] — canonical build tool
- [.github/workflows/] — GitHub Actions publish to Docker Hub (`baraverkstad/journald-plus`)

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
- **Arch tags** — plugins cannot have multi-arch manifests; publish as `:<ver>-amd64`, `:<ver>-arm64` etc.

## Design Notes
- **Architecture**: Docker v2 managed plugin; writes structured log entries to host journald socket.
- **Protocol**: per-container FIFO with 4-byte big-endian length-prefixed protobuf; must drain FIFO before responding to `StopLogging`.
- **Tag**: defaults to container name (not short ID, unlike the built-in driver).
- **Pipeline**: Proto decode → Partial reassembly → Multiline merge → JSON/Regex extract → Priority detect → journald socket write
- **Multiline**: Buffer lines matching regex; flush on 10ms timeout
- **Priority**: Detect via JSON `level` key first, then regex matches on message
- **JSON extraction**: Zero overhead when disabled (single boolean check); invalid JSON falls back to raw text
- **Testing**: Inject `SendFunc` for isolated unit tests without a live journald socket

## Workflow
- **Builds** — use `make build` over `go build`
- **Unit Tests** — use `make build test` over `go test`, even for single tests (fast enough)
- **Conformance Tests** — check if `README.md` or `CHANGELOG.md` affected by change, suggest relevant manual tests to user
- **Version Bumps** — update `go.mod`, `Dockerfile`, and GitHub Actions together; run `make outdated` after
- **Writing Examples:** always verify runtime behavior against the source or command output

## Command Reference
```
make                    # list top-level make targets
make build test         # build and test
make outdated           # list outdated dependencies and code modernizations
```
