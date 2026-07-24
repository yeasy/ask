package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSkillMDWithFrontmatter(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a SKILL.md with frontmatter
	skillMD := `---
name: test-skill
description: A test skill for testing
version: 1.0.0
author: Test Author
tags:
  - test
  - example
dependencies:
  - python
---

# Test Skill

This is a test skill for unit testing.
`
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Parse the skill
	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Failed to parse SKILL.md: %v", err)
	}

	// Verify metadata
	if meta.Name != "test-skill" {
		t.Errorf("Expected name 'test-skill', got '%s'", meta.Name)
	}
	if meta.Description != "A test skill for testing" {
		t.Errorf("Expected description 'A test skill for testing', got '%s'", meta.Description)
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got '%s'", meta.Version)
	}
	if meta.Author != "Test Author" {
		t.Errorf("Expected author 'Test Author', got '%s'", meta.Author)
	}
	if len(meta.Tags) != 2 {
		t.Errorf("Expected 2 tags, got %d", len(meta.Tags))
	}
	if len(meta.Dependencies) != 1 {
		t.Errorf("Expected 1 dependency, got %d", len(meta.Dependencies))
	}
}

func TestParseSkillMDWithoutFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a SKILL.md without frontmatter
	skillMD := `# Browser Use

A skill for browser automation using Playwright.

## Features
- Web scraping
`
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Parse the skill
	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Failed to parse SKILL.md: %v", err)
	}

	// Verify metadata extracted from content
	if meta.Name != "Browser Use" {
		t.Errorf("Expected name 'Browser Use', got '%s'", meta.Name)
	}
	if meta.Description != "A skill for browser automation using Playwright." {
		t.Errorf("Expected description from content, got '%s'", meta.Description)
	}
}

func TestFindSkillMD(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Test without SKILL.md
	if FindSkillMD(skillDir) {
		t.Error("Expected FindSkillMD to return false, got true")
	}

	// Create SKILL.md
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMDPath, []byte("# Test"), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Test with SKILL.md
	if !FindSkillMD(skillDir) {
		t.Error("Expected FindSkillMD to return true, got false")
	}
}

func TestCreateSkillTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	skillName := "my-test-skill"

	// Create skill template
	err := CreateSkillTemplateWithData(TemplateData{
		Name:        skillName,
		Description: "A new skill for AI Agents",
		Author:      GetGitAuthor(),
	}, tmpDir)
	if err != nil {
		t.Fatalf("Failed to create skill template: %v", err)
	}

	skillDir := filepath.Join(tmpDir, skillName)

	// Verify directory structure
	expectedDirs := []string{
		skillDir,
		filepath.Join(skillDir, "scripts"),
		filepath.Join(skillDir, "references"),
		filepath.Join(skillDir, "assets"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Expected directory %s to exist", dir)
		}
	}

	// Verify SKILL.md exists
	skillMDPath := filepath.Join(skillDir, "SKILL.md")
	if _, err := os.Stat(skillMDPath); os.IsNotExist(err) {
		t.Error("Expected SKILL.md to exist")
	}

	// Verify script exists
	scriptPath := filepath.Join(skillDir, "scripts", "hello.sh")
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		t.Error("Expected hello.sh script to exist")
	}

	// Verify reference exists
	refPath := filepath.Join(skillDir, "references", "ref.md")
	if _, err := os.Stat(refPath); os.IsNotExist(err) {
		t.Error("Expected ref.md to exist")
	}

	// Parse and verify SKILL.md metadata
	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Failed to parse generated SKILL.md: %v", err)
	}

	if meta.Name != skillName {
		t.Errorf("Expected name '%s', got '%s'", skillName, meta.Name)
	}
	if meta.Description == "" {
		t.Error("Expected description to be set")
	}
	if meta.Version != "0.1.0" {
		t.Errorf("Expected version '0.1.0', got '%s'", meta.Version)
	}
}

func TestGetGitAuthor(t *testing.T) {
	author := GetGitAuthor()

	// Should return a non-empty string (either from git config or "User")
	if author == "" {
		t.Error("Expected GetGitAuthor to return a non-empty string")
	}

	// The author should be either from git config or the fallback "User"
	// We can't test the exact value as it depends on the environment
	t.Logf("Git author: %s", author)
}

