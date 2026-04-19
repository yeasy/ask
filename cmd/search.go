package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/cache"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/github"
	"github.com/yeasy/ask/internal/repository"
	"github.com/yeasy/ask/internal/ui"
)

// searchCmd represents the search command
var searchCmd = &cobra.Command{
	Use:   "search [keyword]",
	Short: "Search for skills on GitHub",
	Long: `Search for skills matching the keyword. 

By default, uses local cache if available (fastest), otherwise fetches from remote.
Use --local to force local-only search.
Use --remote to force remote API search.`,
	Example: `  # Search (local-first, then remote)
  ask skill search pdf
  
  # Force local cache only (offline, fastest)
  ask skill search pdf --local
  
  # Force remote API (latest data)
  ask skill search pdf --remote`,
	Run: runSearch,
}

func runSearch(cmd *cobra.Command, args []string) {
	keyword := ""
	if len(args) > 0 {
		keyword = strings.Join(args, " ")
	}

	forceLocal, _ := cmd.Flags().GetBool("local")
	forceRemote, _ := cmd.Flags().GetBool("remote")
	minStars, _ := cmd.Flags().GetInt("min-stars")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Load config or use default
	cfg, err := config.LoadConfig()
	if err != nil {
		def := config.DefaultConfig()
		cfg = &def
	}

	// Build a set of installed skills for marking
	installedSkills := make(map[string]bool)
	for _, s := range cfg.Skills {
		installedSkills[s] = true
	}
	for _, s := range cfg.SkillsInfo {
		installedSkills[s.Name] = true
	}

	// When no keyword specified, show popular skills overview
	if keyword == "" && !forceLocal && !forceRemote {
		reposCache, err := cache.NewReposCache()
		if err == nil {
			skills, searchErr := reposCache.SearchSkills("")
			if searchErr != nil {
				ui.Debug(fmt.Sprintf("Cache search failed: %v", searchErr))
			}
			if len(skills) > 0 {
				fmt.Println("Popular Skills:")
				fmt.Println()

				w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
				_, _ = fmt.Fprintln(w, "NAME\tSOURCE\tINSTALLED\tDESCRIPTION")

				count := 0
				for _, s := range skills {
					if count >= 20 {
						break
					}
					installed := ""
					if installedSkills[s.Name] {
						installed = "✓"
					}
					desc := s.Description
					if len(desc) > 50 {
						desc = desc[:47] + "..."
					}
					_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, s.RepoName, installed, desc)
					count++
				}
				_ = w.Flush()

				fmt.Printf("\nShowing %d of %d available skills.\n", count, len(skills))
				fmt.Println("\nTip: ask search <keyword> to find specific skills")
				fmt.Println("     ask install <name>   to install a skill")
				return
			}
		}
	}

	var allRepos []github.Repository
	var searchErrors []string
	var searchSource string

	// Check local cache first (unless --remote is specified)
	if !forceRemote {
		reposCache, err := cache.NewReposCache()
		if err == nil {
			repoInfos, err := reposCache.LoadIndex()
			// Lazy Init: If cache is empty or index missing, sync automatically
			if err != nil || len(repoInfos) == 0 {
				ui.Debug("Initializing local skill database (this may take a minute)...")
				exe, err := os.Executable()
				if err == nil {
					// Run sync synchronously for the first time
					syncCtx, syncCancel := context.WithTimeout(context.Background(), 5*time.Minute)
					cmd := exec.CommandContext(syncCtx, exe, "repo", "sync")
					cmd.Stdout = os.Stdout
					cmd.Stderr = os.Stderr
					syncErr := cmd.Run()
					syncCancel()
					if syncErr != nil {
						ui.Warn(fmt.Sprintf("Initial sync failed: %v", syncErr))
					} else {
						// Reload index after sync
						var loadErr error
						repoInfos, loadErr = reposCache.LoadIndex()
						if loadErr != nil {
							ui.Debug(fmt.Sprintf("Failed to load index after sync: %v", loadErr))
						}
					}
				}
			} else {
				// Stale Check: If cache is older than 3 days, sync in background
				oldestSync := time.Now()
				for _, info := range repoInfos {
					if info.LastSyncedAt.IsZero() {
						oldestSync = time.Time{} // Treat as very old
						break
					}
					if info.LastSyncedAt.Before(oldestSync) {
						oldestSync = info.LastSyncedAt
					}
				}

				if time.Since(oldestSync) > 72*time.Hour {
					ui.Debug("Cache is stale, updating in background...")
					exe, err := os.Executable()
					if err == nil {
						// Background sync: start child process and wait to prevent zombie
						bgSyncCtx, bgSyncCancel := context.WithTimeout(context.Background(), 5*time.Minute)
						cmd := exec.CommandContext(bgSyncCtx, exe, "repo", "sync")
						if err := cmd.Start(); err == nil {
							go func() {
								defer bgSyncCancel()
								if waitErr := cmd.Wait(); waitErr != nil {
									ui.Debug(fmt.Sprintf("Background sync failed: %v", waitErr))
								}
							}()
						} else {
							bgSyncCancel()
						}
					}
				}
			}

			if len(repoInfos) > 0 || forceLocal {
				ui.Debug(fmt.Sprintf("Searching local cache for '%s'...", keyword))
				skills, searchErr := reposCache.SearchSkills(keyword)
				if searchErr != nil {
					ui.Debug(fmt.Sprintf("Cache search failed: %v", searchErr))
				}

				for _, skill := range skills {
					allRepos = append(allRepos, github.Repository{
						Name:        skill.Name,
						Description: skill.Description,
						Source:      skill.RepoName,
					})
				}
				searchSource = "local"

				if len(allRepos) > 0 || forceLocal {
					// Display results from local cache
					displaySearchResults(allRepos, installedSkills, searchSource, minStars, jsonOutput)
					if forceLocal && len(allRepos) == 0 {
						fmt.Println("\nTip: Run 'ask repo sync' to populate local cache.")
					}
					return
				}
				// No local results and not forced local, fall through to remote
			}
		}
	}

	// Remote search
	ui.Debug(fmt.Sprintf("Searching for skills matching '%s'...", keyword))
	searchSource = "remote"

	// Create progress bar for scanning sources
	bar := ui.NewProgressBar(len(cfg.Repos), "Scanning sources")

	// Search sources in parallel
	type searchResult struct {
		source string
		repos  []github.Repository
		err    error
	}

	results := make(chan searchResult, len(cfg.Repos))

	// Overall timeout for all remote search goroutines
	searchCtx, searchCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer searchCancel()

	// Limit concurrent goroutines to avoid excessive parallel requests
	sem := make(chan struct{}, 5)
	for _, repo := range cfg.Repos {
		go func(r config.Repo) {
			select {
			case sem <- struct{}{}:
			case <-searchCtx.Done():
				results <- searchResult{source: r.Name, err: searchCtx.Err()}
				return
			}
			defer func() { <-sem }()
			var repos []github.Repository
			var err error

			switch r.Type {
			case "topic":
				repos, err = github.SearchTopic(r.URL, keyword)
			case "dir":
				parts := strings.Split(r.URL, "/")
				if len(parts) >= 2 {
					owner := parts[0]
					repoName := parts[1]
					path := ""
					if len(parts) > 2 {
						path = strings.Join(parts[2:], "/")
					}
					repos, err = github.SearchDir(owner, repoName, path)

					// Filter client-side by keyword
					if err == nil && keyword != "" {
						var filtered []github.Repository
						for _, rp := range repos {
							if strings.Contains(strings.ToLower(rp.Name), strings.ToLower(keyword)) {
								filtered = append(filtered, rp)
							}
						}
						repos = filtered
					}
				}
			case "registry":
				repos, err = repository.FetchSkillsFromRegistry(r.URL, keyword)
			case "skillhub":
				repos, err = repository.FetchSkillsFromSkillHub(keyword, "")
			}

			// Set source name for each repo
			for i := range repos {
				repos[i].Source = r.Name
			}

			results <- searchResult{source: r.Name, repos: repos, err: err}
		}(repo)
	}

	for i := 0; i < len(cfg.Repos); i++ {
		result := <-results
		_ = bar.Add(1)
		if result.err != nil {
			searchErrors = append(searchErrors, fmt.Sprintf("%s: %v", result.source, result.err))
			continue
		}
		allRepos = append(allRepos, result.repos...)
	}

	fmt.Println()
	if len(searchErrors) > 0 {
		ui.Warn("Some sources failed to load:")
		for _, errMsg := range searchErrors {
			ui.Warn(fmt.Sprintf("  - %s", errMsg))
		}
	}

	displaySearchResults(allRepos, installedSkills, searchSource, minStars, jsonOutput)
}

