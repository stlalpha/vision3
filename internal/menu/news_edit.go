package menu

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// runEditNews is the sysop news management interface (Add/Delete/Edit/List/View).
// Maps to V2's editnews procedure in MAINMENU.PAS.
func runEditNews(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if !e.isCoSysOpOrAbove(currentUser) {
		return currentUser, "", nil
	}
	log.Printf("DEBUG: Node %d: Running EDITNEWS (sysop) for user %s", nodeNumber, currentUser.Handle)

	for {
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2J\x1b[H"), outputMode)

		newsMu.Lock()
		nd, err := loadNewsData(e.RootConfigPath)
		newsMu.Unlock()
		if err != nil {
			wv(terminal, "\r\n|04Error loading news.\r\n", outputMode)
			return currentUser, "", nil
		}

		wv(terminal, fmt.Sprintf("\r\n|15System News Management |07(%d items)\r\n|08%s\r\n",
			len(nd.Items), strings.Repeat("\xc4", 50)), outputMode)
		wv(terminal, "|15[A]|07dd  |15[D]|07el  |15[E]|07dit  |15[L]|07ist  |15[V]|07iew  |15[Q]|07uit: ", outputMode)

		input, err := readLineFromSessionIH(s, terminal)
		if err != nil {
			return currentUser, "", nil
		}
		cmd := strings.ToUpper(strings.TrimSpace(input))

		switch cmd {
		case "Q", "":
			return currentUser, "", nil
		case "L":
			newsListSysop(terminal, nd, outputMode)
			e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
		case "A":
			newsAddItem(e, s, terminal, currentUser, nd, outputMode)
		case "D":
			newsDeleteItem(e, s, terminal, nd, outputMode)
		case "E":
			newsEditItem(e, s, terminal, nd, outputMode)
		case "V":
			newsViewItem(e, s, terminal, nd, outputMode, termWidth, termHeight)
		default:
			n, nerr := strconv.Atoi(cmd)
			if nerr == nil && n >= 1 && n <= len(nd.Items) {
				displayNewsItem(e, terminal, &nd.Items[n-1], n, outputMode)
				e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
			}
		}
	}
}

func newsListSysop(terminal *term.Terminal, nd *NewsData, outputMode ansi.OutputMode) {
	wv(terminal, "\r\n|15 #   Title                        Min    Max    Display\r\n", outputMode)
	wv(terminal, "|08"+strings.Repeat("\xc4", 60)+"\r\n", outputMode)
	for i, item := range nd.Items {
		shown := "|07Once"
		if item.Always {
			shown = "|10Always"
		}
		maxStr := strconv.Itoa(item.MaxLevel)
		if item.MaxLevel <= 0 {
			maxStr = "All"
		}
		wv(terminal, fmt.Sprintf("|11%2d|07. |15%-28s |07%-6d %-6s %s\r\n",
			i+1, item.Title, item.Level, maxStr, shown), outputMode)
	}
}

func newsAddItem(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	currentUser *user.User, nd *NewsData, outputMode ansi.OutputMode) {

	wv(terminal, "\r\n|15Adding News Item\r\n|08"+strings.Repeat("\xc4", 40)+"\r\n", outputMode)

	wv(terminal, "|07Title (max 28 chars): ", outputMode)
	title, err := readLineFromSessionIH(s, terminal)
	if err != nil || strings.TrimSpace(title) == "" {
		return
	}
	title = strings.TrimSpace(title)
	if len(title) > 28 {
		title = title[:28]
	}

	wv(terminal, "|07Minimum level to read |15[|110|15]|07: ", outputMode)
	lvlIn, _ := readLineFromSessionIH(s, terminal)
	level, _ := strconv.Atoi(strings.TrimSpace(lvlIn))

	wv(terminal, "|07Maximum level |15[|110=All|15]|07: ", outputMode)
	maxIn, _ := readLineFromSessionIH(s, terminal)
	maxLevel, _ := strconv.Atoi(strings.TrimSpace(maxIn))

	wv(terminal, "|07Always display every login? |15[Y/N]|07: ", outputMode)
	alwaysIn, _ := readLineFromSessionIH(s, terminal)
	always := strings.ToUpper(strings.TrimSpace(alwaysIn)) == "Y"

	wv(terminal, fmt.Sprintf("|07Author |15[|11%s|15]|07: ", currentUser.Handle), outputMode)
	fromIn, _ := readLineFromSessionIH(s, terminal)
	from := strings.TrimSpace(fromIn)
	if from == "" {
		from = currentUser.Handle
	}

	wv(terminal, "|07Enter news body (blank line to finish):\r\n", outputMode)
	var bodyLines []string
	for {
		wv(terminal, "|07> ", outputMode)
		line, lerr := readLineFromSessionIH(s, terminal)
		if lerr != nil || strings.TrimSpace(line) == "" {
			break
		}
		bodyLines = append(bodyLines, line)
	}

	if len(bodyLines) == 0 {
		wv(terminal, "|07No body entered, item not created.\r\n", outputMode)
		return
	}

	// Prepend newest first (matches V2: seek(0), shift all down, write at position 0)
	newsMu.Lock()
	fresh, loadErr := loadNewsData(e.RootConfigPath)
	if loadErr != nil {
		newsMu.Unlock()
		log.Printf("ERROR: Failed to load news data before add: %v", loadErr)
		wv(terminal, "|04Error adding news item.\r\n", outputMode)
		return
	}
	item := NewsItem{
		ID:       len(fresh.Items) + 1,
		Title:    title,
		From:     from,
		When:     time.Now(),
		Level:    level,
		MaxLevel: maxLevel,
		Always:   always,
		Body:     strings.Join(bodyLines, "\n"),
	}
	fresh.Items = append([]NewsItem{item}, fresh.Items...)
	saveErr := saveNewsData(e.RootConfigPath, fresh)
	newsMu.Unlock()
	if saveErr != nil {
		log.Printf("ERROR: Failed to save news data after add: %v", saveErr)
		wv(terminal, "|04Error saving news item.\r\n", outputMode)
		return
	}

	wv(terminal, "|10News item added!\r\n", outputMode)
}

func newsDeleteItem(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	nd *NewsData, outputMode ansi.OutputMode) {

	if len(nd.Items) == 0 {
		wv(terminal, "\r\n|07No news items to delete.\r\n", outputMode)
		return
	}
	newsListSysop(terminal, nd, outputMode)
	wv(terminal, "|07Delete which item #: ", outputMode)
	input, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return
	}
	n, nerr := strconv.Atoi(strings.TrimSpace(input))
	if nerr != nil || n < 1 || n > len(nd.Items) {
		wv(terminal, "\r\n|07Invalid selection.\r\n", outputMode)
		return
	}
	wv(terminal, fmt.Sprintf("|07Delete #%d (%s)? |15[Y/N]|07: ", n, nd.Items[n-1].Title), outputMode)
	confirm, _ := readLineFromSessionIH(s, terminal)
	if strings.ToUpper(strings.TrimSpace(confirm)) != "Y" {
		return
	}

	newsMu.Lock()
	fresh, loadErr := loadNewsData(e.RootConfigPath)
	if loadErr != nil {
		newsMu.Unlock()
		log.Printf("ERROR: Failed to load news data before delete: %v", loadErr)
		wv(terminal, "|04Error deleting news item.\r\n", outputMode)
		return
	}
	if n > len(fresh.Items) {
		newsMu.Unlock()
		wv(terminal, "|04Unable to delete item; please try again.\r\n", outputMode)
		return
	}
	fresh.Items = append(fresh.Items[:n-1], fresh.Items[n:]...)
	saveErr := saveNewsData(e.RootConfigPath, fresh)
	newsMu.Unlock()
	if saveErr != nil {
		log.Printf("ERROR: Failed to save news data after delete: %v", saveErr)
		wv(terminal, "|04Error deleting news item.\r\n", outputMode)
		return
	}
	wv(terminal, "|10Item deleted.\r\n", outputMode)
}