func TestParseSkillMD_MalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "bad-yaml")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Invalid YAML: tabs instead of spaces, broken structure
	skillMD := "---\nname: test\n\tdescription: broken\n  bad:\n    - [unclosed\n---\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	_, err := ParseSkillMD(skillDir)
	if err == nil {
		t.Error("Expected error for malformed YAML, got nil")
	}
}

func TestParseSkillMD_EmptyFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "empty-fm")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Frontmatter delimiters with nothing between them
	skillMD := "---\n---\n# My Skill\nSome description here.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Empty frontmatter means zero lines collected, so it falls through to parseFromContent
	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if meta.Name != "My Skill" {
		t.Errorf("Expected name 'My Skill' from content fallback, got '%s'", meta.Name)
	}
}

func TestParseSkillMD_FrontmatterNotOnFirstLine(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "late-fm")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Frontmatter delimiter starting on line 3 (past lineCount <= 2 check)
	skillMD := "Some preamble text\nAnother line\n---\nname: ignored\n---\n# Actual Title\nDescription text.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// The --- on line 3 should not be treated as frontmatter start (lineCount > 2)
	// so it falls back to parseFromContent
	if meta.Name != "Actual Title" {
		t.Errorf("Expected name 'Actual Title' from content fallback, got '%s'", meta.Name)
	}
}

func TestParseSkillMD_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	_, err := ParseSkillMD(filepath.Join(tmpDir, "nonexistent"))
	if err == nil {
		t.Error("Expected error when SKILL.md does not exist, got nil")
	}
}

func TestParseSkillMD_AllFields(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "full-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	skillMD := `---
name: full-skill
description: A fully specified skill
version: 2.3.1
author: Jane Doe
license: MIT
compatibility: ">=1.0.0"
tags:
  - automation
  - testing
dependencies:
  - python
  - nodejs
allowed-tools:
  - bash
  - curl
metadata:
  category: devtools
  priority: high
---

# Full Skill
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if meta.License != "MIT" {
		t.Errorf("Expected license 'MIT', got '%s'", meta.License)
	}
	if meta.Compatibility != ">=1.0.0" {
		t.Errorf("Expected compatibility '>=1.0.0', got '%s'", meta.Compatibility)
	}
	if len(meta.AllowedTools) != 2 || meta.AllowedTools[0] != "bash" {
		t.Errorf("Expected allowed-tools [bash, curl], got %v", meta.AllowedTools)
	}
	if len(meta.Metadata) != 2 || meta.Metadata["category"] != "devtools" {
		t.Errorf("Expected metadata map with category=devtools, got %v", meta.Metadata)
	}
	if len(meta.Dependencies) != 2 {
		t.Errorf("Expected 2 dependencies, got %d", len(meta.Dependencies))
	}
}

func TestParseSkillMD_OnlyTitleNoDescription(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "title-only")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Only a title, no paragraph text following
	skillMD := "# Just A Title\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if meta.Name != "Just A Title" {
		t.Errorf("Expected name 'Just A Title', got '%s'", meta.Name)
	}
	if meta.Description != "" {
		t.Errorf("Expected empty description, got '%s'", meta.Description)
	}
}

func TestParseSkillMD_ContentWithMultipleHeadings(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "multi-heading")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Multiple H1 headings; should only capture the first one
	skillMD := "# First Title\nFirst description paragraph.\n\n# Second Title\nSecond description.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if meta.Name != "First Title" {
		t.Errorf("Expected name 'First Title', got '%s'", meta.Name)
	}
	if meta.Description != "First description paragraph." {
		t.Errorf("Expected first description, got '%s'", meta.Description)
	}
}

func TestParseSkillMD_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "empty-file")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(""), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// Empty file should return empty meta from parseFromContent
	if meta.Name != "" {
		t.Errorf("Expected empty name, got '%s'", meta.Name)
	}
}

