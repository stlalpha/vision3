package jam

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Base represents an open JAM message base backed by four files:
// .jhr (headers), .jdt (text), .jdx (index), .jlr (lastread).
type Base struct {
	BasePath    string
	fixedHeader *FixedHeaderInfo
	jhrFile     *os.File
	jdtFile     *os.File
	jdxFile     *os.File
	jlrFile     *os.File
	mu          sync.RWMutex
	isOpen      bool
}

// Open opens an existing JAM message base or creates a new one if it does
// not exist. basePath is the path without file extension (e.g., "data/msgbases/general").
func Open(basePath string) (*Base, error) {
	if err := os.MkdirAll(filepath.Dir(basePath), 0755); err != nil {
		return nil, fmt.Errorf("jam: failed to create directory: %w", err)
	}

	b := &Base{BasePath: basePath}

	jhrPath := basePath + ".jhr"
	jdtPath := basePath + ".jdt"
	jdxPath := basePath + ".jdx"
	jlrPath := basePath + ".jlr"

	stat, err := os.Stat(jhrPath)
	if os.IsNotExist(err) {
		return b, b.create()
	}

	// Corrupted or incomplete base: recreate
	if stat.Size() < HeaderSize {
		removeBaseFiles(jhrPath, jdtPath, jdxPath, jlrPath)
		return b, b.create()
	}
	for _, p := range []string{jdtPath, jdxPath, jlrPath} {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			removeBaseFiles(jhrPath, jdtPath, jdxPath, jlrPath)
			return b, b.create()
		}
	}

	// Open existing files
	b.jhrFile, err = os.OpenFile(jhrPath, os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("jam: failed to open .jhr: %w", err)
	}
	b.jdtFile, err = os.OpenFile(jdtPath, os.O_RDWR, 0644)
	if err != nil {
		b.jhrFile.Close()
		return nil, fmt.Errorf("jam: failed to open .jdt: %w", err)
	}
	b.jdxFile, err = os.OpenFile(jdxPath, os.O_RDWR, 0644)
	if err != nil {
		b.jhrFile.Close()
		b.jdtFile.Close()
		return nil, fmt.Errorf("jam: failed to open .jdx: %w", err)
	}
	b.jlrFile, err = os.OpenFile(jlrPath, os.O_RDWR, 0644)
	if err != nil {
		b.jhrFile.Close()
		b.jdtFile.Close()
		b.jdxFile.Close()
		return nil, fmt.Errorf("jam: failed to open .jlr: %w", err)
	}

	b.isOpen = true
	if err := b.readFixedHeader(); err != nil {
		b.Close()
		if strings.Contains(err.Error(), "EOF") || strings.Contains(err.Error(), "invalid") {
			removeBaseFiles(jhrPath, jdtPath, jdxPath, jlrPath)
			return b, b.create()
		}
		return nil, fmt.Errorf("jam: failed to read header: %w", err)
	}

	return b, nil
}

// create initializes a brand new JAM message base with four empty files
// and a valid fixed header.
func (b *Base) create() error {
	var err error
	jhrPath := b.BasePath + ".jhr"
	jdtPath := b.BasePath + ".jdt"
	jdxPath := b.BasePath + ".jdx"
	jlrPath := b.BasePath + ".jlr"

	b.jhrFile, err = os.Create(jhrPath)
	if err != nil {
		return fmt.Errorf("jam: failed to create .jhr: %w", err)
	}
	b.jdtFile, err = os.Create(jdtPath)
	if err != nil {
		b.jhrFile.Close()
		return fmt.Errorf("jam: failed to create .jdt: %w", err)
	}
	b.jdxFile, err = os.Create(jdxPath)
	if err != nil {
		b.jhrFile.Close()
		b.jdtFile.Close()
		return fmt.Errorf("jam: failed to create .jdx: %w", err)
	}
	b.jlrFile, err = os.Create(jlrPath)
	if err != nil {
		b.jhrFile.Close()
		b.jdtFile.Close()
		b.jdxFile.Close()
		return fmt.Errorf("jam: failed to create .jlr: %w", err)
	}

	b.fixedHeader = &FixedHeaderInfo{
		DateCreated: uint32(time.Now().Unix()),
		BaseMsgNum:  1,
	}
	copy(b.fixedHeader.Signature[:], Signature)

	b.isOpen = true
	if err := b.writeFixedHeader(); err != nil {
		b.Close()
		return fmt.Errorf("jam: failed to write initial header: %w", err)
	}
	return nil
}

// Close closes all file handles for the message base.
func (b *Base) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	var errs []error
	for _, f := range []*os.File{b.jhrFile, b.jdtFile, b.jdxFile, b.jlrFile} {
		if f != nil {
			if err := f.Close(); err != nil {
				errs = append(errs, err)
			}
		}
	}
	b.jhrFile = nil
	b.jdtFile = nil
	b.jdxFile = nil
	b.jlrFile = nil
	b.isOpen = false

	if len(errs) > 0 {
		return fmt.Errorf("jam: errors closing base: %v", errs)
	}
	return nil
}

