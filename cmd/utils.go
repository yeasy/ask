package cmd

import (
	"fmt"
	"os"

	"github.com/yeasy/ask/internal/config"
)

// truncateStr truncates s to maxLen characters, appending "..." if truncated.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen < 4 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

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
		fmt.Fprintf(os.Stderr, "Error creating skills directory: %v\n", err)
		return false
	}

	if err := config.CreateDefaultConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating ask.yaml: %v\n", err)
		return false
	}

	fmt.Println("✓ Initialized ASK project")
	fmt.Printf("  Created: ask.yaml, %s/\n", skillsDir)
	return true
}
