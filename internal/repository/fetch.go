// Package repository manages skill repositories and sources.
package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/git"
	"github.com/yeasy/ask/internal/github"
	"github.com/yeasy/ask/internal/skill"
	"github.com/yeasy/ask/internal/skillhub"
)

// gitCloneTimeout is the maximum time allowed for a git clone operation.
const gitCloneTimeout = 5 * time.Minute

// FetchSkills returns a list of skills available in the given repository
func FetchSkills(repo config.Repo) ([]github.Repository, error) {
	switch repo.Type {
	case "topic":
		return github.SearchTopic(repo.URL, "")
	case "dir":
		// Try git-based discovery first (recursive and more reliable for deep structures)
		skills, err := FetchSkillsViaGit(repo)
		if err == nil && len(skills) > 0 {
			return skills, nil
		}

		// Fallback to API if git failed (e.g. no git installed) or found nothing
		parts := strings.Split(repo.URL, "/")
		if len(parts) >= 2 {
			owner := parts[0]
			name := parts[1]
			path := ""
			if len(parts) >= 3 {
				path = strings.Join(parts[2:], "/")
			}
			return github.SearchDir(owner, name, path)
		}
		return nil, fmt.Errorf("invalid repository URL format: %s", repo.URL)
	case "registry":
		return FetchSkillsFromRegistry(repo.URL, "")
	case "skillhub":
		return FetchSkillsFromSkillHub("", "")
	default:
		return nil, fmt.Errorf("unknown repository type: %s", repo.Type)
	}
}

// FetchSkillsViaGit clones a repo and discovers skills locally (no API needed)
func FetchSkillsViaGit(repo config.Repo) ([]github.Repository, error) {
	if repo.Type != "dir" {
		return nil, fmt.Errorf("git fetch only supports 'dir' type repos")
	}

	parts := strings.Split(repo.URL, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid repository URL format: %s", repo.URL)
	}

	owner := parts[0]
	repoName := parts[1]
	subPath := ""
	if len(parts) > 2 {
		subPath = strings.Join(parts[2:], "/")
	}

	cloneURL := fmt.Sprintf("https://github.com/%s/%s.git", owner, repoName)

	// Create temp dir
	tempDir, err := os.MkdirTemp("", "ask-discovery-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Clone repo with timeout to prevent indefinite hangs
	cloneCtx, cloneCancel := context.WithTimeout(context.Background(), gitCloneTimeout)
	defer cloneCancel()
	if err := git.Clone(cloneCtx, cloneURL, tempDir); err != nil {
		return nil, fmt.Errorf("failed to clone repo: %w", err)
	}

	// Scan for skills recursively
	return ScanSkills(tempDir, subPath, owner, repoName)
}

// ScanSkills scans a directory recursively for skills
func ScanSkills(baseDir, subPath, owner, repoName string) ([]github.Repository, error) {
	baseSearchPath := baseDir
	if subPath != "" {
		baseSearchPath = filepath.Join(baseDir, subPath)
	}

	// Helper to create a repository entry from a skill path
	createSkillRepo := func(fullPath string) (github.Repository, bool) {
		if !skill.FindSkillMD(fullPath) {
			return github.Repository{}, false
		}

		var desc string
		if meta, err := skill.ParseSkillMD(fullPath); err == nil && meta != nil {
			desc = meta.Description
		}

		// Rel path from baseDir (root of repo)
		relPathFromRoot, err := filepath.Rel(baseDir, fullPath)
		if err != nil {
			relPathFromRoot = ""
		}
		if relPathFromRoot == "." {
			relPathFromRoot = ""
		}
		// Normalize to forward slashes for URL-like install arguments
		relPathFromRoot = filepath.ToSlash(relPathFromRoot)

		installArg := fmt.Sprintf("%s/%s", owner, repoName)
		if relPathFromRoot != "" {
			installArg = fmt.Sprintf("%s/%s/%s", owner, repoName, relPathFromRoot)
		}

		// Skill name is the directory name
		name := filepath.Base(fullPath)
		// If base path is root, name might be repoName
		if fullPath == baseDir {
			name = repoName
		}

		return github.Repository{
			Name:        name,
			Description: desc,
			HTMLURL:     installArg,
		}, true
	}

	// 1. Check if the root search path itself is a skill
	if repo, ok := createSkillRepo(baseSearchPath); ok {
		return []github.Repository{repo}, nil
	}

	// 2. If not, scan subdirectories recursively
	var findSkillsRecursive func(currentPath string, depth int) ([]github.Repository, error)
	findSkillsRecursive = func(currentPath string, depth int) ([]github.Repository, error) {
		if depth > 2 { // Max recursion depth
			return nil, nil
		}

		entries, err := os.ReadDir(currentPath)
		if err != nil {
			return nil, err
		}

		var foundSkills []github.Repository

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			// Skip symlinks to prevent following links outside intended directory
			if entry.Type()&os.ModeSymlink != 0 {
				continue
			}

			// Skip hidden dirs (except .curated/.experimental if we want to be specific, but general skip is usually .git)
			if strings.HasPrefix(entry.Name(), ".") &&
				entry.Name() != ".curated" &&
				entry.Name() != ".experimental" {
				continue
			}

			fullPath := filepath.Join(currentPath, entry.Name())

			if repo, ok := createSkillRepo(fullPath); ok {
				foundSkills = append(foundSkills, repo)
			} else {
				// Not a skill, recurse if depth allows
				nestedSkills, err := findSkillsRecursive(fullPath, depth+1)
				if err == nil {
					foundSkills = append(foundSkills, nestedSkills...)
				}
			}
		}
		return foundSkills, nil
	}

	return findSkillsRecursive(baseSearchPath, 0)
}

// FetchSkillsFromSkillHub searches SkillHub and converts results to internal format
func FetchSkillsFromSkillHub(query string, _ string) ([]github.Repository, error) {
	client := skillhub.NewClient()
	skills, err := client.Search(query)
	if err != nil {
		return nil, err
	}

	var repos []github.Repository
	for _, s := range skills {
		desc := s.Description
		// Use slug as the install argument for now.
		// Install command needs to detect if it's a slug and resolve it.
		repo := github.Repository{
			Name:            s.Name,
			FullName:        s.Slug, // Storing slug in FullName for easier access
			Description:     desc,
			HTMLURL:         s.Slug, // Using Slug as the "URL" that install command receives
			StargazersCount: s.Stars,
			Source:          "skillhub",
		}
		repos = append(repos, repo)
	}
	return repos, nil
}
