# sms2mqtt

Bridge between a Huawei E3272s-153 USB modem and Home Assistant via MQTT.

- Polls the modem for incoming SMS and publishes them to `sms2mqtt/inbox`
- Reassembles multipart (concatenated) SMS into a single message before publishing or forwarding
- Subscribes to `sms2mqtt/send` to send outgoing SMS (full Unicode / emoji support)
- Publishes `online`/`offline` to `sms2mqtt/status` (retained Last Will)
- Publishes modem telemetry to `sms2mqtt/modem` each poll cycle (status, network, SIM, signal)
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
| `MQTT_TOPIC_MODEM` | `sms2mqtt/modem` | Modem telemetry topic |
| `FORWARD_TO` | _(none)_ | Phone number to forward received SMS to |

Copy `.env.example` and fill in your values.

## MQTT payload format

**Received SMS** (`sms2mqtt/inbox`):
```json
{"from": "+48123456789", "body": "Hello", "time": "2026-04-30T12:00:00Z"}
```

`from` is a phone number (e.g. `+48123456789`) for regular senders, or an alphanumeric string (e.g. `Play`) for operator shortcodes.

**Send SMS** (publish to `sms2mqtt/send`):
```json
{"to": "+48123456789", "body": "Hello back 👋"}
```

**Modem telemetry** (`sms2mqtt/modem`, retained):
```json
{"status": "ready", "network": "registered", "sim": "ready", "signal_dbm": -67, "signal_level": "good", "operator": "Orange PL", "roaming": false}
```

Published each poll cycle. On clean shutdown, replaced with `{"status":"offline"}` — all other fields are cleared from the broker's retained value.

| Field | Values |
|---|---|
| `status` | `initializing` `ready` `degraded` `offline` `no_sim` `sim_locked` `error` |
| `network` | `registered` `roaming` `searching` `denied` `not_registered` `unknown` |
| `sim` | `ready` `absent` `pin_required` `puk_required` `error` |
| `signal_level` | `none` `poor` `fair` `good` `excellent` |
| `signal_dbm` | integer dBm, omitted when no signal |
| `operator` | carrier name string, omitted when not registered |
| `roaming` | boolean, omitted when registration state is unknown |

## Bot commands

Send these as an SMS to the modem's number:

| Command | Reply |
|---|---|
| `ping` | `pong` |
| `version` | `sms2mqtt v0.7.4` |
| `status` | `sms2mqtt v0.7.4 \| up 1d2h30m \| -65 dBm (good) \| Orange PL \| network: registered \| sim: ready` |

Bot-handled messages are never forwarded via `FORWARD_TO`.

## Linux host setup

### First-time service install

Run on the Linux host:

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

### Passwordless sudo for make targets

`make deploy`, `make start`, `make stop`, `make restart`, and `make status` all run `sudo systemctl` over SSH. Without passwordless sudo the commands will fail with *Interactive authentication required*.

Add a sudoers rule on the Linux host:

```bash
echo 'YOUR_USER ALL=(ALL) NOPASSWD: /usr/bin/systemctl * sms2mqtt' \
  | sudo tee /etc/sudoers.d/sms2mqtt
```

Replace `YOUR_USER` with the SSH user (e.g. `michal`). The wildcard covers `start`, `stop`, `restart`, and `status` for the `sms2mqtt` service only.

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
      payload: "{{ {'to': states('sensor.your_phone_number'), 'body': message} | to_json }}"
  mode: queued
  max: 3
```

### Sensors and device

All entities are grouped under a single **GSM Modem** device. `availability_topic` links each sensor to the LWT topic so HA marks them unavailable on any disconnect — clean or unclean — instead of showing stale values.

```yaml
mqtt:
  sensor:
    - name: "Status"
      unique_id: sms2mqtt_modem_status
      state_topic: sms2mqtt/modem
      value_template: "{{ value_json.status | replace('_', ' ') | capitalize }}"
      availability_topic: sms2mqtt/status
      device:
        identifiers: ["sms2mqtt"]
        name: "GSM Modem"
        model: "Huawei E3272s-153"
        manufacturer: "Huawei"

    - name: "Network"
      unique_id: sms2mqtt_modem_network
      state_topic: sms2mqtt/modem
      value_template: "{{ value_json.network | replace('_', ' ') | capitalize }}"
      availability_topic: sms2mqtt/status
      device:
        identifiers: ["sms2mqtt"]

    - name: "SIM"
      unique_id: sms2mqtt_modem_sim
      state_topic: sms2mqtt/modem
      value_template: "{{ value_json.sim | replace('_', ' ') | capitalize }}"
      availability_topic: sms2mqtt/status
      device:
        identifiers: ["sms2mqtt"]

    - name: "Signal"
      unique_id: sms2mqtt_modem_signal_dbm
      state_topic: sms2mqtt/modem
      value_template: "{{ value_json.signal_dbm | int(0) }}"
      unit_of_measurement: "dBm"
      device_class: signal_strength
      state_class: measurement
      availability_topic: sms2mqtt/status
      device:
        identifiers: ["sms2mqtt"]

    - name: "Signal level"
      unique_id: sms2mqtt_modem_signal_level
      state_topic: sms2mqtt/modem
      value_template: "{{ value_json.signal_level | capitalize }}"
      availability_topic: sms2mqtt/status
      device:
        identifiers: ["sms2mqtt"]

    - name: "Operator"
      unique_id: sms2mqtt_modem_operator
      state_topic: sms2mqtt/modem
      value_template: "{{ value_json.operator }}"
      availability_topic: sms2mqtt/status
      device:
        identifiers: ["sms2mqtt"]

    - name: "Last SMS"
      unique_id: sms2mqtt_last_sms
      state_topic: sms2mqtt/inbox
      value_template: "{{ value_json.from }}"
      json_attributes_topic: sms2mqtt/inbox
      device:
        identifiers: ["sms2mqtt"]

  binary_sensor:
    - name: "Bridge"
      unique_id: sms2mqtt_bridge
      state_topic: sms2mqtt/status
      payload_on: online
      payload_off: offline
      device_class: connectivity
      device:
        identifiers: ["sms2mqtt"]

    - name: "Roaming"
      unique_id: sms2mqtt_modem_roaming
      state_topic: sms2mqtt/modem
      value_template: "{{ value_json.roaming }}"
      payload_on: "True"
      payload_off: "False"
      availability_topic: sms2mqtt/status
      device:
        identifiers: ["sms2mqtt"]
```

## Changelog

See [CHANGELOG.md](CHANGELOG.md).

## License

MIT — see [LICENSE](LICENSE).

---

🤖 Built with [Claude Code](https://claude.ai/code)
