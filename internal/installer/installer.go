// Package installer provides functionality to install skills from various sources.
package installer

import (
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

// InstallOptions contains options for installing a skill
type InstallOptions struct {
	Global bool
	Agents []string
	Config *config.Config
}

// Install installs a single skill
func Install(input string, opts InstallOptions) error {
	if strings.TrimSpace(input) == "" {
		return fmt.Errorf("could not determine skill name: input is empty")
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

				reposCache, _ := cache.NewReposCache()
				if reposCache.HasRepo(repoName) {
					// Check for staleness (24 hours)
					// Only refresh if NOT in offline mode
					if !config.OfflineMode && reposCache.IsStale(repoName, 24*time.Hour) {
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
							reposCache, _ = cache.NewReposCache()
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
				fmt.Printf("Error: Multiple skills named '%s' found:\n", input)
				for _, m := range exactMatches {
					fmt.Printf("  - %s/%s\n", m.RepoName, m.Name)
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
				return Install(resolvedInput, opts)
			}
		}

		// 2. If not found in cache, try resolving as SkillHub slug
		if repoURL == "" {
			client := skillhub.NewClient()
			if resolved, err := client.Resolve(input); err == nil {
				fmt.Printf("Resolved SkillHub slug '%s' to '%s'\n", input, resolved)
				return Install(resolved, opts)
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
			agentType, _ := config.ResolveAgentType(agentName)
			dir, err := config.GetAgentSkillsDir(agentType, opts.Global)
			if err != nil {
				return fmt.Errorf("failed to get skills dir for agent %s: %w", agentName, err)
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
				wd, _ := os.Getwd()
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

	// Check if already installed
	allExist := true
	for _, dir := range targetDirs {
		destPath := filepath.Join(dir, skillName)
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			allExist = false
		}
	}
	if allExist {
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
	} else {
		if subDir != "" {
			err = git.InstallSubdir(repoURL, branch, subDir, tempSkillPath)
		} else {
			err = git.Clone(repoURL, tempSkillPath)
		}

		if err != nil {
			return fmt.Errorf("git operation failed: %w", err)
		}
	}

	// Checkout version
	if version != "" && subDir == "" {
		fmt.Printf("Checking out version %s...\n", version)
		if err := git.Checkout(tempSkillPath, version); err != nil {
			fmt.Printf("Warning: Failed to checkout version %s: %v\n", version, err)
		}
	}

	// Get commit hash
	var commitHash string
	if subDir == "" {
		commitHash, _ = git.GetCurrentCommit(tempSkillPath)
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
				// Sanitize name to be safe file path
				safeName := strings.ReplaceAll(meta.Name, "/", "-")
				safeName = strings.ReplaceAll(safeName, "\\", "-")
				safeName = strings.ReplaceAll(safeName, " ", "-")
				if safeName != "" {
					skillName = safeName
				}
			}
		}
	}
	if skillDescription == "" {
		skillDescription = "Skill installed from " + originalInput
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
	if _, err := os.Stat(centralPath); !os.IsNotExist(err) {
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

		if _, err := os.Stat(destPath); !os.IsNotExist(err) {
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
			fmt.Printf("Warning: Failed to update %s: %v\n", configFile, err)
		}

		// Update lock file
		lockFile, _ := config.LoadLockFileByScope(opts.Global)
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
			fmt.Printf("Warning: Failed to update %s: %v\n", lockFileName, err)
		}
	} else if !opts.Global {
		ui.Debug("Note: ask.yaml not found. Run 'ask init' to track dependencies.")
	}

	return nil
}
