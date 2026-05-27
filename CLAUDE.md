# CLAUDE.md

## Project

`sms2mqtt` is a Go service that bridges a Huawei E3272s-153 USB modem to Home Assistant via MQTT. It polls the modem for incoming SMS and publishes them to MQTT; it also subscribes to an MQTT topic to send outgoing SMS.

## Build & deploy

Development is on macOS; the service runs on a remote Linux host over SSH.

```bash
make build                  # compile check (macOS)
make test                   # run unit tests
make lint                   # run golangci-lint
make deploy                 # cross-compile linux/arm64 + scp to $REMOTE:/usr/local/bin/sms2mqtt
make deploy ARCH=amd64      # x86-64 target
make deploy ARCH=arm        # Pi 4 32-bit OS
make restart && make logs   # restart service and tail logs
```

Set `REMOTE=user@host` in the shell before using `make`. Default `ARCH` is `arm64` (Raspberry Pi 4 64-bit). Available targets: `build`, `test`, `lint`, `build-arm64`, `build-arm`, `build-amd64`, `deploy`, `start`, `stop`, `restart`, `status`, `logs`.

macOS cannot run the service directly — the Huawei E3272 requires the Linux `option` kernel module to expose a serial port.

## Package layout

| Package | File | Role |
|---|---|---|
| `main` | `main.go` | Wires modem + MQTT + bot; drives the poll/send select loop |
| `bot` | `bot/bot.go` | Command dispatch — `Bot`, built-in `Ping()` and `Status()` commands |
| `modem` | `modem/modem.go` | Serial port open, AT command send/receive, drain |
| `modem` | `modem/sms.go` | `ListSMS`, `DeleteSMS`, `SendSMS`, PDU list parser, multipart reassembly |
| `modem` | `modem/pdu.go` | PDU encode (`buildPDUs`) and decode (`decodeSMSDeliverPDU`), GSM-7/UCS-2, UDH parsing |
| `modem` | `modem/signal.go` | `SignalStrength`, `SignalLevel`, `NetworkRegistration`, `SIMStatus`, `Operator` |
| `modem` | `modem/pdu_test.go` | Unit tests for PDU encode/decode (GSM-7, UCS-2, UDH, BCD, SCTS, multipart split) |
| `modem` | `modem/sms_test.go` | Unit tests for `parsePDUList` and `reassembleMultipart` |
| `modem` | `modem/signal_test.go` | Unit tests for `SignalLevel` |
| `mqttclient` | `mqttclient/client.go` | Paho wrapper — LWT, publish inbox, send channel |
| `config` | `config/config.go` | All config from env vars with defaults |

## Modem notes

- **AT port**: `/dev/modemGSM_at` (USB interface `01`, typically `ttyUSB2`) — the AT command interface on the E3272. Use the udev rule in `README.md` to get a stable symlink.  
  `/dev/modemGSM_ppp` (USB interface `00`, typically `ttyUSB1`) is the PPP data port; not used by this service.
- **Idle mode**: text mode (`AT+CMGF=1`), GSM charset (`AT+CSCS="GSM"`).
- **Read mode**: switches to PDU mode (`AT+CMGF=0`) per poll to read raw PDUs via `AT+CMGL=4`. This allows UDH (User Data Header) parsing for multipart SMS reassembly, and correct decoding of alphanumeric sender addresses (TON=0xD0, packed GSM-7). Text mode is restored immediately after listing. Multipart segments arriving in the same poll are reassembled into one message; incomplete groups are left on the modem for the next poll.
- **Send mode**: PDU mode (`AT+CMGF=0`) with UCS-2 encoding, switched per send and restored after. Supports emoji and full Unicode.
- **Push notifications disabled**: `AT+CNMI=0,0,0,0,0` — the service polls instead of reacting to unsolicited result codes.
- **Delete on read**: all modem storage indices belonging to a message (one for single-part, all parts for multipart) are deleted after successful MQTT publish to avoid re-delivery.

## MQTT topics

| Topic | Direction | Payload |
|---|---|---|
| `sms2mqtt/inbox` | modem → HA | `{"from":"+48...","body":"...","time":"RFC3339"}` |
| `sms2mqtt/send` | HA → modem | `{"to":"+48...","body":"..."}` |
| `sms2mqtt/status` | modem → HA | `"online"` / `"offline"` (retained LWT) |
| `sms2mqtt/modem` | modem → HA | `{"status":"ready","network":"registered","sim":"ready","signal_dbm":-67,"signal_level":"good","operator":"Orange PL","roaming":false}` (retained, each poll; `{"status":"offline"}` on shutdown) |

All topics are overridable via env vars (`MQTT_TOPIC_INBOX`, etc.).

## Configuration

All config via environment variables. See `.env.example` for the full list.  
On the server the env file lives at `/etc/sms2mqtt/env` and is loaded by the systemd unit.

Required: `MODEM_DEVICE`, `MQTT_BROKER`.  
Everything else has a default.

Notable optional vars:

- `FORWARD_TO` — phone number to forward every received SMS to, in addition to MQTT. Format: `From: {number}\n{body}`. Empty = disabled. Ping messages are never forwarded.

## Built-in SMS commands

| Message | Response | Notes |
|---|---|---|
| `ping` | `pong` (reply to sender) | Case-sensitive, exact match. Not forwarded via `FORWARD_TO`. |
| `version` | `sms2mqtt <version>` | Reports the running binary version. Not forwarded via `FORWARD_TO`. |
| `status` | `sms2mqtt <ver> \| up Xh Ym \| -Z dBm (level) \| <operator> \| network: <registered\|roaming\|searching\|denied\|not_registered\|unknown> \| sim: <ready\|absent\|pin_required\|puk_required\|error>` | Reports version, uptime, signal+level, operator, network, and SIM. Sent as multipart SMS if needed. Not forwarded. |

Bot-handled messages are never forwarded. Add new commands in `bot/bot.go`.

## Conventions

- Logging via `zerolog` — structured, console-formatted to stderr.
- No comments explaining what the code does; only comments for non-obvious constraints (AT quirks, timing, modem-specific behaviour).
- Pure-logic helpers in the `modem` package (PDU codec, multipart reassembly, signal-level mapping) have unit tests in `modem/pdu_test.go`. Hardware-tied paths (serial I/O, AT command exchange) have no mocks — test those against a real device or a pty.
- Keep `main.go` as a wiring layer only — no business logic.
- Update `CHANGELOG.md` under `## [Unreleased]` for every notable change.
- Run `make lint` before committing — CI enforces `golangci-lint` per `.golangci.yml`.

## Deployment

- Env file: `/etc/sms2mqtt/env` — loaded by the systemd unit.
- Runs as the `sms2mqtt` system user with the `dialout` group for serial port access.
- See `README.md` for first-time install steps and Home Assistant integration.
