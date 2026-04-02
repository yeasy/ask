package cache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSanitizeRepoName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"forward slash replaced", "owner/repo", "owner-repo"},
		{"backslash replaced", "path\\backslash", "path-backslash"},
		{"dot-dot replaced", "../traversal", "_-traversal"},
		{"normal name unchanged", "normal-name", "normal-name"},
		{"deep traversal sanitized", "../../etc/passwd", "_-_-etc-passwd"},
		{"simple name unchanged", "simple", "simple"},
		{"multiple slashes", "a/b/c", "a-b-c"},
		{"double dot only", "..", "_"},
		{"triple dot", "...", "_."},
		{"empty string", "", "_"},
		{"single dot", ".", "_"},
		{"mixed separators", "a/b\\c/../d", "a-b-c-_-d"},
		{"leading dot-dot-slash", "../../../root", "_-_-_-root"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeRepoName(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeRepoName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractDescription(t *testing.T) {
	t.Run("valid frontmatter with description", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "SKILL.md")
		content := "---\ntitle: My Skill\ndescription: \"test desc\"\n---\n# Body\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := extractDescription(path)
		if got != "test desc" {
			t.Errorf("extractDescription() = %q, want %q", got, "test desc")
		}
	})

	t.Run("frontmatter without description field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "SKILL.md")
		content := "---\ntitle: My Skill\nauthor: someone\n---\n# Body\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := extractDescription(path)
		if got != "" {
			t.Errorf("extractDescription() = %q, want empty string", got)
		}
	})

	t.Run("no frontmatter", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "SKILL.md")
		content := "# Just a heading\nSome content\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := extractDescription(path)
		if got != "" {
			t.Errorf("extractDescription() = %q, want empty string", got)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		got := extractDescription("/nonexistent/path/SKILL.md")
		if got != "" {
			t.Errorf("extractDescription() = %q, want empty string", got)
		}
	})

	t.Run("double quoted description stripped", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "SKILL.md")
		content := "---\ndescription: \"hello world\"\n---\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := extractDescription(path)
		if got != "hello world" {
			t.Errorf("extractDescription() = %q, want %q", got, "hello world")
		}
	})

	t.Run("single quoted description stripped", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "SKILL.md")
		content := "---\ndescription: 'single quoted'\n---\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := extractDescription(path)
		if got != "single quoted" {
			t.Errorf("extractDescription() = %q, want %q", got, "single quoted")
		}
	})

	t.Run("file larger than maxDescriptionFileSize", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "SKILL.md")
		// Create a file larger than 8192 bytes
		content := "---\ndescription: \"should not be read\"\n---\n" + strings.Repeat("x", maxDescriptionFileSize+1)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		got := extractDescription(path)
		if got != "" {
			t.Errorf("extractDescription() = %q, want empty string for oversized file", got)
		}
	})
}

func TestGetCachedRepos(t *testing.T) {
	t.Run("lists subdirectories", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "repo-a"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "repo-b"), 0755); err != nil {
			t.Fatal(err)
		}

		c := &ReposCache{baseDir: dir}
		repos := c.GetCachedRepos()

		if len(repos) != 2 {
			t.Fatalf("expected 2 repos, got %d", len(repos))
		}
		found := map[string]bool{}
		for _, r := range repos {
			found[r] = true
		}
		if !found["repo-a"] || !found["repo-b"] {
			t.Errorf("expected repo-a and repo-b, got %v", repos)
		}
	})

	t.Run("excludes .git directory", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "real-repo"), 0755); err != nil {
			t.Fatal(err)
		}

		c := &ReposCache{baseDir: dir}
		repos := c.GetCachedRepos()

		if len(repos) != 1 {
			t.Fatalf("expected 1 repo, got %d: %v", len(repos), repos)
		}
		if repos[0] != "real-repo" {
			t.Errorf("expected real-repo, got %q", repos[0])
		}
	})

	t.Run("excludes regular files", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte("{}"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "my-repo"), 0755); err != nil {
			t.Fatal(err)
		}

		c := &ReposCache{baseDir: dir}
		repos := c.GetCachedRepos()

		if len(repos) != 1 {
			t.Fatalf("expected 1 repo, got %d: %v", len(repos), repos)
		}
		if repos[0] != "my-repo" {
			t.Errorf("expected my-repo, got %q", repos[0])
		}
	})

	t.Run("empty directory returns empty slice", func(t *testing.T) {
		dir := t.TempDir()
		c := &ReposCache{baseDir: dir}
		repos := c.GetCachedRepos()

		if len(repos) != 0 {
			t.Errorf("expected 0 repos, got %d: %v", len(repos), repos)
		}
	})
}

