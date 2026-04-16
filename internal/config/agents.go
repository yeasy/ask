// Package config handles configuration loading and management.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// AgentType represents a supported AI coding agent
type AgentType string

const (
	// AgentClaude represents the Claude agent
	AgentClaude AgentType = "claude"
	// AgentCursor represents the Cursor agent
	AgentCursor AgentType = "cursor"
	// AgentCodex represents the OpenAI Codex agent
	AgentCodex AgentType = "codex"
	// AgentOpenCode represents the OpenCode agent
	AgentOpenCode AgentType = "opencode"
	// AgentAntigravity represents the Antigravity agent
	AgentAntigravity AgentType = "antigravity"
	// AgentGemini represents the Gemini CLI agent
	AgentGemini AgentType = "gemini"
	// AgentCopilot represents the GitHub Copilot agent
	AgentCopilot AgentType = "copilot"
	// AgentWindsurf represents the Windsurf agent
	AgentWindsurf AgentType = "windsurf"
	// AgentAmp represents the Amp agent
	AgentAmp AgentType = "amp"
	// AgentGoose represents the Goose agent
	AgentGoose AgentType = "goose"
	// AgentKilo represents the Kilo agent
	AgentKilo AgentType = "kilo"
	// AgentKiro represents the Kiro agent
	AgentKiro AgentType = "kiro"
	// AgentRoo represents the Roo agent
	AgentRoo AgentType = "roo"
	// AgentTrae represents the Trae agent
	AgentTrae AgentType = "trae"
	// AgentDroid represents the Droid agent
	AgentDroid AgentType = "droid"
	// AgentClawdBot represents the ClawdBot agent
	AgentClawdBot AgentType = "clawdbot"
	// AgentNeovate represents the Neovate agent
	AgentNeovate AgentType = "neovate"
	// AgentCodeBuddy represents the CodeBuddy agent
	AgentCodeBuddy AgentType = "codebuddy"
	// AgentOpenClaw represents the OpenClaw agent
	AgentOpenClaw AgentType = "openclaw"
)

// AgentConfig holds directory paths for an agent
type AgentConfig struct {
	Name       string   // Display name
	ProjectDir string   // Project-level skills directory (e.g., ".claude/skills")
	GlobalDir  string   // User-level skills directory (e.g., "~/.claude/skills")
	Aliases    []string // Alternative names for this agent
}

