// Package menueditor implements the ViSiON/3 BBS Menu Editor TUI.
// It faithfully recreates the original MENUEDIT.EXE from Vision/2 (Turbo Pascal).
package menueditor

import (
	"fmt"
	"strings"

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
	menuEditIdx  int        // index into menus slice
	menuFields   []fieldDef // field definitions (from menuFields())
	menuEditFld  int        // currently focused field index

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
	confirmYes bool

	// Dirty tracking
	dirtyMenus map[int]bool // menu indices with unsaved changes
	dirtyCmds  bool         // current command set has unsaved changes

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
		menuBase:   menuBase,
		menus:      menus,
		menuFields: menuFields(),
		cmdFields:  cmdFields(),
		textInput:  ti,
		dirtyMenus: make(map[int]bool),
		width:      minWidth,
		height:     minHeight,
		mode:       modeMenuList,
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
			m.mode = modeMenuList
			return m, nil
		}
	}
	return m, nil
}

// --- Menu List Mode ---

func (m Model) updateMenuList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.menus)
	switch msg.Type {
	case tea.KeyUp:
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case tea.KeyDown:
		if m.menuCursor < total-1 {
			m.menuCursor++
		}
	case tea.KeyHome:
		m.menuCursor = 0
	case tea.KeyEnd:
		m.menuCursor = total - 1
	case tea.KeyPgUp:
		m.menuCursor -= listVisible
		if m.menuCursor < 0 {
			m.menuCursor = 0
		}
	case tea.KeyPgDown:
		m.menuCursor += listVisible
		if m.menuCursor >= total {
			m.menuCursor = total - 1
		}
	case tea.KeyEnter:
		m.menuEditIdx = m.menuCursor
		m.menuEditFld = 0
		m.mode = modeMenuEdit
		return m, nil
	case tea.KeyF2:
		if total == 0 {
			return m, nil
		}
		m.mode = modeDeleteMenuConfirm
		m.confirmYes = false
		return m, nil
	case tea.KeyF5:
		// Prompt for new menu filename
		m.textInput.SetValue("")
		m.textInput.Placeholder = "NEWMENU"
		m.textInput.CharLimit = 12
		m.textInput.Width = 12
		m.textInput.Focus()
		m.mode = modeAddMenu
		return m, textinput.Blink
	case tea.KeyF10:
		// Jump straight to command list for highlighted menu
		if total == 0 {
			return m, nil
		}
		return m.openCommandList(m.menuCursor)
	case tea.KeyEscape:
		return m.tryExit()
	default:
		switch msg.String() {
		case "alt+h":
			m.mode = modeHelp
		}
	}
	m.clampMenuScroll()
	return m, nil
}

// --- Menu Edit Mode ---

func (m Model) updateMenuEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.menuEditFld > 0 {
			m.menuEditFld--
		}
	case tea.KeyDown, tea.KeyTab:
		if m.menuEditFld < len(m.menuFields)-1 {
			m.menuEditFld++
		}
	case tea.KeyEnter:
		return m.startMenuFieldEdit()
	case tea.KeyPgDown:
		// Save current and move to next menu
		m.saveCurrentMenu()
		m.menuEditIdx++
		if m.menuEditIdx >= len(m.menus) {
			m.menuEditIdx = 0
		}
		m.menuEditFld = 0
		m.menuCursor = m.menuEditIdx
		return m, nil
	case tea.KeyPgUp:
		// Save current and move to previous menu
		m.saveCurrentMenu()
		m.menuEditIdx--
		if m.menuEditIdx < 0 {
			m.menuEditIdx = len(m.menus) - 1
		}
		m.menuEditFld = 0
		m.menuCursor = m.menuEditIdx
		return m, nil
	case tea.KeyF2:
		m.mode = modeDeleteMenuConfirm
		m.confirmYes = false
		return m, nil
	case tea.KeyF5:
		// Add new menu
		m.textInput.SetValue("")
		m.textInput.Placeholder = "NEWMENU"
		m.textInput.CharLimit = 12
		m.textInput.Width = 12
		m.textInput.Focus()
		m.mode = modeAddMenu
		return m, textinput.Blink
	case tea.KeyF10:
		// Edit commands for this menu
		m.saveCurrentMenu()
		return m.openCommandList(m.menuEditIdx)
	case tea.KeyEscape:
		m.saveCurrentMenu()
		m.mode = modeMenuList
		return m, nil
	}
	return m, nil
}

