package usereditor

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/crypto/bcrypt"

	"github.com/stlalpha/vision3/internal/user"
)

const (
	listVisible = 13 // Number of users visible in list
	minWidth    = 80
	minHeight   = 25
)

// editorMode represents the current interaction state.
type editorMode int

const (
	modeList          editorMode = iota // Main list browser
	modeEdit                           // Per-user field editor
	modeEditField                      // Actively editing a field value
	modeSearch                         // Search for user by handle
	modeDeleteConfirm                  // Confirm single user delete
	modeMassDelete                     // Confirm mass delete of tagged
	modeValidate                       // Confirm auto-validate
	modeMassValidate                   // Confirm mass validate
	modeHelp                           // Help screen overlay
	modeFileChanged                    // File modified externally warning
	modeExitConfirm                    // Unsaved changes exit confirm
	modePasswordEntry                  // Password entry for reset
	modeSaveConfirm                    // Confirm save before exit
)

// Model is the BubbleTea model for the user editor TUI.
type Model struct {
	// Data
	users     []*user.User // All users (sorted by ID)
	origUsers []*user.User // Snapshot at load time (for dirty tracking)
	filePath  string
	fileMtime time.Time // mtime at load for optimistic concurrency
	dirty     bool

	// List mode state
	cursor       int          // Current position in user list (0-based)
	scrollOffset int          // First visible row in the list
	listType     int          // Column view mode (1-5)
	listAlpha    bool         // Alphabetical sort active
	tagged       map[int]bool // Tagged user indices (0-based)

	// Edit mode state
	editIndex int        // Index into users slice being edited
	editField int        // Current field index (0-based)
	fields    []fieldDef // Field definitions

	// Text input (shared for editing fields, search, password)
	textInput textinput.Model

	// Search
	searchInput textinput.Model

	// Confirm dialog
	confirmYes bool

	// Terminal
	width   int
	height  int
	mode    editorMode
	message string // Flash message
}

// New creates a new user editor model.
func New(filePath string) (Model, error) {
	users, mtime, err := LoadUsers(filePath)
	if err != nil {
		return Model{}, fmt.Errorf("loading users: %w", err)
	}

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 80
	ti.Width = 40

	si := textinput.New()
	si.Placeholder = "Search handle..."
	si.CharLimit = 30
	si.Width = 25

	// Snapshot original users for dirty tracking
	origUsers := make([]*user.User, len(users))
	for i, u := range users {
		origUsers[i] = CloneUser(u)
	}

	return Model{
		users:     users,
		origUsers: origUsers,
		filePath:  filePath,
		fileMtime: mtime,
		cursor:    0,
		listType:  1,
		tagged:    make(map[int]bool),
		fields:    editFields(),
		textInput: ti,
		searchInput: si,
		width:     minWidth,
		height:    minHeight,
		mode:      modeList,
	}, nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.SetWindowTitle("ViSiON/3 User Editor")
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.width < minWidth {
			m.width = minWidth
		}
		if m.height < minHeight {
			m.height = minHeight
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeEdit:
			return m.updateEdit(msg)
		case modeEditField:
			return m.updateEditField(msg)
		case modeSearch:
			return m.updateSearch(msg)
		case modeDeleteConfirm, modeMassDelete, modeValidate, modeMassValidate,
			modeExitConfirm, modeFileChanged, modeSaveConfirm:
			return m.updateConfirm(msg)
		case modeHelp:
			return m.updateHelp(msg)
		case modePasswordEntry:
			return m.updatePassword(msg)
		}
	}
	return m, nil
}

