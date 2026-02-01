# Docker Journald Plus

This is a Docker log driver for systemd journald output. It improves upon the
built-in journald log driver by adding support for:

- Merging multiline messages
- Adjusting log priority based on indications in log message
- Supports all options as default journald driver

## Multiline Support

By default, log messages starting with whitespace are considered part of the
previous message. Only messages read within 10ms of each other are merged. The
multiline merge prefix can be configured using the `multiline-regex` option.

## Log Priority

By default, log messages are assigned the priority `info` or `error` based on
the source (stdout or stderr). These default priorities can be overridden in
two ways:

- Support for <prio> log message prefix (see
  https://www.freedesktop.org/software/systemd/man/latest/sd-daemon.html)
- Simple text matching on the log message using initial message characters,
  such as "[Note]" or "WARN". These are configurable per container.
