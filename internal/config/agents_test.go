package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSupportedAgentNames(t *testing.T) {
	names := GetSupportedAgentNames()

	if len(names) == 0 {
		t.Fatal("Expected non-empty list of agent names")
	}

	// Should contain known agents
	knownAgents := []string{"claude", "cursor", "codex", "gemini", "copilot"}
	for _, known := range knownAgents {
		found := false
		for _, name := range names {
			if name == known {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected agent '%s' in supported names list", known)
		}
	}

	// Should be sorted
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("Agent names not sorted: '%s' before '%s'", names[i-1], names[i])
		}
	}

	// Count should match SupportedAgents map
	if len(names) != len(SupportedAgents) {
		t.Errorf("Expected %d agent names, got %d", len(SupportedAgents), len(names))
	}
}

func TestIsValidAgent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"direct agent claude", "claude", true},
		{"direct agent cursor", "cursor", true},
		{"direct agent codex", "codex", true},
		{"direct agent gemini", "gemini", true},
		{"direct agent copilot", "copilot", true},
		{"direct agent windsurf", "windsurf", true},
		{"alias claude-code", "claude-code", true},
		{"alias openai-codex", "openai-codex", true},
		{"alias gemini-cli", "gemini-cli", true},
		{"alias github-copilot", "github-copilot", true},
		{"alias kilocode", "kilocode", true},
		{"alias openclaw-ai", "openclaw-ai", true},
		{"invalid agent", "nonexistent-agent", false},
		{"empty string", "", false},
		{"uppercase agent", "Claude", false},
		{"partial name", "clau", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidAgent(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidAgent(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveAgentType(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedType AgentType
		expectedOk   bool
	}{
		{"direct name claude", "claude", AgentClaude, true},
		{"direct name cursor", "cursor", AgentCursor, true},
		{"direct name codex", "codex", AgentCodex, true},
		{"direct name gemini", "gemini", AgentGemini, true},
		{"alias claude-code", "claude-code", AgentClaude, true},
		{"alias openai-codex", "openai-codex", AgentCodex, true},
		{"alias gemini-cli", "gemini-cli", AgentGemini, true},
		{"alias github-copilot", "github-copilot", AgentCopilot, true},
		{"alias gemini-antigravity", "gemini-antigravity", AgentAntigravity, true},
		{"alias kiro-cli", "kiro-cli", AgentKiro, true},
		{"alias openclaw-ai", "openclaw-ai", AgentOpenClaw, true},
		{"invalid name", "nonexistent", "", false},
		{"empty string", "", "", false},
		{"uppercase", "Claude", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agentType, ok := ResolveAgentType(tt.input)
			if ok != tt.expectedOk {
				t.Errorf("ResolveAgentType(%q) ok = %v, want %v", tt.input, ok, tt.expectedOk)
			}
			if agentType != tt.expectedType {
				t.Errorf("ResolveAgentType(%q) type = %q, want %q", tt.input, agentType, tt.expectedType)
			}
		})
	}
}

func TestGetAgentSkillsDir(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home dir: %v", err)
	}

	t.Run("project dir for claude", func(t *testing.T) {
		dir, err := GetAgentSkillsDir(AgentClaude, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if dir != ".claude/skills" {
			t.Errorf("Expected '.claude/skills', got %q", dir)
		}
	})

	t.Run("global dir for claude", func(t *testing.T) {
		dir, err := GetAgentSkillsDir(AgentClaude, true)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := filepath.Join(home, ".claude/skills")
		if dir != expected {
			t.Errorf("Expected %q, got %q", expected, dir)
		}
	})

	t.Run("project dir for cursor", func(t *testing.T) {
		dir, err := GetAgentSkillsDir(AgentCursor, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if dir != ".cursor/skills" {
			t.Errorf("Expected '.cursor/skills', got %q", dir)
		}
	})

	t.Run("global dir for cursor", func(t *testing.T) {
		dir, err := GetAgentSkillsDir(AgentCursor, true)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := filepath.Join(home, ".cursor/skills")
		if dir != expected {
			t.Errorf("Expected %q, got %q", expected, dir)
		}
	})

	t.Run("project dir for windsurf", func(t *testing.T) {
		dir, err := GetAgentSkillsDir(AgentWindsurf, false)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if dir != ".windsurf/skills" {
			t.Errorf("Expected '.windsurf/skills', got %q", dir)
		}
	})

	t.Run("global dir for windsurf", func(t *testing.T) {
		dir, err := GetAgentSkillsDir(AgentWindsurf, true)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		expected := filepath.Join(home, ".codeium/windsurf/skills")
		if dir != expected {
			t.Errorf("Expected %q, got %q", expected, dir)
		}
	})

	t.Run("unsupported agent", func(t *testing.T) {
		_, err := GetAgentSkillsDir(AgentType("nonexistent"), false)
		if err == nil {
			t.Error("Expected error for unsupported agent, got nil")
		}
	})

	t.Run("unsupported agent global", func(t *testing.T) {
		_, err := GetAgentSkillsDir(AgentType("nonexistent"), true)
		if err == nil {
			t.Error("Expected error for unsupported agent, got nil")
		}
	})
}

func TestGetAllAgentSkillsDirs(t *testing.T) {
	dirs := GetAllAgentSkillsDirs()

	if len(dirs) == 0 {
		t.Fatal("Expected non-empty list of skill directories")
	}

	// Should include the default ASK skills dir
	foundDefault := false
	for _, dir := range dirs {
		if dir == DefaultSkillsDir {
			foundDefault = true
			break
		}
	}
	if !foundDefault {
		t.Errorf("Expected DefaultSkillsDir (%q) in directories list", DefaultSkillsDir)
	}

	// Should include project-level directories for known agents
	foundClaudeProject := false
	for _, dir := range dirs {
		if dir == ".claude/skills" {
			foundClaudeProject = true
			break
		}
	}
	if !foundClaudeProject {
		t.Error("Expected '.claude/skills' project dir in directories list")
	}

	// Should include global directories (home-prefixed)
	home, err := os.UserHomeDir()
	if err == nil {
		foundClaudeGlobal := false
		expected := filepath.Join(home, ".claude/skills")
		for _, dir := range dirs {
			if dir == expected {
				foundClaudeGlobal = true
				break
			}
		}
		if !foundClaudeGlobal {
			t.Errorf("Expected global dir %q in directories list", expected)
		}
	}

	// Verify no duplicates
	seen := make(map[string]bool)
	for _, dir := range dirs {
		if seen[dir] {
			t.Errorf("Duplicate directory in result: %s", dir)
		}
		seen[dir] = true
	}

	// Should have at least 1 (default) + some project + some global dirs
	if len(dirs) < 3 {
		t.Errorf("Expected at least 3 directories, got %d", len(dirs))
	}
}
