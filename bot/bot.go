package bot

import (
	"fmt"
	"strings"
	"time"
)

// Command matches an incoming SMS body and produces a reply.
type Command struct {
	Match  func(body string) bool
	Handle func(from, body string) string
}

// Bot dispatches incoming SMS to the first matching command.
type Bot struct {
	commands []Command
}

func New(commands ...Command) *Bot {
	return &Bot{commands: commands}
}

// Reply returns the reply for the first matching command, or ("", false) if none match.
func (b *Bot) Reply(from, body string) (string, bool) {
	for _, cmd := range b.commands {
		if cmd.Match(body) {
			return cmd.Handle(from, body), true
		}
	}
	return "", false
}

func Ping() Command {
	return Command{
		Match:  func(body string) bool { return body == "ping" },
		Handle: func(_, _ string) string { return "pong" },
	}
}

func Version(version string) Command {
	return Command{
		Match:  func(body string) bool { return body == "version" },
		Handle: func(_, _ string) string { return "sms2mqtt " + version },
	}
}

// Status reports version, uptime, signal, network registration, and SIM state.
func Status(
	version string,
	uptime func() time.Duration,
	signal func() (int, bool, error),
	network func() (string, error),
	sim func() (string, error),
) Command {
	return Command{
		Match: func(body string) bool { return body == "status" },
		Handle: func(_, _ string) string {
			parts := []string{"sms2mqtt " + version}
			parts = append(parts, "up "+fmtDuration(uptime()))
			if dbm, ok, _ := signal(); ok {
				parts = append(parts, fmt.Sprintf("%d dBm", dbm))
			}
			if net, err := network(); err == nil {
				parts = append(parts, "net "+fmtNetwork(net))
			}
			if s, err := sim(); err == nil {
				parts = append(parts, "sim "+fmtSIM(s))
			}
			return strings.Join(parts, " | ")
		},
	}
}

func fmtNetwork(s string) string {
	switch s {
	case "registered":
		return "home"
	case "roaming":
		return "roam"
	case "searching":
		return "search"
	case "not_registered":
		return "no net"
	default:
		return s
	}
}

func fmtSIM(s string) string {
	switch s {
	case "ready":
		return "ok"
	case "pin_required":
		return "PIN?"
	case "puk_required":
		return "PUK!"
	default:
		return s
	}
}

func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	d = d.Round(time.Minute)
	days := int(d.Hours()) / 24
	h := int(d.Hours()) % 24
	m := int(d.Minutes()) % 60
	if days > 0 {
		return fmt.Sprintf("%dd%dh%dm", days, h, m)
	}
	if h > 0 {
		return fmt.Sprintf("%dh%dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
