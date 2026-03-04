// Package github interacts with the GitHub API to search and fetch repositories.
package github

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/yeasy/ask/internal/cache"
	"github.com/yeasy/ask/internal/config"
)

const (
	// SkillTopic is the default topic to search for agent skills
	SkillTopic = "agent-skill"
	// APIURL is the GitHub API endpoint for searching repositories
	APIURL = "https://api.github.com/search/repositories"
)

// Global cache instance
var searchCache *cache.Cache

// OfflineMode returns whether the application is in offline mode.
// Delegates to config.OfflineMode as the single source of truth.
func isOffline() bool {
	return config.OfflineMode
}

func init() {
	// Initialize cache with default settings
	var err error
	searchCache, err = cache.New("", cache.DefaultTTL)
	if err != nil {
		// Cache is optional, continue without it
		searchCache = nil
	}
}

// SearchResult represents the response from GitHub search API
type SearchResult struct {
	TotalCount int          `json:"total_count"`
	Items      []Repository `json:"items"`
}

// Repository represents a GitHub repository structure
type Repository struct {
	Name            string    `json:"name"`
	FullName        string    `json:"full_name"`
	Description     string    `json:"description"`
	HTMLURL         string    `json:"html_url"`
	StargazersCount int       `json:"stargazers_count"`
	CloneURL        string    `json:"clone_url"`
	UpdatedAt       time.Time `json:"updated_at"`
	Source          string    `json:"-"` // Source name (e.g., "community", "anthropics")
	Owner           struct {
		Login string `json:"login"`
	} `json:"owner"`
}

// getAuthToken returns the GitHub token from environment variables
func getAuthToken() string {
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GH_TOKEN")
}

// SearchTopic searches GitHub for repositories with a specific topic and keyword
func SearchTopic(topic, keyword string) ([]Repository, error) {
	cacheKey := fmt.Sprintf("topic:%s:%s", topic, keyword)

	// Try cache first
	// In offline mode, we MUST find it in cache or return error
	if searchCache != nil {
		var cached []Repository
		if searchCache.Get(cacheKey, &cached) {
			return cached, nil
		}
	}

	if isOffline() {
		return nil, fmt.Errorf("offline mode: data not found in cache")
	}

	// Construct query: topic:<topic> <keyword>
	q := fmt.Sprintf("topic:%s %s", topic, keyword)

	params := url.Values{}
	params.Add("q", q)
	params.Add("sort", "stars")
	params.Add("order", "desc")

	req, err := http.NewRequest("GET", APIURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	// Request created above
	if token := getAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ask-cli")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var result SearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Cache the result
	if searchCache != nil {
		_ = searchCache.Set(cacheKey, result.Items)
	}

	return result.Items, nil
}

// Content represents a file or directory in a GitHub repository
type Content struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	HTMLURL string `json:"html_url"`
}

// SearchDir searches a specific directory in a GitHub repository for subdirectories (skills)
func SearchDir(owner, repo, path string) ([]Repository, error) {
	cacheKey := fmt.Sprintf("dir:%s/%s/%s", owner, repo, path)

	// Try cache first
	if searchCache != nil {
		var cached []Repository
		if searchCache.Get(cacheKey, &cached) {
			return cached, nil
		}
	}

	if isOffline() {
		return nil, fmt.Errorf("offline mode: data not found in cache")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", owner, repo, path)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	if token := getAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ask-cli")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var contents []Content
	if err := json.NewDecoder(resp.Body).Decode(&contents); err != nil {
		return nil, err
	}

	// Fetch repo details for stars
	repoDetails, err := FetchRepoDetails(owner, repo)
	stars := 0
	if err == nil {
		stars = repoDetails.StargazersCount
	}

	var skills []Repository
	for _, item := range contents {
		if item.Type == "dir" {
			// Try to get description from SKILL.md
			skillPath := path + "/" + item.Name
			if path == "" {
				skillPath = item.Name
			}
			desc := fetchSkillDescription(owner, repo, skillPath)
			if desc == "" {
				desc = "Skill from " + owner + "/" + repo
			}

			skills = append(skills, Repository{
				Name:            item.Name,
				FullName:        fmt.Sprintf("%s/%s/%s/%s", owner, repo, path, item.Name),
				Description:     desc,
				HTMLURL:         item.HTMLURL,
				StargazersCount: stars,
				CloneURL:        repoDetails.CloneURL,
			})
		}
	}

	// Cache the result
	if searchCache != nil {
		_ = searchCache.Set(cacheKey, skills)
	}

	return skills, nil
}

// fetchSkillDescription fetches the description from a skill's SKILL.md file
func fetchSkillDescription(owner, repo, skillPath string) string {
	// Check cache first
	cacheKey := fmt.Sprintf("skill-desc:%s/%s/%s", owner, repo, skillPath)
	if searchCache != nil {
		var cached string
		if searchCache.Get(cacheKey, &cached) {
			return cached
		}
	}

	// Fetch SKILL.md content
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s/SKILL.md", owner, repo, skillPath)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return ""
	}

	if token := getAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	req.Header.Set("Accept", "application/vnd.github.v3.raw") // Get raw file content
	req.Header.Set("User-Agent", "ask-cli")

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	// Read the content (limit to 4KB to avoid huge files)
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	content := string(buf[:n])

	// Parse description from SKILL.md (check both frontmatter and first paragraph)
	desc := parseDescriptionFromSkillMD(content)

	// Cache the description
	if searchCache != nil && desc != "" {
		_ = searchCache.Set(cacheKey, desc)
	}

	return desc
}

// parseDescriptionFromSkillMD extracts description from SKILL.md content
func parseDescriptionFromSkillMD(content string) string {
	lines := splitLines(content)

	// Check for YAML frontmatter
	if len(lines) > 0 && lines[0] == "---" {
		inFrontmatter := true
		for i := 1; i < len(lines) && inFrontmatter; i++ {
			line := lines[i]
			if line == "---" {
				inFrontmatter = false
				continue
			}
			// Look for description field
			if len(line) > 12 && line[:12] == "description:" {
				desc := trimQuotes(line[12:])
				if desc != "" {
					return truncate(desc, 60)
				}
			}
		}
	}

	// If no frontmatter description, look for first non-empty non-heading line
	for _, line := range lines {
		line = trimSpace(line)
		if line == "" || line == "---" {
			continue
		}
		if len(line) > 0 && line[0] == '#' {
			continue // Skip headings
		}
		// Found first content line
		return truncate(line, 60)
	}

	return ""
}

// Helper functions to avoid importing strings package
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func trimQuotes(s string) string {
	s = trimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// FetchRepoDetails fetches details of a GitHub repository including star count
func FetchRepoDetails(owner, repo string) (*Repository, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repo)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	if token := getAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ask-cli")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var repoInfo Repository
	if err := json.NewDecoder(resp.Body).Decode(&repoInfo); err != nil {
		return nil, err
	}
	return &repoInfo, nil
}