func TestHasRepo(t *testing.T) {
	t.Run("repo exists", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "owner-repo"), 0755); err != nil {
			t.Fatal(err)
		}

		c := &ReposCache{baseDir: dir}
		if !c.HasRepo("owner/repo") {
			t.Error("HasRepo() = false, want true for existing repo")
		}
	})

	t.Run("repo does not exist", func(t *testing.T) {
		dir := t.TempDir()
		c := &ReposCache{baseDir: dir}
		if c.HasRepo("nonexistent/repo") {
			t.Error("HasRepo() = true, want false for nonexistent repo")
		}
	})
}

func TestSaveAndLoadIndex(t *testing.T) {
	t.Run("round trip with stars and urls", func(t *testing.T) {
		dir := t.TempDir()
		// Create repo directories so SaveIndexWithStars can stat them
		if err := os.MkdirAll(filepath.Join(dir, "alpha"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "beta"), 0755); err != nil {
			t.Fatal(err)
		}

		c := &ReposCache{baseDir: dir}

		stars := map[string]int{"alpha": 42, "beta": 100}
		urls := map[string]string{"alpha": "https://github.com/a/alpha", "beta": "https://github.com/b/beta"}

		err := c.SaveIndexWithStars(stars, urls)
		if err != nil {
			t.Fatalf("SaveIndexWithStars() error: %v", err)
		}

		infos, err := c.LoadIndex()
		if err != nil {
			t.Fatalf("LoadIndex() error: %v", err)
		}

		if len(infos) != 2 {
			t.Fatalf("expected 2 repo infos, got %d", len(infos))
		}

		infoMap := map[string]RepoInfo{}
		for _, info := range infos {
			infoMap[info.Name] = info
		}

		if infoMap["alpha"].Stars != 42 {
			t.Errorf("alpha stars = %d, want 42", infoMap["alpha"].Stars)
		}
		if infoMap["beta"].Stars != 100 {
			t.Errorf("beta stars = %d, want 100", infoMap["beta"].Stars)
		}
		if infoMap["alpha"].URL != "https://github.com/a/alpha" {
			t.Errorf("alpha URL = %q, want %q", infoMap["alpha"].URL, "https://github.com/a/alpha")
		}
		if infoMap["beta"].URL != "https://github.com/b/beta" {
			t.Errorf("beta URL = %q, want %q", infoMap["beta"].URL, "https://github.com/b/beta")
		}
		if infoMap["alpha"].LocalPath != filepath.Join(dir, "alpha") {
			t.Errorf("alpha LocalPath = %q, want %q", infoMap["alpha"].LocalPath, filepath.Join(dir, "alpha"))
		}
	})
}

func TestCloneOrPull_RejectsHTTP(t *testing.T) {
	rc := &ReposCache{baseDir: t.TempDir()}
	err := rc.CloneOrPull(context.Background(), "http://github.com/owner/repo", "test-repo")
	if err == nil {
		t.Fatal("expected error for http:// URL")
	}
	if !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("error should mention HTTPS, got: %v", err)
	}
}

