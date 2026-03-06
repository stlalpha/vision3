package menu

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// NewsItem represents a single news item (maps to V2's newsrec).
type NewsItem struct {
	ID       int       `json:"id"`
	Title    string    `json:"title"`    // max 28 chars (V2 String[28])
	From     string    `json:"from"`     // author handle
	When     time.Time `json:"when"`
	Level    int       `json:"level"`    // min access level
	MaxLevel int       `json:"max_level"` // max access level (0 = no max / all)
	Always   bool      `json:"always"`   // true = show every login; false = once (new since last login)
	Body     string    `json:"body"`     // news text body
}

// NewsData holds all news items.
type NewsData struct {
	Items []NewsItem `json:"items"`
}

var newsMu sync.Mutex

func newsFilePath(rootConfigPath string) string {
	return filepath.Join(rootConfigPath, "..", "data", "news.json")
}

func loadNewsData(rootConfigPath string) (*NewsData, error) {
	data, err := os.ReadFile(newsFilePath(rootConfigPath))
	if err != nil {
		if os.IsNotExist(err) {
			return &NewsData{}, nil
		}
		return nil, fmt.Errorf("read news.json: %w", err)
	}
	var nd NewsData
	if err := json.Unmarshal(data, &nd); err != nil {
		return nil, fmt.Errorf("parse news.json: %w", err)
	}
	return &nd, nil
}

func saveNewsData(rootConfigPath string, nd *NewsData) error {
	data, err := json.MarshalIndent(nd, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal news data: %w", err)
	}
	return os.WriteFile(newsFilePath(rootConfigPath), data, 0644)
}

// displayNewsItem renders NEWSHDR.ANS with substitution vars, then the body text.
// Substitution vars (V2-compatible mapping):
//   ^NM = item number   ^TI = title    ^FR = from/author
//   ^DT = date          ^TM = time     ^LV = min level   ^MX = max level
func displayNewsItem(e *MenuExecutor, terminal *term.Terminal, item *NewsItem, idx int, outputMode ansi.OutputMode) {
	ansiPath := filepath.Join(e.MenuSetPath, "ansi", "NEWSHDR.ANS")
	if raw, err := os.ReadFile(ansiPath); err == nil {
		maxStr := strconv.Itoa(item.MaxLevel)
		if item.MaxLevel <= 0 {
			maxStr = "All"
		}
		hdr := string(raw)
		hdr = strings.ReplaceAll(hdr, "^NM", strconv.Itoa(idx))
		hdr = strings.ReplaceAll(hdr, "^TI", item.Title)
		hdr = strings.ReplaceAll(hdr, "^FR", item.From)
		hdr = strings.ReplaceAll(hdr, "^DT", item.When.Format("01/02/2006"))
		hdr = strings.ReplaceAll(hdr, "^TM", item.When.Format("3:04 pm"))
		hdr = strings.ReplaceAll(hdr, "^LV", strconv.Itoa(item.Level))
		hdr = strings.ReplaceAll(hdr, "^MX", maxStr)
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(hdr)), outputMode)
	} else {
		// Fallback plain header if NEWSHDR.ANS is missing
		wv(terminal, fmt.Sprintf("\r\n|15News #%d: |11%s\r\n|07From: |11%s |07  Date: |11%s\r\n|08%s\r\n",
			idx, item.Title, item.From, item.When.Format("01/02/2006"),
			strings.Repeat("\xc4", 70)), outputMode)
	}
	if item.Body != "" {
		for _, line := range strings.Split(item.Body, "\n") {
			wv(terminal, strings.TrimRight(line, "\r")+"\r\n", outputMode)
		}
	}
}

// runPrintNews displays news items new since last login (or Always-flagged items).
// Maps to V2's PrintNews(0, True) called at login.
func runPrintNews(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	newsMu.Lock()
	nd, err := loadNewsData(e.RootConfigPath)
	newsMu.Unlock()
	if err != nil {
		log.Printf("WARN: Node %d: Failed to load news data: %v", nodeNumber, err)
		return currentUser, "", nil
	}

	lastLogin := currentUser.LastLogin
	userLevel := currentUser.AccessLevel
	shown := 0

	for i, item := range nd.Items {
		if userLevel < item.Level {
			continue
		}
		if item.MaxLevel > 0 && userLevel > item.MaxLevel {
			continue
		}
		// V2 logic: show if Always, or newer than last login, or no prior login
		if !item.Always && !lastLogin.IsZero() && !item.When.After(lastLogin) {
			continue
		}
		displayNewsItem(e, terminal, &nd.Items[i], i+1, outputMode)
		e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
		shown++
	}

	if shown > 0 {
		log.Printf("DEBUG: Node %d: Displayed %d news item(s) to %s", nodeNumber, shown, currentUser.Handle)
	}
	return currentUser, "", nil
}

// runListNews presents all visible news items in a list and lets users read them.
// Maps to V2's PrintNews(0, False) — show all regardless of date.
func runListNews(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	log.Printf("DEBUG: Node %d: Running LISTNEWS for user %s", nodeNumber, currentUser.Handle)

	newsMu.Lock()
	nd, err := loadNewsData(e.RootConfigPath)
	newsMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading news.\r\n", outputMode)
		return currentUser, "", nil
	}

	userLevel := currentUser.AccessLevel
	var visible []int
	for i, item := range nd.Items {
		if userLevel < item.Level {
			continue
		}
		if item.MaxLevel > 0 && userLevel > item.MaxLevel {
			continue
		}
		visible = append(visible, i)
	}

	if len(visible) == 0 {
		wv(terminal, "\r\n|07No news available.\r\n", outputMode)
		return currentUser, "", nil
	}

	showList := func() {
		terminalio.WriteProcessedBytes(terminal, []byte("\x1b[2J\x1b[H"), outputMode)
		wv(terminal, "\r\n|15System News\r\n|08"+strings.Repeat("\xc4", 70)+"\r\n", outputMode)
		for rank, idx := range visible {
			item := nd.Items[idx]
			newTag := "      "
			if item.When.After(currentUser.LastLogin) {
				newTag = "|12[NEW]|07"
			}
			wv(terminal, fmt.Sprintf("  |11%2d|07. |15%-28s |07%s |11%s\r\n",
				rank+1, item.Title, newTag, item.When.Format("01/02/06")), outputMode)
		}
		wv(terminal, "|08"+strings.Repeat("\xc4", 70)+"\r\n", outputMode)
	}

	showList()
	for {
		prompt := fmt.Sprintf("|07Read which item |15[|111-%d|15]|07, or |15ENTER|07 to continue: ", len(visible))
		wv(terminal, prompt, outputMode)
		input, err := readLineFromSessionIH(s, terminal)
		if err != nil || strings.TrimSpace(input) == "" {
			return currentUser, "", nil
		}
		n, nerr := strconv.Atoi(strings.TrimSpace(input))
		if nerr != nil || n < 1 || n > len(visible) {
			wv(terminal, "\r\n|07Invalid selection.\r\n", outputMode)
			continue
		}
		idx := visible[n-1]
		displayNewsItem(e, terminal, &nd.Items[idx], n, outputMode)
		e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
		showList()
	}
}
