package jam

import (
	"fmt"
	"strings"
)

// LinkResult contains statistics from a Link operation.
type LinkResult struct {
	MessagesScanned int
	LinksUpdated    int
}

// Link rebuilds reply threading chains (ReplyTo/Reply1st/ReplyNext) by
// matching MSGID and ReplyID subfields across all active messages. This
// should be called after Pack or after deleting messages to keep threading
// consistent.
func (b *Base) Link() (LinkResult, error) {
	var result LinkResult

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

	total, err := b.getMessageCountLocked()
	if err != nil {
		return result, err
	}
	if total == 0 {
		return result, nil
	}

	// Phase 1: Scan all headers and build MSGID → msgNum / ReplyID → []msgNum maps.
	type hdrInfo struct {
		hdr     *MessageHeader
		msgNum  int
		msgID   string
		replyID string
	}

	var headers []hdrInfo
	msgIDToNum := make(map[string]int)      // MSGID string → 1-based message number
	replyIDToNums := make(map[string][]int) // ReplyID string → list of replying message numbers

	for n := 1; n <= total; n++ {
		hdr, readErr := b.readMessageHeaderLocked(n)
		if readErr != nil {
			continue
		}
		if hdr.Attribute&MsgDeleted != 0 {
			continue
		}

		var msgID, replyID string
		for _, sf := range hdr.Subfields {
			switch sf.LoID {
			case SfldMsgID:
				msgID = string(sf.Buffer)
			case SfldReplyID:
				replyID = string(sf.Buffer)
			}
		}

		headers = append(headers, hdrInfo{hdr: hdr, msgNum: n, msgID: msgID, replyID: replyID})
		if msgID != "" {
			msgIDToNum[msgID] = n
			// FTN MSGIDs are "address serial" — index the address-only
			// prefix too so prefix-based lookups succeed.
			if idx := strings.LastIndex(msgID, " "); idx > 0 {
				prefix := msgID[:idx]
				if _, exists := msgIDToNum[prefix]; !exists {
					msgIDToNum[prefix] = n
				}
			}
		}
		if replyID != "" {
			replyIDToNums[replyID] = append(replyIDToNums[replyID], n)
		}
	}

	result.MessagesScanned = len(headers)
	if len(headers) == 0 {
		return result, nil
	}

	// Phase 2: Compute desired threading fields and write changes.
	for i := range headers {
		h := &headers[i]
		changed := false

		// ReplyTo: if this message has a ReplyID, find the parent's message number.
		if h.replyID != "" {
			if parentNum, ok := msgIDToNum[h.replyID]; ok {
				if h.hdr.ReplyTo != uint32(parentNum) {
					h.hdr.ReplyTo = uint32(parentNum)
					changed = true
				}
			}
		}

		// Reply1st: if this message has a MSGID with replies, point to the first reply.
		if h.msgID != "" {
			replies := replyIDToNums[h.msgID]
			if len(replies) == 0 {
				if idx := strings.LastIndex(h.msgID, " "); idx > 0 {
					replies = replyIDToNums[h.msgID[:idx]]
				}
			}
			if len(replies) > 0 {
				firstReply := replies[0]
				if h.hdr.Reply1st != uint32(firstReply) {
					h.hdr.Reply1st = uint32(firstReply)
					changed = true
				}
			} else if h.hdr.Reply1st != 0 {
				h.hdr.Reply1st = 0
				changed = true
			}
		}

		// ReplyNext: chain sibling replies to the same parent.
		if h.replyID != "" {
			if siblings, ok := replyIDToNums[h.replyID]; ok && len(siblings) > 1 {
				nextSibling := uint32(0)
				for j, sn := range siblings {
					if sn == h.msgNum && j+1 < len(siblings) {
						nextSibling = uint32(siblings[j+1])
						break
					}
				}
				if h.hdr.ReplyNext != nextSibling {
					h.hdr.ReplyNext = nextSibling
					changed = true
				}
			} else if h.hdr.ReplyNext != 0 {
				h.hdr.ReplyNext = 0
				changed = true
			}
		}

		if changed {
			if err := b.updateMessageHeaderLocked(h.msgNum, h.hdr); err != nil {
				return result, fmt.Errorf("updating message %d: %w", h.msgNum, err)
			}
			result.LinksUpdated++
		}
	}

	return result, nil
}
