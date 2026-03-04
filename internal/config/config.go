package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// OfflineMode indicates if the application is running in offline mode
var OfflineMode bool

// SetOffline sets the offline mode
func SetOffline(offline bool) {
	OfflineMode = offline
}

// Repo represents a skill repository
type Repo struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"` // "topic" or "dir"
	URL  string `yaml:"url"`  // GitHub topic or "owner/repo/path"
}

// ToolTarget represents a supported AI coding tool
type ToolTarget struct {
	Name      string `yaml:"name" json:"name"`
	SkillsDir string `yaml:"skills_dir" json:"skills_dir"`
	Enabled   bool   `yaml:"enabled" json:"enabled"`
}

// DefaultToolTargets returns the supported AI coding tools
func DefaultToolTargets() []ToolTarget {
	targets := []ToolTarget{}
	// Add default Agent skills
	targets = append(targets, ToolTarget{
		Name:      "agent",
		SkillsDir: DefaultSkillsDir,
		Enabled:   true,
	})

	// Add DETECTED agents only, not all possible ones.
	// This prevents showing clutter like "clawdbot" when not applicable.
	// We use the current working directory to detect.

	if !OfflineMode {
		if cwd, err := os.Getwd(); err == nil {

			// DetectExistingToolDirs returns ToolTarget structs created from DefaultToolTargets logic which was cyclical.
			// We need a helper that creates targets from detected dirs WITHOUT calling DefaultToolTargets.
			// Let's implement detection logic directly here or fix DetectExistingToolDirs.

			// Implementation of direct detection to avoid cycle:
			for _, name := range GetSupportedAgentNames() {
				if agentType, ok := ResolveAgentType(name); ok {
					config := SupportedAgents[agentType]
					// Check if project dir exists
					// config.ProjectDir is like ".claude/skills"
					// We check if ".claude" exists
					agentRootDir := filepath.Dir(config.ProjectDir)
					if agentRootDir == "." {
						agentRootDir = config.ProjectDir
					}
					if _, err := os.Stat(filepath.Join(cwd, agentRootDir)); err == nil {
						// Found!
						targets = append(targets, ToolTarget{
							Name:      name,
							SkillsDir: config.ProjectDir,
							Enabled:   true,
						})
					}
				}
			}
		}
	}

	return targets
}

// SkillInfo represents an installed skill with metadata
type SkillInfo struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	URL         string `yaml:"url,omitempty"`
}

// Config represents the structure of ask.yaml
type Config struct {
	Version         string       `yaml:"version"`
	SkillsDir       string       `yaml:"skills_dir,omitempty"`   // Skills installation directory (default: .agent/skills)
	ToolTargets     []ToolTarget `yaml:"tool_targets,omitempty"` // Target AI tools for skill installation
	Skills          []string     `yaml:"skills,omitempty"`       // Legacy: simple list of skill names
	SkillsInfo      []SkillInfo  `yaml:"skills_info,omitempty"`  // New: skills with metadata
	Repos           []Repo       `yaml:"repos,omitempty"`
	LastProjectRoot string       `yaml:"last_project_root,omitempty"` // Last used project root (global only)
}

// DefaultSkillsDir is the default directory to install skills
const DefaultSkillsDir = ".agent/skills"

// GlobalConfigDirName is the name of the global config directory
// Global installation paths
const GlobalConfigDirName = ".ask"

// GlobalConfigFileName is the name of the global config file
const GlobalConfigFileName = "config.yaml"

// GlobalSkillsDirName is the name of the global skills directory
const GlobalSkillsDirName = "skills"

// GlobalLockFileName is the name of the global lock file
const GlobalLockFileName = "ask.lock"

// GetGlobalConfigDir returns the global config directory path (~/.ask)
func GetGlobalConfigDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, GlobalConfigDirName)
}

// GetGlobalConfigPath returns the global config file path (~/.ask/config.yaml)
func GetGlobalConfigPath() string {
	return filepath.Join(GetGlobalConfigDir(), GlobalConfigFileName)
}

// GetGlobalSkillsDir returns the global skills directory path (~/.ask/skills)
func GetGlobalSkillsDir() string {
	return filepath.Join(GetGlobalConfigDir(), GlobalSkillsDirName)
}

