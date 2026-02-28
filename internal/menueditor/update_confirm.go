package menueditor

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

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
		idx := m.pendingDeleteMenuIdx
		m.pendingDeleteMenuIdx = -1
		if idx < 0 || idx >= len(m.menus) {
			m.mode = modeMenuList
			return m, nil
		}
		name := m.menus[idx].Name
		if err := DeleteMenu(m.menuBase, name); err != nil {
			m.message = fmt.Sprintf("Delete error: %v", err)
			m.mode = modeMenuList
			return m, nil
		}
		delete(m.dirtyMenus, name)
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
		if !m.saveAll() {
			// Save failed — stay in the editor so the user can see the error.
			m.mode = modeMenuList
			return m, nil
		}
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
		m.pendingDeleteMenuIdx = -1
		m.mode = m.deleteReturnMode
	case modeDeleteCmdConfirm:
		m.mode = modeCommandList
	}
	return m, nil
}

func (m Model) tryExit() (Model, tea.Cmd) {
	if len(m.dirtyMenus) > 0 || m.dirtyCmds {
		m.mode = modeExitConfirm
		m.confirmYes = true
		return m, nil
	}
	return m, tea.Quit
}
