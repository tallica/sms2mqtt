package modem

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"unicode/utf16"
)

// decodeUCS2Hex decodes a UCS-2 big-endian hex string as the Huawei E3272 returns
// for non-GSM-7 incoming SMS in text mode. Returns the original string unchanged if
// it does not look like UCS-2 hex (odd length, non-hex chars, or decode failure).
func decodeUCS2Hex(s string) string {
	if len(s) == 0 || len(s)%4 != 0 {
		return s
	}
	up := strings.ToUpper(s)
	for _, c := range up {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'F')) {
			return s
		}
	}
	b, err := hex.DecodeString(up)
	if err != nil {
		return s
	}
	u16 := make([]uint16, len(b)/2)
	for i := range u16 {
		u16[i] = uint16(b[i*2])<<8 | uint16(b[i*2+1])
	}
	return string(utf16.Decode(u16))
}

// buildPDUs constructs one or more SMS-SUBMIT PDUs with UCS-2 encoding.
// Messages exceeding 140 bytes of encoded user data are split into concatenated
// parts with a 6-byte UDH, allowing up to 134 bytes (67 UCS-2 chars) per part.
// Returns hex PDU strings and their AT+CMGS octet counts.
func buildPDUs(to, body string) (pdus []string, ns []int, err error) {
	da, err := encodeAddress(to)
	if err != nil {
		return nil, nil, err
	}
	ud := encodeUCS2(body)

	const singleMax = 140 // max UD bytes in a single-part SMS
	const partMax   = 134 // max UD bytes per part when UDH present (140 − 6)

	if len(ud) <= singleMax {
		pdu, n := assemblePDU(0x11, da, nil, ud)
		return []string{pdu}, []int{n}, nil
	}

	parts := splitUCS2(ud, partMax)
	ref := byte(rand.Intn(256))
	for i, part := range parts {
		udh := []byte{0x05, 0x00, 0x03, ref, byte(len(parts)), byte(i + 1)}
		pdu, n := assemblePDU(0x51, da, udh, part) // 0x51 = SMS-SUBMIT + UDHI
		pdus = append(pdus, pdu)
		ns = append(ns, n)
	}
	return
}

// assemblePDU builds a single SMS-SUBMIT PDU.
// submitByte is 0x11 for single-part or 0x51 (UDHI set) for multipart.
func assemblePDU(submitByte byte, da, udh, ud []byte) (string, int) {
	var pdu []byte
	pdu = append(pdu, 0x00)                       // SMSC length: use SIM default
	pdu = append(pdu, submitByte)                  // SMS-SUBMIT [+UDHI]
	pdu = append(pdu, 0x00)                        // message reference
	pdu = append(pdu, da...)
	pdu = append(pdu, 0x00)                        // PID: standard SMS
	pdu = append(pdu, 0x08)                        // DCS: UCS-2
	pdu = append(pdu, 0xAA)                        // VP: 4 days
	pdu = append(pdu, byte(len(udh)+len(ud)))      // UDL in bytes
	pdu = append(pdu, udh...)
	pdu = append(pdu, ud...)
	// AT+CMGS=n expects n = octets excluding the SMSC info (first byte is 0x00 = 1 byte)
	n := len(pdu) - 1
	return strings.ToUpper(hex.EncodeToString(pdu)), n
}

// splitUCS2 splits UTF-16BE encoded bytes into chunks of at most maxBytes,
// never splitting a surrogate pair.
func splitUCS2(ud []byte, maxBytes int) [][]byte {
	var parts [][]byte
	for len(ud) > maxBytes {
		split := maxBytes
		// Back up if the last code unit is a high surrogate (0xD800–0xDBFF)
		if hi := uint16(ud[split-2])<<8 | uint16(ud[split-1]); hi >= 0xD800 && hi <= 0xDBFF {
			split -= 2
		}
		parts = append(parts, ud[:split])
		ud = ud[split:]
	}
	return append(parts, ud)
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
