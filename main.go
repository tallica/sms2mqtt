package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/tallica/sms2mqtt/config"
	"github.com/tallica/sms2mqtt/modem"
	"github.com/tallica/sms2mqtt/mqttclient"
)

var version = "dev"

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	log.Info().Str("version", version).Msg("sms2mqtt starting")

	cfg, err := config.Load()
	if err != nil {
		log.Fatal().Err(err).Msg("config error")
	}

	log.Info().Str("device", cfg.Modem.Device).Msg("opening modem")
	m, err := modem.Open(cfg.Modem.Device, cfg.Modem.BaudRate)
	if err != nil {
		log.Fatal().Err(err).Msg("modem open failed")
	}
	defer m.Close()
	log.Info().Msg("modem ready")

	log.Info().Str("broker", cfg.MQTT.Broker).Msg("connecting to MQTT")
	mqtt, err := mqttclient.New(cfg.MQTT)
	if err != nil {
		log.Fatal().Err(err).Msg("mqtt connect failed")
	}
	defer mqtt.Disconnect()

	ticker := time.NewTicker(time.Duration(cfg.Modem.PollSeconds) * time.Second)
	defer ticker.Stop()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	if cfg.ForwardTo != "" {
		log.Info().Str("to", cfg.ForwardTo).Msg("SMS forwarding enabled")
	}

	log.Info().Int("interval_s", cfg.Modem.PollSeconds).Msg("polling started")

	for {
		select {
		case <-ticker.C:
			pollSMS(m, mqtt, cfg.ForwardTo)

		case req := <-mqtt.SendRequests():
			log.Info().Str("to", req.To).Msg("sending SMS")
			if err := m.SendSMS(req.To, req.Body); err != nil {
				log.Error().Err(err).Str("to", req.To).Msg("send failed")
			} else {
				log.Info().Str("to", req.To).Msg("SMS sent")
			}

		case sig := <-sigs:
			log.Info().Str("signal", sig.String()).Msg("shutting down")
			return
		}
	}
}

func pollSMS(m *modem.Modem, mqtt *mqttclient.Client, forwardTo string) {
	messages, err := m.ListSMS()
	if err != nil {
		log.Error().Err(err).Msg("list SMS failed")
		return
	}
	for _, sms := range messages {
		log.Info().Str("from", sms.From).Str("body", sms.Body).Msg("received SMS")
		mqtt.PublishInbox(mqttclient.InboxMessage{
			From: sms.From,
			Body: sms.Body,
			Time: sms.Time.Format(time.RFC3339),
		})
		if sms.Body == "ping" {
			if err := m.SendSMS(sms.From, "pong"); err != nil {
				log.Error().Err(err).Str("to", sms.From).Msg("pong failed")
			} else {
				log.Info().Str("to", sms.From).Msg("pong sent")
			}
		}
		if forwardTo != "" {
			body := fmt.Sprintf("From: %s\n%s", sms.From, sms.Body)
			if err := m.SendSMS(forwardTo, body); err != nil {
				log.Error().Err(err).Str("to", forwardTo).Msg("forward failed")
			} else {
				log.Info().Str("to", forwardTo).Str("from", sms.From).Msg("SMS forwarded")
			}
		}
		if err := m.DeleteSMS(sms.Index); err != nil {
			log.Error().Err(err).Int("index", sms.Index).Msg("delete SMS failed")
		}
	}
}
