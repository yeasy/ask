package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCheckConfig_NotExist(t *testing.T) {
	cfg, err := LoadCheckConfig("/nonexistent/path")
	if err != nil {
		t.Fatalf("expected no error for missing config, got: %v", err)
	}
	if cfg != nil {
		t.Fatal("expected nil config for missing file")
	}
}

func TestLoadCheckConfig_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	content := `
ignore:
  - CMD-SUDO
  - NET-HTTP
ignore_paths:
  - "vendor/**"
  - "*.test.js"
rules:
  - id: CUSTOM-TODO
    pattern: "TODO|FIXME|HACK"
    severity: info
    description: "TODO comment found"
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".askcheck.yaml"), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadCheckConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if len(cfg.Ignore) != 2 {
		t.Errorf("expected 2 ignores, got %d", len(cfg.Ignore))
	}
	if len(cfg.IgnorePaths) != 2 {
		t.Errorf("expected 2 ignore_paths, got %d", len(cfg.IgnorePaths))
	}
	if len(cfg.Rules) != 1 {
		t.Errorf("expected 1 custom rule, got %d", len(cfg.Rules))
	}
}

func TestLoadCheckConfig_YmlExtension(t *testing.T) {
	tmpDir := t.TempDir()
	content := `ignore: ["CMD-SUDO"]`
	if err := os.WriteFile(filepath.Join(tmpDir, ".askcheck.yml"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadCheckConfig(tmpDir)
	if err != nil {
		t.Fatalf("failed to load .yml config: %v", err)
	}
	if cfg == nil || len(cfg.Ignore) != 1 {
		t.Fatalf("expected 1 ignore from .yml, got %v", cfg)
	}
}

func TestLoadCheckConfig_Symlink(t *testing.T) {
	dir := t.TempDir()

	// Create a real config file
	realFile := filepath.Join(dir, "real-config.yaml")
	if err := os.WriteFile(realFile, []byte("ignore: []\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink at .askcheck.yaml pointing to the real file
	symlinkPath := filepath.Join(dir, ".askcheck.yaml")
	if err := os.Symlink(realFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	_, err := LoadCheckConfig(dir)
	if err == nil {
		t.Fatal("expected error when loading config from symlink, got nil")
	}
	if !strings.Contains(err.Error(), "non-regular file") {
		t.Errorf("expected error to mention 'non-regular file', got: %v", err)
	}
}

func TestLoadCheckConfig_SymlinkToDir(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory to use as symlink target
	subDir := filepath.Join(dir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a symlink at .askcheck.yaml pointing to a directory
	symlinkPath := filepath.Join(dir, ".askcheck.yaml")
	if err := os.Symlink(subDir, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	_, err := LoadCheckConfig(dir)
	if err == nil {
		t.Fatal("expected error when .askcheck.yaml is a symlink to a directory, got nil")
	}
	if !strings.Contains(err.Error(), "non-regular file") {
		t.Errorf("expected error to mention 'non-regular file', got: %v", err)
	}
}

func TestBuildRules_IgnoresSpecified(t *testing.T) {
	cfg := &CheckConfig{
		Ignore: []string{"CMD-SUDO", "NET-HTTP"},
	}
	rules := cfg.BuildRules()

	for _, r := range rules {
		if r.ID == "CMD-SUDO" || r.ID == "NET-HTTP" {
			t.Errorf("expected rule %s to be ignored, but it was included", r.ID)
		}
	}

	// Should still have other rules
	if len(rules) == 0 {
		t.Error("expected some rules after filtering")
	}
	if len(rules) >= len(defaultRules) {
		t.Errorf("expected fewer rules after ignoring 2, got %d (default %d)", len(rules), len(defaultRules))
	}
}

func TestBuildRules_AddsCustom(t *testing.T) {
	cfg := &CheckConfig{
		Rules: []CustomRuleDef{
			{
				ID:          "CUSTOM-FIXME",
				Pattern:     `FIXME`,
				Severity:    "critical",
				Description: "FIXME found",
			},
		},
	}
	rules := cfg.BuildRules()

	found := false
	for _, r := range rules {
		if r.ID == "CUSTOM-FIXME" {
			found = true
			if r.Severity != SeverityCritical {
				t.Errorf("expected critical severity, got %s", r.Severity)
			}
		}
	}
	if !found {
		t.Error("expected custom rule CUSTOM-FIXME to be present")
	}
}

func TestBuildRules_InvalidPatternSkipped(t *testing.T) {
	cfg := &CheckConfig{
		Rules: []CustomRuleDef{
			{
				ID:      "BAD-REGEX",
				Pattern: `[invalid`,
			},
		},
	}
	rules := cfg.BuildRules()
	for _, r := range rules {
		if r.ID == "BAD-REGEX" {
			t.Error("expected invalid regex rule to be skipped")
		}
	}
}

func TestBuildRules_NilConfig(t *testing.T) {
	var cfg *CheckConfig
	rules := cfg.BuildRules()
	if len(rules) != len(defaultRules) {
		t.Errorf("nil config should return all default rules, got %d want %d", len(rules), len(defaultRules))
	}
}

func TestIsPathIgnored(t *testing.T) {
	cfg := &CheckConfig{
		IgnorePaths: []string{
			"vendor/**",
			"*.test.js",
			"**/*.min.js",
		},
	}

	tests := []struct {
		path    string
		ignored bool
	}{
		{"vendor/lib/foo.go", true},
		{"vendor/bar.js", true},
		{"src/main.go", false},
		{"app.test.js", true},
		{"src/app.test.js", true},
		{"dist/bundle.min.js", true},
		{"src/index.js", false},
	}

	for _, tt := range tests {
		got := cfg.IsPathIgnored(tt.path)
		if got != tt.ignored {
			t.Errorf("IsPathIgnored(%q) = %v, want %v", tt.path, got, tt.ignored)
		}
	}
}

func TestIsPathIgnored_NilConfig(t *testing.T) {
	var cfg *CheckConfig
	if cfg.IsPathIgnored("anything.go") {
		t.Error("nil config should never ignore paths")
	}
}

func TestCheckSafety_WithIgnoreRule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create SKILL.md
	if err := os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("---\nname: test\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file with sudo usage
	if err := os.WriteFile(filepath.Join(tmpDir, "run.sh"), []byte("sudo apt install curl\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .askcheck.yaml that ignores CMD-SUDO
	if err := os.WriteFile(filepath.Join(tmpDir, ".askcheck.yaml"), []byte("ignore:\n  - CMD-SUDO\n"), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(tmpDir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	for _, f := range result.Findings {
		if f.RuleID == "CMD-SUDO" {
			t.Error("expected CMD-SUDO to be ignored, but found it")
		}
	}
}

func TestCheckSafety_WithIgnorePath(t *testing.T) {
	tmpDir := t.TempDir()

	// Create SKILL.md
	if err := os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("---\nname: test\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create vendor directory with an AWS key
	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "lib.py"), []byte("key = 'AKIAIOSFODNN7EXAMPLE'\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .askcheck.yaml that ignores vendor/
	if err := os.WriteFile(filepath.Join(tmpDir, ".askcheck.yaml"), []byte("ignore_paths:\n  - \"vendor/**\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(tmpDir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	for _, f := range result.Findings {
		if f.RuleID == "SECRET-AWS-KEY" {
			t.Error("expected AWS key in vendor/ to be ignored, but found it")
		}
	}
}

func TestCheckSafety_WithCustomRule(t *testing.T) {
	tmpDir := t.TempDir()

	// Create SKILL.md
	if err := os.WriteFile(filepath.Join(tmpDir, "SKILL.md"), []byte("---\nname: test\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file with FIXME
	if err := os.WriteFile(filepath.Join(tmpDir, "main.py"), []byte("# FIXME: clean this up\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create .askcheck.yaml with custom rule
	cfg := `rules:
  - id: CUSTOM-FIXME
    pattern: "FIXME"
    severity: warning
    description: "FIXME comment found"
`
	if err := os.WriteFile(filepath.Join(tmpDir, ".askcheck.yaml"), []byte(cfg), 0600); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(tmpDir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	found := false
	for _, f := range result.Findings {
		if f.RuleID == "CUSTOM-FIXME" {
			found = true
		}
	}
	if !found {
		t.Error("expected custom FIXME rule to trigger, but it didn't")
	}
}
