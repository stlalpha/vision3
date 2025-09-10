package main

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/term"
)

func main() {
	// Test basic terminal setup
	fmt.Println("Testing basic terminal functionality...")
	
	// Get terminal size
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		log.Printf("Error getting terminal size: %v", err)
		width, height = 80, 25
	}
	fmt.Printf("Terminal size: %dx%d\n", width, height)
	
	// Test raw mode
	fmt.Println("Setting raw mode...")
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatalf("Failed to set raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)
	
	// Clear screen and test basic drawing
	fmt.Print("\033[2J\033[H")  // Clear screen, cursor to home
	fmt.Print("\033[44m")       // Blue background
	
	// Draw a simple test pattern
	for y := 0; y < height; y++ {
		fmt.Printf("\033[%d;1H", y+1) // Move cursor to row y+1, column 1
		for x := 0; x < width; x++ {
			if y == 0 || y == height-1 || x == 0 || x == width-1 {
				fmt.Print("#") // Border
			} else {
				fmt.Print(" ") // Background
			}
		}
	}
	
	// Draw test message
	fmt.Print("\033[10;10H") // Move to row 10, col 10
	fmt.Print("\033[47m\033[30m") // White background, black text
	fmt.Print(" BASIC TUI TEST - Press any key to exit ")
	fmt.Print("\033[0m") // Reset
	
	// Wait for keypress
	buf := make([]byte, 1)
	os.Stdin.Read(buf)
	
	// Reset terminal
	fmt.Print("\033[0m\033[2J\033[H")
}