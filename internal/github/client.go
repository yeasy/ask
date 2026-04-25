// Package github interacts with the GitHub API to search and fetch repositories.
package github

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/yeasy/ask/internal/cache"
	"github.com/yeasy/ask/internal/config"
)

const (
	// SkillTopic is the default topic to search for agent skills
	SkillTopic = "agent-skill"
	// APIURL is the GitHub API endpoint for searching repositories
	APIURL = "https://api.github.com/search/repositories"

	// httpTimeoutDefault is the default timeout for GitHub API requests
	httpTimeoutDefault = 10 * time.Second
	// httpTimeoutShort is a shorter timeout for non-critical requests like fetching descriptions
	httpTimeoutShort = 5 * time.Second
	// maxDescriptionReadBytes limits how much of SKILL.md we read for description extraction
	maxDescriptionReadBytes = 4096
	// maxResponseBodySize limits how much of an HTTP response body we read
	maxResponseBodySize = 5 * 1024 * 1024 // 5MB
)

// safeRedirect strips the Authorization header on cross-host redirects to
// prevent token leakage if the server redirects to a different domain.
func safeRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return fmt.Errorf("stopped after 10 redirects")
	}
	if len(via) > 0 && req.URL.Host != via[0].URL.Host {
		req.Header.Del("Authorization")
	}
	return nil
}

// Shared HTTP clients to enable connection reuse across requests
var (
	httpClientDefault = &http.Client{
		Timeout:       httpTimeoutDefault,
		CheckRedirect: safeRedirect,
	}
	httpClientShort = &http.Client{
		Timeout:       httpTimeoutShort,
		CheckRedirect: safeRedirect,
	}
)

// Global cache instance, protected by cacheMu for concurrent access
var (
	searchCache *cache.Cache
	cacheMu     sync.RWMutex
)

// isOffline returns whether the application is in offline mode.
// Delegates to config.IsOffline as the single source of truth.
func isOffline() bool {
	return config.IsOffline()
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

// cacheGet safely reads from the global cache under a read lock.
func cacheGet(key string, dest interface{}) bool {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if searchCache == nil {
		return false
	}
	return searchCache.Get(key, dest)
}

// cacheSet safely writes to the global cache under a write lock.
func cacheSet(key string, value interface{}) {
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if searchCache != nil {
		_ = searchCache.Set(key, value)
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

// getAuthToken returns the GitHub token from environment variables.
// Priority: ASK_GITHUB_TOKEN → GITHUB_TOKEN → GH_TOKEN.
func getAuthToken() string {
	if token := os.Getenv("ASK_GITHUB_TOKEN"); token != "" {
		return token
	}
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token
	}
	return os.Getenv("GH_TOKEN")
}

// GetTokenForRepo returns the best available token for a given repo config.
// Priority: per-repo token → environment variable → empty string.
func GetTokenForRepo(repo config.Repo) string {
	if repo.Token != "" {
		return repo.Token
	}
	return getAuthToken()
}

// GetAPIBaseURL returns the API base URL for a repo.
// Defaults to "https://api.github.com" if not specified.
func GetAPIBaseURL(repo config.Repo) string {
	if repo.BaseURL != "" {
		return strings.TrimRight(repo.BaseURL, "/")
	}
	return "https://api.github.com"
}

// SearchTopic searches GitHub for repositories with a specific topic and keyword
func SearchTopic(topic, keyword string) ([]Repository, error) {
	cacheKey := fmt.Sprintf("topic:%s:%s", topic, keyword)

	// Try cache first
	// In offline mode, we MUST find it in cache or return error
	var cached []Repository
	if cacheGet(cacheKey, &cached) {
		return cached, nil
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

	resp, err := httpClientDefault.Do(req)
	if err != nil {
		return nil, err
	}
	limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)
	defer func() {
		_, _ = io.Copy(io.Discard, limitedBody)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var result SearchResult
	if err := json.NewDecoder(limitedBody).Decode(&result); err != nil {
		return nil, err
	}

	// Cache the result
	cacheSet(cacheKey, result.Items)

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
	var cached []Repository
	if cacheGet(cacheKey, &cached) {
		return cached, nil
	}

	if isOffline() {
		return nil, fmt.Errorf("offline mode: data not found in cache")
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s", url.PathEscape(owner), url.PathEscape(repo), escapePathSegments(path))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	if token := getAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ask-cli")

	resp, err := httpClientDefault.Do(req)
	if err != nil {
		return nil, err
	}
	limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)
	defer func() {
		_, _ = io.Copy(io.Discard, limitedBody)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var contents []Content
	if err := json.NewDecoder(limitedBody).Decode(&contents); err != nil {
		return nil, err
	}

	// Fetch repo details for stars
	repoDetails, err := FetchRepoDetails(owner, repo)
	stars := 0
	cloneURL := ""
	if err == nil && repoDetails != nil {
		stars = repoDetails.StargazersCount
		cloneURL = repoDetails.CloneURL
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
				CloneURL:        cloneURL,
			})
		}
	}

	// Cache the result
	cacheSet(cacheKey, skills)

	return skills, nil
}

