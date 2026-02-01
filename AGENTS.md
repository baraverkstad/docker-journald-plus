# Agent Instructions

See **README.md** for features, installation, and usage documentation.
See **DEVELOPMENT.md** for build, test, and development workflow instructions.

## Project Goals & Ethos
- **Core Utility**: A pure-Go Docker log driver for `journald` with multiline merging and priority parsing.
- **Design Philosophy**: Radical brevity. Compact code. No CGO. Minimal dependencies.
- **Architecture**: Plugin v2 (HTTP) -> FIFO Read -> Pipeline (Decode/Merge/Strip/Priority) -> Socket Write.
- **Reliability**: Robust error handling is critical (log drivers cannot crash).

## Workflows

- **Issue Tracking**:
    - Use `bd` (beads). See reference below.
- **Verification**:
    - Always run `make test` before finishing.
    - Use `make plugin` to verify build.
- **Session End**:
    - Create issues for anything that needs follow-up
    - Close finished issues, update in-progress items
    - Commit all changes with a succinct message
    - Ensure no temporary or uncommitted files remain

### Quick Reference (bd)
```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```
