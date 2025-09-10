package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stlalpha/vision3/internal/config"
)

var (
	configPath = flag.String("config", "configs", "Path to configuration directory")
	help       = flag.Bool("help", false, "Show help information")
)

type ConfigTool struct {
	app           *tview.Application
	pages         *tview.Pages
	configPath    string
	stringsConfig config.StringsConfig
}

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

	// Create TVI application
	app := tview.NewApplication()
	
	// Create config tool
	tool := &ConfigTool{
		app:           app,
		pages:         tview.NewPages(),
		configPath:    *configPath,
		stringsConfig: stringsConfig,
	}

	// Build UI
	tool.buildMainMenu()
	
	// Set root and run
	app.SetRoot(tool.pages, true)
	if err := app.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}

func (ct *ConfigTool) buildMainMenu() {
	// Create main menu list
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" ViSiON/3 BBS Configuration Tool ")
	list.SetTitleAlign(tview.AlignCenter)

	// Add main menu items
	list.AddItem("String Configuration", "Edit BBS text strings and prompts", 's', func() {
		ct.showStringConfigMenu()
	})
	list.AddItem("Area Management", "Configure message and file areas", 'a', func() {
		ct.showAreaManagementMenu()
	})
	list.AddItem("Door Configuration", "Set up external programs and games", 'd', func() {
		ct.showDoorConfigMenu()
	})
	list.AddItem("Node Monitoring", "Multi-node status and management", 'n', func() {
		ct.showNodeMonitoringMenu()
	})
	list.AddItem("System Settings", "General BBS configuration", 'y', func() {
		ct.showSystemSettingsMenu()
	})
	list.AddItem("Exit", "Exit configuration tool", 'x', func() {
		ct.app.Stop()
	})

	// Add status bar
	statusBar := tview.NewTextView()
	statusBar.SetText("Ready - Use arrow keys to navigate, Enter to select, F10 to exit")
	statusBar.SetTextAlign(tview.AlignCenter)
	statusBar.SetBorder(true)

	// Create main layout
	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(list, 0, 1, true)
	flex.AddItem(statusBar, 3, 0, false)

	// Add global key bindings
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyF10:
			ct.app.Stop()
			return nil
		case tcell.KeyEscape:
			ct.app.Stop()
			return nil
		}
		return event
	})

	ct.pages.AddPage("main", flex, true, true)
}

func (ct *ConfigTool) showStringConfigMenu() {
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" String Configuration ")
	list.SetTitleAlign(tview.AlignCenter)

	list.AddItem("System Name", "Configure BBS system name", '1', func() {
		ct.editSystemName()
	})
	list.AddItem("Welcome Messages", "Edit login and welcome text", '2', func() {
		ct.editWelcomeMessages()
	})
	list.AddItem("Menu Prompts", "Customize menu system prompts", '3', func() {
		ct.editMenuPrompts()
	})
	list.AddItem("Error Messages", "Configure system error messages", '4', func() {
		ct.editErrorMessages()
	})
	list.AddItem("Time/Date Formats", "Set display formatting options", '5', func() {
		ct.editTimeDateFormats()
	})
	list.AddItem("Color Definitions", "Define system color schemes", '6', func() {
		ct.editColorDefinitions()
	})
	list.AddItem("Back", "Return to main menu", 'b', func() {
		ct.pages.SwitchToPage("main")
	})

	// Add status and navigation
	statusBar := tview.NewTextView()
	statusBar.SetText("String Configuration - Select item to edit")
	statusBar.SetTextAlign(tview.AlignCenter)
	statusBar.SetBorder(true)

	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(list, 0, 1, true)
	flex.AddItem(statusBar, 3, 0, false)

	// Add escape key handling
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("main")
			return nil
		}
		return event
	})

	ct.pages.AddPage("strings", flex, true, false)
	ct.pages.SwitchToPage("strings")
}

