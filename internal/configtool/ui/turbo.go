package ui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// TurboUI provides a Turbo Pascal-style text user interface
type TurboUI struct {
	screenWidth  int
	screenHeight int
	currentColor int
	savedState   *unix.Termios
}

// Color constants (DOS/Turbo Pascal style)
const (
	Black        = 0
	Blue         = 1
	Green        = 2
	Cyan         = 3
	Red          = 4
	Magenta      = 5
	Brown        = 6
	LightGray    = 7
	DarkGray     = 8
	LightBlue    = 9
	LightGreen   = 10
	LightCyan    = 11
	LightRed     = 12
	LightMagenta = 13
	Yellow       = 14
	White        = 15
)

// Box drawing characters (CP437/DOS style)
const (
	BoxTopLeft     = "┌"
	BoxTopRight    = "┐"
	BoxBottomLeft  = "└"
	BoxBottomRight = "┘"
	BoxHorizontal  = "─"
	BoxVertical    = "│"
	BoxTeeDown     = "┬"
	BoxTeeUp       = "┴"
	BoxTeeRight    = "├"
	BoxTeeLeft     = "┤"
	BoxCross       = "┼"

	// Double line box drawing
	DoubleBoxTopLeft     = "╔"
	DoubleBoxTopRight    = "╗"
	DoubleBoxBottomLeft  = "╚"
	DoubleBoxBottomRight = "╝"
	DoubleBoxHorizontal  = "═"
	DoubleBoxVertical    = "║"
)

// Dialog button styles
type ButtonStyle struct {
	NormalFg   int
	NormalBg   int
	SelectedFg int
	SelectedBg int
}

var (
	DefaultButton = ButtonStyle{Black, LightGray, White, Blue}
	OKButton      = ButtonStyle{Black, Green, White, LightGreen}
	CancelButton  = ButtonStyle{Black, Red, White, LightRed}
)

// Menu item structure
type MenuItem struct {
	Text     string
	HotKey   rune
	Action   func() error
	Enabled  bool
	SubMenu  []MenuItem
}

// Dialog input field
type InputField struct {
	Label    string
	Value    string
	MaxLen   int
	Required bool
	Hidden   bool // For password fields
}

// List box item
type ListItem struct {
	Text     string
	Data     interface{}
	Selected bool
}

// NewTurboUI creates a new Turbo Pascal-style UI
func NewTurboUI() (*TurboUI, error) {
	ui := &TurboUI{
		screenWidth:  80,
		screenHeight: 25,
		currentColor: LightGray,
	}

	// Save terminal state and switch to raw mode
	if err := ui.initTerminal(); err != nil {
		return nil, fmt.Errorf("failed to initialize terminal: %w", err)
	}

	// Clear screen and hide cursor
	ui.ClearScreen()
	ui.HideCursor()

	return ui, nil
}

// Initialize terminal for raw mode input
func (ui *TurboUI) initTerminal() error {
	// Get current terminal attributes
	termios, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS)
	if err != nil {
		return err
	}
	ui.savedState = termios

	// Set raw mode
	newTermios := *termios
	newTermios.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	newTermios.Oflag &^= unix.OPOST
	newTermios.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	newTermios.Cflag &^= unix.CSIZE | unix.PARENB
	newTermios.Cflag |= unix.CS8

	return unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETS, &newTermios)
}

// Cleanup restores terminal state
func (ui *TurboUI) Cleanup() {
	ui.ShowCursor()
	ui.GotoXY(1, ui.screenHeight)
	ui.SetColor(LightGray, Black)

	// Restore terminal state
	if ui.savedState != nil {
		unix.IoctlSetTermios(int(os.Stdin.Fd()), unix.TCSETS, ui.savedState)
	}
}

// Screen control functions
func (ui *TurboUI) ClearScreen() {
	fmt.Print("\033[2J\033[H")
}

func (ui *TurboUI) GotoXY(x, y int) {
	fmt.Printf("\033[%d;%dH", y, x)
}

func (ui *TurboUI) SetColor(fg, bg int) {
	ui.currentColor = fg
	// Convert DOS colors to ANSI
	ansiFg := ui.dosToAnsi(fg)
	ansiBg := ui.dosToAnsi(bg)
	fmt.Printf("\033[%d;%dm", ansiFg, ansiBg+10)
}

