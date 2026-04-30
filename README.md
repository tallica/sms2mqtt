# sms2mqtt

Bridge between a Huawei E3272s-153 USB modem and Home Assistant via MQTT.

- Polls the modem for incoming SMS and publishes them to `sms2mqtt/inbox`
- Subscribes to `sms2mqtt/send` to send outgoing SMS (full Unicode / emoji support)
- Publishes `online`/`offline` to `sms2mqtt/status` (retained Last Will)
- Forwards incoming SMS to a phone number (`FORWARD_TO`)
- Built-in bot commands: `ping`, `version`, `status`

> **⚠️ AI-assisted project**  
> This codebase was built with [Claude Code](https://claude.ai/code). It works for the author's specific setup but has not been independently audited. Review the code before running it in any security-sensitive or production environment.

## Requirements

- Linux host with Huawei E3272 visible as `/dev/ttyUSB0`
- MQTT broker (e.g. Mosquitto bundled with Home Assistant)
- Go 1.21+ (build only)

## Build & deploy

The service runs on Linux. Development from macOS uses cross-compilation and `make` to deploy over SSH.

```bash
# Compile check
go build ./...

# Cross-compile and deploy to remote Linux host
export REMOTE=user@your-linux-box
make deploy            # default: linux/arm64 (Raspberry Pi 4 64-bit)
make deploy ARCH=amd64 # x86-64 host
make deploy ARCH=arm   # Pi 4 with 32-bit OS (GOARM=7)

# Service control
make start
make restart
make stop
make status
make logs              # tails journalctl -fu sms2mqtt on the remote
```

| `ARCH` | Target |
|---|---|
| `arm64` | Raspberry Pi 4, 64-bit OS (default) |
| `arm` | Raspberry Pi 4, 32-bit OS |
| `amd64` | x86-64 Linux host |

macOS note: the Huawei E3272 requires Linux kernel drivers (`option` module) to appear as a serial port. It will not expose a `cu.*` device on macOS, so the modem must be connected to the Linux host.

## Configuration

All configuration is via environment variables:

| Variable | Default | Description |
|---|---|---|
| `MODEM_DEVICE` | `/dev/ttyUSB0` | Serial device path |
| `MODEM_BAUD_RATE` | `115200` | Serial baud rate |
| `MODEM_POLL_SECONDS` | `10` | SMS poll interval |
| `MQTT_BROKER` | `tcp://localhost:1883` | MQTT broker URL |
| `MQTT_CLIENT_ID` | `sms2mqtt` | MQTT client identifier |
| `MQTT_USERNAME` | _(none)_ | MQTT username |
| `MQTT_PASSWORD` | _(none)_ | MQTT password |
| `MQTT_TOPIC_INBOX` | `sms2mqtt/inbox` | Topic for received SMS |
| `MQTT_TOPIC_SEND` | `sms2mqtt/send` | Topic to trigger outgoing SMS |
| `MQTT_TOPIC_STATUS` | `sms2mqtt/status` | LWT topic |
| `FORWARD_TO` | _(none)_ | Phone number to forward received SMS to |

Copy `.env.example` and fill in your values.

## MQTT payload format

**Received SMS** (`sms2mqtt/inbox`):
```json
{"from": "+48123456789", "body": "Hello", "time": "2026-04-30T12:00:00Z"}
```

**Send SMS** (publish to `sms2mqtt/send`):
```json
{"to": "+48123456789", "body": "Hello back 👋"}
```

## Bot commands

Send these as an SMS to the modem's number:

| Command | Reply |
|---|---|
| `ping` | `pong` |
| `version` | `sms2mqtt v0.5.1` |
| `status` | `sms2mqtt v0.5.1 \| up 2h30m \| signal -65 dBm` |

Bot-handled messages are never forwarded via `FORWARD_TO`.

## Deploy as a systemd service

First-time setup on the Linux host:

```bash
# Create dedicated user
sudo useradd -r -s /usr/sbin/nologin sms2mqtt
sudo usermod -aG dialout sms2mqtt

# Copy and edit config
sudo mkdir -p /etc/sms2mqtt
sudo cp .env.example /etc/sms2mqtt/env
sudo nano /etc/sms2mqtt/env

# Install service
sudo cp sms2mqtt.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable sms2mqtt

# Deploy binary and start
export REMOTE=user@host
make deploy && make start && make logs
```

## Home Assistant integration

### Sending SMS

```yaml
notify_sms_michal:
  alias: "Send SMS to Michal"
  fields:
    message:
      description: "The message content"
  sequence:
  - action: mqtt.publish
    data:
      topic: sms2mqtt/send
      payload: "{{ {'to': states('sensor.michal_phone_number'), 'body': message} | to_json }}"
  mode: queued
  max: 3
```

### Status and incoming SMS sensors

```yaml
mqtt:
  binary_sensor:
    - name: "sms2mqtt"
      state_topic: sms2mqtt/status
      payload_on: online
      payload_off: offline
      device_class: connectivity

  sensor:
    - name: "Last SMS"
      state_topic: sms2mqtt/inbox
      value_template: "{{ value_json.from }}"
      json_attributes_topic: sms2mqtt/inbox
      json_attributes_template: "{{ value_json | to_json }}"
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

---

🤖 Built with [Claude Code](https://claude.ai/code)