// GetGlobalLockPath returns the global lock file path (~/.ask/ask.lock)
func GetGlobalLockPath() string {
	return filepath.Join(GetGlobalConfigDir(), GlobalLockFileName)
}

// EnsureGlobalDirExists creates the global config directory if it doesn't exist
func EnsureGlobalDirExists() error {
	globalDir := GetGlobalConfigDir()
	if globalDir == "" {
		return fmt.Errorf("could not determine home directory")
	}
	return os.MkdirAll(globalDir, 0755)
}

// GetSkillsDirByScope returns the skills directory based on global flag
func GetSkillsDirByScope(global bool) string {
	if global {
		return GetGlobalSkillsDir()
	}
	return DefaultSkillsDir
}

// GetSkillsDir returns the skills directory, using default if not set
func (c *Config) GetSkillsDir() string {
	if c.SkillsDir == "" {
		return DefaultSkillsDir
	}
	return c.SkillsDir
}

// OptionalRepos returns a list of optional repositories that are not enabled by default
var OptionalRepos = []Repo{
	{
		Name: "community",
		Type: "topic",
		URL:  "agent-skill OR topic:agent-skills",
	},
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Version: "1.2",
		Skills:  []string{},
		Repos: []Repo{
			{
				Name: "anthropics",
				Type: "dir",
				URL:  "anthropics/skills/skills",
			},
			{
				Name: "openai",
				Type: "dir",
				URL:  "openai/skills/skills",
			},
			{
				Name: "composio",
				Type: "dir",
				URL:  "ComposioHQ/awesome-claude-skills",
			},
			{
				Name: "vercel",
				Type: "dir",
				URL:  "vercel-labs/agent-skills",
			},
		},
	}
}

// loadConfigFromPath loads and merges a config from the given file path
func loadConfigFromPath(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	mergeDefaults(&cfg)
	return &cfg, nil
}

// mergeDefaults merges default repos and tool targets into the config
func mergeDefaults(cfg *Config) {
	// Merge default repos with existing (add missing defaults)
	defaultRepos := DefaultConfig().Repos
	existingNames := make(map[string]bool)
	for _, r := range cfg.Repos {
		existingNames[r.Name] = true
	}
	for _, dr := range defaultRepos {
		if !existingNames[dr.Name] {
			cfg.Repos = append(cfg.Repos, dr)
		}
	}

	// Merge default tool targets with existing
	defaultTargets := DefaultToolTargets()
	existingTargets := make(map[string]bool)
	for _, t := range cfg.ToolTargets {
		existingTargets[t.Name] = true
	}
	for _, dt := range defaultTargets {
		if !existingTargets[dt.Name] {
			cfg.ToolTargets = append(cfg.ToolTargets, dt)
		}
	}
}

// LoadConfig loads the current ask.yaml configuration
func LoadConfig() (*Config, error) {
	return loadConfigFromPath("ask.yaml")
}

// Save saves the configuration to ask.yaml
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile("ask.yaml", data, 0644)
}

// RemoveSkill removes a skill from the configuration
func (c *Config) RemoveSkill(skillName string) {
	for i, s := range c.Skills {
		if s == skillName {
			c.Skills = append(c.Skills[:i], c.Skills[i+1:]...)
			return
		}
	}
}

// RemoveSkillInfo removes skill metadata from the configuration
func (c *Config) RemoveSkillInfo(skillName string) {
	for i, s := range c.SkillsInfo {
		if s.Name == skillName {
			c.SkillsInfo = append(c.SkillsInfo[:i], c.SkillsInfo[i+1:]...)
			return
		}
	}
}

// AddSkill adds a skill to the configuration if it doesn't exist
func (c *Config) AddSkill(skillName string) {
	for _, s := range c.Skills {
		if s == skillName {
			return
		}
	}
	c.Skills = append(c.Skills, skillName)
}

// AddSkillInfo adds a skill with metadata to the configuration
func (c *Config) AddSkillInfo(info SkillInfo) {
	// Check if skill already exists
	for i, s := range c.SkillsInfo {
		if s.Name == info.Name {
			// Update existing
			c.SkillsInfo[i] = info
			return
		}
	}
	c.SkillsInfo = append(c.SkillsInfo, info)

	// Also add to legacy Skills list for backward compatibility
	c.AddSkill(info.Name)
}

// GetSkillInfo returns skill info by name
func (c *Config) GetSkillInfo(name string) *SkillInfo {
	for _, s := range c.SkillsInfo {
		if s.Name == name {
			return &s
		}
	}
	return nil
}