// --- List Mode ---

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.users)
	if total == 0 {
		if msg.Type == tea.KeyEscape {
			return m, tea.Quit
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
		}
	case tea.KeyDown:
		if m.cursor < total-1 {
			m.cursor++
		}
	case tea.KeyHome:
		m.cursor = 0
	case tea.KeyEnd:
		m.cursor = total - 1
	case tea.KeyPgUp:
		m.cursor -= listVisible
		if m.cursor < 0 {
			m.cursor = 0
		}
	case tea.KeyPgDown:
		m.cursor += listVisible
		if m.cursor >= total {
			m.cursor = total - 1
		}
	case tea.KeyEnter:
		// Open edit screen for highlighted user
		m.editIndex = m.cursor
		m.editField = 0
		m.mode = modeEdit
		return m, nil
	case tea.KeyEscape:
		if m.dirty {
			m.mode = modeExitConfirm
			m.confirmYes = false
			return m, nil
		}
		return m, tea.Quit
	case tea.KeyF2:
		// Delete highlighted user
		m.mode = modeDeleteConfirm
		m.confirmYes = false
		return m, nil
	case tea.KeyF3:
		// Toggle alphabetical sort
		m.toggleSort()
		return m, nil
	case tea.KeyF5:
		// Auto-validate highlighted user
		m.mode = modeValidate
		m.confirmYes = false
		return m, nil
	case tea.KeyF10:
		// Tag all users
		for i := range m.users {
			m.tagged[i] = true
		}
		return m, nil
	case tea.KeySpace:
		// Toggle tag
		m.tagged[m.cursor] = !m.tagged[m.cursor]
		if m.cursor < total-1 {
			m.cursor++
		}
		m.clampScroll()
		return m, nil
	default:
		switch msg.String() {
		case "left":
			if m.listType > 1 {
				m.listType--
			}
		case "right":
			if m.listType < 5 {
				m.listType++
			}
		case "shift+f2":
			// Mass delete tagged
			tagCount := m.taggedCount()
			if tagCount == 0 {
				m.message = "You have not tagged anyone to Delete!"
				return m, nil
			}
			m.mode = modeMassDelete
			m.confirmYes = false
			return m, nil
		case "shift+f5":
			// Mass validate tagged
			tagCount := m.taggedCount()
			if tagCount == 0 {
				m.message = "You have not tagged anyone to Quick-Validate!"
				return m, nil
			}
			m.mode = modeMassValidate
			m.confirmYes = false
			return m, nil
		case "shift+f10":
			// Untag all
			m.tagged = make(map[int]bool)
			return m, nil
		case "alt+h":
			m.mode = modeHelp
			return m, nil
		}
	}
	m.clampScroll()
	return m, nil
}

// --- Edit Mode (field navigation) ---

func (m Model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab, tea.KeyEnter:
		f := m.fields[m.editField]
		if f.Type == ftAction {
			// Password field - enter password entry mode
			m.mode = modePasswordEntry
			m.textInput.SetValue("")
			m.textInput.Placeholder = "New password..."
			m.textInput.EchoMode = textinput.EchoPassword
			m.textInput.Focus()
			return m, textinput.Blink
		}
		if f.Type == ftDisplay {
			// Skip display-only fields
			m.editField = m.nextEditableField(1)
			return m, nil
		}
		// Start editing the current field
		return m.startFieldEdit()

	case tea.KeyDown:
		m.editField = m.nextEditableField(1)

	case tea.KeyUp:
		m.editField = m.nextEditableField(-1)

	case tea.KeyEscape:
		// Save and return to list
		m.saveCurrentUser()
		m.mode = modeList
		return m, nil

	case tea.KeyPgDown:
		// Save current, go to next user
		m.saveCurrentUser()
		m.editIndex++
		if m.editIndex >= len(m.users) {
			m.editIndex = 0
		}
		m.editField = 0
		return m, nil

	case tea.KeyPgUp:
		// Save current, go to previous user
		m.saveCurrentUser()
		m.editIndex--
		if m.editIndex < 0 {
			m.editIndex = len(m.users) - 1
		}
		m.editField = 0
		return m, nil

	case tea.KeyF2:
		// Delete current user
		m.mode = modeDeleteConfirm
		m.confirmYes = false
		return m, nil

	case tea.KeyF5:
		// Set defaults
		m.mode = modeValidate
		m.confirmYes = false
		return m, nil

	case tea.KeyF10:
		// Abort - discard changes for this user
		m.mode = modeList
		return m, nil

	default:
		switch msg.String() {
		case "ctrl+home":
			m.editField = m.firstEditableField()
		case "ctrl+end":
			m.editField = m.lastEditableField()
		}
	}
	return m, nil
}