// SupportedAgents maps agent types to their configurations
var SupportedAgents = map[AgentType]AgentConfig{
	AgentClaude: {
		Name:       "Claude",
		ProjectDir: ".claude/skills",
		GlobalDir:  ".claude/skills", // Will be prefixed with home dir
		Aliases:    []string{"claude-code"},
	},
	AgentCursor: {
		Name:       "Cursor",
		ProjectDir: ".cursor/skills",
		GlobalDir:  ".cursor/skills",
		Aliases:    []string{},
	},
	AgentCodex: {
		Name:       "Codex",
		ProjectDir: ".codex/skills",
		GlobalDir:  ".codex/skills",
		Aliases:    []string{"openai-codex"},
	},
	AgentOpenCode: {
		Name:       "OpenCode",
		ProjectDir: ".opencode/skills",
		GlobalDir:  ".config/opencode/skills",
		Aliases:    []string{},
	},
	AgentAntigravity: {
		Name:       "Antigravity",
		ProjectDir: ".agent/skills",
		GlobalDir:  ".gemini/antigravity/skills",
		Aliases:    []string{"gemini-antigravity"},
	},
	AgentGemini: {
		Name:       "Gemini CLI",
		ProjectDir: ".gemini/skills",
		GlobalDir:  ".gemini/skills",
		Aliases:    []string{"gemini-cli"},
	},
	AgentCopilot: {
		Name:       "GitHub Copilot",
		ProjectDir: ".github/skills",
		GlobalDir:  ".copilot/skills",
		Aliases:    []string{"github-copilot"},
	},
	AgentWindsurf: {
		Name:       "Windsurf",
		ProjectDir: ".windsurf/skills",
		GlobalDir:  ".codeium/windsurf/skills",
		Aliases:    []string{},
	},
	AgentAmp: {
		Name:       "Amp",
		ProjectDir: ".agents/skills",
		GlobalDir:  ".config/agents/skills",
		Aliases:    []string{},
	},
	AgentGoose: {
		Name:       "Goose",
		ProjectDir: ".goose/skills",
		GlobalDir:  ".config/goose/skills",
		Aliases:    []string{},
	},
	AgentKilo: {
		Name:       "Kilo",
		ProjectDir: ".kilocode/skills",
		GlobalDir:  ".kilocode/skills",
		Aliases:    []string{"kilocode"},
	},
	AgentKiro: {
		Name:       "Kiro",
		ProjectDir: ".kiro/skills",
		GlobalDir:  ".kiro/skills",
		Aliases:    []string{"kiro-cli"},
	},
	AgentRoo: {
		Name:       "Roo",
		ProjectDir: ".roo/skills",
		GlobalDir:  ".roo/skills",
		Aliases:    []string{},
	},
	AgentTrae: {
		Name:       "Trae",
		ProjectDir: ".trae/skills",
		GlobalDir:  ".trae/skills",
		Aliases:    []string{},
	},
	AgentDroid: {
		Name:       "Droid",
		ProjectDir: ".factory/skills",
		GlobalDir:  ".factory/skills",
		Aliases:    []string{},
	},
	AgentClawdBot: {
		Name:       "ClawdBot",
		ProjectDir: "skills",
		GlobalDir:  ".clawdbot/skills",
		Aliases:    []string{},
	},
	AgentNeovate: {
		Name:       "Neovate",
		ProjectDir: ".neovate/skills",
		GlobalDir:  ".neovate/skills",
		Aliases:    []string{},
	},
	AgentCodeBuddy: {
		Name:       "CodeBuddy",
		ProjectDir: ".codebuddy/skills",
		GlobalDir:  ".codebuddy/skills",
		Aliases:    []string{},
	},
	AgentOpenClaw: {
		Name:       "OpenClaw",
		ProjectDir: ".openclaw/skills",
		GlobalDir:  ".openclaw/workspace/skills",
		Aliases:    []string{"openclaw-ai"},
	},
}

// GetSupportedAgentNames returns a list of all supported agent type names
func GetSupportedAgentNames() []string {
	names := make([]string, 0, len(SupportedAgents))
	for agent := range SupportedAgents {
		names = append(names, string(agent))
	}
	sort.Strings(names)
	return names
}

// IsValidAgent checks if the given agent name is supported
func IsValidAgent(name string) bool {
	// Check direct match
	if _, ok := SupportedAgents[AgentType(name)]; ok {
		return true
	}
	// Check aliases
	for _, config := range SupportedAgents {
		for _, alias := range config.Aliases {
			if alias == name {
				return true
			}
		}
	}
	return false
}

// ResolveAgentType resolves an agent name (including aliases) to its AgentType
func ResolveAgentType(name string) (AgentType, bool) {
	// Check direct match
	if _, ok := SupportedAgents[AgentType(name)]; ok {
		return AgentType(name), true
	}
	// Check aliases
	for agentType, config := range SupportedAgents {
		for _, alias := range config.Aliases {
			if alias == name {
				return agentType, true
			}
		}
	}
	return "", false
}

// GetAgentSkillsDir returns the skills directory for a specific agent
// If global is true, returns the user-level directory (e.g., ~/.claude/skills)
// Otherwise returns the project-level directory (e.g., .claude/skills)
func GetAgentSkillsDir(agent AgentType, global bool) (string, error) {
	config, ok := SupportedAgents[agent]
	if !ok {
		return "", fmt.Errorf("unsupported agent type: %s", string(agent))
	}

	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, config.GlobalDir), nil
	}

	return config.ProjectDir, nil
}

// GetAllAgentSkillsDirs returns all unique skill directories for discovery.
// Returns both project-level and global directories for all supported agents.
func GetAllAgentSkillsDirs() []string {
	seen := make(map[string]bool)
	var dirs []string
	add := func(d string) {
		if !seen[d] {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}

	// Add default ASK directory
	add(DefaultSkillsDir)

	// Add project-level directories
	for _, config := range SupportedAgents {
		add(config.ProjectDir)
	}

	// Add global directories
	home, err := os.UserHomeDir()
	if err == nil {
		for _, config := range SupportedAgents {
			add(filepath.Join(home, config.GlobalDir))
		}
	}

	return dirs
}
