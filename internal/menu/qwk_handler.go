package menu

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/qwk"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/transfer"
	"github.com/stlalpha/vision3/internal/user"
)

// qwkBBSID returns a short BBS identifier for QWK packet filenames.
// Derived from BoardName: alphanumeric only, max 8 chars, uppercase.
func qwkBBSID(boardName string) string {
	var b strings.Builder
	for _, r := range boardName {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			if b.Len() >= 8 {
				break
			}
		}
	}
	id := strings.ToUpper(b.String())
	if id == "" {
		id = "BBS"
	}
	return id
}

// runQWKDownload builds and sends a QWK mail packet to the user.
func runQWKDownload(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running QWKDOWNLOAD", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	bbsID := qwkBBSID(e.ServerCfg.BoardName)
	pw := qwk.NewPacketWriter(bbsID, e.ServerCfg.BoardName, e.ServerCfg.SysOpName)
	pw.SetPersonalTo(currentUser.Handle)

	// Gather messages from areas the user has tagged for newscan
	taggedAreas := currentUser.TaggedMessageAreaTags
	if len(taggedAreas) == 0 {
		// Fall back to all areas the user can access
		for _, area := range e.MessageMgr.ListAreas() {
			taggedAreas = append(taggedAreas, area.Tag)
		}
	}

	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|15Building QWK packet...|07\r\n")), outputMode)

	// pendingLastRead accumulates per-area last-read updates.
	// They are only committed to the database after a successful transfer
	// so that a failed or cancelled download does not advance the pointers.
	type lastReadUpdate struct {
		areaID int
		msgNum int
	}
	var pendingLastRead []lastReadUpdate

	totalMsgs := 0
	for _, areaTag := range taggedAreas {
		area, exists := e.MessageMgr.GetAreaByTag(areaTag)
		if !exists {
			continue
		}

		pw.AddConference(area.ID, area.Name)

		// Get last read for this user in this area
		lastRead, err := e.MessageMgr.GetLastRead(area.ID, currentUser.Handle)
		if err != nil {
			log.Printf("WARN: Node %d: QWK: failed to get lastread for area %d: %v", nodeNumber, area.ID, err)
			continue
		}

		msgCount, err := e.MessageMgr.GetMessageCountForArea(area.ID)
		if err != nil {
			log.Printf("WARN: Node %d: QWK: failed to get msg count for area %d: %v", nodeNumber, area.ID, err)
			continue
		}

		// Pack new messages (up to 500 per area to limit packet size)
		maxPerArea := 500
		packed := 0
		highestPacked := lastRead
		for msgNum := lastRead + 1; msgNum <= msgCount && packed < maxPerArea; msgNum++ {
			msg, err := e.MessageMgr.GetMessage(area.ID, msgNum)
			if err != nil {
				continue
			}
			if msg.IsDeleted {
				continue
			}

			pw.AddMessage(qwk.PacketMessage{
				Conference: area.ID,
				Number:     msg.MsgNum,
				From:       msg.From,
				To:         msg.To,
				Subject:    msg.Subject,
				DateTime:   msg.DateTime,
				Body:       msg.Body,
				Private:    msg.IsPrivate,
			})
			packed++
			totalMsgs++
			if msgNum > highestPacked {
				highestPacked = msgNum
			}
		}

		if packed > 0 {
			newLastRead := highestPacked
			if newLastRead > msgCount {
				newLastRead = msgCount
			}
			pendingLastRead = append(pendingLastRead, lastReadUpdate{areaID: area.ID, msgNum: newLastRead})
		}
	}

	if totalMsgs == 0 {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07No new messages to download.|07\r\n")), outputMode)
		time.Sleep(2 * time.Second)
		return currentUser, "", nil
	}

	statusMsg := fmt.Sprintf("\r\n|14%d|07 message(s) packed into QWK packet.\r\n", totalMsgs)
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(statusMsg)), outputMode)

	// Prompt user to send or quit
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n"+e.LoadedStrings.SendQWKPacketPrompt)), outputMode)
	promptInput, promptErr := readLineFromSessionIH(s, terminal)
	if promptErr != nil {
		if errors.Is(promptErr, io.EOF) {
			return nil, "LOGOFF", promptErr
		}
		return currentUser, "", nil
	}
	if strings.ToUpper(strings.TrimSpace(promptInput)) == "Q" {
		return currentUser, "", nil
	}

	// Write packet to temp file
	tmpFile, err := os.CreateTemp("", "qwk-*.zip")
	if err != nil {
		log.Printf("ERROR: Node %d: QWK: failed to create temp file: %v", nodeNumber, err)
		return currentUser, "", nil
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if err := pw.WritePacket(tmpFile); err != nil {
		tmpFile.Close()
		log.Printf("ERROR: Node %d: QWK: failed to write packet: %v", nodeNumber, err)
		return currentUser, "", nil
	}
	tmpFile.Close()

	// Rename to BBSID.QWK for the transfer
	qwkPath := filepath.Join(filepath.Dir(tmpPath), bbsID+".QWK")
	if err := os.Rename(tmpPath, qwkPath); err != nil {
		log.Printf("ERROR: Node %d: QWK: rename failed: %v", nodeNumber, err)
		return currentUser, "", nil
	}
	defer os.Remove(qwkPath)

	// Protocol selection and send
	proto, ok, protoErr := e.selectTransferProtocol(s, terminal, outputMode)
	if protoErr != nil {
		if errors.Is(protoErr, io.EOF) {
			return nil, "LOGOFF", protoErr
		}
		return currentUser, "", nil
	}
	if !ok {
		return currentUser, "", nil
	}

	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(fmt.Sprintf("\r\n|15Sending %s.QWK via %s...|07\r\n", bbsID, proto.Name))), outputMode)

	resetSessionIH(s)
	ctx, cancel := e.transferContext(s.Context())
	defer cancel()
	sendErr := proto.ExecuteSend(ctx, s, qwkPath)
	time.Sleep(250 * time.Millisecond)
	getSessionIH(s)

	if sendErr != nil {
		if errors.Is(sendErr, transfer.ErrBinaryNotFound) {
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Transfer program not found!|07\r\n")), outputMode)
		} else {
			log.Printf("WARN: Node %d: QWK download transfer failed: %v", nodeNumber, sendErr)
			terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Transfer failed.|07\r\n")), outputMode)
		}
	} else {
		// Transfer succeeded — commit the newscan pointer advances.
		for _, upd := range pendingLastRead {
			if err := e.MessageMgr.SetLastRead(upd.areaID, currentUser.Handle, upd.msgNum); err != nil {
				log.Printf("WARN: Node %d: QWK: failed to update lastread for area %d: %v", nodeNumber, upd.areaID, err)
			}
		}
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|10QWK packet sent successfully.|07\r\n")), outputMode)
	}
	time.Sleep(2 * time.Second)

	return currentUser, "", nil
}

