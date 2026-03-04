package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yeasy/ask/internal/cache"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/github"
	"github.com/yeasy/ask/internal/skill"
)

// SkillInfo represents a skill for API responses
type SkillInfo struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	URL         string   `json:"url,omitempty"`
	Version     string   `json:"version,omitempty"`
	Path        string   `json:"path,omitempty"`
	Agents      []string `json:"agents,omitempty"`
	Repo        string   `json:"repo,omitempty"`
	IconURL     string   `json:"icon_url,omitempty"`
}

func getRepoFromGitConfig(path string) string {
	// Only check the skill's own .git directory
	// Skills are typically cloned directly, so .git should be at the skill path
	gitConfigPath := filepath.Join(path, ".git", "config")
	return parseGitConfigForRepo(gitConfigPath)
}

func parseGitConfigForRepo(gitConfigPath string) string {
	data, err := os.ReadFile(gitConfigPath)
	if err != nil {
		return ""
	}
	content := string(data)
	// Simple parsing for remote "origin" url
	// [remote "origin"]
	// 	url = https://github.com/owner/repo.git or git@github.com:owner/repo.git
	if idx := strings.Index(content, "[remote \"origin\"]"); idx != -1 {
		rest := content[idx:]
		if urlIdx := strings.Index(rest, "url = "); urlIdx != -1 {
			start := urlIdx + 6
			end := strings.Index(rest[start:], "\n")
			if end != -1 {
				url := strings.TrimSpace(rest[start : start+end])
				// Clean up URL to get owner/repo
				url = strings.TrimSuffix(url, ".git")
				if strings.Contains(url, "github.com") {
					parts := strings.Split(url, "github.com")
					if len(parts) > 1 {
						return strings.Trim(parts[1], "/:")
					}
				}
			}
		}
	}
	return ""
}

