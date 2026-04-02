// Package git provides git operations helpers.
package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yeasy/ask/internal/filesystem"
	"github.com/yeasy/ask/internal/ui"
)

// Clone clones a git repository to the specified destination.
// Only HTTPS URLs are accepted for security.
func Clone(ctx context.Context, url, dest string) error {
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("git clone requires HTTPS URL: %s", url)
	}
	bar := ui.NewSpinner(fmt.Sprintf("Cloning %s...", filepath.Base(url)))
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--progress", "--", url, dest)
	cmd.Stdout = bar
	cmd.Stderr = bar
	err := cmd.Run()
	_ = bar.Finish()
	if err != nil {
		return fmt.Errorf("git clone %s: %w", url, err)
	}
	return nil
}

// SparseClone clones only a specific subdirectory using sparse checkout.
// This is much faster than full clone for large repos when only a subdir is needed.
// Only HTTPS URLs are accepted for security.
func SparseClone(ctx context.Context, repoURL, branch, subDir, dest string) error {
	if !strings.HasPrefix(repoURL, "https://") {
		return fmt.Errorf("git clone requires HTTPS URL: %s", repoURL)
	}
	// Validate subDir to prevent path traversal.
	// Use filepath.ToSlash for consistent comparison across platforms
	// (filepath.Clean converts / to \ on Windows).
	cleaned := filepath.Clean(subDir)
	if filepath.ToSlash(cleaned) != filepath.ToSlash(subDir) || strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return fmt.Errorf("invalid subdirectory: path traversal not allowed")
	}

	bar := ui.NewSpinner(fmt.Sprintf("Sparse cloning %s from %s...", subDir, filepath.Base(repoURL)))
	defer func() { _ = bar.Finish() }()

	// Step 1: Clone with filter and no checkout
	ui.UpdateDescription(bar, "Initializing sparse clone...")
	args := []string{"clone", "--filter=blob:none", "--no-checkout", "--depth", "1", "--progress"}
	if branch != "" {
		if err := validateGitRef(branch); err != nil {
			return fmt.Errorf("invalid branch ref: %w", err)
		}
		args = append(args, "--branch", branch)
	}
	args = append(args, "--", repoURL, dest)

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stdout = bar
	cmd.Stderr = bar
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sparse clone init failed: %w", err)
	}

	// Clean up dest directory if any subsequent step fails
	cleanup := func(stepErr error) error {
		_ = os.RemoveAll(dest)
		return stepErr
	}

	// Step 2: Initialize sparse-checkout in cone mode
	ui.UpdateDescription(bar, "Configuring sparse checkout...")
	cmd = exec.CommandContext(ctx, "git", "sparse-checkout", "init", "--cone")
	cmd.Dir = dest
	cmd.Stdout = bar
	cmd.Stderr = bar
	if err := cmd.Run(); err != nil {
		return cleanup(fmt.Errorf("sparse-checkout init failed: %w", err))
	}

	// Step 3: Set the subdirectory to checkout
	ui.UpdateDescription(bar, fmt.Sprintf("Setting checkout path to %s...", subDir))
	cmd = exec.CommandContext(ctx, "git", "sparse-checkout", "set", "--", subDir)
	cmd.Dir = dest
	cmd.Stdout = bar
	cmd.Stderr = bar
	if err := cmd.Run(); err != nil {
		return cleanup(fmt.Errorf("sparse-checkout set failed: %w", err))
	}

	// Step 4: Checkout
	ui.UpdateDescription(bar, "Checking out files...")
	cmd = exec.CommandContext(ctx, "git", "checkout")
	cmd.Dir = dest
	cmd.Stdout = bar
	cmd.Stderr = bar
	if err := cmd.Run(); err != nil {
		return cleanup(fmt.Errorf("checkout failed: %w", err))
	}

	return nil
}

// InstallSubdir installs a subdirectory from a repository
// Uses sparse checkout for efficiency, falls back to full clone if sparse fails
func InstallSubdir(ctx context.Context, repoURL, branch, subDir, dest string) error {
	// Validate subDir early so both sparse and fallback paths are protected.
	cleaned := filepath.Clean(subDir)
	if filepath.ToSlash(cleaned) != filepath.ToSlash(subDir) || strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return fmt.Errorf("invalid subdirectory: path traversal not allowed")
	}

	// Create temp dir for sparse clone
	tempDir, err := os.MkdirTemp("", "ask-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }() // Clean up

	// Try sparse checkout first
	if err := SparseClone(ctx, repoURL, branch, subDir, tempDir); err != nil {
		// Fallback to full clone
		fmt.Fprintf(os.Stderr, "Sparse checkout failed, falling back to full clone...\n")
		_ = os.RemoveAll(tempDir) // Clean up failed sparse clone
		tempDir, err = os.MkdirTemp("", "ask-install-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}

		if err := Clone(ctx, repoURL, tempDir); err != nil {
			return fmt.Errorf("failed to clone repo: %w", err)
		}

		// If branch is specified, checkout that branch in full clone fallback
		if branch != "" {
			if err := Checkout(ctx, tempDir, branch); err != nil {
				return fmt.Errorf("failed to checkout branch %s: %w", branch, err)
			}
		}
	}

	// Copy subdirectory to destination
	srcPath := filepath.Join(tempDir, subDir)

	// Check if srcPath exists
	if _, err := os.Stat(srcPath); os.IsNotExist(err) {
		return fmt.Errorf("subdirectory %s not found in repo", subDir)
	}

	return filesystem.CopyDir(srcPath, dest)
}

// GetLatestTag returns the latest tag for a repository in the given path
func GetLatestTag(ctx context.Context, repoPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "describe", "--tags", "--abbrev=0")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// Checkout checks out a specific tag or branch.
// The ref is validated to prevent unexpected git behavior from malformed references.
func Checkout(ctx context.Context, repoPath, ref string) error {
	if err := validateGitRef(ref); err != nil {
		return fmt.Errorf("invalid git ref: %w", err)
	}
	cmd := exec.CommandContext(ctx, "git", "checkout", ref)
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// validateGitRef checks that a git reference string is safe to use.
func validateGitRef(ref string) error {
	if ref == "" {
		return fmt.Errorf("ref cannot be empty")
	}
	if strings.Contains(ref, "..") {
		return fmt.Errorf("ref cannot contain '..'")
	}
	if strings.ContainsAny(ref, " \t\n\r~^:?*[\\") {
		return fmt.Errorf("ref contains invalid characters")
	}
	if strings.HasPrefix(ref, "-") {
		return fmt.Errorf("ref cannot start with '-'")
	}
	if strings.HasPrefix(ref, "/") {
		return fmt.Errorf("ref cannot start with '/'")
	}
	return nil
}

// GetCurrentCommit returns the current commit hash of the repository
func GetCurrentCommit(ctx context.Context, repoPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
