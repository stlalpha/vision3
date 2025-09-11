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

// MessageArea represents a message area configuration
type MessageArea struct {
	ID           int    `json:"id"`
	Tag          string `json:"tag"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	ACSRead      string `json:"acs_read"`
	ACSWrite     string `json:"acs_write"`
	IsNetworked  bool   `json:"is_networked"`
	OriginNodeID string `json:"origin_node_id"`
}

// FileArea represents a file area configuration
type FileArea struct {
	ID          int    `json:"id"`
	Tag         string `json:"tag"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	ACSList     string `json:"acs_list"`
	ACSUpload   string `json:"acs_upload"`
	ACSDownload string `json:"acs_download"`
}

// Door represents a door/external program configuration
type Door struct {
	Name               string            `json:"name"`
	Command            string            `json:"command"`
	Args               []string          `json:"args"`
	WorkingDirectory   string            `json:"working_directory,omitempty"`
	DropfileType       string            `json:"dropfile_type"`
	IOMode             string            `json:"io_mode"`
	RequiresRawTerm    bool             `json:"requires_raw_terminal,omitempty"`
	EnvironmentVars    map[string]string `json:"environment_variables,omitempty"`
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
	
	// Create persistent footer
	footer := tview.NewTextView()
	footer.SetText("ViSiON/3 © 2025 Ruthless Enterprises")
	footer.SetTextAlign(tview.AlignCenter)
	footer.SetBorder(true)
	
	// Create main layout with persistent footer
	mainLayout := tview.NewFlex().SetDirection(tview.FlexRow)
	mainLayout.AddItem(tool.pages, 0, 1, true)
	mainLayout.AddItem(footer, 3, 0, false)
	
	// Set root and run
	app.SetRoot(mainLayout, true)
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

	// Add main menu items with numbered shortcuts
	list.AddItem("1. String Configuration", "Edit BBS text strings and prompts", '1', func() {
		ct.showStringConfigMenu()
	})
	list.AddItem("2. Area Management", "Configure message and file areas", '2', func() {
		ct.showAreaManagementMenu()
	})
	list.AddItem("3. Door Configuration", "Set up external programs and games", '3', func() {
		ct.showDoorConfigMenu()
	})
	list.AddItem("4. Node Monitoring", "Multi-node status and management", '4', func() {
		ct.showNodeMonitoringMenu()
	})
	list.AddItem("5. System Settings", "General BBS configuration", '5', func() {
		ct.showSystemSettingsMenu()
	})
	list.AddItem("X. Exit", "Exit configuration tool", 'x', func() {
		ct.app.Stop()
	})

	// Add status bar
	statusBar := tview.NewTextView()
	statusBar.SetText("Ready - Use numbers 1-5 or X to select, arrow keys to navigate, F10 to exit")
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

// showMainMenu switches to the main menu page
func (ct *ConfigTool) showMainMenu() {
	ct.pages.SwitchToPage("main")
}

// clearPages removes all pages except main
func (ct *ConfigTool) clearPages() {
	// Remove all pages except main
	pageNames := []string{"area-management", "message-areas", "file-areas", "door-config", "node-monitoring", "system-settings", "strings", "welcomemsgs", "menuprompts", "errormsgs", "allstrings"}
	for _, pageName := range pageNames {
		ct.pages.RemovePage(pageName)
	}
}

func (ct *ConfigTool) showStringConfigMenu() {
	list := tview.NewList()
	list.SetBorder(true)
	list.SetTitle(" String Configuration ")
	list.SetTitleAlign(tview.AlignCenter)

	list.AddItem("System Name", "Configure BBS system name", '1', func() {
		ct.editSystemName()
	})
	list.AddItem("All Strings Editor", "Edit all configurable text strings in one scrollable form", '2', func() {
		ct.editAllStrings()
	})
	list.AddItem("Welcome Messages", "Edit login and welcome text", '3', func() {
		ct.editWelcomeMessages()
	})
	list.AddItem("Menu Prompts", "Customize menu system prompts", '4', func() {
		ct.editMenuPrompts()
	})
	list.AddItem("Error Messages", "Configure system error messages", '5', func() {
		ct.editErrorMessages()
	})
	list.AddItem("Time/Date Formats", "Set display formatting options", '6', func() {
		ct.editTimeDateFormats()
	})
	list.AddItem("Color Definitions", "Define system color schemes", '7', func() {
		ct.editColorDefinitions()
	})
	list.AddItem("Back", "Return to main menu", 'b', func() {
		ct.pages.SwitchToPage("main")
	})

	// Add status and navigation
	statusBar := tview.NewTextView()
	statusBar.SetText("String Configuration - Use numbers 1-7 to select, B or Esc to go back")
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
	
	// Add buttons with shortcuts
	form.AddButton("S. Save", func() {
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
	
	form.AddButton("C. Cancel", func() {
		ct.pages.SwitchToPage("strings")
	})

	// Add keyboard shortcuts (S=Save, C=Cancel, Esc=Cancel)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("strings")
			return nil
		}
		if event.Rune() == 's' || event.Rune() == 'S' {
			// Trigger Save button
			newName := form.GetFormItemByLabel("System Name").(*tview.InputField).GetText()
			if newName == "" {
				ct.showError("System name cannot be empty")
				return nil
			}
			// Update config
			configData["boardName"] = newName
			// Save back to file
			updatedData, _ := json.MarshalIndent(configData, "", "  ")
			os.WriteFile(configFile, updatedData, 0644)
			ct.showInfo("System name updated successfully!")
			ct.pages.SwitchToPage("strings")
			return nil
		}
		if event.Rune() == 'c' || event.Rune() == 'C' {
			// Trigger Cancel button
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
	
	// Add buttons with shortcuts
	form.AddButton("S. Save", func() {
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
	
	form.AddButton("C. Cancel", func() {
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

	// Add keyboard shortcuts including arrow navigation
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			ct.pages.SwitchToPage("strings")
			return nil
		case tcell.KeyUp:
			// Navigate up through form fields
			currentItem, _ := form.GetFocusedItemIndex()
			if currentItem <= 0 {
				currentItem = form.GetFormItemCount() - 1
			} else {
				currentItem--
			}
			if currentItem >= 0 && currentItem < form.GetFormItemCount() {
				ct.app.SetFocus(form.GetFormItem(currentItem))
			}
			return nil
		case tcell.KeyDown:
			// Navigate down through form fields
			currentItem, _ := form.GetFocusedItemIndex()
			if currentItem < 0 || currentItem >= form.GetFormItemCount()-1 {
				currentItem = 0
			} else {
				currentItem++
			}
			if currentItem >= 0 && currentItem < form.GetFormItemCount() {
				ct.app.SetFocus(form.GetFormItem(currentItem))
			}
			return nil
		}
		
		switch event.Rune() {
		case 's', 'S':
			// Trigger Save
			ct.stringsConfig.WelcomeNewUser = welcomeField.GetRawText()
			ct.stringsConfig.LoginNow = loginField.GetRawText() 
			ct.stringsConfig.ConnectionStr = connField.GetRawText()
			ct.saveStringsConfig()
			ct.showInfo("Welcome messages updated successfully!")
			ct.pages.SwitchToPage("strings")
			return nil
		case 'c', 'C':
			// Trigger Cancel
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
	form.AddButton("S. Save", func() {
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
	
	form.AddButton("C. Cancel", func() {
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

	// Add keyboard shortcuts including arrow navigation
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			ct.pages.SwitchToPage("strings")
			return nil
		case tcell.KeyUp:
			// Navigate up through form fields
			currentItem, _ := form.GetFocusedItemIndex()
			if currentItem <= 0 {
				currentItem = form.GetFormItemCount() - 1
			} else {
				currentItem--
			}
			if currentItem >= 0 && currentItem < form.GetFormItemCount() {
				ct.app.SetFocus(form.GetFormItem(currentItem))
			}
			return nil
		case tcell.KeyDown:
			// Navigate down through form fields
			currentItem, _ := form.GetFocusedItemIndex()
			if currentItem < 0 || currentItem >= form.GetFormItemCount()-1 {
				currentItem = 0
			} else {
				currentItem++
			}
			if currentItem >= 0 && currentItem < form.GetFormItemCount() {
				ct.app.SetFocus(form.GetFormItem(currentItem))
			}
			return nil
		}
		
		switch event.Rune() {
		case 's', 'S':
			// Trigger Save
			ct.stringsConfig.DefPrompt = defPromptField.GetRawText()
			ct.stringsConfig.MessageMenuPrompt = msgPromptField.GetRawText()
			ct.stringsConfig.ContinueStr = contStrField.GetRawText()
			ct.stringsConfig.PauseString = pauseStrField.GetRawText()
			ct.saveStringsConfig()
			ct.showInfo("Menu prompts updated successfully!")
			ct.pages.SwitchToPage("strings")
			return nil
		case 'c', 'C':
			// Trigger Cancel
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
	form.AddButton("S. Save", func() {
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
	
	form.AddButton("C. Cancel", func() {
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

	// Add keyboard shortcuts including arrow navigation
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			ct.pages.SwitchToPage("strings")
			return nil
		case tcell.KeyUp:
			// Navigate up through form fields
			currentItem, _ := form.GetFocusedItemIndex()
			if currentItem <= 0 {
				currentItem = form.GetFormItemCount() - 1
			} else {
				currentItem--
			}
			if currentItem >= 0 && currentItem < form.GetFormItemCount() {
				ct.app.SetFocus(form.GetFormItem(currentItem))
			}
			return nil
		case tcell.KeyDown:
			// Navigate down through form fields
			currentItem, _ := form.GetFocusedItemIndex()
			if currentItem < 0 || currentItem >= form.GetFormItemCount()-1 {
				currentItem = 0
			} else {
				currentItem++
			}
			if currentItem >= 0 && currentItem < form.GetFormItemCount() {
				ct.app.SetFocus(form.GetFormItem(currentItem))
			}
			return nil
		}
		
		switch event.Rune() {
		case 's', 'S':
			// Trigger Save
			ct.stringsConfig.UserNotFound = userNotFoundField.GetRawText()
			ct.stringsConfig.WrongPassword = wrongPWField.GetRawText()
			ct.stringsConfig.InvalidUserName = invalidUserField.GetRawText()
			ct.stringsConfig.NotValidated = notValidField.GetRawText()
			ct.stringsConfig.WrongFilePW = wrongFilePWField.GetRawText()
			ct.saveStringsConfig()
			ct.showInfo("Error messages updated successfully!")
			ct.pages.SwitchToPage("strings")
			return nil
		case 'c', 'C':
			// Trigger Cancel
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
	form.AddButton("S. Save", func() {
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
	
	form.AddButton("C. Cancel", func() {
		ct.pages.SwitchToPage("strings")
	})

	// Create flex layout with form and help
	flex := tview.NewFlex()
	flex.SetDirection(tview.FlexRow)
	flex.AddItem(form, 0, 2, true)
	flex.AddItem(help, 4, 0, false)

	// Add keyboard shortcuts (S=Save, C=Cancel, Esc=Cancel)
	flex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ct.pages.SwitchToPage("strings")
			return nil
		}
		if event.Rune() == 's' || event.Rune() == 'S' {
			// Trigger Save - simplified version for now
			ct.saveStringsConfig()
			ct.showInfo("Color definitions updated successfully!")
			ct.pages.SwitchToPage("strings")
			return nil
		}
		if event.Rune() == 'c' || event.Rune() == 'C' {
			// Trigger Cancel
			ct.pages.SwitchToPage("strings")
			return nil
		}
		return event
	})

	ct.pages.AddPage("colors", flex, true, false)
	ct.pages.SwitchToPage("colors")
}

func (ct *ConfigTool) editAllStrings() {
	// Create the form that will hold all string fields
	form := tview.NewForm()
	form.SetBorder(true)
	form.SetTitle(" All Configurable Strings (200+ fields) ")
	form.SetTitleAlign(tview.AlignCenter)
	
	// Get all string fields
	stringFields := ct.getAllStringFields()
	allFields := make(map[string]*PipeCodeInputField)
	
	// Sort field names alphabetically for consistent display
	var sortedNames []string
	for fieldName := range stringFields {
		// Skip color fields - they're numbers, not strings
		if strings.HasPrefix(fieldName, "DefColor") {
			continue
		}
		sortedNames = append(sortedNames, fieldName)
	}
	
	// Simple bubble sort for field names
	for i := 0; i < len(sortedNames); i++ {
		for j := i + 1; j < len(sortedNames); j++ {
			if sortedNames[i] > sortedNames[j] {
				sortedNames[i], sortedNames[j] = sortedNames[j], sortedNames[i]
			}
		}
	}
	
	// Create input fields for all strings in alphabetical order
	fieldWidth := ct.getFieldWidth()
	for _, fieldName := range sortedNames {
		currentValue := stringFields[fieldName]
		label := ct.formatFieldLabel(fieldName)
		
		// Create pipe code input field
		field := ct.newPipeCodeInputField(label, currentValue, fieldWidth, ct.renderPipeCodes)
		allFields[fieldName] = field
		form.AddFormItem(field)
	}
	
	// Add action buttons at the bottom
	form.AddButton("S. Save All Changes", func() {
		ct.saveAllStringChanges(allFields)
	})
	
	form.AddButton("C. Cancel", func() {
		ct.pages.SwitchToPage("strings")
	})
	
	// Create help panel
	helpText := tview.NewTextView()
	helpText.SetBorder(true)
	helpText.SetTitle(" Pipe Code Help ")
	helpText.SetDynamicColors(true)
	helpText.SetWrap(true)
	helpText.SetText("All 200+ BBS text strings are listed here in alphabetical order. Scroll up/down to navigate through all fields.\n\nPipe Codes for Colors:\n[white]|00-|07[white:-] Dark colors  [white]|08-|15[white:-] Bright colors\n[white]|B0-|B7[white:-] Background colors\n\nExamples:\n[white]|09[white:-] [#FF5555]Bright Red[white:-]  [white]|14[white:-] [#55FFFF]Bright Cyan[white:-]\n[white]|B1[white:-] [#FFFFFF:#AA0000]Red Background[white:-]\n\nNavigation:\n• Scroll up/down through all fields\n• Click in fields to edit pipe codes\n• Click out to see color preview\n• S = Save All, C = Cancel, Esc = Cancel")
	
	// Create layout
	mainFlex := tview.NewFlex()
	if ct.screenWidth >= 120 {
		// Wide screen: form on left, help on right
		mainFlex.SetDirection(tview.FlexColumn)
		mainFlex.AddItem(form, 0, 2, true)
		mainFlex.AddItem(helpText, 45, 0, false)
	} else {
		// Narrow screen: form on top, help on bottom
		mainFlex.SetDirection(tview.FlexRow)
		mainFlex.AddItem(form, 0, 3, true)
		mainFlex.AddItem(helpText, 12, 0, false)
	}
	
	// Set up keyboard shortcuts
	mainFlex.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			ct.pages.SwitchToPage("strings")
			return nil
		case tcell.KeyCtrlS:
			ct.saveAllStringChanges(allFields)
			return nil
		}
		
		switch event.Rune() {
		case 's', 'S':
			if event.Modifiers() == tcell.ModNone {
				ct.saveAllStringChanges(allFields)
				return nil
			}
		case 'c', 'C':
			ct.pages.SwitchToPage("strings")
			return nil
		}
		return event
	})
	
	ct.pages.AddPage("allstrings", mainFlex, true, false)
	ct.pages.SwitchToPage("allstrings")
	
	// Focus the form for immediate scrolling
	ct.app.SetFocus(form)
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
	ct.clearPages()
	
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	
	// Header
	header := tview.NewTextView().
		SetText("[white:-:b]ViSiON/3 BBS Configuration Tool - Area Management[-:-]").
		SetTextAlign(tview.AlignCenter)
	header.SetBorder(true)
	
	// Menu options with selection tracking
	menu := tview.NewTextView().SetDynamicColors(true)
	menu.SetTitle(" Select Option ").SetBorder(true)
	
	menuItems := []string{
		"Message Areas",
		"File Areas",
		"Back to Main Menu",
		"Exit",
	}
	
	selectedIndex := 0
	maxIndex := len(menuItems) - 1
	
	updateMenu := func() {
		menuText := ""
		for i, item := range menuItems {
			if i == selectedIndex {
				// Highlighted selection
				if i < 2 {
					menuText += fmt.Sprintf("[black:white]%d. %s[-:-]\n", i+1, item)
				} else if i == 2 {
					menuText += "[black:white]B. " + item + "[-:-]\n"
				} else {
					menuText += "[black:white]ESC. " + item + "[-:-]\n"
				}
			} else {
				// Normal display
				if i < 2 {
					menuText += fmt.Sprintf("[white:-:b]%d.[-:-] %s\n", i+1, item)
				} else if i == 2 {
					menuText += "[white:-:b]B.[-:-] " + item + "\n"
				} else {
					menuText += "[white:-:b]ESC.[-:-] " + item + "\n"
				}
			}
			
			if i == 1 {
				menuText += "\n" // Add spacing after file areas
			}
		}
		menu.SetText(menuText)
	}
	
	updateMenu()
	
	// Footer
	footer := tview.NewTextView().
		SetText("ViSiON/3 © 2025 Ruthless Enterprises").
		SetTextAlign(tview.AlignCenter)
	
	flex.AddItem(header, 3, 0, false)
	flex.AddItem(menu, 0, 1, true)
	flex.AddItem(footer, 1, 0, false)
	
	// Handle navigation
	menu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if selectedIndex > 0 {
				selectedIndex--
				updateMenu()
			}
			return nil
		case tcell.KeyDown:
			if selectedIndex < maxIndex {
				selectedIndex++
				updateMenu()
			}
			return nil
		case tcell.KeyEnter:
			switch selectedIndex {
			case 0:
				ct.showMessageAreasMenu()
			case 1:
				ct.showFileAreasMenu()
			case 2:
				ct.showMainMenu()
			case 3:
				ct.app.Stop()
			}
			return nil
		case tcell.KeyEscape:
			ct.app.Stop()
			return nil
		}
		
		switch event.Rune() {
		case '1':
			selectedIndex = 0
			updateMenu()
			ct.showMessageAreasMenu()
			return nil
		case '2':
			selectedIndex = 1
			updateMenu()
			ct.showFileAreasMenu()
			return nil
		case 'b', 'B':
			selectedIndex = 2
			updateMenu()
			ct.showMainMenu()
			return nil
		}
		
		return event
	})
	
	ct.pages.AddPage("area-management", flex, true, true)
	ct.app.SetFocus(menu)
}

func (ct *ConfigTool) showMessageAreasMenu() {
	ct.clearPages()
	
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	
	// Header
	header := tview.NewTextView().
		SetText("[white:-:b]ViSiON/3 BBS Configuration Tool - Message Areas[-:-]").
		SetTextAlign(tview.AlignCenter)
	header.SetBorder(true)
	
	// Load message areas
	messageAreas, err := ct.loadMessageAreas()
	if err != nil {
		ct.showError("Failed to load message areas: " + err.Error())
		return
	}
	
	// Menu options
	menu := tview.NewTextView().SetDynamicColors(true)
	menu.SetTitle(" Message Areas ").SetBorder(true)
	
	menuText := "[white:-:b]Current Message Areas:[-:-]\n\n"
	for i, area := range messageAreas {
		menuText += fmt.Sprintf("[white:-:b]%d.[-:-] %s (%s)\n", i+1, area.Name, area.Tag)
	}
	
	menuText += "\n[white:-:b]A.[-:-] Add New Area\n"
	menuText += "[white:-:b]B.[-:-] Back to Area Management\n"
	menuText += "[white:-:b]ESC.[-:-] Exit"
	
	menu.SetText(menuText)
	
	// Footer
	footer := tview.NewTextView().
		SetText("ViSiON/3 © 2025 Ruthless Enterprises").
		SetTextAlign(tview.AlignCenter)
	
	flex.AddItem(header, 3, 0, false)
	flex.AddItem(menu, 0, 1, true)
	flex.AddItem(footer, 1, 0, false)
	
	// Handle navigation
	menu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'a', 'A':
			ct.showInfo("Add New Message Area - Coming soon!")
			return nil
		case 'b', 'B':
			ct.showAreaManagementMenu()
			return nil
		default:
			// Handle numbered area selection
			if event.Rune() >= '1' && event.Rune() <= '9' {
				areaIndex := int(event.Rune() - '1')
				if areaIndex < len(messageAreas) {
					ct.showInfo(fmt.Sprintf("Edit Message Area: %s - Coming soon!", messageAreas[areaIndex].Name))
				}
				return nil
			}
		}
		
		switch event.Key() {
		case tcell.KeyEscape:
			ct.app.Stop()
			return nil
		}
		
		return event
	})
	
	ct.pages.AddPage("message-areas", flex, true, true)
	ct.app.SetFocus(menu)
}

func (ct *ConfigTool) showFileAreasMenu() {
	ct.clearPages()
	
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	
	// Header
	header := tview.NewTextView().
		SetText("[white:-:b]ViSiON/3 BBS Configuration Tool - File Areas[-:-]").
		SetTextAlign(tview.AlignCenter)
	header.SetBorder(true)
	
	// Load file areas
	fileAreas, err := ct.loadFileAreas()
	if err != nil {
		ct.showError("Failed to load file areas: " + err.Error())
		return
	}
	
	// Menu options
	menu := tview.NewTextView().SetDynamicColors(true)
	menu.SetTitle(" File Areas ").SetBorder(true)
	
	menuText := "[white:-:b]Current File Areas:[-:-]\n\n"
	for i, area := range fileAreas {
		menuText += fmt.Sprintf("[white:-:b]%d.[-:-] %s (%s)\n", i+1, area.Name, area.Tag)
	}
	
	menuText += "\n[white:-:b]A.[-:-] Add New Area\n"
	menuText += "[white:-:b]B.[-:-] Back to Area Management\n"
	menuText += "[white:-:b]ESC.[-:-] Exit"
	
	menu.SetText(menuText)
	
	// Footer
	footer := tview.NewTextView().
		SetText("ViSiON/3 © 2025 Ruthless Enterprises").
		SetTextAlign(tview.AlignCenter)
	
	flex.AddItem(header, 3, 0, false)
	flex.AddItem(menu, 0, 1, true)
	flex.AddItem(footer, 1, 0, false)
	
	// Handle navigation
	menu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'a', 'A':
			ct.showInfo("Add New File Area - Coming soon!")
			return nil
		case 'b', 'B':
			ct.showAreaManagementMenu()
			return nil
		default:
			// Handle numbered area selection
			if event.Rune() >= '1' && event.Rune() <= '9' {
				areaIndex := int(event.Rune() - '1')
				if areaIndex < len(fileAreas) {
					ct.showInfo(fmt.Sprintf("Edit File Area: %s - Coming soon!", fileAreas[areaIndex].Name))
				}
				return nil
			}
		}
		
		switch event.Key() {
		case tcell.KeyEscape:
			ct.app.Stop()
			return nil
		}
		
		return event
	})
	
	ct.pages.AddPage("file-areas", flex, true, true)
	ct.app.SetFocus(menu)
}

func (ct *ConfigTool) showDoorConfigMenu() {
	ct.clearPages()
	
	// Load doors
	doors, err := ct.loadDoors()
	if err != nil {
		ct.showError("Failed to load door configuration: " + err.Error())
		return
	}
	
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	
	// Header
	header := tview.NewTextView().
		SetText("[white:-:b]ViSiON/3 BBS Configuration Tool - Door Configuration[-:-]").
		SetTextAlign(tview.AlignCenter)
	header.SetBorder(true)
	
	// Menu options with selection tracking
	menu := tview.NewTextView().SetDynamicColors(true)
	menu.SetTitle(" Door Programs ").SetBorder(true)
	
	// Build menu items dynamically based on loaded doors
	menuItems := []string{}
	for _, door := range doors {
		menuItems = append(menuItems, fmt.Sprintf("%s (%s)", door.Name, door.Command))
	}
	menuItems = append(menuItems, "Add New Door", "Back to Main Menu", "Exit")
	
	selectedIndex := 0
	maxIndex := len(menuItems) - 1
	
	updateMenu := func() {
		menuText := "[white:-:b]Current Door Programs:[-:-]\n\n"
		for i, item := range menuItems {
			if i == selectedIndex {
				// Highlighted selection
				if i < len(doors) {
					menuText += fmt.Sprintf("[black:white]%d. %s[-:-]\n", i+1, item)
				} else if i == len(doors) {
					menuText += "[black:white]A. " + item + "[-:-]\n"
				} else if i == len(doors)+1 {
					menuText += "[black:white]B. " + item + "[-:-]\n"
				} else {
					menuText += "[black:white]ESC. " + item + "[-:-]\n"
				}
			} else {
				// Normal display
				if i < len(doors) {
					menuText += fmt.Sprintf("[white:-:b]%d.[-:-] %s\n", i+1, item)
				} else if i == len(doors) {
					menuText += "[white:-:b]A.[-:-] " + item + "\n"
				} else if i == len(doors)+1 {
					menuText += "[white:-:b]B.[-:-] " + item + "\n"
				} else {
					menuText += "[white:-:b]ESC.[-:-] " + item + "\n"
				}
			}
			
			if i == len(doors)-1 {
				menuText += "\n" // Add spacing after door list
			}
		}
		menu.SetText(menuText)
	}
	
	updateMenu()
	
	// Footer
	footer := tview.NewTextView().
		SetText("ViSiON/3 © 2025 Ruthless Enterprises").
		SetTextAlign(tview.AlignCenter)
	
	flex.AddItem(header, 3, 0, false)
	flex.AddItem(menu, 0, 1, true)
	flex.AddItem(footer, 1, 0, false)
	
	// Handle navigation
	menu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if selectedIndex > 0 {
				selectedIndex--
				updateMenu()
			}
			return nil
		case tcell.KeyDown:
			if selectedIndex < maxIndex {
				selectedIndex++
				updateMenu()
			}
			return nil
		case tcell.KeyEnter:
			if selectedIndex < len(doors) {
				ct.showInfo(fmt.Sprintf("Edit Door: %s - Coming soon!", doors[selectedIndex].Name))
			} else if selectedIndex == len(doors) {
				ct.showInfo("Add New Door - Coming soon!")
			} else if selectedIndex == len(doors)+1 {
				ct.showMainMenu()
			} else {
				ct.app.Stop()
			}
			return nil
		case tcell.KeyEscape:
			ct.app.Stop()
			return nil
		}
		
		switch event.Rune() {
		case 'a', 'A':
			selectedIndex = len(doors)
			updateMenu()
			ct.showInfo("Add New Door - Coming soon!")
			return nil
		case 'b', 'B':
			selectedIndex = len(doors) + 1
			updateMenu()
			ct.showMainMenu()
			return nil
		default:
			// Handle numbered door selection
			if event.Rune() >= '1' && event.Rune() <= '9' {
				doorIndex := int(event.Rune() - '1')
				if doorIndex < len(doors) {
					selectedIndex = doorIndex
					updateMenu()
					ct.showInfo(fmt.Sprintf("Edit Door: %s - Coming soon!", doors[doorIndex].Name))
				}
				return nil
			}
		}
		
		return event
	})
	
	ct.pages.AddPage("door-config", flex, true, true)
	ct.app.SetFocus(menu)
}

