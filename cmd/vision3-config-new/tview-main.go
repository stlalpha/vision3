package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	screenWidth   int
	screenHeight  int
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
	
	// Get initial screen size (default to reasonable values)
	screenWidth, screenHeight := 120, 30
	
	// Create config tool
	tool := &ConfigTool{
		app:           app,
		pages:         tview.NewPages(),
		configPath:    *configPath,
		stringsConfig: stringsConfig,
		screenWidth:   screenWidth,
		screenHeight:  screenHeight,
	}

	// Build UI
	tool.buildMainMenu()
	
	// Set up resize handler to update screen dimensions
	app.SetBeforeDrawFunc(func(screen tcell.Screen) bool {
		if screen != nil {
			w, h := screen.Size()
			if w != tool.screenWidth || h != tool.screenHeight {
				tool.screenWidth = w
				tool.screenHeight = h
			}
		}
		return false
	})
	
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

	// Add input field with responsive width
	form.AddInputField("System Name", currentName, ct.getFieldWidth(), nil, nil)
	
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

// PipeCodeInputField creates a custom component that shows rendered text when not focused
type PipeCodeInputField struct {
	*tview.Flex
	inputField   *tview.InputField
	textView     *tview.TextView
	rawText      string
	renderFunc   func(string) string
	label        string
	isEditing    bool
}

func (ct *ConfigTool) newPipeCodeInputField(label, text string, width int, renderFunc func(string) string) *PipeCodeInputField {
	// Create the input field for editing
	inputField := tview.NewInputField().
		SetLabel(label + ": ").
		SetFieldWidth(width).
		SetText(text)
	
	// Create the text view for display
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetWrap(false)
	
	// Create the flex container
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	
	field := &PipeCodeInputField{
		Flex:       flex,
		inputField: inputField,
		textView:   textView,
		rawText:    text,
		renderFunc: renderFunc,
		label:      label,
		isEditing:  false,
	}
	
	// Show rendered text initially
	field.showRendered()
	
	// Set up focus handlers for the input field
	inputField.SetFocusFunc(func() {
		field.showRaw()
	})
	
	inputField.SetBlurFunc(func() {
		field.rawText = inputField.GetText()
		field.showRendered()
	})
	
	inputField.SetChangedFunc(func(text string) {
		field.rawText = text
	})
	
	// Set up focus delegation from the Flex to the appropriate child
	flex.SetFocusFunc(func() {
		if field.isEditing {
			// Focus the input field when editing
			ct.app.SetFocus(inputField)
		} else {
			// Switch to editing mode when focused
			field.showRaw()
			ct.app.SetFocus(inputField)
		}
	})
	
	// Add mouse click handling to the text view to start editing
	textView.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		if action == tview.MouseLeftClick {
			field.showRaw()
			ct.app.SetFocus(inputField)
			return tview.MouseConsumed, nil
		}
		return action, event
	})
	
	// Handle key events for the flex container
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEnter || event.Key() == tcell.KeyTab {
			if !field.isEditing {
				field.showRaw()
				ct.app.SetFocus(inputField)
				return nil
			}
		}
		return event
	})
	
	return field
}

func (pcif *PipeCodeInputField) showRendered() {
	if pcif.renderFunc != nil {
		rendered := pcif.renderFunc(pcif.rawText)
		pcif.textView.SetText(fmt.Sprintf("%s: %s", pcif.label, rendered))
		
		// Clear and show text view
		pcif.Clear()
		pcif.AddItem(pcif.textView, 1, 0, true)
		pcif.isEditing = false
	}
}

func (pcif *PipeCodeInputField) showRaw() {
	// Update the input field with current raw text
	pcif.inputField.SetText(pcif.rawText)
	
	// Clear and show input field
	pcif.Clear()
	pcif.AddItem(pcif.inputField, 1, 0, true)
	pcif.isEditing = true
}

func (pcif *PipeCodeInputField) GetRawText() string {
	return pcif.rawText
}

// FormItem interface implementation
func (pcif *PipeCodeInputField) GetLabel() string {
	return pcif.label
}

func (pcif *PipeCodeInputField) GetFieldWidth() int {
	return pcif.inputField.GetFieldWidth()
}

