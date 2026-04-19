// Package skillhub provides an interface to search and interact with skill registries.
package skillhub

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/yeasy/ask/internal/config"
)

// SearchURL is the endpoint for quick search
const SearchURL = "https://www.skillhub.club/api/search/quick"

const (
	// maxResponseBodySize is the maximum number of bytes read from HTTP responses.
	maxResponseBodySize = 2 * 1024 * 1024
	// defaultSearchLimit is the maximum number of results for quick search.
	defaultSearchLimit = "50"
)

// Pre-compiled regexes for Resolve()
var (
	reGitHubHref    = regexp.MustCompile(`href="(https://github\.com/[^"]+)"`)
	reRepoURLJSON   = regexp.MustCompile(`"repoUrl":"(https://github\.com/[^"]+)"`)
	reRepoURLEscape = regexp.MustCompile(`\\?"repoUrl\\?":\\?"(https://github\.com/[^"\\]+)\\?"`)
)

// Skill represents a skill from SkillHub search
type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Author      string   `json:"author"`
	Stars       int      `json:"github_stars"`
	Tags        []string `json:"tags"`
}

type searchResponse struct {
	Skills []Skill `json:"skills"`
}

// Client handles interaction with SkillHub
type Client struct {
	HTTPClient *http.Client
	BaseURL    string // override for testing; empty means use default SearchURL
}

// NewClient creates a new SkillHub client
func NewClient() *Client {
	return &Client{
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Search searches for skills on SkillHub
func (c *Client) Search(query string) ([]Skill, error) {
	// If query is empty, use a generic term or handle differently,
	// but the API seems to require a query param or returns empty.
	// For "list all", perhaps we need the catalog API, but that requires auth.
	// Let's rely on search for now.
	if query == "" {
		query = "agent" // default search term if none provided?
	}

	if config.IsOffline() {
		return nil, fmt.Errorf("search is not available in offline mode")
	}

	baseURL := SearchURL
	if c.BaseURL != "" {
		baseURL = c.BaseURL + "/api/search/quick"
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("limit", defaultSearchLimit)
	u.RawQuery = q.Encode()

	resp, err := c.HTTPClient.Get(u.String())
	if err != nil {
		return nil, err
	}
	limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)
	defer func() {
		_, _ = io.Copy(io.Discard, limitedBody)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SkillHub API returned status: %d", resp.StatusCode)
	}

	var result searchResponse
	if err := json.NewDecoder(limitedBody).Decode(&result); err != nil {
		return nil, err
	}

	return result.Skills, nil
}

// Resolve fetches the GitHub URL for a given skill slug
func (c *Client) Resolve(slug string) (string, error) {
	if config.IsOffline() {
		return "", fmt.Errorf("skill resolution is not available in offline mode")
	}
	baseHost := "https://www.skillhub.club"
	if c.BaseURL != "" {
		baseHost = c.BaseURL
	}
	skillURL := fmt.Sprintf("%s/skills/%s", baseHost, url.PathEscape(slug))
	resp, err := c.HTTPClient.Get(skillURL)
	if err != nil {
		return "", err
	}
	limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)
	defer func() {
		_, _ = io.Copy(io.Discard, limitedBody)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch skill page: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(limitedBody)
	if err != nil {
		return "", err
	}

	// Look for GitHub link in the page content
	bodyStr := string(body)
	matches := reGitHubHref.FindStringSubmatch(bodyStr)

	if len(matches) > 1 {
		rawURL := matches[1]
		// clean fragments (e.g. #plugins...)
		if idx := strings.Index(rawURL, "#"); idx != -1 {
			rawURL = rawURL[:idx]
		}
		if err := validateResolvedURL(rawURL); err != nil {
			return "", fmt.Errorf("resolved URL failed validation: %w", err)
		}
		return rawURL, nil
	}

	// Fallback: try to find repoUrl in Next.js hydration data or JSON
	matchesJSON := reRepoURLJSON.FindStringSubmatch(bodyStr)
	if len(matchesJSON) > 1 {
		rawURL := strings.ReplaceAll(matchesJSON[1], `\/`, "/")
		if err := validateResolvedURL(rawURL); err != nil {
			return "", fmt.Errorf("resolved URL failed validation: %w", err)
		}
		return rawURL, nil
	}

	// Try one more pattern for escaped JSON
	matchesEscaped := reRepoURLEscape.FindStringSubmatch(bodyStr)
	if len(matchesEscaped) > 1 {
		rawURL := matchesEscaped[1]
		if err := validateResolvedURL(rawURL); err != nil {
			return "", fmt.Errorf("resolved URL failed validation: %w", err)
		}
		return rawURL, nil
	}

	return "", fmt.Errorf("GitHub URL not found for skill: %s", slug)
}

// validateResolvedURL checks that a URL resolved from SkillHub is a valid GitHub repository URL.
// This prevents potential SSRF or supply-chain attacks if SkillHub returns unexpected URLs.
func validateResolvedURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return fmt.Errorf("URL must use HTTPS scheme, got %q", parsed.Scheme)
	}
	if parsed.Host != "github.com" {
		return fmt.Errorf("URL must be from github.com, got %q", parsed.Host)
	}
	// Must have at least owner/repo in path
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return fmt.Errorf("URL must contain owner/repo path")
	}
	// Reject path traversal
	if strings.Contains(parsed.Path, "..") {
		return fmt.Errorf("URL path contains path traversal")
	}
	return nil
}