func (ct *ConfigTool) showNodeMonitoringMenu() {
	ct.clearPages()
	
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	
	// Header
	header := tview.NewTextView().
		SetText("[white:-:b]ViSiON/3 BBS Configuration Tool - Node Monitoring[-:-]").
		SetTextAlign(tview.AlignCenter)
	header.SetBorder(true)
	
	// Menu options with selection tracking
	menu := tview.NewTextView().SetDynamicColors(true)
	menu.SetTitle(" Node Monitoring Options ").SetBorder(true)
	
	menuItems := []string{
		"View Active Connections",
		"Connection History",
		"System Resource Usage", 
		"Log Viewer",
		"Performance Statistics",
		"Back to Main Menu",
		"Exit",
	}
	
	selectedIndex := 0
	maxIndex := len(menuItems) - 1
	
	updateMenu := func() {
		menuText := ""
		for i, item := range menuItems {
			if i == selectedIndex {
				// Highlighted selection
				if i < 5 {
					menuText += fmt.Sprintf("[black:white]%d. %s[-:-]\n", i+1, item)
				} else if i == 5 {
					menuText += "[black:white]B. " + item + "[-:-]\n"
				} else {
					menuText += "[black:white]ESC. " + item + "[-:-]\n"
				}
			} else {
				// Normal display
				if i < 5 {
					menuText += fmt.Sprintf("[white:-:b]%d.[-:-] %s\n", i+1, item)
				} else if i == 5 {
					menuText += "[white:-:b]B.[-:-] " + item + "\n"
				} else {
					menuText += "[white:-:b]ESC.[-:-] " + item + "\n"
				}
			}
			
			if i == 4 {
				menuText += "\n" // Add spacing after monitoring options
			}
		}
		menu.SetText(menuText)
	}
	
	updateMenu()
	
	// Footer
	footer := tview.NewTextView().
		SetText("ViSiON/3 © 2025 Ruthless Enterprises").
		SetTextAlign(tview.AlignCenter)
	
	flex.AddItem(header, 3, 0, false)
	flex.AddItem(menu, 0, 1, true)
	flex.AddItem(footer, 1, 0, false)
	
	// Handle navigation
	menu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if selectedIndex > 0 {
				selectedIndex--
				updateMenu()
			}
			return nil
		case tcell.KeyDown:
			if selectedIndex < maxIndex {
				selectedIndex++
				updateMenu()
			}
			return nil
		case tcell.KeyEnter:
			switch selectedIndex {
			case 0:
				ct.showInfo("View Active Connections - Coming soon!")
			case 1:
				ct.showInfo("Connection History - Coming soon!")
			case 2:
				ct.showInfo("System Resource Usage - Coming soon!")
			case 3:
				ct.showInfo("Log Viewer - Coming soon!")
			case 4:
				ct.showInfo("Performance Statistics - Coming soon!")
			case 5:
				ct.showMainMenu()
			case 6:
				ct.app.Stop()
			}
			return nil
		case tcell.KeyEscape:
			ct.app.Stop()
			return nil
		}
		
		switch event.Rune() {
		case '1':
			selectedIndex = 0
			updateMenu()
			ct.showInfo("View Active Connections - Coming soon!")
			return nil
		case '2':
			selectedIndex = 1
			updateMenu()
			ct.showInfo("Connection History - Coming soon!")
			return nil
		case '3':
			selectedIndex = 2
			updateMenu()
			ct.showInfo("System Resource Usage - Coming soon!")
			return nil
		case '4':
			selectedIndex = 3
			updateMenu()
			ct.showInfo("Log Viewer - Coming soon!")
			return nil
		case '5':
			selectedIndex = 4
			updateMenu()
			ct.showInfo("Performance Statistics - Coming soon!")
			return nil
		case 'b', 'B':
			selectedIndex = 5
			updateMenu()
			ct.showMainMenu()
			return nil
		}
		
		return event
	})
	
	ct.pages.AddPage("node-monitoring", flex, true, true)
	ct.app.SetFocus(menu)
}

