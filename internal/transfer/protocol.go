package transfer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gliderlabs/ssh"
)

// ErrBinaryNotFound is returned by ExecuteSend/ExecuteReceive when the
// external transfer binary (e.g. sexyz) cannot be found on disk. Callers
// should check errors.Is(err, ErrBinaryNotFound) and display a sysop-friendly
// message to the user instead of a generic "transfer failed" notice.
var ErrBinaryNotFound = errors.New("transfer binary not found")

// Connection type constants for ProtocolConfig.ConnectionType.
const (
	ConnTypeAny    = ""       // Available on all connection types
	ConnTypeSSH    = "ssh"    // SSH sessions only
	ConnTypeTelnet = "telnet" // Telnet sessions only
)

// ProtocolConfig defines a user-visible file transfer protocol.
// The send/receive commands are external programs (e.g., lrzsz, sexyz).
//
// Argument placeholders:
//
//	{filePath}     — expanded to one or more file paths (send only)
//	{fileListPath} — replaced by a temp file path containing one filename per
//	                 line; commonly prefixed with "@" for sexyz (e.g. "@{fileListPath}")
//	{targetDir}    — expanded to the upload target directory (recv only)
//
// If {filePath} is absent from send_args, file paths are appended at the end.
type ProtocolConfig struct {
	Key            string   `json:"key"`             // Selection key shown to users (e.g. "Z", "ZST")
	Name           string   `json:"name"`            // Display name shown to users
	Description    string   `json:"description"`     // Short description for help text
	SendCmd        string   `json:"send_cmd"`        // Executable for sending (download to user)
	SendArgs       []string `json:"send_args"`       // Arguments for send command
	RecvCmd        string   `json:"recv_cmd"`        // Executable for receiving (upload from user)
	RecvArgs       []string `json:"recv_args"`       // Arguments for receive command
	BatchSend      bool     `json:"batch_send"`      // True if the protocol supports multi-file batch sends
	UsePTY         bool     `json:"use_pty"`         // True if the command requires a PTY
	Default        bool     `json:"default"`         // True if this is the default protocol when none is selected
	ConnectionType string   `json:"connection_type"` // "" = any, "ssh" = SSH only, "telnet" = telnet only
}

// defaultProtocols returns built-in defaults.
func defaultProtocols() []ProtocolConfig {
	return []ProtocolConfig{{Key: "Z", Name: "Zmodem", Description: "Zmodem (lrzsz)", SendCmd: "sz", SendArgs: []string{"-b", "-e"}, RecvCmd: "rz", RecvArgs: []string{"-b", "-r"}, BatchSend: true, UsePTY: true, Default: true}}
}

// LoadProtocols reads a JSON array of ProtocolConfig definitions from path.
func LoadProtocols(path string) ([]ProtocolConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: protocols file not found, using built-in defaults")
			return defaultProtocols(), nil
		}
		return nil, fmt.Errorf("failed to read protocols file %q: %w", path, err)
	}
	var protocols []ProtocolConfig
	if err := json.Unmarshal(data, &protocols); err != nil {
		return nil, fmt.Errorf("failed to parse protocols file %q: %w", path, err)
	}
	return protocols, nil
}

func FindProtocol(ps []ProtocolConfig, key string) (ProtocolConfig, bool) {
	u := strings.ToUpper(key)
	for _, p := range ps {
		if strings.ToUpper(p.Key) == u {
			return p, true
		}
	}
	d, _ := DefaultProtocol(ps)
	return d, false
}

// DefaultProtocol returns the first protocol marked as default, or the first
// protocol in the slice if none is marked default. Returns false if the slice
// is empty.
func DefaultProtocol(protocols []ProtocolConfig) (ProtocolConfig, bool) {
	if len(protocols) == 0 {
		return ProtocolConfig{}, false
	}
	for _, p := range protocols {
		if p.Default {
			return p, true
		}
	}
	return protocols[0], true
}

// ExecuteSend runs this protocol's send command to transfer files to the user.
// filePaths must be absolute paths to the files being sent.
// ctx controls cancellation and timeout; when ctx.Done() fires, the transfer is aborted.
// Pass a context with timeout (e.g. context.WithTimeout) for transfer time limits.
func (p *ProtocolConfig) ExecuteSend(ctx context.Context, s ssh.Session, filePaths ...string) error {
	if len(filePaths) == 0 {
		return fmt.Errorf("no files provided for send via protocol %q", p.Name)
	}
	// Validate all paths are absolute.
	for _, fp := range filePaths {
		if !filepath.IsAbs(fp) {
			return fmt.Errorf("send path must be absolute, got %q", fp)
		}
	}

	cmdPath, err := exec.LookPath(p.SendCmd)
	if err != nil {
		log.Printf("ERROR: send command %q not found for protocol %q: %v", p.SendCmd, p.Name, err)
		return fmt.Errorf("%w: send command %q not found for protocol %q — see docs/sysop/files/file-transfer.md", ErrBinaryNotFound, p.SendCmd, p.Name)
	}
	// Resolve to absolute path so relative binary paths survive any working-directory changes.
	if !filepath.IsAbs(cmdPath) {
		if abs, absErr := filepath.Abs(cmdPath); absErr == nil {
			cmdPath = abs
		}
	}

	args, listFile := expandArgs(p.SendArgs, filePaths, "")
	if listFile != "" {
		defer os.Remove(listFile)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, cmdPath, args...)

	log.Printf("INFO: Protocol %q send: %s %v (pty=%v)", p.Name, cmdPath, args, p.UsePTY)
	if p.UsePTY {
		return RunCommandWithPTY(ctx, s, cmd, 0)
	}
	return RunCommandDirect(ctx, s, cmd, 0)
}

