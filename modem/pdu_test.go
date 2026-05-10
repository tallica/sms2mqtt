package modem

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestDecodeBCDAddr(t *testing.T) {
	tests := []struct {
		name      string
		bcd       string
		ton       byte
		numDigits int
		want      string
	}{
		{"international even", "1346610089F6", 0x91, 11, "+31641600986"},
		{"international odd padded", "21436587F9", 0x91, 9, "+123456789"},
		{"national even", "214365", 0x81, 6, "123456"},
		{"national odd padded", "2143F5", 0x81, 5, "12345"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := hex.DecodeString(tt.bcd)
			if err != nil {
				t.Fatal(err)
			}
			got := decodeBCDAddr(b, tt.ton, tt.numDigits)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeSCTS(t *testing.T) {
	// Year=02, Mon=08, Day=26, Hr=19, Min=37, Sec=41, TZ=+02:00 (8 quarters).
	// Stored with semi-octet swap: 20 80 62 91 73 14 80.
	b, _ := hex.DecodeString("20806291731480")
	got := decodeSCTS(b)
	want := time.Date(2002, 8, 26, 19, 37, 41, 0, time.FixedZone("", 2*3600))
	if !got.Equal(want) {
		t.Errorf("got %s, want %s", got, want)
	}
	_, off := got.Zone()
	if off != 2*3600 {
		t.Errorf("tz offset = %d, want %d", off, 2*3600)
	}
}

func TestDecodeSCTSNegativeTZ(t *testing.T) {
	// TZ -05:00 = 20 quarters. Digits "2","0", stored swapped = 0x02, sign bit set in
	// high nibble (0x08 mask on stored low-nibble bit-3): 0x02 | 0x08 = 0x0A.
	b, _ := hex.DecodeString("2080629173140A")
	got := decodeSCTS(b)
	_, off := got.Zone()
	if off != -5*3600 {
		t.Errorf("tz offset = %d, want %d", off, -5*3600)
	}
}

func TestParseUDH(t *testing.T) {
	t.Run("IEI 0x00 8-bit ref", func(t *testing.T) {
		// IEI=00, IEDL=03, ref=AA, total=03, part=02
		udh, _ := hex.DecodeString("0003AA0302")
		got := parseUDH(udh)
		if got == nil {
			t.Fatal("expected concatInfo")
		}
		if got.ref != 0xAA || got.total != 3 || got.part != 2 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("IEI 0x08 16-bit ref", func(t *testing.T) {
		// IEI=08, IEDL=04, ref=1234, total=04, part=03
		udh, _ := hex.DecodeString("080412340403")
		got := parseUDH(udh)
		if got == nil {
			t.Fatal("expected concatInfo")
		}
		if got.ref != 0x1234 || got.total != 4 || got.part != 3 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("non-concat IE skipped", func(t *testing.T) {
		// Unknown IEI 0x0A with 2 bytes, then concat 8-bit IE.
		udh, _ := hex.DecodeString("0A02FFFF0003AA0201")
		got := parseUDH(udh)
		if got == nil || got.ref != 0xAA || got.total != 2 || got.part != 1 {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("no concat returns nil", func(t *testing.T) {
		udh, _ := hex.DecodeString("0A02FFFF")
		if got := parseUDH(udh); got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("truncated IE", func(t *testing.T) {
		udh, _ := hex.DecodeString("0005AA")
		if got := parseUDH(udh); got != nil {
			t.Errorf("expected nil for truncated UDH, got %+v", got)
		}
	})
}

func TestDecodeGSM7Hello(t *testing.T) {
	// "hello" packed: E8329BFD06 (5 septets, no fill bits).
	b, _ := hex.DecodeString("E8329BFD06")
	got := decodeGSM7(b, 5, 0)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestDecodeGSM7WithFillBits(t *testing.T) {
	// Build "hi" (2 septets) with 1 fill bit prefix to simulate UDH alignment.
	// Septets: 'h'=0x68, 'i'=0x69. With 1 fill bit:
	// bit positions 1..7 hold first septet 0x68, bits 8..14 second septet 0x69.
	// byte0 = (0x68 << 1) & 0xFF = 0xD0; byte1 = (0x68 >> 7) | (0x69 << 0) shifted... let's compute directly.
	// pos0 septet at bit 1 of byte 0: byte0 |= 0x68 << 1 = 0xD0
	// pos1 septet at bit 8 (= byte1 bit 0): byte1 |= 0x69 = 0x69
	// (the high bit of 0x68 also lands in byte1 bit 0... 0x68 has bit 6 set, no bit 7. Actually 0x68<<1 = 0xD0; high bit shifted out is 0, so byte1 stays 0x69.)
	got := decodeGSM7([]byte{0xD0, 0x69}, 2, 1)
	if got != "hi" {
		t.Errorf("got %q, want %q", got, "hi")
	}
}

func TestDecodeGSM7Extension(t *testing.T) {
	// ESC (0x1B) followed by '€' code 0x65. Two septets.
	// pack: byte0 = 0x1B | (0x65 << 7) & 0xFF = 0x1B | 0x80 = 0x9B
	//       byte1 = 0x65 >> 1 = 0x32
	got := decodeGSM7([]byte{0x9B, 0x32}, 2, 0)
	if got != "€" {
		t.Errorf("got %q, want %q", got, "€")
	}
}

func TestDecodeUCS2Bytes(t *testing.T) {
	// "Héllo" in UTF-16BE.
	b := []byte{0x00, 'H', 0x00, 0xE9, 0x00, 'l', 0x00, 'l', 0x00, 'o'}
	got := decodeUCS2Bytes(b)
	if got != "Héllo" {
		t.Errorf("got %q", got)
	}
}

func TestDecodeUCS2BytesSurrogatePair(t *testing.T) {
	// 🙂 U+1F642 → surrogates D83D DE42.
	b := []byte{0xD8, 0x3D, 0xDE, 0x42}
	got := decodeUCS2Bytes(b)
	if got != "🙂" {
		t.Errorf("got %q", got)
	}
}

func TestDecodeUCS2BytesOddLength(t *testing.T) {
	b := []byte{0x00, 'A', 0x00}
	got := decodeUCS2Bytes(b)
	if got != "A" {
		t.Errorf("got %q", got)
	}
}

func TestDecodeSMSDeliverPDU_UCS2(t *testing.T) {
	// Hand-crafted SMS-DELIVER:
	//   00              SMSC length 0
	//   04              first octet (SMS-DELIVER, no UDH)
	//   0B 91 1346610089F6   OA: 11 digits, international, +31641600986
	//   00              PID
	//   08              DCS: UCS-2
	//   20 80 62 91 73 14 80   SCTS: 2002-08-26 19:37:41 +02:00
	//   0A              UDL: 10 bytes
	//   00 48 00 65 00 6C 00 6C 00 6F   "Hello" UCS-2BE
	pdu := "00040B911346610089F600082080629173148000A00480065006C006C006F"
	// Re-build cleanly to avoid typos.
	pdu = "00" + "04" + "0B91" + "1346610089F6" + "00" + "08" +
		"20806291731480" + "0A" + "00480065006C006C006F"

	part, err := decodeSMSDeliverPDU(pdu, 7)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if part.index != 7 {
		t.Errorf("index = %d, want 7", part.index)
	}
	if part.from != "+31641600986" {
		t.Errorf("from = %q", part.from)
	}
	if part.body != "Hello" {
		t.Errorf("body = %q", part.body)
	}
	if part.concat != nil {
		t.Errorf("expected no concat, got %+v", part.concat)
	}
	want := time.Date(2002, 8, 26, 19, 37, 41, 0, time.FixedZone("", 2*3600))
	if !part.time.Equal(want) {
		t.Errorf("time = %s, want %s", part.time, want)
	}
}

func TestDecodeSMSDeliverPDU_GSM7(t *testing.T) {
	// SMS-DELIVER with GSM-7 "hello".
	//   00 04 0B 91 1346610089F6 00 00 (DCS=0)
	//   SCTS, UDL=5, UD = packed "hello" = E8329BFD06
	pdu := "00" + "04" + "0B91" + "1346610089F6" + "00" + "00" +
		"20806291731480" + "05" + "E8329BFD06"

	part, err := decodeSMSDeliverPDU(pdu, 1)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if part.body != "hello" {
		t.Errorf("body = %q, want hello", part.body)
	}
}

func TestDecodeSMSDeliverPDU_GSM7WithUDH(t *testing.T) {
	// SMS-DELIVER GSM-7 multipart with concat UDH.
	// First octet 0x44 = SMS-DELIVER + UDHI.
	// UDH: 05 (length) 00 03 AA 02 01 → 6 bytes total including length byte.
	// UDH bits = 48 → 7 septets (with 1 fill bit to align). Body "hi" = 2 septets.
	// UDL (in septets) = 7 + 2 = 9. Body packed with fill-bit-1: D0 69.
	pdu := "00" + "44" + "0B91" + "1346610089F6" + "00" + "00" +
		"20806291731480" + "09" + "050003AA0201" + "D069"

	part, err := decodeSMSDeliverPDU(pdu, 2)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if part.body != "hi" {
		t.Errorf("body = %q, want hi", part.body)
	}
	if part.concat == nil {
		t.Fatal("expected concat info")
	}
	if part.concat.ref != 0xAA || part.concat.total != 2 || part.concat.part != 1 {
		t.Errorf("concat = %+v", part.concat)
	}
}

func TestDecodeSMSDeliverPDU_Truncated(t *testing.T) {
	if _, err := decodeSMSDeliverPDU("00", 0); err == nil {
		t.Error("expected error on truncated PDU")
	}
}

func TestDecodeSMSDeliverPDU_BadHex(t *testing.T) {
	if _, err := decodeSMSDeliverPDU("ZZ", 0); err == nil {
		t.Error("expected hex decode error")
	}
}

func TestEncodeAddress(t *testing.T) {
	tests := []struct {
		name   string
		number string
		want   string // hex
	}{
		{"international even", "+31641600986", "0B911346610089F6"},
		{"international odd", "+123456789", "0991214365 87F9"},
		{"national even", "123456", "06812143 65"},
		{"national odd", "12345", "058121432 1F4"}, // sentinel; assert below by computing manually
	}
	// Compute expected dynamically rather than maintain typo-prone hex literals.
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeAddress(tt.number)
			if err != nil {
				t.Fatal(err)
			}
			// Re-decode to verify roundtrip.
			oaLen := got[0]
			ton := got[1]
			bcd := got[2:]
			back := decodeBCDAddr(bcd, ton, int(oaLen))
			want := tt.number
			if !strings.HasPrefix(want, "+") {
				// non-+ numbers come back without '+'
			}
			if back != want {
				t.Errorf("roundtrip = %q, want %q", back, want)
			}
		})
	}
}

func TestEncodeAddressInvalid(t *testing.T) {
	if _, err := encodeAddress(""); err == nil {
		t.Error("expected error for empty number")
	}
	if _, err := encodeAddress("+abc"); err == nil {
		t.Error("expected error for non-numeric")
	}
}

func TestEncodeUCS2Roundtrip(t *testing.T) {
	cases := []string{
		"Hello",
		"Zażółć gęślą jaźń",
		"こんにちは",
		"emoji 🙂🚀",
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			b := encodeUCS2(s)
			got := decodeUCS2Bytes(b)
			if got != s {
				t.Errorf("roundtrip: got %q, want %q", got, s)
			}
		})
	}
}

func TestSplitUCS2NoSurrogateSplit(t *testing.T) {
	// Build a string where naive split would land mid-surrogate-pair.
	// 33 ASCII chars (66 bytes) + emoji 🙂 (4 bytes) → 70 bytes total.
	// maxBytes=68 would split inside the 4-byte emoji at byte 68 (between hi and lo surrogate).
	s := strings.Repeat("a", 33) + "🙂"
	b := encodeUCS2(s)
	if len(b) != 70 {
		t.Fatalf("setup: encoded len = %d, want 70", len(b))
	}
	parts := splitUCS2(b, 68)
	// Reassemble and decode — surrogate pair must remain intact.
	var all []byte
	for _, p := range parts {
		all = append(all, p...)
	}
	if decodeUCS2Bytes(all) != s {
		t.Errorf("reassembled string mismatch")
	}
	// Verify no part ends mid-high-surrogate.
	for i, p := range parts {
		if len(p)%2 != 0 {
			t.Errorf("part %d has odd byte length", i)
		}
		if len(p) >= 2 {
			last := uint16(p[len(p)-2])<<8 | uint16(p[len(p)-1])
			if last >= 0xD800 && last <= 0xDBFF {
				t.Errorf("part %d ends with high surrogate", i)
			}
		}
	}
}

func TestSplitUCS2NoSplitNeeded(t *testing.T) {
	b := encodeUCS2("short")
	parts := splitUCS2(b, 134)
	if len(parts) != 1 {
		t.Errorf("expected 1 part, got %d", len(parts))
	}
}

func TestBuildPDUsSinglePart(t *testing.T) {
	pdus, ns, err := buildPDUs("+48123456789", "Hello")
	if err != nil {
		t.Fatal(err)
	}
	if len(pdus) != 1 || len(ns) != 1 {
		t.Fatalf("expected 1 PDU, got %d", len(pdus))
	}
	// n must equal byte length of TPDU (everything after SMSC info length byte).
	raw, _ := hex.DecodeString(pdus[0])
	if ns[0] != len(raw)-1 {
		t.Errorf("n = %d, want %d", ns[0], len(raw)-1)
	}
	// First octet 0x11 (SMS-SUBMIT, no UDHI).
	if raw[1] != 0x11 {
		t.Errorf("first octet = 0x%02X, want 0x11", raw[1])
	}
}

func TestBuildPDUsMultipart(t *testing.T) {
	// 71 chars = 142 UCS-2 bytes > 140 → multipart.
	body := strings.Repeat("a", 71)
	pdus, ns, err := buildPDUs("+48123456789", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(pdus) < 2 {
		t.Fatalf("expected ≥2 parts, got %d", len(pdus))
	}
	if len(pdus) != len(ns) {
		t.Fatal("pdus/ns length mismatch")
	}

	var refs []byte
	var totals []byte
	var rebuilt strings.Builder
	for i, p := range pdus {
		raw, _ := hex.DecodeString(p)
		if raw[1] != 0x51 {
			t.Errorf("part %d first octet = 0x%02X, want 0x51 (UDHI)", i, raw[1])
		}
		if ns[i] != len(raw)-1 {
			t.Errorf("part %d n = %d, want %d", i, ns[i], len(raw)-1)
		}

		// Walk fields to extract UD with UDH.
		r := &pduReader{b: raw}
		smscLen, _ := r.readByte()
		_ = r.skip(int(smscLen))
		_, _ = r.readByte() // first octet
		_, _ = r.readByte() // MR
		daLen, _ := r.readByte()
		_, _ = r.readByte()                          // TON
		_, _ = r.readN((int(daLen) + 1) / 2)         // BCD
		_, _ = r.readByte()                           // PID
		dcs, _ := r.readByte()
		if dcs != 0x08 {
			t.Errorf("part %d DCS = 0x%02X, want 0x08", i, dcs)
		}
		_, _ = r.readByte() // VP
		udl, _ := r.readByte()
		ud := r.rest()
		if int(udl) != len(ud) {
			t.Errorf("part %d UDL = %d, UD bytes = %d", i, udl, len(ud))
		}
		// Concat UDH: 05 00 03 ref total part
		if len(ud) < 6 || ud[0] != 0x05 || ud[1] != 0x00 || ud[2] != 0x03 {
			t.Errorf("part %d UDH unexpected: %X", i, ud[:min(6, len(ud))])
			continue
		}
		refs = append(refs, ud[3])
		totals = append(totals, ud[4])
		if int(ud[5]) != i+1 {
			t.Errorf("part %d index = %d, want %d", i, ud[5], i+1)
		}
		rebuilt.Write(ud[6:])
	}
	// All parts share the same ref and total.
	for i := 1; i < len(refs); i++ {
		if refs[i] != refs[0] {
			t.Errorf("ref mismatch at %d: %d vs %d", i, refs[i], refs[0])
		}
		if totals[i] != byte(len(pdus)) {
			t.Errorf("total mismatch at %d: %d", i, totals[i])
		}
	}
	if got := decodeUCS2Bytes([]byte(rebuilt.String())); got != body {
		t.Errorf("rebuilt body mismatch: got len %d, want %d", len(got), len(body))
	}
}

func TestBuildPDUsAtBoundary(t *testing.T) {
	// 70 UCS-2 chars = 140 bytes → single-part (boundary).
	body := strings.Repeat("x", 70)
	pdus, _, err := buildPDUs("+48123456789", body)
	if err != nil {
		t.Fatal(err)
	}
	if len(pdus) != 1 {
		t.Errorf("70 chars: expected 1 part, got %d", len(pdus))
	}
}

func TestBuildPDUsInvalidNumber(t *testing.T) {
	if _, _, err := buildPDUs("notanumber", "x"); err == nil {
		t.Error("expected error for invalid number")
	}
}

// Sanity check: utf16 round-trip helper used in tests matches encodeUCS2.
func TestEncodeUCS2MatchesUTF16BE(t *testing.T) {
	s := "🙂a"
	got := encodeUCS2(s)
	u16 := utf16.Encode([]rune(s))
	want := make([]byte, len(u16)*2)
	for i, v := range u16 {
		want[i*2] = byte(v >> 8)
		want[i*2+1] = byte(v)
	}
	if string(got) != string(want) {
		t.Errorf("encodeUCS2 mismatch")
	}
}
