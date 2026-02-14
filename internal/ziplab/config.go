package ziplab

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// StepConfig holds the configuration for an individual ZipLab pipeline step.
type StepConfig struct {
	Enabled bool   `json:"enabled"`
	Command string `json:"command,omitempty"` // External command path
	Args    []string `json:"args,omitempty"`  // Command arguments (supports {FILE}, {WORKDIR} placeholders)
	Timeout int    `json:"timeoutSeconds,omitempty"` // Timeout in seconds (0 = default 60s)
}

// RemoveAdsConfig extends StepConfig with a patterns file.
type RemoveAdsConfig struct {
	StepConfig
	PatternsFile string `json:"patternsFile,omitempty"` // Path to REMOVE.TXT
}

// AddCommentConfig extends StepConfig with a comment file.
type AddCommentConfig struct {
	StepConfig
	CommentFile string `json:"commentFile,omitempty"` // Path to ZCOMMENT.TXT
}

// IncludeFileConfig extends StepConfig with the file to include.
type IncludeFileConfig struct {
	StepConfig
	FilePath string `json:"filePath,omitempty"` // Path to BBS.AD or similar
}

// StepsConfig holds all pipeline step configurations.
type StepsConfig struct {
	TestIntegrity StepConfig        `json:"testIntegrity"`
	ExtractToTemp StepConfig        `json:"extractToTemp"`
	VirusScan     StepConfig        `json:"virusScan"`
	RemoveAds     RemoveAdsConfig   `json:"removeAds"`
	AddComment    AddCommentConfig  `json:"addComment"`
	IncludeFile   IncludeFileConfig `json:"includeFile"`
}

// ArchiveType defines how to handle a specific archive format.
type ArchiveType struct {
	Extension      string   `json:"extension"`                // e.g., ".zip", ".rar"
	Native         bool     `json:"native"`                   // true = handled by Go stdlib
	ExtractCommand string   `json:"extractCommand,omitempty"` // External extract command
	ExtractArgs    []string `json:"extractArgs,omitempty"`    // Extract arguments
	TestCommand    string   `json:"testCommand,omitempty"`    // Integrity test command
	TestArgs       []string `json:"testArgs,omitempty"`       // Test arguments
	AddCommand     string   `json:"addCommand,omitempty"`     // Add file command
	AddArgs        []string `json:"addArgs,omitempty"`        // Add arguments
	CommentCommand string   `json:"commentCommand,omitempty"` // Add comment command
	CommentArgs    []string `json:"commentArgs,omitempty"`    // Comment arguments
}

// Config holds the complete ZipLab configuration.
type Config struct {
	Enabled          bool          `json:"enabled"`
	RunOnUpload      bool          `json:"runOnUpload"`
	ScanFailBehavior string        `json:"scanFailBehavior"` // "delete" or "quarantine"
	QuarantinePath   string        `json:"quarantinePath,omitempty"`
	Steps            StepsConfig   `json:"steps"`
	ArchiveTypes     []ArchiveType `json:"archiveTypes"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:          true,
		RunOnUpload:      true,
		ScanFailBehavior: "delete",
		Steps: StepsConfig{
			TestIntegrity: StepConfig{Enabled: true},
			ExtractToTemp: StepConfig{Enabled: true},
			VirusScan:     StepConfig{Enabled: false, Command: "clamscan", Args: []string{"--stdout", "--no-summary", "{WORKDIR}"}, Timeout: 120},
			RemoveAds:     RemoveAdsConfig{StepConfig: StepConfig{Enabled: true}, PatternsFile: "REMOVE.TXT"},
			AddComment:    AddCommentConfig{StepConfig: StepConfig{Enabled: true}, CommentFile: "ZCOMMENT.TXT"},
			IncludeFile:   IncludeFileConfig{StepConfig: StepConfig{Enabled: true}, FilePath: "BBS.AD"},
		},
		ArchiveTypes: []ArchiveType{
			{Extension: ".zip", Native: true},
		},
	}
}

// LoadConfig loads ZipLab configuration from ziplab.json in the given config directory.
// Returns defaults if the file doesn't exist.
func LoadConfig(configPath string) (Config, error) {
	filePath := filepath.Join(configPath, "ziplab.json")
	log.Printf("INFO: Loading ZipLab config from %s", filePath)

	cfg := DefaultConfig()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: ziplab.json not found at %s, using defaults", filePath)
			return cfg, nil
		}
		return cfg, fmt.Errorf("failed to read ziplab config %s: %w", filePath, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse ziplab config %s: %w", filePath, err)
	}

	log.Printf("INFO: Loaded ZipLab config: enabled=%v, runOnUpload=%v, scanFailBehavior=%s",
		cfg.Enabled, cfg.RunOnUpload, cfg.ScanFailBehavior)
	return cfg, nil
}

// IsArchiveSupported checks if a filename matches a configured archive type.
func (c *Config) IsArchiveSupported(filename string) bool {
	_, ok := c.GetArchiveType(filename)
	return ok
}

// GetArchiveType returns the ArchiveType config for a given filename.
func (c *Config) GetArchiveType(filename string) (ArchiveType, bool) {
	if filename == "" {
		return ArchiveType{}, false
	}
	lowerName := strings.ToLower(filename)
	for _, at := range c.ArchiveTypes {
		if strings.HasSuffix(lowerName, strings.ToLower(at.Extension)) {
			return at, true
		}
	}
	return ArchiveType{}, false
}
