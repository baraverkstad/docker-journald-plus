# Project Intelligence
*Last Updated: 2026-02-17*

## 🛠 System Architecture
- **Core:** Pure Go (no CGO), Docker v2 managed plugin, writes to host journald socket.
- **Pipeline:** Proto decode → Partial reassembly → Multiline merge → JSON/Regex extract → Priority detect → Write.
- **Output:** No `ReadLogs` support; users must use `journalctl`.
- **Artifacts:** Separate generic tags per arch (`:latest-amd64`, `:arm64`) due to plugin limitations.

## 📋 Development Constraints
- **Deps:** Avoid `gogo/protobuf`; use internal decoder.
- **Build:** `make plugin` needs Linux; `make build` allows local macOS dev.
- **Testing:** Inject `SendFunc` for isolated unit tests.
- **CI:** Pass `VERSION` to Make; `latest` for main, `v*` for tags.

## 🧠 Operational Workflow
- **Local Dev:** Use `make build` (native arch, fast) not buildx for iteration
- **Release:** `make publish` builds and pushes all arch tags (called by CI).
- **Secrets:** `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` required for CI.
- **Docs:** README.md (users), DEVELOPMENT.md (developers), CLAUDE.md (AI agents)

## ⚡ Project Preferences
- **Design Philosophy:** Radical brevity, compact code, minimal dependencies
- **Error Handling:** Robust error handling is critical (log drivers cannot crash)
- **JSON Parsing:** Zero overhead when disabled (single boolean check); invalid JSON falls back to raw text
- **Extraction:** Use `--log-opt field-NAME='pattern'` to extract regex capture groups.
- **Multiline:** Buffer lines matching regex; flush on 10ms timeout.
- **Priorities:** Detect via JSON `level` key or regex matches on message.
