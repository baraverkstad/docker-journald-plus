# Plan: README fixup

Easy documentation improvements that require no code changes.

## Comparison table

- [ ] Add a table comparing the built-in `journald` driver vs `journald-plus` —
      what fields are written, what features are added, what is intentionally
      absent (`docker logs`). New users need this to assess the trade-off
      without reading the whole README.

## Clarify multiline defaults

- [ ] Explicitly call out that the default `multiline-regex: ^\s` (leading
      whitespace) handles Java stack traces (`at com.example.Foo`) and Python
      tracebacks out of the box. Currently silent on this; readers assume the
      worst.

## Per-service field extraction example

- [ ] Add a `docker-compose.yml` example showing `field-*` options set
      per-service under `logging.options`. Counters the impression that field
      extraction is daemon-global-only.

## Graduate experimental labels

- [ ] Evaluate whether `strip-timestamp`, `strip-priority`, and `json-parse` are
      stable enough to drop the "experimental" label, or document a concrete
      path to stability. The label is the first thing a cautious evaluator
      notices.

## Install wrapper script

- [ ] Provide a copy-paste one-liner (or small shell script) that runs the
      `uname -m` / `sed` / `docker plugin install --alias` sequence. Reduces the
      first-impression friction for new users.

## Surface journalctl earlier

- [ ] Move the `journalctl` usage examples to a more prominent position —
      ideally immediately after the `docker logs` is not supported note. Make
      `journalctl -f`, `-p`, `-t` feel like a natural replacement rather than an
      afterthought.
