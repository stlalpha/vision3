package types

// AutoRunTracker keeps track of which run-once ('//') commands have executed in a session.
// Key format: "menuName:commandString"
type AutoRunTracker map[string]bool