// ExecuteReceive runs this protocol's receive command to accept files from the user.
// targetDir must be an absolute path to the directory where received files will be stored.
// ctx controls cancellation and timeout; when ctx.Done() fires, the transfer is aborted.
func (p *ProtocolConfig) ExecuteReceive(ctx context.Context, s ssh.Session, targetDir string) error {
	if targetDir == "" {
		return fmt.Errorf("target directory cannot be empty for receive via protocol %q", p.Name)
	}
	if !filepath.IsAbs(targetDir) {
		return fmt.Errorf("target directory must be absolute, got %q", targetDir)
	}

	cmdPath, err := exec.LookPath(p.RecvCmd)
	if err != nil {
		log.Printf("ERROR: recv command %q not found for protocol %q: %v", p.RecvCmd, p.Name, err)
		return fmt.Errorf("%w: recv command %q not found for protocol %q — see docs/sysop/files/file-transfer.md", ErrBinaryNotFound, p.RecvCmd, p.Name)
	}
	// Resolve to absolute path so that cmd.Dir does not break relative binary paths.
	if !filepath.IsAbs(cmdPath) {
		if abs, absErr := filepath.Abs(cmdPath); absErr == nil {
			cmdPath = abs
		}
	}

	args, listFile := expandArgs(p.RecvArgs, nil, targetDir)
	if listFile != "" {
		defer os.Remove(listFile)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cmd := exec.CommandContext(ctx, cmdPath, args...)
	cmd.Dir = targetDir

	// Apply an idle-timeout only for sexyz receive: sexyz loops re-sending
	// ZRINIT indefinitely when the user cancels without a ZModem CAN abort.
	// Keep this shorter than its ZRINIT retransmit interval (~15 s) so the
	// kill fires before SyncTerm re-opens the upload dialog.
	// CAN bytes do NOT reset this timer; CAN abort is handled separately in
	// RunCommandDirect. Other protocols get no idle timeout (0).
	var recvIdleTimeout time.Duration
	if strings.EqualFold(filepath.Base(cmdPath), "sexyz") {
		recvIdleTimeout = 10 * time.Second
	}

	log.Printf("INFO: Protocol %q receive in %s: %s %v (pty=%v)", p.Name, targetDir, cmdPath, args, p.UsePTY)
	if p.UsePTY {
		return RunCommandWithPTY(ctx, s, cmd, recvIdleTimeout)
	}
	return RunCommandDirect(ctx, s, cmd, recvIdleTimeout)
}

// expandArgs substitutes placeholders in a command argument template.
//
// Rules:
//   - A standalone "{filePath}" arg is replaced by all filePaths (one arg each).
//   - A standalone "{targetDir}" arg is replaced by targetDir (always ends with
//     a path separator — some external programs like sexyz concatenate the
//     directory path with the filename without adding a separator).
//   - "{fileListPath}" (standalone or inline) is replaced by the path to a
//     temporary file containing one filename per line. Callers must clean up
//     the returned listFile path (if non-empty) after the command completes.
//   - Inline occurrences (e.g. "@{fileListPath}") perform in-place substitution.
//   - If {filePath} never appears as a standalone arg and {fileListPath} is not
//     used either, filePaths are appended at the end.
func expandArgs(template []string, filePaths []string, targetDir string) ([]string, string) {
	var result []string
	filePathUsed := false
	var listFilePath string

	// Ensure targetDir always ends with a path separator.
	// Some external programs (e.g. Synchronet sexyz) concatenate the directory
	// path + filename without inserting a separator, producing a mangled path
	// like "/tmp/.incoming-123MYFILE.ZIP" instead of "/tmp/.incoming-123/MYFILE.ZIP".
	if targetDir != "" && !strings.HasSuffix(targetDir, string(filepath.Separator)) {
		targetDir += string(filepath.Separator)
	}

	for _, arg := range template {
		switch arg {
		case "{filePath}":
			result = append(result, filePaths...)
			filePathUsed = true
		case "{targetDir}":
			result = append(result, targetDir)
		case "{fileListPath}":
			// Write a temp file with one path per line.
			if listFilePath == "" && len(filePaths) > 0 {
				listFilePath = writeFileList(filePaths)
			}
			result = append(result, listFilePath)
			filePathUsed = true
		default:
			a := arg
			if strings.Contains(a, "{fileListPath}") && len(filePaths) > 0 {
				if listFilePath == "" {
					listFilePath = writeFileList(filePaths)
				}
				a = strings.ReplaceAll(a, "{fileListPath}", listFilePath)
				filePathUsed = true
			}
			if len(filePaths) > 0 && strings.Contains(a, "{filePath}") {
				a = strings.ReplaceAll(a, "{filePath}", filePaths[0])
				filePathUsed = true
			}
			a = strings.ReplaceAll(a, "{targetDir}", targetDir)
			result = append(result, a)
		}
	}

	// Append file paths at end only when {filePath} never appeared (standalone or inline).
	if !filePathUsed && len(filePaths) > 0 {
		result = append(result, filePaths...)
	}

	return result, listFilePath
}

// writeFileList creates a temporary file containing one path per line and
// returns the file path in the OS temp directory.  Returns "" on error.
func writeFileList(paths []string) string {
	f, err := os.CreateTemp("", "sexyz-filelist-*.txt")
	if err != nil {
		log.Printf("ERROR: failed to create file list temp file: %v", err)
		return ""
	}
	for _, p := range paths {
		fmt.Fprintln(f, p)
	}
	_ = f.Close()
	return f.Name()
}
