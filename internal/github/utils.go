package github

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ParseBrowserURL parses a GitHub browser URL and extracts components.
// Input: https://github.com/owner/repo/tree/branch/path/to/skill
// Returns: repoURL, branch, subDir, skillName, ok
//
// Note: branch names containing "/" (e.g. "feature/v2") are not supported
// because the URL format is ambiguous — there is no way to distinguish
// the branch/path boundary without an API call.
func ParseBrowserURL(url string) (repoURL, branch, subDir, skillName string, ok bool) {
	// Remove trailing slashes
	url = strings.TrimSuffix(url, "/")

	// Upgrade http to https for security
	if strings.HasPrefix(url, "http://github.com/") {
		url = "https://" + url[len("http://"):]
	}

	// Verify this is actually a GitHub HTTPS URL
	if !strings.HasPrefix(url, "https://github.com/") {
		return "", "", "", "", false
	}

	// Check if it contains /tree/ (GitHub browser URL format)
	if !strings.Contains(url, "/tree/") {
		return "", "", "", "", false
	}

	// Pattern: https://github.com/owner/repo/tree/branch/path
	parts := strings.SplitN(url, "/tree/", 2)
	if len(parts) != 2 {
		return "", "", "", "", false
	}

	repoURL = parts[0]
	if !strings.HasSuffix(repoURL, ".git") {
		repoURL += ".git"
	}

	// Split branch and path
	branchAndPath := parts[1]
	pathParts := strings.SplitN(branchAndPath, "/", 2)
	branch = pathParts[0]

	if len(pathParts) > 1 {
		subDir = pathParts[1]
		// Skill name is the last component of the path
		skillName = filepath.Base(subDir)
	} else {
		// No subdir, use repo name from URL
		urlParts := strings.Split(parts[0], "/")
		skillName = urlParts[len(urlParts)-1]
	}

	return repoURL, branch, subDir, skillName, true
}

// ParseRepoURL parses a GitHub repository URL to extract owner and repo name
// Supports formats:
// - owner/repo
// - https://github.com/owner/repo
// - https://github.com/owner/repo.git
// - git@github.com:owner/repo.git
func ParseRepoURL(url string) (owner, repo string, err error) {
	url = strings.TrimSpace(url)
	url = strings.TrimSuffix(url, "/")
	url = strings.TrimSuffix(url, ".git")

	// Handle git@github.com:owner/repo
	if strings.HasPrefix(url, "git@") {
		parts := strings.Split(url, ":")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid git url: %s", url)
		}
		host := strings.TrimPrefix(parts[0], "git@")
		if host != "github.com" {
			return "", "", fmt.Errorf("unsupported git host: %s", host)
		}
		url = parts[1]
	}

	// Handle https://github.com/owner/repo
	if strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://") {
		// Match only exact github.com host (not subdomains like evil.github.com)
		matched := false
		for _, prefix := range []string{
			"https://github.com/",
			"http://github.com/",
			"https://www.github.com/",
			"http://www.github.com/",
		} {
			if strings.HasPrefix(url, prefix) {
				url = strings.TrimPrefix(url, prefix)
				matched = true
				break
			}
		}
		if !matched {
			return "", "", fmt.Errorf("invalid repo URL (must be github.com): %s", url)
		}
	}

	// Split owner/repo
	parts := strings.Split(url, "/")
	if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("invalid repo format (expected owner/repo): %s", url)
}
