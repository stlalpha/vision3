package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"fmt"
)

// Application represents the main TUI application
type Application struct {
	windowManager *WindowManager
	menuBar      *MenuBar
	statusBar    *StatusBar
	width        int
	height       int
	currentView  View
	keyHandler   *KeyHandler
}

// View represents different application views/screens
type View int

const (
	MainView View = iota
	FileMenuView
	EditMenuView
	ConfigMenuView
	ToolsMenuView
	HelpMenuView
)

// NewApplication creates a new TUI application with Turbo Pascal styling
func NewApplication() *Application {
	app := &Application{
		width:       80,
		height:      25,
		currentView: MainView,
	}
	
	// Initialize components
	app.windowManager = NewWindowManager()
	app.menuBar = NewMenuBar()
	app.statusBar = NewStatusBar()
	app.keyHandler = NewKeyHandler()
	
	return app
}

// Init implements tea.Model
func (a *Application) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		a.menuBar.Init(),
		a.statusBar.Init(),
	)
}

// Update implements tea.Model
func (a *Application) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		
		// Update component sizes
		a.menuBar.SetSize(a.width, 1)
		a.statusBar.SetSize(a.width, 1)
		a.windowManager.SetSize(a.width, a.height-2) // Account for menu and status bars
		
	case tea.KeyMsg:
		// Handle global keyboard shortcuts
		if cmd := a.keyHandler.HandleKey(msg, a); cmd != nil {
			cmds = append(cmds, cmd)
		}
		
		// Pass to active component
		switch a.currentView {
		case MainView:
			// Handle main view interactions
		default:
			// Handle other views
		}
		
	case MenuActionMsg:
		// Handle menu actions
		if cmd := a.handleMenuAction(msg.Action); cmd != nil {
			cmds = append(cmds, cmd)
		}
		
	case DialogCloseMsg:
		// Handle dialog close
		a.CloseTopWindow()
		
	case FunctionKeyMsg:
		// Handle function key actions
		if cmd := a.handleFunctionKey(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		
	case tea.QuitMsg:
		return a, tea.Quit
	}
	
	// Update components
	var cmd tea.Cmd
	a.menuBar, cmd = a.menuBar.Update(msg)
	cmds = append(cmds, cmd)
	
	a.statusBar, cmd = a.statusBar.Update(msg)
	cmds = append(cmds, cmd)
	
	a.windowManager, cmd = a.windowManager.Update(msg)
	cmds = append(cmds, cmd)
	
	return a, tea.Batch(cmds...)
}

// View implements tea.Model
func (a *Application) View() string {
	if a.width < 40 || a.height < 10 {
		// Terminal too small
		return CenterText(a.width, a.height, 
			"Terminal too small\nMinimum 40x10 required")
	}
	
	// Build the main layout
	content := []string{
		a.menuBar.View(),
		a.renderMainContent(),
		a.statusBar.View(),
	}
	
	return lipgloss.JoinVertical(lipgloss.Top, content...)
}

// renderMainContent renders the main content area
func (a *Application) renderMainContent() string {
	contentHeight := a.height - 2 // Subtract menu and status bars
	
	// Create main content area with Turbo Pascal background
	mainStyle := lipgloss.NewStyle().
		Width(a.width).
		Height(contentHeight).
		Background(ColorBackground).
		Foreground(ColorText)
	
	// Render based on current view
	var content string
	switch a.currentView {
	case MainView:
		content = a.renderWelcomeScreen()
	default:
		content = "View not implemented"
	}
	
	// Add any active windows/dialogs
	if a.windowManager.HasActiveWindows() {
		content = a.windowManager.RenderWithOverlays(content)
	}
	
	return mainStyle.Render(content)
}

// Handle menu actions and other messages
func (a *Application) handleMenuAction(action string) tea.Cmd {
	switch action {
	case "system_config":
		return a.showSystemConfig()
	case "user_config":
		return a.showUserConfig()
	case "message_config":
		return a.showMessageConfig()
	case "file_config":
		return a.showFileConfig()
	case "network_config":
		return a.showNetworkConfig()
	case "security_config":
		return a.showSecurityConfig()
	case "user_editor":
		return a.showUserEditor()
	case "file_manager":
		return a.showFileManager()
	case "log_viewer":
		return a.showLogViewer()
	case "system_info":
		return a.showSystemInfo()
	case "statistics":
		return a.showStatistics()
	case "help_contents":
		return a.showHelpContents()
	case "keyboard_help":
		return a.showKeyboardHelp()
	case "about":
		return a.showAbout()
	default:
		// Show placeholder dialog for unimplemented features
		dialog := NewDialog("Not Implemented", 
			fmt.Sprintf("The '%s' feature is not yet implemented.", action), 
			[]string{"OK"})
		dialog.Center(a.width, a.height)
		a.ShowDialog(dialog)
		return nil
	}
}

// handleFunctionKey handles function key messages
func (a *Application) handleFunctionKey(msg FunctionKeyMsg) tea.Cmd {
	switch msg.Action {
	case ActionHelp:
		return a.showHelpContents()
	case ActionQuit:
		return tea.Quit
	default:
		// Show info about unimplemented function key
		dialog := NewDialog("Function Key", 
			fmt.Sprintf("F%d (%s) pressed in context '%s'", msg.Key, msg.Action, msg.Context), 
			[]string{"OK"})
		dialog.Center(a.width, a.height)
		a.ShowDialog(dialog)
		return nil
	}
}

