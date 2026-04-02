package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/yeasy/ask/internal/cache"
	"github.com/yeasy/ask/internal/config"
)

// ConfigInfo represents configuration for API responses
type ConfigInfo struct {
	Version     string              `json:"version"`
	SkillsDir   string              `json:"skills_dir"`
	Agents      []string            `json:"agents"`
	ToolTargets []config.ToolTarget `json:"tool_targets"`
	GlobalDir   string              `json:"global_dir"`
	ProjectRoot string              `json:"project_root"`
	Initialized bool                `json:"initialized"`
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.cwdMu.RLock()
	defer s.cwdMu.RUnlock()

	cfg, err := config.LoadConfig()
	initialized := true
	if err != nil {
		if os.IsNotExist(err) {
			initialized = false
			def := config.DefaultConfig()
			cfg = &def
		} else {
			jsonError(w, "failed to load configuration", http.StatusInternalServerError)
			return
		}
	}

	info := ConfigInfo{
		Version:     s.version,
		SkillsDir:   cfg.GetSkillsDir(),
		Agents:      config.GetSupportedAgentNames(),
		ToolTargets: cfg.GetToolTargets(),

		GlobalDir:   config.GetGlobalSkillsDir(),
		Initialized: initialized,
	}

	// Get current working directory for ProjectRoot
	if cwd, err := os.Getwd(); err == nil {
		info.ProjectRoot = cwd
	}

	jsonResponse(w, info)
}

func (s *Server) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitRequestBody(w, r)
	if !requireJSONContentType(w, r) {
		return
	}

	s.cwdMu.RLock()
	defer s.cwdMu.RUnlock()

	// Clear Repos Cache
	reposCache, err := cache.New(cache.GetReposCacheDir(), 0)
	if err != nil {
		log.Printf("failed to access repos cache: %v", err)
		jsonError(w, "Failed to access repos cache", http.StatusInternalServerError)
		return
	}
	if err := reposCache.Clear(); err != nil {
		log.Printf("failed to clear repos cache: %v", err)
		jsonError(w, "Failed to clear repos cache", http.StatusInternalServerError)
		return
	}

	// Assume skills cache might be similar if needed, but for now repos cache is the main one
	// or we can clear the whole cache directory if preferred.
	// The cache package allows creating a cache instance on a dir.
	// Let's assume verifying cache clearing via repos cache is sufficient or we create a general one.

	jsonResponse(w, map[string]string{"status": "success", "message": "Cache cleared"})
}

