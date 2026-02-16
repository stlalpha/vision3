package jam

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// PackResult contains statistics from a Pack operation.
type PackResult struct {
	MessagesBefore int
	MessagesAfter  int
	DeletedRemoved int
	BytesBefore    int64
	BytesAfter     int64
}

// GetFixedHeader returns the fixed header info for the base.
func (b *Base) GetFixedHeader() *FixedHeaderInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.fixedHeader == nil {
		return nil
	}
	fh := *b.fixedHeader
	return &fh
}

// GetAllLastReadRecords reads all lastread records from the .jlr file.
func (b *Base) GetAllLastReadRecords() ([]LastReadRecord, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.isOpen {
		return nil, ErrBaseNotOpen
	}

	info, err := b.jlrFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("jam: failed to stat .jlr: %w", err)
	}
	if info.Size()%LastReadSize != 0 {
		return nil, fmt.Errorf("jam: invalid .jlr size %d (not aligned to record size %d)", info.Size(), LastReadSize)
	}
	count := info.Size() / LastReadSize
	if count == 0 {
		return nil, nil
	}

	if _, err := b.jlrFile.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("jam: seek failed in .jlr: %w", err)
	}
	records := make([]LastReadRecord, 0, count)
	for i := int64(0); i < count; i++ {
		var lr LastReadRecord
		if err := readBinaryLE(b.jlrFile, &lr.UserCRC, "lastread user crc"); err != nil {
			return nil, err
		}
		if err := readBinaryLE(b.jlrFile, &lr.UserID, "lastread user id"); err != nil {
			return nil, err
		}
		if err := readBinaryLE(b.jlrFile, &lr.LastReadMsg, "lastread message pointer"); err != nil {
			return nil, err
		}
		if err := readBinaryLE(b.jlrFile, &lr.HighReadMsg, "lastread high read pointer"); err != nil {
			return nil, err
		}
		records = append(records, lr)
	}
	return records, nil
}

// ResetLastRead zeroes a user's lastread and highread pointers.
func (b *Base) ResetLastRead(username string) error {
	return b.SetLastRead(username, 0, 0)
}

// Pack defragments the message base by rewriting all non-deleted messages
// to new files, then atomically replacing the originals. The .jlr file
// is preserved as-is.
func (b *Base) Pack() (PackResult, error) {
	return b.packWithReplyIDCleanup(false)
}

// PackWithReplyIDCleanup performs a pack operation while cleaning malformed ReplyIDs.
func (b *Base) PackWithReplyIDCleanup() (PackResult, error) {
	return b.packWithReplyIDCleanup(true)
}