func (pcif *PipeCodeInputField) GetFieldHeight() int {
	return 1 // Always single line
}

func (pcif *PipeCodeInputField) SetFormAttributes(labelWidth int, labelColor, bgColor, fieldTextColor, fieldBgColor tcell.Color) tview.FormItem {
	// Apply to the input field when editing
	pcif.inputField.SetFormAttributes(labelWidth, labelColor, bgColor, fieldTextColor, fieldBgColor)
	return pcif
}

func (pcif *PipeCodeInputField) SetFinishedFunc(handler func(key tcell.Key)) tview.FormItem {
	// Apply to the input field
	pcif.inputField.SetFinishedFunc(handler)
	return pcif
}

func (pcif *PipeCodeInputField) SetDisabled(disabled bool) tview.FormItem {
	// Apply to the input field
	pcif.inputField.SetDisabled(disabled)
	return pcif
}

func (ct *ConfigTool) editWelcomeMessages() {
	// Create form for welcome messages
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Edit Welcome Messages ")
	form.SetTitleAlign(tview.AlignCenter)

	// Create pipe code input fields with responsive widths
	fieldWidth := ct.getFieldWidth()
	welcomeField := ct.newPipeCodeInputField("Welcome New User", ct.stringsConfig.WelcomeNewUser, fieldWidth, ct.renderPipeCodes)
	loginField := ct.newPipeCodeInputField("Login Now", ct.stringsConfig.LoginNow, fieldWidth, ct.renderPipeCodes)
	connField := ct.newPipeCodeInputField("Connection String", ct.stringsConfig.ConnectionStr, fieldWidth, ct.renderPipeCodes)
	
	// Add fields to form
	form.AddFormItem(welcomeField)
	form.AddFormItem(loginField)
	form.AddFormItem(connField)
	
	// Create help area
	helpText := tview.NewTextView()
	helpText.SetBorder(true)
	helpText.SetTitle(" Pipe Code Help ")
	helpText.SetDynamicColors(true)
	helpText.SetText("Foreground colors (Dark/Bright intensity):\n[white]|00[white:-] [#000000]Black[white:-]      [white]|08[white:-] [#555555]Dark Gray[white:-]\n[white]|01[white:-] [#AA0000]Red (Dark)[white:-]  [white]|09[white:-] [#FF5555]Bright Red[white:-]\n[white]|02[white:-] [#00AA00]Green (Dark)[white:-] [white]|10[white:-] [#55FF55]Bright Green[white:-]\n[white]|03[white:-] [#AA5500]Brown[white:-]      [white]|11[white:-] [#FFFF55]Yellow[white:-]\n[white]|04[white:-] [#0000AA]Blue (Dark)[white:-] [white]|12[white:-] [#5555FF]Bright Blue[white:-]\n[white]|05[white:-] [#AA00AA]Magenta[white:-]    [white]|13[white:-] [#FF55FF]Bright Magenta[white:-]\n[white]|06[white:-] [#00AAAA]Cyan (Dark)[white:-] [white]|14[white:-] [#55FFFF]Bright Cyan[white:-]\n[white]|07[white:-] [#AAAAAA]Gray[white:-]       [white]|15[white:-] [#FFFFFF]White[white:-]\n\nBackground colors:\n[white]|B0[white:-] [#FFFFFF:#000000]Black BG[white:-]  [white]|B1[white:-] [#FFFFFF:#AA0000]Red BG[white:-]   [white]|B2[white:-] [#000000:#00AA00]Green BG[white:-]  [white]|B3[white:-] [#000000:#AA5500]Brown BG[white:-]\n[white]|B4[white:-] [#FFFFFF:#0000AA]Blue BG[white:-]  [white]|B5[white:-] [#FFFFFF:#AA00AA]Magenta BG[white:-] [white]|B6[white:-] [#000000:#00AAAA]Cyan BG[white:-]  [white]|B7[white:-] [#000000:#AAAAAA]Gray BG[white:-]\n\nTip: Click in field to edit raw pipe codes, click out to see rendered preview")
	
	// Add buttons
	form.AddButton("Save", func() {
		// Get the raw values
		ct.stringsConfig.WelcomeNewUser = welcomeField.GetRawText()
		ct.stringsConfig.LoginNow = loginField.GetRawText() 
		ct.stringsConfig.ConnectionStr = connField.GetRawText()
		
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

	// Create responsive flex layout
	flex := tview.NewFlex()
	
	// On narrow screens, stack vertically; on wide screens, side by side
	if ct.screenWidth < 120 {
		// Vertical layout for narrow screens
		flex.SetDirection(tview.FlexRow)
		flex.AddItem(form, 0, 2, true)
		flex.AddItem(helpText, 0, 1, false)
	} else {
		// Horizontal layout for wide screens
		flex.SetDirection(tview.FlexColumn)
		formWidth := ct.getFieldWidth() + 10 // Form padding
		helpWidth := ct.getHelpWidth()
		flex.AddItem(form, formWidth, 0, true)
		flex.AddItem(helpText, helpWidth, 0, false)
	}

	// Add escape key handling
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("strings")
			return nil
		}
		return event
	})

	ct.pages.AddPage("welcomemsgs", flex, true, false)
	ct.pages.SwitchToPage("welcomemsgs")
}

