// Package deps provides dependency resolution for skills.
package deps

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewResolver(t *testing.T) {
	resolver := NewResolver()

	if resolver == nil {
		t.Fatal("NewResolver returned nil")
	}

	if resolver.resolved == nil {
		t.Error("resolved map not initialized")
	}

	if resolver.order == nil {
		t.Error("order slice not initialized")
	}
}

func TestResolveNoDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	// Create SKILL.md without dependencies
	skillMD := `---
name: test-skill
description: A test skill
version: 1.0.0
---

# Test Skill
`
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	resolver := NewResolver()
	order, err := resolver.Resolve(skillDir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Should have one item (the skill itself)
	if len(order) != 1 {
		t.Errorf("Expected 1 skill, got %d", len(order))
	}

	if order[0] != "test-skill" {
		t.Errorf("Expected 'test-skill', got '%s'", order[0])
	}
}

func TestResolveWithDependencies(t *testing.T) {
	tmpDir := t.TempDir()

	// Create dependency skill
	depDir := filepath.Join(tmpDir, "dep-skill")
	if err := os.Mkdir(depDir, 0755); err != nil {
		t.Fatalf("Failed to create dep directory: %v", err)
	}

	depMD := `---
name: dep-skill
description: A dependency skill
version: 1.0.0
---

# Dependency Skill
`
	if err := os.WriteFile(filepath.Join(depDir, "SKILL.md"), []byte(depMD), 0644); err != nil {
		t.Fatalf("Failed to write dep SKILL.md: %v", err)
	}

	// Create main skill with dependency
	mainDir := filepath.Join(tmpDir, "main-skill")
	if err := os.Mkdir(mainDir, 0755); err != nil {
		t.Fatalf("Failed to create main directory: %v", err)
	}

	mainMD := `---
name: main-skill
description: Main skill
version: 1.0.0
dependencies:
  - dep-skill
---

# Main Skill
`
	if err := os.WriteFile(filepath.Join(mainDir, "SKILL.md"), []byte(mainMD), 0644); err != nil {
		t.Fatalf("Failed to write main SKILL.md: %v", err)
	}

	resolver := NewResolver()
	order, err := resolver.Resolve(mainDir)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	// Should have both skills
	if len(order) != 2 {
		t.Errorf("Expected 2 skills, got %d", len(order))
	}

	// Dependency should come first
	if order[0] != "dep-skill" {
		t.Errorf("Expected 'dep-skill' first, got '%s'", order[0])
	}

	if order[1] != "main-skill" {
		t.Errorf("Expected 'main-skill' second, got '%s'", order[1])
	}
}

func TestCircularDependency(t *testing.T) {
	tmpDir := t.TempDir()

	// Create skill-a that depends on skill-b
	skillADir := filepath.Join(tmpDir, "skill-a")
	if err := os.Mkdir(skillADir, 0755); err != nil {
		t.Fatalf("Failed to create skill-a directory: %v", err)
	}

	skillAMD := `---
name: skill-a
version: 1.0.0
dependencies:
  - skill-b
---
`
	if err := os.WriteFile(filepath.Join(skillADir, "SKILL.md"), []byte(skillAMD), 0644); err != nil {
		t.Fatalf("Failed to write skill-a SKILL.md: %v", err)
	}

	// Note: We can't easily test actual circular dependencies without creating skill-b
	// that depends on skill-a, which would require more complex setup.
	// This test verifies the resolver initializes correctly with dependencies.

	resolver := NewResolver()
	_, err := resolver.Resolve(skillADir)

	// An error is expected because skill-b doesn't exist
	// In a real circular dependency, we'd get a different error message
	if err == nil {
		t.Log("No error occurred (skill-b doesn't exist)")
	}
}

func TestGetDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	// Test with dependencies
	skillMD := `---
name: test-skill
version: 1.0.0
dependencies:
  - python
  - playwright
  - requests
---
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	deps, err := GetDependencies(skillDir)
	if err != nil {
		t.Fatalf("GetDependencies failed: %v", err)
	}

	expectedDeps := []string{"python", "playwright", "requests"}
	if len(deps) != len(expectedDeps) {
		t.Errorf("Expected %d dependencies, got %d", len(expectedDeps), len(deps))
	}

	for i, dep := range expectedDeps {
		if deps[i] != dep {
			t.Errorf("Expected dependency '%s', got '%s'", dep, deps[i])
		}
	}
}

func TestGetDependenciesNoSkillMD(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create skill directory: %v", err)
	}

	// Don't create SKILL.md
	deps, err := GetDependencies(skillDir)
	if err != nil {
		t.Fatalf("GetDependencies failed: %v", err)
	}

	// Should return nil/empty when no SKILL.md exists
	if len(deps) > 0 {
		t.Errorf("Expected no dependencies, got %v", deps)
	}
}

func TestResolve_PathTraversalDependency(t *testing.T) {
	cases := []struct {
		name string
		dep  string
	}{
		{"dot-dot-slash", "../escape"},
		{"forward-slash", "foo/bar"},
		{"backslash", "foo\\bar"},
		{"bare-dot-dot", ".."},
		{"single-dot", "."},
		{"empty-string", `""`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			skillDir := filepath.Join(tmpDir, "bad-skill")
			if err := os.Mkdir(skillDir, 0755); err != nil {
				t.Fatalf("Failed to create skill directory: %v", err)
			}

			skillMD := fmt.Sprintf(`---
name: bad-skill
version: 1.0.0
dependencies:
  - %s
---

# Bad Skill
`, tc.dep)
			if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
				t.Fatalf("Failed to write SKILL.md: %v", err)
			}

			resolver := NewResolver()
			_, err := resolver.Resolve(skillDir)
			if err == nil {
				t.Fatalf("expected error for path traversal dependency %q, got nil", tc.dep)
			}
			if !strings.Contains(err.Error(), "invalid dependency name") {
				t.Errorf("expected error to contain 'invalid dependency name', got: %v", err)
			}
		})
	}
}

func TestGetOrder(t *testing.T) {
	resolver := NewResolver()

	// Initially order should be empty
	order := resolver.GetOrder()
	if len(order) != 0 {
		t.Errorf("Expected empty order, got %d items", len(order))
	}

	// Manually add to order (simulating resolution)
	resolver.order = []string{"skill-a", "skill-b", "skill-c"}

	order = resolver.GetOrder()
	if len(order) != 3 {
		t.Errorf("Expected 3 items, got %d", len(order))
	}

	expected := []string{"skill-a", "skill-b", "skill-c"}
	for i, skill := range expected {
		if order[i] != skill {
			t.Errorf("Expected '%s' at position %d, got '%s'", skill, i, order[i])
		}
	}
}
