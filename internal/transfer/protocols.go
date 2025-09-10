package transfer

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/gliderlabs/ssh"
)

// Protocol defines a file transfer protocol configuration
type Protocol struct {
	Name         string   // Human readable name (e.g., "ZMODEM", "XMODEM")
	SendCommand  string   // Command for sending files (e.g., "sz", "sx")
	RecvCommand  string   // Command for receiving files (e.g., "rz", "rx")
	SendArgs     []string // Base arguments for sending
	RecvArgs     []string // Base arguments for receiving
	Description  string   // Protocol description
	RequiresPTY  bool     // Whether protocol requires PTY
	MultiFile    bool     // Whether protocol supports multiple files
}

// Predefined protocols
var (
	ZMODEM = Protocol{
		Name:        "ZMODEM",
		SendCommand: "sz",
		RecvCommand: "rz", 
		SendArgs:    []string{"-b"},        // Binary mode
		RecvArgs:    []string{"-b"},        // Binary mode
		Description: "ZMODEM protocol (fastest, resume capable)",
		RequiresPTY: true,
		MultiFile:   true,
	}
	
	ZMODEM_8K = Protocol{
		Name:        "ZMODEM-8K",
		SendCommand: "sz",
		RecvCommand: "rz",
		SendArgs:    []string{"-b", "-8"},  // Binary mode, 8K blocks
		RecvArgs:    []string{"-b", "-8"},  // Binary mode, 8K blocks  
		Description: "ZMODEM with 8K blocks (faster for large files)",
		RequiresPTY: true,
		MultiFile:   true,
	}
	
	YMODEM = Protocol{
		Name:        "YMODEM", 
		SendCommand: "sb",
		RecvCommand: "rb",
		SendArgs:    []string{"-k"},        // 1K blocks
		RecvArgs:    []string{"-k"},        // 1K blocks
		Description: "YMODEM protocol (batch capable)",
		RequiresPTY: true,
		MultiFile:   true,
	}
	
	XMODEM = Protocol{
		Name:        "XMODEM",
		SendCommand: "sx", 
		RecvCommand: "rx",
		SendArgs:    []string{"-k"},        // 1K blocks
		RecvArgs:    []string{"-k"},        // 1K blocks
		Description: "XMODEM protocol (single file only)",
		RequiresPTY: true,
		MultiFile:   false,
	}
	
	XMODEM_CRC = Protocol{
		Name:        "XMODEM-CRC",
		SendCommand: "sx",
		RecvCommand: "rx", 
		SendArgs:    []string{"-k", "-c"}, // 1K blocks, CRC
		RecvArgs:    []string{"-k", "-c"}, // 1K blocks, CRC
		Description: "XMODEM with CRC (more reliable)",
		RequiresPTY: true,
		MultiFile:   false,
	}
)

// ProtocolManager handles all file transfer protocols
type ProtocolManager struct {
	protocols map[string]Protocol
}

// NewProtocolManager creates a new protocol manager
func NewProtocolManager() *ProtocolManager {
	pm := &ProtocolManager{
		protocols: make(map[string]Protocol),
	}
	
	// Register built-in protocols
	pm.RegisterProtocol(ZMODEM)
	pm.RegisterProtocol(ZMODEM_8K)
	pm.RegisterProtocol(YMODEM) 
	pm.RegisterProtocol(XMODEM)
	pm.RegisterProtocol(XMODEM_CRC)
	
	return pm
}

// RegisterProtocol adds a protocol to the manager
func (pm *ProtocolManager) RegisterProtocol(protocol Protocol) {
	pm.protocols[protocol.Name] = protocol
}

// GetProtocol retrieves a protocol by name
func (pm *ProtocolManager) GetProtocol(name string) (Protocol, bool) {
	protocol, exists := pm.protocols[name]
	return protocol, exists
}

