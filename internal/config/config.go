package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"

	"gopkg.in/yaml.v3"
)

// offlineMode indicates if the application is running in offline mode (atomic for thread safety)
var offlineMode atomic.Bool

// IsOffline returns whether the application is in offline mode (thread-safe)
func IsOffline() bool {
	return offlineMode.Load()
}

// SetOffline sets the offline mode (thread-safe)
func SetOffline(offline bool) {
	offlineMode.Store(offline)
}

// Repo represents a skill repository
type Repo struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`                     // "topic", "dir", "registry", or "skillhub"
	URL     string `yaml:"url"`                      // GitHub topic or "owner/repo/path"
	Token   string `yaml:"token,omitempty" json:"-"` // Per-repo auth token (private repos)
	BaseURL string `yaml:"base_url,omitempty"`       // GitHub Enterprise base URL
	Private bool   `yaml:"private,omitempty"`        // Whether the repo is private
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

	if !IsOffline() {
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

// EnterpriseConfig holds enterprise-level policy enforcement settings
type EnterpriseConfig struct {
	AllowedSources  []string `yaml:"allowed_sources,omitempty"`  // Glob patterns for allowed skill sources
	RequireCheck    bool     `yaml:"require_check,omitempty"`    // Require security check before/after install
	RequireLock     bool     `yaml:"require_lock,omitempty"`     // Require ask.lock for all installs
	PrivateRegistry string   `yaml:"private_registry,omitempty"` // GitHub Enterprise API base URL
}

// Config represents the structure of ask.yaml
type Config struct {
	Version         string            `yaml:"version"`
	SkillsDir       string            `yaml:"skills_dir,omitempty"`   // Skills installation directory
	ToolTargets     []ToolTarget      `yaml:"tool_targets,omitempty"` // Target AI tools
	Skills          []string          `yaml:"skills,omitempty"`       // Legacy: simple list of skill names
	SkillsInfo      []SkillInfo       `yaml:"skills_info,omitempty"`  // Skills with metadata
	Repos           []Repo            `yaml:"repos,omitempty"`
	Enterprise      *EnterpriseConfig `yaml:"enterprise,omitempty"`
	LastProjectRoot string            `yaml:"last_project_root,omitempty"`
}

// GetAllSkillNames returns a deduplicated list of skill names from both
// the legacy Skills list and the SkillsInfo list.
func (c *Config) GetAllSkillNames() []string {
	if c == nil {
		return nil
	}
	allSkills := make([]string, 0, len(c.Skills)+len(c.SkillsInfo))
	seen := make(map[string]bool)
	for _, s := range c.Skills {
		if !seen[s] {
			seen[s] = true
			allSkills = append(allSkills, s)
		}
	}
	for _, si := range c.SkillsInfo {
		if !seen[si.Name] {
			seen[si.Name] = true
			allSkills = append(allSkills, si.Name)
		}
	}
	return allSkills
}

// IsSourceAllowed checks if a URL matches any of the allowed source patterns.
// Patterns support glob-style matching (e.g. "anthropics/*", "company-org/*").
func IsSourceAllowed(sourceURL string, allowedPatterns []string) bool {
	if len(allowedPatterns) == 0 {
		return true
	}
	// Normalize URL: strip https://github.com/ prefix
	normalized := strings.TrimPrefix(sourceURL, "https://github.com/")
	normalized = strings.TrimPrefix(normalized, "http://github.com/")
	normalized = strings.TrimSuffix(normalized, ".git")

	for _, pattern := range allowedPatterns {
		matched, err := filepath.Match(pattern, normalized)
		if err == nil && matched {
			return true
		}
		// Simple prefix match for patterns like "company-org/*"
		prefix := strings.TrimSuffix(pattern, "/*")
		if prefix != pattern && strings.HasPrefix(normalized, prefix+"/") {
			return true
		}
	}
	return false
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

// GetGlobalConfigDir returns the global config directory path (~/.ask).
// Returns an error if the user's home directory cannot be determined.
func GetGlobalConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, GlobalConfigDirName), nil
}

// GetGlobalConfigPath returns the global config file path (~/.ask/config.yaml).
// Returns an error if the user's home directory cannot be determined.
func GetGlobalConfigPath() (string, error) {
	dir, err := GetGlobalConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, GlobalConfigFileName), nil
}

// GetGlobalSkillsDir returns the global skills directory path (~/.ask/skills).
// Returns an error if the user's home directory cannot be determined.
func GetGlobalSkillsDir() (string, error) {
	dir, err := GetGlobalConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, GlobalSkillsDirName), nil
}

// GetGlobalLockPath returns the global lock file path (~/.ask/ask.lock).
// Returns an error if the user's home directory cannot be determined.
func GetGlobalLockPath() (string, error) {
	dir, err := GetGlobalConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, GlobalLockFileName), nil
}

// EnsureGlobalDirExists creates the global config directory if it doesn't exist
func EnsureGlobalDirExists() error {
	globalDir, err := GetGlobalConfigDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(globalDir, 0755)
}

// GetSkillsDirByScope returns the skills directory based on global flag
func GetSkillsDirByScope(global bool) (string, error) {
	if global {
		return GetGlobalSkillsDir()
	}
	return DefaultSkillsDir, nil
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
				Name: "featured",
				Type: "registry",
				URL:  "yeasy/awesome-agent-skills/registry/index.json",
			},
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
			{
				Name: "openclaw",
				Type: "dir",
				URL:  "openclaw/openclaw/skills",
			},
		},
	}
}

// maxConfigFileSize limits the config file size to prevent OOM on malformed files
const maxConfigFileSize = 1024 * 1024 // 1MB

// loadConfigFromPath loads and merges a config from the given file path.
// Uses Lstat pre-check for symlinks, then open-then-fstat for size validation.
func loadConfigFromPath(path string) (*Config, error) {
	// Pre-check for symlinks (Lstat does not follow symlinks)
	linfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if linfo.Mode()&os.ModeSymlink != 0 || !linfo.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file: %s", path)
	}
	if info.Size() > maxConfigFileSize {
		return nil, fmt.Errorf("config file too large: %d bytes (max %d)", info.Size(), maxConfigFileSize)
	}
	data, err := io.ReadAll(io.LimitReader(f, maxConfigFileSize))
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

// Save saves the configuration to ask.yaml atomically
func (c *Config) Save() error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return atomicWriteFile("ask.yaml", data, 0600)
}

// atomicWriteFile writes data to a temp file then renames it to the target path.
// This prevents partial writes from corrupting the file.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
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
	for i := range c.SkillsInfo {
		if c.SkillsInfo[i].Name == name {
			return &c.SkillsInfo[i]
		}
	}
	return nil
}

// CreateDefaultConfig creates a new ask.yaml in the current directory
func CreateDefaultConfig() error {
	config := DefaultConfig()
	return config.Save()
}

// CreateConfigWithAgents creates ask.yaml with specific agents enabled
func CreateConfigWithAgents(agents []string) error {
	cfg := DefaultConfig()
	var targets []ToolTarget
	// Always include default agent directory
	targets = append(targets, ToolTarget{
		Name:      "agent",
		SkillsDir: DefaultSkillsDir,
		Enabled:   true,
	})
	for _, name := range agents {
		if agentType, ok := ResolveAgentType(name); ok {
			ac := SupportedAgents[agentType]
			targets = append(targets, ToolTarget{
				Name:      string(agentType),
				SkillsDir: ac.ProjectDir,
				Enabled:   true,
			})
		}
	}
	cfg.ToolTargets = targets
	return cfg.Save()
}

// LoadGlobalConfig loads the global config file (~/.ask/config.yaml)
func LoadGlobalConfig() (*Config, error) {
	path, err := GetGlobalConfigPath()
	if err != nil {
		return nil, err
	}
	cfg, err := loadConfigFromPath(path)
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

// SaveGlobal saves the configuration to the global config file (~/.ask/config.yaml) atomically
func (c *Config) SaveGlobal() error {
	if err := EnsureGlobalDirExists(); err != nil {
		return err
	}

	path, err := GetGlobalConfigPath()
	if err != nil {
		return err
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data, 0600)
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

	// Collect agent names and sort for deterministic output
	agentNames := make([]string, 0, len(SupportedAgents))
	for name := range SupportedAgents {
		agentNames = append(agentNames, string(name))
	}
	sort.Strings(agentNames)

	// Check for each supported agent's directory
	for _, nameStr := range agentNames {
		agentConfig := SupportedAgents[AgentType(nameStr)]
		// Check if the tool's parent directory exists (e.g., .claude, .cursor)
		toolDir := filepath.Dir(agentConfig.ProjectDir)
		if toolDir == "." {
			toolDir = agentConfig.ProjectDir
		}
		if _, err := os.Stat(filepath.Join(projectDir, toolDir)); err == nil {
			detected = append(detected, ToolTarget{
				Name:      nameStr,
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
	targets := c.GetToolTargets()
	for i := range targets {
		if targets[i].Name == name {
			return &targets[i]
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
