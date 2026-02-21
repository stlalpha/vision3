package transfer

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gliderlabs/ssh"
)

// ProtocolConfig defines a user-visible file transfer protocol.
// The send/receive commands are external programs (e.g., lrzsz, sexyz).
//
// Argument placeholders:
//
//	{filePath}  — expanded to one or more file paths (send only)
//	{targetDir} — expanded to the upload target directory (recv only)
//
// If {filePath} is absent from send_args, file paths are appended at the end.
type ProtocolConfig struct {
	Key         string   `json:"key"`         // Machine-readable key (e.g. "zmodem-lrzsz")
	Name        string   `json:"name"`        // Display name shown to users
	Description string   `json:"description"` // Short description for help text
	SendCmd     string   `json:"send_cmd"`    // Executable for sending (download to user)
	SendArgs    []string `json:"send_args"`   // Arguments for send command
	RecvCmd     string   `json:"recv_cmd"`    // Executable for receiving (upload from user)
	RecvArgs    []string `json:"recv_args"`   // Arguments for receive command
	BatchSend   bool     `json:"batch_send"`  // True if the protocol supports multi-file batch sends
	UsePTY      bool     `json:"use_pty"`     // True if the command requires a PTY
	Default     bool     `json:"default"`     // True if this is the default protocol when none is selected
}

// defaultProtocols returns built-in defaults.
func defaultProtocols() []ProtocolConfig {return []ProtocolConfig{{Key:"Z",Name:"Zmodem",Description:"Zmodem (lrzsz)",SendCmd:"sz",SendArgs:[]string{"-b","-e"},RecvCmd:"rz",RecvArgs:[]string{"-b","-r"},BatchSend:true,UsePTY:true,Default:true}}}

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

func FindProtocol(ps []ProtocolConfig,key string)(ProtocolConfig,bool){u:=strings.ToUpper(key);for _,p:=range ps{if strings.ToUpper(p.Key)==u{return p,true}};d,_:=DefaultProtocol(ps);return d,false}

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
func (p *ProtocolConfig) ExecuteSend(s ssh.Session, filePaths ...string) error {
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
		return fmt.Errorf("send command %q not found for protocol %q: %w", p.SendCmd, p.Name, err)
	}

	args := expandArgs(p.SendArgs, filePaths, "")
	cmd := exec.Command(cmdPath, args...)

	log.Printf("INFO: Protocol %q send: %s %v", p.Name, cmdPath, args)
	return RunCommandWithPTY(s, cmd)
}

// ExecuteReceive runs this protocol's receive command to accept files from the user.
// targetDir must be an absolute path to the directory where received files will be stored.
func (p *ProtocolConfig) ExecuteReceive(s ssh.Session, targetDir string) error {
	if targetDir == "" {
		return fmt.Errorf("target directory cannot be empty for receive via protocol %q", p.Name)
	}
	if !filepath.IsAbs(targetDir) {
		return fmt.Errorf("target directory must be absolute, got %q", targetDir)
	}

	cmdPath, err := exec.LookPath(p.RecvCmd)
	if err != nil {
		log.Printf("ERROR: recv command %q not found for protocol %q: %v", p.RecvCmd, p.Name, err)
		return fmt.Errorf("recv command %q not found for protocol %q: %w", p.RecvCmd, p.Name, err)
	}

	args := expandArgs(p.RecvArgs, nil, targetDir)
	cmd := exec.Command(cmdPath, args...)
	cmd.Dir = targetDir

	log.Printf("INFO: Protocol %q receive in %s: %s %v", p.Name, targetDir, cmdPath, args)
	return RunCommandWithPTY(s, cmd)
}

// expandArgs substitutes placeholders in a command argument template.
//
// Rules:
//   - A standalone "{filePath}" arg is replaced by all filePaths (one arg each).
//   - A standalone "{targetDir}" arg is replaced by targetDir.
//   - Inline occurrences (e.g. "sz:{filePath}") use only the first filePath.
//   - If {filePath} never appears as a standalone arg, filePaths are appended at the end.
func expandArgs(template []string, filePaths []string, targetDir string) []string {
	var result []string
	filePathUsed := false

	for _, arg := range template {
		switch arg {
		case "{filePath}":
			result = append(result, filePaths...)
			filePathUsed = true
		case "{targetDir}":
			result = append(result, targetDir)
		default:
			a := arg
			if len(filePaths) > 0 {
				a = strings.ReplaceAll(a, "{filePath}", filePaths[0])
			}
			a = strings.ReplaceAll(a, "{targetDir}", targetDir)
			result = append(result, a)
		}
	}

	// Append file paths at end if the template had no standalone {filePath}.
	if !filePathUsed && len(filePaths) > 0 {
		result = append(result, filePaths...)
	}

	return result
}
