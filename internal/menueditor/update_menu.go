package menueditor

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

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
		if total > 0 {
			m.menuCursor = total - 1
		}
	case tea.KeyPgUp:
		m.menuCursor -= listVisible
		if m.menuCursor < 0 {
			m.menuCursor = 0
		}
	case tea.KeyPgDown:
		m.menuCursor += listVisible
		if m.menuCursor >= total {
			if total > 0 {
				m.menuCursor = total - 1
			} else {
				m.menuCursor = 0
			}
		}
	case tea.KeyEnter:
		if total == 0 {
			return m, nil
		}
		m.menuEditIdx = m.menuCursor
		m.menuEditFld = 0
		m.mode = modeMenuEdit
		return m, nil
	case tea.KeyF2:
		if total == 0 {
			return m, nil
		}
		m.pendingDeleteMenuIdx = m.menuCursor
		m.deleteReturnMode = modeMenuList
		m.mode = modeDeleteMenuConfirm
		m.confirmYes = false
		return m, nil
	case tea.KeyF5:
		// Prompt for new menu filename
		m.textInput.SetValue("")
		m.textInput.Placeholder = "NEWMENU"
		m.textInput.CharLimit = 8
		m.textInput.Width = 8
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
			m.helpReturnMode = m.mode
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
		m.pendingDeleteMenuIdx = m.menuEditIdx
		m.deleteReturnMode = modeMenuEdit
		m.mode = modeDeleteMenuConfirm
		m.confirmYes = false
		return m, nil
	case tea.KeyF5:
		// Add new menu
		m.textInput.SetValue("")
		m.textInput.Placeholder = "NEWMENU"
		m.textInput.CharLimit = 8
		m.textInput.Width = 8
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
	if m.menuEditIdx < 0 || m.menuEditIdx >= len(m.menus) {
		return m, nil
	}
	if m.menuEditFld < 0 || m.menuEditFld >= len(m.menuFields) {
		return m, nil
	}
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
		m.dirtyMenus[m.menus[m.menuEditIdx].Name] = true
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
		if err := m.applyMenuField(f); err != nil {
			m.message = fmt.Sprintf("Invalid value: %v", err)
			return m, nil // stay in edit mode so user can fix
		}
		m.textInput.Blur()
		m.mode = modeMenuEdit
		if m.menuEditFld < len(m.menuFields)-1 {
			m.menuEditFld++
		}
		return m, nil
	case tea.KeyUp:
		if err := m.applyMenuField(f); err != nil {
			m.message = fmt.Sprintf("Invalid value: %v", err)
			return m, nil
		}
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

func (m *Model) applyMenuField(f fieldDef) error {
	d := &m.menus[m.menuEditIdx].Data
	if f.SetM != nil {
		if err := f.SetM(d, m.textInput.Value()); err != nil {
			return err
		}
		m.dirtyMenus[m.menus[m.menuEditIdx].Name] = true
	}
	return nil
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
