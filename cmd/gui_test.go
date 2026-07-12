package cmd

import (
	"strings"
	"testing"
)

func TestGuiCommand(t *testing.T) {
	if guiCmd == nil {
		t.Fatal("guiCmd is nil")
	}

	if guiCmd.Use != "gui" {
		t.Errorf("guiCmd.Use = %s; want gui", guiCmd.Use)
	}

	// Verify it's added to root
	found := false
	for _, c := range rootCmd.Commands() {
		if c == guiCmd {
			found = true
			break
		}
	}
	if !found {
		t.Error("guiCmd not added to rootCmd")
	}
}

// TestGuiCommandSuggestsServe ensures the gui command's help discloses that the
// desktop UI is a desktop-build-only feature and points users at the web UI
// (`ask serve`) that works in the standard CLI build.
func TestGuiCommandSuggestsServe(t *testing.T) {
	help := guiCmd.Long + " " + guiCmd.Short
	if !strings.Contains(help, "ask serve") {
		t.Errorf("gui help should point users to `ask serve`; got Short=%q Long=%q", guiCmd.Short, guiCmd.Long)
	}
}
