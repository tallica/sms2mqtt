package modem

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode/utf16"
)

// buildPDU constructs an SMS-SUBMIT PDU with UCS-2 encoding.
// Returns the uppercase hex string and the octet count (excluding the SMSC length byte).
func buildPDU(to, body string) (string, int, error) {
	da, err := encodeAddress(to)
	if err != nil {
		return "", 0, err
	}

	ud := encodeUCS2(body)

	var pdu []byte
	pdu = append(pdu, 0x00)       // SMSC length: use SIM default
	pdu = append(pdu, 0x11)       // SMS-SUBMIT, VP=relative
	pdu = append(pdu, 0x00)       // message reference
	pdu = append(pdu, da...)
	pdu = append(pdu, 0x00)       // PID: standard SMS
	pdu = append(pdu, 0x08)       // DCS: UCS-2
	pdu = append(pdu, 0xAA)       // VP: 4 days
	pdu = append(pdu, byte(len(ud)))
	pdu = append(pdu, ud...)

	// AT+CMGS=n expects n = octets excluding the SMSC info (first byte is 0x00 = 1 byte)
	n := len(pdu) - 1
	return strings.ToUpper(hex.EncodeToString(pdu)), n, nil
}

func encodeAddress(number string) ([]byte, error) {
	tonNpi := byte(0x81) // unknown/national
	if strings.HasPrefix(number, "+") {
		tonNpi = 0x91 // international
		number = number[1:]
	}

	var digits strings.Builder
	for _, r := range number {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	d := digits.String()
	if d == "" {
		return nil, fmt.Errorf("invalid phone number: %q", number)
	}

	bcd := make([]byte, (len(d)+1)/2)
	for i := range bcd {
		lo := d[i*2] - '0'
		hi := byte(0x0F) // padding nibble for odd-length numbers
		if i*2+1 < len(d) {
			hi = d[i*2+1] - '0'
		}
		bcd[i] = hi<<4 | lo
	}

	result := []byte{byte(len(d)), tonNpi}
	return append(result, bcd...), nil
}

// encodeUCS2 encodes s as UTF-16 big-endian, with surrogate pairs for code points outside the BMP.
func encodeUCS2(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	buf := make([]byte, len(u16)*2)
	for i, v := range u16 {
		buf[i*2] = byte(v >> 8)
		buf[i*2+1] = byte(v)
	}
	return buf
}