func (ct *ConfigTool) editMenuPrompts() {
	// Create form for menu prompts
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Edit Menu Prompts ")
	form.SetTitleAlign(tview.AlignCenter)

	// Create pipe code input fields with responsive widths
	fieldWidth := ct.getFieldWidth()
	defPromptField := ct.newPipeCodeInputField("Default Prompt", ct.stringsConfig.DefPrompt, fieldWidth, ct.renderPipeCodes)
	msgPromptField := ct.newPipeCodeInputField("Message Menu Prompt", ct.stringsConfig.MessageMenuPrompt, fieldWidth, ct.renderPipeCodes)
	contStrField := ct.newPipeCodeInputField("Continue String", ct.stringsConfig.ContinueStr, fieldWidth, ct.renderPipeCodes)
	pauseStrField := ct.newPipeCodeInputField("Pause String", ct.stringsConfig.PauseString, fieldWidth, ct.renderPipeCodes)
	
	// Add fields to form
	form.AddFormItem(defPromptField)
	form.AddFormItem(msgPromptField)
	form.AddFormItem(contStrField)
	form.AddFormItem(pauseStrField)
	
	// Create help area
	helpText := tview.NewTextView()
	helpText.SetBorder(true)
	helpText.SetTitle(" Pipe Code Help ")
	helpText.SetDynamicColors(true)
	helpText.SetText("Foreground colors (Dark/Bright intensity):\n[white]|00[white:-] [#000000]Black[white:-]      [white]|08[white:-] [#555555]Dark Gray[white:-]\n[white]|01[white:-] [#AA0000]Red (Dark)[white:-]  [white]|09[white:-] [#FF5555]Bright Red[white:-]\n[white]|02[white:-] [#00AA00]Green (Dark)[white:-] [white]|10[white:-] [#55FF55]Bright Green[white:-]\n[white]|03[white:-] [#AA5500]Brown[white:-]      [white]|11[white:-] [#FFFF55]Yellow[white:-]\n[white]|04[white:-] [#0000AA]Blue (Dark)[white:-] [white]|12[white:-] [#5555FF]Bright Blue[white:-]\n[white]|05[white:-] [#AA00AA]Magenta[white:-]    [white]|13[white:-] [#FF55FF]Bright Magenta[white:-]\n[white]|06[white:-] [#00AAAA]Cyan (Dark)[white:-] [white]|14[white:-] [#55FFFF]Bright Cyan[white:-]\n[white]|07[white:-] [#AAAAAA]Gray[white:-]       [white]|15[white:-] [#FFFFFF]White[white:-]\n\nBackground colors:\n[white]|B0[white:-] [#FFFFFF:#000000]Black BG[white:-]  [white]|B1[white:-] [#FFFFFF:#AA0000]Red BG[white:-]   [white]|B2[white:-] [#000000:#00AA00]Green BG[white:-]  [white]|B3[white:-] [#000000:#AA5500]Brown BG[white:-]\n[white]|B4[white:-] [#FFFFFF:#0000AA]Blue BG[white:-]  [white]|B5[white:-] [#FFFFFF:#AA00AA]Magenta BG[white:-] [white]|B6[white:-] [#000000:#00AAAA]Cyan BG[white:-]  [white]|B7[white:-] [#000000:#AAAAAA]Gray BG[white:-]\n\nTip: Click in field to edit raw pipe codes, click out to see rendered preview")
	
	// Add buttons
	form.AddButton("Save", func() {
		// Get the raw values
		ct.stringsConfig.DefPrompt = defPromptField.GetRawText()
		ct.stringsConfig.MessageMenuPrompt = msgPromptField.GetRawText()
		ct.stringsConfig.ContinueStr = contStrField.GetRawText()
		ct.stringsConfig.PauseString = pauseStrField.GetRawText()
		
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

	// Create responsive flex layout
	flex := tview.NewFlex()
	
	// On narrow screens, stack vertically; on wide screens, side by side
	if ct.screenWidth < 120 {
		// Vertical layout for narrow screens
		flex.SetDirection(tview.FlexRow)
		flex.AddItem(form, 0, 2, true)
		flex.AddItem(helpText, 0, 1, false)
	} else {
		// Horizontal layout for wide screens
		flex.SetDirection(tview.FlexColumn)
		formWidth := ct.getFieldWidth() + 10 // Form padding
		helpWidth := ct.getHelpWidth()
		flex.AddItem(form, formWidth, 0, true)
		flex.AddItem(helpText, helpWidth, 0, false)
	}

	// Add escape key handling
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("strings")
			return nil
		}
		return event
	})

	ct.pages.AddPage("menuprompts", flex, true, false)
	ct.pages.SwitchToPage("menuprompts")
}

