package repository

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/github"
)

const maxResponseBodySize = 5 * 1024 * 1024 // 5MB

// registryHTTPClient is a shared HTTP client for registry requests.
var registryHTTPClient = &http.Client{Timeout: 15 * time.Second}

// rawBaseURL is the base URL for fetching raw files from GitHub.
// It can be overridden in tests to point to a local httptest server.
var rawBaseURL = "https://raw.githubusercontent.com"

// RegistrySkill represents a skill entry in the registry index
type RegistrySkill struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Stars       int      `json:"stars"`
	Featured    bool     `json:"featured"`
	InstallCmd  string   `json:"install_cmd"`
}

// RegistryIndex represents the registry index.json structure
type RegistryIndex struct {
	Version string          `json:"version"`
	Skills  []RegistrySkill `json:"skills"`
}

// FetchSkillsFromRegistry fetches skills from a registry index.json hosted on GitHub
func FetchSkillsFromRegistry(registryURL string, keyword string) ([]github.Repository, error) {
	if config.IsOffline() {
		return nil, fmt.Errorf("offline mode: cannot fetch registry")
	}

	// Construct raw GitHub URL from path like "yeasy/awesome-agent-skills/registry/index.json"
	parts := strings.SplitN(registryURL, "/", 3)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid registry URL format: %s (expected owner/repo/path)", registryURL)
	}
	owner := parts[0]
	repo := parts[1]
	path := parts[2]

	// Reject path traversal and dangerous characters in registry URL segments
	if strings.Contains(owner, "..") || strings.Contains(repo, "..") || strings.Contains(path, "..") {
		return nil, fmt.Errorf("invalid registry URL: path traversal detected")
	}
	if strings.ContainsAny(owner, "/\\") || strings.ContainsAny(repo, "/\\") {
		return nil, fmt.Errorf("invalid registry URL: invalid characters in owner or repo")
	}
	if owner == "" || repo == "" || path == "" {
		return nil, fmt.Errorf("invalid registry URL: empty segment in %s", registryURL)
	}

	body, err := fetchRegistryIndex(owner, repo, path)
	if err != nil {
		return nil, err
	}

	var index RegistryIndex
	if err := json.Unmarshal(body, &index); err != nil {
		return nil, fmt.Errorf("failed to parse registry index: %w", err)
	}

	var results []github.Repository
	keywordLower := strings.ToLower(keyword)

	for _, skill := range index.Skills {
		// Filter by keyword if provided
		if keyword != "" {
			matched := strings.Contains(strings.ToLower(skill.Name), keywordLower) ||
				strings.Contains(strings.ToLower(skill.Description), keywordLower)
			if !matched {
				// Check tags
				for _, tag := range skill.Tags {
					if strings.Contains(strings.ToLower(tag), keywordLower) {
						matched = true
						break
					}
				}
			}
			if !matched {
				continue
			}
		}

		results = append(results, github.Repository{
			Name:            skill.Name,
			FullName:        skill.Source + "/" + skill.Name,
			Description:     skill.Description,
			HTMLURL:         skill.URL,
			StargazersCount: skill.Stars,
		})
	}

	return results, nil
}

// registryBranches lists the default-branch names tried, in order, when fetching
// a registry index. GitHub raw URLs require an explicit branch, so a registry
// hosted on a repo whose default branch is "master" would 404 if only "main"
// were tried.
var registryBranches = []string{"main", "master"}

// fetchRegistryIndex downloads the registry index bytes, trying each candidate
// default branch until one is found. A 404 falls through to the next branch;
// any other non-200 status (or a transport error) is returned immediately.
func fetchRegistryIndex(owner, repo, path string) ([]byte, error) {
	lastStatus := http.StatusNotFound
	for _, branch := range registryBranches {
		rawURL := fmt.Sprintf("%s/%s/%s/%s/%s", rawBaseURL, owner, repo, branch, path)

		req, err := http.NewRequest("GET", rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "ask-cli")
		if token := github.GetTokenForRepo(config.Repo{}); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}

		resp, err := registryHTTPClient.Do(req)
		if err != nil {
			return nil, err
		}
		limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)

		if resp.StatusCode == http.StatusNotFound {
			lastStatus = resp.StatusCode
			_, _ = io.Copy(io.Discard, limitedBody)
			_ = resp.Body.Close()
			continue // branch may not exist; try the next candidate
		}
		if resp.StatusCode != http.StatusOK {
			status := resp.StatusCode
			_, _ = io.Copy(io.Discard, limitedBody)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("registry returned status %d", status)
		}

		body, err := io.ReadAll(limitedBody)
		_, _ = io.Copy(io.Discard, limitedBody)
		_ = resp.Body.Close()
		if err != nil {
			return nil, err
		}
		return body, nil
	}
	return nil, fmt.Errorf("registry returned status %d", lastStatus)
}