func (ct *ConfigTool) editSystemName() {
	// Load current board name from config.json
	configFile := filepath.Join(ct.configPath, "config.json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		ct.showError("Failed to read config.json: " + err.Error())
		return
	}

	var configData map[string]interface{}
	if err := json.Unmarshal(data, &configData); err != nil {
		ct.showError("Failed to parse config.json: " + err.Error())
		return
	}

	currentName := ""
	if name, ok := configData["boardName"].(string); ok {
		currentName = name
	}

	// Create form
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Edit System Name ")
	form.SetTitleAlign(tview.AlignCenter)

	// Add input field
	form.AddInputField("System Name", currentName, 50, nil, nil)
	
	// Add buttons
	form.AddButton("Save", func() {
		// Get the new name
		newName := form.GetFormItemByLabel("System Name").(*tview.InputField).GetText()
		
		if newName == "" {
			ct.showError("System name cannot be empty")
			return
		}

		// Update config
		configData["boardName"] = newName
		
		// Save back to file
		updatedData, err := json.MarshalIndent(configData, "", "  ")
		if err != nil {
			ct.showError("Failed to marshal config: " + err.Error())
			return
		}

		if err := os.WriteFile(configFile, updatedData, 0644); err != nil {
			ct.showError("Failed to save config: " + err.Error())
			return
		}

		ct.showInfo("System name updated successfully!")
		ct.pages.SwitchToPage("strings")
	})
	
	form.AddButton("Cancel", func() {
		ct.pages.SwitchToPage("strings")
	})

	// Add escape key handling
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("strings")
			return nil
		}
		return event
	})

	ct.pages.AddPage("systemname", form, true, false)
	ct.pages.SwitchToPage("systemname")
}

func (ct *ConfigTool) editWelcomeMessages() {
	// Create form for welcome messages
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Edit Welcome Messages ")
	form.SetTitleAlign(tview.AlignCenter)

	// Add text areas for different welcome messages using actual StringsConfig fields
	form.AddTextArea("Welcome New User", ct.stringsConfig.WelcomeNewUser, 60, 5, 0, nil)
	form.AddTextArea("Login Now", ct.stringsConfig.LoginNow, 60, 5, 0, nil)
	form.AddTextArea("Connection String", ct.stringsConfig.ConnectionStr, 60, 3, 0, nil)
	
	// Add buttons
	form.AddButton("Save", func() {
		// Get the new values
		ct.stringsConfig.WelcomeNewUser = form.GetFormItemByLabel("Welcome New User").(*tview.TextArea).GetText()
		ct.stringsConfig.LoginNow = form.GetFormItemByLabel("Login Now").(*tview.TextArea).GetText() 
		ct.stringsConfig.ConnectionStr = form.GetFormItemByLabel("Connection String").(*tview.TextArea).GetText()
		
		// Save to file
		if err := ct.saveStringsConfig(); err != nil {
			ct.showError("Failed to save strings config: " + err.Error())
			return
		}

		ct.showInfo("Welcome messages updated successfully!")
		ct.pages.SwitchToPage("strings")
	})
	
	form.AddButton("Cancel", func() {
		ct.pages.SwitchToPage("strings")
	})

	// Add escape key handling
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("strings")
			return nil
		}
		return event
	})

	ct.pages.AddPage("welcomemsgs", form, true, false)
	ct.pages.SwitchToPage("welcomemsgs")
}

func (ct *ConfigTool) editMenuPrompts() {
	// Create form for menu prompts
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Edit Menu Prompts ")
	form.SetTitleAlign(tview.AlignCenter)

	// Add input fields for menu prompts using actual StringsConfig fields
	form.AddInputField("Default Prompt", ct.stringsConfig.DefPrompt, 50, nil, nil)
	form.AddInputField("Message Menu Prompt", ct.stringsConfig.MessageMenuPrompt, 50, nil, nil)
	form.AddInputField("Continue String", ct.stringsConfig.ContinueStr, 50, nil, nil)
	form.AddInputField("Pause String", ct.stringsConfig.PauseString, 50, nil, nil)
	
	// Add buttons
	form.AddButton("Save", func() {
		// Get the new values
		ct.stringsConfig.DefPrompt = form.GetFormItemByLabel("Default Prompt").(*tview.InputField).GetText()
		ct.stringsConfig.MessageMenuPrompt = form.GetFormItemByLabel("Message Menu Prompt").(*tview.InputField).GetText()
		ct.stringsConfig.ContinueStr = form.GetFormItemByLabel("Continue String").(*tview.InputField).GetText()
		ct.stringsConfig.PauseString = form.GetFormItemByLabel("Pause String").(*tview.InputField).GetText()
		
		// Save to file
		if err := ct.saveStringsConfig(); err != nil {
			ct.showError("Failed to save strings config: " + err.Error())
			return
		}

		ct.showInfo("Menu prompts updated successfully!")
		ct.pages.SwitchToPage("strings")
	})
	
	form.AddButton("Cancel", func() {
		ct.pages.SwitchToPage("strings")
	})

	// Add escape key handling
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("strings")
			return nil
		}
		return event
	})

	ct.pages.AddPage("menuprompts", form, true, false)
	ct.pages.SwitchToPage("menuprompts")
}

