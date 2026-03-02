// Package menueditor implements the ViSiON/3 BBS Menu Editor TUI.
// It faithfully recreates the original MENUEDIT.EXE from Vision/2 (Turbo Pascal).
package menueditor

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	minWidth  = 80
	minHeight = 25

	listVisible    = 15 // rows visible in the menu/command list
	menuEditFields = 7  // number of editable menu fields
	cmdEditFields  = 6  // number of editable command fields
)

// editorMode represents the current interaction state.
type editorMode int

const (
	modeMenuList          editorMode = iota // Scrollable list of all menus
	modeMenuEdit                            // Field editor for a single menu
	modeMenuEditField                       // Active text input on a menu field
	modeCommandList                         // Scrollable list of commands for selected menu
	modeCommandEdit                         // Field editor for a single command
	modeCommandEditField                    // Active text input on a command field
	modeDeleteMenuConfirm                   // Confirm menu delete
	modeDeleteCmdConfirm                    // Confirm command delete
	modeAddMenu                             // Input dialog: enter new menu filename
	modeExitConfirm                         // Unsaved changes on exit
	modeHelp                                // Help overlay
)

// Model is the BubbleTea model for the menu editor TUI.
type Model struct {
	menuBase string // path to menu set directory (parent of mnu/ and cfg/)

	// Menu list state
	menus      []menuEntry
	menuCursor int
	menuScroll int

	// Menu edit state
	menuEditIdx int        // index into menus slice
	menuFields  []fieldDef // field definitions (from menuFields())
	menuEditFld int        // currently focused field index

	// Command list state
	cmds        []CmdData // commands loaded for cmdsMenuIdx
	cmdsMenuIdx int       // which menu index the cmds belong to
	cmdCursor   int
	cmdScroll   int

	// Command edit state
	cmdEditIdx int
	cmdFields  []fieldDef
	cmdEditFld int

	// Shared text input for field editing and prompts
	textInput textinput.Model

	// Confirm dialog
	confirmYes           bool
	deleteReturnMode     editorMode // mode to return to if delete is cancelled
	pendingDeleteMenuIdx int        // index of menu to delete (-1 if none pending)

	// Dirty tracking
	dirtyMenus map[string]bool // menu names with unsaved changes
	dirtyCmds  bool            // current command set has unsaved changes

	// Help overlay
	helpReturnMode editorMode // mode to restore when help is dismissed

	// Terminal dimensions
	width  int
	height int
	mode   editorMode

	message string // flash message (cleared on next key)
}

// New creates a new menu editor model.
func New(menuBase string) (Model, error) {
	menus, err := LoadMenus(menuBase)
	if err != nil {
		return Model{}, fmt.Errorf("loading menus: %w", err)
	}

	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 80
	ti.Width = 40

	return Model{
		menuBase:             menuBase,
		menus:                menus,
		menuFields:           menuFields(),
		cmdFields:            cmdFields(),
		textInput:            ti,
		dirtyMenus:           make(map[string]bool),
		pendingDeleteMenuIdx: -1,
		width:                minWidth,
		height:               minHeight,
		mode:                 modeMenuList,
	}, nil
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return tea.SetWindowTitle("ViSiON/3 Menu Editor")
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
		m.message = "" // clear flash on any key
		switch m.mode {
		case modeMenuList:
			return m.updateMenuList(msg)
		case modeMenuEdit:
			return m.updateMenuEdit(msg)
		case modeMenuEditField:
			return m.updateMenuEditField(msg)
		case modeCommandList:
			return m.updateCommandList(msg)
		case modeCommandEdit:
			return m.updateCommandEdit(msg)
		case modeCommandEditField:
			return m.updateCommandEditField(msg)
		case modeDeleteMenuConfirm, modeDeleteCmdConfirm, modeExitConfirm:
			return m.updateConfirm(msg)
		case modeAddMenu:
			return m.updateAddMenu(msg)
		case modeHelp:
			m.mode = m.helpReturnMode
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) clampMenuScroll() {
	total := len(m.menus)
	threshold := listVisible * 2 / 3
	if m.menuCursor < m.menuScroll {
		m.menuScroll = m.menuCursor
	}
	if m.menuCursor >= m.menuScroll+threshold {
		m.menuScroll = m.menuCursor - threshold
	}
	maxOffset := total - listVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.menuScroll > maxOffset {
		m.menuScroll = maxOffset
	}
	if m.menuScroll < 0 {
		m.menuScroll = 0
	}
}

func (m *Model) clampCmdScroll() {
	total := len(m.cmds)
	threshold := listVisible * 2 / 3
	if m.cmdCursor < m.cmdScroll {
		m.cmdScroll = m.cmdCursor
	}
	if m.cmdCursor >= m.cmdScroll+threshold {
		m.cmdScroll = m.cmdCursor - threshold
	}
	maxOffset := total - listVisible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.cmdScroll > maxOffset {
		m.cmdScroll = maxOffset
	}
	if m.cmdScroll < 0 {
		m.cmdScroll = 0
	}
}
