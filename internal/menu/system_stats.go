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
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"github.com/stlalpha/vision3/internal/version"
)

func runSystemStats(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	if currentUser == nil {
		return nil, "", nil
	}

	topPath := filepath.Join(e.MenuSetPath, "templates", "SYSSTATS.TOP")
	botPath := filepath.Join(e.MenuSetPath, "templates", "SYSSTATS.BOT")

	topBytes, err := readTemplateFile(topPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Node %d: Failed to read %s: %v", nodeNumber, topPath, err)
	}
	botBytes, err := readTemplateFile(botPath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("WARN: Node %d: Failed to read %s: %v", nodeNumber, botPath, err)
	}

	topBytes = stripSauceMetadata(topBytes)
	botBytes = stripSauceMetadata(botBytes)
	topBytes = normalizePipeCodeDelimiters(topBytes)
	botBytes = normalizePipeCodeDelimiters(botBytes)

	sysopName := ""
	if sysopUser, ok := userManager.GetUserByID(1); ok {
		sysopName = sysopUser.Handle
	}

	now := config.NowIn(e.ServerCfg.Timezone)
	tokens := map[string]string{
		"BBSNAME":     e.ServerCfg.BoardName,
		"SYSOP":       sysopName,
		"VERSION":     version.Number,
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
		fmt.Sprintf(e.LoadedStrings.StatsBBSName, tokens["BBSNAME"]),
		fmt.Sprintf(e.LoadedStrings.StatsSysOp, tokens["SYSOP"]),
		fmt.Sprintf(e.LoadedStrings.StatsVersion, tokens["VERSION"]),
		"",
		fmt.Sprintf(e.LoadedStrings.StatsTotalUsers, tokens["TOTALUSERS"]),
		fmt.Sprintf(e.LoadedStrings.StatsTotalCalls, tokens["TOTALCALLS"]),
		fmt.Sprintf(e.LoadedStrings.StatsTotalMsgs, tokens["TOTALMSGS"]),
		fmt.Sprintf(e.LoadedStrings.StatsTotalFiles, tokens["TOTALFILES"]),
		fmt.Sprintf(e.LoadedStrings.StatsActiveNodes, tokens["ACTIVENODES"], tokens["MAXNODES"]),
		"",
		fmt.Sprintf(e.LoadedStrings.StatsDate, tokens["DATE"]),
		fmt.Sprintf(e.LoadedStrings.StatsTime, tokens["TIME"]),
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
	if err := writeCenteredPausePrompt(s, terminal, pausePrompt, outputMode, termWidth, termHeight); err != nil {
		return nil, "", err
	}

	return nil, "", nil
}
