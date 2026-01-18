// Package menu provides menu system functionality for ViSiON/3 BBS.
//
// dispatcher.go contains command parsing and routing logic for menu commands.
// It handles the dispatch of GOTO, RUN, DOOR, and LOGOFF commands.
package menu

import (
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/user"
)

// CommandType represents the type of a parsed command.
type CommandType int

const (
	// CommandTypeUnknown indicates an unrecognized command type.
	CommandTypeUnknown CommandType = iota
	// CommandTypeGoto indicates a GOTO:MENU navigation command.
	CommandTypeGoto
	// CommandTypeLogoff indicates a LOGOFF disconnect command.
	CommandTypeLogoff
	// CommandTypeRun indicates a RUN:FUNCTION command to execute a runnable.
	CommandTypeRun
	// CommandTypeDoor indicates a DOOR:NAME command to launch an external program.
	CommandTypeDoor
)

// ActionType constants represent the result types from command execution.
const (
	// ActionTypeGoto indicates the menu system should navigate to another menu.
	ActionTypeGoto = "GOTO"
	// ActionTypeLogoff indicates the user should be logged off.
	ActionTypeLogoff = "LOGOFF"
	// ActionTypeContinue indicates the current menu should continue execution.
	ActionTypeContinue = "CONTINUE"
)

// ParseCommand parses a command string and returns its type and argument.
// For GOTO:MENU, it returns CommandTypeGoto and the uppercased menu name.
// For RUN:FUNCTION, it returns CommandTypeRun and the uppercased function string (target + args).
// For DOOR:NAME, it returns CommandTypeDoor and the door name.
// For LOGOFF, it returns CommandTypeLogoff and an empty string.
// For unknown commands, it returns CommandTypeUnknown and the original string.
func ParseCommand(command string) (CommandType, string) {
	if command == "" {
		return CommandTypeUnknown, ""
	}

	if strings.HasPrefix(command, "GOTO:") {
		menuName := strings.ToUpper(strings.TrimPrefix(command, "GOTO:"))
		return CommandTypeGoto, menuName
	}

	if command == "LOGOFF" {
		return CommandTypeLogoff, ""
	}

	if strings.HasPrefix(command, "RUN:") {
		// Extract function name and optional arguments
		remainder := strings.TrimPrefix(command, "RUN:")
		// Split into target and args, uppercase the target
		parts := strings.SplitN(remainder, " ", 2)
		target := strings.ToUpper(parts[0])
		if len(parts) > 1 {
			return CommandTypeRun, target + " " + parts[1]
		}
		return CommandTypeRun, target
	}

	if strings.HasPrefix(command, "DOOR:") {
		doorName := strings.TrimPrefix(command, "DOOR:")
		return CommandTypeDoor, doorName
	}

	return CommandTypeUnknown, command
}

// ExtractRunTarget splits a RUN command argument into target name and arguments.
// For "SETRENDER MODE=LIGHTBAR", it returns ("SETRENDER", "MODE=LIGHTBAR").
// For "LASTCALLERS", it returns ("LASTCALLERS", "").
func ExtractRunTarget(runArg string) (target string, args string) {
	if runArg == "" {
		return "", ""
	}

	parts := strings.SplitN(runArg, " ", 2)
	target = parts[0]
	if len(parts) > 1 {
		args = parts[1]
	}
	return target, args
}

// executeCommandAction handles the logic for executing a command string (GOTO, RUN, DOOR, LOGOFF).
// Returns: actionType (GOTO, LOGOFF, CONTINUE), nextMenu, resultingUser, error
func (e *MenuExecutor) executeCommandAction(action string, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, outputMode terminalPkg.OutputMode) (actionType string, nextMenu string, userResult *user.User, err error) {
	cmdType, cmdArg := ParseCommand(action)

	switch cmdType {
	case CommandTypeGoto:
		return ActionTypeGoto, cmdArg, currentUser, nil

	case CommandTypeLogoff:
		return ActionTypeLogoff, "", currentUser, nil

	case CommandTypeRun:
		return e.executeRunCommand(cmdArg, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, outputMode)

	case CommandTypeDoor:
		return e.executeDoorCommand(cmdArg, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, outputMode)

	default:
		log.Printf("WARN: Unhandled command action type in executeCommandAction: %s", action)
		return ActionTypeContinue, "", currentUser, nil
	}
}

