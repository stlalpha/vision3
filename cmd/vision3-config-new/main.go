package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/pkg/goturbotui"
	"github.com/stlalpha/vision3/pkg/goturbotui/components"
)

var (
	configPath = flag.String("config", "configs", "Path to configuration directory")
	help       = flag.Bool("help", false, "Show help information")
)

func main() {
	flag.Parse()

	if *help {
		showHelp()
		return
	}

	// Validate config path
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		log.Fatalf("Configuration directory does not exist: %s", *configPath)
	}

	// Load existing configuration
	stringsConfig, err := config.LoadStrings(*configPath)
	if err != nil {
		log.Printf("Warning: Could not load strings config: %v", err)
		stringsConfig = config.StringsConfig{}
	}

	// Create TUI application
	app := goturbotui.NewApplication()
	theme := goturbotui.DefaultTurboTheme()
	app.SetTheme(theme)

	// Create main window
	window := components.NewWindow("ViSiON/3 BBS Configuration Tool - "+filepath.Base(*configPath), theme)

	// Create menu bar
	menuBar := components.NewMenuBar(theme)
	menuBar.SetItems([]components.MenuItem{
		{Text: "File", Hotkey: 'f', Enabled: true},
		{Text: "Edit", Hotkey: 'e', Enabled: true},
		{Text: "Config", Hotkey: 'c', Enabled: true},
		{Text: "Tools", Hotkey: 't', Enabled: true},
		{Text: "Help", Hotkey: 'h', Enabled: true},
	})
	window.SetMenuBar(menuBar)

	// Create status bar
	statusBar := components.NewStatusBar(theme)
	statusBar.SetItems([]components.StatusItem{
		{Key: "F1", Text: "Help"},
		{Key: "F2", Text: "Save"},
		{Key: "F3", Text: "View"},
		{Key: "F10", Text: "Exit", Action: func() { app.Stop() }},
	})
	statusBar.SetMessage("Ready - Press F1 for Help")
	window.SetStatusBar(statusBar)

	// Create configuration app
	configApp := NewConfigApp(*configPath, stringsConfig, theme, statusBar)
	window.SetClient(configApp)

	// Set window as desktop
	app.SetDesktop(window)

	// Run application
	if err := app.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}

func showHelp() {
	// Same help function as before
	// ... (keeping it brief for space)
}

// ConfigApp represents the main configuration interface
type ConfigApp struct {
	*goturbotui.BaseView
	configPath    string
	stringsConfig config.StringsConfig
	theme         *goturbotui.Theme
	statusBar     *components.StatusBar
	currentDialog goturbotui.View
}

// NewConfigApp creates a new configuration application
func NewConfigApp(configPath string, stringsConfig config.StringsConfig, theme *goturbotui.Theme, statusBar *components.StatusBar) *ConfigApp {
	app := &ConfigApp{
		BaseView:      goturbotui.NewBaseView(),
		configPath:    configPath,
		stringsConfig: stringsConfig,
		theme:         theme,
		statusBar:     statusBar,
	}
	
	app.SetCanFocus(true)
	return app
}

// Draw renders the configuration app
func (ca *ConfigApp) Draw(canvas goturbotui.Canvas) {
	if !ca.IsVisible() {
		return
	}
	
	// Background is already filled by the window, just show dialogs
	
	// Show main configuration dialog if no other dialog is active
	if ca.currentDialog == nil {
		ca.showMainDialog(canvas)
	} else {
		ca.currentDialog.Draw(canvas)
	}
}

// showMainDialog creates and shows the main configuration categories dialog
func (ca *ConfigApp) showMainDialog(canvas goturbotui.Canvas) {
	// Create centered dialog
	dialogWidth := 76
	dialogHeight := 12
	dialog := components.NewDialog("Configuration Categories", dialogWidth, dialogHeight, ca.theme)
	
	// Center dialog
	parentWidth, parentHeight := canvas.Size()
	dialog.Center(parentWidth, parentHeight)
	
	// Create list box with configuration options
	listBox := components.NewListBox(ca.theme)
	listBox.SetItems([]string{
		"String Configuration     Edit BBS text strings and prompts",
		"Area Management         Configure message and file areas", 
		"Door Configuration      Set up external programs and games",
		"Node Monitoring         Multi-node status and management",
		"System Settings         General BBS configuration",
	})
	
	// Calculate exact positioning within dialog
	dialogBounds := dialog.GetBounds()
	// Content area is inside the dialog border: x+1, y+1, w-2, h-2
	listBounds := goturbotui.NewRect(
		dialogBounds.X + 2,  // Inside border + 1 space
		dialogBounds.Y + 2,  // Below title + 1 space  
		dialogBounds.W - 4,  // Account for borders and margins
		5,                   // Height for 5 items
	)
	listBox.SetBounds(listBounds)
	listBox.SetFocused(true)
	
	// Set up selection handler
	listBox.SetOnSelect(func(index int, item string) {
		ca.handleMainSelection(index)
	})
	
	dialog.AddChild(listBox)
	dialog.Draw(canvas)
	
	// Store as current dialog for event handling
	ca.currentDialog = dialog
}

// handleMainSelection handles selection from main configuration menu
func (ca *ConfigApp) handleMainSelection(index int) {
	switch index {
	case 0:
		ca.statusBar.SetMessage("String Configuration - Edit BBS text strings")
		// TODO: Show string configuration dialog
	case 1:
		ca.statusBar.SetMessage("Area Management - Configure message and file areas")
		// TODO: Show area management dialog
	case 2:
		ca.statusBar.SetMessage("Door Configuration - Set up external programs")
		// TODO: Show door configuration dialog
	case 3:
		ca.statusBar.SetMessage("Node Monitoring - Multi-node status and control")
		// TODO: Show node monitoring dialog
	case 4:
		ca.statusBar.SetMessage("System Settings - General BBS configuration")
		// TODO: Show system settings dialog
	}
}

// HandleEvent handles configuration app events
func (ca *ConfigApp) HandleEvent(event goturbotui.Event) bool {
	if !ca.IsVisible() {
		return false
	}
	
	// Pass events to current dialog
	if ca.currentDialog != nil {
		if ca.currentDialog.HandleEvent(event) {
			return true
		}
		
		// Handle escape to close dialog
		if event.Type == goturbotui.EventKey && event.Key.Code == goturbotui.KeyEscape {
			ca.currentDialog = nil
			ca.statusBar.SetMessage("Ready - Press F1 for Help")
			return true
		}
	}
	
	return false
}