// packWithReplyIDCleanup is the internal pack implementation that optionally cleans ReplyIDs.
func (b *Base) packWithReplyIDCleanup(cleanReplyIDs bool) (PackResult, error) {
	var result PackResult

	release, err := b.acquireFileLock()
	if err != nil {
		return result, err
	}
	defer release()

	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isOpen {
		return result, ErrBaseNotOpen
	}

	totalCount, err := b.getMessageCountLocked()
	if err != nil {
		return result, err
	}
	result.MessagesBefore = totalCount

	// Record original file sizes
	for _, f := range []*os.File{b.jhrFile, b.jdtFile, b.jdxFile} {
		info, err := f.Stat()
		if err != nil {
			return result, fmt.Errorf("jam: failed to stat file: %w", err)
		}
		result.BytesBefore += info.Size()
	}

	// Create temp files in the same directory
	tmpJhr := b.BasePath + ".jhr.tmp"
	tmpJdt := b.BasePath + ".jdt.tmp"
	tmpJdx := b.BasePath + ".jdx.tmp"

	jhrOut, err := os.Create(tmpJhr)
	if err != nil {
		return result, fmt.Errorf("jam: failed to create temp .jhr: %w", err)
	}
	jdtOut, err := os.Create(tmpJdt)
	if err != nil {
		jhrOut.Close()
		os.Remove(tmpJhr)
		return result, fmt.Errorf("jam: failed to create temp .jdt: %w", err)
	}
	jdxOut, err := os.Create(tmpJdx)
	if err != nil {
		jhrOut.Close()
		jdtOut.Close()
		os.Remove(tmpJhr)
		os.Remove(tmpJdt)
		return result, fmt.Errorf("jam: failed to create temp .jdx: %w", err)
	}

	cleanup := func() {
		jhrOut.Close()
		jdtOut.Close()
		jdxOut.Close()
		os.Remove(tmpJhr)
		os.Remove(tmpJdt)
		os.Remove(tmpJdx)
	}

	// Write placeholder fixed header (will update ActiveMsgs at end)
	newFH := *b.fixedHeader
	newFH.ActiveMsgs = 0
	newFH.ModCounter++
	if err := binary.Write(jhrOut, binary.LittleEndian, &newFH); err != nil {
		cleanup()
		return result, fmt.Errorf("jam: failed to write temp fixed header: %w", err)
	}

	activeCount := 0
	newMsgNum := uint32(0)

	for n := 1; n <= totalCount; n++ {
		idx, err := b.readIndexRecordLocked(n)
		if err != nil {
			continue // skip invalid index entries
		}

		// Read header at the offset
		if _, err := b.jhrFile.Seek(int64(idx.HdrOffset), 0); err != nil {
			cleanup()
			return result, fmt.Errorf("jam: failed to seek header for msg %d: %w", n, err)
		}
		hdr, err := b.readHeaderFromReader(b.jhrFile)
		if err != nil {
			cleanup()
			return result, fmt.Errorf("jam: failed to read header for msg %d: %w", n, err)
		}

		if hdr.Attribute&MsgDeleted != 0 {
			continue
		}

		// Read text from original .jdt
		var textBuf []byte
		if hdr.TxtLen > 0 {
			textBuf = make([]byte, hdr.TxtLen)
			if _, err := b.jdtFile.Seek(int64(hdr.Offset), 0); err != nil {
				cleanup()
				return result, fmt.Errorf("jam: failed to seek text for msg %d: %w", n, err)
			}
			if _, err := io.ReadFull(b.jdtFile, textBuf); err != nil {
				cleanup()
				return result, fmt.Errorf("jam: failed to read text for msg %d: %w", n, err)
			}
		}

		// Write text to new .jdt
		newTextOffset := uint32(0)
		if len(textBuf) > 0 {
			pos, _ := jdtOut.Seek(0, io.SeekEnd)
			newTextOffset = uint32(pos)
			if _, err := jdtOut.Write(textBuf); err != nil {
				cleanup()
				return result, fmt.Errorf("jam: failed to write text: %w", err)
			}
		}

		// Update header for new positions
		newMsgNum++
		hdr.Offset = newTextOffset
		hdr.MessageNumber = newMsgNum + b.fixedHeader.BaseMsgNum - 1
		hdr.ReplyTo = 0
		hdr.Reply1st = 0
		hdr.ReplyNext = 0

		// Clean ReplyID if requested
		if cleanReplyIDs {
			for i := range hdr.Subfields {
				if hdr.Subfields[i].LoID == SfldReplyID {
					replyID := string(hdr.Subfields[i].Buffer)
					if parts := strings.Fields(replyID); len(parts) > 1 {
						// Clean the ReplyID by taking only the first token
						cleanedReplyID := parts[0]
						hdr.Subfields[i].Buffer = []byte(cleanedReplyID)
						hdr.Subfields[i].DatLen = uint32(len(cleanedReplyID))

						// Recalculate total subfield length
						hdr.SubfieldLen = 0
						for _, sf := range hdr.Subfields {
							hdr.SubfieldLen += SubfieldHdrSize + sf.DatLen
						}
					}
					break
				}
			}
		}

		// Write header to new .jhr
		hdrPos, err := jhrOut.Seek(0, io.SeekEnd)
		if err != nil {
			cleanup()
			return result, fmt.Errorf("jam: failed to seek temp .jhr: %w", err)
		}
		if err := b.writeHeaderToWriter(jhrOut, hdr); err != nil {
			cleanup()
			return result, fmt.Errorf("jam: failed to write header: %w", err)
		}

		// Write index record to new .jdx
		if err := writeBinaryLE(jdxOut, idx.ToCRC, "packed index ToCRC"); err != nil {
			cleanup()
			return result, err
		}
		if err := writeBinaryLE(jdxOut, uint32(hdrPos), "packed index header offset"); err != nil {
			cleanup()
			return result, err
		}

		activeCount++
	}

	// Update final ActiveMsgs in the fixed header
	newFH.ActiveMsgs = uint32(activeCount)
	if _, err := jhrOut.Seek(0, 0); err != nil {
		cleanup()
		return result, fmt.Errorf("jam: failed to seek temp fixed header: %w", err)
	}
	if err := binary.Write(jhrOut, binary.LittleEndian, &newFH); err != nil {
		cleanup()
		return result, fmt.Errorf("jam: failed to update fixed header: %w", err)
	}

	// Sync and close temp files
	for _, f := range []*os.File{jhrOut, jdtOut, jdxOut} {
		if err := f.Sync(); err != nil {
			cleanup()
			return result, fmt.Errorf("jam: failed to sync temp file: %w", err)
		}
		if err := f.Close(); err != nil {
			cleanup()
			return result, fmt.Errorf("jam: failed to close temp file: %w", err)
		}
	}

	// Close original file handles
	b.jhrFile.Close()
	b.jdtFile.Close()
	b.jdxFile.Close()

	// Atomic rename
	renameFailed := false
	for _, pair := range [][2]string{
		{tmpJhr, b.BasePath + ".jhr"},
		{tmpJdt, b.BasePath + ".jdt"},
		{tmpJdx, b.BasePath + ".jdx"},
	} {
		if err := os.Rename(pair[0], pair[1]); err != nil {
			renameFailed = true
			// Try to clean up remaining temp files
			os.Remove(tmpJhr)
			os.Remove(tmpJdt)
			os.Remove(tmpJdx)
			// Attempt to reopen original files
			b.jhrFile, _ = os.OpenFile(b.BasePath+".jhr", os.O_RDWR, 0644)
			b.jdtFile, _ = os.OpenFile(b.BasePath+".jdt", os.O_RDWR, 0644)
			b.jdxFile, _ = os.OpenFile(b.BasePath+".jdx", os.O_RDWR, 0644)
			b.readFixedHeader()
			return result, fmt.Errorf("jam: rename failed: %w — base may need manual recovery", err)
		}
	}

	if renameFailed {
		return result, fmt.Errorf("jam: pack failed during rename")
	}

	// Reopen files
	b.jhrFile, err = os.OpenFile(b.BasePath+".jhr", os.O_RDWR, 0644)
	if err != nil {
		b.isOpen = false
		return result, fmt.Errorf("jam: failed to reopen .jhr after pack: %w", err)
	}
	b.jdtFile, err = os.OpenFile(b.BasePath+".jdt", os.O_RDWR, 0644)
	if err != nil {
		b.isOpen = false
		return result, fmt.Errorf("jam: failed to reopen .jdt after pack: %w", err)
	}
	b.jdxFile, err = os.OpenFile(b.BasePath+".jdx", os.O_RDWR, 0644)
	if err != nil {
		b.isOpen = false
		return result, fmt.Errorf("jam: failed to reopen .jdx after pack: %w", err)
	}

	if err := b.readFixedHeader(); err != nil {
		return result, fmt.Errorf("jam: failed to read header after pack: %w", err)
	}

	// Calculate new sizes
	for _, f := range []*os.File{b.jhrFile, b.jdtFile, b.jdxFile} {
		info, err := f.Stat()
		if err != nil {
			return result, fmt.Errorf("jam: failed to stat file after pack: %w", err)
		}
		result.BytesAfter += info.Size()
	}

	result.MessagesAfter = activeCount
	result.DeletedRemoved = totalCount - activeCount
	return result, nil
}