// IsOpen reports whether the message base is currently open.
func (b *Base) IsOpen() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.isOpen
}

// RefreshFixedHeader reloads the fixed header from disk.
// Useful when external tools modify the base.
func (b *Base) RefreshFixedHeader() (*FixedHeaderInfo, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.isOpen {
		return nil, ErrBaseNotOpen
	}
	if err := b.readFixedHeader(); err != nil {
		return nil, err
	}
	return b.fixedHeader, nil
}

// GetModCounter returns the current ModCounter value from disk.
func (b *Base) GetModCounter() (uint32, error) {
	fh, err := b.RefreshFixedHeader()
	if err != nil {
		return 0, err
	}
	if fh == nil {
		return 0, nil
	}
	return fh.ModCounter, nil
}

// readFixedHeader reads the 1024-byte fixed header from the .jhr file.
func (b *Base) readFixedHeader() error {
	b.jhrFile.Seek(0, 0)
	b.fixedHeader = &FixedHeaderInfo{}
	if err := binary.Read(b.jhrFile, binary.LittleEndian, b.fixedHeader); err != nil {
		return fmt.Errorf("failed to read fixed header: %w", err)
	}
	if string(b.fixedHeader.Signature[:]) != Signature {
		return ErrInvalidSignature
	}
	return nil
}

// writeFixedHeader writes the fixed header to the .jhr file.
func (b *Base) writeFixedHeader() error {
	b.jhrFile.Seek(0, 0)
	return binary.Write(b.jhrFile, binary.LittleEndian, b.fixedHeader)
}

// GetMessageCount returns the total number of messages (including deleted)
// by computing the number of index records in the .jdx file.
func (b *Base) GetMessageCount() (int, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getMessageCountLocked()
}

func (b *Base) getMessageCountLocked() (int, error) {
	if !b.isOpen {
		return 0, ErrBaseNotOpen
	}
	info, err := b.jdxFile.Stat()
	if err != nil {
		return 0, fmt.Errorf("jam: failed to stat index: %w", err)
	}
	return int(info.Size() / IndexRecordSize), nil
}

// GetActiveMessageCount returns the number of non-deleted messages.
func (b *Base) GetActiveMessageCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.fixedHeader == nil {
		return 0
	}
	return int(b.fixedHeader.ActiveMsgs)
}

// ReadIndexRecord reads the index record for a 1-based message number.
func (b *Base) ReadIndexRecord(msgNum int) (*IndexRecord, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.readIndexRecordLocked(msgNum)
}

func (b *Base) readIndexRecordLocked(msgNum int) (*IndexRecord, error) {
	if !b.isOpen {
		return nil, ErrBaseNotOpen
	}
	count, err := b.getMessageCountLocked()
	if err != nil {
		return nil, err
	}
	if msgNum < 1 || msgNum > count {
		return nil, ErrInvalidMessage
	}

	offset := int64((msgNum - 1) * IndexRecordSize)
	b.jdxFile.Seek(offset, 0)

	var toCRC, hdrOffset uint32
	binary.Read(b.jdxFile, binary.LittleEndian, &toCRC)
	binary.Read(b.jdxFile, binary.LittleEndian, &hdrOffset)

	if toCRC == 0xFFFFFFFF && hdrOffset == 0xFFFFFFFF {
		return nil, ErrNotFound
	}
	return &IndexRecord{ToCRC: toCRC, HdrOffset: hdrOffset}, nil
}

// WriteIndexRecord writes an index record for a 1-based message number.
func (b *Base) writeIndexRecord(msgNum int, rec *IndexRecord) error {
	if !b.isOpen {
		return ErrBaseNotOpen
	}
	offset := int64((msgNum - 1) * IndexRecordSize)
	b.jdxFile.Seek(offset, 0)
	binary.Write(b.jdxFile, binary.LittleEndian, rec.ToCRC)
	binary.Write(b.jdxFile, binary.LittleEndian, rec.HdrOffset)
	return nil
}

// GetNextMsgSerial atomically increments and returns the next MSGID serial.
// The serial counter is stored in Reserved[0:4] of the fixed header.
func (b *Base) GetNextMsgSerial() (uint32, error) {
	if !b.isOpen {
		return 0, ErrBaseNotOpen
	}
	serial := binary.LittleEndian.Uint32(b.fixedHeader.Reserved[0:4])
	if serial == 0 {
		serial = uint32(time.Now().Unix())
	}
	serial++
	binary.LittleEndian.PutUint32(b.fixedHeader.Reserved[0:4], serial)
	if err := b.writeFixedHeader(); err != nil {
		return 0, err
	}
	return serial, nil
}

func removeBaseFiles(paths ...string) {
	for _, p := range paths {
		os.Remove(p)
	}
}
