package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeasy/ask/internal/filesystem"
)

// runGit executes a git command in the specified directory with a clean environment
func runGit(t *testing.T, dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	// Filter out GIT_ vars from environment to prevent contamination from parent repo
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GIT_") {
			env = append(env, e)
		}
	}
	cmd.Env = env

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
	}
}

// setupTestRepo creates a temp dir, initializes git, and configures a user
func setupTestRepo(t *testing.T) string {
	tmpDir := t.TempDir()
	runGit(t, tmpDir, "init")
	runGit(t, tmpDir, "config", "user.email", "test@example.com")
	runGit(t, tmpDir, "config", "user.name", "Test User")
	runGit(t, tmpDir, "config", "commit.gpgsign", "false")
	return tmpDir
}

// TestGetLatestTag tests the GetLatestTag function
// TestGetCurrentCommit tests the GetCurrentCommit function
func TestGetCurrentCommit(t *testing.T) {
	tmpDir := setupTestRepo(t)

	// Create and commit a file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	runGit(t, tmpDir, "add", "test.txt")
	runGit(t, tmpDir, "commit", "-m", "Test commit")

	// Test GetCurrentCommit
	commit, err := GetCurrentCommit(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("GetCurrentCommit failed: %v", err)
	}

	// Verify it's a valid SHA (40 characters)
	if len(commit) != 40 {
		t.Errorf("Expected 40 character SHA, got %d characters: %s", len(commit), commit)
	}
}

// TestCheckout tests the Checkout function
func TestCheckout(t *testing.T) {
	tmpDir := setupTestRepo(t)

	// Create initial commit on main
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("main"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create initial commit
	runGit(t, tmpDir, "add", "test.txt")
	runGit(t, tmpDir, "commit", "-m", "Main commit")

	// Create a branch
	runGit(t, tmpDir, "branch", "test-branch")

	// Test checkout
	err := Checkout(context.Background(), tmpDir, "test-branch")
	if err != nil {
		t.Errorf("Checkout failed: %v", err)
	}

	// Verify we're on the branch
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = tmpDir
	// Sanitize env here too for verification
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GIT_") {
			env = append(env, e)
		}
	}
	cmd.Env = env

	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("Failed to verify branch: %v", err)
	}

	currentBranch := string(output)
	if currentBranch != "test-branch\n" {
		t.Errorf("Expected to be on 'test-branch', got '%s'", currentBranch)
	}
}

// TestCopyDir tests the copyDir function
func TestCopyDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source directory with files
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	// Create test files
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}

	subDir := filepath.Join(srcDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(subDir, "file2.txt"), []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Copy directory
	dstDir := filepath.Join(tmpDir, "dst")
	if err := filesystem.CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	// Verify files were copied
	if _, err := os.Stat(filepath.Join(dstDir, "file1.txt")); os.IsNotExist(err) {
		t.Error("file1.txt was not copied")
	}

	if _, err := os.Stat(filepath.Join(dstDir, "subdir", "file2.txt")); os.IsNotExist(err) {
		t.Error("subdir/file2.txt was not copied")
	}

	// Verify content
	content, err := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}
	if string(content) != "content1" {
		t.Errorf("Expected content 'content1', got '%s'", string(content))
	}
}

// TestCopyFile tests the copyFile function
func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	testContent := "test file content"
	if err := os.WriteFile(srcFile, []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Copy file
	if err := filesystem.CopyFile(srcFile, dstFile); err != nil {
		t.Fatalf("CopyFile failed: %v", err)
	}

	// Verify destination file exists
	if _, err := os.Stat(dstFile); os.IsNotExist(err) {
		t.Error("Destination file was not created")
	}

	// Verify content
	content, err := os.ReadFile(dstFile)
	if err != nil {
		t.Fatalf("Failed to read destination file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Expected content '%s', got '%s'", testContent, string(content))
	}
}

// TestClone_RejectsNonHTTPS verifies Clone rejects non-HTTPS URLs
func TestClone_RejectsNonHTTPS(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"http://github.com/owner/repo", true},
		{"git://github.com/owner/repo", true},
		{"ssh://git@github.com/owner/repo", true},
		{"/local/path", true},
		{"", true},
	}
	for _, tt := range tests {
		err := Clone(context.Background(), tt.url, t.TempDir())
		if (err != nil) != tt.wantErr {
			t.Errorf("Clone(%q) err=%v, wantErr=%v", tt.url, err, tt.wantErr)
		}
	}
}