// ListAvailableProtocols returns protocols that have their commands available
func (pm *ProtocolManager) ListAvailableProtocols() []Protocol {
	var available []Protocol
	
	for _, protocol := range pm.protocols {
		// Check if both send and receive commands exist
		if _, err := exec.LookPath(protocol.SendCommand); err == nil {
			if _, err := exec.LookPath(protocol.RecvCommand); err == nil {
				available = append(available, protocol)
			}
		}
	}
	
	return available
}

// SendFiles sends files using the specified protocol
func (pm *ProtocolManager) SendFiles(s ssh.Session, protocolName string, filePaths ...string) error {
	protocol, exists := pm.protocols[protocolName]
	if !exists {
		return fmt.Errorf("protocol %s not found", protocolName)
	}
	
	if len(filePaths) == 0 {
		return fmt.Errorf("no files provided for %s send", protocol.Name)
	}
	
	if !protocol.MultiFile && len(filePaths) > 1 {
		return fmt.Errorf("protocol %s does not support multiple files", protocol.Name)
	}
	
	// Check if command exists
	cmdPath, err := exec.LookPath(protocol.SendCommand)
	if err != nil {
		return fmt.Errorf("'%s' command not found, %s send unavailable", protocol.SendCommand, protocol.Name)
	}
	
	// Build command arguments
	args := append(protocol.SendArgs, filePaths...)
	cmd := exec.Command(cmdPath, args...)
	
	log.Printf("INFO: Executing %s send: %s %v", protocol.Name, cmdPath, args)
	
	// Execute the command
	if protocol.RequiresPTY {
		err = RunCommandWithPTY(s, cmd)
	} else {
		cmd.Stdin = s
		cmd.Stdout = s  
		cmd.Stderr = s
		err = cmd.Run()
	}
	
	if err != nil {
		log.Printf("ERROR: %s send failed: %v", protocol.Name, err)
		return fmt.Errorf("%s send failed: %w", protocol.Name, err)
	}
	
	log.Printf("INFO: %s send completed successfully for files: %v", protocol.Name, filePaths)
	return nil
}

// ReceiveFiles receives files using the specified protocol
func (pm *ProtocolManager) ReceiveFiles(s ssh.Session, protocolName string, targetDir string) error {
	protocol, exists := pm.protocols[protocolName] 
	if !exists {
		return fmt.Errorf("protocol %s not found", protocolName)
	}
	
	// Validate and create target directory
	if targetDir == "" {
		return fmt.Errorf("target directory cannot be empty for %s receive", protocol.Name)
	}
	
	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for target directory '%s': %w", targetDir, err)
	}
	
	if err := os.MkdirAll(absTargetDir, 0755); err != nil {
		if fileInfo, statErr := os.Stat(absTargetDir); !(statErr == nil && fileInfo.IsDir()) {
			return fmt.Errorf("failed to create or access target directory '%s': %w", absTargetDir, err)
		}
	}
	
	// Check if command exists
	cmdPath, err := exec.LookPath(protocol.RecvCommand)
	if err != nil {
		return fmt.Errorf("'%s' command not found, %s receive unavailable", protocol.RecvCommand, protocol.Name)
	}
	
	// Build command
	cmd := exec.Command(cmdPath, protocol.RecvArgs...)
	cmd.Dir = absTargetDir // Run in target directory
	
	log.Printf("INFO: Executing %s receive in directory '%s': %s %v", protocol.Name, absTargetDir, cmdPath, protocol.RecvArgs)
	
	// Execute the command
	if protocol.RequiresPTY {
		err = RunCommandWithPTY(s, cmd)
	} else {
		cmd.Stdin = s
		cmd.Stdout = s
		cmd.Stderr = s
		err = cmd.Run()
	}
	
	if err != nil {
		log.Printf("ERROR: %s receive failed: %v", protocol.Name, err)
		return fmt.Errorf("%s receive failed: %w", protocol.Name, err)
	}
	
	log.Printf("INFO: %s receive completed successfully into directory: %s", protocol.Name, absTargetDir)
	return nil
}