func (ct *ConfigTool) showSystemSettingsMenu() {
	ct.clearPages()
	
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	
	// Header
	header := tview.NewTextView().
		SetText("[white:-:b]ViSiON/3 BBS Configuration Tool - System Settings[-:-]").
		SetTextAlign(tview.AlignCenter)
	header.SetBorder(true)
	
	// Menu options with selection tracking
	menu := tview.NewTextView().SetDynamicColors(true)
	menu.SetTitle(" System Configuration ").SetBorder(true)
	
	menuItems := []string{
		"Network Configuration",
		"Security Settings",
		"File Transfer Protocols",
		"Terminal Settings", 
		"Logging Configuration",
		"System Limits",
		"Menu System Settings",
		"Back to Main Menu",
		"Exit",
	}
	
	selectedIndex := 0
	maxIndex := len(menuItems) - 1
	
	updateMenu := func() {
		menuText := ""
		for i, item := range menuItems {
			if i == selectedIndex {
				// Highlighted selection
				if i < 7 {
					menuText += fmt.Sprintf("[black:white]%d. %s[-:-]\n", i+1, item)
				} else if i == 7 {
					menuText += "[black:white]B. " + item + "[-:-]\n"
				} else {
					menuText += "[black:white]ESC. " + item + "[-:-]\n"
				}
			} else {
				// Normal display
				if i < 7 {
					menuText += fmt.Sprintf("[white:-:b]%d.[-:-] %s\n", i+1, item)
				} else if i == 7 {
					menuText += "[white:-:b]B.[-:-] " + item + "\n"
				} else {
					menuText += "[white:-:b]ESC.[-:-] " + item + "\n"
				}
			}
			
			if i == 6 {
				menuText += "\n" // Add spacing after system options
			}
		}
		menu.SetText(menuText)
	}
	
	updateMenu()
	
	// Footer
	footer := tview.NewTextView().
		SetText("ViSiON/3 © 2025 Ruthless Enterprises").
		SetTextAlign(tview.AlignCenter)
	
	flex.AddItem(header, 3, 0, false)
	flex.AddItem(menu, 0, 1, true)
	flex.AddItem(footer, 1, 0, false)
	
	// Handle navigation
	menu.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyUp:
			if selectedIndex > 0 {
				selectedIndex--
				updateMenu()
			}
			return nil
		case tcell.KeyDown:
			if selectedIndex < maxIndex {
				selectedIndex++
				updateMenu()
			}
			return nil
		case tcell.KeyEnter:
			switch selectedIndex {
			case 0:
				ct.showInfo("Network Configuration - Coming soon!")
			case 1:
				ct.showInfo("Security Settings - Coming soon!")
			case 2:
				ct.showInfo("File Transfer Protocols - Coming soon!")
			case 3:
				ct.showInfo("Terminal Settings - Coming soon!")
			case 4:
				ct.showInfo("Logging Configuration - Coming soon!")
			case 5:
				ct.showInfo("System Limits - Coming soon!")
			case 6:
				ct.showInfo("Menu System Settings - Coming soon!")
			case 7:
				ct.showMainMenu()
			case 8:
				ct.app.Stop()
			}
			return nil
		case tcell.KeyEscape:
			ct.app.Stop()
			return nil
		}
		
		switch event.Rune() {
		case '1':
			selectedIndex = 0
			updateMenu()
			ct.showInfo("Network Configuration - Coming soon!")
			return nil
		case '2':
			selectedIndex = 1
			updateMenu()
			ct.showInfo("Security Settings - Coming soon!")
			return nil
		case '3':
			selectedIndex = 2
			updateMenu()
			ct.showInfo("File Transfer Protocols - Coming soon!")
			return nil
		case '4':
			selectedIndex = 3
			updateMenu()
			ct.showInfo("Terminal Settings - Coming soon!")
			return nil
		case '5':
			selectedIndex = 4
			updateMenu()
			ct.showInfo("Logging Configuration - Coming soon!")
			return nil
		case '6':
			selectedIndex = 5
			updateMenu()
			ct.showInfo("System Limits - Coming soon!")
			return nil
		case '7':
			selectedIndex = 6
			updateMenu()
			ct.showInfo("Menu System Settings - Coming soon!")
			return nil
		case 'b', 'B':
			selectedIndex = 7
			updateMenu()
			ct.showMainMenu()
			return nil
		}
		
		return event
	})
	
	ct.pages.AddPage("system-settings", flex, true, true)
	ct.app.SetFocus(menu)
}