func (ui *TurboUI) dosToAnsi(dosColor int) int {
	ansiColors := []int{0, 4, 2, 6, 1, 5, 3, 7, 8, 12, 10, 14, 9, 13, 11, 15}
	if dosColor >= 0 && dosColor < len(ansiColors) {
		if dosColor < 8 {
			return 30 + ansiColors[dosColor]
		}
		return 90 + ansiColors[dosColor] - 8
	}
	return 37 // Default to white
}

func (ui *TurboUI) HideCursor() {
	fmt.Print("\033[?25l")
}

func (ui *TurboUI) ShowCursor() {
	fmt.Print("\033[?25h")
}

// Text output functions
func (ui *TurboUI) WriteText(x, y int, text string) {
	ui.GotoXY(x, y)
	fmt.Print(text)
}

func (ui *TurboUI) WriteColorText(x, y int, text string, fg, bg int) {
	ui.SetColor(fg, bg)
	ui.WriteText(x, y, text)
}

func (ui *TurboUI) WriteCentered(y int, text string, fg, bg int) {
	x := (ui.screenWidth - len(text)) / 2
	ui.WriteColorText(x, y, text, fg, bg)
}

// Box drawing functions
func (ui *TurboUI) DrawBox(x, y, width, height int, fg, bg int, doubleLines bool) {
	ui.SetColor(fg, bg)

	var topLeft, topRight, bottomLeft, bottomRight, horizontal, vertical string
	if doubleLines {
		topLeft = DoubleBoxTopLeft
		topRight = DoubleBoxTopRight
		bottomLeft = DoubleBoxBottomLeft
		bottomRight = DoubleBoxBottomRight
		horizontal = DoubleBoxHorizontal
		vertical = DoubleBoxVertical
	} else {
		topLeft = BoxTopLeft
		topRight = BoxTopRight
		bottomLeft = BoxBottomLeft
		bottomRight = BoxBottomRight
		horizontal = BoxHorizontal
		vertical = BoxVertical
	}

	// Top line
	ui.GotoXY(x, y)
	fmt.Print(topLeft)
	for i := 0; i < width-2; i++ {
		fmt.Print(horizontal)
	}
	fmt.Print(topRight)

	// Side lines
	for i := 1; i < height-1; i++ {
		ui.GotoXY(x, y+i)
		fmt.Print(vertical)
		for j := 0; j < width-2; j++ {
			fmt.Print(" ")
		}
		ui.GotoXY(x+width-1, y+i)
		fmt.Print(vertical)
	}

	// Bottom line
	ui.GotoXY(x, y+height-1)
	fmt.Print(bottomLeft)
	for i := 0; i < width-2; i++ {
		fmt.Print(horizontal)
	}
	fmt.Print(bottomRight)
}

func (ui *TurboUI) FillBox(x, y, width, height int, fg, bg int) {
	ui.SetColor(fg, bg)
	for i := 0; i < height; i++ {
		ui.GotoXY(x, y+i)
		for j := 0; j < width; j++ {
			fmt.Print(" ")
		}
	}
}

// Dialog functions
func (ui *TurboUI) ShowDialog(title string, message string, buttons []string) int {
	// Calculate dialog size
	lines := strings.Split(message, "\n")
	maxLineLen := len(title)
	for _, line := range lines {
		if len(line) > maxLineLen {
			maxLineLen = len(line)
		}
	}

	buttonWidth := 0
	for _, button := range buttons {
		buttonWidth += len(button) + 4 // 2 spaces padding on each side
	}

	dialogWidth := maxLineLen + 4
	if buttonWidth > dialogWidth {
		dialogWidth = buttonWidth + 4
	}
	dialogHeight := len(lines) + 6 // Title, message, buttons, borders

	dialogX := (ui.screenWidth - dialogWidth) / 2
	dialogY := (ui.screenHeight - dialogHeight) / 2

	// Draw shadow
	ui.FillBox(dialogX+1, dialogY+1, dialogWidth, dialogHeight, Black, Black)

	// Draw dialog box
	ui.DrawBox(dialogX, dialogY, dialogWidth, dialogHeight, Black, LightGray, true)

	// Draw title
	titleX := dialogX + (dialogWidth-len(title))/2
	ui.WriteColorText(titleX, dialogY, " "+title+" ", White, Blue)

	// Draw message
	for i, line := range lines {
		lineX := dialogX + (dialogWidth-len(line))/2
		ui.WriteColorText(lineX, dialogY+2+i, line, Black, LightGray)
	}

	// Draw buttons
	selectedButton := 0
	ui.drawDialogButtons(dialogX, dialogY+dialogHeight-3, dialogWidth, buttons, selectedButton)

	// Handle input
	for {
		key := ui.ReadKey()
		switch key {
		case 27: // ESC
			return -1
		case 13: // Enter
			return selectedButton
		case 9: // Tab
			selectedButton = (selectedButton + 1) % len(buttons)
			ui.drawDialogButtons(dialogX, dialogY+dialogHeight-3, dialogWidth, buttons, selectedButton)
		case 'A', 'a': // Left arrow (after ESC sequence)
			if selectedButton > 0 {
				selectedButton--
				ui.drawDialogButtons(dialogX, dialogY+dialogHeight-3, dialogWidth, buttons, selectedButton)
			}
		case 'C', 'c': // Right arrow (after ESC sequence)
			if selectedButton < len(buttons)-1 {
				selectedButton++
				ui.drawDialogButtons(dialogX, dialogY+dialogHeight-3, dialogWidth, buttons, selectedButton)
			}
		}
	}
}

