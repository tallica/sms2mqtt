# sms2mqtt

Bridge between a Huawei E3272s-153 USB modem and Home Assistant via MQTT.

- Polls the modem for incoming SMS and publishes them to `sms2mqtt/inbox`
- Subscribes to `sms2mqtt/send` to send outgoing SMS
- Publishes `online`/`offline` to `sms2mqtt/status` (retained Last Will)

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

Copy `.env.example` and fill in your values.

## MQTT payload format

**Received SMS** (`sms2mqtt/inbox`):
```json
{"from": "+48123456789", "body": "Hello", "time": "2026-04-30T12:00:00Z"}
```

**Send SMS** (publish to `sms2mqtt/send`):
```json
{"to": "+48123456789", "body": "Hello back"}
```

## Deploy as a systemd service

First-time setup on the Linux host:

```bash
# Copy and edit config
sudo mkdir -p /etc/sms2mqtt
scp .env.example user@host:/tmp/env
ssh user@host "sudo mv /tmp/env /etc/sms2mqtt/env && sudo nano /etc/sms2mqtt/env"

# Install service
scp sms2mqtt.service user@host:/tmp/
ssh user@host "sudo mv /tmp/sms2mqtt.service /etc/systemd/system/ \
  && sudo systemctl daemon-reload \
  && sudo systemctl enable sms2mqtt"

# Deploy binary and start
export REMOTE=user@host
make deploy && make start && make logs
```

The unit runs as the `homeassistant` user with the `dialout` group for serial port access. Adjust `User=` in `sms2mqtt.service` if your setup differs.

## Home Assistant integration

Add to `configuration.yaml`:

```yaml
mqtt:
  sensor:
    - name: "Last SMS"
      state_topic: "sms2mqtt/inbox"
      value_template: "{{ value_json.from }}"
      json_attributes_topic: "sms2mqtt/inbox"

  binary_sensor:
    - name: "SMS Bridge"
      state_topic: "sms2mqtt/status"
      payload_on: "online"
      payload_off: "offline"
      device_class: connectivity
```

Example automation — reply to a "STATUS" SMS:

```yaml
automation:
  - alias: "SMS status reply"
    trigger:
      platform: mqtt
      topic: sms2mqtt/inbox
    condition:
      condition: template
      value_template: "{{ trigger.payload_json.body | upper == 'STATUS' }}"
    action:
      service: mqtt.publish
      data:
        topic: sms2mqtt/send
        payload_template: >
          {"to": "{{ trigger.payload_json.from }}", "body": "All systems OK"}
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md).
