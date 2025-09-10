package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"strings"
)

// KeyHandler manages global keyboard shortcuts and function keys
type KeyHandler struct {
	context string // Current context for contextual key handling
}

// NewKeyHandler creates a new key handler
func NewKeyHandler() *KeyHandler {
	return &KeyHandler{
		context: "main",
	}
}

// HandleKey processes keyboard input and returns appropriate commands
func (kh *KeyHandler) HandleKey(msg tea.KeyMsg, app *Application) tea.Cmd {
	// Handle function keys first
	if cmd := kh.handleFunctionKeys(msg, app); cmd != nil {
		return cmd
	}
	
	// Handle global shortcuts
	if cmd := kh.handleGlobalShortcuts(msg, app); cmd != nil {
		return cmd
	}
	
	// Handle Alt+Key combinations for menu access
	if cmd := kh.handleAltKeys(msg, app); cmd != nil {
		return cmd
	}
	
	return nil
}

// handleFunctionKeys processes F1-F10 keys
func (kh *KeyHandler) handleFunctionKeys(msg tea.KeyMsg, app *Application) tea.Cmd {
	switch msg.Type {
	case tea.KeyF1:
		return kh.handleF1(app) // Help
	case tea.KeyF2:
		return kh.handleF2(app) // Save
	case tea.KeyF3:
		return kh.handleF3(app) // Open/View
	case tea.KeyF4:
		return kh.handleF4(app) // Menu/Edit
	case tea.KeyF5:
		return kh.handleF5(app) // Copy/Refresh
	case tea.KeyF6:
		return kh.handleF6(app) // Move/Switch
	case tea.KeyF7:
		return kh.handleF7(app) // New Directory/Create
	case tea.KeyF8:
		return kh.handleF8(app) // Delete
	case tea.KeyF9:
		return kh.handleF9(app) // Config/Options
	case tea.KeyF10:
		return kh.handleF10(app) // Quit/Exit
	}
	return nil
}

// handleGlobalShortcuts processes global keyboard shortcuts
func (kh *KeyHandler) handleGlobalShortcuts(msg tea.KeyMsg, app *Application) tea.Cmd {
	switch msg.Type {
	case tea.KeyCtrlQ:
		// Quick quit
		return tea.Quit
		
	case tea.KeyCtrlS:
		// Quick save
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 2, Action: "save", Context: kh.context}
		}
		
	case tea.KeyCtrlO:
		// Quick open
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 3, Action: "open", Context: kh.context}
		}
		
	case tea.KeyCtrlN:
		// New
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 7, Action: "new", Context: kh.context}
		}
		
	case tea.KeyCtrlH:
		// Help
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 1, Action: "help", Context: kh.context}
		}
		
	case tea.KeyCtrlR:
		// Refresh
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 5, Action: "refresh", Context: kh.context}
		}
		
	case tea.KeyCtrlD:
		// Delete (with confirmation)
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 8, Action: "delete", Context: kh.context}
		}
	}
	return nil
}

// handleAltKeys processes Alt+Key combinations for menu access
func (kh *KeyHandler) handleAltKeys(msg tea.KeyMsg, app *Application) tea.Cmd {
	if !msg.Alt || len(msg.Runes) == 0 {
		return nil
	}
	
	key := strings.ToLower(string(msg.Runes[0]))
	
	switch key {
	case "f":
		// Alt+F - File menu
		return func() tea.Msg {
			return MenuActivateMsg{Menu: "file"}
		}
	case "e":
		// Alt+E - Edit menu
		return func() tea.Msg {
			return MenuActivateMsg{Menu: "edit"}
		}
	case "c":
		// Alt+C - Config menu
		return func() tea.Msg {
			return MenuActivateMsg{Menu: "config"}
		}
	case "t":
		// Alt+T - Tools menu
		return func() tea.Msg {
			return MenuActivateMsg{Menu: "tools"}
		}
	case "h":
		// Alt+H - Help menu
		return func() tea.Msg {
			return MenuActivateMsg{Menu: "help"}
		}
	}
	
	return nil
}

// Context-specific function key handlers
func (kh *KeyHandler) handleF1(app *Application) tea.Cmd {
	// F1 is always Help
	return func() tea.Msg {
		return FunctionKeyMsg{Key: 1, Action: "help", Context: kh.context}
	}
}

func (kh *KeyHandler) handleF2(app *Application) tea.Cmd {
	switch kh.context {
	case "config":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 2, Action: "save", Context: kh.context}
		}
	case "file_manager":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 2, Action: "rename", Context: kh.context}
		}
	case "user_editor":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 2, Action: "save_user", Context: kh.context}
		}
	default:
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 2, Action: "save", Context: kh.context}
		}
	}
}

