package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddEntry_New(t *testing.T) {
	lf := &LockFile{Version: 1, Skills: []LockEntry{}}

	now := time.Now()
	lf.AddEntry(LockEntry{
		Name:        "skill-a",
		Source:      "github",
		URL:         "https://github.com/owner/skill-a",
		Commit:      "abc123",
		Version:     "1.0.0",
		InstalledAt: now,
	})

	if len(lf.Skills) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(lf.Skills))
	}
	if lf.Skills[0].Name != "skill-a" {
		t.Fatalf("expected name skill-a, got %s", lf.Skills[0].Name)
	}
	if lf.Skills[0].Version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %s", lf.Skills[0].Version)
	}
}

func TestAddEntry_UpdateExisting(t *testing.T) {
	now := time.Now()
	lf := &LockFile{Version: 1, Skills: []LockEntry{
		{Name: "skill-a", URL: "https://github.com/owner/skill-a", Version: "1.0.0", InstalledAt: now},
	}}

	later := now.Add(time.Hour)
	lf.AddEntry(LockEntry{
		Name:        "skill-a",
		URL:         "https://github.com/owner/skill-a",
		Version:     "2.0.0",
		InstalledAt: later,
	})

	if len(lf.Skills) != 1 {
		t.Fatalf("expected 1 entry after update, got %d", len(lf.Skills))
	}
	if lf.Skills[0].Version != "2.0.0" {
		t.Fatalf("expected version 2.0.0 after update, got %s", lf.Skills[0].Version)
	}
	if !lf.Skills[0].InstalledAt.Equal(later) {
		t.Fatalf("expected InstalledAt to be updated")
	}
}

func TestRemoveEntry_Existing(t *testing.T) {
	now := time.Now()
	lf := &LockFile{Version: 1, Skills: []LockEntry{
		{Name: "skill-a", URL: "https://github.com/owner/skill-a", InstalledAt: now},
		{Name: "skill-b", URL: "https://github.com/owner/skill-b", InstalledAt: now},
	}}

	lf.RemoveEntry("skill-a")

	if len(lf.Skills) != 1 {
		t.Fatalf("expected 1 entry after removal, got %d", len(lf.Skills))
	}
	if lf.Skills[0].Name != "skill-b" {
		t.Fatalf("expected remaining entry to be skill-b, got %s", lf.Skills[0].Name)
	}
}

func TestRemoveEntry_Nonexistent(t *testing.T) {
	now := time.Now()
	lf := &LockFile{Version: 1, Skills: []LockEntry{
		{Name: "skill-a", URL: "https://github.com/owner/skill-a", InstalledAt: now},
	}}

	// Should not panic or change anything
	lf.RemoveEntry("nonexistent")

	if len(lf.Skills) != 1 {
		t.Fatalf("expected 1 entry unchanged, got %d", len(lf.Skills))
	}
}

func TestRemoveEntry_EmptyList(t *testing.T) {
	lf := &LockFile{Version: 1, Skills: []LockEntry{}}

	// Should not panic on empty list
	lf.RemoveEntry("anything")

	if len(lf.Skills) != 0 {
		t.Fatalf("expected 0 entries, got %d", len(lf.Skills))
	}
}

func TestGetEntry_Existing(t *testing.T) {
	now := time.Now()
	lf := &LockFile{Version: 1, Skills: []LockEntry{
		{Name: "skill-a", URL: "https://github.com/owner/skill-a", Version: "1.0.0", InstalledAt: now},
		{Name: "skill-b", URL: "https://github.com/owner/skill-b", Version: "2.0.0", InstalledAt: now},
	}}

	entry := lf.GetEntry("skill-b")
	if entry == nil {
		t.Fatal("expected non-nil entry for skill-b")
	}
	if entry.Version != "2.0.0" {
		t.Fatalf("expected version 2.0.0, got %s", entry.Version)
	}
}

func TestGetEntry_Nonexistent(t *testing.T) {
	lf := &LockFile{Version: 1, Skills: []LockEntry{
		{Name: "skill-a", URL: "https://github.com/owner/skill-a", InstalledAt: time.Now()},
	}}

	entry := lf.GetEntry("nonexistent")
	if entry != nil {
		t.Fatalf("expected nil for nonexistent entry, got %+v", entry)
	}
}