// loadMessageAreas loads message areas from data directory
func (ct *ConfigTool) loadMessageAreas() ([]MessageArea, error) {
	dataPath := filepath.Join(filepath.Dir(ct.configPath), "data", "message_areas.json")
	
	data, err := os.ReadFile(dataPath)
	if err != nil {
		return nil, err
	}
	
	var areas []MessageArea
	err = json.Unmarshal(data, &areas)
	if err != nil {
		return nil, err
	}
	
	return areas, nil
}

// loadFileAreas loads file areas from configs directory
func (ct *ConfigTool) loadFileAreas() ([]FileArea, error) {
	filePath := filepath.Join(ct.configPath, "file_areas.json")
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var areas []FileArea
	err = json.Unmarshal(data, &areas)
	if err != nil {
		return nil, err
	}
	
	return areas, nil
}

// loadDoors loads door configurations from configs directory
func (ct *ConfigTool) loadDoors() ([]Door, error) {
	filePath := filepath.Join(ct.configPath, "doors.json")
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	
	var doors []Door
	err = json.Unmarshal(data, &doors)
	if err != nil {
		return nil, err
	}
	
	return doors, nil
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

// getAllStringFields uses reflection to get all string fields from StringsConfig
func (ct *ConfigTool) getAllStringFields() map[string]string {
	// We'll use reflection to get all string fields from the StringsConfig struct
	// For now, let's manually build this map since we know the structure
	fields := make(map[string]string)
	
	// All the string fields from StringsConfig
	fields["ConnectionStr"] = ct.stringsConfig.ConnectionStr
	fields["LockedBaudStr"] = ct.stringsConfig.LockedBaudStr
	fields["ApplyAsNewStr"] = ct.stringsConfig.ApplyAsNewStr
	fields["GetNupStr"] = ct.stringsConfig.GetNupStr
	fields["ChatRequestStr"] = ct.stringsConfig.ChatRequestStr
	fields["LeaveFBStr"] = ct.stringsConfig.LeaveFBStr
	fields["QuoteTitle"] = ct.stringsConfig.QuoteTitle
	fields["QuoteMessageStr"] = ct.stringsConfig.QuoteMessageStr
	fields["QuoteStartLine"] = ct.stringsConfig.QuoteStartLine
	fields["QuoteEndLine"] = ct.stringsConfig.QuoteEndLine
	fields["Erase5MsgsStr"] = ct.stringsConfig.Erase5MsgsStr
	fields["ChangeBoardStr"] = ct.stringsConfig.ChangeBoardStr
	fields["NewscanBoardStr"] = ct.stringsConfig.NewscanBoardStr
	fields["PostOnBoardStr"] = ct.stringsConfig.PostOnBoardStr
	fields["MsgTitleStr"] = ct.stringsConfig.MsgTitleStr
	fields["MsgToStr"] = ct.stringsConfig.MsgToStr
	fields["UploadMsgStr"] = ct.stringsConfig.UploadMsgStr
	fields["MsgAnonStr"] = ct.stringsConfig.MsgAnonStr
	fields["SlashStr"] = ct.stringsConfig.SlashStr
	fields["NewScanningStr"] = ct.stringsConfig.NewScanningStr
	fields["ChangeFileAreaStr"] = ct.stringsConfig.ChangeFileAreaStr
	fields["LogOffStr"] = ct.stringsConfig.LogOffStr
	fields["ChangeAutoMsgStr"] = ct.stringsConfig.ChangeAutoMsgStr
	fields["NewUserNameStr"] = ct.stringsConfig.NewUserNameStr
	fields["CreateAPassword"] = ct.stringsConfig.CreateAPassword
	fields["PauseString"] = ct.stringsConfig.PauseString
	fields["WhatsYourAlias"] = ct.stringsConfig.WhatsYourAlias
	fields["WhatsYourPw"] = ct.stringsConfig.WhatsYourPw
	fields["SysopWorkingStr"] = ct.stringsConfig.SysopWorkingStr
	fields["SysopInDos"] = ct.stringsConfig.SysopInDos
	fields["SystemPasswordStr"] = ct.stringsConfig.SystemPasswordStr
	fields["DefPrompt"] = ct.stringsConfig.DefPrompt
	fields["EnterChat"] = ct.stringsConfig.EnterChat
	fields["ExitChat"] = ct.stringsConfig.ExitChat
	fields["SysOpIsIn"] = ct.stringsConfig.SysOpIsIn
	fields["SysOpIsOut"] = ct.stringsConfig.SysOpIsOut
	fields["HeaderStr"] = ct.stringsConfig.HeaderStr
	fields["InfoformPrompt"] = ct.stringsConfig.InfoformPrompt
	fields["NewInfoFormPrompt"] = ct.stringsConfig.NewInfoFormPrompt
	fields["UserNotFound"] = ct.stringsConfig.UserNotFound
	fields["DesignNewPrompt"] = ct.stringsConfig.DesignNewPrompt
	fields["YourCurrentPrompt"] = ct.stringsConfig.YourCurrentPrompt
	fields["WantHotKeys"] = ct.stringsConfig.WantHotKeys
	fields["WantRumors"] = ct.stringsConfig.WantRumors
	fields["YourUserNum"] = ct.stringsConfig.YourUserNum
	fields["WelcomeNewUser"] = ct.stringsConfig.WelcomeNewUser
	fields["EnterNumberHeader"] = ct.stringsConfig.EnterNumberHeader
	fields["EnterNumber"] = ct.stringsConfig.EnterNumber
	fields["EnterUserNote"] = ct.stringsConfig.EnterUserNote
	fields["CurFileArea"] = ct.stringsConfig.CurFileArea
	fields["EnterRealName"] = ct.stringsConfig.EnterRealName
	fields["ReEnterPassword"] = ct.stringsConfig.ReEnterPassword
	fields["QuoteTop"] = ct.stringsConfig.QuoteTop
	fields["QuoteBottom"] = ct.stringsConfig.QuoteBottom
	fields["AskOneLiner"] = ct.stringsConfig.AskOneLiner
	fields["EnterOneLiner"] = ct.stringsConfig.EnterOneLiner
	fields["NewScanDateStr"] = ct.stringsConfig.NewScanDateStr
	fields["AddBatchPrompt"] = ct.stringsConfig.AddBatchPrompt
	fields["ListUsers"] = ct.stringsConfig.ListUsers
	fields["ViewArchivePrompt"] = ct.stringsConfig.ViewArchivePrompt
	fields["AreaMsgNewScan"] = ct.stringsConfig.AreaMsgNewScan
	fields["GetInfoPrompt"] = ct.stringsConfig.GetInfoPrompt
	fields["MsgNewScanPrompt"] = ct.stringsConfig.MsgNewScanPrompt
	fields["TypeFilePrompt"] = ct.stringsConfig.TypeFilePrompt
	fields["ConfPrompt"] = ct.stringsConfig.ConfPrompt
	fields["FileListPrompt"] = ct.stringsConfig.FileListPrompt
	fields["UploadFileStr"] = ct.stringsConfig.UploadFileStr
	fields["DownloadStr"] = ct.stringsConfig.DownloadStr
	fields["ListRange"] = ct.stringsConfig.ListRange
	fields["ContinueStr"] = ct.stringsConfig.ContinueStr
	fields["ViewWhichForm"] = ct.stringsConfig.ViewWhichForm
	fields["CheckingPhoneNum"] = ct.stringsConfig.CheckingPhoneNum
	fields["CheckingUserBase"] = ct.stringsConfig.CheckingUserBase
	fields["NameAlreadyUsed"] = ct.stringsConfig.NameAlreadyUsed
	fields["InvalidUserName"] = ct.stringsConfig.InvalidUserName
	fields["SysPwIs"] = ct.stringsConfig.SysPwIs
	fields["NotValidated"] = ct.stringsConfig.NotValidated
	fields["HaveMail"] = ct.stringsConfig.HaveMail
	fields["ReadMailNow"] = ct.stringsConfig.ReadMailNow
	fields["DeleteNotice"] = ct.stringsConfig.DeleteNotice
	fields["HaveFeedback"] = ct.stringsConfig.HaveFeedback
	fields["ReadFeedback"] = ct.stringsConfig.ReadFeedback
	fields["LoginNow"] = ct.stringsConfig.LoginNow
	fields["NewUsersWaiting"] = ct.stringsConfig.NewUsersWaiting
	fields["VoteOnNewUsers"] = ct.stringsConfig.VoteOnNewUsers
	fields["WrongPassword"] = ct.stringsConfig.WrongPassword
	fields["MessageMenuPrompt"] = ct.stringsConfig.MessageMenuPrompt
	
	// Continue with remaining fields...
	fields["AddBBSName"] = ct.stringsConfig.AddBBSName
	fields["AddBBSNumber"] = ct.stringsConfig.AddBBSNumber
	fields["AddBBSBaud"] = ct.stringsConfig.AddBBSBaud
	fields["AddBBSSoftware"] = ct.stringsConfig.AddBBSSoftware
	fields["AddExtendedBBSDescr"] = ct.stringsConfig.AddExtendedBBSDescr
	fields["BBSEntryAdded"] = ct.stringsConfig.BBSEntryAdded
	fields["ViewNextDescrip"] = ct.stringsConfig.ViewNextDescrip
	fields["JoinedMsgConf"] = ct.stringsConfig.JoinedMsgConf
	fields["JoinedFileConf"] = ct.stringsConfig.JoinedFileConf
	fields["WhosBeingVotedOn"] = ct.stringsConfig.WhosBeingVotedOn
	fields["NumYesVotes"] = ct.stringsConfig.NumYesVotes
	fields["NumNoVotes"] = ct.stringsConfig.NumNoVotes
	fields["NUVCommentHeader"] = ct.stringsConfig.NUVCommentHeader
	fields["EnterNUVCommentPrompt"] = ct.stringsConfig.EnterNUVCommentPrompt
	fields["NUVVotePrompt"] = ct.stringsConfig.NUVVotePrompt
	fields["YesVoteCast"] = ct.stringsConfig.YesVoteCast
	fields["NoVoteCast"] = ct.stringsConfig.NoVoteCast
	fields["NoNewUsersPending"] = ct.stringsConfig.NoNewUsersPending
	fields["EnterRumorTitle"] = ct.stringsConfig.EnterRumorTitle
	fields["AddRumorAnonymous"] = ct.stringsConfig.AddRumorAnonymous
	fields["EnterRumorLevel"] = ct.stringsConfig.EnterRumorLevel
	fields["EnterRumorPrompt"] = ct.stringsConfig.EnterRumorPrompt
	fields["RumorAdded"] = ct.stringsConfig.RumorAdded
	fields["ListRumorsPrompt"] = ct.stringsConfig.ListRumorsPrompt
	fields["SendMailToWho"] = ct.stringsConfig.SendMailToWho
	fields["CarbonCopyMail"] = ct.stringsConfig.CarbonCopyMail
	fields["NotifyEMail"] = ct.stringsConfig.NotifyEMail
	fields["EMailAnnouncement"] = ct.stringsConfig.EMailAnnouncement
	fields["SysOpNotHere"] = ct.stringsConfig.SysOpNotHere
	fields["ChatCostsHeader"] = ct.stringsConfig.ChatCostsHeader
	fields["StillWantToTry"] = ct.stringsConfig.StillWantToTry
	fields["NotEnoughFPPoints"] = ct.stringsConfig.NotEnoughFPPoints
	fields["ChatCallOff"] = ct.stringsConfig.ChatCallOff
	fields["ChatCallOn"] = ct.stringsConfig.ChatCallOn
	fields["FeedbackSent"] = ct.stringsConfig.FeedbackSent
	fields["YouHaveReadMail"] = ct.stringsConfig.YouHaveReadMail
	fields["DeleteMailNow"] = ct.stringsConfig.DeleteMailNow
	fields["CurrentMailNone"] = ct.stringsConfig.CurrentMailNone
	fields["CurrentMailWaiting"] = ct.stringsConfig.CurrentMailWaiting
	fields["PickMailHeader"] = ct.stringsConfig.PickMailHeader
	fields["ListTitleType"] = ct.stringsConfig.ListTitleType
	fields["NoMoreTitles"] = ct.stringsConfig.NoMoreTitles
	fields["ListTitlesToYou"] = ct.stringsConfig.ListTitlesToYou
	fields["SubDoesNotExist"] = ct.stringsConfig.SubDoesNotExist
	fields["MsgNewScanAborted"] = ct.stringsConfig.MsgNewScanAborted
	fields["MsgReadingPrompt"] = ct.stringsConfig.MsgReadingPrompt
	fields["CurrentSubNewScan"] = ct.stringsConfig.CurrentSubNewScan
	fields["JumpToMessageNum"] = ct.stringsConfig.JumpToMessageNum
	fields["PostingQWKMsg"] = ct.stringsConfig.PostingQWKMsg
	fields["TotalQWKAdded"] = ct.stringsConfig.TotalQWKAdded
	fields["SendQWKPacketPrompt"] = ct.stringsConfig.SendQWKPacketPrompt
	fields["ThreadWhichWay"] = ct.stringsConfig.ThreadWhichWay
	fields["AutoValidatingFile"] = ct.stringsConfig.AutoValidatingFile
	fields["FileIsWorth"] = ct.stringsConfig.FileIsWorth
	fields["GrantingUserFP"] = ct.stringsConfig.GrantingUserFP
	fields["FileIsOffline"] = ct.stringsConfig.FileIsOffline
	fields["CrashedFile"] = ct.stringsConfig.CrashedFile
	fields["BadBaudRate"] = ct.stringsConfig.BadBaudRate
	fields["UnvalidatedFile"] = ct.stringsConfig.UnvalidatedFile
	fields["SpecialFile"] = ct.stringsConfig.SpecialFile
	fields["NoDownloadsHere"] = ct.stringsConfig.NoDownloadsHere
	fields["PrivateFile"] = ct.stringsConfig.PrivateFile
	fields["FilePassword"] = ct.stringsConfig.FilePassword
	fields["WrongFilePW"] = ct.stringsConfig.WrongFilePW
	fields["FileNewScanPrompt"] = ct.stringsConfig.FileNewScanPrompt
	fields["InvalidArea"] = ct.stringsConfig.InvalidArea
	fields["UntaggingBatchFile"] = ct.stringsConfig.UntaggingBatchFile
	fields["FileExtractionPrompt"] = ct.stringsConfig.FileExtractionPrompt
	fields["BadUDRatio"] = ct.stringsConfig.BadUDRatio
	fields["BadUDKRatio"] = ct.stringsConfig.BadUDKRatio
	fields["ExceededDailyKBLimit"] = ct.stringsConfig.ExceededDailyKBLimit
	fields["FilePointCommision"] = ct.stringsConfig.FilePointCommision
	fields["SuccessfulDownload"] = ct.stringsConfig.SuccessfulDownload
	fields["FileCrashSave"] = ct.stringsConfig.FileCrashSave
	fields["InvalidFilename"] = ct.stringsConfig.InvalidFilename
	fields["AlreadyEnteredFilename"] = ct.stringsConfig.AlreadyEnteredFilename
	fields["FileAlreadyExists"] = ct.stringsConfig.FileAlreadyExists
	fields["EnterFileDescription"] = ct.stringsConfig.EnterFileDescription
	fields["ExtendedUploadSetup"] = ct.stringsConfig.ExtendedUploadSetup
	fields["ReEnterFileDescrip"] = ct.stringsConfig.ReEnterFileDescrip
	fields["NotifyIfDownloaded"] = ct.stringsConfig.NotifyIfDownloaded
	fields["FiftyFilesMaximum"] = ct.stringsConfig.FiftyFilesMaximum
	fields["YouCantDownloadHere"] = ct.stringsConfig.YouCantDownloadHere
	fields["FileAlreadyMarked"] = ct.stringsConfig.FileAlreadyMarked
	fields["NotEnoughFP"] = ct.stringsConfig.NotEnoughFP
	fields["FileAreaPassword"] = ct.stringsConfig.FileAreaPassword
	fields["QuotePrefix"] = ct.stringsConfig.QuotePrefix
	
	return fields
}

// formatFieldLabel converts a field name like "WelcomeNewUser" to "Welcome New User"
func (ct *ConfigTool) formatFieldLabel(fieldName string) string {
	// Add spaces before capital letters
	var result []rune
	for i, r := range fieldName {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, ' ')
		}
		result = append(result, r)
	}
	return string(result)
}

