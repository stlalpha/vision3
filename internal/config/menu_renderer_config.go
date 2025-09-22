package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MenuRendererConfig controls how programmatic menus are rendered.
type MenuRendererConfig struct {
	Enable            bool                            `json:"enable"`
	DefaultTheme      string                          `json:"defaultTheme"`
	Palette           string                          `json:"palette"`
	Codepage          string                          `json:"codepage"`
	AllowExternalAnsi bool                            `json:"allowExternalAnsi"`
	MenuOverrides     map[string]MenuRendererOverride `json:"menuOverrides"`
}

// MenuRendererOverride customizes rendering for a specific menu.
type MenuRendererOverride struct {
	Mode     string `json:"mode,omitempty"` // built_in, external
	Theme    string `json:"theme,omitempty"`
	Palette  string `json:"palette,omitempty"`
	Codepage string `json:"codepage,omitempty"`
}

// DefaultMenuRendererConfig returns built-in defaults when no config file is present.
func DefaultMenuRendererConfig() MenuRendererConfig {
	return MenuRendererConfig{
		Enable:            true,
		DefaultTheme:      "visionx",
		Palette:           "amiga",
		Codepage:          "amiga_topaz",
		AllowExternalAnsi: true,
		MenuOverrides: map[string]MenuRendererOverride{
			"LOGIN": {Mode: "external"},
		},
	}
}

// LoadMenuRendererConfig reads menu_renderer.json from the provided config directory.
// If the file does not exist a default configuration is returned.
func LoadMenuRendererConfig(configDir string) (MenuRendererConfig, error) {
	cfgPath := filepath.Join(configDir, "menu_renderer.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultMenuRendererConfig()
			cfg.Normalise()
			return cfg, nil
		}
		return MenuRendererConfig{}, fmt.Errorf("failed to read menu renderer config %s: %w", cfgPath, err)
	}

	var cfg MenuRendererConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return MenuRendererConfig{}, fmt.Errorf("failed to parse menu renderer config %s: %w", cfgPath, err)
	}

	cfg.Normalise()
	return cfg, nil
}

// Normalise applies defaults for missing fields and normalises case.
func (cfg *MenuRendererConfig) Normalise() {
	if cfg.MenuOverrides == nil {
		cfg.MenuOverrides = make(map[string]MenuRendererOverride)
	}

	if strings.TrimSpace(cfg.DefaultTheme) == "" {
		cfg.DefaultTheme = "visionx"
	}
	if strings.TrimSpace(cfg.Palette) == "" {
		cfg.Palette = "amiga"
	}
	if strings.TrimSpace(cfg.Codepage) == "" {
		cfg.Codepage = "amiga_topaz"
	}

	// Uppercase override keys for predictable lookups.
	normalised := make(map[string]MenuRendererOverride, len(cfg.MenuOverrides))
	for key, override := range cfg.MenuOverrides {
		normalised[strings.ToUpper(strings.TrimSpace(key))] = override
	}
	cfg.MenuOverrides = normalised
}

// SaveMenuRendererConfig persists the renderer configuration to disk.
func SaveMenuRendererConfig(configDir string, cfg MenuRendererConfig) error {
	cfgPath := filepath.Join(configDir, "menu_renderer.json")
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode menu renderer config: %w", err)
	}
	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write menu renderer config %s: %w", cfgPath, err)
	}
	return nil
}
