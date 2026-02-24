package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
)

// runSponsorMenu is the handler for RUN:SPONSORMENU.
//
// Triggered by "%" in the Messages Menu. Sysop, co-sysop, or the named area
// sponsor may enter; all others are silently refused.
//
// Flow:
//  1. Resolve the user's current message area.
//  2. Gate via CanAccessSponsorMenu.
//  3. Display SPONSORM.ANS header.
//  4. Single-key loop: E=Edit Area, Q=Quit.
func runSponsorMenu(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User,
	nodeNumber int, sessionStartTime time.Time, args string,
	outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return nil, "", nil
	}

	if e.MessageMgr == nil || currentUser.CurrentMessageAreaID == 0 {
		msg := "\r\n|03No message area selected.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	area, found := e.MessageMgr.GetAreaByID(currentUser.CurrentMessageAreaID)
	if !found {
		msg := "\r\n|03No message area selected.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	cfg := e.GetServerConfig()
	if !CanAccessSponsorMenu(currentUser, area, cfg) {
		log.Printf("INFO: Node %d: User %s denied sponsor menu for area %s",
			nodeNumber, currentUser.Handle, area.Tag)
		return currentUser, "", nil
	}

	log.Printf("INFO: Node %d: User %s entering sponsor menu for area %s",
		nodeNumber, currentUser.Handle, area.Tag)

	if err := e.displayFile(terminal, "SPONSORM.ANS", outputMode); err != nil {
		log.Printf("WARN: Node %d: Failed to display SPONSORM.ANS: %v", nodeNumber, err)
	}

	ih := getSessionIH(s)

	for {
		prompt := fmt.Sprintf("\r\n|15[|14%s|15] Sponsor: |11E|07=Edit Area  |11Q|07=Quit: ", area.Tag)
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

		key, err := ih.ReadKey()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return currentUser, "", err
		}

		switch key {
		case int('e'), int('E'):
			updated, next, runErr := runSponsorEditArea(e, s, terminal, userManager,
				currentUser, nodeNumber, sessionStartTime, args, outputMode, termWidth, termHeight)
			if runErr != nil {
				if errors.Is(runErr, io.EOF) {
					return nil, "LOGOFF", io.EOF
				}
				return updated, "", runErr
			}
			if next != "" {
				return updated, next, nil
			}
			currentUser = updated
			// Re-fetch area in case the edit updated its name/tag display.
			if a, ok := e.MessageMgr.GetAreaByID(currentUser.CurrentMessageAreaID); ok {
				area = a
			}

		case int('q'), int('Q'), 27: // ESC
			_ = terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)
			return currentUser, "", nil
		}
	}
}

