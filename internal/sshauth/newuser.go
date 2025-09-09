package sshauth

import (
	"fmt"
	"log"
	"strings"

	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/user"
)

// TerminalInterface defines what we need from a terminal for registration
type TerminalInterface interface {
	WriteString(s string) error
	ReadLine() (string, error)
	Write(data []byte) (int, error)
}

// NewUserRegistration handles the registration flow for new users
type NewUserRegistration struct {
	userMgr       *user.UserMgr
	config        config.SSHAuthConfig
	strings       config.StringsConfig
	outputMode    ansi.OutputMode
}

// NewNewUserRegistration creates a new user registration handler
func NewNewUserRegistration(userMgr *user.UserMgr, authConfig config.SSHAuthConfig, strings config.StringsConfig) *NewUserRegistration {
	return &NewUserRegistration{
		userMgr:    userMgr,
		config:     authConfig,
		strings:    strings,
		outputMode: ansi.OutputModeAuto, // Default, will be updated per session
	}
}

// SetOutputMode sets the output mode for terminal display
func (n *NewUserRegistration) SetOutputMode(mode ansi.OutputMode) {
	n.outputMode = mode
}

// RunRegistration runs the new user registration flow
func (n *NewUserRegistration) RunRegistration(terminal TerminalInterface, remoteAddr string) (*user.User, error) {
	log.Printf("INFO: Starting new user registration from %s", remoteAddr)
	
	// Display welcome screen
	err := n.displayWelcome(terminal)
	if err != nil {
		log.Printf("ERROR: Failed to display welcome screen: %v", err)
		return nil, fmt.Errorf("failed to display welcome screen: %w", err)
	}
	
	// Get username
	username, err := n.promptForUsername(terminal)
	if err != nil {
		log.Printf("ERROR: Failed to get username: %v", err)
		return nil, fmt.Errorf("failed to get username: %w", err)
	}
	
	// Get password
	password, err := n.promptForPassword(terminal)
	if err != nil {
		log.Printf("ERROR: Failed to get password: %v", err)
		return nil, fmt.Errorf("failed to get password: %w", err)
	}
	
	// Get real name
	realName, err := n.promptForRealName(terminal)
	if err != nil {
		log.Printf("ERROR: Failed to get real name: %v", err)
		return nil, fmt.Errorf("failed to get real name: %w", err)
	}
	
	// Get location
	location, err := n.promptForLocation(terminal)
	if err != nil {
		log.Printf("ERROR: Failed to get location: %v", err)
		return nil, fmt.Errorf("failed to get location: %w", err)
	}
	
	// Create the user
	newUser, err := n.createUser(username, password, realName, location, remoteAddr)
	if err != nil {
		log.Printf("ERROR: Failed to create user: %v", err)
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	// Display completion message
	err = n.displayCompletion(terminal, newUser)
	if err != nil {
		log.Printf("WARN: Failed to display completion message: %v", err)
	}
	
	log.Printf("INFO: Successfully registered new user '%s' (ID: %d) from %s", 
		newUser.Handle, newUser.ID, remoteAddr)
	
	return newUser, nil
}

// displayWelcome displays the welcome screen for new users
func (n *NewUserRegistration) displayWelcome(terminal TerminalInterface) error {
	welcome := fmt.Sprintf("%s\r\n%s\r\n\r\n%s\r\n\r\n",
		ansi.ClearScreen(),
		ansi.ReplacePipeCodes([]byte("|15|04 Welcome to New User Registration |00")),
		n.strings.WelcomeNewUser,
	)
	
	return terminal.WriteString(welcome)
}

// promptForUsername prompts for and validates a username
func (n *NewUserRegistration) promptForUsername(terminal TerminalInterface) (string, error) {
	maxAttempts := 3
	
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Display prompt
		prompt := fmt.Sprintf("%s: ", n.strings.NewUserNameStr)
		if prompt == ": " { // Fallback if string not configured
			prompt = "Enter your desired username: "
		}
		
		err := terminal.WriteString(prompt)
		if err != nil {
			return "", err
		}
		
		// Read username
		username, err := terminal.ReadLine()
		if err != nil {
			return "", err
		}
		
		username = strings.TrimSpace(username)
		
		// Validate username
		if len(username) < 3 {
			terminal.WriteString("Username must be at least 3 characters long.\r\n\r\n")
			continue
		}
		
		if len(username) > 25 {
			terminal.WriteString("Username must be 25 characters or less.\r\n\r\n")
			continue
		}
		
		// Check for invalid characters
		if strings.ContainsAny(username, " \t\r\n@#$%^&*()+=[]{}\\|;:\"'<>?,./") {
			terminal.WriteString(n.strings.InvalidUserName + "\r\n\r\n")
			continue
		}
		
		// Check if username already exists
		if _, exists := n.userMgr.GetUser(username); exists {
			terminal.WriteString(n.strings.NameAlreadyUsed + "\r\n\r\n")
			continue
		}
		
		// Reserve "new" username
		if strings.ToLower(username) == "new" {
			terminal.WriteString("The username 'new' is reserved. Please choose another.\r\n\r\n")
			continue
		}
		
		return username, nil
	}
	
	return "", fmt.Errorf("failed to get valid username after %d attempts", maxAttempts)
}

