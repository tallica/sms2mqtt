# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/tallica/sms2mqtt/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/tallica/sms2mqtt/releases/tag/v0.1.0