// runSponsorEditArea is the handler for RUN:SPONSOREDITAREA.
//
// Sequential field editor for the current message area. Editable fields:
// Name, Description, ACS Read, ACS Write, Sponsor handle, Max Messages.
//
// Key map:
//   - N/D/R/W/S/M — edit the named field
//   - Q           — save changes and exit
//   - ESC         — discard changes and exit
//
// The Sponsor field is validated against the user database. Enter "-" to clear.
func runSponsorEditArea(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User,
	nodeNumber int, sessionStartTime time.Time, args string,
	outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return nil, "", nil
	}

	if e.MessageMgr == nil || currentUser.CurrentMessageAreaID == 0 {
		msg := "\r\n|03No message area selected.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	area, found := e.MessageMgr.GetAreaByID(currentUser.CurrentMessageAreaID)
	if !found {
		msg := "\r\n|03Area not found.|07\r\n"
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
		time.Sleep(1 * time.Second)
		return currentUser, "", nil
	}

	cfg := e.GetServerConfig()
	if !CanAccessSponsorMenu(currentUser, area, cfg) {
		return currentUser, "", nil
	}

	// Work on a copy; apply to the live pointer only on save.
	edited := *area

	showFields := func() {
		var b strings.Builder
		b.WriteString("\r\n")
		b.WriteString(fmt.Sprintf("|15Edit Area: |14%s|07\r\n", edited.Tag))
		b.WriteString("|08────────────────────────────────────────────────────\r\n")
		b.WriteString(fmt.Sprintf("|11N|07) Name         : |15%s\r\n", edited.Name))
		b.WriteString(fmt.Sprintf("|11D|07) Description  : |15%s\r\n", edited.Description))
		b.WriteString(fmt.Sprintf("|11R|07) ACS Read     : |15%s\r\n", edited.ACSRead))
		b.WriteString(fmt.Sprintf("|11W|07) ACS Write    : |15%s\r\n", edited.ACSWrite))
		b.WriteString(fmt.Sprintf("|11S|07) Sponsor      : |15%s\r\n", edited.Sponsor))
		b.WriteString(fmt.Sprintf("|11M|07) Max Messages : |15%d\r\n", edited.MaxMessages))
		b.WriteString("|08────────────────────────────────────────────────────\r\n")
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(b.String())), outputMode)
	}

	showFields()

	ih := getSessionIH(s)

	for {
		prompt := "|07Edit (|11N|07/|11D|07/|11R|07/|11W|07/|11S|07/|11M|07)  |11Q|07=Save  |03ESC|07=Cancel: "
		_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

		key, err := ih.ReadKey()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return currentUser, "", err
		}

		_ = terminalio.WriteProcessedBytes(terminal, []byte("\r\n"), outputMode)

		switch key {
		case int('n'), int('N'):
			edited.Name = promptAreaField(s, terminal, outputMode, "Name", edited.Name, 60)
			showFields()

		case int('d'), int('D'):
			edited.Description = promptAreaField(s, terminal, outputMode,
				"Description", edited.Description, 80)
			showFields()

		case int('r'), int('R'):
			edited.ACSRead = promptAreaField(s, terminal, outputMode,
				"ACS Read", edited.ACSRead, 40)
			showFields()

		case int('w'), int('W'):
			edited.ACSWrite = promptAreaField(s, terminal, outputMode,
				"ACS Write", edited.ACSWrite, 40)
			showFields()

		case int('s'), int('S'):
			newHandle := promptAreaField(s, terminal, outputMode,
				"Sponsor handle (- to clear)", edited.Sponsor, 30)
			switch {
			case newHandle == "-":
				edited.Sponsor = ""
			case newHandle != "":
				if userManager != nil {
					if _, exists := userManager.GetUserByHandle(newHandle); !exists {
						msg := fmt.Sprintf("|01User '%s' not found — sponsor unchanged.|07\r\n", newHandle)
						_ = terminalio.WriteProcessedBytes(terminal,
							ansi.ReplacePipeCodes([]byte(msg)), outputMode)
						time.Sleep(1 * time.Second)
					} else {
						edited.Sponsor = newHandle
					}
				} else {
					edited.Sponsor = newHandle
				}
			}
			showFields()

		case int('m'), int('M'):
			raw := promptAreaField(s, terminal, outputMode,
				"Max Messages (0=unlimited)", fmt.Sprintf("%d", edited.MaxMessages), 10)
			if raw != "" {
				var n int
				if _, scanErr := fmt.Sscanf(raw, "%d", &n); scanErr == nil && n >= 0 {
					edited.MaxMessages = n
				} else {
					msg := "|01Invalid number — unchanged.|07\r\n"
					_ = terminalio.WriteProcessedBytes(terminal,
						ansi.ReplacePipeCodes([]byte(msg)), outputMode)
					time.Sleep(1 * time.Second)
				}
			}
			showFields()

		case int('q'), int('Q'): // Q = save and quit
			*area = edited
			if saveErr := e.MessageMgr.SaveAreas(); saveErr != nil {
				log.Printf("ERROR: Node %d: Failed to save areas: %v", nodeNumber, saveErr)
				msg := "|01Error saving area — changes may be lost.|07\r\n"
				_ = terminalio.WriteProcessedBytes(terminal,
					ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(2 * time.Second)
			} else {
				log.Printf("INFO: Node %d: User %s saved area %s", nodeNumber, currentUser.Handle, edited.Tag)
				msg := fmt.Sprintf("|02Area |14%s|02 saved.|07\r\n", edited.Tag)
				_ = terminalio.WriteProcessedBytes(terminal,
					ansi.ReplacePipeCodes([]byte(msg)), outputMode)
				time.Sleep(500 * time.Millisecond)
			}
			return currentUser, "", nil

		case 27: // ESC = discard and quit
			msg := "|03Changes discarded.|07\r\n"
			_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(500 * time.Millisecond)
			return currentUser, "", nil
		}
	}
}

// promptAreaField prints a prompt, reads a line, and returns the new value.
// If the user presses Enter with no input, the original value is returned
// unchanged.
func promptAreaField(s ssh.Session, terminal *term.Terminal,
	outputMode ansi.OutputMode, label, current string, maxLen int) string {

	prompt := fmt.Sprintf("|15%s|07 [|11%s|07]: ", label, current)
	_ = terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

	input, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return current
	}
	input = strings.TrimSpace(input)
	if input == "" {
		return current
	}
	if len(input) > maxLen {
		input = input[:maxLen]
	}
	return input
}
