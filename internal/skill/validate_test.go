package skill

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMeta_Valid(t *testing.T) {
	meta := &Meta{
		Name:        "my-skill",
		Description: "A valid skill description",
	}

	validationErrors := ValidateMeta(meta, "my-skill")
	if len(validationErrors) != 0 {
		t.Errorf("Expected no errors for valid meta, got %d: %+v", len(validationErrors), validationErrors)
	}
}

func TestValidateMeta_MissingName(t *testing.T) {
	meta := &Meta{
		Description: "A skill without a name",
	}

	validationErrors := ValidateMeta(meta, "test")
	found := false
	for _, e := range validationErrors {
		if e.Field == "name" && strings.Contains(e.Message, "required") {
			found = true
		}
	}
	if !found {
		t.Error("Expected error for missing name")
	}
}

func TestValidateMeta_UppercaseName(t *testing.T) {
	meta := &Meta{
		Name:        "My-Skill",
		Description: "A skill with uppercase name",
	}

	validationErrors := ValidateMeta(meta, "My-Skill")
	found := false
	for _, e := range validationErrors {
		if e.Field == "name" && strings.Contains(e.Message, "lowercase") {
			found = true
		}
	}
	if !found {
		t.Error("Expected error for uppercase name")
	}
}

func TestValidateMeta_ConsecutiveHyphens(t *testing.T) {
	meta := &Meta{
		Name:        "my--skill",
		Description: "A skill with consecutive hyphens",
	}

	validationErrors := ValidateMeta(meta, "my--skill")
	found := false
	for _, e := range validationErrors {
		if e.Field == "name" && strings.Contains(e.Message, "consecutive") {
			found = true
		}
	}
	if !found {
		t.Error("Expected error for consecutive hyphens")
	}
}

func TestValidateMeta_LeadingHyphen(t *testing.T) {
	meta := &Meta{
		Name:        "-myskill",
		Description: "A skill with leading hyphen",
	}

	validationErrors := ValidateMeta(meta, "-myskill")
	found := false
	for _, e := range validationErrors {
		if e.Field == "name" && strings.Contains(e.Message, "start or end") {
			found = true
		}
	}
	if !found {
		t.Error("Expected error for leading hyphen")
	}
}

func TestValidateMeta_MissingDescription(t *testing.T) {
	meta := &Meta{
		Name: "my-skill",
	}

	validationErrors := ValidateMeta(meta, "my-skill")
	found := false
	for _, e := range validationErrors {
		if e.Field == "description" && strings.Contains(e.Message, "required") {
			found = true
		}
	}
	if !found {
		t.Error("Expected error for missing description")
	}
}

func TestValidateMeta_DescriptionTooLong(t *testing.T) {
	meta := &Meta{
		Name:        "my-skill",
		Description: strings.Repeat("a", 1025),
	}

	validationErrors := ValidateMeta(meta, "my-skill")
	found := false
	for _, e := range validationErrors {
		if e.Field == "description" && strings.Contains(e.Message, "1-1024") {
			found = true
		}
	}
	if !found {
		t.Error("Expected error for description too long")
	}
}

func TestValidateMeta_DirectoryMismatch(t *testing.T) {
	meta := &Meta{
		Name:        "my-skill",
		Description: "Valid description",
	}

	validationErrors := ValidateMeta(meta, "different-name")
	found := false
	for _, e := range validationErrors {
		if e.Field == "name" && strings.Contains(e.Message, "should match") {
			found = true
		}
	}
	if !found {
		t.Error("Expected warning for name/directory mismatch")
	}
}

func TestValidateMeta_CompatibilityTooLong(t *testing.T) {
	meta := &Meta{
		Name:          "my-skill",
		Description:   "Valid description",
		Compatibility: strings.Repeat("x", 501),
	}

	validationErrors := ValidateMeta(meta, "my-skill")
	found := false
	for _, e := range validationErrors {
		if e.Field == "compatibility" && strings.Contains(e.Message, "1-500") {
			found = true
		}
	}
	if !found {
		t.Error("Expected warning for compatibility too long")
	}
}

func TestCheckSafety_IncludesValidation(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "Bad--Name")
	if err := os.Mkdir(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create SKILL.md with invalid name (uppercase, consecutive hyphens)
	skillMD := `---
name: Bad--Name
description: Test skill
---
# Test
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := CheckSafety(skillDir)
	if err != nil {
		t.Fatalf("CheckSafety failed: %v", err)
	}

	// Should find validation errors
	foundFormat := false
	for _, f := range result.Findings {
		if strings.HasPrefix(f.RuleID, "SKILL-FORMAT-") {
			foundFormat = true
			break
		}
	}

	if !foundFormat {
		t.Error("Expected SKILL-FORMAT-* findings for invalid skill name")
	}
}
