package stringeditor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	itemsPerPage = 20 // Matches the Pascal original's 20-item pages
	labelCol     = 30 // Column where values start (dcol in Pascal)
	minWidth     = 80 // Minimum terminal width
	minHeight    = 25 // Minimum terminal height (matching 80x25 DOS)
)

// editorMode represents the current interaction state.
type editorMode int

const (
	modeNavigate editorMode = iota
	modeEdit
	modeAbortConfirm
	modeSearch
)

// Model is the BubbleTea model for the string editor TUI.
type Model struct {
	// Data
	entries  []StringEntry     // Ordered metadata entries
	values   map[string]string // Current string values (key -> value)
	filePath string            // Path to strings.json
	dirty    bool              // Whether values have been modified

	// Navigation
	cursor   int // Current item index (0-based, across all pages)
	page     int // Current page (0-based)
	numPages int

	// UI state
	mode   editorMode
	width  int
	height int

	// Editing
	textInput textinput.Model
	editKey   string // The key being edited

	// Abort confirm dialog
	abortYes bool // true = Yes selected in abort dialog

	// Search
	searchInput textinput.Model
	searchQuery string

	// Message (flash message shown briefly)
	message string
}

// New creates a new string editor model.
func New(filePath string) (Model, error) {
	entries := StringEntries()
	values, err := LoadStrings(filePath)
	if err != nil {
		return Model{}, fmt.Errorf("loading strings: %w", err)
	}

	ti := textinput.New()
	ti.CharLimit = 200
	ti.Width = 48

	si := textinput.New()
	si.Placeholder = "Search..."
	si.CharLimit = 40
	si.Width = 30

	numPages := (len(entries) + itemsPerPage - 1) / itemsPerPage

	return Model{
		entries:     entries,
		values:      values,
		filePath:    filePath,
		cursor:      0,
		page:        0,
		numPages:    numPages,
		mode:        modeNavigate,
		width:       minWidth,
		height:      minHeight,
		textInput:   ti,
		searchInput: si,
		abortYes:    false,
	}, nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.SetWindowTitle("ViSiON/3 String Configuration")
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
		m.textInput.Width = m.width - labelCol - 4
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeNavigate:
			return m.updateNavigate(msg)
		case modeEdit:
			return m.updateEdit(msg)
		case modeAbortConfirm:
			return m.updateAbortConfirm(msg)
		case modeSearch:
			return m.updateSearch(msg)
		}
	}
	return m, nil
}

// updateNavigate handles keys in navigation mode.
func (m Model) updateNavigate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.cursor > 0 {
			m.cursor--
			m.page = m.cursor / itemsPerPage
		}
	case tea.KeyDown:
		if m.cursor < len(m.entries)-1 {
			m.cursor++
			m.page = m.cursor / itemsPerPage
		}
	case tea.KeyPgUp:
		m.page--
		if m.page < 0 {
			m.page = 0
		}
		m.cursor = m.page * itemsPerPage
	case tea.KeyPgDown:
		m.page++
		if m.page >= m.numPages {
			m.page = m.numPages - 1
		}
		m.cursor = m.page * itemsPerPage
		if m.cursor >= len(m.entries) {
			m.cursor = len(m.entries) - 1
		}
	case tea.KeyHome:
		m.cursor = 0
		m.page = 0
	case tea.KeyEnd:
		m.cursor = len(m.entries) - 1
		m.page = m.numPages - 1
	case tea.KeyEnter:
		return m.startEdit("")
	case tea.KeyEscape:
		m.mode = modeAbortConfirm
		m.abortYes = false
		return m, nil
	case tea.KeyF2:
		// Edit with pre-filled value
		entry := m.entries[m.cursor]
		return m.startEdit(m.getValue(entry.Key))
	case tea.KeyF10:
		// Save and exit
		if err := SaveStrings(m.filePath, m.values); err != nil {
			m.message = fmt.Sprintf("ERROR: %v", err)
			return m, nil
		}
		return m, tea.Quit

	default:
		switch msg.String() {
		case "/":
			// Enter search mode
			m.mode = modeSearch
			m.searchInput.SetValue("")
			m.searchInput.Focus()
			return m, textinput.Blink
		default:
			// If a printable character is typed, start editing with that char
			if len(msg.Runes) == 1 && msg.Runes[0] >= 32 {
				return m.startEdit(string(msg.Runes))
			}
		}
	}
	return m, nil
}

// startEdit enters edit mode for the currently selected item.
func (m Model) startEdit(prefill string) (tea.Model, tea.Cmd) {
	entry := m.entries[m.cursor]
	if entry.Key[0] == '_' {
		// Can't edit placeholder entries
		m.message = "This field is reserved and cannot be edited"
		return m, nil
	}
	m.mode = modeEdit
	m.editKey = entry.Key
	m.textInput.SetValue(prefill)
	m.textInput.CursorEnd()
	m.textInput.Focus()
	m.message = ""
	return m, textinput.Blink
}

// updateEdit handles keys in edit mode.
func (m Model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Confirm edit
		newVal := m.textInput.Value()
		if newVal != "" || m.textInput.Value() != m.getValue(m.editKey) {
			m.values[m.editKey] = newVal
			m.dirty = true
		}
		m.mode = modeNavigate
		m.textInput.Blur()
		return m, nil
	case tea.KeyEscape:
		// Cancel edit
		m.mode = modeNavigate
		m.textInput.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

// updateAbortConfirm handles keys in the abort confirmation dialog.
func (m Model) updateAbortConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyLeft, tea.KeyRight:
		m.abortYes = !m.abortYes
	case tea.KeyEnter:
		if m.abortYes {
			return m, tea.Quit
		}
		m.mode = modeNavigate
	case tea.KeyEscape:
		m.mode = modeNavigate
	default:
		switch msg.String() {
		case "y", "Y":
			return m, tea.Quit
		case "n", "N":
			m.mode = modeNavigate
		}
	}
	return m, nil
}

// updateSearch handles keys in search mode.
func (m Model) updateSearch(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		// Find first match
		query := strings.ToLower(m.searchInput.Value())
		if query != "" {
			// Search from current position forward, wrapping
			for offset := 0; offset < len(m.entries); offset++ {
				idx := (m.cursor + offset + 1) % len(m.entries)
				entry := m.entries[idx]
				if strings.Contains(strings.ToLower(entry.Label), query) ||
					strings.Contains(strings.ToLower(entry.Key), query) ||
					strings.Contains(strings.ToLower(entry.Description), query) {
					m.cursor = idx
					m.page = idx / itemsPerPage
					m.message = fmt.Sprintf("Found: %s", entry.Label)
					break
				}
			}
		}
		m.mode = modeNavigate
		m.searchInput.Blur()
		return m, nil
	case tea.KeyEscape:
		m.mode = modeNavigate
		m.searchInput.Blur()
		return m, nil
	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}
}

// getValue returns the current value for a key, or empty string.
func (m Model) getValue(key string) string {
	if v, ok := m.values[key]; ok {
		return v
	}
	return ""
}
