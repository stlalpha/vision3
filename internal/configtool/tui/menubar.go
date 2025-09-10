package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

// MenuItem represents a single menu item
type MenuItem struct {
	Label    string
	Key      rune
	Action   func() tea.Cmd
	Enabled  bool
	SubItems []MenuItem
}

// MenuBar represents the horizontal menu bar
type MenuBar struct {
	menus        []Menu
	activeMenu   int
	isActive     bool
	width        int
	height       int
	dropdownOpen bool
}

// Menu represents a top-level menu
type Menu struct {
	Title     string
	Key       rune // Alt key shortcut
	Items     []MenuItem
	isOpen    bool
	selected  int
}

// NewMenuBar creates a new menu bar with default menus
func NewMenuBar() *MenuBar {
	mb := &MenuBar{
		activeMenu:   -1,
		isActive:     false,
		width:        80,
		height:       1,
		dropdownOpen: false,
	}
	
	// Initialize default menus matching Turbo Pascal IDE
	mb.menus = []Menu{
		{
			Title: "File",
			Key:   'f',
			Items: []MenuItem{
				{Label: "New", Key: 'n', Enabled: true, Action: mb.actionNew},
				{Label: "Open...", Key: 'o', Enabled: true, Action: mb.actionOpen},
				{Label: "Save", Key: 's', Enabled: true, Action: mb.actionSave},
				{Label: "Save As...", Key: 'a', Enabled: true, Action: mb.actionSaveAs},
				{Label: "-", Key: 0, Enabled: false}, // Separator
				{Label: "Exit", Key: 'x', Enabled: true, Action: mb.actionExit},
			},
		},
		{
			Title: "Edit",
			Key:   'e',
			Items: []MenuItem{
				{Label: "Undo", Key: 'u', Enabled: true, Action: mb.actionUndo},
				{Label: "Redo", Key: 'r', Enabled: true, Action: mb.actionRedo},
				{Label: "-", Key: 0, Enabled: false}, // Separator
				{Label: "Cut", Key: 't', Enabled: true, Action: mb.actionCut},
				{Label: "Copy", Key: 'c', Enabled: true, Action: mb.actionCopy},
				{Label: "Paste", Key: 'p', Enabled: true, Action: mb.actionPaste},
				{Label: "-", Key: 0, Enabled: false}, // Separator
				{Label: "Select All", Key: 'a', Enabled: true, Action: mb.actionSelectAll},
			},
		},
		{
			Title: "Config",
			Key:   'c',
			Items: []MenuItem{
				{Label: "System Settings", Key: 's', Enabled: true, Action: mb.actionSystemConfig},
				{Label: "User Management", Key: 'u', Enabled: true, Action: mb.actionUserConfig},
				{Label: "Message Areas", Key: 'm', Enabled: true, Action: mb.actionMessageConfig},
				{Label: "File Areas", Key: 'f', Enabled: true, Action: mb.actionFileConfig},
				{Label: "-", Key: 0, Enabled: false}, // Separator
				{Label: "Network Settings", Key: 'n', Enabled: true, Action: mb.actionNetworkConfig},
				{Label: "Security", Key: 'e', Enabled: true, Action: mb.actionSecurityConfig},
			},
		},
		{
			Title: "Tools",
			Key:   't',
			Items: []MenuItem{
				{Label: "User Editor", Key: 'u', Enabled: true, Action: mb.actionUserEditor},
				{Label: "File Manager", Key: 'f', Enabled: true, Action: mb.actionFileManager},
				{Label: "Log Viewer", Key: 'l', Enabled: true, Action: mb.actionLogViewer},
				{Label: "-", Key: 0, Enabled: false}, // Separator
				{Label: "System Info", Key: 'i', Enabled: true, Action: mb.actionSystemInfo},
				{Label: "Statistics", Key: 's', Enabled: true, Action: mb.actionStatistics},
			},
		},
		{
			Title: "Help",
			Key:   'h',
			Items: []MenuItem{
				{Label: "Contents", Key: 'c', Enabled: true, Action: mb.actionHelpContents},
				{Label: "Keyboard Shortcuts", Key: 'k', Enabled: true, Action: mb.actionKeyboardHelp},
				{Label: "-", Key: 0, Enabled: false}, // Separator
				{Label: "About", Key: 'a', Enabled: true, Action: mb.actionAbout},
			},
		},
	}
	
	return mb
}

// Init initializes the menu bar
func (mb *MenuBar) Init() tea.Cmd {
	return nil
}

// Update handles menu bar updates
func (mb *MenuBar) Update(msg tea.Msg) (*MenuBar, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return mb, mb.handleKey(msg)
	}
	return mb, nil
}

