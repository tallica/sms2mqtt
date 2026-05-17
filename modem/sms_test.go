package modem

import (
	"testing"
	"time"
)

func TestParsePDUList(t *testing.T) {
	t.Parallel()
	pdu := "00" + "04" + "0B91" + "1346610089F6" + "00" + "08" +
		"20806291731480" + "0A" + "00480065006C006C006F"

	lines := []string{
		"+CMGL: 3,1,,33",
		pdu,
		"+CMGL: 5,1,,33",
		pdu,
	}
	parts := parsePDUList(lines)
	if len(parts) != 2 {
		t.Fatalf("got %d parts, want 2", len(parts))
	}
	if parts[0].index != 3 || parts[1].index != 5 {
		t.Errorf("indices = %d, %d", parts[0].index, parts[1].index)
	}
	if parts[0].body != "Hello" {
		t.Errorf("body = %q", parts[0].body)
	}
}

func TestParsePDUListSkipsBadPDU(t *testing.T) {
	t.Parallel()
	lines := []string{
		"+CMGL: 1,1,,33",
		"NOTHEX!!",
		"+CMGL: 2,1,,33",
		"00" + "04" + "0B91" + "1346610089F6" + "00" + "08" +
			"20806291731480" + "0A" + "00480065006C006C006F",
	}
	parts := parsePDUList(lines)
	if len(parts) != 1 {
		t.Fatalf("got %d parts, want 1 (bad PDU should be skipped)", len(parts))
	}
	if parts[0].index != 2 {
		t.Errorf("index = %d, want 2", parts[0].index)
	}
}

func TestReassembleMultipart_SinglePart(t *testing.T) {
	t.Parallel()
	parts := []rawSMSPart{
		{index: 1, from: "+1", time: time.Unix(100, 0), body: "alone"},
	}
	got := reassembleMultipart(parts)
	if len(got) != 1 || got[0].Body != "alone" {
		t.Errorf("got %+v", got)
	}
	if len(got[0].Indices) != 1 || got[0].Indices[0] != 1 {
		t.Errorf("indices = %v", got[0].Indices)
	}
}

func TestReassembleMultipart_Concat(t *testing.T) {
	t.Parallel()
	t1 := time.Unix(200, 0)
	t2 := time.Unix(100, 0)
	parts := []rawSMSPart{
		{index: 5, from: "+1", time: t2, body: " world", concat: &concatInfo{ref: 7, total: 2, part: 2}},
		{index: 4, from: "+1", time: t2, body: "hello", concat: &concatInfo{ref: 7, total: 2, part: 1}},
		{index: 9, from: "+2", time: t1, body: "later"},
	}
	got := reassembleMultipart(parts)
	if len(got) != 2 {
		t.Fatalf("got %d, want 2", len(got))
	}
	if got[0].Body != "hello world" {
		t.Errorf("body[0] = %q", got[0].Body)
	}
	if len(got[0].Indices) != 2 || got[0].Indices[0] != 4 || got[0].Indices[1] != 5 {
		t.Errorf("indices[0] = %v, want [4 5]", got[0].Indices)
	}
	if got[1].Body != "later" {
		t.Errorf("body[1] = %q", got[1].Body)
	}
}

func TestReassembleMultipart_IncompleteHeldBack(t *testing.T) {
	t.Parallel()
	parts := []rawSMSPart{
		{index: 1, from: "+1", time: time.Unix(100, 0), body: "a", concat: &concatInfo{ref: 1, total: 3, part: 1}},
		{index: 2, from: "+1", time: time.Unix(100, 0), body: "b", concat: &concatInfo{ref: 1, total: 3, part: 2}},
	}
	got := reassembleMultipart(parts)
	if len(got) != 0 {
		t.Errorf("expected 0 messages (incomplete group held back), got %d", len(got))
	}
}

func TestReassembleMultipart_DifferentSendersNotMerged(t *testing.T) {
	t.Parallel()
	parts := []rawSMSPart{
		{index: 1, from: "+1", time: time.Unix(100, 0), body: "x", concat: &concatInfo{ref: 5, total: 2, part: 1}},
		{index: 2, from: "+2", time: time.Unix(100, 0), body: "y", concat: &concatInfo{ref: 5, total: 2, part: 2}},
	}
	got := reassembleMultipart(parts)
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}
