// Package app provides the Wails application logic.
package app

import (
	"context"
	"log"
	"os"

	"github.com/yeasy/ask/internal/config"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// Startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// 0. Ensure global config exists
	globalPath := config.GetGlobalConfigPath()
	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		log.Println("Global config not found, initializing default...")
		defaultCfg := config.DefaultConfig()
		if err := defaultCfg.SaveGlobal(); err != nil {
			log.Printf("Failed to initialize global config: %v", err)
		} else {
			log.Printf("Initialized global config at %s", globalPath)
		}
	}

	// 1. Load global config
	globalCfg, err := config.LoadGlobalConfig()
	if err == nil && globalCfg.LastProjectRoot != "" {
		// 2. Try to switch to LastProjectRoot
		if err := os.Chdir(globalCfg.LastProjectRoot); err != nil {
			log.Printf("Failed to switch to last project root: %v", err)
			// If failed, fall back to home
			fallbackToHome()
		} else {
			log.Printf("Restored project root: %s", globalCfg.LastProjectRoot)
		}
	} else {
		// 3. Default to user home if no last root or config load failed
		fallbackToHome()
	}
}

func fallbackToHome() {
	if home, err := os.UserHomeDir(); err == nil {
		if err := os.Chdir(home); err != nil {
			log.Printf("Failed to switch to home dir: %v", err)
		} else {
			log.Printf("Switched to home dir: %s", home)
		}
	}
}
