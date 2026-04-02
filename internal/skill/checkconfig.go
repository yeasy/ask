package skill

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// CheckConfig represents a .askcheck.yaml configuration file
type CheckConfig struct {
	// Ignore lists rule IDs to suppress (e.g., ["SECRET-GENERIC-TOKEN", "CMD-SUDO"])
	Ignore []string `yaml:"ignore"`
	// IgnorePaths lists file/directory glob patterns to skip (e.g., ["vendor/**", "*.test.js"])
	IgnorePaths []string `yaml:"ignore_paths"`
	// Rules defines additional custom rules
	Rules []CustomRuleDef `yaml:"rules"`

	// SkillBundled indicates the config was loaded from within a skill directory.
	// When true, CRITICAL severity built-in rules cannot be ignored.
	// This prevents a malicious skill from bundling a config that disables its own security audit.
	SkillBundled bool `yaml:"-"`
}

// CustomRuleDef represents a user-defined rule in .askcheck.yaml
type CustomRuleDef struct {
	ID          string `yaml:"id"`
	Pattern     string `yaml:"pattern"`
	Severity    string `yaml:"severity"`
	Description string `yaml:"description"`
}

// maxCheckConfigSize limits the .askcheck.yaml file size
const maxCheckConfigSize = 256 * 1024 // 256KB

// readFileIfSafe reads a file after verifying it is not a symlink and within size limit.
// Uses Lstat pre-check for symlinks, then open-then-fstat for size validation.
func readFileIfSafe(path string, maxSize int64) ([]byte, error) {
	// Pre-check for symlinks before opening (Lstat does not follow symlinks)
	linfo, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read non-regular file: %s", path)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	// Verify via fd (fstat) for size to avoid TOCTOU on size check
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file: %s", path)
	}
	if info.Size() > maxSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", info.Size(), maxSize)
	}
	return io.ReadAll(io.LimitReader(f, maxSize))
}

// LoadCheckConfig loads .askcheck.yaml from the given directory.
// Returns nil (no error) if the file does not exist.
func LoadCheckConfig(dir string) (*CheckConfig, error) {
	path := filepath.Join(dir, ".askcheck.yaml")
	data, err := readFileIfSafe(path, maxCheckConfigSize)
	if err != nil {
		if os.IsNotExist(err) {
			// Also try .askcheck.yml
			path = filepath.Join(dir, ".askcheck.yml")
			data, err = readFileIfSafe(path, maxCheckConfigSize)
			if err != nil {
				if os.IsNotExist(err) {
					return nil, nil
				}
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	var cfg CheckConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// protectedRuleIDs returns the set of built-in rule IDs with CRITICAL severity.
// These rules cannot be ignored by skill-bundled .askcheck.yaml configs.
func protectedRuleIDs() map[string]bool {
	protected := make(map[string]bool)
	for _, r := range defaultRules {
		if r.Severity == SeverityCritical {
			protected[strings.ToUpper(r.ID)] = true
		}
	}
	return protected
}

// BuildRules returns the effective rule set: default rules (minus ignored) plus custom rules.
// When SkillBundled is true, built-in CRITICAL rules cannot be ignored.
func (cc *CheckConfig) BuildRules() []Rule {
	ignoreSet := make(map[string]bool)
	if cc != nil {
		protected := protectedRuleIDs()
		for _, id := range cc.Ignore {
			upper := strings.ToUpper(id)
			// Skill-bundled configs cannot ignore built-in CRITICAL rules
			if cc.SkillBundled && protected[upper] {
				continue
			}
			ignoreSet[upper] = true
		}
	}

	// Start with default rules, filtering out ignored
	var rules []Rule
	for _, r := range defaultRules {
		if !ignoreSet[strings.ToUpper(r.ID)] {
			rules = append(rules, r)
		}
	}

	// Append custom rules
	if cc != nil {
		for _, cr := range cc.Rules {
			compiled, err := regexp.Compile(cr.Pattern)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping custom rule %q: invalid pattern: %v\n", cr.ID, err)
				continue
			}
			sev := SeverityWarning
			switch strings.ToLower(cr.Severity) {
			case "critical":
				sev = SeverityCritical
			case "info":
				sev = SeverityInfo
			}
			rules = append(rules, Rule{
				ID:          cr.ID,
				Description: cr.Description,
				Severity:    sev,
				Regex:       compiled,
			})
		}
	}

	return rules
}

// IsPathIgnored returns true if the relative path matches any ignore_paths pattern.
func (cc *CheckConfig) IsPathIgnored(relPath string) bool {
	if cc == nil {
		return false
	}
	for _, pattern := range cc.IgnorePaths {
		if matched, _ := filepath.Match(pattern, relPath); matched {
			return true
		}
		// Also check against just the filename
		if matched, _ := filepath.Match(pattern, filepath.Base(relPath)); matched {
			return true
		}
		// Support ** prefix by matching the suffix pattern against each sub-path.
		// For example, pattern "**/vendor" should match "src/vendor" but not
		// "src/vendor-tools/file.go", and "**/*.min.js" should match
		// "dist/bundle.min.js".
		if strings.HasPrefix(pattern, "**") {
			suffix := strings.TrimPrefix(pattern, "**")
			suffix = strings.TrimPrefix(suffix, "/")
			suffix = strings.TrimPrefix(suffix, string(filepath.Separator))
			// Try matching suffix against the full path and every sub-path
			// obtained by stripping leading directories one at a time.
			candidate := relPath
			for {
				if matched, _ := filepath.Match(suffix, candidate); matched {
					return true
				}
				i := strings.IndexAny(candidate, "/"+string(filepath.Separator))
				if i < 0 {
					break
				}
				candidate = candidate[i+1:]
			}
		}
		// Support directory prefix patterns like "vendor/**"
		if strings.HasSuffix(pattern, "/**") {
			prefix := strings.TrimSuffix(pattern, "/**")
			if strings.HasPrefix(relPath, prefix+"/") || strings.HasPrefix(relPath, prefix+string(filepath.Separator)) || relPath == prefix {
				return true
			}
		}
	}
	return false
}