func (m Model) startMenuFieldEdit() (Model, tea.Cmd) {
	f := m.menuFields[m.menuEditFld]
	d := &m.menus[m.menuEditIdx].Data
	val := f.GetM(d)

	if f.Type == ftYesNo {
		// Toggle directly without opening text input
		if val == "Y" {
			f.SetM(d, "N")
		} else {
			f.SetM(d, "Y")
		}
		m.dirtyMenus[m.menuEditIdx] = true
		return m, nil
	}

	m.mode = modeMenuEditField
	m.textInput.SetValue(val)
	m.textInput.CharLimit = f.Width
	m.textInput.Width = f.Width
	m.textInput.Placeholder = ""
	m.textInput.CursorEnd()
	m.textInput.Focus()
	return m, textinput.Blink
}

// --- Menu Field Editing Mode ---

func (m Model) updateMenuEditField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.menuFields[m.menuEditFld]
	switch msg.Type {
	case tea.KeyEnter, tea.KeyTab, tea.KeyDown:
		m.applyMenuField(f)
		m.textInput.Blur()
		m.mode = modeMenuEdit
		if m.menuEditFld < len(m.menuFields)-1 {
			m.menuEditFld++
		}
		return m, nil
	case tea.KeyUp:
		m.applyMenuField(f)
		m.textInput.Blur()
		m.mode = modeMenuEdit
		if m.menuEditFld > 0 {
			m.menuEditFld--
		}
		return m, nil
	case tea.KeyEscape:
		m.textInput.Blur()
		m.mode = modeMenuEdit
		return m, nil
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m *Model) applyMenuField(f fieldDef) {
	d := &m.menus[m.menuEditIdx].Data
	if f.SetM != nil {
		if err := f.SetM(d, m.textInput.Value()); err == nil {
			m.dirtyMenus[m.menuEditIdx] = true
		}
	}
}

// --- Command List Mode ---

func (m Model) updateCommandList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.cmds)
	switch msg.Type {
	case tea.KeyUp:
		if m.cmdCursor > 0 {
			m.cmdCursor--
		}
	case tea.KeyDown:
		if m.cmdCursor < total-1 {
			m.cmdCursor++
		}
	case tea.KeyHome:
		m.cmdCursor = 0
	case tea.KeyEnd:
		if total > 0 {
			m.cmdCursor = total - 1
		}
	case tea.KeyPgUp:
		m.cmdCursor -= listVisible
		if m.cmdCursor < 0 {
			m.cmdCursor = 0
		}
	case tea.KeyPgDown:
		m.cmdCursor += listVisible
		if m.cmdCursor >= total {
			m.cmdCursor = total - 1
		}
	case tea.KeyEnter:
		if total > 0 {
			m.cmdEditIdx = m.cmdCursor
			m.cmdEditFld = 0
			m.mode = modeCommandEdit
		}
		return m, nil
	case tea.KeyF2:
		if total == 0 {
			return m, nil
		}
		m.mode = modeDeleteCmdConfirm
		m.confirmYes = false
		return m, nil
	case tea.KeyF5:
		// Append a new empty command and open it for editing
		m.cmds = append(m.cmds, CmdData{})
		m.dirtyCmds = true
		m.cmdCursor = len(m.cmds) - 1
		m.cmdEditIdx = m.cmdCursor
		m.cmdEditFld = 0
		m.mode = modeCommandEdit
		return m, nil
	case tea.KeyEscape:
		if m.dirtyCmds {
			m.saveCurrentCommands()
		}
		m.mode = modeMenuEdit
		m.menuEditIdx = m.cmdsMenuIdx
		m.menuEditFld = 0
		return m, nil
	}
	m.clampCmdScroll()
	return m, nil
}

// --- Command Edit Mode ---

