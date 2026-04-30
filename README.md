# sms2mqtt

Bridge between a Huawei E3272s-153 USB modem and Home Assistant via MQTT.

- Polls the modem for incoming SMS and publishes them to `sms2mqtt/inbox`
- Subscribes to `sms2mqtt/send` to send outgoing SMS
- Publishes `online`/`offline` to `sms2mqtt/status` (retained Last Will)

## Requirements

- Linux host with Huawei E3272 visible as `/dev/ttyUSB0`
- MQTT broker (e.g. Mosquitto bundled with Home Assistant)
- Go 1.21+ (build only)

## Build

```bash
# Local
go build -o sms2mqtt .

# Cross-compile for Linux/amd64 (e.g. from macOS)
GOOS=linux GOARCH=amd64 go build -o sms2mqtt .
```

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

```bash
sudo cp sms2mqtt /usr/local/bin/
sudo mkdir -p /etc/sms2mqtt
sudo cp .env.example /etc/sms2mqtt/env
# edit /etc/sms2mqtt/env with real values

sudo cp sms2mqtt.service /etc/systemd/system/
sudo systemctl enable --now sms2mqtt
sudo journalctl -fu sms2mqtt
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
