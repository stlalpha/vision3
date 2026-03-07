package qwk

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"time"
)

// PacketWriter builds a QWK mail packet (ZIP archive) containing
// CONTROL.DAT, MESSAGES.DAT, DOOR.ID, and per-conference .NDX files.
type PacketWriter struct {
	bbsID       string // Short BBS ID for packet filename (max 8 chars)
	bbsName     string
	sysOpName   string
	bbsPhone    string
	conferences []ConferenceInfo

	messages   []PacketMessage
	personalTo string // username to match for PERSONAL.NDX
}

// NewPacketWriter creates a new QWK packet writer.
// bbsID should be a short identifier (max 8 chars, e.g. "VISION3").
func NewPacketWriter(bbsID, bbsName, sysOpName string) *PacketWriter {
	if len(bbsID) > 8 {
		bbsID = bbsID[:8]
	}
	return &PacketWriter{
		bbsID:     strings.ToUpper(bbsID),
		bbsName:   bbsName,
		sysOpName: sysOpName,
		bbsPhone:  "000-000-0000",
	}
}

// SetPersonalTo sets the username for PERSONAL.NDX matching.
func (pw *PacketWriter) SetPersonalTo(username string) {
	pw.personalTo = strings.ToLower(username)
}

// AddConference registers a conference for inclusion in CONTROL.DAT.
func (pw *PacketWriter) AddConference(number int, name string) {
	pw.conferences = append(pw.conferences, ConferenceInfo{Number: number, Name: name})
}

// AddMessage adds a message to the packet.
func (pw *PacketWriter) AddMessage(msg PacketMessage) {
	pw.messages = append(pw.messages, msg)
}

// MessageCount returns the number of messages added.
func (pw *PacketWriter) MessageCount() int {
	return len(pw.messages)
}

// WritePacket writes the complete QWK ZIP packet to w.
func (pw *PacketWriter) WritePacket(w io.Writer) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	if err := pw.writeControlDAT(zw); err != nil {
		return fmt.Errorf("CONTROL.DAT: %w", err)
	}
	if err := pw.writeDoorID(zw); err != nil {
		return fmt.Errorf("DOOR.ID: %w", err)
	}

	ndxData, personalNDX, err := pw.writeMessagesDAT(zw)
	if err != nil {
		return fmt.Errorf("MESSAGES.DAT: %w", err)
	}

	for confNum, data := range ndxData {
		name := fmt.Sprintf("%03d.NDX", confNum)
		if err := writeZipEntry(zw, name, data); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}

	if len(personalNDX) > 0 {
		if err := writeZipEntry(zw, "PERSONAL.NDX", personalNDX); err != nil {
			return fmt.Errorf("PERSONAL.NDX: %w", err)
		}
	}

	return nil
}

// writeControlDAT writes the BBS info and conference list.
func (pw *PacketWriter) writeControlDAT(zw *zip.Writer) error {
	var buf bytes.Buffer
	now := time.Now()

	lines := []string{
		pw.bbsName,                     // Line 1: BBS name
		"",                             // Line 2: city/state
		pw.bbsPhone,                    // Line 3: phone number
		pw.sysOpName,                   // Line 4: sysop name
		"00000," + pw.bbsID,            // Line 5: serial number, BBS ID
		now.Format("01-02-2006,15:04"), // Line 6: date, time
		pw.personalTo,                  // Line 7: username
		"",                             // Line 8: empty
		"0",                            // Line 9: unused
		"0",                            // Line 10: total messages (filled by reader)
	}

	// Line 11: number of conferences - 1 (0-based)
	confCount := len(pw.conferences)
	if confCount > 0 {
		confCount = confCount - 1
	}
	lines = append(lines, fmt.Sprintf("%d", confCount))

	// Conference entries: number then name, alternating lines
	for _, c := range pw.conferences {
		lines = append(lines, fmt.Sprintf("%d", c.Number))
		lines = append(lines, c.Name)
	}

	// Display file references
	lines = append(lines, "HELLO", "NEWS", "GOODBYE")

	for _, line := range lines {
		buf.WriteString(line + "\r\n")
	}

	return writeZipEntry(zw, "CONTROL.DAT", buf.Bytes())
}

// writeDoorID writes the door identification file.
func (pw *PacketWriter) writeDoorID(zw *zip.Writer) error {
	var buf bytes.Buffer
	buf.WriteString("DOOR = ViSiON/3\r\n")
	buf.WriteString("VERSION = 1.0\r\n")
	buf.WriteString("CONTROLNAME = " + pw.bbsID + "\r\n")
	buf.WriteString("CONTROLTYPE = ADD\r\n")
	buf.WriteString("PRODUCED BY = ViSiON/3 BBS\r\n")
	return writeZipEntry(zw, "DOOR.ID", buf.Bytes())
}

