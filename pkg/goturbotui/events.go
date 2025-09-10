package goturbotui

// EventType represents the type of input event
type EventType int

const (
	EventKey EventType = iota
	EventMouse
	EventResize
)

// Event represents a user input event
type Event struct {
	Type     EventType
	Key      Key
	Rune     rune
	Mouse    MouseEvent
	Resize   ResizeEvent
}

// Key represents keyboard input
type Key struct {
	Code      KeyCode
	Modifiers KeyMod
}

// KeyCode represents specific keyboard keys
type KeyCode int

const (
	KeyUnknown KeyCode = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyEnter
	KeyEscape
	KeyTab
	KeyBackspace
	KeyDelete
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

// KeyMod represents key modifiers
type KeyMod int

const (
	ModNone KeyMod = 0
	ModAlt  KeyMod = 1 << iota
	ModCtrl
	ModShift
)

// MouseEvent represents mouse input
type MouseEvent struct {
	X, Y   int
	Button MouseButton
	Action MouseAction
}

// MouseButton represents mouse buttons
type MouseButton int

const (
	MouseNone MouseButton = iota
	MouseLeft
	MouseRight
	MouseMiddle
)

// MouseAction represents mouse actions
type MouseAction int

const (
	MousePress MouseAction = iota
	MouseRelease
	MouseMove
)

// ResizeEvent represents terminal resize
type ResizeEvent struct {
	Width, Height int
}