package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Modem ModemConfig
	MQTT  MQTTConfig
}

type ModemConfig struct {
	Device      string // e.g. /dev/ttyUSB0
	BaudRate    int
	PollSeconds int // how often to check for new SMS
}

type MQTTConfig struct {
	Broker   string // e.g. tcp://192.168.1.10:1883
	ClientID string
	Username string
	Password string
	TopicInbox  string // publish received SMS here
	TopicSend   string // subscribe for outgoing SMS
	TopicStatus string // LWT topic
}

func Load() (*Config, error) {
	cfg := &Config{
		Modem: ModemConfig{
			Device:      env("MODEM_DEVICE", "/dev/ttyUSB0"),
			BaudRate:    envInt("MODEM_BAUD_RATE", 115200),
			PollSeconds: envInt("MODEM_POLL_SECONDS", 10),
		},
		MQTT: MQTTConfig{
			Broker:      env("MQTT_BROKER", "tcp://localhost:1883"),
			ClientID:    env("MQTT_CLIENT_ID", "sms2mqtt"),
			Username:    env("MQTT_USERNAME", ""),
			Password:    env("MQTT_PASSWORD", ""),
			TopicInbox:  env("MQTT_TOPIC_INBOX", "sms2mqtt/inbox"),
			TopicSend:   env("MQTT_TOPIC_SEND", "sms2mqtt/send"),
			TopicStatus: env("MQTT_TOPIC_STATUS", "sms2mqtt/status"),
		},
	}

	if cfg.Modem.Device == "" {
		return nil, fmt.Errorf("MODEM_DEVICE is required")
	}
	if cfg.MQTT.Broker == "" {
		return nil, fmt.Errorf("MQTT_BROKER is required")
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
