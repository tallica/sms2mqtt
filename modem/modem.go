package modem

import (
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.bug.st/serial"
)

type Modem struct {
	port serial.Port
}

func Open(device string, baud int) (*Modem, error) {
	port, err := serial.Open(device, &serial.Mode{BaudRate: baud})
	if err != nil {
		return nil, fmt.Errorf("open serial %s: %w", device, err)
	}

	m := &Modem{port: port}

	m.drain()

	if err := m.init(); err != nil {
		port.Close()
		return nil, err
	}

	return m, nil
}

func (m *Modem) Close() error {
	return m.port.Close()
}

// init puts the modem into a known state.
func (m *Modem) init() error {
	required := []string{
		"ATZ",               // reset
		"ATE0",              // echo off
		"AT+CMGF=1",        // SMS text mode
		"AT+CNMI=0,0,0,0,0", // disable SMS push notifications (we poll)
	}
	for _, cmd := range required {
		if _, err := m.Command(cmd); err != nil {
			return fmt.Errorf("init command %q: %w", cmd, err)
		}
	}

	// Optional: set GSM charset; may fail if SIM is absent
	if _, err := m.Command(`AT+CSCS="GSM"`); err != nil {
		log.Warn().Err(err).Msg("AT+CSCS failed, using modem default charset")
	}

	return nil
}

// Command sends an AT command and returns the response lines (without OK/ERROR).
func (m *Modem) Command(cmd string) ([]string, error) {
	if _, err := fmt.Fprintf(m.port, "%s\r\n", cmd); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	var lines []string
	deadline := time.Now().Add(5 * time.Second)
	for {
		line, err := m.readLine(deadline)
		if err != nil {
			return nil, fmt.Errorf("command %q: %w", cmd, err)
		}
		if line == "" || line == cmd {
			continue // skip blank lines and echo
		}
		switch {
		case line == "OK":
			return lines, nil
		case line == "ERROR", line == "NO CARRIER",
			strings.HasPrefix(line, "+CME ERROR"),
			strings.HasPrefix(line, "+CMS ERROR"):
			return nil, fmt.Errorf("%s", line)
		default:
			lines = append(lines, line)
		}
	}
}

// CommandRaw sends a raw byte sequence (used for SMS body + Ctrl-Z).
func (m *Modem) CommandRaw(data []byte) error {
	_, err := m.port.Write(data)
	return err
}

// readLine reads one CR/LF-terminated line from the port, respecting a deadline.
// go.bug.st/serial returns (0, nil) on timeout, so we poll with a short interval
// and check the deadline ourselves rather than relying on bufio.
func (m *Modem) readLine(deadline time.Time) (string, error) {
	_ = m.port.SetReadTimeout(100 * time.Millisecond)
	var buf []byte
	b := make([]byte, 1)
	for time.Now().Before(deadline) {
		n, err := m.port.Read(b)
		if err != nil {
			return "", err
		}
		if n == 0 {
			continue // poll interval elapsed, keep waiting
		}
		if b[0] == '\n' {
			return strings.TrimSpace(string(buf)), nil
		}
		if b[0] != '\r' {
			buf = append(buf, b[0])
		}
	}
	return "", fmt.Errorf("read timeout")
}

// drain discards any bytes already waiting in the modem's buffer.
func (m *Modem) drain() {
	_ = m.port.SetReadTimeout(200 * time.Millisecond)
	buf := make([]byte, 256)
	for {
		n, err := m.port.Read(buf)
		if err != nil || n == 0 {
			return
		}
	}
}