func (ct *ConfigTool) editErrorMessages() {
	// Create form for error messages
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Edit Error Messages ")
	form.SetTitleAlign(tview.AlignCenter)

	// Add input fields for error messages using actual StringsConfig fields
	form.AddInputField("User Not Found", ct.stringsConfig.UserNotFound, 50, nil, nil)
	form.AddInputField("Wrong Password", ct.stringsConfig.WrongPassword, 50, nil, nil)
	form.AddInputField("Invalid Username", ct.stringsConfig.InvalidUserName, 50, nil, nil)
	form.AddInputField("Not Validated", ct.stringsConfig.NotValidated, 50, nil, nil)
	form.AddInputField("Wrong File Password", ct.stringsConfig.WrongFilePW, 50, nil, nil)
	
	// Add buttons
	form.AddButton("Save", func() {
		// Get the new values
		ct.stringsConfig.UserNotFound = form.GetFormItemByLabel("User Not Found").(*tview.InputField).GetText()
		ct.stringsConfig.WrongPassword = form.GetFormItemByLabel("Wrong Password").(*tview.InputField).GetText()
		ct.stringsConfig.InvalidUserName = form.GetFormItemByLabel("Invalid Username").(*tview.InputField).GetText()
		ct.stringsConfig.NotValidated = form.GetFormItemByLabel("Not Validated").(*tview.InputField).GetText()
		ct.stringsConfig.WrongFilePW = form.GetFormItemByLabel("Wrong File Password").(*tview.InputField).GetText()
		
		// Save to file
		if err := ct.saveStringsConfig(); err != nil {
			ct.showError("Failed to save strings config: " + err.Error())
			return
		}

		ct.showInfo("Error messages updated successfully!")
		ct.pages.SwitchToPage("strings")
	})
	
	form.AddButton("Cancel", func() {
		ct.pages.SwitchToPage("strings")
	})

	// Add escape key handling
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("strings")
			return nil
		}
		return event
	})

	ct.pages.AddPage("errormsgs", form, true, false)
	ct.pages.SwitchToPage("errormsgs")
}

func (ct *ConfigTool) editTimeDateFormats() {
	ct.showInfo("Time/Date formats are configured in theme.json - Coming soon!")
	ct.pages.SwitchToPage("strings")
}