// CreateDefaultConfig creates a new ask.yaml in the current directory
func CreateDefaultConfig() error {
	config := DefaultConfig()
	return config.Save()
}

// LoadGlobalConfig loads the global config file (~/.ask/config.yaml)
func LoadGlobalConfig() (*Config, error) {
	cfg, err := loadConfigFromPath(GetGlobalConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			// Return default config if doesn't exist
			def := DefaultConfig()
			return &def, nil
		}
		return nil, err
	}
	return cfg, nil
}

// SaveGlobal saves the configuration to the global config file (~/.ask/config.yaml)
func (c *Config) SaveGlobal() error {
	if err := EnsureGlobalDirExists(); err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(GetGlobalConfigPath(), data, 0644)
}

// LoadConfigByScope loads config based on global flag
func LoadConfigByScope(global bool) (*Config, error) {
	if global {
		return LoadGlobalConfig()
	}
	return LoadConfig()
}

// SaveByScope saves config based on global flag
func (c *Config) SaveByScope(global bool) error {
	if global {
		return c.SaveGlobal()
	}
	return c.Save()
}

// GetToolTargets returns the configured tool targets, or defaults if none configured
func (c *Config) GetToolTargets() []ToolTarget {
	if len(c.ToolTargets) > 0 {
		return c.ToolTargets
	}
	return DefaultToolTargets()
}

// GetEnabledToolTargets returns only the enabled tool targets
func (c *Config) GetEnabledToolTargets() []ToolTarget {
	var enabled []ToolTarget
	for _, t := range c.GetToolTargets() {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

// GetEnabledSkillsDirs returns all enabled skill directories
func (c *Config) GetEnabledSkillsDirs() []string {
	var dirs []string
	for _, t := range c.GetEnabledToolTargets() {
		dirs = append(dirs, t.SkillsDir)
	}
	return dirs
}

// DetectExistingToolDirs detects which AI tool directories already exist in the project
func DetectExistingToolDirs(projectDir string) []ToolTarget {
	var detected []ToolTarget

	// Check for default agent directory
	if _, err := os.Stat(filepath.Join(projectDir, filepath.Dir(DefaultSkillsDir))); err == nil {
		detected = append(detected, ToolTarget{
			Name:      "agent",
			SkillsDir: DefaultSkillsDir,
			Enabled:   true,
		})
	}

	// Check for each supported agent's directory
	for name, agentConfig := range SupportedAgents {
		// Check if the tool's parent directory exists (e.g., .claude, .cursor)
		toolDir := filepath.Dir(agentConfig.ProjectDir)
		if toolDir == "." {
			toolDir = agentConfig.ProjectDir
		}
		if _, err := os.Stat(filepath.Join(projectDir, toolDir)); err == nil {
			detected = append(detected, ToolTarget{
				Name:      string(name),
				SkillsDir: agentConfig.ProjectDir,
				Enabled:   true,
			})
		}
	}
	return detected
}

// GetActiveSkillsDirs returns skill directories that exist or should be created
// If specific tool directories exist, only those are returned; otherwise returns all enabled
func (c *Config) GetActiveSkillsDirs(projectDir string) []string {
	detected := DetectExistingToolDirs(projectDir)
	if len(detected) > 0 {
		var dirs []string
		for _, t := range detected {
			if t.Enabled {
				dirs = append(dirs, t.SkillsDir)
			}
		}
		return dirs
	}
	// No specific tool detected, use default
	return []string{c.GetSkillsDir()}
}

// GetToolTargetByName returns a tool target by name
func (c *Config) GetToolTargetByName(name string) *ToolTarget {
	for _, t := range c.GetToolTargets() {
		if t.Name == name {
			return &t
		}
	}
	return nil
}

// ParseToolTargetFlags parses a comma-separated list of tool names into directories
func (c *Config) ParseToolTargetFlags(targetFlags string) []string {
	if targetFlags == "" {
		return nil
	}
	var dirs []string
	for _, name := range splitAndTrim(targetFlags, ",") {
		if t := c.GetToolTargetByName(name); t != nil && t.Enabled {
			dirs = append(dirs, t.SkillsDir)
		}
	}
	return dirs
}

// splitAndTrim splits a string and trims whitespace from each part
func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
