# CLAUDE.md

## Project

`sms2mqtt` is a Go service that bridges a Huawei E3272s-153 USB modem to Home Assistant via MQTT. It polls the modem for incoming SMS and publishes them to MQTT; it also subscribes to an MQTT topic to send outgoing SMS.

## Build & deploy

Development is on macOS; the service runs on a remote Linux host over SSH.

```bash
go build ./...        # compile check (macOS)
make deploy           # cross-compile linux/amd64 + scp to $REMOTE:/usr/local/bin/sms2mqtt
make restart          # ssh remote systemctl restart sms2mqtt
make logs             # ssh remote journalctl -fu sms2mqtt
```

Set `REMOTE=user@host` in the shell before using `make`. The Makefile targets are: `build-linux`, `deploy`, `start`, `stop`, `restart`, `status`, `logs`.

macOS cannot run the service directly — the Huawei E3272 requires the Linux `option` kernel module to expose a serial port.

## Package layout

| Package | File | Role |
|---|---|---|
| `main` | `main.go` | Wires modem + MQTT; drives the poll/send select loop |
| `modem` | `modem/modem.go` | Serial port open, AT command send/receive, drain |
| `modem` | `modem/sms.go` | `ListSMS`, `DeleteSMS`, `SendSMS`, AT+CMGL parser |
| `mqttclient` | `mqttclient/client.go` | Paho wrapper — LWT, publish inbox, send channel |
| `config` | `config/config.go` | All config from env vars with defaults |

## Modem notes

- **AT port**: `/dev/ttyUSB0` — the AT command interface on the E3272.  
  `/dev/ttyUSB2` is a secondary NDIS port; not used here.
- **SMS mode**: text mode (`AT+CMGF=1`), GSM charset (`AT+CSCS="GSM"`).
- **Push notifications disabled**: `AT+CNMI=0,0,0,0,0` — the service polls instead of reacting to unsolicited result codes.
- **Delete on read**: messages are deleted from modem storage after successful MQTT publish to avoid re-delivery.
- **Non-ASCII SMS**: GSM charset drops accented characters. If Polish/UTF-8 support is needed, switch to PDU mode (`AT+CMGF=0`).

## MQTT topics

| Topic | Direction | Payload |
|---|---|---|
| `sms2mqtt/inbox` | modem → HA | `{"from":"+48...","body":"...","time":"RFC3339"}` |
| `sms2mqtt/send` | HA → modem | `{"to":"+48...","body":"..."}` |
| `sms2mqtt/status` | modem → HA | `"online"` / `"offline"` (retained LWT) |

All topics are overridable via env vars (`MQTT_TOPIC_INBOX`, etc.).

## Configuration

All config via environment variables. See `.env.example` for the full list.  
On the server the env file lives at `/etc/sms2mqtt/env` and is loaded by the systemd unit.

Required: `MODEM_DEVICE`, `MQTT_BROKER`.  
Everything else has a default.

## Conventions

- Logging via `zerolog` — structured, console-formatted to stderr.
- No comments explaining what the code does; only comments for non-obvious constraints (AT quirks, timing, modem-specific behaviour).
- No mocks — the modem package is thin enough to test against a real device or a pty.
- Keep `main.go` as a wiring layer only — no business logic.
- Update `CHANGELOG.md` under `## [Unreleased]` for every notable change.

## Deployment

First-time service install on the Linux host (run once manually):

```bash
sudo mkdir -p /etc/sms2mqtt
sudo cp .env.example /etc/sms2mqtt/env   # then edit with real values
sudo cp sms2mqtt.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable sms2mqtt
```

Subsequent deploys from macOS: `make deploy && make restart && make logs`.

The systemd unit runs as the `homeassistant` user with the `dialout` supplementary group for serial port access.
