package mqttclient

import (
	"encoding/json"
	"fmt"
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
}

func New(cfg config.MQTTConfig) (*Client, error) {
	opts := paho.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID).
		SetWill(cfg.TopicStatus, "offline", 1, true).
		SetAutoReconnect(true).
		SetConnectRetry(true).
		SetConnectRetryInterval(5 * time.Second).
		SetOnConnectHandler(func(c paho.Client) {
			log.Info().Str("broker", cfg.Broker).Msg("MQTT connected")
		}).
		SetConnectionLostHandler(func(c paho.Client, err error) {
			log.Warn().Err(err).Msg("MQTT connection lost")
		})

	if cfg.Username != "" {
		opts.SetUsername(cfg.Username).SetPassword(cfg.Password)
	}

	client := &Client{
		cfg:    cfg,
		sendCh: make(chan SendRequest, 16),
	}

	c := paho.NewClient(opts)
	if token := c.Connect(); token.WaitTimeout(10*time.Second) && token.Error() != nil {
		return nil, fmt.Errorf("mqtt connect: %w", token.Error())
	}
	client.c = c

	// Mark bridge as online
	client.publish(cfg.TopicStatus, "online", true)

	// Subscribe to outbound send topic
	token := c.Subscribe(cfg.TopicSend, 1, func(_ paho.Client, msg paho.Message) {
		var req SendRequest
		if err := json.Unmarshal(msg.Payload(), &req); err != nil {
			log.Error().Err(err).Msg("invalid send payload")
			return
		}
		client.sendCh <- req
	})
	if token.WaitTimeout(5*time.Second) && token.Error() != nil {
		return nil, fmt.Errorf("mqtt subscribe: %w", token.Error())
	}

	log.Info().Str("topic", cfg.TopicSend).Msg("subscribed for outbound SMS")
	return client, nil
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
	c.publish(c.cfg.TopicInbox, string(payload), false)
	log.Info().Str("from", msg.From).Msg("published SMS to MQTT")
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

func (c *Client) publish(topic, payload string, retain bool) {
	token := c.c.Publish(topic, 1, retain, payload)
	if !token.WaitTimeout(5 * time.Second) {
		log.Warn().Str("topic", topic).Msg("mqtt publish timeout")
	}
}

func (c *Client) Disconnect() {
	c.publish(c.cfg.TopicStatus, "offline", true)
	c.c.Disconnect(500)
}
