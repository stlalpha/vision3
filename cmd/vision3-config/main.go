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
	pageNames := []string{"area-management", "message-areas", "file-areas", "door-config", "node-monitoring", "system-settings", "strings", "welcomemsgs", "menuprompts", "errormsgs"}
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
	statusBar.SetText("String Configuration - Use numbers 1-6 to select, B or Esc to go back")
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