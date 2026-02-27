package configeditor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// --- Record List Mode ---

func (m Model) updateRecordList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	total := m.recordCount()
	listVisible := m.recordListVisible()

	switch msg.Type {
	case tea.KeyUp:
		if m.recordCursor > 0 {
			m.recordCursor--
		}
	case tea.KeyDown:
		if m.recordCursor < total-1 {
			m.recordCursor++
		}
	case tea.KeyHome:
		m.recordCursor = 0
	case tea.KeyEnd:
		if total > 0 {
			m.recordCursor = total - 1
		}
	case tea.KeyPgUp:
		m.recordCursor -= listVisible
		if m.recordCursor < 0 {
			m.recordCursor = 0
		}
	case tea.KeyPgDown:
		m.recordCursor += listVisible
		if m.recordCursor >= total {
			m.recordCursor = total - 1
		}
		if m.recordCursor < 0 {
			m.recordCursor = 0
		}
	case tea.KeyEnter:
		if total > 0 {
			m.recordEditIdx = m.recordCursor
			m.recordFields = m.buildRecordFields()
			m.editField = 0
			m.fieldScroll = 0
			m.mode = modeRecordEdit
		}
		return m, nil
	case tea.KeyEscape:
		m.mode = modeTopMenu
		return m, nil
	default:
		switch msg.String() {
		case "i", "I", "insert":
			m.insertRecord()
			m.dirty = true
			m.recordCursor = m.recordCount() - 1
			m.clampRecordScroll()
			return m, nil
		case "g", "G":
			if m.recordType == "ftn" {
				m.recordFields = m.fieldsFTNGlobal()
				m.editField = 0
				m.fieldScroll = 0
				m.recordEditIdx = -1
				m.mode = modeRecordEdit
			}
			return m, nil
		case "d", "D", "delete":
			if total > 0 {
				m.mode = modeDeleteConfirm
				m.confirmYes = false
			}
			return m, nil
		case "p", "P":
			if total > 0 && m.recordTypeSupportsReorder() {
				m.reorderSourceIdx = m.recordCursor
				m.reorderMinIdx = 0
				m.reorderMaxIdx = total - 1

				// For message areas, clamp to the conference of the source item.
				if m.recordType == "msgarea" && m.recordCursor < len(m.configs.MsgAreas) {
					confID := m.configs.MsgAreas[m.recordCursor].ConferenceID
					lo, hi := m.recordCursor, m.recordCursor
					for lo > 0 && m.configs.MsgAreas[lo-1].ConferenceID == confID {
						lo--
					}
					for hi < len(m.configs.MsgAreas)-1 && m.configs.MsgAreas[hi+1].ConferenceID == confID {
						hi++
					}
					m.reorderMinIdx = lo
					m.reorderMaxIdx = hi
				}

				m.mode = modeRecordReorder
			}
			return m, nil
		}
	}
	m.clampRecordScroll()
	return m, nil
}

// recordTypeSupportsReorder returns true if the current record type supports P-key reordering.
func (m Model) recordTypeSupportsReorder() bool {
	switch m.recordType {
	case "msgarea", "filearea", "conference", "login", "protocol", "archiver":
		return true
	}
	return false
}

// --- Record Reorder Mode ---

func (m Model) updateRecordReorder(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	listVisible := m.recordListVisible()
	lo := m.reorderMinIdx
	hi := m.reorderMaxIdx

	switch msg.Type {
	case tea.KeyUp:
		if m.recordCursor > lo {
			m.recordCursor--
		}
	case tea.KeyDown:
		if m.recordCursor < hi {
			m.recordCursor++
		}
	case tea.KeyHome:
		m.recordCursor = lo
	case tea.KeyEnd:
		m.recordCursor = hi
	case tea.KeyPgUp:
		m.recordCursor -= listVisible
		if m.recordCursor < lo {
			m.recordCursor = lo
		}
	case tea.KeyPgDown:
		m.recordCursor += listVisible
		if m.recordCursor > hi {
			m.recordCursor = hi
		}
	case tea.KeyEnter:
		m.reorderRecord()
		m.dirty = true
		m.reorderSourceIdx = -1
		m.mode = modeRecordList
		m.clampRecordScroll()
		return m, nil
	case tea.KeyEscape:
		m.recordCursor = m.reorderSourceIdx
		m.reorderSourceIdx = -1
		m.mode = modeRecordList
		m.clampRecordScroll()
		return m, nil
	}
	m.clampRecordScroll()
	return m, nil
}

// --- Record Edit Mode ---

func (m Model) updateRecordEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if len(m.recordFields) == 0 {
		if msg.Type == tea.KeyEscape {
			m.mode = modeRecordList
		}
		return m, nil
	}

	switch msg.Type {
	case tea.KeyTab, tea.KeyEnter:
		f := m.recordFields[m.editField]
		if f.Type == ftDisplay {
			m.editField = m.nextRecordEditableField(1)
			m.clampFieldScroll(m.recordFields)
			return m, nil
		}
		if f.Type == ftYesNo {
			m.toggleYesNo(f)
			return m, nil
		}
		return m.startRecordFieldEdit()

	case tea.KeySpace:
		f := m.recordFields[m.editField]
		if f.Type == ftYesNo {
			m.toggleYesNo(f)
		}
		return m, nil

	case tea.KeyDown:
		m.editField = m.nextRecordEditableField(1)
		m.clampFieldScroll(m.recordFields)

	case tea.KeyUp:
		m.editField = m.nextRecordEditableField(-1)
		m.clampFieldScroll(m.recordFields)

	case tea.KeyEscape:
		m.mode = modeRecordList
		return m, nil

	case tea.KeyPgDown:
		total := m.recordCount()
		if m.recordEditIdx >= 0 && total > 0 && m.recordEditIdx < total-1 {
			m.recordEditIdx++
			m.recordFields = m.buildRecordFields()
			m.editField = 0
			m.fieldScroll = 0
		}
		return m, nil

	case tea.KeyPgUp:
		if m.recordEditIdx > 0 {
			m.recordEditIdx--
			m.recordFields = m.buildRecordFields()
			m.editField = 0
			m.fieldScroll = 0
		}
		return m, nil
	}
	return m, nil
}

