# Backlog

## Bot command framework

Introduce a `bot` package with a dispatch table to replace the hardcoded ping/pong `if` in `pollSMS`.

**Design:**
- `bot.Command` struct with `Match func(body string) bool` and `Handle func(from, body string) string`
- `Bot` holds a `[]Command` slice, returns the first match's reply
- `main.go` calls `bot.Handle(sms.From, sms.Body)` and sends the reply if non-empty
- Match functions can be exact, prefix, or regex — per command

**Why:** scales cleanly beyond 3+ commands; keeps `main.go` as a wiring layer; each command is independently testable.
