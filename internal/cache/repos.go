package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ReposCache manages local git repository cache for skill discovery
type ReposCache struct {
	baseDir string
}

// RepoInfo represents cached repository metadata
type RepoInfo struct {
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	LocalPath    string    `json:"local_path"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastSyncedAt time.Time `json:"last_synced_at"`
	Stars        int       `json:"stars"`
}

// SkillEntry represents a skill found in local cache
type SkillEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path"`
	RepoName    string `json:"repo_name"`
}

// NewReposCache creates a new repos cache instance
func NewReposCache() (*ReposCache, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}
	baseDir := filepath.Join(homeDir, ".ask", "repos")

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create repos cache dir: %w", err)
	}

	return &ReposCache{baseDir: baseDir}, nil
}

// GetReposCacheDir returns the repos cache directory path
func GetReposCacheDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".ask", "repos")
	}
	return filepath.Join(homeDir, ".ask", "repos")
}

// HasRepo checks if a repository is cached locally.
// Uses Lstat to avoid following symlinks.
func (c *ReposCache) HasRepo(repoName string) bool {
	repoPath := filepath.Join(c.baseDir, sanitizeRepoName(repoName))
	fi, err := os.Lstat(repoPath)
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeSymlink == 0
}

// IsStale checks if a repository's cache is older than the ttl
func (c *ReposCache) IsStale(repoName string, ttl time.Duration) bool {
	infos, err := c.LoadIndex()
	if err != nil {
		return true
	}
	for _, info := range infos {
		if info.Name == repoName {
			return time.Since(info.LastSyncedAt) > ttl
		}
	}
	return true
}

// CloneOrPull clones a repo if not exists, or pulls if exists
func (c *ReposCache) CloneOrPull(ctx context.Context, repoURL, repoName string) error {
	repoPath := filepath.Join(c.baseDir, sanitizeRepoName(repoName))

	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		// Validate URL scheme to prevent cloning arbitrary local paths
		if !strings.HasPrefix(repoURL, "https://") {
			return fmt.Errorf("repository URL must use HTTPS: %s", repoURL)
		}
		// Clone with depth=1 for speed
		cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", "--", repoURL, repoPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git clone failed: %w\nOutput: %s", err, string(output))
		}
		return nil
	}

	// Pull latest
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "pull", "--ff-only")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git pull failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// ListSkills lists all skills in a cached repo
func (c *ReposCache) ListSkills(repoName string) ([]SkillEntry, error) {
	repoPath := filepath.Join(c.baseDir, sanitizeRepoName(repoName))
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repo %s not cached", repoName)
	}

	var skills []SkillEntry

	// Walk the repo looking for SKILL.md files
	err := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Skip permission errors but continue walking
			if os.IsPermission(err) {
				return nil
			}
			return err
		}
		// Skip symlinks to prevent following links outside intended directory
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		// Skip .git directory
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		// Look for SKILL.md
		if !d.IsDir() && strings.ToUpper(d.Name()) == "SKILL.MD" {
			skillDir := filepath.Dir(path)
			skillName := filepath.Base(skillDir)

			// Try to extract description from SKILL.md
			description := extractDescription(path)

			skills = append(skills, SkillEntry{
				Name:        skillName,
				Description: description,
				Path:        skillDir,
				RepoName:    repoName,
			})
		}
		return nil
	})

	return skills, err
}

// SearchSkills searches for skills matching keyword in all cached repos
func (c *ReposCache) SearchSkills(keyword string) ([]SkillEntry, error) {
	keyword = strings.ToLower(keyword)
	var results []SkillEntry

	// List all cached repos
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skills, err := c.ListSkills(entry.Name())
		if err != nil {
			continue
		}

		for _, skill := range skills {
			if strings.Contains(strings.ToLower(skill.Name), keyword) ||
				strings.Contains(strings.ToLower(skill.Description), keyword) {
				results = append(results, skill)
			}
		}
	}

	return results, nil
}

// GetCachedRepos returns list of cached repo names
func (c *ReposCache) GetCachedRepos() []string {
	var repos []string
	entries, err := os.ReadDir(c.baseDir)
	if err != nil {
		return repos
	}
	for _, entry := range entries {
		if entry.IsDir() && entry.Name() != ".git" {
			repos = append(repos, entry.Name())
		}
	}
	return repos
}

// sanitizeRepoName converts owner/repo to owner-repo for filesystem
func sanitizeRepoName(name string) string {
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, "\\", "-")
	name = strings.ReplaceAll(name, "..", "_")
	name = strings.TrimLeft(name, ".-")
	if name == "" {
		name = "_"
	}
	return name
}