func (m Model) updateCommandEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.cmdEditFld > 0 {
			m.cmdEditFld--
		}
	case tea.KeyDown, tea.KeyTab:
		if m.cmdEditFld < len(m.cmdFields)-1 {
			m.cmdEditFld++
		}
	case tea.KeyEnter:
		return m.startCmdFieldEdit()
	case tea.KeyPgDown:
		m.saveCurrentCmdEdit()
		m.cmdEditIdx++
		if m.cmdEditIdx >= len(m.cmds) {
			m.cmdEditIdx = 0
		}
		m.cmdEditFld = 0
		m.cmdCursor = m.cmdEditIdx
		return m, nil
	case tea.KeyPgUp:
		m.saveCurrentCmdEdit()
		m.cmdEditIdx--
		if m.cmdEditIdx < 0 {
			m.cmdEditIdx = len(m.cmds) - 1
		}
		m.cmdEditFld = 0
		m.cmdCursor = m.cmdEditIdx
		return m, nil
	case tea.KeyF2:
		m.mode = modeDeleteCmdConfirm
		m.confirmYes = false
		return m, nil
	case tea.KeyF5:
		// Save current, append new command, open it
		m.saveCurrentCmdEdit()
		m.cmds = append(m.cmds, CmdData{})
		m.dirtyCmds = true
		m.cmdEditIdx = len(m.cmds) - 1
		m.cmdEditFld = 0
		m.cmdCursor = m.cmdEditIdx
		return m, nil
	case tea.KeyF8:
		// Abort without saving
		m.mode = modeCommandList
		return m, nil
	case tea.KeyEscape:
		m.saveCurrentCmdEdit()
		m.mode = modeCommandList
		return m, nil
	}
	return m, nil
}

func (m Model) startCmdFieldEdit() (Model, tea.Cmd) {
	f := m.cmdFields[m.cmdEditFld]
	d := &m.cmds[m.cmdEditIdx]
	val := f.GetC(d)

	if f.Type == ftYesNo {
		// Toggle directly
		if val == "Y" {
			f.SetC(d, "N")
		} else {
			f.SetC(d, "Y")
		}
		m.dirtyCmds = true
		return m, nil
	}

	m.mode = modeCommandEditField
	m.textInput.SetValue(val)
	m.textInput.CharLimit = f.Width
	m.textInput.Width = f.Width
	m.textInput.Placeholder = ""
	m.textInput.CursorEnd()
	m.textInput.Focus()
	return m, textinput.Blink
}

// --- Command Field Editing Mode ---

func (m Model) updateCommandEditField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.cmdFields[m.cmdEditFld]
	switch msg.Type {
	case tea.KeyEnter, tea.KeyTab, tea.KeyDown:
		m.applyCmdField(f)
		m.textInput.Blur()
		m.mode = modeCommandEdit
		if m.cmdEditFld < len(m.cmdFields)-1 {
			m.cmdEditFld++
		}
		return m, nil
	case tea.KeyUp:
		m.applyCmdField(f)
		m.textInput.Blur()
		m.mode = modeCommandEdit
		if m.cmdEditFld > 0 {
			m.cmdEditFld--
		}
		return m, nil
	case tea.KeyEscape:
		m.textInput.Blur()
		m.mode = modeCommandEdit
		return m, nil
	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m *Model) applyCmdField(f fieldDef) {
	d := &m.cmds[m.cmdEditIdx]
	if f.SetC != nil {
		if err := f.SetC(d, m.textInput.Value()); err == nil {
			m.dirtyCmds = true
		}
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
		return m.rejectConfirm()
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

func (m Model) executeConfirm() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeDeleteMenuConfirm:
		idx := m.menuCursor
		if m.mode == modeDeleteMenuConfirm {
			// If we're in menu edit, delete that menu
			if m.menuEditIdx >= 0 {
				idx = m.menuEditIdx
			}
		}
		name := m.menus[idx].Name
		if err := DeleteMenu(m.menuBase, name); err != nil {
			m.message = fmt.Sprintf("Delete error: %v", err)
			m.mode = modeMenuList
			return m, nil
		}
		delete(m.dirtyMenus, idx)
		m.menus = append(m.menus[:idx], m.menus[idx+1:]...)
		if m.menuCursor >= len(m.menus) && m.menuCursor > 0 {
			m.menuCursor = len(m.menus) - 1
		}
		m.message = fmt.Sprintf("Deleted menu: %s", name)
		m.mode = modeMenuList
		m.clampMenuScroll()
		return m, nil

	case modeDeleteCmdConfirm:
		idx := m.cmdCursor
		if idx < len(m.cmds) {
			m.cmds = append(m.cmds[:idx], m.cmds[idx+1:]...)
			m.dirtyCmds = true
		}
		if m.cmdCursor >= len(m.cmds) && m.cmdCursor > 0 {
			m.cmdCursor = len(m.cmds) - 1
		}
		m.mode = modeCommandList
		m.clampCmdScroll()
		return m, nil

	case modeExitConfirm:
		m.saveAll()
		return m, tea.Quit
	}
	m.mode = modeMenuList
	return m, nil
}