func (s *Server) handleSkills(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		if os.IsNotExist(err) {
			def := config.DefaultConfig()
			cfg = &def
		} else {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Map to aggregate skill info by name
	skillMap := make(map[string]*SkillInfo)

	// Helper to get or create skill info
	getOrCreate := func(name string) *SkillInfo {
		if _, exists := skillMap[name]; !exists {
			skillMap[name] = &SkillInfo{Name: name, Agents: []string{}}
		}
		return skillMap[name]
	}

	// Load lockfile for additional metadata
	lockFile, _ := config.LoadLockFile() // Ignore error, file might not exist
	lockMap := make(map[string]string)   // map name -> url
	if lockFile != nil {
		for _, entry := range lockFile.Skills {
			lockMap[entry.Name] = entry.URL
		}
	}

	// 1. Scan Configured/Legacy Skills first (base metadata)
	for _, si := range cfg.SkillsInfo {
		info := getOrCreate(si.Name)
		info.Description = si.Description
		info.URL = si.URL
		// Try to deduce repo from URL if present
		if si.URL != "" && strings.Contains(si.URL, "github.com") {
			parts := strings.Split(si.URL, "github.com/")
			if len(parts) > 1 {
				repoName := strings.TrimSuffix(parts[1], ".git")
				info.Repo = repoName
			}
		}
	}
	for _, name := range cfg.Skills {
		getOrCreate(name)
	}

	// 2. Scan each Agent directory for installed skills
	// We want to verify which agents actually have the skill installed
	toolTargets := cfg.GetEnabledToolTargets()

	for _, target := range toolTargets {
		// List subdirectories in the agent's skills dir
		entries, err := os.ReadDir(target.SkillsDir)
		if err != nil {
			continue // Skip if dir doesn't exist or is unreadable
		}

		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				name := entry.Name()
				skillPath := filepath.Join(target.SkillsDir, name)

				// Only consider it a skill if it contains SKILL.md
				// This prevents repo root directories from being mistakenly treated as skills
				if !skill.FindSkillMD(skillPath) {
					continue
				}

				info := getOrCreate(name)

				// Add this agent to the list (deduplicate just in case)
				found := false
				for _, a := range info.Agents {
					if a == target.Name {
						found = true
						break
					}
				}
				if !found {
					info.Agents = append(info.Agents, target.Name)
				}

				// If we don't have a path yet (or if this is the first one found), set it
				// This might be ambiguous if installed in multiple locations, but just picking one for "Path" is fine for basic file ops
				if info.Path == "" {
					info.Path = skillPath
				}

				// Try to deduce repo from .git/config if not present
				if info.Repo == "" {
					info.Repo = getRepoFromGitConfig(skillPath)
				}

				// Try to deduce repo from lockfile if still missing
				if info.Repo == "" && info.URL == "" {
					if url, ok := lockMap[name]; ok {
						info.URL = url
					}
				}

				// Ensure info.Repo is populated if URL is present (from lockfile or other source)
				if info.Repo == "" && info.URL != "" && strings.Contains(info.URL, "github.com") {
					// Cleaner logic:
					// If contains "github.com/", get everything after the LAST occurrence.
					lastIdx := strings.LastIndex(info.URL, "github.com/")
					if lastIdx != -1 {
						repoName := info.URL[lastIdx+11:] // len("github.com/") = 11
						repoName = strings.TrimSuffix(repoName, ".git")
						// Clean up optional /tree/... path if present (deep link)
						if idx := strings.Index(repoName, "/tree/"); idx != -1 {
							repoName = repoName[:idx]
						}
						info.Repo = repoName
					}
				}

				// Try to read SKILL.md for metadata if missing
				if info.Version == "" || info.Description == "" || info.Repo == "" {
					if meta, err := skill.ParseSkillMD(skillPath); err == nil && meta != nil {
						if meta.Version != "" {
							info.Version = meta.Version
						}
						if info.Description == "" && meta.Description != "" {
							info.Description = meta.Description
						}
						// If SKILL.md has repository info (hypothetically), we could use it.
						// Currently skill.Metadata might not have it, but we can assume descriptions are meaningful.
					}
				}
			}
		}
	}

	// Convert map to slice
	skills := make([]SkillInfo, 0, len(skillMap))
	for _, info := range skillMap {
		// Normalize Repo Name to match configured aliases
		// If info.Repo matches a configured repo URL or Name, use the Name.
		if info.Repo != "" {
			for _, repo := range cfg.Repos {
				// Check specific name match
				if strings.EqualFold(info.Repo, repo.Name) {
					info.Repo = repo.Name
					break
				}
				// Check URL match
				// Repo URL might be "https://github.com/owner/repo" or "owner/repo"
				// info.Repo (deduced) is usually "owner/repo"

				// Normalize repo.URL to owner/repo for comparison if possible
				repoURL := repo.URL
				if strings.HasPrefix(repoURL, "https://github.com/") {
					repoURL = strings.TrimPrefix(repoURL, "https://github.com/")
					repoURL = strings.TrimSuffix(repoURL, ".git")
				}
				// Also strip /tree/... if present
				if idx := strings.Index(repoURL, "/tree/"); idx != -1 {
					repoURL = repoURL[:idx]
				}

				// Check matches
				if strings.EqualFold(info.Repo, repoURL) {
					info.Repo = repo.Name
					break
				}
			}
		}

		// Only include skills that are either in config OR installed in at least one agent
		// Actually, if it's in config but directory not found, it might be "broken" or uninstalled properly
		// But let's show everything we know about.
		skills = append(skills, *info)
	}

	jsonResponse(w, skills)
}

// SearchResult represents a search result
type SearchResult struct {
	Name        string   `json:"name"`
	FullName    string   `json:"full_name"`
	Description string   `json:"description"`
	Stars       int      `json:"stars"`
	URL         string   `json:"url"`
	Source      string   `json:"source"`
	RepoName    string   `json:"repo,omitempty"`
	Agents      []string `json:"agents,omitempty"`
}

func (s *Server) handleSkillSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query().Get("q")
	repoFilter := r.URL.Query().Get("repo")
	topic := "agent-skill"

	results := make([]SearchResult, 0)

	// 1. Search Local/Configured Repos via Cache
	reposCache, err := cache.NewReposCache()
	if err == nil {
		repoInfos, _ := reposCache.LoadIndex()
		repoMap := make(map[string]cache.RepoInfo)
		for _, info := range repoInfos {
			repoMap[info.Name] = info
		}

		skills, err := reposCache.SearchSkills(query)
		if err == nil {
			for _, skill := range skills {
				repo := repoMap[skill.RepoName]
				skillURL := repo.URL
				if skillURL != "" && !strings.HasSuffix(skillURL, ".git") {
					skillURL = fmt.Sprintf("%s/tree/HEAD/%s", strings.TrimSuffix(skillURL, "/"), skill.Path)
				}

				if repoFilter != "" && !strings.EqualFold(skill.RepoName, repoFilter) {
					continue
				}

				results = append(results, SearchResult{
					Name:        skill.Name,
					FullName:    skill.Name,
					Description: skill.Description,
					Stars:       repo.Stars,
					URL:         skillURL,
					Source:      "repo",
					RepoName:    skill.RepoName,
				})
			}
		} else {
			fmt.Printf("Error searching local cache: %v\n", err)
		}
	}

	// 2. Search GitHub
	ghRepos, err := github.SearchTopic(topic, query)
	if err != nil {
		if len(results) > 0 {
			jsonResponse(w, results)
			return
		}
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, repo := range ghRepos {
		results = append(results, SearchResult{
			Name:        repo.Name,
			FullName:    repo.FullName, // e.g. owner/repo
			Description: repo.Description,
			Stars:       repo.StargazersCount,
			URL:         repo.HTMLURL,
			Source:      "github",
			RepoName:    repo.FullName,
		})
	}

	jsonResponse(w, results)
}

