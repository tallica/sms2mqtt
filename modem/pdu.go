package modem

import (
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"
	"unicode/utf16"
)

// concatInfo holds the concatenated SMS header extracted from UDH.
type concatInfo struct {
	ref   uint16
	total uint8
	part  uint8
}

// rawSMSPart is a single PDU as received from the modem, before reassembly.
type rawSMSPart struct {
	index  int
	from   string
	time   time.Time
	body   string
	concat *concatInfo // nil if not a multipart segment
}

// gsm7Table is the GSM 03.38 / 3GPP TS 23.038 basic character set.
var gsm7Table = [128]rune{
	'@', '£', '$', '¥', 'è', 'é', 'ù', 'ì', 'ò', 'Ç', '\n', 'Ø', 'ø', '\r', 'Å', 'å',
	'Δ', '_', 'Φ', 'Γ', 'Λ', 'Ω', 'Π', 'Ψ', 'Σ', 'Θ', 'Ξ', 0, 'Æ', 'æ', 'ß', 'É',
	' ', '!', '"', '#', '¤', '%', '&', '\'', '(', ')', '*', '+', ',', '-', '.', '/',
	'0', '1', '2', '3', '4', '5', '6', '7', '8', '9', ':', ';', '<', '=', '>', '?',
	'¡', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O',
	'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', 'Ä', 'Ö', 'Ñ', 'Ü', '§',
	'¿', 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o',
	'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'ä', 'ö', 'ñ', 'ü', 'à',
}

// gsm7Ext is the GSM 03.38 extension table (reached after ESC 0x1B).
var gsm7Ext = map[byte]rune{
	0x0A: '\f', 0x14: '^', 0x28: '{', 0x29: '}', 0x2F: '\\',
	0x3C: '[', 0x3D: '~', 0x3E: ']', 0x40: '|', 0x65: '€',
}

// decodeSMSDeliverPDU decodes a hex-encoded SMS-DELIVER PDU from the modem.
func decodeSMSDeliverPDU(hexStr string, index int) (rawSMSPart, error) {
	b, err := hex.DecodeString(strings.ToUpper(strings.TrimSpace(hexStr)))
	if err != nil {
		return rawSMSPart{}, fmt.Errorf("hex decode: %w", err)
	}
	r := &pduReader{b: b}

	// Skip SMSC info.
	smscLen, err := r.readByte()
	if err != nil {
		return rawSMSPart{}, fmt.Errorf("SMSC length: %w", err)
	}
	if err := r.skip(int(smscLen)); err != nil {
		return rawSMSPart{}, fmt.Errorf("SMSC data: %w", err)
	}

	// PDU flags: bit 6 = UDHI.
	flags, err := r.readByte()
	if err != nil {
		return rawSMSPart{}, fmt.Errorf("PDU flags: %w", err)
	}
	udhi := flags&0x40 != 0

	// Originating address.
	oaDigits, err := r.readByte()
	if err != nil {
		return rawSMSPart{}, fmt.Errorf("OA length: %w", err)
	}
	oaTON, err := r.readByte()
	if err != nil {
		return rawSMSPart{}, fmt.Errorf("OA TON: %w", err)
	}
	oaBCD, err := r.readN((int(oaDigits) + 1) / 2)
	if err != nil {
		return rawSMSPart{}, fmt.Errorf("OA BCD: %w", err)
	}
	from := decodeBCDAddr(oaBCD, oaTON, int(oaDigits))

	// PID (skip).
	if _, err := r.readByte(); err != nil {
		return rawSMSPart{}, fmt.Errorf("PID: %w", err)
	}

	// DCS.
	dcs, err := r.readByte()
	if err != nil {
		return rawSMSPart{}, fmt.Errorf("DCS: %w", err)
	}

	// SCTS (7 bytes).
	scts, err := r.readN(7)
	if err != nil {
		return rawSMSPart{}, fmt.Errorf("SCTS: %w", err)
	}
	t := decodeSCTS(scts)

	// UDL.
	udl, err := r.readByte()
	if err != nil {
		return rawSMSPart{}, fmt.Errorf("UDL: %w", err)
	}

	ud := r.rest()

	var concat *concatInfo
	udhBytes := 0
	if udhi && len(ud) > 0 {
		udhBytes = int(ud[0]) + 1 // length byte + content
		if udhBytes > len(ud) {
			udhBytes = len(ud)
		}
		concat = parseUDH(ud[1:udhBytes])
		ud = ud[udhBytes:]
	}

	body := decodeUserData(dcs, ud, int(udl), udhi, udhBytes)
	return rawSMSPart{index: index, from: from, time: t, body: body, concat: concat}, nil
}

type pduReader struct {
	b   []byte
	pos int
}

func (r *pduReader) readByte() (byte, error) {
	if r.pos >= len(r.b) {
		return 0, fmt.Errorf("PDU truncated")
	}
	v := r.b[r.pos]
	r.pos++
	return v, nil
}

func (r *pduReader) skip(n int) error {
	if r.pos+n > len(r.b) {
		return fmt.Errorf("PDU truncated")
	}
	r.pos += n
	return nil
}

func (r *pduReader) readN(n int) ([]byte, error) {
	if r.pos+n > len(r.b) {
		return nil, fmt.Errorf("PDU truncated")
	}
	v := r.b[r.pos : r.pos+n]
	r.pos += n
	return v, nil
}

func (r *pduReader) rest() []byte {
	return r.b[r.pos:]
}

