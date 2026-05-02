package modem

import (
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
		if strings.HasPrefix(line, "+CSQ:") {
			var rssi int
			fmt.Sscanf(strings.TrimSpace(strings.TrimPrefix(line, "+CSQ:")), "%d", &rssi)
			if rssi == 99 {
				return 0, false, nil
			}
			return -113 + 2*rssi, true, nil
		}
	}
	return 0, false, fmt.Errorf("+CSQ response missing")
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
		if strings.HasPrefix(line, "+CREG:") {
			parts := strings.TrimSpace(strings.TrimPrefix(line, "+CREG:"))
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
	}
	return "unknown", fmt.Errorf("+CREG response missing")
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
		if strings.HasPrefix(line, "+CPIN:") {
			state := strings.TrimSpace(strings.TrimPrefix(line, "+CPIN:"))
			switch state {
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
	}
	return "error", fmt.Errorf("+CPIN response missing")
}
