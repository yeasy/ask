package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config.Version != "1.2" {
		t.Errorf("Expected version 1.2, got %s", config.Version)
	}
	if len(config.Skills) != 0 {
		t.Errorf("Expected empty skills list, got %d", len(config.Skills))
	}
	// We now have 6 default repos: featured, anthropics, openai, composio, vercel, openclaw
	if len(config.Repos) != 6 {
		t.Errorf("Expected 6 default repos, got %d", len(config.Repos))
	}
}

func TestDefaultReposConfiguration(t *testing.T) {
	config := DefaultConfig()

	expectedRepos := map[string]struct {
		repoType string
		url      string
	}{
		"featured":   {repoType: "registry", url: "yeasy/awesome-agent-skills/registry/index.json"},
		"anthropics": {repoType: "dir", url: "anthropics/skills/skills"},
		"openai":     {repoType: "dir", url: "openai/skills/skills"},
		"composio":   {repoType: "dir", url: "ComposioHQ/awesome-claude-skills"},
		"vercel":     {repoType: "dir", url: "vercel-labs/agent-skills"},
		"openclaw":   {repoType: "dir", url: "openclaw/openclaw/skills"},
	}

	for _, repo := range config.Repos {
		expected, exists := expectedRepos[repo.Name]
		if !exists {
			t.Errorf("Unexpected repo in defaults: %s", repo.Name)
			continue
		}
		if repo.Type != expected.repoType {
			t.Errorf("Repo %s: expected type '%s', got '%s'", repo.Name, expected.repoType, repo.Type)
		}
		if repo.URL != expected.url {
			t.Errorf("Repo %s: expected URL '%s', got '%s'", repo.Name, expected.url, repo.URL)
		}
	}

	// Verify OptionalRepos
	if len(OptionalRepos) != 1 {
		t.Errorf("Expected 1 optional repo, got %d", len(OptionalRepos))
	}
	if OptionalRepos[0].Name != "community" {
		t.Errorf("Expected optional repo 'community', got '%s'", OptionalRepos[0].Name)
	}
}

func TestSaveAndLoadConfig(t *testing.T) {
	// Setup temporary file
	tmpFile := "test_ask.yaml"
	defer func() { _ = os.Remove(tmpFile) }()

	// Test Save
	config := DefaultConfig()
	config.Skills = append(config.Skills, "test-skill")

	// Temporarily redirect Save to write to tmpFile by modifying how we use Save
	// Since Save() uses correct "ask.yaml" hardcoded, we should mock or change current dir.
	// For simplicity in this env, we will change directory or just test logic if refactored.
	// Let's refactor Config.Save() in the future to take a path, but for now
	// we'll run this test in a temp dir.

	dir := t.TempDir()
	originalDir, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(originalDir) }()

	err := config.Save()
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Test Load
	loadedConfig, err := LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	if len(loadedConfig.Skills) != 1 || loadedConfig.Skills[0] != "test-skill" {
		t.Errorf("Config persistence failed. Expected [test-skill], got %v", loadedConfig.Skills)
	}
}

func TestAddSkill(t *testing.T) {
	config := DefaultConfig()
	config.AddSkill("skill-a")
	config.AddSkill("skill-b")
	config.AddSkill("skill-a") // Duplicate

	if len(config.Skills) != 2 {
		t.Errorf("AddSkill should handle duplicates. Expected 2 skills, got %d", len(config.Skills))
	}
}

func TestRemoveSkill(t *testing.T) {
	config := DefaultConfig()
	config.AddSkill("skill-a")
	config.AddSkill("skill-b")

	config.RemoveSkill("skill-a")
	if len(config.Skills) != 1 {
		t.Errorf("Expected 1 skill after removal, got %d", len(config.Skills))
	}
	if config.Skills[0] != "skill-b" {
		t.Errorf("Expected skill-b to remain, got %s", config.Skills[0])
	}

	config.RemoveSkill("non-existent")
	if len(config.Skills) != 1 {
		t.Errorf("Removing non-existent skill should not change list size")
	}
}

func TestRemoveSkillInfo(t *testing.T) {
	config := DefaultConfig()
	config.AddSkillInfo(SkillInfo{Name: "skill-a", Description: "Skill A"})
	config.AddSkillInfo(SkillInfo{Name: "skill-b", Description: "Skill B"})

	if len(config.SkillsInfo) != 2 {
		t.Errorf("Expected 2 skill infos, got %d", len(config.SkillsInfo))
	}

	config.RemoveSkillInfo("skill-a")
	if len(config.SkillsInfo) != 1 {
		t.Errorf("Expected 1 skill info after removal, got %d", len(config.SkillsInfo))
	}
	if config.SkillsInfo[0].Name != "skill-b" {
		t.Errorf("Expected skill-b to remain, got %s", config.SkillsInfo[0].Name)
	}

	config.RemoveSkillInfo("non-existent")
	if len(config.SkillsInfo) != 1 {
		t.Errorf("Removing non-existent skill info should not change list size")
	}
}

