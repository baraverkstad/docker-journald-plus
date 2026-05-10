# Changelog

## v1.0 - 2026-05-10

- Documentation updates
- [Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.7...v1.0)

## v0.7 - 2026-04-25

- Replaced `net/http` with a minimal HTTP/1.1 handler, reducing binary size by 50%
- [Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.6...v0.7)

## v0.6 - 2026-04-14

- Added `normalize-whitespace` option to collapse tabs and repeated spaces to a single space
- Added `strip-priority` and `strip-priority-regex` options to remove log level from message text
- Fixed timestamp stripping for single-digit hours (e.g. `2026-02-21 6:36:00`)
- [Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.5...v0.6)

## v0.5 - 2026-04-06

- Fixed timestamp parsing of space-separated timezone offsets
- Removed last third-party library dependency
- [Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.4...v0.5)

## v0.4 - 2026-04-05

- Changed `parse-json` option to `json-parse` (backwards compatible)
- Added `json-skip-keys` and `json-extra` options for JSON log parsing
- Fixed `[Note]` log level priority mapped to INFO instead of NOTICE
- [Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.3...v0.4)

## v0.3 - 2026-04-05

- Removed almost all third-party library dependencies
- [Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.2...v0.3)

## v0.2 - 2026-02-15

- Fixed priority detection to ignore up to 30 chars before level keyword
- [Commit list](https://github.com/baraverkstad/docker-journald-plus/compare/v0.1...v0.2)

## v0.1 - 2026-02-15

- Initial release
- [Commit list](https://github.com/baraverkstad/docker-journald-plus/commits/v0.1/)