func (m Model) nextRecordEditableField(dir int) int {
	n := len(m.recordFields)
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

func (m Model) startRecordFieldEdit() (Model, tea.Cmd) {
	f := m.recordFields[m.editField]
	if f.Type == ftDisplay {
		return m, nil
	}

	if f.Type == ftLookup && f.LookupItems != nil {
		m.pickerItems = f.LookupItems()
		m.pickerCursor = 0
		m.pickerScroll = 0
		m.pickerReturnMode = modeRecordEdit
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
	m.mode = modeRecordField
	m.textInput.SetValue(val)
	m.textInput.CharLimit = f.Width
	m.textInput.Width = f.Width
	m.textInput.EchoMode = textinput.EchoNormal
	m.textInput.Placeholder = ""
	m.textInput.CursorEnd()
	m.textInput.Focus()

	return m, textinput.Blink
}

// --- Record Field Editing Mode ---

func (m Model) updateRecordField(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	f := m.recordFields[m.editField]

	switch msg.Type {
	case tea.KeyEnter, tea.KeyTab, tea.KeyDown:
		if err := m.applyRecordFieldValue(f); err != nil {
			m.message = fmt.Sprintf("Invalid: %v", err)
			return m, nil
		}
		m.textInput.Blur()
		m.mode = modeRecordEdit
		if !m.stayOnField {
			m.editField = m.nextRecordEditableField(1)
		}
		m.stayOnField = false
		m.clampFieldScroll(m.recordFields)
		return m, nil

	case tea.KeyUp:
		if err := m.applyRecordFieldValue(f); err != nil {
			m.message = fmt.Sprintf("Invalid: %v", err)
			return m, nil
		}
		m.textInput.Blur()
		m.mode = modeRecordEdit
		if !m.stayOnField {
			m.editField = m.nextRecordEditableField(-1)
		}
		m.stayOnField = false
		m.clampFieldScroll(m.recordFields)
		return m, nil

	case tea.KeyEscape:
		m.textInput.Blur()
		m.mode = modeRecordEdit
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
				if err := m.applyRecordFieldValue(f); err == nil {
					m.textInput.Blur()
					m.mode = modeRecordEdit
					m.editField = m.nextRecordEditableField(1)
					m.clampFieldScroll(m.recordFields)
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

func (m *Model) applyRecordFieldValue(f fieldDef) error {
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
		if f.AfterSet != nil {
			f.AfterSet(m, val)
		}
	}

	// Rebuild field list in case a toggle (e.g. Is DOS) changed visible fields.
	m.recordFields = m.buildRecordFields()
	if m.editField >= len(m.recordFields) {
		m.editField = len(m.recordFields) - 1
	}

	return nil
}

// toggleYesNo flips a Y/N field value in place.
func (m *Model) toggleYesNo(f fieldDef) {
	if f.Get != nil && f.Set != nil {
		if f.Get() == "Y" {
			f.Set("N")
		} else {
			f.Set("Y")
		}
		m.dirty = true
		m.message = ""
		// Rebuild fields in case toggle changed visible fields (e.g. Is DOS)
		m.recordFields = m.buildRecordFields()
		if m.editField >= len(m.recordFields) {
			m.editField = len(m.recordFields) - 1
		}
	}
}

// maxFieldRows is the maximum number of field rows visible in the edit box.
const maxFieldRows = 12

// clampFieldScroll adjusts fieldScroll so the active field row is visible.
func (m *Model) clampFieldScroll(fields []fieldDef) {
	if len(fields) == 0 {
		m.fieldScroll = 0
		return
	}
	// Get the row of the current field
	activeRow := fields[m.editField].Row

	if activeRow < m.fieldScroll+1 {
		m.fieldScroll = activeRow - 1
	}
	if activeRow > m.fieldScroll+maxFieldRows {
		m.fieldScroll = activeRow - maxFieldRows
	}
	// Keep 1 row of context above the cursor when at the top edge
	if m.fieldScroll > 0 && activeRow == m.fieldScroll+1 {
		m.fieldScroll--
	}
	if m.fieldScroll < 0 {
		m.fieldScroll = 0
	}
}

// --- Scroll helper ---

func (m *Model) clampRecordScroll() {
	total := m.recordCount()
	visible := m.recordListVisible()
	scrollThreshold := visible * 2 / 3

	if m.recordCursor < m.recordScroll {
		m.recordScroll = m.recordCursor
	}
	if m.recordCursor >= m.recordScroll+scrollThreshold {
		m.recordScroll = m.recordCursor - scrollThreshold
	}
	maxOffset := total - visible
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.recordScroll > maxOffset {
		m.recordScroll = maxOffset
	}
	if m.recordScroll < 0 {
		m.recordScroll = 0
	}
}