// nextEditableField finds the next non-display field in the given direction (+1 or -1).
func (m Model) nextEditableField(dir int) int {
	n := len(m.fields)
	idx := m.editField
	for i := 0; i < n; i++ {
		idx += dir
		if idx > n-1 {
			idx = 0
		} else if idx < 0 {
			idx = n - 1
		}
		if m.fields[idx].Type != ftDisplay {
			return idx
		}
	}
	return m.editField // fallback (all display, shouldn't happen)
}

// firstEditableField returns the index of the first non-display field.
func (m Model) firstEditableField() int {
	for i, f := range m.fields {
		if f.Type != ftDisplay {
			return i
		}
	}
	return 0
}

// lastEditableField returns the index of the last non-display field.
func (m Model) lastEditableField() int {
	for i := len(m.fields) - 1; i >= 0; i-- {
		if m.fields[i].Type != ftDisplay {
			return i
		}
	}
	return len(m.fields) - 1
}

// startFieldEdit enters field editing mode for the current field.
func (m Model) startFieldEdit() (Model, tea.Cmd) {
	f := m.fields[m.editField]
	if f.Type == ftDisplay {
		return m, nil
	}

	u := m.users[m.editIndex]
	val := f.Get(u)

	m.mode = modeEditField
	m.textInput.SetValue(val)
	m.textInput.CharLimit = f.Width
	m.textInput.Width = f.Width
	m.textInput.EchoMode = textinput.EchoNormal
	m.textInput.Placeholder = ""
	m.textInput.CursorEnd()
	m.textInput.Focus()

	return m, textinput.Blink
}

// --- Field Editing Mode ---

func (m Model) updateEditField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.fields[m.editField]

	switch msg.Type {
	case tea.KeyEnter, tea.KeyTab, tea.KeyDown:
		// Confirm and move to next field
		if err := m.applyFieldValue(f); err != nil {
			m.message = fmt.Sprintf("Invalid: %v", err)
			return m, nil
		}
		m.textInput.Blur()
		m.mode = modeEdit
		m.editField = m.nextEditableField(1)
		return m, nil

	case tea.KeyUp:
		// Confirm and move to previous field
		if err := m.applyFieldValue(f); err != nil {
			m.message = fmt.Sprintf("Invalid: %v", err)
			return m, nil
		}
		m.textInput.Blur()
		m.mode = modeEdit
		m.editField = m.nextEditableField(-1)
		return m, nil

	case tea.KeyEscape:
		// Cancel edit, don't apply
		m.textInput.Blur()
		m.mode = modeEdit
		return m, nil

	default:
		// For Y/N fields, only accept Y, N, y, n
		if f.Type == ftYesNo {
			if len(msg.Runes) == 1 {
				ch := msg.Runes[0]
				if ch == 'y' || ch == 'Y' {
					m.textInput.SetValue("Y")
				} else if ch == 'n' || ch == 'N' {
					m.textInput.SetValue("N")
				}
				// Auto-confirm Y/N
				if err := m.applyFieldValue(f); err == nil {
					m.textInput.Blur()
					m.mode = modeEdit
					m.editField = m.nextEditableField(1)
				}
				return m, nil
			}
			return m, nil
		}

		// For integer fields, filter non-numeric input
		if f.Type == ftInteger {
			if len(msg.Runes) == 1 {
				ch := msg.Runes[0]
				if ch < '0' || ch > '9' {
					if ch != '-' {
						return m, nil // Reject non-numeric
					}
				}
			}
		}

		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

// applyFieldValue validates and applies the current text input value to the user field.
func (m *Model) applyFieldValue(f fieldDef) error {
	val := m.textInput.Value()
	u := m.users[m.editIndex]

	switch f.Type {
	case ftInteger:
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("not a number")
		}
		if n < f.Min || n > f.Max {
			return fmt.Errorf("must be %d-%d", f.Min, f.Max)
		}
	case ftYesNo:
		upper := strings.ToUpper(val)
		if upper != "Y" && upper != "N" {
			return fmt.Errorf("must be Y or N")
		}
		val = upper
	}

	if f.Set != nil {
		if err := f.Set(u, val); err != nil {
			return err
		}
		u.UpdatedAt = time.Now()
		m.dirty = true
		m.message = ""
	}
	return nil
}

