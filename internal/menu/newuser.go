package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/terminalio"
	"github.com/stlalpha/vision3/internal/user"
	"golang.org/x/term"
)

const (
	newUserHandleMaxLen   = 30
	newUserRealNameMaxLen = 30
	newUserNoteMaxLen     = 30
	newUserLocationMaxLen = 30
)

// handleNewUserApplication runs the new user signup form.
// Flow matches the original ViSiON/2 Pascal NewUser() in GETLOGIN.PAS.
func (e *MenuExecutor) handleNewUserApplication(
	s ssh.Session,
	terminal *term.Terminal,
	userManager *user.UserMgr,
	nodeNumber int,
	outputMode ansi.OutputMode,
	termWidth int,
	termHeight int,
) error {
	log.Printf("INFO: Node %d: Starting new user application", nodeNumber)

	// 1. "Apply for access?" prompt (before showing the ANS screen, matching Pascal flow)
	applyPrompt := e.LoadedStrings.ApplyAsNewStr
	if applyPrompt == "" {
		applyPrompt = "|08A|07p|15ply |08F|07o|15r |08A|07c|15cess? @"
	}
	applyYes, err := e.promptYesNo(s, terminal, applyPrompt, outputMode, nodeNumber, termWidth, termHeight)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return io.EOF
		}
		log.Printf("ERROR: Node %d: Error during apply prompt: %v", nodeNumber, err)
		return err
	}
	if !applyYes {
		log.Printf("INFO: Node %d: User declined new user application", nodeNumber)
		return nil
	}

	// Show blinking underline cursor for form input
	terminalio.WriteProcessedBytes(terminal, []byte("\x1b[?25h\x1b[3 q"), outputMode)

	// 2. Display NEWUSER.ANS welcome screen with pause
	if err := e.displayNewUserScreen(terminal, outputMode, nodeNumber); err != nil {
		log.Printf("WARN: Node %d: Failed to display NEWUSER.ANS: %v", nodeNumber, err)
		// Continue without the screen - not fatal
	} else {
		// Pause after displaying the ANS screen (matching Pascal HoldScreen)
		pausePrompt := e.LoadedStrings.PauseString
		if pausePrompt == "" {
			pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... "
		}
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte("\r\n\r\n"+pausePrompt)), outputMode)
		terminal.ReadLine()
	}

	// 3. Handle/Alias entry
	handle, err := e.promptForHandle(s, terminal, userManager, nodeNumber, outputMode)
	if err != nil {
		return err
	}
	if handle == "" {
		return nil // User cancelled
	}

	// 4. Clear screen, show welcome and user number (matching Pascal: AnsiCls → Welcome → UserNum)
	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)

	welcomeStr := e.LoadedStrings.WelcomeNewUser
	if welcomeStr == "" {
		welcomeStr = "|15Welcome to the system!"
	}
	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(welcomeStr+"\r\n")), outputMode)

	userNumStr := e.LoadedStrings.YourUserNum
	if userNumStr == "" {
		userNumStr = "|15Your User # is |09|UN|CR"
	}
	nextID := userManager.NextUserID()
	userNumStr = strings.ReplaceAll(userNumStr, "|UN", strconv.Itoa(nextID))
	userNumStr = strings.ReplaceAll(userNumStr, "|CR", "\r\n")
	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(userNumStr)), outputMode)

	// 5. Password creation with confirmation
	password, err := e.promptForPassword(s, terminal, nodeNumber, outputMode)
	if err != nil {
		return err
	}
	if password == "" {
		return nil // User cancelled
	}

	// 6. Real name
	realName, err := e.promptForRealName(s, terminal, nodeNumber, outputMode)
	if err != nil {
		return err
	}
	if realName == "" {
		return nil // User cancelled
	}

	// 7. User Note (optional)
	userNote, err := e.promptForUserNote(s, terminal, nodeNumber, outputMode)
	if err != nil {
		return err
	}

	// 8. Location
	location, err := e.promptForLocation(s, terminal, nodeNumber, outputMode)
	if err != nil {
		return err
	}

	// 9. Create account
	newUser, addErr := userManager.AddUser(
		strings.ToLower(handle), // username = lowercase handle
		password,
		handle,
		realName,
		"", // phone number (legacy, not collected)
		location,
	)
	if addErr != nil {
		log.Printf("ERROR: Node %d: Failed to create new user '%s': %v", nodeNumber, handle, addErr)
		errMsg := "\r\n|13Error creating account. Please try again later.|07\r\n"
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
		time.Sleep(2 * time.Second)
		return nil
	}

	// Set the private note with creation date
	newUser.PrivateNote = userNote
	newUser.CreatedAt = time.Now()
	if saveErr := userManager.UpdateUser(newUser); saveErr != nil {
		log.Printf("ERROR: Node %d: Failed to save user note for '%s': %v", nodeNumber, handle, saveErr)
	}

	log.Printf("INFO: Node %d: New user '%s' created (ID: %d, Handle: %s)", nodeNumber, newUser.Username, newUser.ID, newUser.Handle)

	// 11. Show validation message
	validationMsg := "\r\n|15Your account has been created but requires |13SysOp validation|15.\r\n|08Please call back later to check your access.|07\r\n"
	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(validationMsg)), outputMode)

	// Pause before returning
	pausePrompt := e.LoadedStrings.PauseString
	if pausePrompt == "" {
		pausePrompt = "\r\n|07Press |15[ENTER]|07 to continue... "
	}
	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte("\r\n"+pausePrompt)), outputMode)
	terminal.ReadLine()

	return nil
}

