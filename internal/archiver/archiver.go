// Package archiver provides a centralized registry of archive format
// definitions. It loads archivers.json from the configs directory and
// exposes a shared configuration that all subsystems (ZipLab upload
// pipeline, FTN bundle processing, file area management, etc.) use to
// determine which archive types are supported and how to pack/unpack them.
//
// This mirrors the classic BBS pattern where archiver definitions are
// configured once and referenced system-wide.
package archiver

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Archiver defines how to handle a specific archive format.
// Each archiver specifies detection (extension + optional magic bytes),
// and commands for pack/unpack/test/view/comment operations.
//
// When Native is true, Go's stdlib archive/zip is used for the core
// operations and the external command fields are ignored.
type Archiver struct {
	// ID is a short unique identifier, e.g. "zip", "rar", "arj", "7z".
	ID string `json:"id"`

	// Name is a human-readable description, e.g. "ZIP Archive".
	Name string `json:"name"`

	// Extension is the primary file extension including the dot, e.g. ".zip".
	Extension string `json:"extension"`

	// Extensions lists additional file extensions for this format.
	// The primary Extension is always included implicitly.
	Extensions []string `json:"extensions,omitempty"`

	// Magic is the hex-encoded magic bytes at offset 0 for format detection.
	// e.g. "504B0304" for ZIP (PK\x03\x04). Empty means extension-only detection.
	Magic string `json:"magic,omitempty"`

	// Native means this format is handled by Go's stdlib (currently only ZIP).
	// When true, external command fields are ignored for basic operations.
	Native bool `json:"native"`

	// Enabled controls whether this archiver is active. Disabled archivers
	// are skipped during detection and processing.
	Enabled bool `json:"enabled"`

	// Pack defines the command to create an archive.
	// Placeholders: {ARCHIVE} = output archive path, {FILES} = input files,
	// {WORKDIR} = working directory.
	Pack CommandDef `json:"pack,omitempty"`

	// Unpack defines the command to extract an archive.
	// Placeholders: {ARCHIVE} = archive path, {OUTDIR} = output directory.
	Unpack CommandDef `json:"unpack,omitempty"`

	// Test defines the command to verify archive integrity.
	// Placeholders: {ARCHIVE} = archive path.
	Test CommandDef `json:"test,omitempty"`

	// List defines the command to list archive contents.
	// Placeholders: {ARCHIVE} = archive path.
	List CommandDef `json:"list,omitempty"`

	// Comment defines the command to add a comment to an archive.
	// Placeholders: {ARCHIVE} = archive path, {FILE} = comment file path.
	Comment CommandDef `json:"comment,omitempty"`

	// AddFile defines the command to add a file to an existing archive.
	// Placeholders: {ARCHIVE} = archive path, {FILE} = file to add.
	AddFile CommandDef `json:"addFile,omitempty"`
}

// CommandDef specifies an external command with arguments.
type CommandDef struct {
	Command string   `json:"command,omitempty"` // Binary path or name
	Args    []string `json:"args,omitempty"`    // Arguments with placeholders
}

// IsEmpty reports whether this command definition has no command set.
func (cd CommandDef) IsEmpty() bool {
	return cd.Command == ""
}

// Config holds the complete archivers configuration.
type Config struct {
	Archivers []Archiver `json:"archivers"`
}

// allExtensions returns the primary extension plus any additional extensions.
func (a *Archiver) allExtensions() []string {
	exts := []string{strings.ToLower(a.Extension)}
	for _, e := range a.Extensions {
		lower := strings.ToLower(e)
		if lower != exts[0] {
			exts = append(exts, lower)
		}
	}
	return exts
}