func (ct *ConfigTool) editColorDefinitions() {
	// Create form for color definitions
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Edit Color Definitions ")
	form.SetTitleAlign(tview.AlignCenter)

	// Add input fields for the default color definitions that exist in StringsConfig
	form.AddInputField("Default Color 1", fmt.Sprintf("%d", ct.stringsConfig.DefColor1), 5, nil, nil)
	form.AddInputField("Default Color 2", fmt.Sprintf("%d", ct.stringsConfig.DefColor2), 5, nil, nil)
	form.AddInputField("Default Color 3", fmt.Sprintf("%d", ct.stringsConfig.DefColor3), 5, nil, nil)
	form.AddInputField("Default Color 4", fmt.Sprintf("%d", ct.stringsConfig.DefColor4), 5, nil, nil)
	form.AddInputField("Default Color 5", fmt.Sprintf("%d", ct.stringsConfig.DefColor5), 5, nil, nil)
	form.AddInputField("Default Color 6", fmt.Sprintf("%d", ct.stringsConfig.DefColor6), 5, nil, nil)
	form.AddInputField("Default Color 7", fmt.Sprintf("%d", ct.stringsConfig.DefColor7), 5, nil, nil)
	
	// Add help text
	help := tview.NewTextView()
	help.SetText("Color values: 0-255 (DOS color codes)\nExamples: 7 (white), 12 (bright red), 14 (yellow)\nThese map to |C1-|C7 pipe codes in menus")
	help.SetBorder(true)
	help.SetTitle(" Color Help ")
	
	// Add buttons
	form.AddButton("Save", func() {
		// Get the new values and convert to uint8
		if val := form.GetFormItemByLabel("Default Color 1").(*tview.InputField).GetText(); val != "" {
			if parsed, err := fmt.Sscanf(val, "%d", &ct.stringsConfig.DefColor1); err != nil || parsed != 1 {
				ct.showError("Invalid value for Default Color 1")
				return
			}
		}
		if val := form.GetFormItemByLabel("Default Color 2").(*tview.InputField).GetText(); val != "" {
			if parsed, err := fmt.Sscanf(val, "%d", &ct.stringsConfig.DefColor2); err != nil || parsed != 1 {
				ct.showError("Invalid value for Default Color 2")
				return
			}
		}
		if val := form.GetFormItemByLabel("Default Color 3").(*tview.InputField).GetText(); val != "" {
			if parsed, err := fmt.Sscanf(val, "%d", &ct.stringsConfig.DefColor3); err != nil || parsed != 1 {
				ct.showError("Invalid value for Default Color 3")
				return
			}
		}
		if val := form.GetFormItemByLabel("Default Color 4").(*tview.InputField).GetText(); val != "" {
			if parsed, err := fmt.Sscanf(val, "%d", &ct.stringsConfig.DefColor4); err != nil || parsed != 1 {
				ct.showError("Invalid value for Default Color 4")
				return
			}
		}
		if val := form.GetFormItemByLabel("Default Color 5").(*tview.InputField).GetText(); val != "" {
			if parsed, err := fmt.Sscanf(val, "%d", &ct.stringsConfig.DefColor5); err != nil || parsed != 1 {
				ct.showError("Invalid value for Default Color 5")
				return
			}
		}
		if val := form.GetFormItemByLabel("Default Color 6").(*tview.InputField).GetText(); val != "" {
			if parsed, err := fmt.Sscanf(val, "%d", &ct.stringsConfig.DefColor6); err != nil || parsed != 1 {
				ct.showError("Invalid value for Default Color 6")
				return
			}
		}
		if val := form.GetFormItemByLabel("Default Color 7").(*tview.InputField).GetText(); val != "" {
			if parsed, err := fmt.Sscanf(val, "%d", &ct.stringsConfig.DefColor7); err != nil || parsed != 1 {
				ct.showError("Invalid value for Default Color 7")
				return
			}
		}
		
		// Save to file
		if err := ct.saveStringsConfig(); err != nil {
			ct.showError("Failed to save strings config: " + err.Error())
			return
		}

		ct.showInfo("Color definitions updated successfully!")
		ct.pages.SwitchToPage("strings")
	})
	
	form.AddButton("Cancel", func() {
		ct.pages.SwitchToPage("strings")
	})

	// Create flex layout with form and help
	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(form, 0, 2, true)
	flex.AddItem(help, 4, 0, false)

	// Add escape key handling
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("strings")
			return nil
		}
		return event
	})

	ct.pages.AddPage("colors", flex, true, false)
	ct.pages.SwitchToPage("colors")
}

func (ct *ConfigTool) saveStringsConfig() error {
	stringsFile := filepath.Join(ct.configPath, "strings.json")
	data, err := json.MarshalIndent(ct.stringsConfig, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stringsFile, data, 0644)
}

func (ct *ConfigTool) showAreaManagementMenu() {
	ct.showInfo("Area Management - Coming soon!")
}

func (ct *ConfigTool) showDoorConfigMenu() {
	ct.showInfo("Door Configuration - Coming soon!")
}

func (ct *ConfigTool) showNodeMonitoringMenu() {
	ct.showInfo("Node Monitoring - Coming soon!")
}

func (ct *ConfigTool) showSystemSettingsMenu() {
	ct.showInfo("System Settings - Coming soon!")
}

func (ct *ConfigTool) showError(message string) {
	modal := tview.NewModal()
	modal.SetText("Error: " + message)
	modal.AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		ct.pages.RemovePage("modal")
	})
	ct.pages.AddPage("modal", modal, true, true)
}

func (ct *ConfigTool) showInfo(message string) {
	modal := tview.NewModal()
	modal.SetText(message)
	modal.AddButtons([]string{"OK"})
	modal.SetDoneFunc(func(buttonIndex int, buttonLabel string) {
		ct.pages.RemovePage("modal")
	})
	ct.pages.AddPage("modal", modal, true, true)
}

func showHelp() {
	fmt.Println("ViSiON/3 BBS Configuration Tool")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  vision3-config [options]")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -config <path>   Path to configuration directory (default: configs)")
	fmt.Println("  -help           Show this help message")
	fmt.Println("")
}