func TestGetEntry_ReturnsMutablePointer(t *testing.T) {
	now := time.Now()
	lf := &LockFile{Version: 1, Skills: []LockEntry{
		{Name: "skill-a", URL: "https://github.com/owner/skill-a", Version: "1.0.0", InstalledAt: now},
	}}

	entry := lf.GetEntry("skill-a")
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	// Mutating through the pointer should affect the original
	entry.Version = "9.9.9"
	if lf.Skills[0].Version != "9.9.9" {
		t.Fatalf("expected mutation through pointer to affect original, got %s", lf.Skills[0].Version)
	}
}

func TestLoadLockFromSymlink(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory to use as a symlink target.
	// The Lstat pre-check in loadLockFromPath detects the symlink and
	// rejects it before opening the file.
	targetDir := filepath.Join(dir, "target-dir")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatalf("Failed to create target directory: %v", err)
	}

	// Create a symlink pointing to the directory
	symlinkPath := filepath.Join(dir, "symlink-lock.yaml")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// loadLockFromPath should reject this because the resolved target is not a regular file
	_, err := loadLockFromPath(symlinkPath)
	if err == nil {
		t.Fatal("expected error when loading lock from symlink to non-regular file, got nil")
	}
	if !strings.Contains(err.Error(), "non-regular file") {
		t.Errorf("expected error to contain 'non-regular file', got: %v", err)
	}
}

func TestLoadLockFromSymlinkToRegularFile(t *testing.T) {
	dir := t.TempDir()

	// Create a regular file as the symlink target
	targetFile := filepath.Join(dir, "real-lock.yaml")
	if err := os.WriteFile(targetFile, []byte("version: 1\nskills: []\n"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Create a symlink pointing to the regular file
	symlinkPath := filepath.Join(dir, "symlink-lock.yaml")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// loadLockFromPath should reject this because the Lstat detects a symlink
	_, err := loadLockFromPath(symlinkPath)
	if err == nil {
		t.Fatal("expected error when loading lock from symlink to regular file, got nil")
	}
	if !strings.Contains(err.Error(), "non-regular file") {
		t.Errorf("expected error to contain 'non-regular file', got: %v", err)
	}
}

func TestLoadAndSaveRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("warning: could not chdir back: %v", err)
		}
	}()

	now := time.Now().Truncate(time.Second).UTC()
	original := &LockFile{
		Version: 1,
		Skills: []LockEntry{
			{
				Name:        "skill-alpha",
				Source:      "github",
				URL:         "https://github.com/owner/skill-alpha",
				Commit:      "deadbeef",
				Version:     "1.2.3",
				InstalledAt: now,
			},
			{
				Name:        "skill-beta",
				Source:      "registry",
				URL:         "https://github.com/owner/skill-beta",
				Version:     "0.1.0",
				InstalledAt: now.Add(-24 * time.Hour),
			},
		},
	}

	if err := original.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify the file exists on disk
	if _, err := os.Stat(LockFileName); err != nil {
		t.Fatalf("lock file not found on disk: %v", err)
	}

	loaded, err := LoadLockFile()
	if err != nil {
		t.Fatalf("LoadLockFile failed: %v", err)
	}

	if loaded.Version != original.Version {
		t.Fatalf("version mismatch: got %d, want %d", loaded.Version, original.Version)
	}
	if len(loaded.Skills) != len(original.Skills) {
		t.Fatalf("skills count mismatch: got %d, want %d", len(loaded.Skills), len(original.Skills))
	}

	for i, orig := range original.Skills {
		got := loaded.Skills[i]
		if got.Name != orig.Name {
			t.Fatalf("skill[%d] name: got %s, want %s", i, got.Name, orig.Name)
		}
		if got.Source != orig.Source {
			t.Fatalf("skill[%d] source: got %s, want %s", i, got.Source, orig.Source)
		}
		if got.URL != orig.URL {
			t.Fatalf("skill[%d] url: got %s, want %s", i, got.URL, orig.URL)
		}
		if got.Commit != orig.Commit {
			t.Fatalf("skill[%d] commit: got %s, want %s", i, got.Commit, orig.Commit)
		}
		if got.Version != orig.Version {
			t.Fatalf("skill[%d] version: got %s, want %s", i, got.Version, orig.Version)
		}
		if !got.InstalledAt.Equal(orig.InstalledAt) {
			t.Fatalf("skill[%d] installed_at: got %v, want %v", i, got.InstalledAt, orig.InstalledAt)
		}
	}
}