func (m Model) rejectConfirm() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeExitConfirm:
		// "No" = quit without saving
		return m, tea.Quit
	case modeDeleteMenuConfirm:
		m.mode = modeMenuList
	case modeDeleteCmdConfirm:
		m.mode = modeCommandList
	}
	return m, nil
}

// --- Add Menu Input Dialog ---

func (m Model) updateAddMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		name := strings.ToUpper(strings.TrimSpace(m.textInput.Value()))
		m.textInput.Blur()
		if name == "" {
			m.mode = modeMenuList
			return m, nil
		}
		// Validate: alphanumeric + underscore only
		for _, ch := range name {
			if !((ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_') {
				m.message = "Invalid filename: use A-Z, 0-9, _ only"
				m.mode = modeMenuList
				return m, nil
			}
		}
		if MenuExists(m.menuBase, name) {
			m.message = fmt.Sprintf("Menu %s already exists!", name)
			m.mode = modeMenuList
			return m, nil
		}
		if err := CreateMenu(m.menuBase, name); err != nil {
			m.message = fmt.Sprintf("Create error: %v", err)
			m.mode = modeMenuList
			return m, nil
		}
		// Reload menus and jump to the new one
		menus, err := LoadMenus(m.menuBase)
		if err != nil {
			m.message = fmt.Sprintf("Reload error: %v", err)
			m.mode = modeMenuList
			return m, nil
		}
		m.menus = menus
		// Find the new menu's index
		for i, me := range m.menus {
			if me.Name == name {
				m.menuCursor = i
				m.menuEditIdx = i
				break
			}
		}
		m.menuEditFld = 0
		m.mode = modeMenuEdit
		m.message = fmt.Sprintf("Created menu: %s", name)
		return m, nil

	case tea.KeyEscape:
		m.textInput.Blur()
		m.mode = modeMenuList
		return m, nil

	default:
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

// --- Helper Methods ---

func (m Model) tryExit() (Model, tea.Cmd) {
	if len(m.dirtyMenus) > 0 || m.dirtyCmds {
		m.mode = modeExitConfirm
		m.confirmYes = true
		return m, nil
	}
	return m, tea.Quit
}

func (m *Model) saveCurrentMenu() {
	idx := m.menuEditIdx
	if idx < 0 || idx >= len(m.menus) {
		return
	}
	if !m.dirtyMenus[idx] {
		return
	}
	entry := m.menus[idx]
	if err := SaveMenu(m.menuBase, entry.Name, entry.Data); err != nil {
		m.message = fmt.Sprintf("Save error: %v", err)
		return
	}
	delete(m.dirtyMenus, idx)
}

func (m *Model) saveCurrentCmdEdit() {
	// Just marks changes; actual save happens in saveCurrentCommands
	m.dirtyCmds = true
}

func (m *Model) saveCurrentCommands() {
	if m.cmdsMenuIdx < 0 || m.cmdsMenuIdx >= len(m.menus) {
		return
	}
	name := m.menus[m.cmdsMenuIdx].Name
	if err := SaveCommands(m.menuBase, name, m.cmds); err != nil {
		m.message = fmt.Sprintf("Save error: %v", err)
		return
	}
	m.dirtyCmds = false
}

func (m *Model) saveAll() {
	for idx := range m.dirtyMenus {
		if idx < 0 || idx >= len(m.menus) {
			continue
		}
		entry := m.menus[idx]
		if err := SaveMenu(m.menuBase, entry.Name, entry.Data); err != nil {
			m.message = fmt.Sprintf("Save error: %v", err)
		}
	}
	m.dirtyMenus = make(map[int]bool)
	if m.dirtyCmds {
		m.saveCurrentCommands()
	}
}

func (m Model) openCommandList(menuIdx int) (Model, tea.Cmd) {
	if menuIdx < 0 || menuIdx >= len(m.menus) {
		return m, nil
	}
	name := m.menus[menuIdx].Name
	cmds, err := LoadCommands(m.menuBase, name)
	if err != nil {
		m.message = fmt.Sprintf("Load commands error: %v", err)
		return m, nil
	}
	m.cmds = cmds
	m.cmdsMenuIdx = menuIdx
	m.cmdCursor = 0
	m.cmdScroll = 0
	m.dirtyCmds = false
	m.mode = modeCommandList
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
