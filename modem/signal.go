package modem

import (
	"errors"
	"fmt"
	"strings"
)

// SignalStrength queries AT+CSQ and returns signal strength in dBm.
// ok=false means the modem reported no signal (CSQ=99).
func (m *Modem) SignalStrength() (dbm int, ok bool, err error) {
	lines, err := m.Command("AT+CSQ")
	if err != nil {
		return 0, false, err
	}
	for _, line := range lines {
		rest, found := strings.CutPrefix(line, "+CSQ:")
		if !found {
			continue
		}
		var rssi int
		if _, err := fmt.Sscanf(strings.TrimSpace(rest), "%d", &rssi); err != nil {
			return 0, false, fmt.Errorf("parse +CSQ: %w", err)
		}
		if rssi == 99 {
			return 0, false, nil
		}
		return -113 + 2*rssi, true, nil
	}
	return 0, false, errors.New("+CSQ response missing")
}

// SignalLevel maps a dBm value to a human-readable level.
func SignalLevel(dbm int) string {
	switch {
	case dbm >= -73:
		return "excellent"
	case dbm >= -83:
		return "good"
	case dbm >= -93:
		return "fair"
	default:
		return "poor"
	}
}

// NetworkRegistration queries AT+CREG? and returns the registration status.
func (m *Modem) NetworkRegistration() (string, error) {
	lines, err := m.Command("AT+CREG?")
	if err != nil {
		return "unknown", err
	}
	for _, line := range lines {
		rest, found := strings.CutPrefix(line, "+CREG:")
		if !found {
			continue
		}
		parts := strings.TrimSpace(rest)
		var n, stat int
		if count, _ := fmt.Sscanf(parts, "%d,%d", &n, &stat); count < 2 {
			stat = n // single-value response: the value is stat
		}
		switch stat {
		case 1:
			return "registered", nil
		case 5:
			return "roaming", nil
		case 2:
			return "searching", nil
		case 3:
			return "denied", nil
		case 0:
			return "not_registered", nil
		default:
			return "unknown", nil
		}
	}
	return "unknown", errors.New("+CREG response missing")
}

// Operator queries AT+COPS? and returns the registered operator name.
// Returns an empty string when the modem is not registered on any network.
func (m *Modem) Operator() (string, error) {
	lines, err := m.Command("AT+COPS?")
	if err != nil {
		return "", err
	}
	for _, line := range lines {
		rest, found := strings.CutPrefix(line, "+COPS:")
		if !found {
			continue
		}
		fields := strings.SplitN(strings.TrimSpace(rest), ",", 4)
		if len(fields) >= 3 {
			return strings.Trim(fields[2], `"`), nil
		}
		return "", nil // not registered — no operator field
	}
	return "", errors.New("+COPS response missing")
}

// SIMStatus queries AT+CPIN? and returns the SIM state.
func (m *Modem) SIMStatus() (string, error) {
	lines, err := m.Command("AT+CPIN?")
	if err != nil {
		// +CME ERROR: 10 = SIM not inserted
		if strings.Contains(err.Error(), "CME ERROR") {
			return "absent", nil
		}
		return "error", err
	}
	for _, line := range lines {
		state, found := strings.CutPrefix(line, "+CPIN:")
		if !found {
			continue
		}
		switch strings.TrimSpace(state) {
		case "READY":
			return "ready", nil
		case "SIM PIN":
			return "pin_required", nil
		case "SIM PUK":
			return "puk_required", nil
		default:
			return "error", nil
		}
	}
	return "error", errors.New("+CPIN response missing")
}
