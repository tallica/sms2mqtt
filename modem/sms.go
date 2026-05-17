package modem

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
)

type SMS struct {
	Indices []int
	From    string
	Time    time.Time
	Body    string
}

// ListSMS reads all SMS messages from the modem in PDU mode, reassembling any
// multipart (concatenated) messages into single entries before returning them.
func (m *Modem) ListSMS() ([]SMS, error) {
	if _, err := m.Command("AT+CMGF=0"); err != nil {
		return nil, fmt.Errorf("PDU mode: %w", err)
	}
	defer func() {
		if _, err := m.Command("AT+CMGF=1"); err != nil {
			log.Warn().Err(err).Msg("restore text mode after list SMS")
		}
	}()

	lines, err := m.Command("AT+CMGL=4")
	if err != nil {
		return nil, err
	}
	return reassembleMultipart(parsePDUList(lines)), nil
}

// DeleteSMS removes a message by its modem storage index.
func (m *Modem) DeleteSMS(index int) error {
	_, err := m.Command(fmt.Sprintf("AT+CMGD=%d", index))
	return err
}

// SendSMS sends a message using PDU mode with UCS-2 encoding. Messages longer than
// 70 UCS-2 characters are split into concatenated multipart SMS automatically.
// Text mode is restored after all parts are sent.
func (m *Modem) SendSMS(to, body string) error {
	pdus, ns, err := buildPDUs(to, body)
	if err != nil {
		return fmt.Errorf("encode PDU: %w", err)
	}

	if _, err := m.Command("AT+CMGF=0"); err != nil {
		return fmt.Errorf("PDU mode: %w", err)
	}
	defer func() {
		if _, err := m.Command("AT+CMGF=1"); err != nil {
			log.Warn().Err(err).Msg("restore text mode after send")
		}
	}()

	for i := range pdus {
		if err := m.sendPDU(pdus[i], ns[i]); err != nil {
			return err
		}
	}
	return nil
}

// sendPDU sends one PDU and waits for the modem's +CMGS:/OK response.
// Modem replies with "> " prompt (not OK), so the PDU is written directly.
// +CMGS: is always followed by OK — read both so the buffer is clean before
// the deferred AT+CMGF=1 restore runs; consuming only +CMGS: leaves a stale
// OK that Command() would misread as the CMGF=1 acknowledgement.
func (m *Modem) sendPDU(pdu string, n int) error {
	if _, err := fmt.Fprintf(m.port, "AT+CMGS=%d\r", n); err != nil {
		return fmt.Errorf("send header: %w", err)
	}
	time.Sleep(300 * time.Millisecond)

	if err := m.CommandRaw(append([]byte(pdu), 0x1A)); err != nil {
		return fmt.Errorf("send body: %w", err)
	}

	deadline := time.Now().Add(15 * time.Second)
	cmgsSeen := false
	for {
		line, err := m.readLine(deadline)
		if err != nil {
			return fmt.Errorf("send response: %w", err)
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "+CMGS:") {
			cmgsSeen = true
			continue
		}
		if line == "OK" && cmgsSeen {
			return nil
		}
		if line == "ERROR" || strings.HasPrefix(line, "+CMS ERROR") {
			return fmt.Errorf("modem rejected send: %s", line)
		}
	}
}

// parsePDUList parses AT+CMGL=4 output in PDU mode.
// Each message is a "+CMGL: <index>,..." header followed by a hex PDU line.
func parsePDUList(lines []string) []rawSMSPart {
	var parts []rawSMSPart
	var pendingIndex *int

	for _, line := range lines {
		if rest, found := strings.CutPrefix(line, "+CMGL: "); found {
			fields := strings.SplitN(rest, ",", 5)
			var idx int
			if _, err := fmt.Sscanf(fields[0], "%d", &idx); err != nil {
				log.Warn().Err(err).Str("field", fields[0]).Msg("parse +CMGL index, skipping")
				continue
			}
			pendingIndex = &idx
		} else if pendingIndex != nil && line != "" {
			part, err := decodeSMSDeliverPDU(line, *pendingIndex)
			if err != nil {
				log.Warn().Err(err).Int("index", *pendingIndex).Str("pdu", line).Msg("PDU decode failed, skipping")
			} else {
				parts = append(parts, part)
			}
			pendingIndex = nil
		}
	}
	return parts
}

// reassembleMultipart groups multipart SMS segments by (sender, ref) and
// concatenates them in order. Incomplete groups (not all parts present yet)
// are left on the modem and excluded from the result.
func reassembleMultipart(parts []rawSMSPart) []SMS {
	type groupKey struct {
		from string
		ref  uint16
	}

	groups := make(map[groupKey][]rawSMSPart)
	var groupOrder []groupKey
	var messages []SMS

	for _, p := range parts {
		if p.concat == nil {
			messages = append(messages, SMS{
				Indices: []int{p.index},
				From:    p.from,
				Time:    p.time,
				Body:    p.body,
			})
			continue
		}
		k := groupKey{p.from, p.concat.ref}
		if _, exists := groups[k]; !exists {
			groupOrder = append(groupOrder, k)
		}
		groups[k] = append(groups[k], p)
	}

	for _, k := range groupOrder {
		group := groups[k]
		if len(group) < int(group[0].concat.total) {
			// Not all parts received yet; leave them on the modem.
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return group[i].concat.part < group[j].concat.part
		})
		var body strings.Builder
		var indices []int
		for _, p := range group {
			body.WriteString(p.body)
			indices = append(indices, p.index)
		}
		messages = append(messages, SMS{
			Indices: indices,
			From:    group[0].from,
			Time:    group[0].time,
			Body:    body.String(),
		})
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].Time.Before(messages[j].Time)
	})
	return messages
}