// saveAllStringChanges saves all the modified string values back to the StringsConfig
func (ct *ConfigTool) saveAllStringChanges(allFields map[string]*PipeCodeInputField) {
	// Update the StringsConfig with all the new values
	for fieldName, field := range allFields {
		newValue := field.GetRawText()
		ct.updateStringConfigField(fieldName, newValue)
	}
	
	// Save to file
	if err := ct.saveStringsConfig(); err != nil {
		ct.showError("Failed to save strings config: " + err.Error())
		return
	}
	
	ct.showInfo(fmt.Sprintf("Successfully updated %d string values!", len(allFields)))
	ct.pages.SwitchToPage("strings")
}

// updateStringConfigField updates a specific field in the StringsConfig struct
func (ct *ConfigTool) updateStringConfigField(fieldName, value string) {
	// This is a large switch statement to update each field
	switch fieldName {
	case "ConnectionStr":
		ct.stringsConfig.ConnectionStr = value
	case "LockedBaudStr":
		ct.stringsConfig.LockedBaudStr = value
	case "ApplyAsNewStr":
		ct.stringsConfig.ApplyAsNewStr = value
	case "GetNupStr":
		ct.stringsConfig.GetNupStr = value
	case "ChatRequestStr":
		ct.stringsConfig.ChatRequestStr = value
	case "LeaveFBStr":
		ct.stringsConfig.LeaveFBStr = value
	case "QuoteTitle":
		ct.stringsConfig.QuoteTitle = value
	case "QuoteMessageStr":
		ct.stringsConfig.QuoteMessageStr = value
	case "QuoteStartLine":
		ct.stringsConfig.QuoteStartLine = value
	case "QuoteEndLine":
		ct.stringsConfig.QuoteEndLine = value
	case "Erase5MsgsStr":
		ct.stringsConfig.Erase5MsgsStr = value
	case "ChangeBoardStr":
		ct.stringsConfig.ChangeBoardStr = value
	case "NewscanBoardStr":
		ct.stringsConfig.NewscanBoardStr = value
	case "PostOnBoardStr":
		ct.stringsConfig.PostOnBoardStr = value
	case "MsgTitleStr":
		ct.stringsConfig.MsgTitleStr = value
	case "MsgToStr":
		ct.stringsConfig.MsgToStr = value
	case "UploadMsgStr":
		ct.stringsConfig.UploadMsgStr = value
	case "MsgAnonStr":
		ct.stringsConfig.MsgAnonStr = value
	case "SlashStr":
		ct.stringsConfig.SlashStr = value
	case "NewScanningStr":
		ct.stringsConfig.NewScanningStr = value
	case "ChangeFileAreaStr":
		ct.stringsConfig.ChangeFileAreaStr = value
	case "LogOffStr":
		ct.stringsConfig.LogOffStr = value
	case "ChangeAutoMsgStr":
		ct.stringsConfig.ChangeAutoMsgStr = value
	case "NewUserNameStr":
		ct.stringsConfig.NewUserNameStr = value
	case "CreateAPassword":
		ct.stringsConfig.CreateAPassword = value
	case "PauseString":
		ct.stringsConfig.PauseString = value
	case "WhatsYourAlias":
		ct.stringsConfig.WhatsYourAlias = value
	case "WhatsYourPw":
		ct.stringsConfig.WhatsYourPw = value
	case "SysopWorkingStr":
		ct.stringsConfig.SysopWorkingStr = value
	case "SysopInDos":
		ct.stringsConfig.SysopInDos = value
	case "SystemPasswordStr":
		ct.stringsConfig.SystemPasswordStr = value
	case "DefPrompt":
		ct.stringsConfig.DefPrompt = value
	case "EnterChat":
		ct.stringsConfig.EnterChat = value
	case "ExitChat":
		ct.stringsConfig.ExitChat = value
	case "SysOpIsIn":
		ct.stringsConfig.SysOpIsIn = value
	case "SysOpIsOut":
		ct.stringsConfig.SysOpIsOut = value
	case "HeaderStr":
		ct.stringsConfig.HeaderStr = value
	case "InfoformPrompt":
		ct.stringsConfig.InfoformPrompt = value
	case "NewInfoFormPrompt":
		ct.stringsConfig.NewInfoFormPrompt = value
	case "UserNotFound":
		ct.stringsConfig.UserNotFound = value
	case "DesignNewPrompt":
		ct.stringsConfig.DesignNewPrompt = value
	case "YourCurrentPrompt":
		ct.stringsConfig.YourCurrentPrompt = value
	case "WantHotKeys":
		ct.stringsConfig.WantHotKeys = value
	case "WantRumors":
		ct.stringsConfig.WantRumors = value
	case "YourUserNum":
		ct.stringsConfig.YourUserNum = value
	case "WelcomeNewUser":
		ct.stringsConfig.WelcomeNewUser = value
	case "EnterNumberHeader":
		ct.stringsConfig.EnterNumberHeader = value
	case "EnterNumber":
		ct.stringsConfig.EnterNumber = value
	case "EnterUserNote":
		ct.stringsConfig.EnterUserNote = value
	case "CurFileArea":
		ct.stringsConfig.CurFileArea = value
	case "EnterRealName":
		ct.stringsConfig.EnterRealName = value
	case "ReEnterPassword":
		ct.stringsConfig.ReEnterPassword = value
	case "QuoteTop":
		ct.stringsConfig.QuoteTop = value
	case "QuoteBottom":
		ct.stringsConfig.QuoteBottom = value
	case "AskOneLiner":
		ct.stringsConfig.AskOneLiner = value
	case "EnterOneLiner":
		ct.stringsConfig.EnterOneLiner = value
	case "NewScanDateStr":
		ct.stringsConfig.NewScanDateStr = value
	case "AddBatchPrompt":
		ct.stringsConfig.AddBatchPrompt = value
	case "ListUsers":
		ct.stringsConfig.ListUsers = value
	case "ViewArchivePrompt":
		ct.stringsConfig.ViewArchivePrompt = value
	case "AreaMsgNewScan":
		ct.stringsConfig.AreaMsgNewScan = value
	case "GetInfoPrompt":
		ct.stringsConfig.GetInfoPrompt = value
	case "MsgNewScanPrompt":
		ct.stringsConfig.MsgNewScanPrompt = value
	case "TypeFilePrompt":
		ct.stringsConfig.TypeFilePrompt = value
	case "ConfPrompt":
		ct.stringsConfig.ConfPrompt = value
	case "FileListPrompt":
		ct.stringsConfig.FileListPrompt = value
	case "UploadFileStr":
		ct.stringsConfig.UploadFileStr = value
	case "DownloadStr":
		ct.stringsConfig.DownloadStr = value
	case "ListRange":
		ct.stringsConfig.ListRange = value
	case "ContinueStr":
		ct.stringsConfig.ContinueStr = value
	case "ViewWhichForm":
		ct.stringsConfig.ViewWhichForm = value
	case "CheckingPhoneNum":
		ct.stringsConfig.CheckingPhoneNum = value
	case "CheckingUserBase":
		ct.stringsConfig.CheckingUserBase = value
	case "NameAlreadyUsed":
		ct.stringsConfig.NameAlreadyUsed = value
	case "InvalidUserName":
		ct.stringsConfig.InvalidUserName = value
	case "SysPwIs":
		ct.stringsConfig.SysPwIs = value
	case "NotValidated":
		ct.stringsConfig.NotValidated = value
	case "HaveMail":
		ct.stringsConfig.HaveMail = value
	case "ReadMailNow":
		ct.stringsConfig.ReadMailNow = value
	case "DeleteNotice":
		ct.stringsConfig.DeleteNotice = value
	case "HaveFeedback":
		ct.stringsConfig.HaveFeedback = value
	case "ReadFeedback":
		ct.stringsConfig.ReadFeedback = value
	case "LoginNow":
		ct.stringsConfig.LoginNow = value
	case "NewUsersWaiting":
		ct.stringsConfig.NewUsersWaiting = value
	case "VoteOnNewUsers":
		ct.stringsConfig.VoteOnNewUsers = value
	case "WrongPassword":
		ct.stringsConfig.WrongPassword = value
	case "MessageMenuPrompt":
		ct.stringsConfig.MessageMenuPrompt = value
	case "AddBBSName":
		ct.stringsConfig.AddBBSName = value
	case "AddBBSNumber":
		ct.stringsConfig.AddBBSNumber = value
	case "AddBBSBaud":
		ct.stringsConfig.AddBBSBaud = value
	case "AddBBSSoftware":
		ct.stringsConfig.AddBBSSoftware = value
	case "AddExtendedBBSDescr":
		ct.stringsConfig.AddExtendedBBSDescr = value
	case "BBSEntryAdded":
		ct.stringsConfig.BBSEntryAdded = value
	case "ViewNextDescrip":
		ct.stringsConfig.ViewNextDescrip = value
	case "JoinedMsgConf":
		ct.stringsConfig.JoinedMsgConf = value
	case "JoinedFileConf":
		ct.stringsConfig.JoinedFileConf = value
	case "WhosBeingVotedOn":
		ct.stringsConfig.WhosBeingVotedOn = value
	case "NumYesVotes":
		ct.stringsConfig.NumYesVotes = value
	case "NumNoVotes":
		ct.stringsConfig.NumNoVotes = value
	case "NUVCommentHeader":
		ct.stringsConfig.NUVCommentHeader = value
	case "EnterNUVCommentPrompt":
		ct.stringsConfig.EnterNUVCommentPrompt = value
	case "NUVVotePrompt":
		ct.stringsConfig.NUVVotePrompt = value
	case "YesVoteCast":
		ct.stringsConfig.YesVoteCast = value
	case "NoVoteCast":
		ct.stringsConfig.NoVoteCast = value
	case "NoNewUsersPending":
		ct.stringsConfig.NoNewUsersPending = value
	case "EnterRumorTitle":
		ct.stringsConfig.EnterRumorTitle = value
	case "AddRumorAnonymous":
		ct.stringsConfig.AddRumorAnonymous = value
	case "EnterRumorLevel":
		ct.stringsConfig.EnterRumorLevel = value
	case "EnterRumorPrompt":
		ct.stringsConfig.EnterRumorPrompt = value
	case "RumorAdded":
		ct.stringsConfig.RumorAdded = value
	case "ListRumorsPrompt":
		ct.stringsConfig.ListRumorsPrompt = value
	case "SendMailToWho":
		ct.stringsConfig.SendMailToWho = value
	case "CarbonCopyMail":
		ct.stringsConfig.CarbonCopyMail = value
	case "NotifyEMail":
		ct.stringsConfig.NotifyEMail = value
	case "EMailAnnouncement":
		ct.stringsConfig.EMailAnnouncement = value
	case "SysOpNotHere":
		ct.stringsConfig.SysOpNotHere = value
	case "ChatCostsHeader":
		ct.stringsConfig.ChatCostsHeader = value
	case "StillWantToTry":
		ct.stringsConfig.StillWantToTry = value
	case "NotEnoughFPPoints":
		ct.stringsConfig.NotEnoughFPPoints = value
	case "ChatCallOff":
		ct.stringsConfig.ChatCallOff = value
	case "ChatCallOn":
		ct.stringsConfig.ChatCallOn = value
	case "FeedbackSent":
		ct.stringsConfig.FeedbackSent = value
	case "YouHaveReadMail":
		ct.stringsConfig.YouHaveReadMail = value
	case "DeleteMailNow":
		ct.stringsConfig.DeleteMailNow = value
	case "CurrentMailNone":
		ct.stringsConfig.CurrentMailNone = value
	case "CurrentMailWaiting":
		ct.stringsConfig.CurrentMailWaiting = value
	case "PickMailHeader":
		ct.stringsConfig.PickMailHeader = value
	case "ListTitleType":
		ct.stringsConfig.ListTitleType = value
	case "NoMoreTitles":
		ct.stringsConfig.NoMoreTitles = value
	case "ListTitlesToYou":
		ct.stringsConfig.ListTitlesToYou = value
	case "SubDoesNotExist":
		ct.stringsConfig.SubDoesNotExist = value
	case "MsgNewScanAborted":
		ct.stringsConfig.MsgNewScanAborted = value
	case "MsgReadingPrompt":
		ct.stringsConfig.MsgReadingPrompt = value
	case "CurrentSubNewScan":
		ct.stringsConfig.CurrentSubNewScan = value
	case "JumpToMessageNum":
		ct.stringsConfig.JumpToMessageNum = value
	case "PostingQWKMsg":
		ct.stringsConfig.PostingQWKMsg = value
	case "TotalQWKAdded":
		ct.stringsConfig.TotalQWKAdded = value
	case "SendQWKPacketPrompt":
		ct.stringsConfig.SendQWKPacketPrompt = value
	case "ThreadWhichWay":
		ct.stringsConfig.ThreadWhichWay = value
	case "AutoValidatingFile":
		ct.stringsConfig.AutoValidatingFile = value
	case "FileIsWorth":
		ct.stringsConfig.FileIsWorth = value
	case "GrantingUserFP":
		ct.stringsConfig.GrantingUserFP = value
	case "FileIsOffline":
		ct.stringsConfig.FileIsOffline = value
	case "CrashedFile":
		ct.stringsConfig.CrashedFile = value
	case "BadBaudRate":
		ct.stringsConfig.BadBaudRate = value
	case "UnvalidatedFile":
		ct.stringsConfig.UnvalidatedFile = value
	case "SpecialFile":
		ct.stringsConfig.SpecialFile = value
	case "NoDownloadsHere":
		ct.stringsConfig.NoDownloadsHere = value
	case "PrivateFile":
		ct.stringsConfig.PrivateFile = value
	case "FilePassword":
		ct.stringsConfig.FilePassword = value
	case "WrongFilePW":
		ct.stringsConfig.WrongFilePW = value
	case "FileNewScanPrompt":
		ct.stringsConfig.FileNewScanPrompt = value
	case "InvalidArea":
		ct.stringsConfig.InvalidArea = value
	case "UntaggingBatchFile":
		ct.stringsConfig.UntaggingBatchFile = value
	case "FileExtractionPrompt":
		ct.stringsConfig.FileExtractionPrompt = value
	case "BadUDRatio":
		ct.stringsConfig.BadUDRatio = value
	case "BadUDKRatio":
		ct.stringsConfig.BadUDKRatio = value
	case "ExceededDailyKBLimit":
		ct.stringsConfig.ExceededDailyKBLimit = value
	case "FilePointCommision":
		ct.stringsConfig.FilePointCommision = value
	case "SuccessfulDownload":
		ct.stringsConfig.SuccessfulDownload = value
	case "FileCrashSave":
		ct.stringsConfig.FileCrashSave = value
	case "InvalidFilename":
		ct.stringsConfig.InvalidFilename = value
	case "AlreadyEnteredFilename":
		ct.stringsConfig.AlreadyEnteredFilename = value
	case "FileAlreadyExists":
		ct.stringsConfig.FileAlreadyExists = value
	case "EnterFileDescription":
		ct.stringsConfig.EnterFileDescription = value
	case "ExtendedUploadSetup":
		ct.stringsConfig.ExtendedUploadSetup = value
	case "ReEnterFileDescrip":
		ct.stringsConfig.ReEnterFileDescrip = value
	case "NotifyIfDownloaded":
		ct.stringsConfig.NotifyIfDownloaded = value
	case "FiftyFilesMaximum":
		ct.stringsConfig.FiftyFilesMaximum = value
	case "YouCantDownloadHere":
		ct.stringsConfig.YouCantDownloadHere = value
	case "FileAlreadyMarked":
		ct.stringsConfig.FileAlreadyMarked = value
	case "NotEnoughFP":
		ct.stringsConfig.NotEnoughFP = value
	case "FileAreaPassword":
		ct.stringsConfig.FileAreaPassword = value
	case "QuotePrefix":
		ct.stringsConfig.QuotePrefix = value
	}
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