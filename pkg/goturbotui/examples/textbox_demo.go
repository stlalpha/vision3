package main

import (
	"fmt"
	"os"
	
	"github.com/stlalpha/vision3/pkg/goturbotui"
	"github.com/stlalpha/vision3/pkg/goturbotui/components"
)

// TextBoxDemo demonstrates the TextBox component functionality
type TextBoxDemo struct {
	app        *goturbotui.Application
	desktop    *Desktop
	dialog     *components.Dialog
	textBox1   *components.TextBox
	textBox2   *components.TextBox
	textBox3   *components.TextBox
	statusText string
}

// Desktop component for the demo
type Desktop struct {
	*goturbotui.BaseContainer
	demo *TextBoxDemo
}

// NewDesktop creates a new desktop container
func NewDesktop(demo *TextBoxDemo) *Desktop {
	return &Desktop{
		BaseContainer: goturbotui.NewBaseContainer(),
		demo:          demo,
	}
}

// Draw renders the desktop
func (d *Desktop) Draw(canvas goturbotui.Canvas) {
	theme := goturbotui.DefaultTurboTheme()
	
	// Clear with desktop background
	bounds := d.GetBounds()
	canvas.Fill(bounds, '░', theme.Desktop)
	
	// Draw children
	d.BaseContainer.Draw(canvas)
	
	// Draw labels on the dialog
	if d.demo.dialog != nil && d.demo.dialog.IsVisible() {
		dialogBounds := d.demo.dialog.GetBounds()
		labelStyle := theme.DialogText
		
		// Adjust positions relative to dialog bounds
		canvas.SetString(dialogBounds.X+2, dialogBounds.Y+1, "Name:", labelStyle)
		canvas.SetString(dialogBounds.X+2, dialogBounds.Y+4, "Password:", labelStyle)
		canvas.SetString(dialogBounds.X+2, dialogBounds.Y+7, "Numbers:", labelStyle)
	}
	
	// Draw status at bottom
	width, height := canvas.Size()
	statusStyle := theme.StatusBar
	canvas.Fill(goturbotui.NewRect(0, height-1, width, 1), ' ', statusStyle)
	canvas.SetString(1, height-1, d.demo.statusText, statusStyle)
	
	// Draw help text
	helpText := "Tab: Next field | Esc: Quit | Ctrl+A: Select All"
	canvas.SetString(width-len(helpText)-1, height-1, helpText, statusStyle)
}

// HandleEvent handles desktop events
func (d *Desktop) HandleEvent(event goturbotui.Event) bool {
	if event.Type == goturbotui.EventKey {
		switch event.Key.Code {
		case goturbotui.KeyEscape:
			d.demo.app.Stop()
			return true
		case goturbotui.KeyTab:
			// Cycle through text boxes
			current := d.GetFocused()
			switch current {
			case d.demo.textBox1:
				d.SetFocus(d.demo.textBox2)
			case d.demo.textBox2:
				d.SetFocus(d.demo.textBox3)
			case d.demo.textBox3:
				d.SetFocus(d.demo.textBox1)
			default:
				d.SetFocus(d.demo.textBox1)
			}
			return true
		}
	}
	
	// Pass to children
	return d.BaseContainer.HandleEvent(event)
}

// NewTextBoxDemo creates a new demo application
func NewTextBoxDemo() *TextBoxDemo {
	// Create demo
	demo := &TextBoxDemo{
		statusText: "TextBox Demo - Tab to switch, Esc to quit",
	}
	
	// Create application
	demo.app = goturbotui.NewApplication()
	
	// Create theme
	theme := goturbotui.DefaultTurboTheme()
	
	// Create desktop
	demo.desktop = NewDesktop(demo)
	demo.app.SetDesktop(demo.desktop)
	
	// Create main dialog
	demo.dialog = components.NewDialog("TextBox Component Demo", 50, 15, theme)
	
	// Create text boxes
	demo.textBox1 = components.NewTextBox(theme)
	demo.textBox1.SetBounds(goturbotui.NewRect(2, 2, 30, 1))
	demo.textBox1.SetPlaceholder("Enter your name...")
	demo.textBox1.SetOnChange(func(text string) {
		demo.statusText = fmt.Sprintf("Name changed: %s", text)
	})
	
	demo.textBox2 = components.NewTextBox(theme)
	demo.textBox2.SetBounds(goturbotui.NewRect(2, 5, 30, 1))
	demo.textBox2.SetPasswordMode(true)
	demo.textBox2.SetPlaceholder("Enter password...")
	demo.textBox2.SetMaxLength(20)
	demo.textBox2.SetOnChange(func(text string) {
		demo.statusText = fmt.Sprintf("Password length: %d", len(text))
	})
	
	demo.textBox3 = components.NewTextBox(theme)
	demo.textBox3.SetBounds(goturbotui.NewRect(2, 8, 40, 1))
	demo.textBox3.SetPlaceholder("Numbers only (max 10 chars)")
	demo.textBox3.SetMaxLength(10)
	demo.textBox3.SetValidator(func(text string) bool {
		// Only allow digits
		for _, r := range text {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	})
	demo.textBox3.SetOnChange(func(text string) {
		demo.statusText = fmt.Sprintf("Number: %s", text)
	})
	
	// Add children to dialog
	demo.dialog.AddChild(demo.textBox1)
	demo.dialog.AddChild(demo.textBox2)
	demo.dialog.AddChild(demo.textBox3)
	
	// Set initial focus
	demo.dialog.SetFocus(demo.textBox1)
	
	// Add dialog to desktop
	demo.desktop.AddChild(demo.dialog)
	demo.desktop.SetFocus(demo.dialog)
	
	return demo
}

// Run starts the demo application
func (demo *TextBoxDemo) Run() error {
	// Center the dialog
	width, height := demo.app.GetScreen().Size()
	demo.dialog.Center(width, height)
	
	// Run application
	return demo.app.Run()
}

func main() {
	demo := NewTextBoxDemo()
	if err := demo.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}