func TestParseSkillMD_FrontmatterOnSecondLine(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "second-line")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Blank first line, then frontmatter delimiter on line 2 (lineCount <= 2 is true)
	skillMD := "\n---\nname: second-line\ndescription: Starts on line two\n---\n# Content\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if meta.Name != "second-line" {
		t.Errorf("Expected name 'second-line', got '%s'", meta.Name)
	}
	if meta.Description != "Starts on line two" {
		t.Errorf("Expected description 'Starts on line two', got '%s'", meta.Description)
	}
}

func TestParseSkillMD_FrontmatterWithExtraWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "whitespace")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Frontmatter delimiter with leading/trailing spaces
	skillMD := "  ---  \nname: whitespace-skill\ndescription: Trimmed delimiters\n  ---  \n# Title\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	// TrimSpace on "  ---  " equals "---", so frontmatter should be detected
	if meta.Name != "whitespace-skill" {
		t.Errorf("Expected name 'whitespace-skill', got '%s'", meta.Name)
	}
}

func TestParseSkillMD_UnclosedFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "unclosed")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Opening --- but no closing ---
	skillMD := "---\nname: unclosed\ndescription: No closing delimiter\n# Title\nBody text.\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	meta, err := ParseSkillMD(skillDir)
	if err != nil {
		// The entire file content after --- will be treated as YAML, which may or may not parse
		// Either outcome is acceptable; just verify no panic
		t.Logf("Got expected error for unclosed frontmatter: %v", err)
		return
	}
	// If it parsed, the name should come from the YAML lines collected
	if meta.Name != "unclosed" {
		t.Errorf("Expected name 'unclosed', got '%s'", meta.Name)
	}
}

func TestValidateMeta_NameExactly64Chars(t *testing.T) {
	// Exactly 64 characters should be valid
	name := strings.Repeat("a", 64)
	meta := &Meta{
		Name:        name,
		Description: "Valid description",
	}

	errs := ValidateMeta(meta, name)
	for _, e := range errs {
		if e.Field == "name" && strings.Contains(e.Message, "1-64") {
			t.Error("Name of exactly 64 chars should not trigger length error")
		}
	}
}

func TestValidateMeta_NameExceeds64Chars(t *testing.T) {
	name := strings.Repeat("a", 65)
	meta := &Meta{
		Name:        name,
		Description: "Valid description",
	}

	errs := ValidateMeta(meta, name)
	found := false
	for _, e := range errs {
		if e.Field == "name" && strings.Contains(e.Message, "1-64") {
			found = true
		}
	}
	if !found {
		t.Error("Expected length error for 65-char name")
	}
}

func TestValidateMeta_TrailingHyphen(t *testing.T) {
	meta := &Meta{
		Name:        "myskill-",
		Description: "A skill with trailing hyphen",
	}

	errs := ValidateMeta(meta, "myskill-")
	found := false
	for _, e := range errs {
		if e.Field == "name" && strings.Contains(e.Message, "start or end") {
			found = true
		}
	}
	if !found {
		t.Error("Expected error for trailing hyphen")
	}
}

func TestValidateMeta_NameWithSpecialChars(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
		wantErr   bool
	}{
		{"underscore", "my_skill", true},
		{"dot", "my.skill", true},
		{"space", "my skill", true},
		{"at sign", "my@skill", true},
		{"digits only", "123", false},          // digits are allowed but regex requires starting with a-z
		{"starts with digit", "1skill", false}, // nameRegex requires ^[a-z], but only "hasOther" check fires for non a-z/digit/hyphen
		{"valid with digits", "my-skill-2", false},
		{"single char", "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &Meta{
				Name:        tt.inputName,
				Description: "test",
			}
			errs := ValidateMeta(meta, tt.inputName)
			hasNameErr := false
			for _, e := range errs {
				if e.Field == "name" && e.Severity == SeverityCritical {
					hasNameErr = true
					break
				}
			}
			if tt.wantErr && !hasNameErr {
				t.Errorf("Expected critical name error for %q, got none", tt.inputName)
			}
			if !tt.wantErr && hasNameErr {
				t.Errorf("Did not expect critical name error for %q, but got one", tt.inputName)
			}
		})
	}
}

