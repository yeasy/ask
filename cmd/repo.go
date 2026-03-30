package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/cache"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/repository"
	"github.com/yeasy/ask/internal/ui"
)

const maxResponseBodySize = 5 * 1024 * 1024 // 5MB

var (
	githubAPIBaseURL = "https://api.github.com"
	githubHTTPClient = &http.Client{Timeout: 10 * time.Second}
)

type githubRepoContent struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// repoCmd represents the repo command
var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Manage skill repositories",
	Long:  `Add, remove, or list skill repository sources.`,
	Example: `  # Add a custom repository
  ask repo add my-org/agent-skills
  
  # List all configured sources
  ask repo list
  
  # Remove a source
  ask repo remove my-org`,
}

// repoAddCmd represents the repo add command
var repoAddCmd = &cobra.Command{
	Use:   "add <owner/repo|URL>",
	Short: "Add a skill repository",
	Long: `Add a GitHub repository as a skill source.
Format: owner/repo or full URL

Examples:
  ask repo add anthropics/skills
  ask repo add browser-use/browser-use
  ask repo add openai/skills`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		input := args[0]

		// Parse username/repo format
		parts := strings.Split(input, "/")
		if len(parts) < 2 {
			fmt.Fprintln(os.Stderr, "Error: Invalid format. Use: owner/repo")
			os.Exit(1)
		}

		owner := parts[0]
		repo := parts[1]

		// Optional path within repo (if more than 2 parts)
		path := ""
		if len(parts) > 2 {
			path = strings.Join(parts[2:], "/")
		}

		ui.Debug(fmt.Sprintf("Validating repository %s/%s...", owner, repo))

		// Validate repository exists and is a valid skills repo
		valid, repoType, detectedPath := validateSkillsRepo(owner, repo, path)
		if !valid {
			fmt.Fprintln(os.Stderr, "Error: Repository does not appear to be a valid skills repository.")
			fmt.Fprintln(os.Stderr, "A valid skills repo should contain:")
			fmt.Fprintln(os.Stderr, "  - A 'skills/' directory with skill folders, or")
			fmt.Fprintln(os.Stderr, "  - Skills directly at root with SKILL.md files")
			os.Exit(1)
		}

		// Load or create config
		cfg, err := config.LoadConfig()
		if err != nil {
			if os.IsNotExist(err) {
				def := config.DefaultConfig()
				cfg = &def
			} else {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}
		}

		// Create repo entry
		repoName := repo
		repoURL := fmt.Sprintf("%s/%s", owner, repo)
		if detectedPath != "" {
			repoURL = fmt.Sprintf("%s/%s/%s", owner, repo, detectedPath)
		}

		// Check if repo already exists
		for _, r := range cfg.Repos {
			if r.URL == repoURL || r.Name == repoName {
				ui.Warn(fmt.Sprintf("Repo '%s' already exists.", repoName))
				return
			}
		}

		// Get private repo flags
		token, _ := cmd.Flags().GetString("token")
		baseURL, _ := cmd.Flags().GetString("base-url")
		private, _ := cmd.Flags().GetBool("private")

		// Auto-detect token from gh auth if private and no token provided
		if private && token == "" {
			token = detectGHToken()
		}

		// Add repo
		newRepo := config.Repo{
			Name:    repoName,
			Type:    repoType,
			URL:     repoURL,
			Token:   token,
			BaseURL: baseURL,
			Private: private,
		}
		cfg.Repos = append(cfg.Repos, newRepo)

		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✓ Added repo '%s' (type: %s)\n", repoName, repoType)
		fmt.Printf("  URL: %s\n", repoURL)

		// Handle --sync flag
		if sync, _ := cmd.Flags().GetBool("sync"); sync {
			fmt.Printf("Syncing repo '%s'...\n", repoName)
			// Trigger sync command logic
			// Locate the executable
			exe, err := os.Executable()
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to locate executable for sync: %v\n", err)
				return
			}

			// We can run sync in foreground here since user explicitly asked for it
			cmd := exec.Command(exe, "repo", "sync", repoName)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error syncing repo: %v\n", err)
				// Don't exit 1, as add was successful
			}
		} else {
			fmt.Println("Run 'ask repo sync' to download content.")
		}
	},
}