// displayNewUserScreen loads and displays NEWUSER.ANS.
func (e *MenuExecutor) displayNewUserScreen(terminal *term.Terminal, outputMode ansi.OutputMode, nodeNumber int) error {
	fullAnsPath := filepath.Join(e.MenuSetPath, "ansi", "NEWUSER.ANS")
	rawContent, err := ansi.GetAnsiFileContent(fullAnsPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("DEBUG: Node %d: NEWUSER.ANS not found, skipping display", nodeNumber)
			return nil
		}
		return fmt.Errorf("failed to read NEWUSER.ANS: %w", err)
	}

	terminalio.WriteProcessedBytes(terminal, []byte(ansi.ClearScreen()), outputMode)
	terminalio.WriteProcessedBytes(terminal, rawContent, outputMode)
	return nil
}

// promptForHandle prompts for and validates a handle/alias.
func (e *MenuExecutor) promptForHandle(
	s ssh.Session,
	terminal *term.Terminal,
	userManager *user.UserMgr,
	nodeNumber int,
	outputMode ansi.OutputMode,
) (string, error) {
	prompt := e.LoadedStrings.NewUserNameStr
	if prompt == "" {
		prompt = "|CR|08E|07n|15ter |08Y|07o|15ur |08A|07l|15ias|09.|CR|08:"
	}
	prompt = strings.ReplaceAll(prompt, "|CR", "\r\n")

	invalidMsg := e.LoadedStrings.InvalidUserName
	if invalidMsg == "" {
		invalidMsg = "|05 |10Invalid Name .. Try again!"
	}

	nameUsedMsg := e.LoadedStrings.NameAlreadyUsed
	if nameUsedMsg == "" {
		nameUsedMsg = "|15 |13Name is already in use! |15"
	}

	checkingMsg := e.LoadedStrings.CheckingUserBase
	if checkingMsg == "" {
		checkingMsg = "|08 |05Fi|13nding |05A |05Pl|13ace |05Fo|13r |05Y|13ou!"
	}

	maxAttempts := 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(prompt)), outputMode)

		input, err := styledInput(terminal, s, outputMode, newUserHandleMaxLen, "")
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", io.EOF
			}
			return "", fmt.Errorf("failed reading handle: %w", err)
		}

		handle := strings.TrimSpace(input)
		if handle == "" {
			return "", nil // User cancelled with empty input
		}

		// Validate handle format
		if !validateHandle(handle) {
			terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(invalidMsg+"\r\n")), outputMode)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Show "checking user base" message (matches Pascal: MultiColor(Strng^.Checking_User_Base))
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(checkingMsg+"\r\n")), outputMode)

		// Check for duplicate username
		if _, exists := userManager.GetUser(strings.ToLower(handle)); exists {
			terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(nameUsedMsg+"\r\n")), outputMode)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Check for duplicate handle
		if _, exists := userManager.GetUserByHandle(handle); exists {
			terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(nameUsedMsg+"\r\n")), outputMode)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		log.Printf("INFO: Node %d: Handle '%s' accepted for new user application", nodeNumber, handle)
		return handle, nil
	}

	// Max attempts reached
	errMsg := "\r\n|05Too many invalid attempts.|07\r\n"
	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
	time.Sleep(1 * time.Second)
	return "", nil
}

// promptForPassword prompts for password with confirmation.
func (e *MenuExecutor) promptForPassword(
	s ssh.Session,
	terminal *term.Terminal,
	nodeNumber int,
	outputMode ansi.OutputMode,
) (string, error) {
	createPrompt := e.LoadedStrings.CreateAPassword
	if createPrompt == "" {
		createPrompt = "|08C|07r|15eate A |08P|07a|15ssword |09: "
	}

	confirmPrompt := e.LoadedStrings.ReEnterPassword
	if confirmPrompt == "" {
		confirmPrompt = "|08R|07e|15nter |08P|07a|15ssword |09: "
	}

	maxAttempts := 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// First entry
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte("\r\n"+createPrompt)), outputMode)
		password, err := readPasswordSecurely(s, terminal, outputMode)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", io.EOF
			}
			if err.Error() == "password entry interrupted" {
				return "", nil
			}
			return "", fmt.Errorf("failed reading password: %w", err)
		}

		if len(password) < 3 {
			msg := "\r\n|09Password must be at least 3 characters.|07\r\n"
			terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		// Confirmation
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(confirmPrompt)), outputMode)
		confirm, err := readPasswordSecurely(s, terminal, outputMode)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", io.EOF
			}
			if err.Error() == "password entry interrupted" {
				return "", nil
			}
			return "", fmt.Errorf("failed reading password confirmation: %w", err)
		}

		if password != confirm {
			msg := "\r\n|09They don't match!|07\r\n"
			terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		log.Printf("INFO: Node %d: Password accepted for new user application", nodeNumber)
		return password, nil
	}

	errMsg := "\r\n|09Too many failed attempts.|07\r\n"
	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
	time.Sleep(1 * time.Second)
	return "", nil
}

