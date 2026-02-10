package jam

import (
	"encoding/binary"
	"fmt"
	"io"
	"strings"
)

// GetLastRead returns the lastread record for the given username.
// Returns ErrNotFound if the user has no record in this base.
func (b *Base) GetLastRead(username string) (*LastReadRecord, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getLastReadLocked(username)
}

func (b *Base) getLastReadLocked(username string) (*LastReadRecord, error) {
	if !b.isOpen {
		return nil, ErrBaseNotOpen
	}

	userCRC := CRC32String(strings.ToLower(username))

	info, err := b.jlrFile.Stat()
	if err != nil {
		return nil, fmt.Errorf("jam: failed to stat .jlr: %w", err)
	}
	recordCount := info.Size() / LastReadSize

	b.jlrFile.Seek(0, 0)
	for i := int64(0); i < recordCount; i++ {
		lr := &LastReadRecord{}
		binary.Read(b.jlrFile, binary.LittleEndian, &lr.UserCRC)
		binary.Read(b.jlrFile, binary.LittleEndian, &lr.UserID)
		binary.Read(b.jlrFile, binary.LittleEndian, &lr.LastReadMsg)
		binary.Read(b.jlrFile, binary.LittleEndian, &lr.HighReadMsg)

		if lr.UserCRC == userCRC {
			return lr, nil
		}
	}
	return nil, ErrNotFound
}

// SetLastRead updates or creates a lastread record for the given username.
func (b *Base) SetLastRead(username string, lastRead, highRead uint32) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isOpen {
		return ErrBaseNotOpen
	}

	userCRC := CRC32String(strings.ToLower(username))

	info, err := b.jlrFile.Stat()
	if err != nil {
		return fmt.Errorf("jam: failed to stat .jlr: %w", err)
	}
	recordCount := info.Size() / LastReadSize

	// Search for existing record
	for i := int64(0); i < recordCount; i++ {
		pos := i * LastReadSize
		b.jlrFile.Seek(pos, 0)

		var readCRC uint32
		binary.Read(b.jlrFile, binary.LittleEndian, &readCRC)

		if readCRC == userCRC {
			b.jlrFile.Seek(pos, 0)
			binary.Write(b.jlrFile, binary.LittleEndian, userCRC)
			binary.Write(b.jlrFile, binary.LittleEndian, userCRC)
			binary.Write(b.jlrFile, binary.LittleEndian, lastRead)
			binary.Write(b.jlrFile, binary.LittleEndian, highRead)
			return nil
		}
	}

	// Append new record
	b.jlrFile.Seek(0, io.SeekEnd)
	binary.Write(b.jlrFile, binary.LittleEndian, userCRC)
	binary.Write(b.jlrFile, binary.LittleEndian, userCRC)
	binary.Write(b.jlrFile, binary.LittleEndian, lastRead)
	binary.Write(b.jlrFile, binary.LittleEndian, highRead)
	return nil
}

// GetNextUnreadMessage returns the next unread message number for the user.
// Returns ErrNotFound if there are no unread messages.
func (b *Base) GetNextUnreadMessage(username string) (int, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.isOpen {
		return 0, ErrBaseNotOpen
	}

	lr, err := b.getLastReadLocked(username)
	if err != nil {
		if err == ErrNotFound {
			count, cerr := b.getMessageCountLocked()
			if cerr != nil {
				return 0, cerr
			}
			if count > 0 {
				return 1, nil
			}
			return 0, ErrNotFound
		}
		return 0, err
	}

	nextMsg := int(lr.LastReadMsg) + 1
	count, err := b.getMessageCountLocked()
	if err != nil {
		return 0, err
	}
	if nextMsg <= count {
		return nextMsg, nil
	}
	return 0, ErrNotFound
}

// MarkMessageRead updates the lastread pointer after reading a message.
func (b *Base) MarkMessageRead(username string, msgNum int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.isOpen {
		return ErrBaseNotOpen
	}

	lr, err := b.getLastReadLocked(username)
	if err != nil {
		if err == ErrNotFound {
			// Temporarily release/re-acquire not needed since we hold write lock
			return b.setLastReadLocked(username, uint32(msgNum), uint32(msgNum))
		}
		return err
	}

	newLast := uint32(msgNum)
	newHigh := lr.HighReadMsg
	if newLast > newHigh {
		newHigh = newLast
	}
	return b.setLastReadLocked(username, newLast, newHigh)
}

// setLastReadLocked is the non-locking version of SetLastRead, for use
// when the caller already holds the write lock.
func (b *Base) setLastReadLocked(username string, lastRead, highRead uint32) error {
	userCRC := CRC32String(strings.ToLower(username))

	info, err := b.jlrFile.Stat()
	if err != nil {
		return fmt.Errorf("jam: failed to stat .jlr: %w", err)
	}
	recordCount := info.Size() / LastReadSize

	for i := int64(0); i < recordCount; i++ {
		pos := i * LastReadSize
		b.jlrFile.Seek(pos, 0)

		var readCRC uint32
		binary.Read(b.jlrFile, binary.LittleEndian, &readCRC)

		if readCRC == userCRC {
			b.jlrFile.Seek(pos, 0)
			binary.Write(b.jlrFile, binary.LittleEndian, userCRC)
			binary.Write(b.jlrFile, binary.LittleEndian, userCRC)
			binary.Write(b.jlrFile, binary.LittleEndian, lastRead)
			binary.Write(b.jlrFile, binary.LittleEndian, highRead)
			return nil
		}
	}

	b.jlrFile.Seek(0, io.SeekEnd)
	binary.Write(b.jlrFile, binary.LittleEndian, userCRC)
	binary.Write(b.jlrFile, binary.LittleEndian, userCRC)
	binary.Write(b.jlrFile, binary.LittleEndian, lastRead)
	binary.Write(b.jlrFile, binary.LittleEndian, highRead)
	return nil
}

// GetUnreadCount returns the number of unread messages for the user.
func (b *Base) GetUnreadCount(username string) (int, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if !b.isOpen {
		return 0, ErrBaseNotOpen
	}

	count, err := b.getMessageCountLocked()
	if err != nil {
		return 0, err
	}
	if count == 0 {
		return 0, nil
	}

	lr, err := b.getLastReadLocked(username)
	if err != nil {
		if err == ErrNotFound {
			return count, nil
		}
		return 0, err
	}

	unread := count - int(lr.LastReadMsg)
	if unread < 0 {
		unread = 0
	}
	return unread, nil
}
