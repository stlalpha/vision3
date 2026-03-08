package menu

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// runCheckNUV is the login-sequence hook: if the user qualifies to vote (UseNUV
// and AccessLevel >= NUVUseLevel), and there are pending candidates they
// haven't voted on yet, notify them and offer a quick-scan.
// Mapped to login command "CHECKNUV".
func runCheckNUV(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	cfg := e.GetServerConfig()
	if !cfg.UseNUV || currentUser == nil {
		return currentUser, "", nil
	}
	if currentUser.AccessLevel < cfg.NUVUseLevel {
		return currentUser, "", nil
	}

	nuvMu.Lock()
	nd, err := loadNUVData(e.RootConfigPath)
	nuvMu.Unlock()
	if err != nil || len(nd.Candidates) == 0 {
		return currentUser, "", nil
	}

	// Count how many candidates this user hasn't voted on yet.
	unvoted := 0
	for i := range nd.Candidates {
		if nuvVoteIndex(&nd.Candidates[i], currentUser.Handle) < 0 {
			unvoted++
		}
	}
	if unvoted == 0 {
		return currentUser, "", nil
	}

	wv(terminal, fmt.Sprintf("\r\n|15New User Voting: |11%d candidate(s)|15 awaiting your vote!\r\n", unvoted), outputMode)
	wv(terminal, "|07Vote now? |15[Y/N]|07: ", outputMode)

	ih := getSessionIH(s)
	key, err := ih.ReadKey()
	if err != nil {
		return currentUser, "", nil
	}
	if key != 'Y' && key != 'y' {
		wv(terminal, "\r\n", outputMode)
		return currentUser, "", nil
	}
	wv(terminal, "\r\n", outputMode)

	// Quick-scan: iterate through unvoted candidates.
	nuvMu.Lock()
	nd, err = loadNUVData(e.RootConfigPath)
	nuvMu.Unlock()
	if err != nil {
		return currentUser, "", nil
	}
	nuvRunScan(e, s, terminal, userManager, currentUser, nd, outputMode, termWidth, termHeight)
	return currentUser, "", nil
}

// runNUVScan lets the current user vote on all pending NUV candidates they
// haven't voted on. Mapped to menu command "SCANNUV".
func runNUVScan(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	cfg := e.GetServerConfig()
	if !cfg.UseNUV {
		wv(terminal, "\r\n|07New User Voting is disabled.\r\n", outputMode)
		e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
		return currentUser, "", nil
	}
	if currentUser.AccessLevel < cfg.NUVUseLevel {
		wv(terminal, "\r\n|12You do not have access to New User Voting.\r\n", outputMode)
		e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
		return currentUser, "", nil
	}

	nuvMu.Lock()
	nd, err := loadNUVData(e.RootConfigPath)
	nuvMu.Unlock()
	if err != nil {
		log.Printf("WARN: Node %d: SCANNUV: load error: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	if len(nd.Candidates) == 0 {
		wv(terminal, "\r\n|07No candidates pending in NUV queue.\r\n", outputMode)
		e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
		return currentUser, "", nil
	}

	nuvRunScan(e, s, terminal, userManager, currentUser, nd, outputMode, termWidth, termHeight)
	return currentUser, "", nil
}

// runNUVList displays all current NUV candidates with their vote tallies.
// Mapped to menu command "LISTNUV". SysOp-level; no UseNUV guard so the
// queue is always inspectable regardless of whether NUV is currently enabled.
func runNUVList(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	nuvMu.Lock()
	nd, err := loadNUVData(e.RootConfigPath)
	nuvMu.Unlock()
	if err != nil {
		log.Printf("WARN: Node %d: LISTNUV: load error: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2J\x1b[H"), outputMode)
	wv(terminal, fmt.Sprintf("|15New User Voting Queue — %d Candidate(s)\r\n", len(nd.Candidates)), outputMode)
	wv(terminal, fmt.Sprintf("|08%s\r\n", strings.Repeat("\xc4", 60)), outputMode)

	if len(nd.Candidates) == 0 {
		wv(terminal, "|07No candidates pending.\r\n", outputMode)
	} else {
		wv(terminal, fmt.Sprintf("|08%-4s %-20s %-10s %4s %4s %6s\r\n", "#", "Handle", "Added", "Yes", "No", "Voted?"), outputMode)
		wv(terminal, fmt.Sprintf("|08%s\r\n", strings.Repeat("\xc4", 60)), outputMode)
		for i, c := range nd.Candidates {
			yes := nuvYesCount(&c)
			no := len(c.Votes) - yes
			voted := "|12No "
			if nuvVoteIndex(&c, currentUser.Handle) >= 0 {
				voted = "|10Yes"
			}
			wv(terminal, fmt.Sprintf("|07%-4d |11%-20s |07%-10s |10%4d |12%4d |07%s\r\n",
				i+1, c.Handle, c.When.Format("01/02/06"), yes, no, voted), outputMode)
		}
	}
	wv(terminal, "\r\n", outputMode)
	e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
	return currentUser, "", nil
}

// nuvRunScan iterates through all candidates the user hasn't voted on and
// calls nuvVoteOn for each. Modifies nd in-place as votes are cast.
func nuvRunScan(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User,
	nd *NUVData, outputMode ansi.OutputMode, termWidth, termHeight int) {

	for i := 0; i < len(nd.Candidates); {
		if nuvVoteIndex(&nd.Candidates[i], currentUser.Handle) >= 0 {
			i++
			continue
		}
		removed := nuvVoteOn(e, s, terminal, userManager, currentUser, nd, i, outputMode, termWidth, termHeight)
		if removed {
			// candidate was deleted from nd; don't increment i
			continue
		}
		// user quit or voted without threshold trigger
		break
	}
}