// runQWKUpload receives and processes a QWK REP packet from the user.
func runQWKUpload(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	log.Printf("DEBUG: Node %d: Running QWKUPLOAD", nodeNumber)

	if currentUser == nil {
		msg := "\r\n|01Error: You must be logged in.|07\r\n"
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return nil, "", nil
	}

	bbsID := qwkBBSID(e.ServerCfg.BoardName)

	// Protocol selection
	proto, ok, protoErr := e.selectTransferProtocol(s, terminal, outputMode)
	if protoErr != nil {
		if errors.Is(protoErr, io.EOF) {
			return nil, "LOGOFF", protoErr
		}
		return currentUser, "", nil
	}
	if !ok {
		return currentUser, "", nil
	}

	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(fmt.Sprintf("\r\n|11Send your %s.REP file via %s now.|07\r\n", bbsID, proto.Name))), outputMode)

	// Receive into temp directory
	incomingDir, err := os.MkdirTemp("", "qwk-rep-*")
	if err != nil {
		log.Printf("ERROR: Node %d: QWK: failed to create temp dir: %v", nodeNumber, err)
		return currentUser, "", nil
	}
	defer os.RemoveAll(incomingDir)

	resetSessionIH(s)
	ctx, cancel := e.transferContext(s.Context())
	defer cancel()
	recvErr := proto.ExecuteReceive(ctx, s, incomingDir)
	time.Sleep(250 * time.Millisecond)
	getSessionIH(s)

	if recvErr != nil && !errors.Is(recvErr, context.Canceled) {
		log.Printf("WARN: Node %d: QWK REP receive: %v (checking for files anyway)", nodeNumber, recvErr)
	}

	// Find the .REP file
	repPath := findREPFile(incomingDir, bbsID)
	if repPath == "" {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01No REP packet received.|07\r\n")), outputMode)
		time.Sleep(2 * time.Second)
		return currentUser, "", nil
	}

	// Process the REP packet
	repInfo, err := os.Stat(repPath)
	if err != nil {
		log.Printf("ERROR: Node %d: QWK: failed to stat REP: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	repData, err := os.ReadFile(repPath)
	if err != nil {
		log.Printf("ERROR: Node %d: QWK: failed to read REP: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	messages, err := qwk.ReadREP(bytes.NewReader(repData), repInfo.Size(), bbsID)
	if err != nil {
		log.Printf("ERROR: Node %d: QWK: failed to parse REP: %v", nodeNumber, err)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|01Error reading REP packet.|07\r\n")), outputMode)
		time.Sleep(2 * time.Second)
		return currentUser, "", nil
	}

	if len(messages) == 0 {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n|07REP packet contains no messages.|07\r\n")), outputMode)
		time.Sleep(2 * time.Second)
		return currentUser, "", nil
	}

	// Post each message
	posted := 0
	for _, msg := range messages {
		area, exists := e.MessageMgr.GetAreaByID(msg.Conference)
		if !exists {
			log.Printf("WARN: Node %d: QWK REP: unknown conference %d, skipping", nodeNumber, msg.Conference)
			continue
		}

		// Check write ACS
		if area.ACSWrite != "" && !checkACS(area.ACSWrite, currentUser, s, terminal, sessionStartTime) {
			log.Printf("WARN: Node %d: QWK REP: user lacks write ACS for area %s", nodeNumber, area.Tag)
			continue
		}

		postMsg := strings.ReplaceAll(e.LoadedStrings.PostingQWKMsg, "|BN", area.Name)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n"+postMsg)), outputMode)

		_, err := e.MessageMgr.AddMessage(area.ID, currentUser.Handle, msg.To, msg.Subject, msg.Body, "")
		if err != nil {
			log.Printf("ERROR: Node %d: QWK REP: failed to post to area %d: %v", nodeNumber, area.ID, err)
			continue
		}
		posted++
	}

	// Update user stats
	if posted > 0 && userManager != nil {
		currentUser.MessagesPosted += posted
		if updateErr := userManager.UpdateUser(currentUser); updateErr != nil {
			log.Printf("ERROR: Node %d: QWK: failed to update user stats: %v", nodeNumber, updateErr)
		}
	}

	statusMsg := strings.ReplaceAll(e.LoadedStrings.TotalQWKAdded, "|TO", fmt.Sprintf("%d", posted))
	terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte("\r\n"+statusMsg+"\r\n")), outputMode)
	time.Sleep(2 * time.Second)

	return currentUser, "", nil
}

// findREPFile looks for a .REP file in the directory, matching the BBS ID.
func findREPFile(dir string, bbsID string) string {
	expected := strings.ToUpper(bbsID) + ".REP"
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name(), expected) {
			return filepath.Join(dir, e.Name())
		}
	}
	// Fall back: any .REP file
	for _, e := range entries {
		if strings.HasSuffix(strings.ToUpper(e.Name()), ".REP") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}