func TestValidateMeta_DescriptionBoundary(t *testing.T) {
	// Exactly 1024 chars should be valid
	meta := &Meta{
		Name:        "my-skill",
		Description: strings.Repeat("a", 1024),
	}
	errs := ValidateMeta(meta, "my-skill")
	for _, e := range errs {
		if e.Field == "description" && strings.Contains(e.Message, "1-1024") {
			t.Error("Description of exactly 1024 chars should not trigger length error")
		}
	}
}

func TestValidateMeta_CompatibilityBoundary(t *testing.T) {
	// Exactly 500 chars should be valid
	meta := &Meta{
		Name:          "my-skill",
		Description:   "test",
		Compatibility: strings.Repeat("x", 500),
	}
	errs := ValidateMeta(meta, "my-skill")
	for _, e := range errs {
		if e.Field == "compatibility" {
			t.Error("Compatibility of exactly 500 chars should not trigger error")
		}
	}
}

func TestValidateMeta_EmptyDirName(t *testing.T) {
	// When dirName is empty, no mismatch warning should be generated
	meta := &Meta{
		Name:        "my-skill",
		Description: "test",
	}
	errs := ValidateMeta(meta, "")
	for _, e := range errs {
		if strings.Contains(e.Message, "should match") {
			t.Error("Expected no directory mismatch warning when dirName is empty")
		}
	}
}

func TestFindSkillMD_NonexistentDir(t *testing.T) {
	result := FindSkillMD("/nonexistent/path/that/does/not/exist")
	if result {
		t.Error("Expected FindSkillMD to return false for nonexistent directory")
	}
}

func TestParseSkillMD_LargeFileWithoutFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "large-skill")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a file larger than maxSkillFileSize (1MB)
	largeContent := "# Large Skill\n" + strings.Repeat("x", 1024*1024+100)
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(largeContent), 0644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	_, err := ParseSkillMD(skillDir)
	if err == nil {
		t.Error("Expected error for oversized file without frontmatter, got nil")
	}
	// Either "too large" from parseFromContent or "token too long" from bufio.Scanner
	if err != nil {
		errMsg := err.Error()
		if !strings.Contains(errMsg, "too large") && !strings.Contains(errMsg, "token too long") {
			t.Errorf("Expected 'too large' or 'token too long' error, got: %v", err)
		}
	}
}

func TestScanDirectory_Basic(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two skill directories
	for _, name := range []string{"skill-a", "skill-b"} {
		skillDir := filepath.Join(tmpDir, name)
		if err := os.Mkdir(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		content := fmt.Sprintf("# %s\nDescription of %s.\n", name, name)
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := ScanDirectory(tmpDir, 3)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 skills, got %d", len(results))
	}
}

func TestScanDirectory_SkipsHiddenDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a hidden directory with a SKILL.md inside
	hiddenDir := filepath.Join(tmpDir, ".hidden-skill")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "SKILL.md"), []byte("# Hidden\nSecret.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := ScanDirectory(tmpDir, 3)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 skills (hidden dir skipped), got %d", len(results))
	}
}

func TestScanDirectory_RespectsDepthLimit(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a deeply nested skill: tmpDir/a/b/c/deep-skill/SKILL.md (depth 4)
	deepDir := filepath.Join(tmpDir, "a", "b", "c", "deep-skill")
	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deepDir, "SKILL.md"), []byte("# Deep\nDeep skill.\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Depth limit 2 should not find it
	results, err := ScanDirectory(tmpDir, 2)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("Expected 0 skills with depth limit 2, got %d", len(results))
	}

	// Depth limit 5 should find it
	results, err = ScanDirectory(tmpDir, 5)
	if err != nil {
		t.Fatalf("ScanDirectory failed: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Expected 1 skill with depth limit 5, got %d", len(results))
	}
}

func TestScanDirectory_NonexistentRoot(t *testing.T) {
	_, err := ScanDirectory("/nonexistent/root/path", 3)
	if err == nil {
		t.Error("Expected error for nonexistent root directory")
	}
}