func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limitRequestBody(w, r)
	if !requireJSONContentType(w, r) {
		return
	}

	var req struct {
		Agent       string `json:"agent"`
		Enabled     bool   `json:"enabled"`
		SkillsDir   string `json:"skills_dir"`
		ProjectRoot string `json:"project_root"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// If ProjectRoot is being changed, acquire the write lock upfront to
	// avoid a TOCTOU gap between reading the config and calling os.Chdir.
	if req.ProjectRoot != "" {
		s.cwdMu.Lock()
		defer s.cwdMu.Unlock()
	} else {
		s.cwdMu.RLock()
		defer s.cwdMu.RUnlock()
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		if os.IsNotExist(err) {
			// Initialize default if missing
			def := config.DefaultConfig()
			cfg = &def
		} else {
			log.Printf("Failed to load config: %v", err)
			jsonError(w, "Failed to load configuration", http.StatusInternalServerError)
			return
		}
	}

	updated := false

	// Handle project_root update first, as it changes the context for everything else
	if req.ProjectRoot != "" {
		// Sanitize and restrict path
		cleanRoot, pathErr := sanitizeAndRestrictPath(req.ProjectRoot)
		if pathErr != nil {
			jsonError(w, pathErr.Error(), http.StatusBadRequest)
			return
		}
		req.ProjectRoot = cleanRoot

		// Validate the path exists before changing
		if info, err := os.Stat(req.ProjectRoot); err != nil || !info.IsDir() {
			jsonError(w, "Invalid project root: not a valid directory", http.StatusBadRequest)
			return
		}
		if err := os.Chdir(req.ProjectRoot); err != nil {
			log.Printf("Failed to change directory to %s: %v", req.ProjectRoot, err)
			jsonError(w, "Failed to change project root", http.StatusBadRequest)
			return
		}
		// RELOAD config from the new directory to ensure we are editing the correct file
		// and not overwriting the new directory's config with the old one.
		newCfg, err := config.LoadConfig()
		if err != nil {
			if os.IsNotExist(err) {
				// Initialize default if missing in new location
				def := config.DefaultConfig()
				cfg = &def
			} else {
				jsonError(w, "failed to load config in new root", http.StatusInternalServerError)
				return
			}
		} else {
			cfg = newCfg
		}
		// We successfully switched context. Verification of initialized state will happen on next config fetch.
		updated = true

		// Update persistent global config
		if globalCfg, err := config.LoadGlobalConfig(); err == nil {
			globalCfg.LastProjectRoot = req.ProjectRoot
			if err := globalCfg.SaveGlobal(); err != nil {
				log.Printf("Failed to save global config: %v", err)
			}
		}
	}

	// Handle skills_dir update
	if req.SkillsDir != "" {
		cleanSkillsDir, pathErr := sanitizeAndRestrictPath(req.SkillsDir)
		if pathErr != nil {
			jsonError(w, pathErr.Error(), http.StatusBadRequest)
			return
		}
		cfg.SkillsDir = cleanSkillsDir
		updated = true
	}

	// Handle agent toggle
	if req.Agent != "" {
		// Normalize agent name
		req.Agent = strings.ToLower(req.Agent)

		// Update or add tool target
		found := false
		for i, t := range cfg.ToolTargets {
			if t.Name == req.Agent {
				cfg.ToolTargets[i].Enabled = req.Enabled
				found = true
				break
			}
		}
		if !found {
			// If not in explicit config, check defaults
			defaultTargets := config.DefaultToolTargets()
			for _, t := range defaultTargets {
				if t.Name == req.Agent {
					t.Enabled = req.Enabled
					cfg.ToolTargets = append(cfg.ToolTargets, t)
					found = true
					break
				}
			}
		}

		// If still not found (shouldn't happen for supported agents), create entry
		if !found {
			// Just try to find default settings for this agent type
			agentType, ok := config.ResolveAgentType(req.Agent)
			if ok {
				ac, ok := config.SupportedAgents[agentType]
				if ok {
					cfg.ToolTargets = append(cfg.ToolTargets, config.ToolTarget{
						Name:      req.Agent,
						SkillsDir: ac.ProjectDir,
						Enabled:   req.Enabled,
					})
				}
			}
		}
		updated = true
	}

	if updated {
		if err := cfg.Save(); err != nil {
			jsonError(w, "failed to save configuration", http.StatusInternalServerError)
			return
		}
	}

	jsonResponse(w, map[string]string{"status": "success", "message": "Configuration updated"})
}

// StatsInfo represents dashboard statistics
type StatsInfo struct {
	InstalledSkills int `json:"installed_skills"`
	ConfiguredRepos int `json:"configured_repos"`
	SyncedRepos     int `json:"synced_repos"`
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.cwdMu.RLock()
	defer s.cwdMu.RUnlock()

	cfg, err := config.LoadConfig()
	if err != nil {
		if os.IsNotExist(err) {
			def := config.DefaultConfig()
			cfg = &def
		} else {
			jsonError(w, "failed to load configuration", http.StatusInternalServerError)
			return
		}
	}

	// Count installed skills
	skillCount := len(cfg.SkillsInfo)
	shown := make(map[string]bool)
	for _, si := range cfg.SkillsInfo {
		shown[si.Name] = true
	}
	for _, name := range cfg.Skills {
		if !shown[name] {
			skillCount++
		}
	}

	// Count synced repos
	syncedCount := 0
	reposDir := cache.GetReposCacheDir()
	if entries, err := os.ReadDir(reposDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
				syncedCount++
			}
		}
	}

	stats := StatsInfo{
		InstalledSkills: skillCount,
		ConfiguredRepos: len(cfg.Repos),
		SyncedRepos:     syncedCount,
	}

	jsonResponse(w, stats)
}