// InstallRequest represents an install request
type InstallRequest struct {
	Name  string `json:"name"`
	Agent string `json:"agent,omitempty"`
}

func (s *Server) handleSkillInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitRequestBody(r)

	var req InstallRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := validateSkillName(req.Name); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Execute install command
	exe, err := os.Executable()
	if err != nil {
		jsonError(w, "Failed to get executable path", http.StatusInternalServerError)
		return
	}
	args := []string{"skill", "install", req.Name}
	if req.Agent != "" {
		args = append(args, "--agent", req.Agent)
	}

	cmd := exec.Command(exe, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		jsonError(w, fmt.Sprintf("Install failed: %s", string(output)), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Installed %s", req.Name),
		"output":  string(output),
	})
}

func (s *Server) handleSkillUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitRequestBody(r)

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := validateSkillName(req.Name); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Execute uninstall command with --all to fully remove
	exe, err := os.Executable()
	if err != nil {
		jsonError(w, "Failed to get executable path", http.StatusInternalServerError)
		return
	}
	cmd := exec.Command(exe, "skill", "uninstall", "--all", req.Name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		jsonError(w, fmt.Sprintf("Uninstall failed: %s", string(output)), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Uninstalled %s", req.Name),
		"output":  string(output),
	})
}

func (s *Server) handleSkillScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitRequestBody(r)

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		jsonError(w, "Path is required", http.StatusBadRequest)
		return
	}

	// Verify path exists
	info, err := os.Stat(req.Path)
	if err != nil {
		jsonError(w, fmt.Sprintf("Path access error: %s", err.Error()), http.StatusBadRequest)
		return
	}
	if !info.IsDir() {
		jsonError(w, "Path is not a directory", http.StatusBadRequest)
		return
	}

	// Call scan logic (limit depth to 3 for performance)
	results, err := skill.ScanDirectory(req.Path, 3)
	if err != nil {
		jsonError(w, fmt.Sprintf("Scan failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Add meta about if it's already installed?
	// For now just return raw scan results
	jsonResponse(w, results)
}

func (s *Server) handleSkillImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitRequestBody(r)

	var req struct {
		SrcPath string `json:"src_path"`
		Name    string `json:"name"` // Optional rename
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.SrcPath == "" {
		jsonError(w, "Source path is required", http.StatusBadRequest)
		return
	}

	// Use CLI logic to install from local path
	// ask install /path/to/skill
	exe, err := os.Executable()
	if err != nil {
		jsonError(w, "Failed to get executable path", http.StatusInternalServerError)
		return
	}

	args := []string{"skill", "install", req.SrcPath}
	// TODO: req.Name is currently ignored. The skill name is derived from the directory name.
	// To support renaming, the install CLI would need to be extended with a --name flag.

	cmd := exec.Command(exe, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		jsonError(w, fmt.Sprintf("Import failed: %s", string(output)), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{
		"status":  "success",
		"message": "Skill imported successfully",
		"output":  string(output),
	})
}

// FileNode represents a file or directory in the file tree
type FileNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"` // Relative path from root
	Type     string      `json:"type"` // "file" or "dir"
	Size     int64       `json:"size,omitempty"`
	Children []*FileNode `json:"children,omitempty"`
}

func (s *Server) handleSkillFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Mode: "tree" (default) or "content"
	mode := r.URL.Query().Get("mode")
	skillName := r.URL.Query().Get("skill")

	if skillName == "" {
		jsonError(w, "Skill name is required", http.StatusBadRequest)
		return
	}

	// Find the skill path
	// Reuse logic from handleSkills or just simple lookup?
	// Let's re-use simple lookup for now.
	cfg, _ := config.LoadConfig()
	if cfg == nil {
		def := config.DefaultConfig()
		cfg = &def
	}

	// Check installed locations
	var skillPath string
	toolTargets := cfg.GetEnabledToolTargets()
	for _, target := range toolTargets {
		p := filepath.Join(target.SkillsDir, skillName)
		if skill.FindSkillMD(p) {
			skillPath = p
			break
		}
	}

	// Also check Global if not found?
	if skillPath == "" {
		p := filepath.Join(config.GetGlobalSkillsDir(), skillName)
		if skill.FindSkillMD(p) {
			skillPath = p
		}
	}

	if skillPath == "" {
		jsonError(w, "Skill not found", http.StatusNotFound)
		return
	}

	if mode == "content" {
		// Read specific file
		relPath := r.URL.Query().Get("path")
		if relPath == "" {
			jsonError(w, "File path is required", http.StatusBadRequest)
			return
		}

		// Security check: prevent ../ traversal
		cleanRel := filepath.Clean(relPath)
		if strings.Contains(cleanRel, "..") || strings.HasPrefix(cleanRel, "/") || strings.HasPrefix(cleanRel, string(filepath.Separator)) {
			jsonError(w, "Invalid path", http.StatusForbidden)
			return
		}

		absPath := filepath.Join(skillPath, cleanRel)
		// Verify the resolved path is still inside skillPath
		rel, err := filepath.Rel(skillPath, absPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			jsonError(w, "Access denied", http.StatusForbidden)
			return
		}

		// Symlink Security Check
		fileInfo, err := os.Lstat(absPath)
		if err != nil {
			jsonError(w, "File not found", http.StatusNotFound)
			return
		}
		if fileInfo.Mode()&os.ModeSymlink != 0 {
			jsonError(w, "Symlinks are not allowed", http.StatusForbidden)
			return
		}

		// Size Check (Max 1MB)
		if fileInfo.Size() > 1024*1024 {
			jsonError(w, "File too large (>1MB)", http.StatusBadRequest)
			return
		}

		data, err := os.ReadFile(absPath)
		if err != nil {
			jsonError(w, fmt.Sprintf("Read failed: %v", err), http.StatusNotFound)
			return
		}

		// Detect content type? For now just return text/json
		jsonResponse(w, map[string]string{
			"content": string(data),
		})
		return
	}

	// Default: Return File Tree
	rootNode, err := buildFileTree(skillPath, "")
	if err != nil {
		jsonError(w, fmt.Sprintf("Tree build failed: %v", err), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, rootNode)
}

func buildFileTree(basePath string, relPath string) (*FileNode, error) {
	absPath := filepath.Join(basePath, relPath)
	info, err := os.Lstat(absPath) // Use Lstat to detect symlinks
	if err != nil {
		return nil, err
	}

	node := &FileNode{
		Name: info.Name(),
		Path: relPath, // Return relative path for frontend requests
		Type: "file",
		Size: info.Size(),
	}

	// Handle symlinks explicitly
	if info.Mode()&os.ModeSymlink != 0 {
		node.Type = "symlink"
		// We do NOT recurse into symlinks to prevent escaping root
		return node, nil
	}

	if info.IsDir() {
		node.Type = "dir"
		entries, err := os.ReadDir(absPath)
		if err != nil {
			return node, nil // Return empty dir if read fails
		}

		children := make([]*FileNode, 0)
		for _, entry := range entries {
			// Skip .git
			if entry.Name() == ".git" {
				continue
			}
			childRel := filepath.Join(relPath, entry.Name())
			childNode, err := buildFileTree(basePath, childRel)
			if err == nil {
				children = append(children, childNode)
			}
		}
		node.Children = children
	}

	return node, nil
}

func (s *Server) handleSkillReadme(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	if err := validateSkillName(name); err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		jsonError(w, "Failed to load config", http.StatusInternalServerError)
		return
	}

	skillsDir := cfg.GetSkillsDir()
	skillPath := filepath.Join(skillsDir, name)

	// Check if skill exists
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		jsonError(w, "Skill not found", http.StatusNotFound)
		return
	}

	// Try to find SKILL.md (case insensitive)
	readmePath := ""
	entries, err := os.ReadDir(skillPath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.EqualFold(entry.Name(), "SKILL.md") {
				readmePath = filepath.Join(skillPath, entry.Name())
				break
			}
		}
	}

	if readmePath == "" {
		jsonError(w, "Documentation not found (SKILL.md)", http.StatusNotFound)
		return
	}

	content, err := os.ReadFile(readmePath)
	if err != nil {
		jsonError(w, "Failed to read documentation", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{
		"content": string(content),
	})
}
