package menu

import (
	"fmt"
	"log"
	"time"

	"github.com/gliderlabs/ssh"
	terminalPkg "github.com/stlalpha/vision3/internal/terminal"
	"github.com/stlalpha/vision3/internal/user"
)

// RunnableFunc defines the signature for functions executable via RUN:
// Returns: authenticatedUser, nextAction (e.g., "GOTO:MENU"), err
type RunnableFunc func(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (authenticatedUser *user.User, nextAction string, err error)

// registerPlaceholderRunnables registers placeholder commands that are not yet fully implemented.
func registerPlaceholderRunnables(registry map[string]RunnableFunc) {
	// Keep READMAIL as a placeholder for now
	registry["READMAIL"] = func(e *MenuExecutor, s ssh.Session, terminal *terminalPkg.BBS, userManager *user.UserMgr, currentUser *user.User, nodeNumber int, sessionStartTime time.Time, args string, outputMode terminalPkg.OutputMode) (*user.User, string, error) {
		if currentUser == nil {
			log.Printf("WARN: Node %d: READMAIL called without logged in user.", nodeNumber)
			msg := "\r\n|01Error: You must be logged in to read mail.|07\r\n"
			wErr := terminal.DisplayContent([]byte(msg))
			if wErr != nil {
				log.Printf("ERROR: Failed writing READMAIL error message: %v", wErr)
			}
			time.Sleep(1 * time.Second)
			return nil, "", nil // No user change, no next action, no error
		}
		msg := fmt.Sprintf("\r\n|15Executing |11READMAIL|15 for |14%s|15... (Not Implemented)|07\r\n", currentUser.Handle)
		wErr := terminal.DisplayContent([]byte(msg))
		if wErr != nil {
			log.Printf("ERROR: Failed writing READMAIL placeholder message: %v", wErr)
		}
		time.Sleep(500 * time.Millisecond)
		return nil, "", nil // No user change, no next action, no error
	}

	// Register DOOR handler - implementation in runnables_doors.go
	registry["DOOR:"] = runDoor
}

// registerAppRunnables registers the actual application command functions.
func registerAppRunnables(registry map[string]RunnableFunc) {
	registry["SHOWSTATS"] = runShowStats
	registry["LASTCALLERS"] = runLastCallers
	registry["AUTHENTICATE"] = runAuthenticate
	registry["ONELINER"] = runOneliners                              // Register new placeholder
	registry["FULL_LOGIN_SEQUENCE"] = runFullLoginSequence           // Register the new sequence
	registry["SHOWVERSION"] = runShowVersion                         // Register the version display runnable
	registry["LISTUSERS"] = runListUsers                             // Register the user list runnable
	registry["LISTMSGAR"] = runListMessageAreas                      // <-- ADDED: Register message area list runnable
	registry["COMPOSEMSG"] = runComposeMessage                       // <-- ADDED: Register compose message runnable
	registry["PROMPTANDCOMPOSEMESSAGE"] = runPromptAndComposeMessage // <-- ADDED: Register prompt/compose runnable (Corrected key to uppercase)
	registry["READMSGS"] = runReadMsgs                               // <-- ADDED: Register message reading runnable
	registry["NEWSCAN"] = runNewscan                                 // <-- ADDED: Register newscan runnable
	registry["LISTFILES"] = runListFiles                             // <-- ADDED: Register file list runnable
	registry["LISTFILEAR"] = runListFileAreas                        // <-- ADDED: Register file area list runnable
	registry["SELECTFILEAREA"] = runSelectFileArea                   // <-- ADDED: Register file area selection runnable
	registry["SETRENDER"] = runSetRender                             // Register renderer configuration command
}
