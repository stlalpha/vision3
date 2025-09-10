package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"fmt"
	"strings"
	"time"
)

// StatusBar represents the bottom status bar
type StatusBar struct {
	width        int
	height       int
	message      string
	messageTime  time.Time
	showTime     bool
	showHelp     bool
	functionKeys []FunctionKey
}

// FunctionKey represents a function key mapping
type FunctionKey struct {
	Key    int    // F1=1, F2=2, etc.
	Label  string
	Action string
}

// NewStatusBar creates a new status bar
func NewStatusBar() *StatusBar {
	sb := &StatusBar{
		width:    80,
		height:   1,
		showTime: true,
		showHelp: true,
	}
	
	// Initialize default function key mappings
	sb.functionKeys = []FunctionKey{
		{Key: 1, Label: "Help", Action: "help"},
		{Key: 2, Label: "Save", Action: "save"},
		{Key: 3, Label: "Open", Action: "open"},
		{Key: 4, Label: "Menu", Action: "menu"},
		{Key: 5, Label: "Copy", Action: "copy"},
		{Key: 6, Label: "Move", Action: "move"},
		{Key: 7, Label: "NewDir", Action: "mkdir"},
		{Key: 8, Label: "Delete", Action: "delete"},
		{Key: 9, Label: "Config", Action: "config"},
		{Key: 10, Label: "Quit", Action: "quit"},
	}
	
	return sb
}

// Init initializes the status bar
func (sb *StatusBar) Init() tea.Cmd {
	return nil
}

// Update handles status bar updates
func (sb *StatusBar) Update(msg tea.Msg) (*StatusBar, tea.Cmd) {
	// Handle any status bar specific messages here
	switch msg := msg.(type) {
	case StatusMessageMsg:
		sb.SetMessage(msg.Message)
	case ClearStatusMsg:
		sb.ClearMessage()
	case SetFunctionKeysMsg:
		sb.functionKeys = msg.Keys
	}
	return sb, nil
}

// SetSize sets the status bar dimensions
func (sb *StatusBar) SetSize(width, height int) {
	sb.width = width
	sb.height = height
}

// SetMessage sets a message in the status bar
func (sb *StatusBar) SetMessage(message string) {
	sb.message = message
	sb.messageTime = time.Now()
}

// ClearMessage clears the status bar message
func (sb *StatusBar) ClearMessage() {
	sb.message = ""
}

// SetFunctionKeys sets the function key mappings
func (sb *StatusBar) SetFunctionKeys(keys []FunctionKey) {
	sb.functionKeys = keys
}

// View renders the status bar
func (sb *StatusBar) View() string {
	if sb.showHelp {
		return sb.renderHelpLine()
	} else {
		return sb.renderStatusLine()
	}
}

// renderStatusLine renders the status line with message and time
func (sb *StatusBar) renderStatusLine() string {
	// Left side: message or default text
	leftText := sb.message
	if leftText == "" {
		leftText = "Ready"
	}
	
	// Right side: current time
	rightText := ""
	if sb.showTime {
		rightText = time.Now().Format("15:04:05")
	}
	
	// Calculate spacing
	usedWidth := lipgloss.Width(leftText) + lipgloss.Width(rightText)
	spacing := sb.width - usedWidth
	if spacing < 1 {
		spacing = 1
	}
	
	// Build status line
	statusLine := leftText + strings.Repeat(" ", spacing) + rightText
	
	// Ensure exact width
	if len(statusLine) > sb.width {
		statusLine = statusLine[:sb.width]
	} else if len(statusLine) < sb.width {
		statusLine += strings.Repeat(" ", sb.width-len(statusLine))
	}
	
	return StatusBarStyle.Width(sb.width).Render(statusLine)
}

// renderHelpLine renders the function key help line
func (sb *StatusBar) renderHelpLine() string {
	var keyLabels []string
	
	// Build function key labels
	for _, fkey := range sb.functionKeys {
		if fkey.Key <= 10 {
			label := fmt.Sprintf("F%d", fkey.Key)
			if fkey.Label != "" {
				label += " " + fkey.Label
			}
			keyLabels = append(keyLabels, label)
		}
	}
	
	// Join labels with separators
	helpText := strings.Join(keyLabels, " │ ")
	
	// Truncate if too long
	if lipgloss.Width(helpText) > sb.width {
		// Try to fit as many as possible
		var fitted []string
		currentWidth := 0
		for _, label := range keyLabels {
			labelWidth := lipgloss.Width(label) + 3 // Include separator
			if currentWidth + labelWidth <= sb.width {
				fitted = append(fitted, label)
				currentWidth += labelWidth
			} else {
				break
			}
		}
		helpText = strings.Join(fitted, " │ ")
	}
	
	// Center the help text
	helpText = lipgloss.NewStyle().
		Width(sb.width).
		Align(lipgloss.Center).
		Render(helpText)
	
	return StatusBarStyle.Width(sb.width).Render(helpText)
}