// --- Search Mode ---

func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		query := strings.ToLower(m.searchInput.Value())
		if query != "" {
			// Search from current position forward, wrapping
			for offset := 0; offset < len(m.users); offset++ {
				idx := (m.cursor + offset + 1) % len(m.users)
				u := m.users[idx]
				if strings.Contains(strings.ToLower(u.Handle), query) ||
					strings.Contains(strings.ToLower(u.Username), query) {
					m.cursor = idx
					m.message = fmt.Sprintf("Found: %s", u.Handle)
					break
				}
			}
		}
		m.clampScroll()
		m.mode = modeList
		m.searchInput.Blur()
		return m, nil

	case tea.KeyEscape:
		m.mode = modeList
		m.searchInput.Blur()
		return m, nil

	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}
}

// --- Password Entry ---

func (m Model) updatePassword(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		pw := m.textInput.Value()
		if pw != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
			if err != nil {
				m.message = fmt.Sprintf("Hash error: %v", err)
			} else {
				m.users[m.editIndex].PasswordHash = string(hash)
				m.users[m.editIndex].UpdatedAt = time.Now()
				m.dirty = true
				m.message = "Password updated"
			}
		}
		m.textInput.Blur()
		m.textInput.EchoMode = textinput.EchoNormal
		m.mode = modeEdit
		return m, nil

	case tea.KeyEscape:
		m.textInput.Blur()
		m.textInput.EchoMode = textinput.EchoNormal
		m.mode = modeEdit
		return m, nil

	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

// --- Confirm Dialog ---

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyLeft, tea.KeyRight:
		m.confirmYes = !m.confirmYes
	case tea.KeyEnter:
		if m.confirmYes {
			return m.executeConfirm()
		}
		return m.rejectConfirm()
	case tea.KeyEscape:
		m.mode = m.previousMode()
	default:
		switch msg.String() {
		case "y", "Y":
			m.confirmYes = true
			return m.executeConfirm()
		case "n", "N":
			return m.rejectConfirm()
		}
	}
	return m, nil
}

// rejectConfirm handles the "No" response for confirm dialogs.
// For exit confirm, "No" means quit without saving.
func (m Model) rejectConfirm() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeExitConfirm:
		// "Save changes before exit? No" → quit without saving
		return m, tea.Quit
	default:
		m.mode = m.previousMode()
		return m, nil
	}
}

func (m Model) previousMode() editorMode {
	switch m.mode {
	case modeDeleteConfirm, modeMassDelete, modeValidate, modeMassValidate:
		if m.editIndex >= 0 && m.editIndex < len(m.users) {
			// Could be from edit or list mode; default to list
		}
		return modeList
	case modeExitConfirm, modeSaveConfirm:
		return modeList
	case modeFileChanged:
		return modeList
	}
	return modeList
}

func (m Model) executeConfirm() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeDeleteConfirm:
		idx := m.cursor
		if m.mode == modeDeleteConfirm && m.editIndex >= 0 {
			// Could be from edit screen
		}
		m.softDeleteUser(idx)
		m.mode = modeList
		return m, nil

	case modeMassDelete:
		for i := range m.users {
			if m.tagged[i] {
				m.softDeleteUser(i)
			}
		}
		m.tagged = make(map[int]bool)
		m.mode = modeList
		return m, nil

	case modeValidate:
		idx := m.cursor
		m.autoValidateUser(idx)
		m.mode = modeList
		return m, nil

	case modeMassValidate:
		for i := range m.users {
			if m.tagged[i] {
				m.autoValidateUser(i)
			}
		}
		m.tagged = make(map[int]bool)
		m.mode = modeList
		return m, nil

	case modeExitConfirm:
		// Save and quit
		m.saveAllToDisk()
		return m, tea.Quit

	case modeSaveConfirm:
		m.saveAllToDisk()
		return m, tea.Quit

	case modeFileChanged:
		// Force overwrite
		m.saveAllToDisk()
		m.mode = modeList
		return m, nil
	}

	m.mode = modeList
	return m, nil
}

