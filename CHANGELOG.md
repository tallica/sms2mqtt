# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.4.1] - 2026-05-01

### Fixed

- Incoming SMS with emoji/Unicode are now decoded correctly — the modem returns UCS-2 hex in text mode, which is now transparently decoded before publishing to MQTT or forwarding
- Read trailing `OK` after `+CMGS:` before restoring text mode, preventing stale buffer bytes from causing `+CMS ERROR: operation not allowed` on the next poll

## [0.4.0] - 2026-04-30

### Changed

- `SendSMS` now uses PDU mode with UCS-2 encoding, supporting emoji and full Unicode (fixes `+CMS ERROR: 500` on non-GSM characters)

## [0.3.1] - 2026-04-30

### Changed

- Ping messages are no longer forwarded via `FORWARD_TO` since they are handled locally

### Documentation

- CLAUDE.md: document built-in SMS commands and ping/pong forwarding behaviour

## [0.3.0] - 2026-04-30

### Added

- Ping/pong: replying `pong` when an SMS with body `ping` is received

## [0.2.0] - 2026-04-30

### Added

- `FORWARD_TO` env var: when set, received SMS are forwarded to that number in addition to MQTT, formatted as `From: {number}\n{body}`

## [0.1.1] - 2026-04-30

### Fixed

- `readLine` now treats `\r` as a line terminator in addition to `\n`, fixing indefinite hangs on modems that use CR-only line endings (Huawei E3272)
- `+CME ERROR` responses now correctly terminate AT commands instead of being silently collected and causing a read timeout
- `AT+CSCS="GSM"` demoted to optional — logs a warning instead of failing startup when the modem rejects it (e.g. SIM absent or unsupported firmware)
- Modem package now uses `zerolog` consistently instead of stdlib `log`

## [0.1.0] - 2026-04-30

### Added

- Initial SMS-to-MQTT bridge implementation in Go
- Modem package with AT command layer over serial port (`go.bug.st/serial`)
- SMS polling via `AT+CMGL="ALL"` with configurable interval
- SMS send via `AT+CMGS` triggered by MQTT message on `sms2mqtt/send`
- Automatic SMS deletion after successful publish to MQTT
- MQTT client with Last Will and Testament (`sms2mqtt/status`: `online`/`offline`)
- Structured JSON payloads on `sms2mqtt/inbox` (`from`, `body`, `time`)
- Full configuration via environment variables with sensible defaults
- systemd unit file with `dialout` group for serial port access
- `.env.example` template

[Unreleased]: https://github.com/tallica/sms2mqtt/compare/v0.4.1...HEAD
[0.4.1]: https://github.com/tallica/sms2mqtt/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/tallica/sms2mqtt/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/tallica/sms2mqtt/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/tallica/sms2mqtt/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/tallica/sms2mqtt/compare/v0.1.1...v0.2.0
[0.1.1]: https://github.com/tallica/sms2mqtt/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/tallica/sms2mqtt/releases/tag/v0.1.0