// maxDescriptionFileSize is the maximum SKILL.md file size to read for description extraction
const maxDescriptionFileSize = 8192

// extractDescription reads SKILL.md and extracts description from frontmatter.
// Opens the file then stats via the fd to avoid TOCTOU between check and read.
func extractDescription(skillMDPath string) string {
	f, err := os.Open(skillMDPath)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil || info.Size() > maxDescriptionFileSize || !info.Mode().IsRegular() {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(f, maxDescriptionFileSize))
	if err != nil {
		return ""
	}
	content := string(data)
	// Normalize Windows line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")

	// Check for YAML frontmatter
	if !strings.HasPrefix(content, "---") {
		return ""
	}

	// Find end of frontmatter
	endIdx := strings.Index(content[3:], "---")
	if endIdx == -1 {
		return ""
	}

	frontmatter := content[3 : endIdx+3]
	lines := strings.Split(frontmatter, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "description:") {
			desc := strings.TrimPrefix(line, "description:")
			desc = strings.TrimSpace(desc)
			desc = strings.Trim(desc, "\"'")
			return desc
		}
	}

	return ""
}

// SaveIndex saves the current repo index to disk (without stars)
func (c *ReposCache) SaveIndex() error {
	return c.SaveIndexWithStars(nil, nil)
}

// SaveIndexWithStars saves the current repo index to disk with star counts and URLs
func (c *ReposCache) SaveIndexWithStars(starCounts map[string]int, urls map[string]string) error {
	indexPath := filepath.Join(c.baseDir, "index.json")
	repos := c.GetCachedRepos()

	// Load existing index to preserve stars, URLs, and sync times for repos not synced in this run
	existingStars := make(map[string]int)
	existingURLs := make(map[string]string)
	existingSyncTimes := make(map[string]time.Time)
	existingInfos, _ := c.LoadIndex()
	for _, info := range existingInfos {
		existingStars[info.Name] = info.Stars
		existingURLs[info.Name] = info.URL
		existingSyncTimes[info.Name] = info.LastSyncedAt
	}

	var repoInfos []RepoInfo
	for _, repo := range repos {
		repoPath := filepath.Join(c.baseDir, repo)
		info, err := os.Stat(repoPath)
		if err != nil {
			continue
		}

		// Use new star count if provided, otherwise use existing
		// Logic: if provided in map, it means we just synced it (successfully or attempted)
		// So we update LastSyncedAt if starCounts has entry?
		// Actually starCounts is populated only on success in syncCmd.

		stars := 0
		lastSyncedAt := existingSyncTimes[repo]

		if starCounts != nil {
			if count, ok := starCounts[repo]; ok {
				stars = count
				lastSyncedAt = time.Now()
			} else if existingCount, ok := existingStars[repo]; ok {
				stars = existingCount
			}
		} else if existingCount, ok := existingStars[repo]; ok {
			stars = existingCount
		}

		// Use new URL if provided, otherwise use existing
		url := ""
		if urls != nil {
			if u, ok := urls[repo]; ok {
				url = u
			} else if existingURL, ok := existingURLs[repo]; ok {
				url = existingURL
			}
		} else if existingURL, ok := existingURLs[repo]; ok {
			url = existingURL
		}

		repoInfos = append(repoInfos, RepoInfo{
			Name:         repo,
			URL:          url,
			LocalPath:    repoPath,
			UpdatedAt:    info.ModTime(),
			LastSyncedAt: lastSyncedAt,
			Stars:        stars,
		})
	}

	data, err := json.MarshalIndent(repoInfos, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(indexPath, data, 0600)
}

// atomicWriteFile writes data to a temp file then renames it to the target path.
// This prevents partial writes from corrupting the file on crash.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}

// maxIndexFileSize limits the index file size to prevent OOM on malformed files
const maxIndexFileSize = 5 * 1024 * 1024 // 5MB

// LoadIndex loads the repo index from disk
func (c *ReposCache) LoadIndex() ([]RepoInfo, error) {
	indexPath := filepath.Join(c.baseDir, "index.json")
	info, err := os.Stat(indexPath)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxIndexFileSize {
		return nil, fmt.Errorf("index file too large: %d bytes (max %d)", info.Size(), maxIndexFileSize)
	}
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}
	var repoInfos []RepoInfo
	if err := json.Unmarshal(data, &repoInfos); err != nil {
		return nil, err
	}
	return repoInfos, nil
}