func (ct *ConfigTool) editErrorMessages() {
	// Create form for error messages
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" Edit Error Messages ")
	form.SetTitleAlign(tview.AlignCenter)

	// Create pipe code input fields with responsive widths
	fieldWidth := ct.getFieldWidth()
	userNotFoundField := ct.newPipeCodeInputField("User Not Found", ct.stringsConfig.UserNotFound, fieldWidth, ct.renderPipeCodes)
	wrongPWField := ct.newPipeCodeInputField("Wrong Password", ct.stringsConfig.WrongPassword, fieldWidth, ct.renderPipeCodes)
	invalidUserField := ct.newPipeCodeInputField("Invalid Username", ct.stringsConfig.InvalidUserName, fieldWidth, ct.renderPipeCodes)
	notValidField := ct.newPipeCodeInputField("Not Validated", ct.stringsConfig.NotValidated, fieldWidth, ct.renderPipeCodes)
	wrongFilePWField := ct.newPipeCodeInputField("Wrong File Password", ct.stringsConfig.WrongFilePW, fieldWidth, ct.renderPipeCodes)
	
	// Add fields to form
	form.AddFormItem(userNotFoundField)
	form.AddFormItem(wrongPWField)
	form.AddFormItem(invalidUserField)
	form.AddFormItem(notValidField)
	form.AddFormItem(wrongFilePWField)
	
	// Create help area
	helpText := tview.NewTextView()
	helpText.SetBorder(true)
	helpText.SetTitle(" Pipe Code Help ")
	helpText.SetDynamicColors(true)
	helpText.SetText("Foreground colors (Dark/Bright intensity):\n[white]|00[white:-] [#000000]Black[white:-]      [white]|08[white:-] [#555555]Dark Gray[white:-]\n[white]|01[white:-] [#AA0000]Red (Dark)[white:-]  [white]|09[white:-] [#FF5555]Bright Red[white:-]\n[white]|02[white:-] [#00AA00]Green (Dark)[white:-] [white]|10[white:-] [#55FF55]Bright Green[white:-]\n[white]|03[white:-] [#AA5500]Brown[white:-]      [white]|11[white:-] [#FFFF55]Yellow[white:-]\n[white]|04[white:-] [#0000AA]Blue (Dark)[white:-] [white]|12[white:-] [#5555FF]Bright Blue[white:-]\n[white]|05[white:-] [#AA00AA]Magenta[white:-]    [white]|13[white:-] [#FF55FF]Bright Magenta[white:-]\n[white]|06[white:-] [#00AAAA]Cyan (Dark)[white:-] [white]|14[white:-] [#55FFFF]Bright Cyan[white:-]\n[white]|07[white:-] [#AAAAAA]Gray[white:-]       [white]|15[white:-] [#FFFFFF]White[white:-]\n\nBackground colors:\n[white]|B0[white:-] [#FFFFFF:#000000]Black BG[white:-]  [white]|B1[white:-] [#FFFFFF:#AA0000]Red BG[white:-]   [white]|B2[white:-] [#000000:#00AA00]Green BG[white:-]  [white]|B3[white:-] [#000000:#AA5500]Brown BG[white:-]\n[white]|B4[white:-] [#FFFFFF:#0000AA]Blue BG[white:-]  [white]|B5[white:-] [#FFFFFF:#AA00AA]Magenta BG[white:-] [white]|B6[white:-] [#000000:#00AAAA]Cyan BG[white:-]  [white]|B7[white:-] [#000000:#AAAAAA]Gray BG[white:-]\n\nTip: Click in field to edit raw pipe codes, click out to see rendered preview")
	
	// Add buttons
	form.AddButton("Save", func() {
		// Get the raw values
		ct.stringsConfig.UserNotFound = userNotFoundField.GetRawText()
		ct.stringsConfig.WrongPassword = wrongPWField.GetRawText()
		ct.stringsConfig.InvalidUserName = invalidUserField.GetRawText()
		ct.stringsConfig.NotValidated = notValidField.GetRawText()
		ct.stringsConfig.WrongFilePW = wrongFilePWField.GetRawText()
		
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

	// Create responsive flex layout
	flex := tview.NewFlex()
	
	// On narrow screens, stack vertically; on wide screens, side by side
	if ct.screenWidth < 120 {
		// Vertical layout for narrow screens
		flex.SetDirection(tview.FlexRow)
		flex.AddItem(form, 0, 2, true)
		flex.AddItem(helpText, 0, 1, false)
	} else {
		// Horizontal layout for wide screens
		flex.SetDirection(tview.FlexColumn)
		formWidth := ct.getFieldWidth() + 10 // Form padding
		helpWidth := ct.getHelpWidth()
		flex.AddItem(form, formWidth, 0, true)
		flex.AddItem(helpText, helpWidth, 0, false)
	}

	// Add escape key handling
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("strings")
			return nil
		}
		return event
	})

	ct.pages.AddPage("errormsgs", flex, true, false)
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
	
	// Add help text with color examples
	help := tview.NewTextView()
	help.SetDynamicColors(true)
	help.SetText("Color values: 0-255 (DOS color codes)\n\nForeground Colors (Dark/Bright):\n[white]|01[white:-]/[white]|09[white:-] [red]Red (Dark)[white:-]/[lightred]Bright Red[white:-]   [white]|04[white:-]/[white]|12[white:-] [blue]Blue (Dark)[white:-]/[lightblue]Bright Blue[white:-]\n[white]|02[white:-]/[white]|10[white:-] [green]Green (Dark)[white:-]/[lightgreen]Bright Green[white:-] [white]|03[white:-]/[white]|11[white:-] [yellow]Brown[white:-]/[yellow]Yellow[white:-]\n[white]|05[white:-]/[white]|13[white:-] [magenta]Magenta[white:-]/[lightmagenta]Bright Magenta[white:-] [white]|06[white:-]/[white]|14[white:-] [cyan]Cyan (Dark)[white:-]/[lightcyan]Bright Cyan[white:-]\n[white]|00[white:-] [black]Black[white:-]  [white]|07[white:-] [white]Gray[white:-]  [white]|08[white:-] [gray]Dark Gray[white:-]  [white]|15[white:-] [white]White[white:-]\n\nBackground Colors:\n[white]|B0[white:-] Black BG [white]|B1[white:-] [black:red]Red BG[white:-] [white]|B2[white:-] [white:green]Green BG[white:-] [white]|B3[white:-] [black:yellow]Brown BG[white:-]\n[white]|B4[white:-] [white:blue]Blue BG[white:-] [white]|B5[white:-] [white:magenta]Magenta BG[white:-] [white]|B6[white:-] [black:cyan]Cyan BG[white:-] [white]|B7[white:-] [black:white]White BG[white:-]\n\nThese map to |C1-|C7 pipe codes in menus")
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