func TestDefaultToolTargets(t *testing.T) {
	targets := DefaultToolTargets()
	if len(targets) == 0 {
		t.Error("Expected default tool targets, got empty list")
	}
	// Verify "agent" target exists
	foundAgent := false
	for _, target := range targets {
		if target.Name == "agent" {
			foundAgent = true
			break
		}
	}
	if !foundAgent {
		t.Error("Expected 'agent' tool target")
	}
}

func TestGetAllSkillNames(t *testing.T) {
	config := DefaultConfig()

	// Empty config should return empty slice
	names := config.GetAllSkillNames()
	if len(names) != 0 {
		t.Errorf("Expected 0 skills, got %d", len(names))
	}

	// Add skills via legacy list
	config.Skills = []string{"skill-a", "skill-b"}
	names = config.GetAllSkillNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 skills, got %d", len(names))
	}

	// Add skill via SkillsInfo with one duplicate
	config.SkillsInfo = []SkillInfo{
		{Name: "skill-b", Description: "Duplicate"},
		{Name: "skill-c", Description: "New"},
	}
	names = config.GetAllSkillNames()
	if len(names) != 3 {
		t.Errorf("Expected 3 deduplicated skills, got %d: %v", len(names), names)
	}

	// Verify order: Skills first, then unique SkillsInfo
	expected := []string{"skill-a", "skill-b", "skill-c"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("Expected names[%d]=%s, got %s", i, expected[i], name)
		}
	}

	// Only SkillsInfo, no legacy Skills
	config.Skills = nil
	names = config.GetAllSkillNames()
	if len(names) != 2 {
		t.Errorf("Expected 2 skills from SkillsInfo only, got %d", len(names))
	}
}

func TestAtomicWriteFile_Basic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile.txt")
	data := []byte("hello atomic write")

	err := atomicWriteFile(path, data, 0644)
	if err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read written file: %v", err)
	}
	if string(got) != string(data) {
		t.Errorf("Expected content %q, got %q", string(data), string(got))
	}
}

func TestAtomicWriteFile_Permissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "permfile.txt")

	err := atomicWriteFile(path, []byte("perm test"), 0600)
	if err != nil {
		t.Fatalf("atomicWriteFile failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("Expected permissions 0600, got %o", info.Mode().Perm())
	}
}

func TestAtomicWriteFile_Overwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "overwrite.txt")

	// Write initial content
	err := atomicWriteFile(path, []byte("original"), 0644)
	if err != nil {
		t.Fatalf("First write failed: %v", err)
	}

	// Overwrite with new content
	err = atomicWriteFile(path, []byte("replaced"), 0644)
	if err != nil {
		t.Fatalf("Overwrite failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(got) != "replaced" {
		t.Errorf("Expected content %q, got %q", "replaced", string(got))
	}
}

func TestAtomicWriteFile_InvalidDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent", "file.txt")

	err := atomicWriteFile(path, []byte("data"), 0644)
	if err == nil {
		t.Fatal("Expected error for non-existent directory, got nil")
	}
}

func TestLoadConfigFromSymlink(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory to use as a symlink target.
	// The Lstat pre-check in loadConfigFromPath detects the symlink and
	// rejects it before opening the file.
	targetDir := filepath.Join(dir, "target-dir")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	// Create a symlink pointing to the directory
	symlinkPath := filepath.Join(dir, "symlink-config.yaml")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// loadConfigFromPath should reject this because the resolved target is not a regular file
	_, err := loadConfigFromPath(symlinkPath)
	if err == nil {
		t.Fatal("expected error when loading config from symlink to non-regular file, got nil")
	}
	if !strings.Contains(err.Error(), "non-regular file") {
		t.Errorf("expected error to contain 'non-regular file', got: %v", err)
	}
}

func TestLoadConfigFromSymlinkToRegularFile(t *testing.T) {
	dir := t.TempDir()

	// Create a regular file as the symlink target
	targetFile := filepath.Join(dir, "real-config.yaml")
	if err := os.WriteFile(targetFile, []byte("repos: []\n"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Create a symlink pointing to the regular file
	symlinkPath := filepath.Join(dir, "symlink-config.yaml")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// loadConfigFromPath should reject this because the Lstat detects a symlink
	_, err := loadConfigFromPath(symlinkPath)
	if err == nil {
		t.Fatal("expected error when loading config from symlink to regular file, got nil")
	}
	if !strings.Contains(err.Error(), "non-regular file") {
		t.Errorf("expected error to contain 'non-regular file', got: %v", err)
	}
}

func TestDetectExistingToolDirs(t *testing.T) {
	// Setup temp dir
	dir := t.TempDir()

	// Create .claude directory (mocking existing project)
	err := os.Mkdir(filepath.Join(dir, ".claude"), 0755)
	if err != nil {
		t.Fatalf("Failed to create .claude dir: %v", err)
	}

	detected := DetectExistingToolDirs(dir)
	foundClaude := false
	for _, target := range detected {
		if target.Name == "claude" {
			foundClaude = true
			break
		}
	}
	if !foundClaude {
		t.Error("Expected 'claude' to be detected in " + dir)
	}
}
