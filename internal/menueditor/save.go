package menueditor

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) saveCurrentMenu() {
	idx := m.menuEditIdx
	if idx < 0 || idx >= len(m.menus) {
		return
	}
	entry := m.menus[idx]
	if !m.dirtyMenus[entry.Name] {
		return
	}
	if err := SaveMenu(m.menuBase, entry.Name, entry.Data); err != nil {
		m.message = fmt.Sprintf("Save error: %v", err)
		return
	}
	delete(m.dirtyMenus, entry.Name)
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
	for _, entry := range m.menus {
		if m.dirtyMenus[entry.Name] {
			if err := SaveMenu(m.menuBase, entry.Name, entry.Data); err != nil {
				m.message = fmt.Sprintf("Save error: %v", err)
			}
		}
	}
	m.dirtyMenus = make(map[string]bool)
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