// TestSparseClone_RejectsNonHTTPS verifies SparseClone rejects non-HTTPS URLs
func TestSparseClone_RejectsNonHTTPS(t *testing.T) {
	err := SparseClone(context.Background(), "http://github.com/owner/repo", "main", "subdir", t.TempDir())
	if err == nil {
		t.Fatal("expected error for http:// URL in SparseClone")
	}
	if !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("error should mention HTTPS, got: %v", err)
	}
}

// TestClone_HTTPSRequired verifies that Clone rejects every non-HTTPS scheme
func TestClone_HTTPSRequired(t *testing.T) {
	schemes := []string{
		"http://github.com/owner/repo",
		"git://github.com/owner/repo.git",
		"ssh://git@github.com/owner/repo",
		"ftp://example.com/repo",
		"file:///tmp/repo",
		"/local/path/repo",
		"",
	}
	for _, url := range schemes {
		err := Clone(context.Background(), url, t.TempDir())
		if err == nil {
			t.Errorf("Clone(%q) should have been rejected", url)
		}
		if err != nil && !strings.Contains(err.Error(), "HTTPS") {
			t.Errorf("Clone(%q) error should mention HTTPS, got: %v", url, err)
		}
	}
}

// TestSparseClone_PathTraversalRejected verifies that ".." in subdirectory paths is rejected
func TestSparseClone_PathTraversalRejected(t *testing.T) {
	traversals := []string{
		"..",
		"../etc/passwd",
		"subdir/../../etc",
		"foo/../../../bar",
	}
	for _, sub := range traversals {
		err := SparseClone(context.Background(), "https://github.com/owner/repo", "main", sub, t.TempDir())
		if err == nil {
			t.Errorf("SparseClone with subDir=%q should have been rejected", sub)
		}
		if err != nil && !strings.Contains(err.Error(), "path traversal") {
			t.Errorf("SparseClone with subDir=%q error should mention path traversal, got: %v", sub, err)
		}
	}
}

// TestSparseClone_AbsolutePathRejected verifies that absolute path subdirectories are rejected
func TestSparseClone_AbsolutePathRejected(t *testing.T) {
	absPaths := []string{
		"/etc/passwd",
		"/tmp/evil",
		"/usr/local/bin",
	}
	for _, sub := range absPaths {
		err := SparseClone(context.Background(), "https://github.com/owner/repo", "main", sub, t.TempDir())
		if err == nil {
			t.Errorf("SparseClone with absolute subDir=%q should have been rejected", sub)
		}
		if err != nil && !strings.Contains(err.Error(), "path traversal") {
			t.Errorf("SparseClone with absolute subDir=%q error should mention path traversal, got: %v", sub, err)
		}
	}
}

// TestValidateGitRef_AllowsValid verifies that well-formed git refs pass validation
func TestValidateGitRef_AllowsValid(t *testing.T) {
	validRefs := []string{
		"main",
		"v1.0.0",
		"feature/my-branch",
		"release-2.3.4",
		"abc123def",
		"refs/heads/main",
		"v0.0.1-alpha",
	}
	for _, ref := range validRefs {
		if err := validateGitRef(ref); err != nil {
			t.Errorf("validateGitRef(%q) should pass but got error: %v", ref, err)
		}
	}
}

// TestValidateGitRef_RejectsDangerous verifies that refs with option-injection patterns are rejected
func TestValidateGitRef_RejectsDangerous(t *testing.T) {
	dangerous := []string{
		"--upload-pack",
		"--upload-pack=evil",
		"-c",
		"--exec=malicious",
		"--config=http.proxy=http://evil",
	}
	for _, ref := range dangerous {
		if err := validateGitRef(ref); err == nil {
			t.Errorf("validateGitRef(%q) should reject dash-prefixed ref", ref)
		}
	}
}

// TestValidateGitRef tests the validateGitRef security boundary
func TestValidateGitRef(t *testing.T) {
	tests := []struct {
		ref     string
		wantErr bool
	}{
		{"main", false},
		{"v1.0.0", false},
		{"feature/branch-name", false},
		{"abc123def456", false},
		{"refs/heads/main", false},
		{"", true},
		{"main..dev", true},
		{"ref name", true},
		{"-flag", true},
		{"/leading", true},
		{"ref\ttab", true},
		{"ref~1", true},
		{"ref^2", true},
		{"ref:path", true},
		{"ref?glob", true},
		{"ref*glob", true},
		{"ref[0]", true},
		{"ref\\esc", true},
		{"--exec=malicious", true},
		{"..", true},
	}
	for _, tt := range tests {
		err := validateGitRef(tt.ref)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateGitRef(%q) err=%v, wantErr=%v", tt.ref, err, tt.wantErr)
		}
	}
}