// writeMessagesDAT writes all messages and returns NDX data per conference
// and PERSONAL.NDX data.
func (pw *PacketWriter) writeMessagesDAT(zw *zip.Writer) (map[int][]byte, []byte, error) {
	var msgBuf bytes.Buffer

	// First block is a copyright/spacer block (128 bytes of spaces)
	spacer := make([]byte, BlockSize)
	for i := range spacer {
		spacer[i] = ' '
	}
	copy(spacer, "Produced by ViSiON/3 BBS")
	msgBuf.Write(spacer)

	currentBlock := 2 // Block 1 is the spacer; messages start at block 2
	ndxData := make(map[int][]byte)
	var personalNDX []byte

	for _, msg := range pw.messages {
		msgBytes := formatMessage(msg)
		numBlocks := (len(msgBytes) + BlockSize - 1) / BlockSize

		// Pad to block boundary
		padded := make([]byte, numBlocks*BlockSize)
		for i := range padded {
			padded[i] = ' '
		}
		copy(padded, msgBytes)

		// NDX record: 4-byte float (block offset) + 1-byte conference number
		ndxRecord := makeNDXRecord(currentBlock, msg.Conference)
		ndxData[msg.Conference] = append(ndxData[msg.Conference], ndxRecord...)

		if pw.personalTo != "" && strings.EqualFold(msg.To, pw.personalTo) {
			personalNDX = append(personalNDX, ndxRecord...)
		}

		msgBuf.Write(padded)
		currentBlock += numBlocks
	}

	if err := writeZipEntry(zw, "MESSAGES.DAT", msgBuf.Bytes()); err != nil {
		return nil, nil, err
	}

	log.Printf("INFO: QWK packet: %d messages, %d blocks", len(pw.messages), currentBlock-1)
	return ndxData, personalNDX, nil
}

// formatMessage encodes a single message in QWK format.
// The first 128 bytes are the header; the body follows.
func formatMessage(msg PacketMessage) []byte {
	status := byte(StatusPublic)
	if msg.Private {
		status = byte(StatusPrivate)
	}

	// QWK header is exactly 128 bytes with fixed-width fields
	header := make([]byte, BlockSize)
	for i := range header {
		header[i] = ' '
	}

	header[0] = status

	// Message number: positions 1-7 (7 chars, right-justified)
	copyPadded(header[1:8], fmt.Sprintf("%7d", msg.Number), 7)

	// Date: positions 8-15 (MM-DD-YY, 8 chars)
	copyPadded(header[8:16], msg.DateTime.Format("01-02-06"), 8)

	// Time: positions 16-20 (HH:MM, 5 chars)
	copyPadded(header[16:21], msg.DateTime.Format("15:04"), 5)

	// To: positions 21-45 (25 chars)
	copyPadded(header[21:46], msg.To, 25)

	// From: positions 46-70 (25 chars)
	copyPadded(header[46:71], msg.From, 25)

	// Subject: positions 71-95 (25 chars)
	copyPadded(header[71:96], msg.Subject, 25)

	// Password: positions 96-107 (12 chars) — blank

	// Reference number: positions 108-115 (8 chars)
	copyPadded(header[108:116], fmt.Sprintf("%8d", 0), 8)

	// Body with QWK line ending (0xE3)
	body := strings.ReplaceAll(msg.Body, "\r\n", "\xe3")
	body = strings.ReplaceAll(body, "\n", "\xe3")

	totalLen := BlockSize + len(body)
	numBlocks := (totalLen + BlockSize - 1) / BlockSize

	// Number of blocks (including header): positions 116-121 (6 chars)
	copyPadded(header[116:122], fmt.Sprintf("%6d", numBlocks), 6)

	// Active flag: position 122 (0xE1 = active)
	header[122] = 0xE1

	// Conference number: positions 123-124 (2-byte little-endian uint16)
	header[123] = byte(msg.Conference & 0xFF)
	header[124] = byte((msg.Conference >> 8) & 0xFF)

	// Logical message number: positions 125-127 (unused)

	result := make([]byte, 0, totalLen)
	result = append(result, header...)
	result = append(result, []byte(body)...)

	return result
}

// makeNDXRecord creates a 5-byte NDX index record.
// offset is the 1-based block number; conf is the conference number.
func makeNDXRecord(offset int, conf int) []byte {
	rec := make([]byte, 5)
	// QWK NDX uses IEEE 754 single-precision float, little-endian
	bits := math.Float32bits(float32(offset))
	binary.LittleEndian.PutUint32(rec[0:4], bits)
	rec[4] = byte(conf & 0xFF)
	return rec
}

func copyPadded(dst []byte, src string, maxLen int) {
	if len(src) > maxLen {
		src = src[:maxLen]
	}
	copy(dst, src)
}

func writeZipEntry(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