func (ui *TurboUI) drawDialogButtons(x, y, width int, buttons []string, selected int) {
	buttonWidth := 0
	for _, button := range buttons {
		buttonWidth += len(button) + 4
	}

	startX := x + (width-buttonWidth)/2
	currentX := startX

	for i, button := range buttons {
		var fg, bg int
		if i == selected {
			fg, bg = White, Blue
		} else {
			fg, bg = Black, LightGray
		}

		ui.WriteColorText(currentX, y, " "+button+" ", fg, bg)
		currentX += len(button) + 4
	}
}

// Input dialog
func (ui *TurboUI) InputDialog(title string, fields []InputField) ([]string, bool) {
	// Calculate dialog dimensions
	maxLabelLen := 0
	for _, field := range fields {
		if len(field.Label) > maxLabelLen {
			maxLabelLen = len(field.Label)
		}
	}

	dialogWidth := maxLabelLen + 32 // Label + input field space
	dialogHeight := len(fields) + 6 // Fields + title + buttons + borders

	dialogX := (ui.screenWidth - dialogWidth) / 2
	dialogY := (ui.screenHeight - dialogHeight) / 2

	// Draw dialog
	ui.FillBox(dialogX+1, dialogY+1, dialogWidth, dialogHeight, Black, Black) // Shadow
	ui.DrawBox(dialogX, dialogY, dialogWidth, dialogHeight, Black, LightGray, true)
	ui.WriteColorText(dialogX+(dialogWidth-len(title))/2, dialogY, " "+title+" ", White, Blue)

	// Draw fields
	results := make([]string, len(fields))
	for i, field := range fields {
		copy(results[i:], field.Value)
	}

	currentField := 0
	ui.drawInputFields(dialogX, dialogY, fields, results, currentField)

	// Input loop
	for {
		key := ui.ReadKey()
		switch key {
		case 27: // ESC
			return nil, false
		case 13: // Enter
			if currentField < len(fields)-1 {
				currentField++
				ui.drawInputFields(dialogX, dialogY, fields, results, currentField)
			} else {
				// Validate required fields
				valid := true
				for i, field := range fields {
					if field.Required && strings.TrimSpace(results[i]) == "" {
						valid = false
						break
					}
				}
				if valid {
					return results, true
				} else {
					ui.ShowDialog("Error", "Please fill in all required fields", []string{"OK"})
					ui.drawInputFields(dialogX, dialogY, fields, results, currentField)
				}
			}
		case 9: // Tab
			currentField = (currentField + 1) % len(fields)
			ui.drawInputFields(dialogX, dialogY, fields, results, currentField)
		case 8: // Backspace
			if len(results[currentField]) > 0 {
				results[currentField] = results[currentField][:len(results[currentField])-1]
				ui.drawInputFields(dialogX, dialogY, fields, results, currentField)
			}
		default:
			if key >= 32 && key <= 126 && len(results[currentField]) < fields[currentField].MaxLen {
				results[currentField] += string(rune(key))
				ui.drawInputFields(dialogX, dialogY, fields, results, currentField)
			}
		}
	}
}

