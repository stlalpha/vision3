package configeditor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/stlalpha/vision3/internal/config"
)

const (
	minWidth  = 80
	minHeight = 25
)

// editorMode represents the current interaction state.
type editorMode int

const (
	modeTopMenu        editorMode = iota // Top-level menu
	modeSysConfigMenu                    // System Configuration inner menu
	modeSysConfigEdit                    // System config field navigation
	modeSysConfigField                   // System config field editing (textinput active)
	modeRecordList                       // Scrollable record list
	modeRecordEdit                       // Single record field navigation
	modeRecordField                      // Single record field editing
	modeExitConfirm                      // Unsaved changes exit confirm
	modeSaveConfirm                      // Confirm save
	modeHelp                             // Help screen overlay
	modeDeleteConfirm                    // Confirm delete record
	modeLookupPicker                     // Lookup picker popup
	modeRecordReorder                    // Reorder mode (move record to new position)
)

// topMenuItem defines an entry in the top-level menu.
type topMenuItem struct {
	Key   string // Display key (1-9, A, Q)
	Label string // Display label
}

// sysConfigMenuItem defines an entry in the system config inner menu.
type sysConfigMenuItem struct {
	Label string
}

// Model is the BubbleTea model for the config editor TUI.
type Model struct {
	// Config data
	configs    allConfigs
	origServer config.ServerConfig // snapshot for dirty tracking
	configPath string
	dirty      bool

	// Top menu state
	topCursor int
	topItems  []topMenuItem

	// System config inner menu
	sysMenuCursor int
	sysMenuItems  []sysConfigMenuItem
	sysSubScreen  int        // which sub-screen (0-5)
	sysFields     []fieldDef // current sub-screen fields

	// Record list state
	recordType    string     // "msgarea", "filearea", "conference", etc.
	recordCursor  int
	recordScroll  int
	recordFields  []fieldDef // fields for current record
	recordEditIdx int        // index of record being edited
	editField     int        // current field index
	fieldScroll   int        // first visible field row in edit screens

	// Reorder state
	reorderSourceIdx int // index of record being moved (-1 when inactive)
	reorderMinIdx    int // lower bound for cursor in reorder mode (conference clamp)
	reorderMaxIdx    int // upper bound (inclusive) for cursor in reorder mode

	// Text input (shared for editing fields)
	textInput textinput.Model

	// Lookup picker state
	pickerItems      []LookupItem // items for current picker
	pickerCursor     int          // highlighted item
	pickerScroll     int          // scroll offset
	pickerReturnMode editorMode   // mode to return to on cancel/select

	// Confirm dialog
	confirmYes bool

	// Terminal
	width   int
	height  int
	mode    editorMode
	message string // Flash message
}

// New creates a new config editor model.
func New(configPath string) (Model, error) {
	configs, err := loadAllConfigs(configPath)
	if err != nil {
		return Model{}, fmt.Errorf("loading configs: %w", err)
	}

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 80
	ti.Width = 40

	topItems := []topMenuItem{
		{"1", "System Configuration"},
		{"2", "Message Areas"},
		{"3", "File Areas"},
		{"4", "Conferences"},
		{"5", "Door Programs"},
		{"6", "Event Scheduler"},
		{"7", "FTN Network"},
		{"8", "Transfer Protocols"},
		{"9", "Archivers"},
		{"A", "Login Sequence"},
		{"Q", "Quit Program"},
	}

	sysMenuItems := []sysConfigMenuItem{
		{"BBS Registration"},
		{"Network Setup"},
		{"Connection Limits"},
		{"Access Levels"},
		{"Default Settings"},
		{"IP Blocklist/Allowlist"},
	}

	return Model{
		configs:      configs,
		origServer:   configs.Server,
		configPath:   configPath,
		topItems:     topItems,
		sysMenuItems: sysMenuItems,
		textInput:    ti,
		width:        minWidth,
		height:       minHeight,
		mode:         modeTopMenu,
	}, nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.SetWindowTitle("ViSiON/3 Configuration Editor")
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
		case modeTopMenu:
			return m.updateTopMenu(msg)
		case modeSysConfigMenu:
			return m.updateSysConfigMenu(msg)
		case modeSysConfigEdit:
			return m.updateSysConfigEdit(msg)
		case modeSysConfigField:
			return m.updateSysConfigField(msg)
		case modeRecordList:
			return m.updateRecordList(msg)
		case modeRecordReorder:
			return m.updateRecordReorder(msg)
		case modeRecordEdit:
			return m.updateRecordEdit(msg)
		case modeRecordField:
			return m.updateRecordField(msg)
		case modeExitConfirm, modeSaveConfirm, modeDeleteConfirm:
			return m.updateConfirm(msg)
		case modeLookupPicker:
			return m.updateLookupPicker(msg)
		case modeHelp:
			return m.updateHelp(msg)
		}
	}
	return m, nil
}

// --- Top Menu Mode ---

func (m Model) updateTopMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.topCursor > 0 {
			m.topCursor--
		}
	case tea.KeyDown:
		if m.topCursor < len(m.topItems)-1 {
			m.topCursor++
		}
	case tea.KeyHome:
		m.topCursor = 0
	case tea.KeyEnd:
		m.topCursor = len(m.topItems) - 1
	case tea.KeyEnter:
		return m.selectTopMenuItem()
	case tea.KeyEscape:
		return m.tryExit()
	default:
		key := strings.ToUpper(msg.String())
		for i, item := range m.topItems {
			if item.Key == key {
				m.topCursor = i
				return m.selectTopMenuItem()
			}
		}
	}
	return m, nil
}

func (m Model) selectTopMenuItem() (Model, tea.Cmd) {
	recordTypes := []string{
		"", "msgarea", "filearea", "conference", "door",
		"event", "ftn", "protocol", "archiver", "login",
	}

	switch m.topCursor {
	case 0: // System Configuration
		m.mode = modeSysConfigMenu
		m.sysMenuCursor = 0
		return m, nil
	case 10: // Quit
		return m.tryExit()
	default:
		// Items 1-9 are record list editors
		if m.topCursor > 0 && m.topCursor < len(recordTypes) {
			m.recordType = recordTypes[m.topCursor]
			m.recordCursor = 0
			m.recordScroll = 0
			m.mode = modeRecordList
		}
		return m, nil
	}
}

func (m Model) tryExit() (Model, tea.Cmd) {
	if m.dirty {
		m.mode = modeExitConfirm
		m.confirmYes = true
		return m, nil
	}
	return m, tea.Quit
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
		// Save confirmation requires an explicit yes/no choice
		if m.mode != modeExitConfirm {
			return m.rejectConfirm()
		}
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

func (m Model) rejectConfirm() (Model, tea.Cmd) {
	switch m.mode {
	case modeExitConfirm:
		return m, tea.Quit
	case modeDeleteConfirm:
		m.mode = modeRecordList
		return m, nil
	default:
		m.mode = modeTopMenu
		return m, nil
	}
}

func (m Model) executeConfirm() (Model, tea.Cmd) {
	switch m.mode {
	case modeExitConfirm, modeSaveConfirm:
		m.saveAll()
		return m, tea.Quit
	case modeDeleteConfirm:
		m.deleteRecord()
		m.dirty = true
		m.mode = modeRecordList
		return m, nil
	}
	m.mode = modeTopMenu
	return m, nil
}

// --- Help Mode ---

func (m Model) updateHelp(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = modeTopMenu
	return m, nil
}
