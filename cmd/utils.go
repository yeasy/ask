package cmd

import (
	"fmt"
	"os"

	"github.com/yeasy/ask/internal/config"
)

// ensureInitialized checks if ask.yaml exists. If not, auto-initializes.
// Returns true if initialization succeeded, false otherwise.
func ensureInitialized() bool {
	if _, err := os.Stat("ask.yaml"); err == nil {
		return true // Already initialized
	}

	// Auto-initialize without prompting
	fmt.Println("Project not initialized. Initializing...")
	return runInit()
}

// runInit executes the initialization logic. Returns true on success.
func runInit() bool {
	skillsDir := config.DefaultSkillsDir
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		fmt.Printf("Error creating skills directory: %v\n", err)
		return false
	}

	if err := config.CreateDefaultConfig(); err != nil {
		fmt.Printf("Error creating ask.yaml: %v\n", err)
		return false
	}

	fmt.Println("✓ Initialized ASK project")
	fmt.Printf("  Created: ask.yaml, %s/\n", skillsDir)
	return true
}
