# Agent Instructions

See **README.md** for features, installation, and usage documentation.
See **DEVELOPMENT.md** for build, test, and development workflow instructions.
See **.agent/memory/KNOWLEDGE-BASE.md** for project preferences and know-how.

## Project Goals & Ethos
- **Core Utility**: A pure-Go Docker log driver for `journald` with multiline merging and priority parsing.
- **Design Philosophy**: Radical brevity. Compact code. No CGO. Minimal dependencies.
- **Architecture**: Plugin v2 (HTTP) -> FIFO Read -> Pipeline (Decode/Merge/Strip/Priority) -> Socket Write.
- **Reliability**: Robust error handling is critical (log drivers cannot crash).

## Workflows

- Always run `make build test` before finishing