func TestLoadLockFile_Nonexistent(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("warning: could not chdir back: %v", err)
		}
	}()

	lf, err := LoadLockFile()
	if err != nil {
		t.Fatalf("expected no error for nonexistent file, got %v", err)
	}
	if lf.Version != 1 {
		t.Fatalf("expected Version=1, got %d", lf.Version)
	}
	if len(lf.Skills) != 0 {
		t.Fatalf("expected empty skills, got %d", len(lf.Skills))
	}
}

func TestLoadLockFile_MalformedYAML(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("warning: could not chdir back: %v", err)
		}
	}()

	malformed := []byte("version: [invalid\nskills:\n  - name: {{bad")
	if err := os.WriteFile(LockFileName, malformed, 0600); err != nil {
		t.Fatal(err)
	}

	_, err = LoadLockFile()
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
}

func TestLoadLockFileByScope_Local(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("warning: could not chdir back: %v", err)
		}
	}()

	// Save a local lock file
	lf := &LockFile{
		Version: 1,
		Skills: []LockEntry{
			{Name: "local-skill", URL: "https://github.com/owner/local-skill", InstalledAt: time.Now().Truncate(time.Second).UTC()},
		},
	}
	if err := lf.Save(); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadLockFileByScope(false)
	if err != nil {
		t.Fatalf("LoadLockFileByScope(false) failed: %v", err)
	}
	if len(loaded.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(loaded.Skills))
	}
	if loaded.Skills[0].Name != "local-skill" {
		t.Fatalf("expected local-skill, got %s", loaded.Skills[0].Name)
	}
}

func TestLoadLockFileByScope_Global(t *testing.T) {
	// Override HOME so global operations use a temp directory
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// LoadLockFileByScope(true) should return empty lock file when no global file exists
	loaded, err := LoadLockFileByScope(true)
	if err != nil {
		t.Fatalf("LoadLockFileByScope(true) failed: %v", err)
	}
	if loaded.Version != 1 {
		t.Fatalf("expected Version=1, got %d", loaded.Version)
	}
	if len(loaded.Skills) != 0 {
		t.Fatalf("expected 0 skills, got %d", len(loaded.Skills))
	}
}

func TestSaveByScope_Local(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Chdir(origDir); err != nil {
			t.Logf("warning: could not chdir back: %v", err)
		}
	}()

	lf := &LockFile{
		Version: 1,
		Skills: []LockEntry{
			{Name: "local-save", URL: "https://github.com/owner/local-save", InstalledAt: time.Now().Truncate(time.Second).UTC()},
		},
	}

	if err := lf.SaveByScope(false); err != nil {
		t.Fatalf("SaveByScope(false) failed: %v", err)
	}

	// Verify file was written locally
	if _, err := os.Stat(filepath.Join(tmpDir, LockFileName)); err != nil {
		t.Fatalf("local lock file not found: %v", err)
	}

	// Load it back
	loaded, err := LoadLockFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0].Name != "local-save" {
		t.Fatalf("round-trip through SaveByScope(false) failed: %+v", loaded.Skills)
	}
}

func TestSaveByScope_Global(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	lf := &LockFile{
		Version: 1,
		Skills: []LockEntry{
			{Name: "global-save", URL: "https://github.com/owner/global-save", InstalledAt: time.Now().Truncate(time.Second).UTC()},
		},
	}

	if err := lf.SaveByScope(true); err != nil {
		t.Fatalf("SaveByScope(true) failed: %v", err)
	}

	// Verify the global lock file exists
	globalPath := GetGlobalLockPath()
	if _, err := os.Stat(globalPath); err != nil {
		t.Fatalf("global lock file not found at %s: %v", globalPath, err)
	}

	// Load it back via global path
	loaded, err := LoadGlobalLockFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Skills) != 1 || loaded.Skills[0].Name != "global-save" {
		t.Fatalf("round-trip through SaveByScope(true) failed: %+v", loaded.Skills)
	}
}