// repoListCmd represents the repo list command
var repoListCmd = &cobra.Command{
	Use:   "list [repo-name]",
	Short: "List configured repositories or skills in a repository",
	Long: `List all configured skill repositories.
If a repository name is provided, list all skills available in that repository.

Examples:
  ask repo list                # List all configured repositories
  ask repo list anthropics     # List skills in 'anthropics' repository`,
	Run: func(_ *cobra.Command, args []string) {
		cfg, err := config.LoadConfig()
		if err != nil {
			if os.IsNotExist(err) {
				def := config.DefaultConfig()
				cfg = &def
			} else {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
				os.Exit(1)
			}
		}

		// Case 1: List all repos (no args)
		if len(args) == 0 {
			if len(cfg.Repos) == 0 {
				fmt.Println("No repos configured.")
				return
			}

			// Load cached star counts
			starCache := make(map[string]int)
			reposCache, err := cache.NewReposCache()
			if err == nil {
				repoInfos, err := reposCache.LoadIndex()
				if err == nil {
					for _, info := range repoInfos {
						starCache[info.Name] = info.Stars
					}
				}
			}

			fmt.Println("Configured Repositories:")
			fmt.Println()
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tTYPE\tSTARS\tURL")
			_, _ = fmt.Fprintln(w, "----\t----\t-----\t---")
			for _, r := range cfg.Repos {
				// Build full GitHub URL
				var fullURL string
				var stars string
				if r.Type == "topic" {
					fullURL = fmt.Sprintf("https://github.com/topics/%s", r.URL)
					stars = "-" // Topics don't have star counts
				} else {
					fullURL = fmt.Sprintf("https://github.com/%s", r.URL)
					// Use cached star count
					repoName := buildRepoName(r.URL)
					if cachedStars, ok := starCache[repoName]; ok && cachedStars > 0 {
						stars = fmt.Sprintf("%d", cachedStars)
					} else {
						stars = "-"
					}
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Name, r.Type, stars, fullURL)
			}
			_ = w.Flush()
			fmt.Printf("\nTotal: %d repositories\n", len(cfg.Repos))
			fmt.Println("\nUse 'ask repo list <name>' to view skills in a repository.")
			fmt.Println("Run 'ask repo sync' to update star counts.")
			return
		}

		// Case 2: List skills from specific repo
		repoName := args[0]
		var targetRepo *config.Repo
		var matchedRepos []config.Repo

		for i := range cfg.Repos {
			r := cfg.Repos[i]
			// 1. Exact Name Match (Highest priority)
			if r.Name == repoName {
				targetRepo = &r
				matchedRepos = []config.Repo{r}
				break
			}

			// 2. URL/Path Logic Match
			// Check if the input looks like part of the URL
			// e.g. input "anthropics/skills" match url "anthropics/skills/skills"
			// or input "owner/repo" matches url "https://github.com/owner/repo"

			normalizedURL := strings.TrimSuffix(r.URL, ".git")
			normalizedURL = strings.TrimPrefix(normalizedURL, "https://github.com/")

			// Match if:
			// - Input is exactly the normalized URL (e.g. "owner/repo")
			// - Input matches the last segment(s) of the URL path (e.g. "skills" in "anthropics/skills")
			if normalizedURL == repoName || strings.HasSuffix(normalizedURL, "/"+repoName) {
				matchedRepos = append(matchedRepos, r)
			}
		}

		if targetRepo == nil {
			if len(matchedRepos) == 1 {
				targetRepo = &matchedRepos[0]
				// Use the found repo's name for display
				repoName = targetRepo.Name
			} else if len(matchedRepos) > 1 {
				fmt.Fprintf(os.Stderr, "Error: Multiple repositories match '%s':\n", repoName)
				for _, r := range matchedRepos {
					fmt.Fprintf(os.Stderr, "  - %s (URL: %s)\n", r.Name, r.URL)
				}
				fmt.Fprintln(os.Stderr, "Please specify the exact repository name.")
				os.Exit(1)
			}
		}

		if targetRepo == nil {
			fmt.Printf("Repo '%s' not found.\n", repoName)
			fmt.Println("Use 'ask repo list' to see configured repos.")
			if strings.Contains(repoName, "/") {
				fmt.Printf("Did you mean to add it? Run: ask repo add %s\n", repoName)
			}
			os.Exit(1)
		}

		ui.Debug(fmt.Sprintf("Fetching skills from '%s'...", repoName))

		// Create progress bar
		bar := ui.NewProgressBar(1, "Fetching")

		// Fetch skills
		repos, fetchErr := repository.FetchSkills(*targetRepo)

		_ = bar.Add(1)
		fmt.Println()

		if fetchErr != nil {
			fmt.Fprintf(os.Stderr, "Error fetching skills: %v\n", fetchErr)
			return
		}

		if len(repos) == 0 {
			fmt.Printf("No skills found in '%s'.\n", repoName)
			return
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tSTARS\tDESCRIPTION")
		for _, repo := range repos {
			// Truncate description if too long
			desc := repo.Description
			if len(desc) > 50 {
				desc = desc[:47] + "..."
			}

			// Format stars
			stars := fmt.Sprintf("%d", repo.StargazersCount)
			if repo.StargazersCount == 0 {
				stars = "-"
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", repo.Name, stars, desc)
		}
		_ = w.Flush()

		fmt.Printf("\nFound %d skills in '%s'.\n", len(repos), repoName)
	},
}

func githubAPIURL(path string) string {
	return strings.TrimRight(githubAPIBaseURL, "/") + path
}

func fetchRepoContents(owner, repo, path string) ([]githubRepoContent, error) {
	apiPath := fmt.Sprintf("/repos/%s/%s/contents", url.PathEscape(owner), url.PathEscape(repo))
	if path != "" {
		segments := strings.Split(strings.Trim(path, "/"), "/")
		escapedSegments := make([]string, 0, len(segments))
		for _, segment := range segments {
			if segment == "" {
				continue
			}
			escapedSegments = append(escapedSegments, url.PathEscape(segment))
		}
		if len(escapedSegments) > 0 {
			apiPath += "/" + strings.Join(escapedSegments, "/")
		}
	}

	req, err := http.NewRequest(http.MethodGet, githubAPIURL(apiPath), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ask-cli")

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	limitedBody := io.LimitReader(resp.Body, maxResponseBodySize)
	defer func() {
		_, _ = io.Copy(io.Discard, limitedBody)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var contents []githubRepoContent
	if err := json.NewDecoder(limitedBody).Decode(&contents); err != nil {
		return nil, err
	}

	return contents, nil
}

func hasSkillManifest(contents []githubRepoContent) bool {
	for _, item := range contents {
		if item.Type == "file" && strings.EqualFold(item.Name, "SKILL.md") {
			return true
		}
	}
	return false
}

func directoryLooksLikeSkills(owner, repo, basePath string, contents []githubRepoContent) bool {
	if hasSkillManifest(contents) {
		return true
	}

	for _, item := range contents {
		if item.Type != "dir" {
			continue
		}
		childPath := item.Name
		if basePath != "" {
			childPath = basePath + "/" + item.Name
		}
		childContents, err := fetchRepoContents(owner, repo, childPath)
		if err != nil {
			continue
		}
		if hasSkillManifest(childContents) {
			return true
		}
	}

	return false
}

// validateSkillsRepo checks if a GitHub repo is a valid skills repository
func validateSkillsRepo(owner, repo, path string) (bool, string, string) {
	// First, check if the repo exists
	req, err := http.NewRequest(http.MethodGet, githubAPIURL(fmt.Sprintf("/repos/%s/%s", url.PathEscape(owner), url.PathEscape(repo))), nil)
	if err != nil {
		return false, "", ""
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "ask-cli")

	resp, err := githubHTTPClient.Do(req)
	if err != nil {
		return false, "", ""
	}
	defer func() {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxResponseBodySize))
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusOK {
		return false, "", ""
	}

	// Check for common skills directory patterns
	pathsToCheck := []string{}
	if path != "" {
		pathsToCheck = append(pathsToCheck, path)
	} else {
		pathsToCheck = append(pathsToCheck, "skills", "src", "")
	}

	for _, p := range pathsToCheck {
		contents, err := fetchRepoContents(owner, repo, p)
		if err != nil {
			continue
		}
		if directoryLooksLikeSkills(owner, repo, p, contents) {
			return true, "dir", p
		}
	}

	return false, "", ""
}

// repoRemoveCmd represents the repo remove command
var repoRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a skill repository",
	Long:  `Remove a configured skill repository source.`,
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		name := args[0]
		cfg, err := config.LoadConfig()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		found := false
		var newRepos []config.Repo
		for _, r := range cfg.Repos {
			if r.Name == name {
				found = true
				continue
			}
			newRepos = append(newRepos, r)
		}

		if !found {
			fmt.Printf("Repo '%s' not found.\n", name)
			os.Exit(1)
		}

		cfg.Repos = newRepos
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "Error saving config: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Removed repo '%s'.\n", name)
	},
}

func init() {
	rootCmd.AddCommand(repoCmd)
	repoCmd.AddCommand(repoAddCmd)
	repoAddCmd.Flags().Bool("sync", false, "sync repository immediately after adding")
	repoAddCmd.Flags().String("token", "", "authentication token for private repositories")
	repoAddCmd.Flags().String("base-url", "", "GitHub Enterprise API base URL")
	repoAddCmd.Flags().Bool("private", false, "mark repository as private (auto-detects gh auth token)")
	repoCmd.AddCommand(repoListCmd)
	repoCmd.AddCommand(repoRemoveCmd)
}

// detectGHToken attempts to get a GitHub token from the gh CLI
func detectGHToken() string {
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
