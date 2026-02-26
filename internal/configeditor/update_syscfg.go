package configeditor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// --- System Config Menu Mode ---

func (m Model) updateSysConfigMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyUp:
		if m.sysMenuCursor > 0 {
			m.sysMenuCursor--
		}
	case tea.KeyDown:
		if m.sysMenuCursor < len(m.sysMenuItems)-1 {
			m.sysMenuCursor++
		}
	case tea.KeyHome:
		m.sysMenuCursor = 0
	case tea.KeyEnd:
		m.sysMenuCursor = len(m.sysMenuItems) - 1
	case tea.KeyEnter:
		m.sysSubScreen = m.sysMenuCursor
		m.sysFields = m.buildSysFields(m.sysSubScreen)
		m.editField = 0
		m.fieldScroll = 0
		m.mode = modeSysConfigEdit
		return m, nil
	case tea.KeyEscape:
		m.mode = modeTopMenu
		return m, nil
	default:
		key := strings.ToUpper(msg.String())
		if key == "Q" {
			m.mode = modeTopMenu
			return m, nil
		}
		if len(key) == 1 && key[0] >= '1' && key[0] <= '6' {
			idx := int(key[0] - '1')
			if idx < len(m.sysMenuItems) {
				m.sysMenuCursor = idx
				m.sysSubScreen = idx
				m.sysFields = m.buildSysFields(m.sysSubScreen)
				m.editField = 0
				m.fieldScroll = 0
				m.mode = modeSysConfigEdit
				return m, nil
			}
		}
	}
	return m, nil
}

// --- System Config Edit Mode (field navigation) ---

func (m Model) updateSysConfigEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.sysFields) == 0 {
		if msg.Type == tea.KeyEscape {
			m.mode = modeSysConfigMenu
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyTab, tea.KeyEnter:
		f := m.sysFields[m.editField]
		if f.Type == ftDisplay {
			m.editField = m.nextSysEditableField(1)
			m.clampFieldScroll(m.sysFields)
			return m, nil
		}
		if f.Type == ftYesNo {
			m.toggleSysYesNo(f)
			return m, nil
		}
		return m.startSysFieldEdit()

	case tea.KeySpace:
		f := m.sysFields[m.editField]
		if f.Type == ftYesNo {
			m.toggleSysYesNo(f)
		}
		return m, nil

	case tea.KeyDown:
		m.editField = m.nextSysEditableField(1)
		m.clampFieldScroll(m.sysFields)

	case tea.KeyUp:
		m.editField = m.nextSysEditableField(-1)
		m.clampFieldScroll(m.sysFields)

	case tea.KeyEscape:
		m.mode = modeSysConfigMenu
		return m, nil

	case tea.KeyPgDown:
		if m.sysSubScreen < len(m.sysMenuItems)-1 {
			m.sysSubScreen++
			m.sysFields = m.buildSysFields(m.sysSubScreen)
			m.editField = 0
		}
		return m, nil

	case tea.KeyPgUp:
		if m.sysSubScreen > 0 {
			m.sysSubScreen--
			m.sysFields = m.buildSysFields(m.sysSubScreen)
			m.editField = 0
		}
		return m, nil
	}
	return m, nil
}

func (m Model) nextSysEditableField(dir int) int {
	n := len(m.sysFields)
	if n == 0 {
		return 0
	}
	idx := m.editField + dir
	if idx > n-1 {
		idx = 0
	} else if idx < 0 {
		idx = n - 1
	}
	return idx
}

// toggleSysYesNo flips a Y/N field value in place for system config fields.
func (m *Model) toggleSysYesNo(f fieldDef) {
	if f.Get != nil && f.Set != nil {
		if f.Get() == "Y" {
			f.Set("N")
		} else {
			f.Set("Y")
		}
		m.dirty = true
		m.message = ""
	}
}

func (m Model) startSysFieldEdit() (Model, tea.Cmd) {
	f := m.sysFields[m.editField]
	if f.Type == ftDisplay {
		return m, nil
	}

	if f.Type == ftLookup && f.LookupItems != nil {
		m.pickerItems = f.LookupItems()
		m.pickerCursor = 0
		m.pickerScroll = 0
		m.pickerReturnMode = modeSysConfigEdit
		// Pre-select current value by matching display text
		cur := f.Get()
		for i, item := range m.pickerItems {
			if item.Value == cur || item.Display == cur {
				m.pickerCursor = i
				break
			}
		}
		// Also try matching by value embedded in display (e.g. "Name (ID: 1)")
		if m.pickerCursor == 0 && len(m.pickerItems) > 0 {
			for i, item := range m.pickerItems {
				if strings.Contains(cur, "(ID: "+item.Value+")") {
					m.pickerCursor = i
					break
				}
			}
		}
		m.clampPickerScroll()
		m.mode = modeLookupPicker
		return m, nil
	}

	val := f.Get()
	m.mode = modeSysConfigField
	m.textInput.SetValue(val)
	m.textInput.CharLimit = f.Width
	m.textInput.Width = f.Width
	m.textInput.EchoMode = textinput.EchoNormal
	m.textInput.Placeholder = ""
	m.textInput.CursorEnd()
	m.textInput.Focus()

	return m, textinput.Blink
}

// --- System Config Field Editing Mode ---

func (m Model) updateSysConfigField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.sysFields[m.editField]

	switch msg.Type {
	case tea.KeyEnter, tea.KeyTab, tea.KeyDown:
		if err := m.applySysFieldValue(f); err != nil {
			m.message = fmt.Sprintf("Invalid: %v", err)
			return m, nil
		}
		m.textInput.Blur()
		m.mode = modeSysConfigEdit
		m.editField = m.nextSysEditableField(1)
		m.clampFieldScroll(m.sysFields)
		return m, nil

	case tea.KeyUp:
		if err := m.applySysFieldValue(f); err != nil {
			m.message = fmt.Sprintf("Invalid: %v", err)
			return m, nil
		}
		m.textInput.Blur()
		m.mode = modeSysConfigEdit
		m.editField = m.nextSysEditableField(-1)
		m.clampFieldScroll(m.sysFields)
		return m, nil

	case tea.KeyEscape:
		m.textInput.Blur()
		m.mode = modeSysConfigEdit
		return m, nil

	default:
		if f.Type == ftYesNo {
			if len(msg.Runes) == 1 {
				ch := msg.Runes[0]
				if ch == 'y' || ch == 'Y' {
					m.textInput.SetValue("Y")
				} else if ch == 'n' || ch == 'N' {
					m.textInput.SetValue("N")
				}
				if err := m.applySysFieldValue(f); err == nil {
					m.textInput.Blur()
					m.mode = modeSysConfigEdit
					m.editField = m.nextSysEditableField(1)
					m.clampFieldScroll(m.sysFields)
				}
				return m, nil
			}
			return m, nil
		}

		if f.Type == ftInteger {
			if len(msg.Runes) == 1 {
				ch := msg.Runes[0]
				if (ch < '0' || ch > '9') && ch != '-' {
					return m, nil
				}
			}
		}

		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		return m, cmd
	}
}

func (m *Model) applySysFieldValue(f fieldDef) error {
	val := m.textInput.Value()

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
		if err := f.Set(val); err != nil {
			return err
		}
		m.dirty = true
		m.message = ""
	}
	return nil
}