// promptForRealName prompts for and validates a real name.
func (e *MenuExecutor) promptForRealName(
	s ssh.Session,
	terminal *term.Terminal,
	nodeNumber int,
	outputMode ansi.OutputMode,
) (string, error) {
	prompt := e.LoadedStrings.EnterRealName
	if prompt == "" {
		prompt = "|08E|07n|15ter |08Y|07o|15ur |09REAL |08N|07a|15me |09: "
	}

	maxAttempts := 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte("\r\n"+prompt)), outputMode)

		input, err := styledInput(terminal, s, outputMode, newUserRealNameMaxLen, "")
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", io.EOF
			}
			return "", fmt.Errorf("failed reading real name: %w", err)
		}

		name := strings.TrimSpace(input)
		if name == "" {
			return "", nil // User cancelled
		}

		if !validateRealName(name) {
			msg := "\r\n|05Please enter your |10first |05and |10last |05name.|07\r\n"
			terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(msg)), outputMode)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		log.Printf("INFO: Node %d: Real name '%s' accepted for new user application", nodeNumber, name)
		return name, nil
	}

	errMsg := "\r\n|05Too many invalid attempts.|07\r\n"
	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte(errMsg)), outputMode)
	time.Sleep(1 * time.Second)
	return "", nil
}

// promptForLocation prompts for the caller's location.
func (e *MenuExecutor) promptForLocation(
	s ssh.Session,
	terminal *term.Terminal,
	nodeNumber int,
	outputMode ansi.OutputMode,
) (string, error) {
	prompt := "|08G|07r|15oup|08/|07L|15ocation |09: "
	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte("\r\n"+prompt)), outputMode)

	input, err := styledInput(terminal, s, outputMode, newUserLocationMaxLen, "")
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", io.EOF
		}
		return "", fmt.Errorf("failed reading location: %w", err)
	}

	return strings.TrimSpace(input), nil
}

// promptForUserNote prompts for an optional user note.
func (e *MenuExecutor) promptForUserNote(
	s ssh.Session,
	terminal *term.Terminal,
	nodeNumber int,
	outputMode ansi.OutputMode,
) (string, error) {
	prompt := e.LoadedStrings.EnterUserNote
	if prompt == "" {
		prompt = "|08D|07e|15sired |08U|07s|15er |08N|07o|15te |09: "
	}

	terminalio.WriteStringCP437(terminal, ansi.ReplacePipeCodes([]byte("\r\n"+prompt)), outputMode)

	input, err := styledInput(terminal, s, outputMode, newUserNoteMaxLen, "")
	if err != nil {
		if errors.Is(err, io.EOF) {
			return "", io.EOF
		}
		return "", fmt.Errorf("failed reading user note: %w", err)
	}

	return strings.TrimSpace(input), nil
}

// validateHandle checks a handle against rules from Pascal ValidUserName().
// Rules: not empty, min 3 chars, no special chars (?#/*&:),
// cannot be "new" or "q", cannot be purely numeric.
func validateHandle(handle string) bool {
	if len(handle) < 3 {
		return false
	}

	// Cannot be reserved words
	lower := strings.ToLower(handle)
	if lower == "new" || lower == "q" || lower == "sysop" {
		return false
	}

	// Cannot contain special characters
	invalidChars := "?#/*&:"
	for _, c := range handle {
		if strings.ContainsRune(invalidChars, c) {
			return false
		}
	}

	// Cannot be purely numeric
	allDigits := true
	for _, c := range handle {
		if !unicode.IsDigit(c) {
			allDigits = false
			break
		}
	}
	if allDigits {
		return false
	}

	return true
}

// validateRealName checks that a real name is >3 chars and contains a space.
func validateRealName(name string) bool {
	if len(name) < 4 {
		return false
	}
	return strings.Contains(name, " ")
}

// runNewUser is the RUN:NEWUSER handler for menu system integration.
func runNewUser(e *MenuExecutor, s ssh.Session, terminal *term.Terminal, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode ansi.OutputMode, termWidth int, termHeight int) (*user.User, string, error) {
	err := e.handleNewUserApplication(s, terminal, userManager, nodeNumber, outputMode, termWidth, termHeight)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, "LOGOFF", io.EOF
		}
		return nil, "", err
	}
	return nil, "", nil
}
