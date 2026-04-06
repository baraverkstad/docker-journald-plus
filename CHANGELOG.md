# Changelog

## Unreleased

- Fix timestamp stripping for space-separated timezone offsets (e.g. `2024-01-15 10:30:45 UTC`)

[Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.4...HEAD)

## v0.4 - 2026-04-05

- Rename `parse-json` option to `json-parse`
- Add `json-skip-keys` and `json-extra` options for JSON log parsing
- Fix `[Note]` log level priority mapped to INFO instead of NOTICE

[Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.3...v0.4)

## v0.3 - 2026-04-05

- Removed all third-party library dependencies

[Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.2...v0.3)

## v0.2 - 2026-02-15

- Priority detection tolerates up to 30-character prefix before level keyword

[Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.1...v0.2)

## v0.1 - 2026-02-15

Initial release.

[Commit list](https://github.com/baraverkstad/docker-journald-plus/releases/tag/v0.1)