func (ui *TurboUI) drawInputFields(dialogX, dialogY int, fields []InputField, values []string, current int) {
	for i, field := range fields {
		fieldY := dialogY + 2 + i
		labelColor := Black
		if field.Required {
			labelColor = Red
		}

		// Draw label
		ui.WriteColorText(dialogX+2, fieldY, field.Label+":", labelColor, LightGray)

		// Draw input field
		inputX := dialogX + 25
		var fg, bg int
		if i == current {
			fg, bg = Black, White
		} else {
			fg, bg = Black, LightGray
		}

		ui.SetColor(fg, bg)
		ui.GotoXY(inputX, fieldY)

		displayValue := values[i]
		if field.Hidden {
			displayValue = strings.Repeat("*", len(displayValue))
		}

		// Pad field to fixed width
		fieldText := displayValue + strings.Repeat(" ", field.MaxLen-len(displayValue))
		if len(fieldText) > field.MaxLen {
			fieldText = fieldText[:field.MaxLen]
		}
		fmt.Print(fieldText)
	}
}

// List box
func (ui *TurboUI) ListBox(title string, items []ListItem, x, y, width, height int) int {
	if len(items) == 0 {
		return -1
	}

	selected := 0
	topItem := 0

	// Find first selected item
	for i, item := range items {
		if item.Selected {
			selected = i
			break
		}
	}

	for {
		// Draw list box
		ui.DrawBox(x, y, width, height, Black, LightGray, false)
		ui.WriteColorText(x+1, y, " "+title+" ", White, Blue)

		// Draw items
		for i := 0; i < height-2; i++ {
			itemIndex := topItem + i
			if itemIndex >= len(items) {
				break
			}

			itemY := y + 1 + i
			var fg, bg int
			if itemIndex == selected {
				fg, bg = White, Blue
			} else {
				fg, bg = Black, LightGray
			}

			// Truncate item text if too long
			itemText := items[itemIndex].Text
			if len(itemText) > width-3 {
				itemText = itemText[:width-6] + "..."
			} else {
				itemText += strings.Repeat(" ", width-3-len(itemText))
			}

			ui.WriteColorText(x+1, itemY, itemText, fg, bg)
		}

		// Handle input
		key := ui.ReadKey()
		switch key {
		case 27: // ESC
			return -1
		case 13: // Enter
			return selected
		case 'A': // Up arrow
			if selected > 0 {
				selected--
				if selected < topItem {
					topItem = selected
				}
			}
		case 'B': // Down arrow
			if selected < len(items)-1 {
				selected++
				if selected >= topItem+height-2 {
					topItem = selected - height + 3
				}
			}
		case 'H': // Home
			selected = 0
			topItem = 0
		case 'F': // End
			selected = len(items) - 1
			topItem = selected - height + 3
			if topItem < 0 {
				topItem = 0
			}
		}
	}
}

// Menu system
func (ui *TurboUI) ShowMenu(items []MenuItem, x, y int) int {
	selected := 0

	for {
		// Draw menu items
		for i, item := range items {
			itemY := y + i
			var fg, bg int
			if i == selected {
				if item.Enabled {
					fg, bg = White, Blue
				} else {
					fg, bg = DarkGray, Blue
				}
			} else {
				if item.Enabled {
					fg, bg = Black, LightGray
				} else {
					fg, bg = DarkGray, LightGray
				}
			}

			// Highlight hotkey
			text := item.Text
			if item.HotKey != 0 {
				hotKeyPos := strings.IndexRune(strings.ToLower(text), item.HotKey)
				if hotKeyPos >= 0 {
					prefix := text[:hotKeyPos]
					hotkey := string(text[hotKeyPos])
					suffix := text[hotKeyPos+1:]

					ui.WriteColorText(x, itemY, prefix, fg, bg)
					ui.WriteColorText(x+len(prefix), itemY, hotkey, Yellow, bg)
					ui.WriteColorText(x+len(prefix)+1, itemY, suffix, fg, bg)
				} else {
					ui.WriteColorText(x, itemY, text, fg, bg)
				}
			} else {
				ui.WriteColorText(x, itemY, text, fg, bg)
			}
		}

		// Handle input
		key := ui.ReadKey()
		switch key {
		case 27: // ESC
			return -1
		case 13: // Enter
			if items[selected].Enabled {
				if items[selected].Action != nil {
					items[selected].Action()
				}
				return selected
			}
		case 'A': // Up arrow
			for {
				selected--
				if selected < 0 {
					selected = len(items) - 1
				}
				if items[selected].Enabled || selected == 0 {
					break
				}
			}
		case 'B': // Down arrow
			for {
				selected++
				if selected >= len(items) {
					selected = 0
				}
				if items[selected].Enabled || selected == len(items)-1 {
					break
				}
			}
		default:
			// Check for hotkeys
			for i, item := range items {
				if item.Enabled && item.HotKey != 0 && (key == int(item.HotKey) || key == int(item.HotKey)-32) {
					selected = i
					if item.Action != nil {
						item.Action()
					}
					return i
				}
			}
		}
	}
}

