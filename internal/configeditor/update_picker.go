package configeditor

import (
	tea "github.com/charmbracelet/bubbletea"
)

// pickerVisibleRows is the max number of items visible in the picker popup.
const pickerVisibleRows = 10

// updateLookupPicker handles key input in the lookup picker mode.
func (m Model) updateLookupPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := len(m.pickerItems)
	if total == 0 {
		if msg.Type == tea.KeyEscape {
			m.mode = m.pickerReturnMode
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyUp:
		if m.pickerCursor > 0 {
			m.pickerCursor--
		}

	case tea.KeyDown:
		if m.pickerCursor < total-1 {
			m.pickerCursor++
		}

	case tea.KeyHome:
		m.pickerCursor = 0

	case tea.KeyEnd:
		m.pickerCursor = total - 1

	case tea.KeyPgUp:
		m.pickerCursor -= pickerVisibleRows
		if m.pickerCursor < 0 {
			m.pickerCursor = 0
		}

	case tea.KeyPgDown:
		m.pickerCursor += pickerVisibleRows
		if m.pickerCursor >= total {
			m.pickerCursor = total - 1
		}

	case tea.KeyEnter:
		selected := m.pickerItems[m.pickerCursor]
		var fields []fieldDef
		if m.pickerReturnMode == modeRecordEdit {
			fields = m.recordFields
		} else {
			fields = m.sysFields
		}
		if m.editField >= 0 && m.editField < len(fields) {
			f := fields[m.editField]
			if f.Set != nil {
				if err := f.Set(selected.Value); err != nil {
					m.message = "Invalid selection: " + err.Error()
					return m, nil
				}
				m.dirty = true
				if f.AfterSet != nil {
					f.AfterSet(&m, selected.Value)
				}
			}
		}
		// Rebuild field list in case picker selection changed visible fields.
		if m.pickerReturnMode == modeRecordEdit {
			m.recordFields = m.buildRecordFields()
			if m.editField >= len(m.recordFields) {
				m.editField = len(m.recordFields) - 1
			}
		}
		m.mode = m.pickerReturnMode
		return m, nil

	case tea.KeyEscape:
		m.mode = m.pickerReturnMode
		return m, nil
	}

	// Clamp scroll so cursor is visible
	m.clampPickerScroll()
	return m, nil
}

// clampPickerScroll adjusts pickerScroll so the cursor is visible.
func (m *Model) clampPickerScroll() {
	total := len(m.pickerItems)
	visible := pickerVisibleRows
	if visible > total {
		visible = total
	}

	if m.pickerCursor < m.pickerScroll {
		m.pickerScroll = m.pickerCursor
	}
	if m.pickerCursor >= m.pickerScroll+visible {
		m.pickerScroll = m.pickerCursor - visible + 1
	}

	maxOffset := total - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.pickerScroll > maxOffset {
		m.pickerScroll = maxOffset
	}
	if m.pickerScroll < 0 {
		m.pickerScroll = 0
	}
}
