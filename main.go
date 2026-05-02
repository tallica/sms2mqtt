package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/tallica/sms2mqtt/bot"
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

	startTime := time.Now()

	b := bot.New(
		bot.Ping(),
		bot.Version(version),
		bot.Status(
			version,
			func() time.Duration { return time.Since(startTime) },
			m.SignalStrength,
			m.NetworkRegistration,
			m.SIMStatus,
		),
	)

	if cfg.ForwardTo != "" {
		log.Info().Str("to", cfg.ForwardTo).Msg("SMS forwarding enabled")
	}

	mqtt.PublishModem(mqttclient.ModemMessage{Status: "initializing"})

	log.Info().Int("interval_s", cfg.Modem.PollSeconds).Msg("polling started")

	for {
		select {
		case <-ticker.C:
			pollModem(m, mqtt)
			pollSMS(m, mqtt, b, cfg.ForwardTo)

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

func pollModem(m *modem.Modem, mqtt *mqttclient.Client) {
	sim, err := m.SIMStatus()
	if err != nil {
		log.Error().Err(err).Msg("SIM status check failed")
		sim = "error"
	}

	network, err := m.NetworkRegistration()
	if err != nil {
		log.Error().Err(err).Msg("network registration check failed")
		network = "unknown"
	}

	msg := mqttclient.ModemMessage{
		SIM:     sim,
		Network: network,
	}

	if dbm, ok, err := m.SignalStrength(); err == nil {
		if ok {
			msg.SignalDBm = &dbm
			msg.SignalLevel = modem.SignalLevel(dbm)
		} else {
			msg.SignalLevel = "none"
		}
	}

	msg.Status = deriveModemStatus(sim, network, msg.SignalDBm != nil)
	mqtt.PublishModem(msg)
}

func deriveModemStatus(sim, network string, hasSignal bool) string {
	switch sim {
	case "absent":
		return "no_sim"
	case "pin_required", "puk_required":
		return "sim_locked"
	case "error":
		return "error"
	}
	switch network {
	case "registered", "roaming":
		if !hasSignal {
			return "degraded"
		}
		return "ready"
	case "searching", "not_registered", "denied":
		return "offline"
	default:
		return "error"
	}
}

func pollSMS(m *modem.Modem, mqtt *mqttclient.Client, b *bot.Bot, forwardTo string) {
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
		if reply, ok := b.Reply(sms.From, sms.Body); ok {
			if err := m.SendSMS(sms.From, reply); err != nil {
				log.Error().Err(err).Str("to", sms.From).Msg("bot reply failed")
			} else {
				log.Info().Str("to", sms.From).Str("reply", reply).Msg("bot reply sent")
			}
		} else if forwardTo != "" {
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