func newsEditItem(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	nd *NewsData, outputMode ansi.OutputMode) {

	if len(nd.Items) == 0 {
		wv(terminal, "\r\n|07No news items to edit.\r\n", outputMode)
		return
	}
	newsListSysop(terminal, nd, outputMode)
	wv(terminal, "|07Edit which item #: ", outputMode)
	input, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return
	}
	n, nerr := strconv.Atoi(strings.TrimSpace(input))
	if nerr != nil || n < 1 || n > len(nd.Items) {
		wv(terminal, "\r\n|07Invalid selection.\r\n", outputMode)
		return
	}

	item := nd.Items[n-1]
	for {
		maxStr := strconv.Itoa(item.MaxLevel)
		if item.MaxLevel <= 0 {
			maxStr = "All"
		}
		wv(terminal, fmt.Sprintf("\r\n|15News #%d\r\n", n), outputMode)
		wv(terminal, fmt.Sprintf("|15[T]|07itle......: |11%s\r\n", item.Title), outputMode)
		wv(terminal, fmt.Sprintf("|15[F]|07rom.......: |11%s\r\n", item.From), outputMode)
		wv(terminal, fmt.Sprintf("|15[L]|07evel (min): |11%d\r\n", item.Level), outputMode)
		wv(terminal, fmt.Sprintf("|15[X]|07 Max level: |11%s\r\n", maxStr), outputMode)
		wv(terminal, fmt.Sprintf("|15[A]|07lways.....: |11%v\r\n", item.Always), outputMode)
		wv(terminal, "|15[E]|07dit body  |15[Q]|07uit: ", outputMode)

		cmd, cerr := readLineFromSessionIH(s, terminal)
		if cerr != nil {
			break
		}
		switch strings.ToUpper(strings.TrimSpace(cmd)) {
		case "Q", "":
			newsMu.Lock()
			fresh, loadErr := loadNewsData(e.RootConfigPath)
			if loadErr != nil {
				newsMu.Unlock()
				log.Printf("ERROR: Failed to load news data before edit save: %v", loadErr)
				wv(terminal, "|04Error saving news item.\r\n", outputMode)
				return
			}
			if n > len(fresh.Items) {
				newsMu.Unlock()
				wv(terminal, "|04News item no longer exists.\r\n", outputMode)
				return
			}
			fresh.Items[n-1] = item
			saveErr := saveNewsData(e.RootConfigPath, fresh)
			newsMu.Unlock()
			if saveErr != nil {
				log.Printf("ERROR: Failed to save news data after edit: %v", saveErr)
				wv(terminal, "|04Error saving news item.\r\n", outputMode)
				return
			}
			wv(terminal, "|10Item saved.\r\n", outputMode)
			return
		case "T":
			wv(terminal, "|07New title: ", outputMode)
			val, _ := readLineFromSessionIH(s, terminal)
			if v := strings.TrimSpace(val); v != "" {
				if len(v) > 28 {
					v = v[:28]
				}
				item.Title = v
			}
		case "F":
			wv(terminal, "|07New author: ", outputMode)
			val, _ := readLineFromSessionIH(s, terminal)
			if v := strings.TrimSpace(val); v != "" {
				item.From = v
			}
		case "L":
			wv(terminal, "|07New min level: ", outputMode)
			val, _ := readLineFromSessionIH(s, terminal)
			if v, verr := strconv.Atoi(strings.TrimSpace(val)); verr == nil {
				item.Level = v
			}
		case "X":
			wv(terminal, "|07New max level (0=All): ", outputMode)
			val, _ := readLineFromSessionIH(s, terminal)
			if v, verr := strconv.Atoi(strings.TrimSpace(val)); verr == nil {
				item.MaxLevel = v
			}
		case "A":
			wv(terminal, "|07Always display? |15[Y/N]|07: ", outputMode)
			val, _ := readLineFromSessionIH(s, terminal)
			item.Always = strings.ToUpper(strings.TrimSpace(val)) == "Y"
		case "E":
			wv(terminal, "|07Enter new body (blank line to finish):\r\n", outputMode)
			var bodyLines []string
			for {
				wv(terminal, "|07> ", outputMode)
				line, lerr := readLineFromSessionIH(s, terminal)
				if lerr != nil || strings.TrimSpace(line) == "" {
					break
				}
				bodyLines = append(bodyLines, line)
			}
			if len(bodyLines) > 0 {
				item.Body = strings.Join(bodyLines, "\n")
			}
		}
	}
}

func newsViewItem(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	nd *NewsData, outputMode ansi.OutputMode, termWidth, termHeight int) {

	if len(nd.Items) == 0 {
		wv(terminal, "\r\n|07No news items.\r\n", outputMode)
		return
	}
	wv(terminal, fmt.Sprintf("|07View item # |15[|111-%d|15]|07, or |150|07 for all: ", len(nd.Items)), outputMode)
	input, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return
	}
	n, nerr := strconv.Atoi(strings.TrimSpace(input))
	if nerr != nil {
		return
	}
	if n == 0 {
		for i := range nd.Items {
			displayNewsItem(e, terminal, &nd.Items[i], i+1, outputMode)
			e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
		}
	} else if n >= 1 && n <= len(nd.Items) {
		displayNewsItem(e, terminal, &nd.Items[n-1], n, outputMode)
		e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
	}
}
