package menu

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/editor"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

// BBSListing represents a single BBS entry in the directory.
// Modernized from V2's BBSRec (GENTYPES.PAS): dropped baud, replaced phone
// with hostname + separate ports for telnet/SSH, plus web URL.
type BBSListing struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`         // BBS name (V2: Name/Mstr)
	Sysop       string    `json:"sysop"`        // SysOp name
	Address     string    `json:"address"`      // Hostname or IP (V2: Phone)
	TelnetPort  string    `json:"telnet_port"`  // Telnet port (blank = none)
	SSHPort     string    `json:"ssh_port"`     // SSH port (blank = none)
	Web         string    `json:"web"`          // Web URL
	Software    string    `json:"software"`     // BBS software (V2: Ware/Sstr)
	Description string    `json:"description"`  // Extended description
	AddedBy     string    `json:"added_by"`     // Username who added it (V2: Leftby)
	AddedDate   time.Time `json:"added_date"`
	Verified    bool      `json:"verified"`     // SysOp-verified flag
}

// bbsListData holds all BBS listings.
type bbsListData struct {
	Listings []BBSListing `json:"listings"`
	NextID   int          `json:"next_id"`
}

var bbsListMu sync.Mutex

func bbsListFilePath(rootConfigPath string) string {
	return filepath.Join(rootConfigPath, "..", "data", "bbslist.json")
}

func loadBBSListData(rootConfigPath string) (*bbsListData, error) {
	data, err := os.ReadFile(bbsListFilePath(rootConfigPath))
	if err != nil {
		if os.IsNotExist(err) {
			return &bbsListData{NextID: 1}, nil
		}
		return nil, fmt.Errorf("read bbslist.json: %w", err)
	}
	var bld bbsListData
	if err := json.Unmarshal(data, &bld); err != nil {
		return nil, fmt.Errorf("parse bbslist.json: %w", err)
	}
	if bld.NextID < 1 {
		maxID := 0
		for _, l := range bld.Listings {
			if l.ID > maxID {
				maxID = l.ID
			}
		}
		bld.NextID = maxID + 1
	}
	return &bld, nil
}

func saveBBSListData(rootConfigPath string, bld *bbsListData) error {
	data, err := json.MarshalIndent(bld, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal bbslist data: %w", err)
	}
	return os.WriteFile(bbsListFilePath(rootConfigPath), data, 0644)
}

// bbsListSanitize replaces pipe characters in user-supplied strings to prevent
// them from being interpreted as pipe color codes when displayed.
func bbsListSanitize(s string) string {
	return strings.ReplaceAll(s, "|", "\xc2\xa6") // replace | with ¦ (U+00A6)
}

// bbsListConnectionSummary returns a short connection summary for the list view.
func bbsListConnectionSummary(entry *BBSListing) string {
	var parts []string
	if entry.TelnetPort != "" {
		parts = append(parts, "T:"+entry.TelnetPort)
	}
	if entry.SSHPort != "" {
		parts = append(parts, "S:"+entry.SSHPort)
	}
	if entry.Web != "" {
		parts = append(parts, "Web")
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, " ")
}

// runBBSList displays BBS listings in a split-panel lightbar interface.
// Left panel: scrollable BBS name list with highlight bar.
// Right panel: details for the currently selected entry.
func runBBSList(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	log.Printf("DEBUG: Node %d: Running BBSLIST for user %s", nodeNumber, currentUser.Handle)

	// Resolve terminal dimensions.
	if termWidth <= 0 && currentUser != nil {
		termWidth = currentUser.ScreenWidth
	}
	if termWidth <= 0 {
		termWidth = 80
	}
	if termHeight <= 0 && currentUser != nil {
		termHeight = currentUser.ScreenHeight
	}
	if termHeight <= 0 {
		termHeight = 24
	}

	bbsListMu.Lock()
	bld, err := loadBBSListData(e.RootConfigPath)
	bbsListMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading BBS listings.\r\n", outputMode)
		return currentUser, "", nil
	}

	if len(bld.Listings) == 0 {
		wv(terminal, "\r\n|07No BBS listings yet. Be the first to add one!\r\n", outputMode)
		e.holdScreen(s, terminal, outputMode, termWidth, termHeight)
		return currentUser, "", nil
	}

	// Layout constants.
	const headerRows = 2   // title + separator
	const hintRows = 2     // separator + hint line
	leftPanelWidth := 30   // BBS name list width
	if termWidth < 60 {
		leftPanelWidth = termWidth / 2
	}
	rightPanelCol := leftPanelWidth + 1 // 1-indexed column where right panel starts
	rightPanelWidth := termWidth - leftPanelWidth
	listStartRow := headerRows + 1      // first row of BBS list items
	separatorRow := termHeight - 1
	hintRow := termHeight
	visibleRows := separatorRow - listStartRow
	if visibleRows < 3 {
		visibleRows = 3
	}

	hiColorSeq := colorCodeToAnsi(e.Theme.YesNoHighlightColor)

	// --- Render helpers ---

	emit := func(s string) {
		terminalio.WriteProcessedBytes(terminal, []byte(s), outputMode)
	}

	emitPipe := func(s string) {
		terminalio.WriteProcessedBytes(terminal, ansi.ReplacePipeCodes([]byte(s)), outputMode)
	}

	renderHeader := func() {
		emit(ansi.MoveCursor(1, 1) + "\x1b[2K")
		emitPipe("|15BBS Listings Directory")
		emit(ansi.MoveCursor(2, 1) + "\x1b[2K")
		emitPipe("|08" + strings.Repeat("\xc4", termWidth))
	}

	renderFooter := func() {
		emit(ansi.MoveCursor(separatorRow, 1) + "\x1b[2K")
		emitPipe("|08" + strings.Repeat("\xc4", termWidth))
		emit(ansi.MoveCursor(hintRow, 1) + "\x1b[2K")
		emitPipe("|08[ |15Up|08/|15Dn|08 ] Navigate  [ |15PgUp|08/|15PgDn|08 ] Page  [ |15Q|08/|15Esc|08 ] Quit")
	}

	buildLeftLine := func(entry *BBSListing, idx int) string {
		verifiedMark := " "
		if entry.Verified {
			verifiedMark = "*"
		}
		name := bbsListSanitize(entry.Name)
		nameWidth := leftPanelWidth - 5 // "V ## " prefix = 5 chars
		if nameWidth < 5 {
			nameWidth = 5
		}
		if len(name) > nameWidth {
			name = name[:nameWidth]
		}
		return fmt.Sprintf("%s%2d %s", verifiedMark, idx+1, padRight(name, nameWidth))
	}

	renderLeftRow := func(row, topIndex, selectedIndex int) {
		absRow := listStartRow + row
		emit(ansi.MoveCursor(absRow, 1) + "\x1b[2K")
		idx := topIndex + row
		if idx >= len(bld.Listings) {
			return
		}
		line := buildLeftLine(&bld.Listings[idx], idx)
		if idx == selectedIndex {
			rendered := hiColorSeq + padRight(line, leftPanelWidth) + "\x1b[0m"
			emit(rendered)
		} else {
			emitPipe("|07" + padRight(line, leftPanelWidth))
		}
	}

	renderLeftPanel := func(topIndex, selectedIndex int) {
		for row := 0; row < visibleRows; row++ {
			renderLeftRow(row, topIndex, selectedIndex)
		}
	}

	renderDetailField := func(row, col int, label, value, valueColor string, width int) {
		emit(ansi.MoveCursor(row, col))
		val := value
		valWidth := width - 14 // label takes ~14 visible chars
		if valWidth < 5 {
			valWidth = 5
		}
		if len(val) > valWidth {
			val = val[:valWidth]
		}
		emitPipe(fmt.Sprintf("|08\xb3 |15%-10s |08: %s%-*s", bbsListSanitize(label), valueColor, valWidth, bbsListSanitize(val)))
	}

	renderRightPanel := func(selectedIndex int) {
		// Clear right panel area.
		for row := 0; row < visibleRows; row++ {
			absRow := listStartRow + row
			emit(ansi.MoveCursor(absRow, rightPanelCol))
			emit(strings.Repeat(" ", rightPanelWidth))
		}

		if selectedIndex < 0 || selectedIndex >= len(bld.Listings) {
			return
		}
		entry := &bld.Listings[selectedIndex]

		// Vertical separator + detail fields.
		detailRow := listStartRow
		rw := rightPanelWidth - 2 // account for separator + spacing

		renderDetailField(detailRow, rightPanelCol, "BBS Name", entry.Name, "|11", rw)
		detailRow++
		renderDetailField(detailRow, rightPanelCol, "SysOp", entry.Sysop, "|14", rw)
		detailRow++
		renderDetailField(detailRow, rightPanelCol, "Address", entry.Address, "|13", rw)
		detailRow++
		if entry.TelnetPort != "" {
			renderDetailField(detailRow, rightPanelCol, "Telnet", entry.TelnetPort, "|13", rw)
			detailRow++
		}
		if entry.SSHPort != "" {
			renderDetailField(detailRow, rightPanelCol, "SSH", entry.SSHPort, "|13", rw)
			detailRow++
		}
		if entry.Web != "" {
			renderDetailField(detailRow, rightPanelCol, "Web", entry.Web, "|13", rw)
			detailRow++
		}
		renderDetailField(detailRow, rightPanelCol, "Software", entry.Software, "|14", rw)
		detailRow++
		renderDetailField(detailRow, rightPanelCol, "Added By", entry.AddedBy, "|07", rw)
		detailRow++
		renderDetailField(detailRow, rightPanelCol, "Date", entry.AddedDate.Format("01/02/2006"), "|07", rw)
		detailRow++

		verifiedStr := "No"
		verifiedColor := "|04"
		if entry.Verified {
			verifiedStr = "Yes"
			verifiedColor = "|10"
		}
		renderDetailField(detailRow, rightPanelCol, "Verified", verifiedStr, verifiedColor, rw)
		detailRow++

		if entry.Description != "" {
			// Draw separator then description.
			if detailRow < listStartRow+visibleRows {
				emit(ansi.MoveCursor(detailRow, rightPanelCol))
				sepWidth := rightPanelWidth - 2
				if sepWidth < 0 {
					sepWidth = 0
				}
				emitPipe("|08\xb3 " + strings.Repeat("\xc4", sepWidth))
				detailRow++
			}
			for _, line := range strings.Split(entry.Description, "\n") {
				if detailRow >= listStartRow+visibleRows {
					break
				}
				emit(ansi.MoveCursor(detailRow, rightPanelCol))
				descLine := strings.TrimRight(line, "\r")
				maxDesc := rightPanelWidth - 3
				if maxDesc < 1 {
					maxDesc = 1
				}
				if len(descLine) > maxDesc {
					descLine = descLine[:maxDesc]
				}
				emitPipe("|08\xb3 |07" + bbsListSanitize(descLine))
				detailRow++
			}
		}

		// Draw vertical separator line for remaining rows.
		for ; detailRow < listStartRow+visibleRows; detailRow++ {
			emit(ansi.MoveCursor(detailRow, rightPanelCol))
			emitPipe("|08\xb3")
		}
	}

	renderFull := func(topIndex, selectedIndex int) {
		emit(ansi.ClearScreen())
		renderHeader()
		renderLeftPanel(topIndex, selectedIndex)
		renderRightPanel(selectedIndex)
		renderFooter()
	}

	// --- Input loop ---
	ih := getSessionIH(s)
	emit("\x1b[?25l") // hide cursor
	defer func() { emit("\x1b[?25h") }()

	selectedIndex := 0
	topIndex := 0

	clampSelection := func() {
		if len(bld.Listings) == 0 {
			selectedIndex, topIndex = 0, 0
			return
		}
		if selectedIndex < 0 {
			selectedIndex = 0
		}
		if selectedIndex >= len(bld.Listings) {
			selectedIndex = len(bld.Listings) - 1
		}
		if selectedIndex < topIndex {
			topIndex = selectedIndex
		}
		if selectedIndex >= topIndex+visibleRows {
			topIndex = selectedIndex - visibleRows + 1
		}
		if topIndex < 0 {
			topIndex = 0
		}
	}

	prevSelectedIndex := -1
	prevTopIndex := -1
	needFullRedraw := true

	for {
		clampSelection()

		if needFullRedraw {
			renderFull(topIndex, selectedIndex)
			needFullRedraw = false
		} else if topIndex != prevTopIndex {
			// Viewport scrolled — redraw both panels.
			renderLeftPanel(topIndex, selectedIndex)
			renderRightPanel(selectedIndex)
		} else if selectedIndex != prevSelectedIndex {
			// Only selection changed — update old/new left rows + right panel.
			if prevSelectedIndex >= topIndex && prevSelectedIndex < topIndex+visibleRows {
				renderLeftRow(prevSelectedIndex-topIndex, topIndex, selectedIndex)
			}
			if selectedIndex >= topIndex && selectedIndex < topIndex+visibleRows {
				renderLeftRow(selectedIndex-topIndex, topIndex, selectedIndex)
			}
			renderRightPanel(selectedIndex)
		}

		prevSelectedIndex = selectedIndex
		prevTopIndex = topIndex

		keyInt, err := ih.ReadKey()
		if err != nil {
			if errors.Is(err, editor.ErrIdleTimeout) {
				return nil, "LOGOFF", editor.ErrIdleTimeout
			}
			if errors.Is(err, io.EOF) {
				return nil, "LOGOFF", io.EOF
			}
			return nil, "", err
		}

		switch keyInt {
		case editor.KeyArrowUp:
			selectedIndex--
		case editor.KeyArrowDown:
			selectedIndex++
		case editor.KeyPageUp, editor.KeyCtrlR:
			selectedIndex -= visibleRows
			topIndex -= visibleRows
			if topIndex < 0 {
				topIndex = 0
			}
		case editor.KeyPageDown, editor.KeyCtrlC:
			selectedIndex += visibleRows
			topIndex += visibleRows
		case editor.KeyHome:
			selectedIndex = 0
		case editor.KeyEnd:
			if len(bld.Listings) > 0 {
				selectedIndex = len(bld.Listings) - 1
			}
		case editor.KeyEsc:
			return currentUser, "", nil
		default:
			if keyInt >= 32 && keyInt < 127 {
				ch := rune(keyInt)
				if ch == 'q' || ch == 'Q' {
					return currentUser, "", nil
				}
			}
		}
	}
}

// runBBSListAdd prompts the user to add a new BBS listing.
// Maps to V2's AddBBS procedure (BBSLIST.PAS lines 68-113).
func runBBSListAdd(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	log.Printf("DEBUG: Node %d: Running BBSLISTADD for user %s", nodeNumber, currentUser.Handle)

	wv(terminal, "\r\n|15Add BBS Listing\r\n", outputMode)
	wv(terminal, "|08"+strings.Repeat("\xc4", 40)+"\r\n", outputMode)

	// BBS Name (required)
	wv(terminal, "|08B|07B|15S |13Name|05: ", outputMode)
	name, err := readLineFromSessionIH(s, terminal)
	if err != nil || strings.TrimSpace(name) == "" {
		wv(terminal, "\r\n|07Aborted.\r\n", outputMode)
		return currentUser, "", nil
	}
	name = strings.TrimSpace(name)
	if len(name) > 40 {
		name = name[:40]
	}

	// Address (required)
	wv(terminal, "|08A|07d|15dress |08(|07hostname or IP|08)|05: ", outputMode)
	address, err := readLineFromSessionIH(s, terminal)
	if err != nil || strings.TrimSpace(address) == "" {
		wv(terminal, "\r\n|07Aborted.\r\n", outputMode)
		return currentUser, "", nil
	}
	address = strings.TrimSpace(address)
	if len(address) > 60 {
		address = address[:60]
	}

	// Telnet port (optional)
	wv(terminal, "|08T|07e|15lnet |13Port |08(|07blank if none|08)|05: ", outputMode)
	telnetPort, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return currentUser, "", nil
	}
	telnetPort = strings.TrimSpace(telnetPort)
	if len(telnetPort) > 10 {
		telnetPort = telnetPort[:10]
	}

	// SSH port (optional)
	wv(terminal, "|08S|07S|15H   |13Port |08(|07blank if none|08)|05: ", outputMode)
	sshPort, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return currentUser, "", nil
	}
	sshPort = strings.TrimSpace(sshPort)
	if len(sshPort) > 10 {
		sshPort = sshPort[:10]
	}

	// Web URL (optional)
	wv(terminal, "|08W|07e|15b   |08(|07URL, or blank|08)|05: ", outputMode)
	webAddr, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return currentUser, "", nil
	}
	webAddr = strings.TrimSpace(webAddr)
	if len(webAddr) > 80 {
		webAddr = webAddr[:80]
	}

	// SysOp name (optional)
	wv(terminal, "|08S|07y|15sOp|05: ", outputMode)
	sysop, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return currentUser, "", nil
	}
	sysop = strings.TrimSpace(sysop)
	if len(sysop) > 30 {
		sysop = sysop[:30]
	}

	// Software (optional, default ViSiON/3)
	wv(terminal, "|08B|07B|15S |13Software |08[|07ViSiON/3|08]|05: ", outputMode)
	software, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return currentUser, "", nil
	}
	software = strings.TrimSpace(software)
	if software == "" {
		software = "ViSiON/3"
	}
	if len(software) > 20 {
		software = software[:20]
	}

	// Description (optional)
	wv(terminal, "|08D|07e|15scription |08(|07one line, or blank|08)|05: ", outputMode)
	desc, err := readLineFromSessionIH(s, terminal)
	if err != nil {
		return currentUser, "", nil
	}
	desc = strings.TrimSpace(desc)
	if len(desc) > 200 {
		desc = desc[:200]
	}

	// Save
	bbsListMu.Lock()
	bld, err := loadBBSListData(e.RootConfigPath)
	if err != nil {
		bbsListMu.Unlock()
		wv(terminal, "\r\n|04Error loading BBS list data.\r\n", outputMode)
		return currentUser, "", nil
	}

	entry := BBSListing{
		ID:          bld.NextID,
		Name:        name,
		Sysop:       sysop,
		Address:     address,
		TelnetPort:  telnetPort,
		SSHPort:     sshPort,
		Web:         webAddr,
		Software:    software,
		Description: desc,
		AddedBy:     currentUser.Handle,
		AddedDate:   time.Now(),
		Verified:    false,
	}
	bld.Listings = append(bld.Listings, entry)
	bld.NextID++

	if err := saveBBSListData(e.RootConfigPath, bld); err != nil {
		bbsListMu.Unlock()
		wv(terminal, "\r\n|04Error saving BBS listing.\r\n", outputMode)
		return currentUser, "", nil
	}
	bbsListMu.Unlock()

	wv(terminal, "\r\n|10Your entry has been added!\r\n", outputMode)
	log.Printf("INFO: Node %d: User %s added BBS listing: %s", nodeNumber, currentUser.Handle, name)
	return currentUser, "", nil
}

// runBBSListEdit allows a user to edit their own BBS listing (or any if sysop).
// Maps to V2's ChangeBBS procedure with ownership check.
func runBBSListEdit(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	bbsListMu.Lock()
	bld, err := loadBBSListData(e.RootConfigPath)
	bbsListMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading BBS listings.\r\n", outputMode)
		return currentUser, "", nil
	}

	if len(bld.Listings) == 0 {
		wv(terminal, "\r\n|07No BBS listings to edit.\r\n", outputMode)
		return currentUser, "", nil
	}

	wv(terminal, fmt.Sprintf("\r\n|07Edit which entry |15[|111-%d|15]|07? |08(|07? to list|08)|05: ", len(bld.Listings)), outputMode)
	input, err := readLineFromSessionIH(s, terminal)
	if err != nil || strings.TrimSpace(input) == "" {
		return currentUser, "", nil
	}
	input = strings.TrimSpace(input)

	// Allow ? to show list
	if input == "?" {
		bbsListQuickList(terminal, bld, outputMode)
		wv(terminal, fmt.Sprintf("\r\n|07Edit which entry |15[|111-%d|15]|07? ", len(bld.Listings)), outputMode)
		input, err = readLineFromSessionIH(s, terminal)
		if err != nil || strings.TrimSpace(input) == "" {
			return currentUser, "", nil
		}
		input = strings.TrimSpace(input)
	}

	n, nerr := strconv.Atoi(input)
	if nerr != nil || n < 1 || n > len(bld.Listings) {
		wv(terminal, "|07Invalid selection.\r\n", outputMode)
		return currentUser, "", nil
	}

	idx := n - 1
	entry := &bld.Listings[idx]

	// V2 ownership check: user must own entry or be sysop
	if !strings.EqualFold(entry.AddedBy, currentUser.Handle) && !e.isCoSysOpOrAbove(currentUser) {
		wv(terminal, "\r\n|04You can only edit your own listings.\r\n", outputMode)
		return currentUser, "", nil
	}

	showEditFields := func() {
		wv(terminal, fmt.Sprintf("\r\n|15Editing: |11%s\r\n", bbsListSanitize(entry.Name)), outputMode)
		wv(terminal, "|08"+strings.Repeat("\xc4", 50)+"\r\n", outputMode)
		wv(terminal, "|15[|111|15] |07Name        |08: |11"+bbsListSanitize(entry.Name)+"\r\n", outputMode)
		wv(terminal, "|15[|112|15] |07Address     |08: |11"+bbsListSanitize(entry.Address)+"\r\n", outputMode)
		wv(terminal, "|15[|113|15] |07Telnet Port |08: |11"+bbsListSanitize(entry.TelnetPort)+"\r\n", outputMode)
		wv(terminal, "|15[|114|15] |07SSH Port    |08: |11"+bbsListSanitize(entry.SSHPort)+"\r\n", outputMode)
		wv(terminal, "|15[|115|15] |07Web         |08: |11"+bbsListSanitize(entry.Web)+"\r\n", outputMode)
		wv(terminal, "|15[|116|15] |07SysOp       |08: |11"+bbsListSanitize(entry.Sysop)+"\r\n", outputMode)
		wv(terminal, "|15[|117|15] |07Software    |08: |11"+bbsListSanitize(entry.Software)+"\r\n", outputMode)
		wv(terminal, "|15[|118|15] |07Description |08: |11"+bbsListSanitize(entry.Description)+"\r\n", outputMode)
	}

	showEditFields()

	for {
		wv(terminal, "\r\n|07Edit field |15[|111-8|15]|07, or |15Q|07 to save & quit: ", outputMode)
		choice, err := readLineFromSessionIH(s, terminal)
		if err != nil {
			return currentUser, "", nil
		}
		choice = strings.TrimSpace(strings.ToUpper(choice))

		if choice == "Q" || choice == "" {
			break
		}

		var fieldPrompt string
		switch choice {
		case "1":
			fieldPrompt = "|07New Name: "
		case "2":
			fieldPrompt = "|07New Address: "
		case "3":
			fieldPrompt = "|07New Telnet Port: "
		case "4":
			fieldPrompt = "|07New SSH Port: "
		case "5":
			fieldPrompt = "|07New Web URL: "
		case "6":
			fieldPrompt = "|07New SysOp: "
		case "7":
			fieldPrompt = "|07New Software: "
		case "8":
			fieldPrompt = "|07New Description: "
		default:
			wv(terminal, "|07Invalid choice.\r\n", outputMode)
			continue
		}

		wv(terminal, fieldPrompt, outputMode)
		val, err := readLineFromSessionIH(s, terminal)
		if err != nil {
			return currentUser, "", nil
		}
		val = strings.TrimSpace(val)
		if val == "" {
			continue
		}

		switch choice {
		case "1":
			if len(val) > 40 {
				val = val[:40]
			}
			entry.Name = val
		case "2":
			if len(val) > 60 {
				val = val[:60]
			}
			entry.Address = val
		case "3":
			if len(val) > 10 {
				val = val[:10]
			}
			entry.TelnetPort = val
		case "4":
			if len(val) > 10 {
				val = val[:10]
			}
			entry.SSHPort = val
		case "5":
			if len(val) > 80 {
				val = val[:80]
			}
			entry.Web = val
		case "6":
			if len(val) > 30 {
				val = val[:30]
			}
			entry.Sysop = val
		case "7":
			if len(val) > 20 {
				val = val[:20]
			}
			entry.Software = val
		case "8":
			if len(val) > 200 {
				val = val[:200]
			}
			entry.Description = val
		}
	}

	// Save changes
	bbsListMu.Lock()
	if err := saveBBSListData(e.RootConfigPath, bld); err != nil {
		bbsListMu.Unlock()
		wv(terminal, "\r\n|04Error saving changes.\r\n", outputMode)
		return currentUser, "", nil
	}
	bbsListMu.Unlock()

	wv(terminal, "\r\n|10Entry updated!\r\n", outputMode)
	log.Printf("INFO: Node %d: User %s edited BBS listing #%d (%s)", nodeNumber, currentUser.Handle, n, entry.Name)
	return currentUser, "", nil
}

// runBBSListDelete allows a user to delete their own BBS listing (or any if sysop).
// Maps to V2's Deletebbs procedure with ownership check.
func runBBSListDelete(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	bbsListMu.Lock()
	bld, err := loadBBSListData(e.RootConfigPath)
	bbsListMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading BBS listings.\r\n", outputMode)
		return currentUser, "", nil
	}

	if len(bld.Listings) == 0 {
		wv(terminal, "\r\n|07No BBS listings to delete.\r\n", outputMode)
		return currentUser, "", nil
	}

	wv(terminal, fmt.Sprintf("\r\n|07Delete which entry |15[|111-%d|15]|07? |08(|07? to list|08)|05: ", len(bld.Listings)), outputMode)
	input, err := readLineFromSessionIH(s, terminal)
	if err != nil || strings.TrimSpace(input) == "" {
		return currentUser, "", nil
	}
	input = strings.TrimSpace(input)

	if input == "?" {
		bbsListQuickList(terminal, bld, outputMode)
		wv(terminal, fmt.Sprintf("\r\n|07Delete which entry |15[|111-%d|15]|07? ", len(bld.Listings)), outputMode)
		input, err = readLineFromSessionIH(s, terminal)
		if err != nil || strings.TrimSpace(input) == "" {
			return currentUser, "", nil
		}
		input = strings.TrimSpace(input)
	}

	n, nerr := strconv.Atoi(input)
	if nerr != nil || n < 1 || n > len(bld.Listings) {
		wv(terminal, "|07Invalid selection.\r\n", outputMode)
		return currentUser, "", nil
	}

	idx := n - 1
	entry := bld.Listings[idx]

	// V2 ownership check
	if !strings.EqualFold(entry.AddedBy, currentUser.Handle) && !e.isCoSysOpOrAbove(currentUser) {
		wv(terminal, "\r\n|04You can only delete your own listings.\r\n", outputMode)
		return currentUser, "", nil
	}

	// Confirm deletion
	confirm, err := e.PromptYesNo(s, terminal,
		fmt.Sprintf("\r\n|07Delete |15%s|07? ", entry.Name),
		outputMode, nodeNumber, termWidth, termHeight, false)
	if err != nil || !confirm {
		wv(terminal, "\r\n|07Cancelled.\r\n", outputMode)
		return currentUser, "", nil
	}

	// Remove entry (compact like V2's shift-down)
	bbsListMu.Lock()
	bld.Listings = append(bld.Listings[:idx], bld.Listings[idx+1:]...)
	if err := saveBBSListData(e.RootConfigPath, bld); err != nil {
		bbsListMu.Unlock()
		wv(terminal, "\r\n|04Error saving changes.\r\n", outputMode)
		return currentUser, "", nil
	}
	bbsListMu.Unlock()

	wv(terminal, "\r\n|10Entry deleted.\r\n", outputMode)
	log.Printf("INFO: Node %d: User %s deleted BBS listing: %s", nodeNumber, currentUser.Handle, entry.Name)
	return currentUser, "", nil
}

// runBBSListVerify allows a sysop to toggle the verified flag on a listing.
func runBBSListVerify(e *MenuExecutor, s ssh.Session, terminal *term.Terminal,
	userManager *user.UserMgr, currentUser *user.User, nodeNumber int,
	sessionStartTime time.Time, args string, outputMode ansi.OutputMode,
	termWidth int, termHeight int) (*user.User, string, error) {

	if currentUser == nil {
		return currentUser, "", nil
	}

	if !e.isCoSysOpOrAbove(currentUser) {
		wv(terminal, "\r\n|04SysOp access required.\r\n", outputMode)
		return currentUser, "", nil
	}

	bbsListMu.Lock()
	bld, err := loadBBSListData(e.RootConfigPath)
	bbsListMu.Unlock()
	if err != nil {
		wv(terminal, "\r\n|04Error loading BBS listings.\r\n", outputMode)
		return currentUser, "", nil
	}

	if len(bld.Listings) == 0 {
		wv(terminal, "\r\n|07No BBS listings.\r\n", outputMode)
		return currentUser, "", nil
	}

	bbsListQuickList(terminal, bld, outputMode)
	wv(terminal, fmt.Sprintf("\r\n|07Toggle verified on entry |15[|111-%d|15]|07? ", len(bld.Listings)), outputMode)
	input, err := readLineFromSessionIH(s, terminal)
	if err != nil || strings.TrimSpace(input) == "" {
		return currentUser, "", nil
	}

	n, nerr := strconv.Atoi(strings.TrimSpace(input))
	if nerr != nil || n < 1 || n > len(bld.Listings) {
		wv(terminal, "|07Invalid selection.\r\n", outputMode)
		return currentUser, "", nil
	}

	idx := n - 1
	bbsListMu.Lock()
	bld.Listings[idx].Verified = !bld.Listings[idx].Verified
	status := "unverified"
	if bld.Listings[idx].Verified {
		status = "verified"
	}
	if err := saveBBSListData(e.RootConfigPath, bld); err != nil {
		bbsListMu.Unlock()
		wv(terminal, "\r\n|04Error saving changes.\r\n", outputMode)
		return currentUser, "", nil
	}
	bbsListMu.Unlock()

	wv(terminal, fmt.Sprintf("\r\n|10%s is now %s.\r\n", bld.Listings[idx].Name, status), outputMode)
	return currentUser, "", nil
}

// bbsListQuickList shows a compact numbered list of BBS entries.
func bbsListQuickList(terminal *term.Terminal, bld *bbsListData, outputMode ansi.OutputMode) {
	wv(terminal, "\r\n", outputMode)
	for i, entry := range bld.Listings {
		wv(terminal, fmt.Sprintf("  |11%2d|07. |15%s |08(|07%s|08)\r\n", i+1, bbsListSanitize(entry.Name), bbsListSanitize(entry.Address)), outputMode)
	}
}