// decodeBCDAddr decodes a semi-octet BCD GSM address.
func decodeBCDAddr(bcd []byte, ton byte, numDigits int) string {
	var sb strings.Builder
	if ton == 0x91 {
		sb.WriteByte('+')
	}
	for i, b := range bcd {
		if i*2 < numDigits {
			sb.WriteByte('0' + b&0x0F)
		}
		if i*2+1 < numDigits {
			sb.WriteByte('0' + (b>>4)&0x0F)
		}
	}
	return sb.String()
}

// decodeSCTS decodes the 7-byte GSM Service Centre Time Stamp.
// Each byte is semi-octet BCD: low nibble = tens digit, high nibble = units digit.
// The timezone byte has a sign bit in bit 3 of the low nibble.
func decodeSCTS(b []byte) time.Time {
	bcd := func(v byte) int { return int(v&0x0F)*10 + int(v>>4) }
	year := 2000 + bcd(b[0])
	month := bcd(b[1])
	day := bcd(b[2])
	hour := bcd(b[3])
	min := bcd(b[4])
	sec := bcd(b[5])
	tzByte := b[6]
	negative := tzByte&0x08 != 0
	tzByte &^= 0x08
	tzSecs := bcd(tzByte) * 15 * 60
	if negative {
		tzSecs = -tzSecs
	}
	if year < 2000 || month < 1 || month > 12 || day < 1 || day > 31 {
		return time.Now()
	}
	return time.Date(year, time.Month(month), day, hour, min, sec, 0, time.FixedZone("", tzSecs))
}

// parseUDH scans a User Data Header for concatenated SMS info (IEI 0x00 or 0x08).
func parseUDH(udh []byte) *concatInfo {
	for i := 0; i+1 < len(udh); {
		iei := udh[i]
		ieLen := int(udh[i+1])
		i += 2
		if i+ieLen > len(udh) {
			break
		}
		ie := udh[i : i+ieLen]
		i += ieLen
		switch iei {
		case 0x00:
			if ieLen == 3 {
				return &concatInfo{ref: uint16(ie[0]), total: ie[1], part: ie[2]}
			}
		case 0x08:
			if ieLen == 4 {
				return &concatInfo{ref: uint16(ie[0])<<8 | uint16(ie[1]), total: ie[2], part: ie[3]}
			}
		}
	}
	return nil
}

// decodeUserData decodes SMS user data. ud must have the UDH already stripped.
// udl is the raw TP-UDL from the PDU (septets for GSM-7, bytes for UCS-2/8-bit).
// udhBytes is the total byte size of the UDH (including the length byte), used to
// compute the GSM-7 fill bits that align the UDH to a septet boundary.
func decodeUserData(dcs byte, ud []byte, udl int, udhi bool, udhBytes int) string {
	var charset byte
	switch (dcs >> 6) & 0x03 {
	case 0x00:
		charset = (dcs >> 2) & 0x03
	case 0x03:
		if dcs&0x04 != 0 {
			charset = 1
		}
	}

	switch charset {
	case 0x00: // GSM-7
		fillBits := 0
		messageSeptets := udl
		if udhi && udhBytes > 0 {
			fillBits = (7 - (udhBytes*8)%7) % 7
			messageSeptets = udl - (udhBytes*8+6)/7
		}
		if messageSeptets < 0 {
			messageSeptets = 0
		}
		return decodeGSM7(ud, messageSeptets, fillBits)
	case 0x02: // UCS-2
		return decodeUCS2Bytes(ud)
	default: // 8-bit data
		return string(ud)
	}
}

// decodeGSM7 unpacks numSeptets GSM-7 septets from b, skipping fillBits leading
// padding bits (present when UDH precedes the data to align to a 7-bit boundary).
func decodeGSM7(b []byte, numSeptets, fillBits int) string {
	var sb strings.Builder
	esc := false
	for i := 0; i < numSeptets; i++ {
		pos := fillBits + i*7
		byteIdx := pos / 8
		bitOff := uint(pos % 8)
		var s byte
		if byteIdx < len(b) {
			s = b[byteIdx] >> bitOff
			if bitOff > 1 && byteIdx+1 < len(b) {
				s |= b[byteIdx+1] << (8 - bitOff)
			}
			s &= 0x7F
		}
		if esc {
			esc = false
			if r, ok := gsm7Ext[s]; ok {
				sb.WriteRune(r)
			}
			continue
		}
		if s == 0x1B {
			esc = true
			continue
		}
		if int(s) < len(gsm7Table) && gsm7Table[s] != 0 {
			sb.WriteRune(gsm7Table[s])
		}
	}
	return sb.String()
}

// decodeUCS2Bytes decodes a UCS-2 big-endian byte slice.
func decodeUCS2Bytes(b []byte) string {
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
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
	const partMax = 134   // max UD bytes per part when UDH present (140 − 6)

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
	pdu = append(pdu, 0x00)                  // SMSC length: use SIM default
	pdu = append(pdu, submitByte)             // SMS-SUBMIT [+UDHI]
	pdu = append(pdu, 0x00)                  // message reference
	pdu = append(pdu, da...)
	pdu = append(pdu, 0x00)                  // PID: standard SMS
	pdu = append(pdu, 0x08)                  // DCS: UCS-2
	pdu = append(pdu, 0xAA)                  // VP: 4 days
	pdu = append(pdu, byte(len(udh)+len(ud))) // UDL in bytes
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
