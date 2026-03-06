package qwk

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
)

// ReadREP extracts messages from a QWK REP packet (ZIP archive).
// The REP packet contains a BBSID.MSG file with the same block format
// as MESSAGES.DAT, where the user's replies are stored.
func ReadREP(r io.ReaderAt, size int64, bbsID string) ([]REPMessage, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return nil, fmt.Errorf("failed to open REP archive: %w", err)
	}

	// Look for BBSID.MSG file (case-insensitive)
	msgFileName := strings.ToUpper(bbsID) + ".MSG"
	var msgFile *zip.File
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, msgFileName) {
			msgFile = f
			break
		}
	}
	if msgFile == nil {
		return nil, fmt.Errorf("REP packet missing %s", msgFileName)
	}

	rc, err := msgFile.Open()
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", msgFileName, err)
	}
	defer rc.Close()

	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", msgFileName, err)
	}

	return parseREPMessages(data)
}

// parseREPMessages extracts messages from the raw block data.
func parseREPMessages(data []byte) ([]REPMessage, error) {
	if len(data) < BlockSize {
		return nil, fmt.Errorf("REP data too short (%d bytes)", len(data))
	}

	// Skip the first block (header/spacer)
	pos := BlockSize
	var messages []REPMessage

	for pos+BlockSize <= len(data) {
		header := data[pos : pos+BlockSize]

		// Parse number of blocks from positions 116-121
		blkStr := strings.TrimSpace(string(header[116:122]))
		numBlocks, err := strconv.Atoi(blkStr)
		if err != nil || numBlocks < 1 {
			log.Printf("WARN: QWK REP: invalid block count %q at offset %d", blkStr, pos)
			break
		}

		totalBytes := numBlocks * BlockSize
		if pos+totalBytes > len(data) {
			log.Printf("WARN: QWK REP: message extends past end of data at offset %d", pos)
			break
		}

		// Parse conference from positions 123-124 (little-endian uint16)
		confNum := int(header[123]) | int(header[124])<<8

		// Parse fields
		to := strings.TrimSpace(string(header[21:46]))
		subject := strings.TrimSpace(string(header[71:96]))

		// Extract body (starts after header block)
		bodyBytes := data[pos+BlockSize : pos+totalBytes]
		body := decodeQWKBody(bodyBytes)

		messages = append(messages, REPMessage{
			Conference: confNum,
			To:         to,
			Subject:    subject,
			Body:       body,
		})

		pos += totalBytes
	}

	log.Printf("INFO: QWK REP: parsed %d messages", len(messages))
	return messages, nil
}

// decodeQWKBody converts QWK body bytes (0xE3 line endings) to normal text.
func decodeQWKBody(data []byte) string {
	// Trim trailing spaces
	data = bytes.TrimRight(data, " ")

	// Replace QWK line ending (0xE3) with newline
	var buf strings.Builder
	buf.Grow(len(data))
	for _, b := range data {
		if b == 0xE3 {
			buf.WriteByte('\n')
		} else {
			buf.WriteByte(b)
		}
	}
	return strings.TrimSpace(buf.String())
}