// readHeaderFromReader reads a MessageHeader from an io.Reader at the current position.
func (b *Base) readHeaderFromReader(r io.Reader) (*MessageHeader, error) {
	hdr := &MessageHeader{}
	if err := readBinaryLE(r, &hdr.Signature, "header signature"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.Revision, "header revision"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.ReservedWord, "header reserved word"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.SubfieldLen, "header subfield length"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.TimesRead, "header times read"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.MSGIDcrc, "header MSGID crc"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.REPLYcrc, "header REPLY crc"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.ReplyTo, "header reply to"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.Reply1st, "header reply first"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.ReplyNext, "header reply next"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.DateWritten, "header date written"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.DateReceived, "header date received"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.DateProcessed, "header date processed"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.MessageNumber, "header message number"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.Attribute, "header attribute"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.Attribute2, "header attribute2"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.Offset, "header text offset"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.TxtLen, "header text length"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.PasswordCRC, "header password crc"); err != nil {
		return nil, err
	}
	if err := readBinaryLE(r, &hdr.Cost, "header cost"); err != nil {
		return nil, err
	}

	if string(hdr.Signature[:]) != Signature {
		return nil, ErrInvalidSignature
	}

	bytesRead := uint32(0)
	for bytesRead < hdr.SubfieldLen {
		sf := Subfield{}
		if err := readBinaryLE(r, &sf.LoID, "subfield loID"); err != nil {
			return nil, err
		}
		if err := readBinaryLE(r, &sf.HiID, "subfield hiID"); err != nil {
			return nil, err
		}
		if err := readBinaryLE(r, &sf.DatLen, "subfield data length"); err != nil {
			return nil, err
		}
		sf.Buffer = make([]byte, sf.DatLen)
		if _, err := io.ReadFull(r, sf.Buffer); err != nil {
			return nil, fmt.Errorf("jam: read subfield buffer: %w", err)
		}
		hdr.Subfields = append(hdr.Subfields, sf)
		bytesRead += SubfieldHdrSize + sf.DatLen
	}

	return hdr, nil
}