// Progress bar
func (ui *TurboUI) ShowProgress(title string, percent int, x, y, width int) {
	ui.WriteColorText(x, y-1, title, Black, LightGray)

	// Draw progress bar border
	ui.DrawBox(x, y, width, 3, Black, LightGray, false)

	// Fill progress
	fillWidth := (width - 2) * percent / 100
	ui.FillBox(x+1, y+1, fillWidth, 1, White, Blue)

	// Draw percentage
	percentText := fmt.Sprintf("%d%%", percent)
	percentX := x + (width-len(percentText))/2
	ui.WriteColorText(percentX, y+1, percentText, White, Blue)
}

// Status bar
func (ui *TurboUI) ShowStatusBar(text string) {
	ui.FillBox(1, ui.screenHeight, ui.screenWidth, 1, Black, Cyan)
	ui.WriteColorText(1, ui.screenHeight, text, Black, Cyan)
}

// Key input
func (ui *TurboUI) ReadKey() int {
	var buf [1]byte
	n, err := syscall.Read(int(os.Stdin.Fd()), buf[:])
	if err != nil || n == 0 {
		return 0
	}

	ch := int(buf[0])

	// Handle escape sequences
	if ch == 27 {
		// Check if there are more characters
		syscall.SetNonblock(int(os.Stdin.Fd()), true)
		n, err := syscall.Read(int(os.Stdin.Fd()), buf[:])
		syscall.SetNonblock(int(os.Stdin.Fd()), false)

		if err == nil && n > 0 {
			if buf[0] == '[' {
				// Arrow keys
				n, err := syscall.Read(int(os.Stdin.Fd()), buf[:])
				if err == nil && n > 0 {
					return int(buf[0]) // Return A, B, C, D for arrow keys
				}
			}
		}
	}

	return ch
}

// Utility functions
func (ui *TurboUI) Confirm(message string) bool {
	result := ui.ShowDialog("Confirm", message, []string{"Yes", "No"})
	return result == 0
}

func (ui *TurboUI) Alert(title, message string) {
	ui.ShowDialog(title, message, []string{"OK"})
}

func (ui *TurboUI) GetString(prompt string, defaultValue string, maxLen int) (string, bool) {
	fields := []InputField{
		{Label: prompt, Value: defaultValue, MaxLen: maxLen, Required: false},
	}
	results, ok := ui.InputDialog("Input", fields)
	if ok && len(results) > 0 {
		return results[0], true
	}
	return "", false
}

func (ui *TurboUI) GetPassword(prompt string, maxLen int) (string, bool) {
	fields := []InputField{
		{Label: prompt, Value: "", MaxLen: maxLen, Required: true, Hidden: true},
	}
	results, ok := ui.InputDialog("Password", fields)
	if ok && len(results) > 0 {
		return results[0], true
	}
	return "", false
}

func (ui *TurboUI) GetNumber(prompt string, defaultValue int, min, max int) (int, bool) {
	defaultStr := strconv.Itoa(defaultValue)
	fields := []InputField{
		{Label: prompt, Value: defaultStr, MaxLen: 10, Required: true},
	}

	for {
		results, ok := ui.InputDialog("Input", fields)
		if !ok {
			return 0, false
		}

		if len(results) > 0 {
			if value, err := strconv.Atoi(results[0]); err == nil {
				if value >= min && value <= max {
					return value, true
				}
			}
		}

		ui.Alert("Error", fmt.Sprintf("Please enter a number between %d and %d", min, max))
		fields[0].Value = results[0]
	}
}