func displaySearchResults(repos []github.Repository, installedSkills map[string]bool, source string, minStars int, jsonOutput bool) {
	// Filter repos if minStars > 0
	var displayRepos []github.Repository
	if minStars > 0 {
		for _, repo := range repos {
			if repo.StargazersCount >= minStars {
				displayRepos = append(displayRepos, repo)
			}
		}
	} else {
		displayRepos = repos
	}

	if jsonOutput {
		type searchResultJSON struct {
			Name        string `json:"name"`
			Source      string `json:"source"`
			Installed   bool   `json:"installed"`
			Stars       int    `json:"stars"`
			Description string `json:"description"`
		}

		var results []searchResultJSON
		for _, repo := range displayRepos {
			results = append(results, searchResultJSON{
				Name:        repo.Name,
				Source:      repo.Source,
				Installed:   installedSkills[repo.Name],
				Stars:       repo.StargazersCount,
				Description: repo.Description,
			})
		}

		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(results); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		}
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tSOURCE\tINSTALLED\tSTARS\tDESCRIPTION")
	for _, repo := range displayRepos {
		// Truncate description if too long
		desc := repo.Description
		if len(desc) > 40 {
			desc = desc[:37] + "..."
		}

		// Check if installed
		installed := ""
		if installedSkills[repo.Name] {
			installed = "✓"
		}

		// Format stars (use "-" for local or dir-based if actually 0, but dir-based now have stars)
		stars := fmt.Sprintf("%d", repo.StargazersCount)
		if repo.StargazersCount == 0 {
			stars = "-"
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", repo.Name, repo.Source, installed, stars, desc)
	}
	_ = w.Flush()

	fmt.Printf("\nFound %d skills", len(displayRepos))
	if minStars > 0 {
		fmt.Printf(" (filtered from %d results by stars >= %d)", len(repos), minStars)
	}
	fmt.Println(".")

	if source == "local" {
		ui.Debug("(from local cache - run 'ask repo sync' to update)")
	}
}

func registerSearchFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("local", false, "search only local cache (offline)")
	cmd.Flags().Bool("remote", false, "force remote API search")
	cmd.Flags().Int("min-stars", 0, "filter skills by minimum integer number of GitHub stars")
	cmd.Flags().Bool("json", false, "output results in JSON format")
}

func init() {
	skillCmd.AddCommand(searchCmd)
	registerSearchFlags(searchCmd)
}