// Placeholder methods for different screens/dialogs
func (a *Application) showSystemConfig() tea.Cmd {
	dialog := NewDialog("System Configuration", 
		"Configure basic system settings including:\n\n• System name and description\n• Telnet and SSH ports\n• Time zone and locale\n• System paths and directories", 
		[]string{"Configure", "Cancel"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showUserConfig() tea.Cmd {
	dialog := NewDialog("User Management", 
		"Manage user accounts and permissions:\n\n• Create and edit user accounts\n• Set security levels and access\n• Configure user limits\n• Manage user groups", 
		[]string{"Manage", "Cancel"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showMessageConfig() tea.Cmd {
	dialog := NewDialog("Message Areas", 
		"Configure message areas and forums:\n\n• Create and edit message areas\n• Set access permissions\n• Configure network links\n• Manage moderators", 
		[]string{"Configure", "Cancel"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showFileConfig() tea.Cmd {
	dialog := NewDialog("File Areas", 
		"Configure file transfer areas:\n\n• Create and edit file areas\n• Set upload/download permissions\n• Configure file protocols\n• Manage file descriptions", 
		[]string{"Configure", "Cancel"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showNetworkConfig() tea.Cmd {
	dialog := NewDialog("Network Settings", 
		"Configure network and connectivity:\n\n• Mailer configuration\n• Network protocols\n• Internet connectivity\n• Remote system links", 
		[]string{"Configure", "Cancel"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showSecurityConfig() tea.Cmd {
	dialog := NewDialog("Security Configuration", 
		"Configure system security settings:\n\n• Password policies\n• Login restrictions\n• Security levels\n• Access controls", 
		[]string{"Configure", "Cancel"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showUserEditor() tea.Cmd {
	dialog := NewDialog("User Editor", 
		"Launch the user account editor:\n\n• Edit existing user accounts\n• Create new users\n• Set permissions and access\n• View user statistics", 
		[]string{"Launch", "Cancel"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showFileManager() tea.Cmd {
	dialog := NewDialog("File Manager", 
		"Launch the system file manager:\n\n• Browse system directories\n• Manage BBS files\n• Upload/download files\n• Set file permissions", 
		[]string{"Launch", "Cancel"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showLogViewer() tea.Cmd {
	dialog := NewDialog("Log Viewer", 
		"View system logs and statistics:\n\n• System activity logs\n• User login history\n• Error and debug logs\n• Performance statistics", 
		[]string{"View", "Cancel"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showSystemInfo() tea.Cmd {
	dialog := NewDialog("System Information", 
		"Current system status:\n\n• Vision/3 BBS System\n• Version: 3.0 Beta\n• Uptime: 2 days, 14 hours\n• Users online: 3\n• Total users: 1,247", 
		[]string{"OK"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showStatistics() tea.Cmd {
	dialog := NewDialog("System Statistics", 
		"System usage statistics:\n\n• Total calls: 45,892\n• Messages posted: 12,445\n• Files uploaded: 3,892\n• Files downloaded: 28,445", 
		[]string{"Details", "OK"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showHelpContents() tea.Cmd {
	dialog := NewDialog("Help Contents", 
		"Vision/3 Configuration Help:\n\n• F1: Context-sensitive help\n• Navigate with arrow keys and Tab\n• Alt+Letter: Access menus\n• F10: Exit current screen\n• Esc: Cancel operations", 
		[]string{"OK"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showKeyboardHelp() tea.Cmd {
	dialog := NewDialog("Keyboard Shortcuts", 
		"Function Keys:\n• F1: Help\n• F2: Save\n• F3: Open\n• F10: Quit\n\nNavigation:\n• Tab/Shift+Tab: Move between controls\n• Arrow keys: Navigate lists and menus\n• Enter: Select/Confirm\n• Esc: Cancel", 
		[]string{"OK"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

func (a *Application) showAbout() tea.Cmd {
	dialog := NewDialog("About Vision/3", 
		"Vision/3 BBS Configuration Tool\n\nVersion 3.0 Beta\nCopyright © 2024\n\nA modern BBS system with classic\nTurbo Pascal IDE styling.\n\nBuilt with Go and Bubble Tea", 
		[]string{"OK"})
	dialog.Center(a.width, a.height)
	a.ShowDialog(dialog)
	return nil
}

// renderWelcomeScreen renders the main welcome screen
func (a *Application) renderWelcomeScreen() string {
	welcome := []string{
		"",
		"╔═══════════════════════════════════════════════════════════════════════════════╗",
		"║                            Vision/3 Configuration Tool                       ║",
		"║                                                                               ║",
		"║  Welcome to the Vision/3 BBS Configuration System. Use the menu bar above    ║",
		"║  to configure different aspects of your BBS system.                          ║",
		"║                                                                               ║",
		"║  Quick Start:                                                                 ║",
		"║    • Press Alt+F to access the File menu                                     ║",
		"║    • Press Alt+C to access the Config menu                                   ║",
		"║    • Press F1 for help                                                       ║",
		"║                                                                               ║",
		"║  Navigation:                                                                  ║",
		"║    • Use Tab and Shift+Tab to move between controls                          ║",
		"║    • Use arrow keys to navigate within lists and menus                       ║",
		"║    • Press Enter to select items                                             ║",
		"║    • Press Escape to cancel operations                                       ║",
		"║                                                                               ║",
		"╚═══════════════════════════════════════════════════════════════════════════════╝",
		"",
	}
	
	return CenterText(a.width, a.height-2, lipgloss.JoinVertical(lipgloss.Top, welcome...))
}

// SetView changes the current view
func (a *Application) SetView(view View) {
	a.currentView = view
}

// GetView returns the current view
func (a *Application) GetView() View {
	return a.currentView
}

// ShowDialog displays a dialog window
func (a *Application) ShowDialog(dialog *Dialog) {
	a.windowManager.AddWindow(dialog)
}

// CloseTopWindow closes the topmost window
func (a *Application) CloseTopWindow() {
	a.windowManager.CloseTopWindow()
}