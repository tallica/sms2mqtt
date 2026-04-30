package modem

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type SMS struct {
	Index int
	From  string
	Time  time.Time
	Body  string
}

// ListSMS returns all SMS messages currently stored on the modem.
func (m *Modem) ListSMS() ([]SMS, error) {
	lines, err := m.Command(`AT+CMGL="ALL"`)
	if err != nil {
		return nil, err
	}
	return parseSMSList(lines)
}

// DeleteSMS removes a message by its index.
func (m *Modem) DeleteSMS(index int) error {
	_, err := m.Command(fmt.Sprintf("AT+CMGD=%d", index))
	return err
}

// SendSMS sends a message to the given number using PDU mode with UCS-2 encoding,
// which supports the full Unicode range including emoji. Text mode is restored afterwards.
func (m *Modem) SendSMS(to, body string) error {
	pdu, n, err := buildPDU(to, body)
	if err != nil {
		return fmt.Errorf("encode PDU: %w", err)
	}

	if _, err := m.Command("AT+CMGF=0"); err != nil {
		return fmt.Errorf("PDU mode: %w", err)
	}
	defer func() {
		if _, err := m.Command("AT+CMGF=1"); err != nil {
			log.Warn().Err(err).Msg("restore text mode after send")
		}
	}()

	// Modem replies with "> " prompt (not OK), so write directly and wait
	if _, err := fmt.Fprintf(m.port, "AT+CMGS=%d\r", n); err != nil {
		return fmt.Errorf("send header: %w", err)
	}
	time.Sleep(300 * time.Millisecond)

	payload := append([]byte(pdu), 0x1A)
	if err := m.CommandRaw(payload); err != nil {
		return fmt.Errorf("send body: %w", err)
	}

	// SMS send can take several seconds
	deadline := time.Now().Add(15 * time.Second)
	for {
		line, err := m.readLine(deadline)
		if err != nil {
			return fmt.Errorf("send response: %w", err)
		}
		if line == "" {
			continue
		}
		if line == "OK" || strings.HasPrefix(line, "+CMGS:") {
			return nil
		}
		if line == "ERROR" || strings.HasPrefix(line, "+CMS ERROR") {
			return fmt.Errorf("modem rejected send: %s", line)
		}
	}
}

// parseSMSList parses the output of AT+CMGL="ALL".
//
// Example raw output (text mode):
//   +CMGL: 1,"REC UNREAD","+48123456789",,"24/04/30,12:00:00+08"
//   Hello world
//   +CMGL: 2,"REC READ","+48987654321",,"24/04/30,13:00:00+08"
//   Another message
func parseSMSList(lines []string) ([]SMS, error) {
	var messages []SMS
	var current *SMS

	for _, line := range lines {
		if strings.HasPrefix(line, "+CMGL:") {
			if current != nil {
				messages = append(messages, *current)
			}
			sms, err := parseCMGLHeader(line)
			if err != nil {
				return nil, err
			}
			current = &sms
		} else if current != nil {
			if current.Body != "" {
				current.Body += "\n"
			}
			current.Body += line
		}
	}
	if current != nil {
		messages = append(messages, *current)
	}
	return messages, nil
}

// parseCMGLHeader parses a +CMGL header line.
func parseCMGLHeader(line string) (SMS, error) {
	// +CMGL: <index>,"<status>","<from>",,"<time>"
	line = strings.TrimPrefix(line, "+CMGL: ")
	parts := splitCSV(line)
	if len(parts) < 3 {
		return SMS{}, fmt.Errorf("unexpected CMGL format: %q", line)
	}

	var idx int
	fmt.Sscanf(parts[0], "%d", &idx)

	from := strings.Trim(parts[2], "\"")
	ts := ""
	if len(parts) >= 5 {
		ts = strings.Trim(parts[4], "\"")
	}

	t := parseModemTime(ts)

	return SMS{Index: idx, From: from, Time: t}, nil
}

// parseModemTime parses the modem timestamp format: "YY/MM/DD,HH:MM:SS±ZZ"
func parseModemTime(s string) time.Time {
	s = strings.Trim(s, "\"")
	// Try with timezone offset suffix (e.g. +08 meaning +2h)
	layouts := []string{
		"06/01/02,15:04:05-07",
		"06/01/02,15:04:05+07",
		"06/01/02,15:04:05",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Now()
}

// splitCSV splits a comma-separated string respecting quoted fields.
func splitCSV(s string) []string {
	var fields []string
	var cur strings.Builder
	inQuote := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuote = !inQuote
			cur.WriteRune(r)
		case r == ',' && !inQuote:
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	fields = append(fields, cur.String())
	return fields
}