func TestSearchSkills(t *testing.T) {
	// Helper to set up a repo with skills
	setupRepo := func(t *testing.T, baseDir, repoName string, skills map[string]string) {
		t.Helper()
		repoDir := filepath.Join(baseDir, repoName)
		for skillName, desc := range skills {
			skillDir := filepath.Join(repoDir, skillName)
			if err := os.MkdirAll(skillDir, 0755); err != nil {
				t.Fatal(err)
			}
			content := fmt.Sprintf("---\ndescription: \"%s\"\n---\n# %s\n", desc, skillName)
			if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}

	t.Run("matches skill name case-insensitively", func(t *testing.T) {
		dir := t.TempDir()
		setupRepo(t, dir, "my-repo", map[string]string{
			"Docker-Deploy": "Deploy containers",
			"go-lint":       "Lint Go code",
		})

		c := &ReposCache{baseDir: dir}
		results, err := c.SearchSkills("docker")
		if err != nil {
			t.Fatalf("SearchSkills() error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d: %v", len(results), results)
		}
		if results[0].Name != "Docker-Deploy" {
			t.Errorf("expected Docker-Deploy, got %q", results[0].Name)
		}
	})

	t.Run("matches skill description", func(t *testing.T) {
		dir := t.TempDir()
		setupRepo(t, dir, "tools", map[string]string{
			"formatter":  "Format Go and Python code",
			"test-suite": "Run unit tests",
		})

		c := &ReposCache{baseDir: dir}
		results, err := c.SearchSkills("python")
		if err != nil {
			t.Fatalf("SearchSkills() error: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		if results[0].Name != "formatter" {
			t.Errorf("expected formatter, got %q", results[0].Name)
		}
	})

	t.Run("returns empty for no matches", func(t *testing.T) {
		dir := t.TempDir()
		setupRepo(t, dir, "repo", map[string]string{
			"deploy": "Deploy to production",
		})

		c := &ReposCache{baseDir: dir}
		results, err := c.SearchSkills("zzz-nonexistent")
		if err != nil {
			t.Fatalf("SearchSkills() error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})

	t.Run("searches across multiple repos", func(t *testing.T) {
		dir := t.TempDir()
		setupRepo(t, dir, "repo-a", map[string]string{
			"deploy-aws": "Deploy to AWS",
		})
		setupRepo(t, dir, "repo-b", map[string]string{
			"deploy-gcp": "Deploy to GCP",
			"monitor":    "Monitor services",
		})

		c := &ReposCache{baseDir: dir}
		results, err := c.SearchSkills("deploy")
		if err != nil {
			t.Fatalf("SearchSkills() error: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("expected 2 results, got %d", len(results))
		}
	})

	t.Run("empty base dir returns empty results", func(t *testing.T) {
		dir := t.TempDir()
		c := &ReposCache{baseDir: dir}
		results, err := c.SearchSkills("anything")
		if err != nil {
			t.Fatalf("SearchSkills() error: %v", err)
		}
		if len(results) != 0 {
			t.Errorf("expected 0 results, got %d", len(results))
		}
	})
}

func TestIsStale(t *testing.T) {
	t.Run("returns true when no index exists", func(t *testing.T) {
		dir := t.TempDir()
		c := &ReposCache{baseDir: dir}
		if !c.IsStale("some-repo", time.Hour) {
			t.Error("IsStale() = false, want true when index is missing")
		}
	})

	t.Run("returns true when repo not in index", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "other-repo"), 0755); err != nil {
			t.Fatal(err)
		}
		c := &ReposCache{baseDir: dir}
		// Save index with one repo
		stars := map[string]int{"other-repo": 10}
		urls := map[string]string{"other-repo": "https://github.com/x/y"}
		_ = c.SaveIndexWithStars(stars, urls)

		if !c.IsStale("missing-repo", time.Hour) {
			t.Error("IsStale() = false, want true for repo not in index")
		}
	})

	t.Run("returns false when recently synced", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "fresh-repo"), 0755); err != nil {
			t.Fatal(err)
		}
		c := &ReposCache{baseDir: dir}
		stars := map[string]int{"fresh-repo": 5}
		urls := map[string]string{"fresh-repo": "https://github.com/a/b"}
		_ = c.SaveIndexWithStars(stars, urls)

		// Just saved, so LastSyncedAt is ~now; 1 hour TTL should not be stale
		if c.IsStale("fresh-repo", time.Hour) {
			t.Error("IsStale() = true, want false for recently synced repo")
		}
	})

	t.Run("returns true when TTL exceeded", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.MkdirAll(filepath.Join(dir, "old-repo"), 0755); err != nil {
			t.Fatal(err)
		}
		c := &ReposCache{baseDir: dir}
		stars := map[string]int{"old-repo": 5}
		urls := map[string]string{"old-repo": "https://github.com/a/b"}
		_ = c.SaveIndexWithStars(stars, urls)

		// Use a TTL of 0 so it's immediately stale
		if !c.IsStale("old-repo", 0) {
			t.Error("IsStale() = false, want true when TTL is 0")
		}
	})
}

func TestListSkills(t *testing.T) {
	t.Run("returns error for non-cached repo", func(t *testing.T) {
		dir := t.TempDir()
		c := &ReposCache{baseDir: dir}
		_, err := c.ListSkills("nonexistent")
		if err == nil {
			t.Fatal("expected error for non-cached repo")
		}
		if !strings.Contains(err.Error(), "not cached") {
			t.Errorf("expected 'not cached' in error, got: %v", err)
		}
	})

	t.Run("finds SKILL.md files and skips .git", func(t *testing.T) {
		dir := t.TempDir()
		repoDir := filepath.Join(dir, "test-repo")

		// Create a skill
		skillDir := filepath.Join(repoDir, "my-skill")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\ndescription: \"A skill\"\n---\n"), 0644); err != nil {
			t.Fatal(err)
		}

		// Create a .git directory with a SKILL.md (should be skipped)
		gitSkill := filepath.Join(repoDir, ".git", "hooks")
		if err := os.MkdirAll(gitSkill, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitSkill, "SKILL.md"), []byte("---\ndescription: \"hidden\"\n---\n"), 0644); err != nil {
			t.Fatal(err)
		}

		c := &ReposCache{baseDir: dir}
		skills, err := c.ListSkills("test-repo")
		if err != nil {
			t.Fatalf("ListSkills() error: %v", err)
		}
		if len(skills) != 1 {
			t.Fatalf("expected 1 skill, got %d", len(skills))
		}
		if skills[0].Name != "my-skill" {
			t.Errorf("expected skill name 'my-skill', got %q", skills[0].Name)
		}
		if skills[0].Description != "A skill" {
			t.Errorf("expected description 'A skill', got %q", skills[0].Description)
		}
		if skills[0].RepoName != "test-repo" {
			t.Errorf("expected repo name 'test-repo', got %q", skills[0].RepoName)
		}
	})
}

