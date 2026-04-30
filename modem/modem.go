package modem

import (
	"bufio"
	"fmt"
	"strings"
	"time"

	"go.bug.st/serial"
)

type Modem struct {
	port   serial.Port
	reader *bufio.Reader
}

func Open(device string, baud int) (*Modem, error) {
	port, err := serial.Open(device, &serial.Mode{BaudRate: baud})
	if err != nil {
		return nil, fmt.Errorf("open serial %s: %w", device, err)
	}

	m := &Modem{
		port:   port,
		reader: bufio.NewReader(port),
	}

	// Flush any stale data
	_ = port.SetReadTimeout(200 * time.Millisecond)
	m.drain()
	_ = port.SetReadTimeout(5 * time.Second)

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
	for _, cmd := range []string{
		"ATZ",          // reset
		"ATE0",         // echo off
		"AT+CMGF=1",   // SMS text mode
		"AT+CSCS=\"GSM\"", // GSM character set
		"AT+CNMI=0,0,0,0,0", // disable SMS push notifications (we poll)
	} {
		if _, err := m.Command(cmd); err != nil {
			return fmt.Errorf("init command %q: %w", cmd, err)
		}
	}
	return nil
}

// Command sends an AT command and returns the response lines (without OK/ERROR).
func (m *Modem) Command(cmd string) ([]string, error) {
	_ = m.port.SetReadTimeout(5 * time.Second)

	if _, err := fmt.Fprintf(m.port, "%s\r\n", cmd); err != nil {
		return nil, fmt.Errorf("write: %w", err)
	}

	var lines []string
	for {
		line, err := m.reader.ReadString('\n')
		line = strings.TrimSpace(line)
		if err != nil || line == "" {
			if err != nil {
				return nil, fmt.Errorf("read: %w", err)
			}
			continue
		}
		switch line {
		case "OK":
			return lines, nil
		case "ERROR", "NO CARRIER":
			return nil, fmt.Errorf("modem error for %q: %s", cmd, line)
		default:
			// skip echo
			if line != cmd {
				lines = append(lines, line)
			}
		}
	}
}

// CommandRaw sends a raw byte sequence (used for SMS body + Ctrl-Z).
func (m *Modem) CommandRaw(data []byte) error {
	_, err := m.port.Write(data)
	return err
}

// drain reads and discards all pending data.
func (m *Modem) drain() {
	buf := make([]byte, 256)
	for {
		n, err := m.port.Read(buf)
		if err != nil || n == 0 {
			return
		}
	}
}