// --- Help Mode ---

func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Any key dismisses help
	m.mode = modeList
	return m, nil
}

// --- Helper Methods ---

func (m *Model) softDeleteUser(idx int) {
	if idx < 0 || idx >= len(m.users) {
		return
	}
	u := m.users[idx]
	u.DeletedUser = true
	now := time.Now()
	u.DeletedAt = &now
	u.UpdatedAt = now
	m.dirty = true
	m.message = fmt.Sprintf("Deleted: %s", u.Handle)
}

func (m *Model) autoValidateUser(idx int) {
	if idx < 0 || idx >= len(m.users) {
		return
	}
	u := m.users[idx]
	u.AccessLevel = 10
	u.Validated = true
	u.FilePoints = 100
	u.TimeLimit = 60
	u.UpdatedAt = time.Now()
	m.dirty = true
	m.message = fmt.Sprintf("Validated: %s", u.Handle)
}

func (m *Model) toggleSort() {
	m.listAlpha = !m.listAlpha
	if m.listAlpha {
		m.message = "Alphabetizing User List.. Weeee!"
		// Sort by handle alphabetically
		sortUsers(m.users, true)
	} else {
		m.message = "Restoring User List to Original Order!"
		// Sort by ID
		sortUsers(m.users, false)
	}
	m.cursor = 0
	m.scrollOffset = 0
}

func (m *Model) saveCurrentUser() {
	// Nothing special needed - users slice is modified in-place
	// Dirty flag is already set by field edits
}

func (m *Model) saveAllToDisk() {
	if !m.dirty {
		return
	}

	// Check for external modification
	if CheckFileChanged(m.filePath, m.fileMtime) {
		m.mode = modeFileChanged
		m.confirmYes = false
		m.message = "File modified externally! Overwrite?"
		return
	}

	newMtime, err := SaveUsers(m.filePath, m.users)
	if err != nil {
		m.message = fmt.Sprintf("SAVE ERROR: %v", err)
		return
	}
	m.fileMtime = newMtime
	m.dirty = false
	m.message = "Saved successfully"

	// Update original snapshot
	m.origUsers = make([]*user.User, len(m.users))
	for i, u := range m.users {
		m.origUsers[i] = CloneUser(u)
	}
}

// clampScroll adjusts scrollOffset so the cursor is always visible,
// with the lightbar stopping at ~2/3 of the visible area before scrolling.
func (m *Model) clampScroll() {
	total := len(m.users)
	// Scroll threshold: lightbar stops at this row (0-indexed) and list starts scrolling
	scrollThreshold := listVisible * 2 / 3 // ~8 for 13 visible rows

	// If cursor is above the visible window, scroll up to show it at top
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}

	// If cursor has moved past the threshold row, scroll to keep it at threshold
	if m.cursor >= m.scrollOffset+scrollThreshold {
		m.scrollOffset = m.cursor - scrollThreshold
	}

	// Don't scroll past the end of the list
	maxOffset := total - listVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}

	// Don't scroll before the beginning
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
}

func (m Model) taggedCount() int {
	count := 0
	for _, tagged := range m.tagged {
		if tagged {
			count++
		}
	}
	return count
}

// sortUsers sorts users by handle (alpha) or by ID (numeric).
func sortUsers(users []*user.User, alpha bool) {
	if alpha {
		for i := 1; i < len(users); i++ {
			for j := i; j > 0; j-- {
				if strings.ToLower(users[j].Handle) < strings.ToLower(users[j-1].Handle) {
					users[j], users[j-1] = users[j-1], users[j]
				} else {
					break
				}
			}
		}
	} else {
		for i := 1; i < len(users); i++ {
			for j := i; j > 0; j-- {
				if users[j].ID < users[j-1].ID {
					users[j], users[j-1] = users[j-1], users[j]
				} else {
					break
				}
			}
		}
	}
}
