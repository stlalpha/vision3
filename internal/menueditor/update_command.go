package menueditor

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

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
			if total > 0 {
				m.cmdCursor = total - 1
			} else {
				m.cmdCursor = 0
			}
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
			if err := m.saveCurrentCommands(); err != nil {
				return m, nil // stay in command list so user can retry
			}
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
		if len(m.cmds) == 0 {
			return m, nil
		}
		m.saveCurrentCmdEdit()
		m.cmdEditIdx++
		if m.cmdEditIdx >= len(m.cmds) {
			m.cmdEditIdx = 0
		}
		m.cmdEditFld = 0
		m.cmdCursor = m.cmdEditIdx
		return m, nil
	case tea.KeyPgUp:
		if len(m.cmds) == 0 {
			return m, nil
		}
		m.saveCurrentCmdEdit()
		m.cmdEditIdx--
		if m.cmdEditIdx < 0 {
			m.cmdEditIdx = len(m.cmds) - 1
		}
		m.cmdEditFld = 0
		m.cmdCursor = m.cmdEditIdx
		return m, nil
	case tea.KeyF2:
		if len(m.cmds) == 0 {
			return m, nil
		}
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
	if m.cmdEditIdx < 0 || m.cmdEditIdx >= len(m.cmds) {
		return m, nil
	}
	if m.cmdEditFld < 0 || m.cmdEditFld >= len(m.cmdFields) {
		return m, nil
	}
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
		if err := m.applyCmdField(f); err != nil {
			m.message = fmt.Sprintf("Invalid value: %v", err)
			return m, nil
		}
		m.textInput.Blur()
		m.mode = modeCommandEdit
		if m.cmdEditFld < len(m.cmdFields)-1 {
			m.cmdEditFld++
		}
		return m, nil
	case tea.KeyUp:
		if err := m.applyCmdField(f); err != nil {
			m.message = fmt.Sprintf("Invalid value: %v", err)
			return m, nil
		}
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

func (m *Model) applyCmdField(f fieldDef) error {
	d := &m.cmds[m.cmdEditIdx]
	if f.SetC != nil {
		if err := f.SetC(d, m.textInput.Value()); err != nil {
			return err
		}
		m.dirtyCmds = true
	}
	return nil
}