// promptForPassword prompts for and validates a password
func (n *NewUserRegistration) promptForPassword(terminal TerminalInterface) (string, error) {
	maxAttempts := 3
	
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Display prompt
		prompt := fmt.Sprintf("%s: ", n.strings.CreateAPassword)
		if prompt == ": " { // Fallback if string not configured
			prompt = "Create a password: "
		}
		
		err := terminal.WriteString(prompt)
		if err != nil {
			return "", err
		}
		
		// Read password (should be hidden, but term.Terminal.ReadPassword might not work on all platforms)
		password, err := terminal.ReadLine()
		if err != nil {
			return "", err
		}
		
		password = strings.TrimSpace(password)
		
		// Validate password
		if len(password) < n.config.MinPasswordLength {
			msg := fmt.Sprintf("Password must be at least %d characters long.\r\n\r\n", n.config.MinPasswordLength)
			terminal.WriteString(msg)
			continue
		}
		
		// Confirm password
		prompt = fmt.Sprintf("%s: ", n.strings.ReEnterPassword)
		if prompt == ": " {
			prompt = "Confirm password: "
		}
		
		err = terminal.WriteString(prompt)
		if err != nil {
			return "", err
		}
		
		confirm, err := terminal.ReadLine()
		if err != nil {
			return "", err
		}
		
		confirm = strings.TrimSpace(confirm)
		
		if password != confirm {
			terminal.WriteString("Passwords do not match. Please try again.\r\n\r\n")
			continue
		}
		
		return password, nil
	}
	
	return "", fmt.Errorf("failed to get valid password after %d attempts", maxAttempts)
}

// promptForRealName prompts for the user's real name
func (n *NewUserRegistration) promptForRealName(terminal TerminalInterface) (string, error) {
	prompt := fmt.Sprintf("%s: ", n.strings.EnterRealName)
	if prompt == ": " {
		prompt = "Enter your real name: "
	}
	
	err := terminal.WriteString(prompt)
	if err != nil {
		return "", err
	}
	
	realName, err := terminal.ReadLine()
	if err != nil {
		return "", err
	}
	
	realName = strings.TrimSpace(realName)
	if len(realName) == 0 {
		realName = "Unknown" // Default if not provided
	}
	
	return realName, nil
}

// promptForLocation prompts for the user's location
func (n *NewUserRegistration) promptForLocation(terminal TerminalInterface) (string, error) {
	prompt := "Enter your location (City, State): "
	
	err := terminal.WriteString(prompt)
	if err != nil {
		return "", err
	}
	
	location, err := terminal.ReadLine()
	if err != nil {
		return "", err
	}
	
	location = strings.TrimSpace(location)
	if len(location) == 0 {
		location = "Unknown" // Default if not provided
	}
	
	return location, nil
}

// createUser creates a new user with the provided information
func (n *NewUserRegistration) createUser(username, password, realName, location, remoteAddr string) (*user.User, error) {
	// Use the UserMgr.AddUser method which handles all the creation logic
	newUser, err := n.userMgr.AddUser(username, password, username, realName, "", location)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	// Update validation status if needed
	if n.config.RequireValidation {
		// User needs validation - set Validated to false
		newUser.Validated = false
	} else {
		// Auto-validate the user
		newUser.Validated = true
	}
	
	// Set default access level
	newUser.AccessLevel = 1 // Default security level for new users
	
	// Set default time limit
	newUser.TimeLimit = 60 // Default time limit in minutes
	
	// Save the updated user data
	err = n.userMgr.SaveUsers()
	if err != nil {
		log.Printf("WARN: Failed to save updated user data: %v", err)
	}
	
	return newUser, nil
}

// displayCompletion displays the registration completion message
func (n *NewUserRegistration) displayCompletion(terminal TerminalInterface, newUser *user.User) error {
	message := fmt.Sprintf("\r\n\r\n|10Welcome to the BBS, %s!|07\r\n", newUser.Handle)
	
	if n.config.RequireValidation {
		message += "\r\n|14Your account requires validation by the SysOp before you can fully access the system.|07\r\n"
		message += "You will be contacted when your account is approved.\r\n"
	} else {
		message += "\r\n|11Your account has been created and is ready to use!|07\r\n"
		message += "Press any key to continue to the main menu...\r\n"
	}
	
	err := terminal.WriteString(message)
	if err != nil {
		return err
	}
	
	// Wait for keypress
	if !n.config.RequireValidation {
		terminal.ReadLine()
	}
	
	return nil
}