// renderPipeCodes converts ViSiON/3 pipe codes to tview color markup for preview
func (ct *ConfigTool) renderPipeCodes(text string) string {
	// ViSiON/3 pipe code mapping - using accurate tview color representations
	// Low intensity colors (|00-|07) - Dark variants  
	colorMap := map[string]string{
		"|00": "[#000000]",      // Black (Dark) - pure black
		"|01": "[#AA0000]",      // Red (Dark) - dark red
		"|02": "[#00AA00]",      // Green (Dark) - dark green
		"|03": "[#AA5500]",      // Brown/Yellow (Dark) - brownish
		"|04": "[#0000AA]",      // Blue (Dark) - dark blue  
		"|05": "[#AA00AA]",      // Magenta (Dark) - dark magenta
		"|06": "[#00AAAA]",      // Cyan (Dark) - dark cyan
		"|07": "[#AAAAAA]",      // Gray (Light Gray) - medium gray
		
		// High intensity colors (|08-|15) - Bright variants
		"|08": "[#555555]",      // Dark Gray (Bright Black) - darker gray
		"|09": "[#FF5555]",      // Bright Red - bright red
		"|10": "[#55FF55]",      // Bright Green - bright green
		"|11": "[#FFFF55]",      // Yellow (Bright) - bright yellow
		"|12": "[#5555FF]",      // Bright Blue - bright blue
		"|13": "[#FF55FF]",      // Bright Magenta - bright magenta
		"|14": "[#55FFFF]",      // Bright Cyan - bright cyan
		"|15": "[#FFFFFF]",      // White (Bright White) - pure white
		
		// Hex variants (same mapping as decimal)
		"|0A": "[#55FF55]",      // |0A = 10 decimal - Bright Green
		"|0B": "[#FFFF55]",      // |0B = 11 decimal - Yellow
		"|0C": "[#5555FF]",      // |0C = 12 decimal - Bright Blue  
		"|0D": "[#FF55FF]",      // |0D = 13 decimal - Bright Magenta
		"|0E": "[#55FFFF]",      // |0E = 14 decimal - Bright Cyan
		"|0F": "[#FFFFFF]",      // |0F = 15 decimal - White
	}
	
	// Background color mapping (|B0 - |B7) - using contrasting foreground colors
	backgroundMap := map[string]string{
		"|B0": "[#FFFFFF:#000000]", // White text on Black background
		"|B1": "[#FFFFFF:#AA0000]", // White text on Red background  
		"|B2": "[#000000:#00AA00]", // Black text on Green background
		"|B3": "[#000000:#AA5500]", // Black text on Brown background
		"|B4": "[#FFFFFF:#0000AA]", // White text on Blue background
		"|B5": "[#FFFFFF:#AA00AA]", // White text on Magenta background
		"|B6": "[#000000:#00AAAA]", // Black text on Cyan background
		"|B7": "[#000000:#AAAAAA]", // Black text on White/Gray background
	}
	
	// Apply background colors first (they set both fg and bg)
	for bgCode, bgColor := range backgroundMap {
		text = strings.ReplaceAll(text, bgCode, bgColor)
	}
	
	// Convert hex codes to decimal for easier parsing
	for hex, color := range colorMap {
		text = strings.ReplaceAll(text, hex, color)
	}
	
	// Handle decimal pipe codes |00 through |15
	for i := 0; i < 16; i++ {
		pipeCode := fmt.Sprintf("|%02d", i)
		if colorName, exists := colorMap[fmt.Sprintf("|%02X", i)]; exists {
			text = strings.ReplaceAll(text, pipeCode, colorName)
		}
	}
	
	return text + "[white:-]" // Reset to white with default background at end
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

// getFieldWidth calculates responsive field width based on screen size
func (ct *ConfigTool) getFieldWidth() int {
	// Use 60% of screen width, with min 30 and max 80
	fieldWidth := ct.screenWidth * 60 / 100
	if fieldWidth < 30 {
		fieldWidth = 30
	}
	if fieldWidth > 80 {
		fieldWidth = 80
	}
	return fieldWidth
}

// getHelpWidth calculates help panel width
func (ct *ConfigTool) getHelpWidth() int {
	// Use remaining space, minimum 40 characters
	remaining := ct.screenWidth - ct.getFieldWidth()
	if remaining < 40 {
		return 40
	}
	return remaining
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