// writeHeaderToWriter writes a MessageHeader to an io.Writer, matching
// the exact binary layout used by writeMessageHeader.
func (b *Base) writeHeaderToWriter(w io.Writer, hdr *MessageHeader) error {
	if err := writeBinaryLE(w, hdr.Signature, "header signature"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.Revision, "header revision"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.ReservedWord, "header reserved word"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.SubfieldLen, "header subfield length"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.TimesRead, "header times read"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.MSGIDcrc, "header MSGID crc"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.REPLYcrc, "header REPLY crc"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.ReplyTo, "header reply to"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.Reply1st, "header reply first"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.ReplyNext, "header reply next"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.DateWritten, "header date written"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.DateReceived, "header date received"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.DateProcessed, "header date processed"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.MessageNumber, "header message number"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.Attribute, "header attribute"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.Attribute2, "header attribute2"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.Offset, "header text offset"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.TxtLen, "header text length"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.PasswordCRC, "header password crc"); err != nil {
		return err
	}
	if err := writeBinaryLE(w, hdr.Cost, "header cost"); err != nil {
		return err
	}

	for _, sf := range hdr.Subfields {
		if err := writeBinaryLE(w, sf.LoID, "subfield loID"); err != nil {
			return err
		}
		if err := writeBinaryLE(w, sf.HiID, "subfield hiID"); err != nil {
			return err
		}
		if err := writeBinaryLE(w, sf.DatLen, "subfield data length"); err != nil {
			return err
		}
		if err := writeAll(w, sf.Buffer, "subfield buffer"); err != nil {
			return err
		}
	}
	return nil
}
