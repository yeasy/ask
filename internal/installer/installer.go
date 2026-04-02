// Package installer provides functionality to install skills from various sources.
package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/yeasy/ask/internal/cache"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/filesystem"
	"github.com/yeasy/ask/internal/git"
	"github.com/yeasy/ask/internal/github"
	"github.com/yeasy/ask/internal/repository"
	"github.com/yeasy/ask/internal/skill"
	"github.com/yeasy/ask/internal/skillhub"
	"github.com/yeasy/ask/internal/ui"
)

// maxInstallDepth is the maximum recursion depth for Install to prevent circular resolution.
const maxInstallDepth = 3

// gitCloneTimeout is the maximum time allowed for a git clone operation.
const gitCloneTimeout = 5 * time.Minute

// gitOpTimeout is the maximum time allowed for lightweight git operations (checkout, rev-parse).
const gitOpTimeout = 30 * time.Second

// InstallOptions contains options for installing a skill
type InstallOptions struct {
	Global    bool
	Agents    []string
	Config    *config.Config
	SkipScore bool   // Skip trust score check
	MinScore  string // Minimum acceptable grade (A/B/C/D/F), default "D"
	depth     int    // current recursion depth (internal use only)
}

// Install installs a single skill
func Install(input string, opts InstallOptions) error {
	if strings.TrimSpace(input) == "" {
		return fmt.Errorf("could not determine skill name: input is empty")
	}

	// Reject path traversal attempts early, before reaching any git/network operations
	if strings.Contains(input, "..") {
		return fmt.Errorf("invalid input: path traversal not allowed")
	}

	if opts.depth >= maxInstallDepth {
		return fmt.Errorf("install recursion limit reached (max %d): possible circular resolution for %q", maxInstallDepth, input)
	}

	// Parse version if specified (skill@version)
	var version string
	originalInput := input
	if idx := strings.LastIndex(input, "@"); idx != -1 && !strings.HasPrefix(input, "git@") {
		version = input[idx+1:]
		input = input[:idx]
	}

	var repoURL, subDir, skillName, branch, localSourcePath string

	// First, check if it's a GitHub browser URL with /tree/
	if parsedURL, parsedBranch, parsedSubDir, parsedName, ok := github.ParseBrowserURL(input); ok {
		repoURL = parsedURL
		branch = parsedBranch
		subDir = parsedSubDir
		skillName = parsedName
	} else {
		// Check if it's a direct URL or shorthand
		isURL := strings.HasPrefix(input, "http") || strings.HasPrefix(input, "git@")

		if !isURL {
			parts := strings.Split(input, "/")
			if len(parts) > 2 {
				// It's a subdirectory install: owner/repo/path/to/skill
				owner := parts[0]
				repo := parts[1]
				subDir = strings.Join(parts[2:], "/")
				skillName = parts[len(parts)-1]
				repoURL = fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
			} else if len(parts) >= 2 {
				// Potential RepoName/SkillName from cache (e.g. "anthropics-skills/browser-use")
				// or Standard install: owner/repo (e.g. "browser-use/browser-use")

				foundInCache := false

				repoName := parts[0]
				skillNamePart := parts[1]

				reposCache, cacheErr := cache.NewReposCache()
				if cacheErr == nil && reposCache != nil && reposCache.HasRepo(repoName) {
					// Check for staleness (24 hours)
					// Only refresh if NOT in offline mode
					if !config.IsOffline() && reposCache.IsStale(repoName, 24*time.Hour) {
						ui.Debug(fmt.Sprintf("Repo '%s' is stale, refreshing...", repoName))
						// We need to fetch the repo URL from config to refresh
						var refreshURL string
						if opts.Config != nil {
							for _, r := range opts.Config.Repos {
								if r.Name == repoName {
									refreshURL = r.URL
									break
								}
							}
						}
						// If URL found, trigger fetch (which pulls/syncs)
						if refreshURL != "" {
							// Use repository package to fetch/sync
							// Note: We need to import "github.com/yeasy/ask/internal/repository" if not already imported
							// It is imported as "github.com/yeasy/ask/internal/repository"
							// But we need to construct a Repo struct
							refreshRepo := config.Repo{Name: repoName, URL: refreshURL, Type: "dir"} // Type guess, but FetchSkills handles both kinda?
							// Actually repository.FetchSkills takes a Repo.
							// Let's use it.
							_, _ = repository.FetchSkills(refreshRepo)
							// Re-load cache after sync
							newCache, cacheErr := cache.NewReposCache()
							if cacheErr == nil {
								reposCache = newCache
							}
						}
					}

					// Repo exists in cache, check if skill exists in it
					skills, err := reposCache.ListSkills(repoName)
					if err == nil {
						for _, s := range skills {
							if s.Name == skillNamePart {
								ui.Debug(fmt.Sprintf("Found skill '%s' in cached repo '%s'", skillNamePart, repoName))

								// Resolve URL and subDir
								repoInfos, err := reposCache.LoadIndex()
								if err == nil {
									for _, info := range repoInfos {
										if info.Name == s.RepoName {
											repoURL = info.URL

											// If URL is missing in index (bug fix), lookup from config
											if repoURL == "" && opts.Config != nil {
												for _, r := range opts.Config.Repos {
													// Calculate derived name as used in sync
													derivedName := r.Name
													if !strings.HasPrefix(r.URL, "http") {
														parts := strings.Split(r.URL, "/")
														if len(parts) >= 2 {
															derivedName = parts[0] + "-" + parts[1]
														}
													} else {
														derivedName = strings.ReplaceAll(r.URL, "/", "-")
													}

													if r.Name == s.RepoName || derivedName == s.RepoName {
														if !strings.HasPrefix(r.URL, "http") {
															parts := strings.Split(r.URL, "/")
															if len(parts) >= 2 {
																repoURL = fmt.Sprintf("https://github.com/%s/%s.git", parts[0], parts[1])
															}
														} else {
															repoURL = r.URL
														}
														break
													}
												}
											}

											localSourcePath = s.Path
											rel, err := filepath.Rel(info.LocalPath, s.Path)
											if err == nil && rel != "." {
												subDir = rel
											}
											skillName = s.Name
											foundInCache = true
											break
										}
									}
								}
								if foundInCache {
									break
								}
							}
						}
					}
				}

				if !foundInCache {
					// Fallback: Check if it's a known repo in config
					configMatch := false
					if opts.Config != nil {
						for _, r := range opts.Config.Repos {
							if r.Name == parts[0] {
								// Found matching repo config
								if !strings.HasPrefix(r.URL, "http") {
									repoParts := strings.Split(r.URL, "/")
									if len(repoParts) >= 2 {
										repoURL = fmt.Sprintf("https://github.com/%s/%s.git", repoParts[0], repoParts[1])
									}
								} else {
									repoURL = r.URL
								}

								baseSubDir := ""
								if !strings.HasPrefix(r.URL, "http") {
									repoParts := strings.Split(r.URL, "/")
									if len(repoParts) > 2 {
										baseSubDir = strings.Join(repoParts[2:], "/")
									}
								}

								if baseSubDir != "" {
									subDir = filepath.Join(baseSubDir, parts[1])
								} else {
									subDir = parts[1]
								}

								skillName = parts[1]
								configMatch = true

								// We skip the background sync trigger here to simplify the installer package logic
								// It belonged more to the CLI coordination layer, or could be passed as a callback
								break
							}
						}
					}

					if !configMatch {
						// Standard install: owner/repo
						repoURL = "https://github.com/" + input
						urlParts := strings.Split(strings.TrimSuffix(repoURL, ".git"), "/")
						skillName = urlParts[len(urlParts)-1]
					}
				}
			}
		} else {
			// It's a URL
			repoURL = input
			urlParts := strings.Split(strings.TrimSuffix(repoURL, ".git"), "/")
			skillName = urlParts[len(urlParts)-1]
		}
	}

	// SkillHub Slug Resolution
	if !strings.Contains(input, "/") && !strings.HasPrefix(input, "http") && !strings.HasPrefix(input, "git") {
		// 1. Try resolving from local cache first
		reposCache, err := cache.NewReposCache()
		if err == nil {
			skills, _ := reposCache.SearchSkills(input)

			// Find all exact matches
			var exactMatches []cache.SkillEntry
			for _, s := range skills {
				if s.Name == input {
					exactMatches = append(exactMatches, s)
				}
			}

			if len(exactMatches) > 1 {
				// Ambiguous!
				fmt.Fprintf(os.Stderr, "Error: Multiple skills named '%s' found:\n", input)
				for _, m := range exactMatches {
					fmt.Fprintf(os.Stderr, "  - %s/%s\n", m.RepoName, m.Name)
				}
				return fmt.Errorf("ambiguous skill name '%s'. Please specify the repository like 'RepoName/SkillName'", input)
			} else if len(exactMatches) == 1 {
				// Single match
				s := exactMatches[0]
				ui.Debug(fmt.Sprintf("Found skill '%s' in local cache (repo: %s)", input, s.RepoName))

				// Resolve repo URL logic similar to above (simplified for brevity, should ideally be shared)
				// For now recursing if resolving to a full path
				// But we need repoURL etc. Let's reuse the cache lookup logic structure or just recurse if we can construct a valid input
				// But recursion with resolved names might be tricky if they are not standard format.
				// Let's duplicate the lookup logic for now to ensure correctness

				// ... [Snippet for cache lookup omitted/simplified as we did it above for owner/repo] ...
				// Actually, the above block handles "Repo/Skill". This block handles "Skill" only.
				// If we find it, we can construct "Repo/Skill" and recurse?
				resolvedInput := fmt.Sprintf("%s/%s", exactMatches[0].RepoName, exactMatches[0].Name)
				recurseOpts := opts
				recurseOpts.depth = opts.depth + 1
				return Install(resolvedInput, recurseOpts)
			}
		}

		// 2. If not found in cache, try resolving as SkillHub slug
		if repoURL == "" {
			client := skillhub.NewClient()
			if resolved, err := client.Resolve(input); err == nil {
				fmt.Printf("Resolved SkillHub slug '%s' to '%s'\n", input, resolved)
				recurseOpts := opts
				recurseOpts.depth = opts.depth + 1
				return Install(resolved, recurseOpts)
			}
		}
	}

	// Use branch from version if not set
	if branch == "" && version != "" {
		branch = version
	}

	// Determine target directories based on agents
	var targetDirs []string
	var scopeLabel string

	if len(opts.Agents) > 0 {
		for _, agentName := range opts.Agents {
			agentType, ok := config.ResolveAgentType(agentName)
			if !ok {
				return fmt.Errorf("unknown agent: %s", agentName)
			}
			dir, err := config.GetAgentSkillsDir(agentType, opts.Global)
			if err != nil {
				return fmt.Errorf("failed to get skills dir for agent %s: %w", agentName, err)
			}
			if dir == "" {
				return fmt.Errorf("no skills directory configured for agent %s", agentName)
			}
			targetDirs = append(targetDirs, dir)
		}
		scopeLabel = strings.Join(opts.Agents, ", ")
		if opts.Global {
			scopeLabel += " (global)"
		}
	} else {
		if opts.Global {
			targetDirs = []string{config.GetSkillsDirByScope(true)}
			scopeLabel = "global"
		} else {
			if opts.Config != nil {
				wd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get working directory: %w", err)
				}
				targetDirs = opts.Config.GetActiveSkillsDirs(wd)
				scopeLabel = "detected targets"
			} else {
				targetDirs = []string{config.GetSkillsDirByScope(false)}
				scopeLabel = "project"
			}
		}
	}

	if skillName == "" || strings.TrimSpace(skillName) == "" {
		return fmt.Errorf("could not determine skill name from input '%s'", input)
	}

	fmt.Printf("Installing %s to %s...\n", skillName, scopeLabel)

	// Check if already installed (fast path using initial name;
	// re-checked after SKILL.md may override skillName)
	initialSkillName := skillName
	if allExist := checkAllExist(targetDirs, skillName); allExist {
		fmt.Printf("Skill %s is already installed in all target directories\n", skillName)
		return nil
	}

	// Clone to temp
	tempDir, err := os.MkdirTemp("", "ask-install-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	tempSkillPath := filepath.Join(tempDir, skillName)

	if localSourcePath != "" {
		if err := filesystem.CopyDir(localSourcePath, tempSkillPath); err != nil {
			return fmt.Errorf("failed to copy local skill: %w", err)
		}
	} else if repoURL == "" {
		return fmt.Errorf("could not resolve repository URL for %q", originalInput)
	} else {
		cloneCtx, cloneCancel := context.WithTimeout(context.Background(), gitCloneTimeout)
		defer cloneCancel()
		if subDir != "" {
			err = git.InstallSubdir(cloneCtx, repoURL, branch, subDir, tempSkillPath)
		} else {
			err = git.Clone(cloneCtx, repoURL, tempSkillPath)
		}

		if err != nil {
			return fmt.Errorf("git operation failed: %w", err)
		}
	}

	// Trust score check (before installing)
	if !opts.SkipScore {
		scoreResult, scoreErr := skill.ScoreSkill(tempSkillPath, nil)
		if scoreErr == nil {
			minGrade := skill.GradeD
			if opts.MinScore != "" {
				minGrade = skill.ScoreGrade(opts.MinScore)
			}
			if skill.GradeBelowThreshold(scoreResult.Grade, minGrade) {
				fmt.Fprintf(os.Stderr, "\n⚠ Trust score warning for %s: %.0f/100 (Grade %s)\n",
					skillName, scoreResult.TotalScore, string(scoreResult.Grade))
				fmt.Fprintf(os.Stderr, "  %s\n", scoreResult.Summary)
				for _, cat := range scoreResult.Categories {
					if cat.Score < 70 {
						fmt.Fprintf(os.Stderr, "  - %s: %.0f/100\n", cat.Name, cat.Score)
						for _, d := range cat.Deducts {
							fmt.Fprintf(os.Stderr, "    -%.*f %s\n", 0, d.Points, d.Reason)
						}
					}
				}
				fmt.Fprintf(os.Stderr, "\n  Use --skip-score to bypass this check.\n\n")
				return fmt.Errorf("skill %s scored below minimum grade %s (got %s)",
					skillName, string(minGrade), string(scoreResult.Grade))
			}
			if scoreResult.Grade != skill.GradeA {
				fmt.Printf("  Trust score: %.0f/100 (Grade %s)\n",
					scoreResult.TotalScore, string(scoreResult.Grade))
			}
		}
	}

	// Checkout version
	if version != "" && subDir == "" {
		fmt.Printf("Checking out version %s...\n", version)
		checkoutCtx, checkoutCancel := context.WithTimeout(context.Background(), gitOpTimeout)
		defer checkoutCancel()
		if err := git.Checkout(checkoutCtx, tempSkillPath, version); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to checkout version %s: %v\n", version, err)
		}
	}

	// Get commit hash
	var commitHash string
	if subDir == "" {
		commitCtx, commitCancel := context.WithTimeout(context.Background(), gitOpTimeout)
		defer commitCancel()
		commitHash, _ = git.GetCurrentCommit(commitCtx, tempSkillPath)
	}

	// Get metadata
	var skillDescription string
	if skill.FindSkillMD(tempSkillPath) {
		meta, err := skill.ParseSkillMD(tempSkillPath)
		if err == nil && meta != nil {
			if meta.Description != "" {
				skillDescription = meta.Description
			}
			// If SKILL.md defines a name, use it as the directory name
			// This is important for root-level skills where the repo name might differ
			if meta.Name != "" {
				// Sanitize name using allowlist: keep only safe characters
				safeName := strings.Map(func(r rune) rune {
					if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
						return r
					}
					return '-'
				}, meta.Name)
				// Collapse consecutive dashes and trim leading dots/dashes
				for strings.Contains(safeName, "--") {
					safeName = strings.ReplaceAll(safeName, "--", "-")
				}
				safeName = strings.TrimLeft(safeName, ".-")
				safeName = strings.TrimRight(safeName, ".-")
				// Limit length to prevent filesystem issues
				if len(safeName) > 100 {
					safeName = safeName[:100]
				}
				if safeName != "" && safeName != "." && safeName != ".." {
					skillName = safeName
				}
			}
		}
	}
	if skillDescription == "" {
		skillDescription = "Skill installed from " + originalInput
	}

	// Re-check "already installed" if SKILL.md changed the effective name
	if skillName != initialSkillName {
		if allExist := checkAllExist(targetDirs, skillName); allExist {
			fmt.Printf("Skill %s is already installed in all target directories\n", skillName)
			return nil
		}
	}

	// Environment setup
	// Check for .env.example and create .env if it doesn't exist
	envExamplePath := filepath.Join(tempSkillPath, ".env.example")
	envPath := filepath.Join(tempSkillPath, ".env")
	if _, err := os.Stat(envExamplePath); err == nil {
		if _, err := os.Stat(envPath); os.IsNotExist(err) {
			if err := filesystem.CopyFile(envExamplePath, envPath); err == nil {
				fmt.Printf("Created .env from .env.example\n")
			}
		}
	}

	// Central storage
	centralDir := config.DefaultSkillsDir
	if opts.Global {
		centralDir = config.GetSkillsDirByScope(true)
	}
	centralPath := filepath.Join(centralDir, skillName)

	sourceExists := false
	if _, err := os.Stat(centralPath); err == nil {
		sourceExists = true
	}

	if !sourceExists {
		if err := os.MkdirAll(centralDir, 0755); err != nil {
			return fmt.Errorf("failed to create central skills directory %s: %w", centralDir, err)
		}
		if err := filesystem.CopyDir(tempSkillPath, centralPath); err != nil {
			return fmt.Errorf("failed to copy skill to central storage %s: %w", centralPath, err)
		}
	}

	// Link to targets
	for _, dir := range targetDirs {
		destPath := filepath.Join(dir, skillName)
		if destPath == centralPath {
			continue
		}

		if _, err := os.Stat(destPath); err == nil {
			if filesystem.IsSymlink(destPath) {
				ui.Debug("  → Already linked in " + destPath)
			} else {
				ui.Debug("  → Already installed in " + destPath)
			}
			continue
		}

		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create skills directory %s: %w", dir, err)
		}

		if err := filesystem.CreateSymlinkOrCopy(centralPath, destPath); err != nil {
			return fmt.Errorf("failed to link skill to %s: %w", destPath, err)
		}

		if filesystem.IsSymlink(destPath) {
			ui.Debug("  → Linked to " + destPath)
		} else {
			ui.Debug("  → Copied to " + destPath + " (symlink not supported)")
		}
	}

	// Show sync summary
	if len(targetDirs) > 0 {
		fmt.Printf("\n✓ Installed %s\n", skillName)
		for _, dir := range targetDirs {
			// Try to resolve agent name from directory path
			agentName := resolveAgentFromDir(dir)
			if agentName != "" {
				fmt.Printf("  Synced to: %s (%s)\n", agentName, dir)
			} else {
				fmt.Printf("  Synced to: %s\n", dir)
			}
		}
		fmt.Println()
	}

	// Update config
	updatedCfg, err := config.LoadConfigByScope(opts.Global)
	if err == nil {
		skillInfo := config.SkillInfo{
			Name:        skillName,
			Description: skillDescription,
			URL:         repoURL,
		}
		if subDir != "" {
			if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
				skillInfo.URL = input
			} else {
				skillInfo.URL = fmt.Sprintf("https://github.com/%s", input)
			}
		}

		updatedCfg.AddSkillInfo(skillInfo)
		err = updatedCfg.SaveByScope(opts.Global)
		if err != nil {
			configFile := "ask.yaml"
			if opts.Global {
				configFile = "~/.ask/config.yaml"
			}
			fmt.Fprintf(os.Stderr, "Warning: Failed to update %s: %v\n", configFile, err)
		}

		// Update lock file
		lockFile, lockErr := config.LoadLockFileByScope(opts.Global)
		if lockErr != nil || lockFile == nil {
			lockFile = &config.LockFile{Version: 1, Skills: []config.LockEntry{}}
		}
		lockEntry := config.LockEntry{
			Name:        skillName,
			URL:         skillInfo.URL,
			Commit:      commitHash,
			Version:     version,
			InstalledAt: time.Now(),
		}
		lockFile.AddEntry(lockEntry)
		if err := lockFile.SaveByScope(opts.Global); err != nil {
			lockFileName := "ask.lock"
			if opts.Global {
				lockFileName = "~/.ask/ask.lock"
			}
			fmt.Fprintf(os.Stderr, "Warning: Failed to update %s: %v\n", lockFileName, err)
		}
	} else if !opts.Global {
		ui.Debug("Note: ask.yaml not found. Run 'ask init' to track dependencies.")
	}

	return nil
}

// checkAllExist returns true if skillName already exists in every target directory.
func checkAllExist(targetDirs []string, skillName string) bool {
	if len(targetDirs) == 0 {
		return false
	}
	for _, dir := range targetDirs {
		if _, err := os.Stat(filepath.Join(dir, skillName)); err != nil {
			return false
		}
	}
	return true
}

// resolveAgentFromDir tries to identify the agent name from a skills directory path
func resolveAgentFromDir(dir string) string {
	for _, agentName := range config.GetSupportedAgentNames() {
		agentType, ok := config.ResolveAgentType(agentName)
		if !ok {
			continue
		}
		agentCfg := config.SupportedAgents[agentType]
		if strings.HasSuffix(dir, agentCfg.ProjectDir) || strings.HasSuffix(dir, agentCfg.GlobalDir) {
			return agentName
		}
	}
	// Check default
	if strings.HasSuffix(dir, config.DefaultSkillsDir) {
		return "agent (default)"
	}
	return ""
}