func TestSaveIndexPreservesExistingData(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "repo-a"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "repo-b"), 0755); err != nil {
		t.Fatal(err)
	}

	c := &ReposCache{baseDir: dir}

	// First save: both repos synced
	stars1 := map[string]int{"repo-a": 50, "repo-b": 30}
	urls1 := map[string]string{
		"repo-a": "https://github.com/x/a",
		"repo-b": "https://github.com/x/b",
	}
	if err := c.SaveIndexWithStars(stars1, urls1); err != nil {
		t.Fatalf("first SaveIndexWithStars() error: %v", err)
	}

	// Second save: only repo-a synced (repo-b should preserve old data)
	stars2 := map[string]int{"repo-a": 55}
	urls2 := map[string]string{"repo-a": "https://github.com/x/a"}
	if err := c.SaveIndexWithStars(stars2, urls2); err != nil {
		t.Fatalf("second SaveIndexWithStars() error: %v", err)
	}

	infos, err := c.LoadIndex()
	if err != nil {
		t.Fatalf("LoadIndex() error: %v", err)
	}

	infoMap := map[string]RepoInfo{}
	for _, info := range infos {
		infoMap[info.Name] = info
	}

	// repo-a should have updated stars
	if infoMap["repo-a"].Stars != 55 {
		t.Errorf("repo-a stars = %d, want 55", infoMap["repo-a"].Stars)
	}
	// repo-b should preserve old stars
	if infoMap["repo-b"].Stars != 30 {
		t.Errorf("repo-b stars = %d, want 30 (preserved from first save)", infoMap["repo-b"].Stars)
	}
	// repo-b should preserve old URL
	if infoMap["repo-b"].URL != "https://github.com/x/b" {
		t.Errorf("repo-b URL = %q, want preserved URL", infoMap["repo-b"].URL)
	}
}

func TestLoadIndex_ErrorCases(t *testing.T) {
	t.Run("missing index file", func(t *testing.T) {
		dir := t.TempDir()
		c := &ReposCache{baseDir: dir}
		_, err := c.LoadIndex()
		if err == nil {
			t.Fatal("expected error for missing index.json")
		}
	})

	t.Run("corrupt json", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte("{not valid json!!!"), 0644); err != nil {
			t.Fatal(err)
		}
		c := &ReposCache{baseDir: dir}
		_, err := c.LoadIndex()
		if err == nil {
			t.Fatal("expected error for corrupt index.json")
		}
	})
}

func TestLoadIndex_Symlink(t *testing.T) {
	dir := t.TempDir()

	// Create a subdirectory to use as a symlink target.
	// The Lstat pre-check in LoadIndex detects the symlink and
	// rejects it before opening the file.
	targetDir := filepath.Join(dir, "target-dir")
	if err := os.Mkdir(targetDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a symlink at the expected index.json path pointing to the directory
	symlinkPath := filepath.Join(dir, "index.json")
	if err := os.Symlink(targetDir, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	c := &ReposCache{baseDir: dir}
	_, err := c.LoadIndex()
	if err == nil {
		t.Fatal("expected error when loading index from symlink to non-regular file, got nil")
	}
	if !strings.Contains(err.Error(), "non-regular file") {
		t.Errorf("expected error to contain 'non-regular file', got: %v", err)
	}
}

func TestLoadIndex_SymlinkToRegularFile(t *testing.T) {
	dir := t.TempDir()

	// Create a regular file as the symlink target
	targetFile := filepath.Join(dir, "secret.json")
	if err := os.WriteFile(targetFile, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink at the expected index.json path pointing to the regular file
	symlinkPath := filepath.Join(dir, "index.json")
	if err := os.Symlink(targetFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	c := &ReposCache{baseDir: dir}
	_, err := c.LoadIndex()
	if err == nil {
		t.Fatal("expected error when loading index from symlink to regular file, got nil")
	}
	if !strings.Contains(err.Error(), "non-regular file") {
		t.Errorf("expected error to contain 'non-regular file', got: %v", err)
	}
}

func TestCloneOrPull_RejectsNonHTTP(t *testing.T) {
	rc := &ReposCache{baseDir: t.TempDir()}
	tests := []string{
		"git://github.com/owner/repo",
		"ssh://git@github.com/owner/repo",
		"/local/path",
		"ftp://example.com/repo",
	}
	for _, url := range tests {
		err := rc.CloneOrPull(context.Background(), url, "test-repo")
		if err == nil {
			t.Errorf("expected error for URL %q", url)
		}
	}
}
