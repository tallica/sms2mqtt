package bot

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

func Status(version string) Command {
	return Command{
		Match:  func(body string) bool { return body == "status" },
		Handle: func(_, _ string) string { return "sms2mqtt " + version },
	}
}