// fetchSkillDescription fetches the description from a skill's SKILL.md file
func fetchSkillDescription(owner, repo, skillPath string) string {
	// Check cache first
	cacheKey := fmt.Sprintf("skill-desc:%s/%s/%s", owner, repo, skillPath)
	var cached string
	if cacheGet(cacheKey, &cached) {
		return cached
	}

	// Fetch SKILL.md content
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s/SKILL.md", url.PathEscape(owner), url.PathEscape(repo), escapePathSegments(skillPath))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return ""
	}

	if token := getAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	req.Header.Set("Accept", "application/vnd.github.v3.raw") // Get raw file content
	req.Header.Set("User-Agent", "ask-cli")

	resp, err := httpClientShort.Do(req)
	if err != nil {
		return ""
	}
	limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)
	defer func() {
		_, _ = io.Copy(io.Discard, limitedBody)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	data, err := io.ReadAll(io.LimitReader(limitedBody, maxDescriptionReadBytes))
	if err != nil || len(data) == 0 {
		return ""
	}
	content := string(data)

	// Parse description from SKILL.md (check both frontmatter and first paragraph)
	desc := parseDescriptionFromSkillMD(content)

	// Cache the description
	if desc != "" {
		cacheSet(cacheKey, desc)
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
			if strings.HasPrefix(line, "description:") {
				desc := trimQuotes(strings.TrimPrefix(line, "description:"))
				if desc != "" {
					return truncate(desc, 60)
				}
			}
		}
	}

	// If no frontmatter description, look for first non-empty non-heading line
	for _, line := range lines {
		line = strings.TrimSpace(line)
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

// splitLines splits a string into lines by newline character.
func splitLines(s string) []string {
	return strings.Split(s, "\n")
}

func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && ((s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'')) {
		return s[1 : len(s)-1]
	}
	return s
}

// escapePathSegments escapes each segment of a URL path individually,
// preserving '/' as path delimiters.
func escapePathSegments(p string) string {
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		segments[i] = url.PathEscape(seg)
	}
	return strings.Join(segments, "/")
}

func truncate(s string, maxLen int) string {
	if maxLen < 4 {
		maxLen = 4
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-3]) + "..."
}

// RepoInfo holds enriched repository metadata for scoring
type RepoInfo struct {
	Stars      int
	Forks      int
	IsOrg      bool
	HasLicense bool
	OwnerAge   int // years since account creation
}

// Client provides GitHub API access
type Client struct {
	httpClient *http.Client
	token      string
}

// NewClient creates a new GitHub API client
func NewClient() *Client {
	return &Client{
		httpClient: httpClientDefault,
		token:      getAuthToken(),
	}
}

// GetRepoInfo fetches enriched repository metadata for trust scoring
func (c *Client) GetRepoInfo(owner, repo string) (*RepoInfo, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ask-cli")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)
	defer func() {
		_, _ = io.Copy(io.Discard, limitedBody)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var raw struct {
		Stars   int `json:"stargazers_count"`
		Forks   int `json:"forks_count"`
		License *struct {
			Key string `json:"key"`
		} `json:"license"`
		Owner struct {
			Type      string    `json:"type"`
			CreatedAt time.Time `json:"created_at"`
		} `json:"owner"`
	}
	if decErr := json.NewDecoder(limitedBody).Decode(&raw); decErr != nil {
		return nil, decErr
	}

	info := &RepoInfo{
		Stars:      raw.Stars,
		Forks:      raw.Forks,
		IsOrg:      raw.Owner.Type == "Organization",
		HasLicense: raw.License != nil && raw.License.Key != "",
	}

	// Owner created_at is not included in repo response; fetch owner separately
	ownerAge, _ := c.fetchOwnerAge(owner)
	info.OwnerAge = ownerAge

	return info, nil
}

// fetchOwnerAge returns the age of a GitHub account in years
func (c *Client) fetchOwnerAge(owner string) (int, error) {
	apiURL := fmt.Sprintf("https://api.github.com/users/%s", url.PathEscape(owner))
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ask-cli")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)
	defer func() {
		_, _ = io.Copy(io.Discard, limitedBody)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var user struct {
		CreatedAt time.Time `json:"created_at"`
	}
	if decErr := json.NewDecoder(limitedBody).Decode(&user); decErr != nil {
		return 0, decErr
	}

	years := int(time.Since(user.CreatedAt).Hours() / 8760)
	return years, nil
}

// FetchRepoDetails fetches details of a GitHub repository including star count
func FetchRepoDetails(owner, repo string) (*Repository, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	if token := getAuthToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ask-cli")

	resp, err := httpClientDefault.Do(req)
	if err != nil {
		return nil, err
	}
	limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)
	defer func() {
		_, _ = io.Copy(io.Discard, limitedBody)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned status: %d", resp.StatusCode)
	}

	var repoInfo Repository
	if err := json.NewDecoder(limitedBody).Decode(&repoInfo); err != nil {
		return nil, err
	}
	return &repoInfo, nil
}
