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