// handleKey handles keyboard input for the menu bar
func (mb *MenuBar) handleKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyF10:
		// Use F10 to activate menu bar (traditional behavior)
		mb.isActive = true
		mb.activeMenu = 0
		
	case tea.KeyEsc:
		// Escape closes menu
		mb.closeMenu()
		
	case tea.KeyLeft:
		if mb.isActive {
			mb.previousMenu()
		}
		
	case tea.KeyRight:
		if mb.isActive {
			mb.nextMenu()
		}
		
	case tea.KeyDown:
		if mb.isActive && mb.activeMenu >= 0 {
			mb.openDropdown()
		}
		
	case tea.KeyUp:
		if mb.dropdownOpen {
			mb.previousMenuItem()
		}
		
	case tea.KeyEnter:
		if mb.dropdownOpen && mb.activeMenu >= 0 {
			return mb.executeMenuItem()
		}
		
	default:
		// Check for Alt+Key combinations
		if msg.Alt {
			for i, menu := range mb.menus {
				if strings.ToLower(string(msg.Runes[0])) == strings.ToLower(string(menu.Key)) {
					mb.isActive = true
					mb.activeMenu = i
					mb.openDropdown()
					return nil
				}
			}
		}
		
		// Check for menu item shortcuts when dropdown is open
		if mb.dropdownOpen && mb.activeMenu >= 0 {
			menu := &mb.menus[mb.activeMenu]
			for i, item := range menu.Items {
				if item.Key != 0 && strings.ToLower(string(msg.Runes[0])) == strings.ToLower(string(item.Key)) {
					menu.selected = i
					return mb.executeMenuItem()
				}
			}
		}
	}
	
	return nil
}

// SetSize sets the menu bar dimensions
func (mb *MenuBar) SetSize(width, height int) {
	mb.width = width
	mb.height = height
}

// View renders the menu bar
func (mb *MenuBar) View() string {
	// Create menu bar background
	menuBar := MenuBarStyle.Width(mb.width).Render(mb.renderMenuTitles())
	
	// If dropdown is open, render it
	if mb.dropdownOpen && mb.activeMenu >= 0 {
		dropdown := mb.renderDropdown()
		// Position dropdown below the menu bar
		return lipgloss.JoinVertical(lipgloss.Top, menuBar, dropdown)
	}
	
	return menuBar
}

// renderMenuTitles renders the menu titles in the bar
func (mb *MenuBar) renderMenuTitles() string {
	var titles []string
	
	for i, menu := range mb.menus {
		title := " " + menu.Title + " "
		
		// Highlight active menu
		if mb.isActive && i == mb.activeMenu {
			title = MenuItemActiveStyle.Render(title)
		} else {
			title = MenuItemStyle.Render(title)
		}
		
		titles = append(titles, title)
	}
	
	// Join titles and fill remaining space
	titleBar := lipgloss.JoinHorizontal(lipgloss.Top, titles...)
	remainingWidth := mb.width - lipgloss.Width(titleBar)
	if remainingWidth > 0 {
		titleBar += MenuBarStyle.Render(strings.Repeat(" ", remainingWidth))
	}
	
	return titleBar
}

// renderDropdown renders the dropdown menu
func (mb *MenuBar) renderDropdown() string {
	if mb.activeMenu < 0 || mb.activeMenu >= len(mb.menus) {
		return ""
	}
	
	menu := mb.menus[mb.activeMenu]
	
	// Calculate dropdown position and size
	x := mb.calculateMenuPosition(mb.activeMenu)
	maxWidth := 0
	
	// Find the widest menu item
	for _, item := range menu.Items {
		if lipgloss.Width(item.Label) > maxWidth {
			maxWidth = lipgloss.Width(item.Label)
		}
	}
	
	// Add padding and minimum width
	width := maxWidth + 4
	if width < 20 {
		width = 20
	}
	
	// Create menu items
	var items []string
	for i, item := range menu.Items {
		if item.Label == "-" {
			// Separator
			items = append(items, strings.Repeat(BoxHorizontal, width-2))
		} else {
			itemText := " " + item.Label
			
			// Add shortcut key indication
			if item.Key != 0 {
				itemText += strings.Repeat(" ", width-lipgloss.Width(itemText)-3)
				itemText += string(item.Key)
			}
			
			// Pad to full width
			for lipgloss.Width(itemText) < width-2 {
				itemText += " "
			}
			
			// Style the item
			if i == menu.selected {
				itemText = ListItemSelectedStyle.Render(itemText)
			} else if item.Enabled {
				itemText = ListItemStyle.Render(itemText)
			} else {
				itemText = lipgloss.NewStyle().
					Foreground(ColorDisabled).
					Background(ColorText).
					Render(itemText)
			}
			
			items = append(items, itemText)
		}
	}
	
	// Create the dropdown box
	height := len(items) + 2
	dropdown := CreateBox(width, height, "", "", false)
	
	// Overlay the items
	dropdownLines := strings.Split(dropdown, "\n")
	for i, item := range items {
		if i+1 < len(dropdownLines) {
			dropdownLines[i+1] = BoxVertical + item + BoxVertical
		}
	}
	
	dropdown = strings.Join(dropdownLines, "\n")
	
	// Position the dropdown
	if x+width > mb.width {
		x = mb.width - width
	}
	
	// For now, just return the dropdown (positioning will be handled by the window manager)
	return WindowStyle.Width(width).Render(dropdown)
}

