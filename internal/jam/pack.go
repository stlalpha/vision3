package jam

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
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
	return b.fixedHeader
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
	count := info.Size() / LastReadSize
	if count == 0 {
		return nil, nil
	}

	b.jlrFile.Seek(0, 0)
	records := make([]LastReadRecord, 0, count)
	for i := int64(0); i < count; i++ {
		var lr LastReadRecord
		binary.Read(b.jlrFile, binary.LittleEndian, &lr.UserCRC)
		binary.Read(b.jlrFile, binary.LittleEndian, &lr.UserID)
		binary.Read(b.jlrFile, binary.LittleEndian, &lr.LastReadMsg)
		binary.Read(b.jlrFile, binary.LittleEndian, &lr.HighReadMsg)
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
	b.mu.Lock()
	defer b.mu.Unlock()

	var result PackResult

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
		b.jhrFile.Seek(int64(idx.HdrOffset), 0)
		hdr, err := b.readHeaderFromReader(b.jhrFile)
		if err != nil {
			continue
		}

		if hdr.Attribute&MsgDeleted != 0 {
			continue
		}

		// Read text from original .jdt
		var textBuf []byte
		if hdr.TxtLen > 0 {
			textBuf = make([]byte, hdr.TxtLen)
			b.jdtFile.Seek(int64(hdr.Offset), 0)
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

		// Write header to new .jhr
		hdrPos, _ := jhrOut.Seek(0, io.SeekEnd)
		if err := b.writeHeaderToWriter(jhrOut, hdr); err != nil {
			cleanup()
			return result, fmt.Errorf("jam: failed to write header: %w", err)
		}

		// Write index record to new .jdx
		binary.Write(jdxOut, binary.LittleEndian, idx.ToCRC)
		binary.Write(jdxOut, binary.LittleEndian, uint32(hdrPos))

		activeCount++
	}

	// Update final ActiveMsgs in the fixed header
	newFH.ActiveMsgs = uint32(activeCount)
	jhrOut.Seek(0, 0)
	if err := binary.Write(jhrOut, binary.LittleEndian, &newFH); err != nil {
		cleanup()
		return result, fmt.Errorf("jam: failed to update fixed header: %w", err)
	}

	// Sync and close temp files
	for _, f := range []*os.File{jhrOut, jdtOut, jdxOut} {
		f.Sync()
		f.Close()
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
		info, _ := f.Stat()
		result.BytesAfter += info.Size()
	}

	result.MessagesAfter = activeCount
	result.DeletedRemoved = totalCount - activeCount
	return result, nil
}

// readHeaderFromReader reads a MessageHeader from an io.Reader at the current position.
func (b *Base) readHeaderFromReader(r io.Reader) (*MessageHeader, error) {
	hdr := &MessageHeader{}
	binary.Read(r, binary.LittleEndian, &hdr.Signature)
	binary.Read(r, binary.LittleEndian, &hdr.Revision)
	binary.Read(r, binary.LittleEndian, &hdr.ReservedWord)
	binary.Read(r, binary.LittleEndian, &hdr.SubfieldLen)
	binary.Read(r, binary.LittleEndian, &hdr.TimesRead)
	binary.Read(r, binary.LittleEndian, &hdr.MSGIDcrc)
	binary.Read(r, binary.LittleEndian, &hdr.REPLYcrc)
	binary.Read(r, binary.LittleEndian, &hdr.ReplyTo)
	binary.Read(r, binary.LittleEndian, &hdr.Reply1st)
	binary.Read(r, binary.LittleEndian, &hdr.ReplyNext)
	binary.Read(r, binary.LittleEndian, &hdr.DateWritten)
	binary.Read(r, binary.LittleEndian, &hdr.DateReceived)
	binary.Read(r, binary.LittleEndian, &hdr.DateProcessed)
	binary.Read(r, binary.LittleEndian, &hdr.MessageNumber)
	binary.Read(r, binary.LittleEndian, &hdr.Attribute)
	binary.Read(r, binary.LittleEndian, &hdr.Attribute2)
	binary.Read(r, binary.LittleEndian, &hdr.Offset)
	binary.Read(r, binary.LittleEndian, &hdr.TxtLen)
	binary.Read(r, binary.LittleEndian, &hdr.PasswordCRC)
	binary.Read(r, binary.LittleEndian, &hdr.Cost)

	if string(hdr.Signature[:]) != Signature {
		return nil, ErrInvalidSignature
	}

	bytesRead := uint32(0)
	for bytesRead < hdr.SubfieldLen {
		sf := Subfield{}
		binary.Read(r, binary.LittleEndian, &sf.LoID)
		binary.Read(r, binary.LittleEndian, &sf.HiID)
		binary.Read(r, binary.LittleEndian, &sf.DatLen)
		sf.Buffer = make([]byte, sf.DatLen)
		io.ReadFull(r, sf.Buffer)
		hdr.Subfields = append(hdr.Subfields, sf)
		bytesRead += SubfieldHdrSize + sf.DatLen
	}

	return hdr, nil
}

// writeHeaderToWriter writes a MessageHeader to an io.Writer, matching
// the exact binary layout used by writeMessageHeader.
func (b *Base) writeHeaderToWriter(w io.Writer, hdr *MessageHeader) error {
	binary.Write(w, binary.LittleEndian, hdr.Signature)
	binary.Write(w, binary.LittleEndian, hdr.Revision)
	binary.Write(w, binary.LittleEndian, hdr.ReservedWord)
	binary.Write(w, binary.LittleEndian, hdr.SubfieldLen)
	binary.Write(w, binary.LittleEndian, hdr.TimesRead)
	binary.Write(w, binary.LittleEndian, hdr.MSGIDcrc)
	binary.Write(w, binary.LittleEndian, hdr.REPLYcrc)
	binary.Write(w, binary.LittleEndian, hdr.ReplyTo)
	binary.Write(w, binary.LittleEndian, hdr.Reply1st)
	binary.Write(w, binary.LittleEndian, hdr.ReplyNext)
	binary.Write(w, binary.LittleEndian, hdr.DateWritten)
	binary.Write(w, binary.LittleEndian, hdr.DateReceived)
	binary.Write(w, binary.LittleEndian, hdr.DateProcessed)
	binary.Write(w, binary.LittleEndian, hdr.MessageNumber)
	binary.Write(w, binary.LittleEndian, hdr.Attribute)
	binary.Write(w, binary.LittleEndian, hdr.Attribute2)
	binary.Write(w, binary.LittleEndian, hdr.Offset)
	binary.Write(w, binary.LittleEndian, hdr.TxtLen)
	binary.Write(w, binary.LittleEndian, hdr.PasswordCRC)
	binary.Write(w, binary.LittleEndian, hdr.Cost)

	for _, sf := range hdr.Subfields {
		binary.Write(w, binary.LittleEndian, sf.LoID)
		binary.Write(w, binary.LittleEndian, sf.HiID)
		binary.Write(w, binary.LittleEndian, sf.DatLen)
		w.Write(sf.Buffer)
	}
	return nil
}
