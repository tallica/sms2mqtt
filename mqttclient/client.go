package mqttclient

import (
	"encoding/json"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/rs/zerolog/log"

	"github.com/tallica/sms2mqtt/config"
)

type Client struct {
	c      paho.Client
	cfg    config.MQTTConfig
	sendCh chan SendRequest
}

type SendRequest struct {
	To   string `json:"to"`
	Body string `json:"body"`
}

type InboxMessage struct {
	From string `json:"from"`
	Body string `json:"body"`
	Time string `json:"time"`
}

type ModemMessage struct {
	Status      string `json:"status"`
	Network     string `json:"network,omitempty"`
	SIM         string `json:"sim,omitempty"`
	SignalDBm   *int   `json:"signal_dbm,omitempty"`
	SignalLevel string `json:"signal_level,omitempty"`
	Operator    string `json:"operator,omitempty"`
	Roaming     *bool  `json:"roaming,omitempty"`
}

func New(cfg config.MQTTConfig) *Client {
	client := &Client{
		cfg:    cfg,
		sendCh: make(chan SendRequest, 16),
	}

	opts := paho.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID).
		SetWill(cfg.TopicStatus, "offline", 1, true).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetOnConnectHandler(func(c paho.Client) {
			log.Info().Str("broker", cfg.Broker).Msg("MQTT connected")
			token := c.Publish(cfg.TopicStatus, 1, true, "online")
			if !token.WaitTimeout(5 * time.Second) {
				log.Warn().Str("topic", cfg.TopicStatus).Msg("mqtt publish timeout")
			}
			token = c.Subscribe(cfg.TopicSend, 1, func(_ paho.Client, msg paho.Message) {
				var req SendRequest
				if err := json.Unmarshal(msg.Payload(), &req); err != nil {
					log.Error().Err(err).Msg("invalid send payload")
					return
				}
				client.sendCh <- req
			})
			if token.WaitTimeout(5*time.Second) && token.Error() != nil {
				log.Error().Err(token.Error()).Msg("mqtt subscribe failed")
				return
			}
			log.Info().Str("topic", cfg.TopicSend).Msg("subscribed for outbound SMS")
		}).
		SetConnectionLostHandler(func(c paho.Client, err error) {
			log.Warn().Err(err).Msg("MQTT connection lost")
		})

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username).SetPassword(cfg.Password)
	}

	client.c = paho.NewClient(opts)
	client.c.Connect()
	return client
}

// SendRequests returns a channel of outgoing SMS requests from MQTT.
func (c *Client) SendRequests() <-chan SendRequest {
	return c.sendCh
}

// PublishInbox publishes a received SMS to the inbox topic.
func (c *Client) PublishInbox(msg InboxMessage) {
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Msg("marshal inbox message")
		return
	}
	from := msg.From
	go func() {
		token := c.c.Publish(c.cfg.TopicInbox, 1, false, string(payload))
		if !token.WaitTimeout(5 * time.Second) {
			log.Warn().Str("topic", c.cfg.TopicInbox).Msg("mqtt publish timeout")
			return
		}
		log.Info().Str("from", from).Msg("published SMS to MQTT")
	}()
}

// PublishModem publishes modem state to the modem topic (retained).
func (c *Client) PublishModem(msg ModemMessage) {
	payload, err := json.Marshal(msg)
	if err != nil {
		log.Error().Err(err).Msg("marshal modem message")
		return
	}
	c.publish(c.cfg.TopicModem, string(payload), true)
}

// publish is fire-and-forget so the poll loop is never blocked by MQTT state.
func (c *Client) publish(topic, payload string, retain bool) {
	go func() {
		token := c.c.Publish(topic, 1, retain, payload)
		if !token.WaitTimeout(5 * time.Second) {
			log.Warn().Str("topic", topic).Msg("mqtt publish timeout")
		}
	}()
}

func (c *Client) Disconnect() {
	// Publish offline status synchronously so it lands before the connection closes.
	c.c.Publish(c.cfg.TopicStatus, 1, true, "offline").WaitTimeout(2 * time.Second)
	if payload, err := json.Marshal(ModemMessage{Status: "offline"}); err == nil {
		c.c.Publish(c.cfg.TopicModem, 1, true, string(payload)).WaitTimeout(2 * time.Second)
	}
	c.c.Disconnect(500)
}