func (kh *KeyHandler) handleF3(app *Application) tea.Cmd {
	switch kh.context {
	case "config":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 3, Action: "revert", Context: kh.context}
		}
	case "file_manager":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 3, Action: "view_file", Context: kh.context}
		}
	case "user_editor":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 3, Action: "delete_user", Context: kh.context}
		}
	case "log_viewer":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 3, Action: "filter", Context: kh.context}
		}
	default:
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 3, Action: "open", Context: kh.context}
		}
	}
}

func (kh *KeyHandler) handleF4(app *Application) tea.Cmd {
	switch kh.context {
	case "config":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 4, Action: "test_config", Context: kh.context}
		}
	case "file_manager":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 4, Action: "edit_file", Context: kh.context}
		}
	case "user_editor":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 4, Action: "user_groups", Context: kh.context}
		}
	case "log_viewer":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 4, Action: "search", Context: kh.context}
		}
	default:
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 4, Action: "menu", Context: kh.context}
		}
	}
}

func (kh *KeyHandler) handleF5(app *Application) tea.Cmd {
	switch kh.context {
	case "file_manager":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 5, Action: "copy_file", Context: kh.context}
		}
	case "user_editor":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 5, Action: "user_access", Context: kh.context}
		}
	case "log_viewer":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 5, Action: "export_log", Context: kh.context}
		}
	default:
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 5, Action: "refresh", Context: kh.context}
		}
	}
}

func (kh *KeyHandler) handleF6(app *Application) tea.Cmd {
	switch kh.context {
	case "file_manager":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 6, Action: "move_file", Context: kh.context}
		}
	case "user_editor":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 6, Action: "user_stats", Context: kh.context}
		}
	default:
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 6, Action: "switch", Context: kh.context}
		}
	}
}

func (kh *KeyHandler) handleF7(app *Application) tea.Cmd {
	switch kh.context {
	case "file_manager":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 7, Action: "create_directory", Context: kh.context}
		}
	default:
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 7, Action: "create_new", Context: kh.context}
		}
	}
}

func (kh *KeyHandler) handleF8(app *Application) tea.Cmd {
	// F8 is typically Delete with confirmation
	return func() tea.Msg {
		return FunctionKeyMsg{Key: 8, Action: "delete", Context: kh.context}
	}
}

func (kh *KeyHandler) handleF9(app *Application) tea.Cmd {
	switch kh.context {
	case "config":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 9, Action: "advanced_config", Context: kh.context}
		}
	default:
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 9, Action: "config", Context: kh.context}
		}
	}
}

func (kh *KeyHandler) handleF10(app *Application) tea.Cmd {
	// F10 is typically Quit/Exit
	switch kh.context {
	case "dialog":
		return func() tea.Msg {
			return FunctionKeyMsg{Key: 10, Action: "cancel", Context: kh.context}
		}
	default:
		return tea.Quit
	}
}

// SetContext changes the current context for contextual key handling
func (kh *KeyHandler) SetContext(context string) {
	kh.context = context
}

// GetContext returns the current context
func (kh *KeyHandler) GetContext() string {
	return kh.context
}

// Message types for key handling
type FunctionKeyMsg struct {
	Key     int         // F-key number (1-10)
	Action  string      // Action to perform
	Context string      // Current context
	Data    interface{} // Additional data
}

type MenuActivateMsg struct {
	Menu string // Menu to activate
}

// Helper function to create function key messages
func NewFunctionKeyMsg(key int, action, context string) tea.Cmd {
	return func() tea.Msg {
		return FunctionKeyMsg{Key: key, Action: action, Context: context}
	}
}

// Predefined contexts
const (
	ContextMain        = "main"
	ContextConfig      = "config"
	ContextFileManager = "file_manager"
	ContextUserEditor  = "user_editor"
	ContextLogViewer   = "log_viewer"
	ContextDialog      = "dialog"
	ContextHelp        = "help"
	ContextMenu        = "menu"
)

// Key action constants
const (
	ActionHelp           = "help"
	ActionSave           = "save"
	ActionOpen           = "open"
	ActionNew            = "new"
	ActionDelete         = "delete"
	ActionRefresh        = "refresh"
	ActionQuit           = "quit"
	ActionExit           = "exit"
	ActionCancel         = "cancel"
	ActionMenu           = "menu"
	ActionConfig         = "config"
	ActionCopy           = "copy"
	ActionMove           = "move"
	ActionRename         = "rename"
	ActionView           = "view"
	ActionEdit           = "edit"
	ActionFilter         = "filter"
	ActionSearch         = "search"
	ActionExport         = "export"
	ActionCreateDir      = "create_directory"
	ActionTest           = "test"
	ActionRevert         = "revert"
	ActionAdvanced       = "advanced"
	ActionUserGroups     = "user_groups"
	ActionUserAccess     = "user_access"
	ActionUserStats      = "user_stats"
	ActionSaveUser       = "save_user"
	ActionDeleteUser     = "delete_user"
)