// calculateMenuPosition calculates the X position for a menu dropdown
func (mb *MenuBar) calculateMenuPosition(menuIndex int) int {
	x := 0
	for i := 0; i < menuIndex && i < len(mb.menus); i++ {
		x += lipgloss.Width(" " + mb.menus[i].Title + " ")
	}
	return x
}

// Menu navigation methods
func (mb *MenuBar) nextMenu() {
	mb.activeMenu = (mb.activeMenu + 1) % len(mb.menus)
	mb.dropdownOpen = false
}

func (mb *MenuBar) previousMenu() {
	mb.activeMenu = (mb.activeMenu - 1 + len(mb.menus)) % len(mb.menus)
	mb.dropdownOpen = false
}

func (mb *MenuBar) openDropdown() {
	if mb.activeMenu >= 0 && mb.activeMenu < len(mb.menus) {
		mb.dropdownOpen = true
		mb.menus[mb.activeMenu].isOpen = true
		mb.menus[mb.activeMenu].selected = 0
	}
}

func (mb *MenuBar) closeMenu() {
	mb.isActive = false
	mb.dropdownOpen = false
	mb.activeMenu = -1
	for i := range mb.menus {
		mb.menus[i].isOpen = false
	}
}

func (mb *MenuBar) nextMenuItem() {
	if mb.activeMenu >= 0 && mb.activeMenu < len(mb.menus) {
		menu := &mb.menus[mb.activeMenu]
		menu.selected = (menu.selected + 1) % len(menu.Items)
	}
}

func (mb *MenuBar) previousMenuItem() {
	if mb.activeMenu >= 0 && mb.activeMenu < len(mb.menus) {
		menu := &mb.menus[mb.activeMenu]
		menu.selected = (menu.selected - 1 + len(menu.Items)) % len(menu.Items)
	}
}

func (mb *MenuBar) executeMenuItem() tea.Cmd {
	if mb.activeMenu >= 0 && mb.activeMenu < len(mb.menus) {
		menu := &mb.menus[mb.activeMenu]
		if menu.selected >= 0 && menu.selected < len(menu.Items) {
			item := menu.Items[menu.selected]
			if item.Enabled && item.Action != nil {
				mb.closeMenu()
				return item.Action()
			}
		}
	}
	return nil
}

// Action placeholders (to be implemented based on requirements)
func (mb *MenuBar) actionNew() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "new"} }
}

func (mb *MenuBar) actionOpen() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "open"} }
}

func (mb *MenuBar) actionSave() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "save"} }
}

func (mb *MenuBar) actionSaveAs() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "save_as"} }
}

func (mb *MenuBar) actionExit() tea.Cmd {
	return tea.Quit
}

func (mb *MenuBar) actionUndo() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "undo"} }
}

func (mb *MenuBar) actionRedo() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "redo"} }
}

func (mb *MenuBar) actionCut() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "cut"} }
}

func (mb *MenuBar) actionCopy() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "copy"} }
}

func (mb *MenuBar) actionPaste() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "paste"} }
}

func (mb *MenuBar) actionSelectAll() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "select_all"} }
}

func (mb *MenuBar) actionSystemConfig() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "system_config"} }
}

func (mb *MenuBar) actionUserConfig() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "user_config"} }
}

func (mb *MenuBar) actionMessageConfig() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "message_config"} }
}

func (mb *MenuBar) actionFileConfig() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "file_config"} }
}

func (mb *MenuBar) actionNetworkConfig() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "network_config"} }
}

func (mb *MenuBar) actionSecurityConfig() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "security_config"} }
}

func (mb *MenuBar) actionUserEditor() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "user_editor"} }
}

func (mb *MenuBar) actionFileManager() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "file_manager"} }
}

func (mb *MenuBar) actionLogViewer() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "log_viewer"} }
}

func (mb *MenuBar) actionSystemInfo() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "system_info"} }
}

func (mb *MenuBar) actionStatistics() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "statistics"} }
}

func (mb *MenuBar) actionHelpContents() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "help_contents"} }
}

func (mb *MenuBar) actionKeyboardHelp() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "keyboard_help"} }
}

func (mb *MenuBar) actionAbout() tea.Cmd {
	return func() tea.Msg { return MenuActionMsg{Action: "about"} }
}

// MenuActionMsg represents a menu action
type MenuActionMsg struct {
	Action string
	Data   interface{}
}