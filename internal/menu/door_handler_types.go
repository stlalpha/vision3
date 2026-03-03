package menu

import (
	"time"

	"github.com/gliderlabs/ssh"
	"github.com/stlalpha/vision3/internal/ansi"
	"github.com/stlalpha/vision3/internal/config"
	"golang.org/x/term"
)

// doorUserInfo holds the user fields needed for dropfile generation.
type doorUserInfo struct {
	ID            int
	Handle        string
	RealName      string
	AccessLevel   int
	TimeLimit     int
	TimesCalled   int
	PhoneNumber   string
	GroupLocation string
	ScreenWidth   int
	ScreenHeight  int
}

// DoorCtx holds all context needed to execute a door program.
type DoorCtx struct {
	Executor         *MenuExecutor
	Session          ssh.Session
	Terminal         *term.Terminal
	User             doorUserInfo
	NodeNumber       int
	SessionStartTime time.Time
	OutputMode       ansi.OutputMode
	Config           config.DoorConfig
	DoorName         string
	// Pre-computed values
	NodeNumStr  string
	PortStr     string
	TimeLeftMin int
	TimeLeftStr string
	BaudStr     string
	UserIDStr   string
	Subs        map[string]string
}
