# AGENTS.md
*Pure-Go Docker log driver for journald with multiline merging and priority parsing.*

## File Reference
- [README.md] — features, installation, documentation
- [CHANGELOG.md] — user-facing changes by version
- [DEVELOPMENT.md] — project layout table, build/test commands, and plugin testing steps
- [Makefile] — canonical build tool
- [.github/workflows/] — GitHub Actions; publishes to Docker Hub (`baraverkstad/journald-plus`)

## Command Reference
```
make                    # list top-level make targets
make build test         # build and test
make outdated           # check code modernizations and outdated dependencies
```

## General
- Radical brevity — in code, rules, and responses
- Read related reference documents before starting a task
- Write project notes to `AGENTS.md` or `DEVELOPMENT.md`, not to the memory tool
- Explore feature additions with the user; implement only once intent is mutually clear
- Verify examples against source or command output

## Code
- **Comments:** explain *why*, not *what*; only for exported APIs or non-obvious logic
- **Variables:** short but descriptive; 1–2 chars for loops and iteration
- **Errors:** handle all errors; never panic
- **Dependencies:** ask the user before adding

## Testing
- **TDD Red-Green:** write the failing test before any new implementation
- **Builds:** prefer `make build test` over `go test`
- **Isolation:** inject `SendFunc` for unit tests; no live journald socket needed
- **Conformance:** after any user-facing change, update `README.md` and `CHANGELOG.md`
- **Integration:** after any behavioral change, run integration tests per `DEVELOPMENT.md`

## Design Notes
- **Architecture:** Docker v2 managed plugin; writes structured log entries to host journald socket.
- **Protocol:** per-container FIFO; drain before responding to `StopLogging`.
- **Tag:** defaults to container name (unlike built-in `journald` driver).
- **Multiline:** buffer lines matching regex; flush on 10ms timeout.
- **JSON extraction:** zero overhead when disabled; invalid JSON falls back to raw text.
- **Arch tags:** plugins cannot have multi-arch manifests; published as `:<ver>-amd64`, `:<ver>-arm64` etc.

## Tips
- **Version bumps:** update `go.mod`, `Dockerfile`, and GitHub Actions together; run `make outdated` after.