// MatchesExtension reports whether the given filename has an extension
// that matches this archiver.
func (a *Archiver) MatchesExtension(filename string) bool {
	lower := strings.ToLower(filename)
	for _, ext := range a.allExtensions() {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// DefaultConfig returns a Config with built-in archiver definitions.
// ZIP is native (Go stdlib); others default to common external tools.
func DefaultConfig() Config {
	return Config{
		Archivers: []Archiver{
			{
				ID:        "zip",
				Name:      "ZIP Archive",
				Extension: ".zip",
				Magic:     "504B0304",
				Native:    true,
				Enabled:   true,
				Pack: CommandDef{
					Command: "zip",
					Args:    []string{"-j", "{ARCHIVE}", "{FILES}"},
				},
				Unpack: CommandDef{
					Command: "unzip",
					Args:    []string{"-o", "{ARCHIVE}", "-d", "{OUTDIR}"},
				},
				Test: CommandDef{
					Command: "unzip",
					Args:    []string{"-t", "{ARCHIVE}"},
				},
				List: CommandDef{
					Command: "unzip",
					Args:    []string{"-l", "{ARCHIVE}"},
				},
				Comment: CommandDef{
					Command: "zip",
					Args:    []string{"-z", "{ARCHIVE}"},
				},
				AddFile: CommandDef{
					Command: "zip",
					Args:    []string{"-j", "{ARCHIVE}", "{FILE}"},
				},
			},
			{
				ID:        "7z",
				Name:      "7-Zip Archive",
				Extension: ".7z",
				Magic:     "377ABCAF271C",
				Native:    false,
				Enabled:   false,
				Pack: CommandDef{
					Command: "7z",
					Args:    []string{"a", "{ARCHIVE}", "{FILES}"},
				},
				Unpack: CommandDef{
					Command: "7z",
					Args:    []string{"x", "-o{OUTDIR}", "{ARCHIVE}"},
				},
				Test: CommandDef{
					Command: "7z",
					Args:    []string{"t", "{ARCHIVE}"},
				},
				List: CommandDef{
					Command: "7z",
					Args:    []string{"l", "{ARCHIVE}"},
				},
			},
			{
				ID:        "rar",
				Name:      "RAR Archive",
				Extension: ".rar",
				Magic:     "526172211A07",
				Native:    false,
				Enabled:   false,
				Unpack: CommandDef{
					Command: "unrar",
					Args:    []string{"x", "-o+", "{ARCHIVE}", "{OUTDIR}"},
				},
				Test: CommandDef{
					Command: "unrar",
					Args:    []string{"t", "{ARCHIVE}"},
				},
				List: CommandDef{
					Command: "unrar",
					Args:    []string{"l", "{ARCHIVE}"},
				},
			},
			{
				ID:        "arj",
				Name:      "ARJ Archive",
				Extension: ".arj",
				Magic:     "60EA",
				Native:    false,
				Enabled:   false,
				Pack: CommandDef{
					Command: "arj",
					Args:    []string{"a", "{ARCHIVE}", "{FILES}"},
				},
				Unpack: CommandDef{
					Command: "arj",
					Args:    []string{"x", "-y", "{ARCHIVE}", "{OUTDIR}"},
				},
				Test: CommandDef{
					Command: "arj",
					Args:    []string{"t", "{ARCHIVE}"},
				},
				List: CommandDef{
					Command: "arj",
					Args:    []string{"l", "{ARCHIVE}"},
				},
			},
			{
				ID:         "lha",
				Name:       "LHA/LZH Archive",
				Extension:  ".lha",
				Extensions: []string{".lzh"},
				Native:     false,
				Enabled:    false,
				Pack: CommandDef{
					Command: "lha",
					Args:    []string{"a", "{ARCHIVE}", "{FILES}"},
				},
				Unpack: CommandDef{
					Command: "lha",
					Args:    []string{"x", "{ARCHIVE}", "{OUTDIR}"},
				},
				Test: CommandDef{
					Command: "lha",
					Args:    []string{"t", "{ARCHIVE}"},
				},
				List: CommandDef{
					Command: "lha",
					Args:    []string{"l", "{ARCHIVE}"},
				},
			},
		},
	}
}

// LoadConfig loads archiver definitions from archivers.json in the given
// config directory. Returns defaults if the file doesn't exist.
func LoadConfig(configPath string) (Config, error) {
	filePath := filepath.Join(configPath, "archivers.json")
	log.Printf("INFO: Loading archivers config from %s", filePath)

	cfg := DefaultConfig()

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("INFO: archivers.json not found at %s, using defaults", filePath)
			return cfg, nil
		}
		return cfg, fmt.Errorf("failed to read archivers config %s: %w", filePath, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse archivers config %s: %w", filePath, err)
	}

	log.Printf("INFO: Loaded %d archiver definitions from %s", len(cfg.Archivers), filePath)
	return cfg, nil
}

// SaveDefaultConfig writes the default archivers.json to the given config
// directory. Useful for initial setup or generating a template.
func SaveDefaultConfig(configPath string) error {
	cfg := DefaultConfig()
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return fmt.Errorf("failed to marshal default archivers config: %w", err)
	}

	filePath := filepath.Join(configPath, "archivers.json")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write archivers config %s: %w", filePath, err)
	}
	return nil
}

// EnabledArchivers returns only the archivers that are enabled.
func (c *Config) EnabledArchivers() []Archiver {
	var result []Archiver
	for _, a := range c.Archivers {
		if a.Enabled {
			result = append(result, a)
		}
	}
	return result
}

// SupportedExtensions returns all file extensions handled by enabled archivers.
func (c *Config) SupportedExtensions() []string {
	var exts []string
	for _, a := range c.EnabledArchivers() {
		exts = append(exts, a.allExtensions()...)
	}
	return exts
}

// FindByExtension returns the first enabled archiver that matches the
// given filename's extension.
func (c *Config) FindByExtension(filename string) (Archiver, bool) {
	for _, a := range c.EnabledArchivers() {
		if a.MatchesExtension(filename) {
			return a, true
		}
	}
	return Archiver{}, false
}

// FindByID returns the archiver with the given ID (regardless of enabled state).
func (c *Config) FindByID(id string) (Archiver, bool) {
	lowerID := strings.ToLower(id)
	for _, a := range c.Archivers {
		if strings.ToLower(a.ID) == lowerID {
			return a, true
		}
	}
	return Archiver{}, false
}

// IsSupported reports whether any enabled archiver handles the given filename.
func (c *Config) IsSupported(filename string) bool {
	_, ok := c.FindByExtension(filename)
	return ok
}
