package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stlalpha/vision3/internal/config"
	"github.com/stlalpha/vision3/internal/menu"
)

// ConfigWatcher watches configuration files for changes and hot-reloads them.
type ConfigWatcher struct {
	mu             sync.RWMutex
	watcher        *fsnotify.Watcher
	watcherDone    chan bool
	rootConfigPath string
	menuSetPath    string
	menuExecutor   *menu.MenuExecutor
	serverConfig   *config.ServerConfig
	serverConfigMu *sync.RWMutex // External mutex for server config
}

// NewConfigWatcher creates a new configuration file watcher.
func NewConfigWatcher(rootConfigPath, menuSetPath string, menuExecutor *menu.MenuExecutor, serverConfig *config.ServerConfig, serverConfigMu *sync.RWMutex) (*ConfigWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	cw := &ConfigWatcher{
		watcher:        watcher,
		watcherDone:    make(chan bool),
		rootConfigPath: rootConfigPath,
		menuSetPath:    menuSetPath,
		menuExecutor:   menuExecutor,
		serverConfig:   serverConfig,
		serverConfigMu: serverConfigMu,
	}

	// Watch the configs directory
	if err := watcher.Add(rootConfigPath); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch %s: %w", rootConfigPath, err)
	}
	log.Printf("INFO: Watching %s for config changes (auto-reload enabled)", rootConfigPath)

	// Watch the menu set path for theme.json
	themePath := filepath.Join(menuSetPath, "theme.json")
	if _, err := os.Stat(themePath); err == nil {
		if err := watcher.Add(themePath); err != nil {
			log.Printf("WARN: Failed to watch %s: %v", themePath, err)
		} else {
			log.Printf("INFO: Watching %s for theme changes (auto-reload enabled)", themePath)
		}
	}

	// Start watching in a goroutine
	go cw.watchLoop(watcher)

	return cw, nil
}

// Stop stops the configuration file watcher.
func (cw *ConfigWatcher) Stop() {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if cw.watcher == nil {
		return
	}

	select {
	case <-cw.watcherDone:
		// already closed
	default:
		close(cw.watcherDone)
	}

	cw.watcher.Close()
	cw.watcher = nil
	log.Printf("INFO: Configuration file watcher stopped")
}

// watchLoop handles file system events for configuration files.
func (cw *ConfigWatcher) watchLoop(w *fsnotify.Watcher) {
	// Debounce timer to avoid reloading on rapid successive writes
	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}

			// Only care about Write and Create events
			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				// Cancel existing debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				// Schedule reload after debounce period
				debounceTimer = time.AfterFunc(debounceDuration, func() {
					cw.handleConfigChange(event.Name)
				})
			}

		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			log.Printf("ERROR: Config file watcher error: %v", err)

		case <-cw.watcherDone:
			log.Printf("INFO: Stopping config file watcher")
			return
		}
	}
}

// handleConfigChange identifies which config file changed and reloads it.
func (cw *ConfigWatcher) handleConfigChange(path string) {
	filename := filepath.Base(path)
	log.Printf("INFO: Config file change detected: %s", filename)

	switch strings.ToLower(filename) {
	case "doors.json":
		cw.reloadDoors()
	case "login.json":
		cw.reloadLoginSequence()
	case "strings.json":
		cw.reloadStrings()
	case "theme.json":
		cw.reloadTheme()
	case "config.json":
		cw.reloadServerConfig()
	case "events.json":
		// Events config reload would require restarting the scheduler
		// For now, just log that a restart is needed
		log.Printf("WARN: events.json changed - server restart required for changes to take effect")
	case "ftn.json":
		// FTN config reload would require restarting the message manager
		log.Printf("WARN: ftn.json changed - server restart required for changes to take effect")
	default:
		// Ignore other files
		log.Printf("DEBUG: Ignoring change to %s", filename)
	}
}

// reloadDoors reloads the door configurations.
func (cw *ConfigWatcher) reloadDoors() {
	log.Printf("INFO: Reloading doors.json...")

	doorsPath := filepath.Join(cw.rootConfigPath, "doors.json")
	newDoors, err := config.LoadDoors(doorsPath)
	if err != nil {
		log.Printf("ERROR: Failed to reload doors.json: %v", err)
		return
	}

	// Update MenuExecutor's DoorRegistry atomically
	cw.menuExecutor.SetDoorRegistry(newDoors)
	log.Printf("INFO: doors.json reloaded successfully (%d doors configured)", len(newDoors))
}

// reloadLoginSequence reloads the login sequence configuration.
func (cw *ConfigWatcher) reloadLoginSequence() {
	log.Printf("INFO: Reloading login.json...")

	newSequence, err := config.LoadLoginSequence(cw.rootConfigPath)
	if err != nil {
		log.Printf("ERROR: Failed to reload login.json: %v", err)
		return
	}

	// Update MenuExecutor's LoginSequence atomically
	cw.menuExecutor.SetLoginSequence(newSequence)
	log.Printf("INFO: login.json reloaded successfully (%d steps)", len(newSequence))
}

// reloadStrings reloads the strings configuration.
func (cw *ConfigWatcher) reloadStrings() {
	log.Printf("INFO: Reloading strings.json...")

	newStrings, err := config.LoadStrings(cw.rootConfigPath)
	if err != nil {
		log.Printf("ERROR: Failed to reload strings.json: %v", err)
		return
	}

	// Update MenuExecutor's LoadedStrings atomically
	cw.menuExecutor.SetStrings(newStrings)
	log.Printf("INFO: strings.json reloaded successfully")
}

// reloadTheme reloads the theme configuration.
func (cw *ConfigWatcher) reloadTheme() {
	log.Printf("INFO: Reloading theme.json...")

	newTheme, err := config.LoadThemeConfig(cw.menuSetPath)
	if err != nil {
		log.Printf("ERROR: Failed to reload theme.json: %v", err)
		return
	}

	// Update MenuExecutor's Theme atomically
	cw.menuExecutor.SetTheme(newTheme)
	log.Printf("INFO: theme.json reloaded successfully")
}

// reloadServerConfig reloads the server configuration.
func (cw *ConfigWatcher) reloadServerConfig() {
	log.Printf("INFO: Reloading config.json...")

	newServerConfig, err := config.LoadServerConfig(cw.rootConfigPath)
	if err != nil {
		log.Printf("ERROR: Failed to reload config.json: %v", err)
		return
	}

	// Update server config atomically
	if cw.serverConfigMu != nil {
		cw.serverConfigMu.Lock()
		*cw.serverConfig = newServerConfig
		cw.serverConfigMu.Unlock()
	} else {
		// Fallback if no mutex provided (not thread-safe)
		*cw.serverConfig = newServerConfig
	}

	// Also update MenuExecutor's ServerCfg
	cw.menuExecutor.SetServerConfig(newServerConfig)

	log.Printf("INFO: config.json reloaded successfully")
	log.Printf("WARN: Some config.json changes (ports, keys, IP limits) require a full restart")
}