// ToggleView toggles between status and help view
func (sb *StatusBar) ToggleView() {
	sb.showHelp = !sb.showHelp
}

// ShowHelp shows the function key help
func (sb *StatusBar) ShowHelp() {
	sb.showHelp = true
}

// ShowStatus shows the status message
func (sb *StatusBar) ShowStatus() {
	sb.showHelp = false
}

// SetShowTime enables/disables time display
func (sb *StatusBar) SetShowTime(show bool) {
	sb.showTime = show
}

// GetMessage returns the current status message
func (sb *StatusBar) GetMessage() string {
	return sb.message
}

// IsShowingHelp returns true if showing help line
func (sb *StatusBar) IsShowingHelp() bool {
	return sb.showHelp
}

// CreateContextualKeys creates function keys for a specific context
func CreateContextualKeys(context string) []FunctionKey {
	switch context {
	case "main":
		return []FunctionKey{
			{Key: 1, Label: "Help", Action: "help"},
			{Key: 2, Label: "Save", Action: "save"},
			{Key: 3, Label: "Open", Action: "open"},
			{Key: 4, Label: "Menu", Action: "menu"},
			{Key: 10, Label: "Quit", Action: "quit"},
		}
	case "config":
		return []FunctionKey{
			{Key: 1, Label: "Help", Action: "help"},
			{Key: 2, Label: "Save", Action: "save"},
			{Key: 3, Label: "Revert", Action: "revert"},
			{Key: 4, Label: "Test", Action: "test"},
			{Key: 10, Label: "Exit", Action: "exit"},
		}
	case "file_manager":
		return []FunctionKey{
			{Key: 1, Label: "Help", Action: "help"},
			{Key: 2, Label: "Rename", Action: "rename"},
			{Key: 3, Label: "View", Action: "view"},
			{Key: 4, Label: "Edit", Action: "edit"},
			{Key: 5, Label: "Copy", Action: "copy"},
			{Key: 6, Label: "Move", Action: "move"},
			{Key: 7, Label: "NewDir", Action: "mkdir"},
			{Key: 8, Label: "Delete", Action: "delete"},
			{Key: 10, Label: "Exit", Action: "exit"},
		}
	case "user_editor":
		return []FunctionKey{
			{Key: 1, Label: "Help", Action: "help"},
			{Key: 2, Label: "Save", Action: "save"},
			{Key: 3, Label: "Delete", Action: "delete"},
			{Key: 4, Label: "Groups", Action: "groups"},
			{Key: 5, Label: "Access", Action: "access"},
			{Key: 6, Label: "Stats", Action: "stats"},
			{Key: 10, Label: "Exit", Action: "exit"},
		}
	case "log_viewer":
		return []FunctionKey{
			{Key: 1, Label: "Help", Action: "help"},
			{Key: 2, Label: "Refresh", Action: "refresh"},
			{Key: 3, Label: "Filter", Action: "filter"},
			{Key: 4, Label: "Search", Action: "search"},
			{Key: 5, Label: "Export", Action: "export"},
			{Key: 10, Label: "Exit", Action: "exit"},
		}
	case "dialog":
		return []FunctionKey{
			{Key: 1, Label: "Help", Action: "help"},
			{Key: 10, Label: "Cancel", Action: "cancel"},
		}
	default:
		// Default function keys
		return []FunctionKey{
			{Key: 1, Label: "Help", Action: "help"},
			{Key: 10, Label: "Exit", Action: "exit"},
		}
	}
}

// Message types for status bar communication
type StatusMessageMsg struct {
	Message string
}

type ClearStatusMsg struct{}

type SetFunctionKeysMsg struct {
	Keys []FunctionKey
}

// Helper functions to create status messages
func NewStatusMessage(message string) tea.Cmd {
	return func() tea.Msg {
		return StatusMessageMsg{Message: message}
	}
}

func ClearStatus() tea.Cmd {
	return func() tea.Msg {
		return ClearStatusMsg{}
	}
}

func SetFunctionKeys(keys []FunctionKey) tea.Cmd {
	return func() tea.Msg {
		return SetFunctionKeysMsg{Keys: keys}
	}
}

// Predefined status messages
const (
	StatusReady          = "Ready"
	StatusSaving         = "Saving..."
	StatusSaved          = "Saved"
	StatusLoading        = "Loading..."
	StatusLoaded         = "Loaded"
	StatusConnecting     = "Connecting..."
	StatusConnected      = "Connected"
	StatusDisconnected   = "Disconnected"
	StatusError          = "Error"
	StatusProcessing     = "Processing..."
	StatusComplete       = "Complete"
	StatusCancelled      = "Cancelled"
	StatusConfigChanged  = "Configuration changed - Press F2 to save"
	StatusUnsavedChanges = "Unsaved changes"
)