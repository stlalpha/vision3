package configeditor

import (
	"github.com/stlalpha/vision3/internal/conference"
	"github.com/stlalpha/vision3/internal/message"
)

// reorderRecord performs the slice manipulation for the current record type:
// removes the source item and inserts it at the cursor position, then renumbers positions.
func (m *Model) reorderRecord() {
	src := m.reorderSourceIdx
	dst := m.recordCursor
	if src == dst {
		return
	}

	switch m.recordType {
	case "msgarea":
		m.configs.MsgAreas = reorderSlice(m.configs.MsgAreas, src, dst)
		renumberMsgAreaPositions(m.configs.MsgAreas)
	case "filearea":
		m.configs.FileAreas = reorderSlice(m.configs.FileAreas, src, dst)
	case "conference":
		m.configs.Conferences = reorderSlice(m.configs.Conferences, src, dst)
		renumberConferencePositions(m.configs.Conferences)
	case "protocol":
		m.configs.Protocols = reorderSlice(m.configs.Protocols, src, dst)
	case "archiver":
		m.configs.Archivers.Archivers = reorderSlice(m.configs.Archivers.Archivers, src, dst)
	case "login":
		m.configs.LoginSeq = reorderSlice(m.configs.LoginSeq, src, dst)
	}
}

// reorderSlice removes the item at src and inserts it at dst, returning the modified slice.
func reorderSlice[T any](s []T, src, dst int) []T {
	if src < 0 || src >= len(s) || dst < 0 || dst >= len(s) {
		return s
	}
	item := s[src]
	// Remove source
	result := make([]T, 0, len(s))
	for i := range s {
		if i != src {
			result = append(result, s[i])
		}
	}
	// Insert at destination
	final := make([]T, 0, len(s))
	final = append(final, result[:dst]...)
	final = append(final, item)
	final = append(final, result[dst:]...)
	return final
}

// renumberMsgAreaPositions sets Position = index + 1 for each area in the slice.
func renumberMsgAreaPositions(areas []message.MessageArea) {
	for i := range areas {
		areas[i].Position = i + 1
	}
}

// renumberConferencePositions sets Position = index + 1 for each conference in the slice.
func renumberConferencePositions(confs []conference.Conference) {
	for i := range confs {
		confs[i].Position = i + 1
	}
}
