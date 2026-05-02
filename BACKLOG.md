# Backlog

## Telemetry

- **IMEI + firmware version** — `AT+CGSN` and `AT+CGMR`; add to `sms2mqtt/modem` payload and expose as diagnostic attributes in HA

- **SMS storage monitoring** — `AT+CPMS?` gives used/total SIM/modem storage slots; add to `sms2mqtt/modem` payload and alert before storage fills (already handled as `storage_full` send error)

## Reliability

- **Modem reconnect on serial failure** — currently a port error kills the service and relies on systemd restart; a retry loop inside the service would recover faster and avoid unnecessary restarts

## Messaging

- **Incoming SMS forwarding improvements** — `FORWARD_TO` currently sends `From: +48...\nbody`; could be more configurable (template, subject, multiple recipients)
