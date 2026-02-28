package menueditor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// MenuData represents the JSON stored in a .MNU file.
type MenuData struct {
	Title     string `json:"TITLE"`
	CLR       bool   `json:"CLR"`
	UsePrompt bool   `json:"USEPROMPT"`
	Prompt1   string `json:"PROMPT1"`
	Prompt2   string `json:"PROMPT2"`
	Fallback  string `json:"FALLBACK"`
	ACS       string `json:"ACS"`
	Password  string `json:"PASS"`
}

// CmdData represents a single entry in a .CFG JSON array.
type CmdData struct {
	Keys         string `json:"KEYS"`
	Command      string `json:"CMD"`
	ACS          string `json:"ACS"`
	Hidden       bool   `json:"HIDDEN"`
	AutoRun      string `json:"AUTORUN,omitempty"`
	NodeActivity string `json:"NODE_ACTIVITY,omitempty"`
}

// menuEntry holds a loaded menu with its on-disk name.
type menuEntry struct {
	Name string   // basename without extension, e.g. "MAIN"
	Data MenuData
}

// mnuDir returns the path to the .MNU files directory.
func mnuDir(menuBase string) string {
	return filepath.Join(menuBase, "mnu")
}

// cfgDir returns the path to the .CFG files directory.
func cfgDir(menuBase string) string {
	return filepath.Join(menuBase, "cfg")
}

// LoadMenus reads all .MNU files from {menuBase}/mnu/ and returns them
// sorted alphabetically by name.
func LoadMenus(menuBase string) ([]menuEntry, error) {
	dir := mnuDir(menuBase)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading menu dir %s: %w", dir, err)
	}

	var menus []menuEntry
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToUpper(name), ".MNU") {
			continue
		}
		stem := strings.TrimSuffix(strings.ToUpper(strings.TrimSuffix(name, filepath.Ext(name))), "")
		stem = strings.ToUpper(stem)

		data, err := loadMenuFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", name, err)
		}
		menus = append(menus, menuEntry{Name: stem, Data: data})
	}

	sort.Slice(menus, func(i, j int) bool {
		return menus[i].Name < menus[j].Name
	})

	return menus, nil
}

func loadMenuFile(path string) (MenuData, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return MenuData{}, err
	}
	var d MenuData
	if err := json.Unmarshal(raw, &d); err != nil {
		return MenuData{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	return d, nil
}

// LoadCommands reads the .CFG file for the given menu name from {menuBase}/cfg/.
// Returns an empty slice if no .CFG exists.
func LoadCommands(menuBase, name string) ([]CmdData, error) {
	path := filepath.Join(cfgDir(menuBase), name+".CFG")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []CmdData{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(raw) == 0 {
		return []CmdData{}, nil
	}
	var cmds []CmdData
	if err := json.Unmarshal(raw, &cmds); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return cmds, nil
}

// SaveMenu writes a MenuData record atomically to {menuBase}/mnu/{name}.MNU.
func SaveMenu(menuBase, name string, data MenuData) error {
	path := filepath.Join(mnuDir(menuBase), name+".MNU")
	return atomicWriteJSON(path, data)
}

// SaveCommands writes a command slice atomically to {menuBase}/cfg/{name}.CFG.
func SaveCommands(menuBase, name string, cmds []CmdData) error {
	path := filepath.Join(cfgDir(menuBase), name+".CFG")
	return atomicWriteJSON(path, cmds)
}

// DeleteMenu removes the .MNU and .CFG files for the given menu name.
func DeleteMenu(menuBase, name string) error {
	mnuPath := filepath.Join(mnuDir(menuBase), name+".MNU")
	cfgPath := filepath.Join(cfgDir(menuBase), name+".CFG")

	if err := os.Remove(mnuPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", mnuPath, err)
	}
	if err := os.Remove(cfgPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", cfgPath, err)
	}
	return nil
}

// CreateMenu writes a new empty .MNU and empty .CFG for the given name.
func CreateMenu(menuBase, name string) error {
	d := MenuData{
		CLR:       false,
		UsePrompt: true,
		Fallback:  name,
	}
	if err := SaveMenu(menuBase, name, d); err != nil {
		return err
	}
	return SaveCommands(menuBase, name, []CmdData{})
}

// MenuExists reports whether a .MNU file with the given name exists.
func MenuExists(menuBase, name string) bool {
	path := filepath.Join(mnuDir(menuBase), name+".MNU")
	_, err := os.Stat(path)
	return err == nil
}

// atomicWriteJSON marshals v to pretty-printed JSON and writes it to path
// via a temp file + rename for atomicity.
func atomicWriteJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "    ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".menueditor-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename to %s: %w", path, err)
	}
	return nil
}