// executeRunCommand handles RUN:FUNCTION command execution.
// It looks up the function in the registry and executes it.
func (e *MenuExecutor) executeRunCommand(cmdArg string, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, outputMode terminalPkg.OutputMode) (actionType string, nextMenu string, userResult *user.User, err error) {
	runTarget, runArgs := ExtractRunTarget(cmdArg)
	log.Printf("INFO: Executing RUN action: Target='%s' Args='%s'", runTarget, runArgs)

	runnableFunc, exists := e.RunRegistry[runTarget]
	if !exists {
		log.Printf("WARN: No internal function registered for RUN:%s", runTarget)
		msg := fmt.Sprintf("\r\n|01Internal command '%s' not found.|07\r\n", runTarget)
		terminal.DisplayContent([]byte(msg))
		time.Sleep(1 * time.Second)
		return ActionTypeContinue, "", currentUser, nil
	}

	log.Printf("DEBUG: Node %d: Calling registered function for RUN:%s", nodeNumber, runTarget)
	authUser, nextActionStr, runErr := runnableFunc(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, runArgs, outputMode)

	if runErr != nil {
		if errors.Is(runErr, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during RUN:%s execution.", nodeNumber, runTarget)
			return ActionTypeLogoff, "", nil, nil
		}
		log.Printf("ERROR: RUN:%s function failed: %v", runTarget, runErr)
		errMsg := fmt.Sprintf("\r\n|01Error running command '%s': %v|07\r\n", runTarget, runErr)
		terminal.DisplayContent([]byte(errMsg))
		time.Sleep(1 * time.Second)
		// Assign the potentially updated user before returning
		userResult = authUser                            // Capture potential user changes (like from AUTHENTICATE)
		return ActionTypeContinue, "", userResult, runErr // Continue but report error
	}

	log.Printf("DEBUG: RUN:%s function completed.", runTarget)

	// Check if the runnable function returned a specific next action
	return e.handleRunnableResult(runTarget, authUser, nextActionStr)
}

// executeDoorCommand handles DOOR:NAME command execution.
// It looks up the DOOR handler in the registry and executes it.
func (e *MenuExecutor) executeDoorCommand(doorTarget string, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, outputMode terminalPkg.OutputMode) (actionType string, nextMenu string, userResult *user.User, err error) {
	log.Printf("INFO: Executing DOOR action: '%s'", doorTarget)

	doorFunc, exists := e.RunRegistry["DOOR:"]
	if !exists {
		log.Printf("CRITICAL: DOOR: function not registered!")
		return ActionTypeContinue, "", currentUser, nil
	}

	// DOOR runnable returns user, "", error
	userResultDoor, nextActionStrDoor, doorErr := doorFunc(e, s, terminal, userManager, currentUser, nodeNumber, sessionStartTime, doorTarget, outputMode)

	if doorErr != nil {
		if errors.Is(doorErr, io.EOF) {
			log.Printf("INFO: Node %d: User disconnected during DOOR:%s execution.", nodeNumber, doorTarget)
			return ActionTypeLogoff, "", nil, nil
		}
		log.Printf("ERROR: DOOR:%s execution failed: %v", doorTarget, doorErr)
		errMsg := fmt.Sprintf("\r\n|01Error running door '%s': %v|07\r\n", doorTarget, doorErr)
		terminal.DisplayContent([]byte(errMsg))
		time.Sleep(1 * time.Second)
		// Assign potential user result before returning
		userResult = userResultDoor
		return ActionTypeContinue, "", userResult, doorErr // Continue after door error
	}

	// Handle potential LOGOFF request from DOOR runnable (though currently returns "")
	if nextActionStrDoor == ActionTypeLogoff {
		log.Printf("DEBUG: DOOR:%s requested LOGOFF", doorTarget)
		return ActionTypeLogoff, "", userResultDoor, nil
	}

	log.Printf("DEBUG: DOOR:%s completed.", doorTarget)
	return ActionTypeContinue, "", userResultDoor, nil // Default CONTINUE after door
}

// handleRunnableResult processes the result from a runnable function and
// determines the appropriate action type and next menu.
func (e *MenuExecutor) handleRunnableResult(runTarget string, authUser *user.User, nextActionStr string) (actionType string, nextMenu string, userResult *user.User, err error) {
	if strings.HasPrefix(nextActionStr, "GOTO:") {
		nextMenu = strings.ToUpper(strings.TrimPrefix(nextActionStr, "GOTO:"))
		log.Printf("DEBUG: RUN:%s requested GOTO:%s", runTarget, nextMenu)
		return ActionTypeGoto, nextMenu, authUser, nil
	}

	if nextActionStr == ActionTypeLogoff {
		log.Printf("DEBUG: RUN:%s requested LOGOFF", runTarget)
		return ActionTypeLogoff, "", authUser, nil
	}

	// Default action for RUN is CONTINUE
	return ActionTypeContinue, "", authUser, nil
}
