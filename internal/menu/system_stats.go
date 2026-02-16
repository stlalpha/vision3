package menu

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	term "golang.org/x/term"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/jam"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
)

func runSystemStats(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	topPath := filepath.Join(e.MenuSetPath, "templates", "SYSSTATS.TOP")
	botPath := filepath.Join(e.MenuSetPath, "templates", "SYSSTATS.BOT")

	topBytes, err := os.ReadFile(topPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Node %d: Failed to read %s: %v", nodeNumber, topPath, err)
	}
	botBytes, err := os.ReadFile(botPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Node %d: Failed to read %s: %v", nodeNumber, botPath, err)
	}

	topBytes = stripSauceMetadata(topBytes)
	botBytes = stripSauceMetadata(botBytes)
	topBytes = normalizePipeCodeDelimiters(topBytes)
	botBytes = normalizePipeCodeDelimiters(botBytes)

	now := time.Now()
	tokens := map[string]string{
		"BBSNAME":     e.ServerCfg.BoardName,
		"SYSOP":       e.ServerCfg.SysOpName,
		"VERSION":     jam.Version,
		"TOTALUSERS":  strconv.Itoa(userManager.GetUserCount()),
		"TOTALCALLS":  strconv.FormatUint(userManager.GetTotalCalls(), 10),
		"TOTALMSGS":   strconv.Itoa(e.MessageMgr.GetTotalMessageCount()),
		"TOTALFILES":  strconv.Itoa(e.FileMgr.GetTotalFileCount()),
		"ACTIVENODES": strconv.Itoa(e.SessionRegistry.ActiveCount()),
		"MAXNODES":    strconv.Itoa(e.ServerCfg.MaxNodes),
		"DATE":        now.Format("01/02/2006"),
		"TIME":        now.Format("03:04 PM"),
	}

	// Apply @TOKEN@ substitution to template content
	replaceTokens := func(data []byte) []byte {
		s := string(data)
		for key, val := range tokens {
			s = strings.ReplaceAll(s, "@"+key+"@", val)
		}
		return []byte(s)
	}
	topBytes = replaceTokens(topBytes)
	botBytes = replaceTokens(botBytes)

	lines := []string{
		fmt.Sprintf(" |07BBS Name:       |15%s", tokens["BBSNAME"]),
		fmt.Sprintf(" |07SysOp:          |15%s", tokens["SYSOP"]),
		fmt.Sprintf(" |07Version:        |15ViSiON/3 v%s", tokens["VERSION"]),
		"",
		fmt.Sprintf(" |07Total Users:    |15%s", tokens["TOTALUSERS"]),
		fmt.Sprintf(" |07Total Calls:    |15%s", tokens["TOTALCALLS"]),
		fmt.Sprintf(" |07Total Messages: |15%s", tokens["TOTALMSGS"]),
		fmt.Sprintf(" |07Total Files:    |15%s", tokens["TOTALFILES"]),
		fmt.Sprintf(" |07Active Nodes:   |15%s |07/ |15%s", tokens["ACTIVENODES"], tokens["MAXNODES"]),
		"",
		fmt.Sprintf(" |07Date:           |15%s", tokens["DATE"]),
		fmt.Sprintf(" |07Time:           |15%s", tokens["TIME"]),
	}

	var buf bytes.Buffer
	buf.Write([]byte(ansi.ClearScreen()))

	if len(topBytes) > 0 {
		buf.Write(ansi.ReplacePipeCodes(topBytes))
		buf.WriteString("\r\n")
	}

	for _, line := range lines {
		buf.Write(ansi.ReplacePipeCodes([]byte(line)))
		buf.WriteString("\r\n")
	}

	if len(botBytes) > 0 {
		buf.Write(ansi.ReplacePipeCodes(botBytes))
		buf.WriteString("\r\n")
	}

	terminalio.WriteProcessedBytes(terminal, buf.Bytes(), outputMode)

	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... "
	}
	if err := writeCenteredPausePrompt(s, terminal, pausePrompt, outputMode); err != nil {
		return nil, "", err
	}

	return nil, "", nil
}
