---
reviewer: claude.ai
date: 2026-04-15
prompt: >
  I think docker-journald-plus is becoming quite a good plugin. What do you see
  as its major flaws or omissions? Why isn't it more known?
---

# Project Review — 2026-04-15

## Flaws / Omissions

| Area                                    | Issue                                                                                                                                                                                                                                          |
| --------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `docker logs` unsupported               | Breaking omission for many users — tooling, CI, and muscle memory all use `docker logs`. Forcing a full `journalctl` workflow is a hard sell.                                                                                                  |
| Multi-arch install UX                   | The manual `uname -m` + `sed` + alias dance is clunky. No `latest` tag compounds this. A wrapper install script would help enormously.                                                                                                         |
| Multiline heuristics only               | Continuation-line detection is regex/timeout-based. No support for common structured multiline formats (e.g. Java stack traces starting with `at `, Python tracebacks). The defaults will silently mis-merge or split many real-world outputs. |
| `strip-timestamp` is experimental       | Timestamp stripping — arguably the most universally useful feature — is marked experimental with no clear path to stable.                                                                                                                      |
| JSON parsing is experimental            | Same issue. The most valuable feature for modern apps is the least stable.                                                                                                                                                                     |
| No log rotation / size caps             | Delegates entirely to journald's global config. No per-container retention controls.                                                                                                                                                           |
| Field extraction is global-only         | `field-*` extractors can't be scoped per-image or per-service without separate daemon configs.                                                                                                                                                 |
| No upstream journald driver parity docs | No explicit comparison table showing what the built-in driver does vs. this one. New users can't quickly assess the trade-offs.                                                                                                                |

## Why It's Not Better Known

- 0 stars, 0 forks, 1 contributor — essentially a personal/internal tool that
  happens to be public
- The Docker plugin ecosystem is poorly discoverable; Docker Hub search for log
  drivers is terrible
- Journald itself is niche — the majority of Docker users are on cloud hosts
  using `json-file` + a log shipper (Datadog, Loki, etc.)
- No blog post, no announcement, no HN/Reddit post
- "Experimental" labels on the headline features discourage adoption
- The versioned-alias install requirement has no automation — high friction for
  the cautious ops audience that would actually want this

## Summary

The core idea (priority detection + multiline merging as a plugin) is genuinely
useful for systemd-native deployments. The execution is solid for a one-person
project, but it needs a scripted installer, stable feature flags, and some
community presence to get traction.
