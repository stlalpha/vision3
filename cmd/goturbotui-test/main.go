package main

import (
	"log"

	"github.com/stlalpha/vision3/pkg/goturbotui"
	"github.com/stlalpha/vision3/pkg/goturbotui/components"
)

func main() {
	// Create TUI application
	app := goturbotui.NewApplication()
	theme := goturbotui.DefaultTurboTheme()
	app.SetTheme(theme)

	// Create main window
	window := components.NewWindow("GoTurboTUI Test Application", theme)

	// Create menu bar
	menuBar := components.NewMenuBar(theme)
	menuBar.SetItems([]components.MenuItem{
		{Text: "File", Hotkey: 'f', Enabled: true},
		{Text: "Edit", Hotkey: 'e', Enabled: true},
		{Text: "View", Hotkey: 'v', Enabled: true},
		{Text: "Help", Hotkey: 'h', Enabled: true},
	})
	window.SetMenuBar(menuBar)

	// Create status bar
	statusBar := components.NewStatusBar(theme)
	statusBar.SetItems([]components.StatusItem{
		{Key: "F1", Text: "Help"},
		{Key: "F2", Text: "Save"},
		{Key: "F10", Text: "Exit", Action: func() { app.Stop() }},
	})
	statusBar.SetMessage("GoTurboTUI Alignment Test - Use F10 to Exit")
	window.SetStatusBar(statusBar)

	// Create test dialog
	dialog := components.NewDialog("Alignment Test", 60, 10, theme)
	
	// Create list box
	listBox := components.NewListBox(theme)
	listBox.SetItems([]string{
		"This text should align perfectly to the right edge",
		"No more black gaps on the right side",
		"Proper bounds calculation with TVision principles",
		"Responsive to terminal resizing",
		"Authentic Turbo Pascal aesthetics",
	})
	listBox.SetBounds(goturbotui.NewRect(1, 1, 56, 6))
	listBox.SetFocused(true)
	
	dialog.AddChild(listBox)
	
	// Show dialog as modal
	window.SetClient(&TestClient{dialog: dialog, theme: theme})

	// Set window as desktop
	app.SetDesktop(window)

	// Run application
	if err := app.Run(); err != nil {
		log.Fatalf("Error running application: %v", err)
	}
}

// TestClient displays the test dialog
type TestClient struct {
	*goturbotui.BaseView
	dialog goturbotui.View
	theme  *goturbotui.Theme
}

func (tc *TestClient) Draw(canvas goturbotui.Canvas) {
	if !tc.IsVisible() {
		return
	}
	
	// Center dialog
	width, height := canvas.Size()
	if dialog, ok := tc.dialog.(*components.Dialog); ok {
		dialog.Center(width, height)
	}
	
	tc.dialog.Draw(canvas)
}

func (tc *TestClient) HandleEvent(event goturbotui.Event) bool {
	return tc.dialog.HandleEvent(event)
}