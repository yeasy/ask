package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/github"
	"github.com/yeasy/ask/internal/installer"
	"github.com/yeasy/ask/internal/repository"
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:               "install [url...]",
	Aliases:           []string{"add", "i"},
	ValidArgsFunction: completeSkillNames,
	Short:             "Install one or more skills from git repositories",
	Long: `Download and install skills into agent-specific directories. 
You can provide full git URLs or GitHub shorthands (owner/repo).
You can also specify versions: owner/repo@v1.0.0

If no arguments are provided, it will attempt to restore skills from ask.lock or ask.yaml in the current directory.

Use --agent (-a) to specify target agents (e.g., claude, cursor, codex).
Multiple agents can be specified by repeating the flag.
If no agent is specified, skills are installed to .agent/skills/ by default.`,
	Example: `  # Install from GitHub shorthand
  ask skill install anthropics/pdf
  
  # Restore skills from ask.lock or ask.yaml
  ask skill install

  # Install to specific agents
  ask skill install pdf --agent claude --agent cursor
  ask skill install pdf -a claude -a cursor
  
  # Install globally for an agent
  ask skill install pdf --agent claude --global
  
  # Install multiple skills at once
  ask skill install pdf docx mcp-builder
  
  # Install specific version
  ask skill install browser-use/browser-use@v0.1.0
  
  # Install from subdirectory
  ask skill install anthropics/skills/skills/pdf
  
  # Install from GitHub browser URL
  ask skill install https://github.com/anthropics/skills/tree/main/skills/pdf
  
  # Install from full URL
  ask skill install https://github.com/browser-use/browser-use.git`,
	Args: cobra.MinimumNArgs(0), // Allow 0 args to support restoring from lock/yaml
	Run:  runInstall,
}

const maxInputLength = 255

func runInstall(cmd *cobra.Command, args []string) {
	// Check for offline mode
	if offline, _ := cmd.Flags().GetBool("offline"); offline || config.IsOffline() {
		fmt.Fprintln(os.Stderr, "Error: Cannot install skills in offline mode.")
		os.Exit(1)
	}

	// Check for global flag
	global, _ := cmd.Flags().GetBool("global")

	// Get agent targets
	agents, _ := cmd.Flags().GetStringSlice("agent")

	// Validate agent names
	for _, agent := range agents {
		if !config.IsValidAgent(agent) {
			fmt.Fprintf(os.Stderr, "Error: Unknown agent '%s'. Supported agents: %s\n",
				agent, strings.Join(config.GetSupportedAgentNames(), ", "))
			os.Exit(1)
		}
	}

	// Ensure project is initialized for non-global, non-agent-specific operations
	if !global && len(agents) == 0 {
		if !ensureInitialized() {
			return
		}
	}

	// Track installation results
	var succeeded, failed []string

	// Pre-process args to separate skills and agents
	var skillArgs []string
	agentFlagChanged := cmd.Flags().Changed("agent")

	for _, arg := range args {
		if agentFlagChanged && config.IsValidAgent(arg) {
			agents = append(agents, arg)
		} else {
			skillArgs = append(skillArgs, arg)
		}
	}

	// If no skills specified and no repo flag, try to restore from lock file or config file
	repoFlag, _ := cmd.Flags().GetString("repo")
	if len(skillArgs) == 0 && repoFlag == "" {
		// Only try restore if not in global mode (unless we want to support global restore later)
		// For now, let's support restore in local context primarily

		// 1. Try ask.lock first
		lockFile, err := config.LoadLockFile()
		if err == nil && len(lockFile.Skills) > 0 {
			fmt.Printf("Restoring %d skills from ask.lock...\n", len(lockFile.Skills))
			for _, s := range lockFile.Skills {
				// Use the URL from lock file as it contains the specific version/commit info if available
				// Or construct it from Name/Source?
				// The lock file stores: Name, URL, Version, Commit.
				// We should ideally use the URL or Name@Version if possible.
				// For now, using Name should trigger resolution, but might not be exact version
				// if we don't handle version pinning in install logic yet.
				// But wait, the lock file URL is what we want to re-install.
				if s.URL != "" {
					skillArgs = append(skillArgs, s.URL)
				} else {
					skillArgs = append(skillArgs, s.Name)
				}
			}
		} else {
			// 2. Try ask.yaml
			cfg, err := config.LoadConfig()
			if err == nil {
				count := 0
				// Add from new skills_info
				for _, s := range cfg.SkillsInfo {
					skillArgs = append(skillArgs, s.Name)
					count++
				}
				// Add from legacy skills list if not duplicate
				seen := make(map[string]bool, len(skillArgs))
				for _, existing := range skillArgs {
					seen[existing] = true
				}
				for _, s := range cfg.Skills {
					if !seen[s] {
						skillArgs = append(skillArgs, s)
						seen[s] = true
						count++
					}
				}

				if count > 0 {
					fmt.Printf("Restoring %d skills from ask.yaml...\n", count)
				}
			}
		}

		if len(skillArgs) == 0 {
			fmt.Fprintln(os.Stderr, "No skills specified and no ask.lock or ask.yaml found with skills.")
			os.Exit(1)
		}
	}

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		def := config.DefaultConfig()
		cfg = &def
	}

	var expandedArgs []string
	// Check for repo flag
	repoName, _ := cmd.Flags().GetString("repo")

	// If repo flag is set, fetch skills from that repo
	if repoName != "" {
		// Find the repo in config
		var targetRepo *config.Repo
		for i := range cfg.Repos {
			if cfg.Repos[i].Name == repoName {
				targetRepo = &cfg.Repos[i]
				break
			}
		}

		if targetRepo == nil {
			fmt.Fprintf(os.Stderr, "Error: Repository '%s' not found in configuration. Use 'ask repo list' to see available repositories.\n", repoName)
			os.Exit(1)
		}

		fmt.Printf("Fetching skills from repo '%s'...\n", repoName)

		var repos []github.Repository
		var err error

		if targetRepo.Type == "dir" {
			repos, err = repository.FetchSkillsViaGit(*targetRepo)
		} else {
			repos, err = repository.FetchSkills(*targetRepo)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to fetch skills from repo '%s': %v\n", repoName, err)
			os.Exit(1)
		}

		if len(repos) == 0 {
			fmt.Printf("No skills found in repo '%s'\n", repoName)
			os.Exit(0)
		}

		// Filter skills if args provided
		if len(skillArgs) > 0 {
			for _, wanted := range skillArgs {
				found := false
				for _, r := range repos {
					if r.Name == wanted {
						expandedArgs = append(expandedArgs, r.HTMLURL)
						found = true
						break
					}
				}
				if !found {
					fmt.Fprintf(os.Stderr, "Warning: Skill '%s' not found in repo '%s'\n", wanted, repoName)
					failed = append(failed, wanted)
				}
			}
		} else {
			// Install all skills from repo
			fmt.Printf("Found %d skills in repo '%s'. Queueing all for installation...\n", len(repos), repoName)
			for _, r := range repos {
				expandedArgs = append(expandedArgs, r.HTMLURL)
			}
		}
	} else {
		// Existing logic for mixed args (repo matched or skill matched)
		for _, input := range skillArgs {
			if len(input) > maxInputLength {
				fmt.Fprintf(os.Stderr, "Error: Input '%s...' is too long (max %d chars)\n", input[:20], maxInputLength)
				failed = append(failed, input)
				continue
			}

			// Check if input matches a configured repository name
			var targetRepo *config.Repo
			for i := range cfg.Repos {
				r := &cfg.Repos[i]
				if r.Name == input {
					targetRepo = r
					break
				}
				if strings.Contains(r.URL, input) {
					if strings.HasPrefix(r.URL, input) || strings.Contains(r.URL, "/"+input) {
						targetRepo = r
						break
					}
				}
			}

			if targetRepo != nil {
				fmt.Printf("Fetching skills from repo '%s'...\n", input)

				var repos []github.Repository
				var err error

				if targetRepo.Type == "dir" {
					repos, err = repository.FetchSkillsViaGit(*targetRepo)
					if err != nil {
						// Fallback to API-based fetch
						repos, err = repository.FetchSkills(*targetRepo)
					}
				} else {
					repos, err = repository.FetchSkills(*targetRepo)
				}

				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to fetch skills from repo '%s': %v\n", input, err)
					failed = append(failed, input)
					continue
				}

				if len(repos) == 0 {
					fmt.Printf("No skills found in repo '%s'\n", input)
					continue
				}

				fmt.Printf("Found %d skills in repo '%s'. Queueing for installation...\n", len(repos), input)
				for _, r := range repos {
					expandedArgs = append(expandedArgs, r.HTMLURL)
				}
			} else {
				expandedArgs = append(expandedArgs, input)
			}
		}
	}

	// Enterprise policy enforcement
	if cfg.Enterprise != nil {
		// Enforce lock file requirement
		if cfg.Enterprise.RequireLock {
			lockFile, lockErr := config.LoadLockFile()
			if lockErr != nil || len(lockFile.Skills) == 0 {
				fmt.Fprintln(os.Stderr, "Enterprise policy: ask.lock is required. Run 'ask lock-install' instead.")
				os.Exit(1)
			}
		}

		// Enforce allowed sources
		if len(cfg.Enterprise.AllowedSources) > 0 {
			var blocked []string
			for _, input := range expandedArgs {
				if !config.IsSourceAllowed(input, cfg.Enterprise.AllowedSources) {
					blocked = append(blocked, input)
				}
			}
			if len(blocked) > 0 {
				fmt.Fprintf(os.Stderr, "Enterprise policy: the following sources are not allowed:\n")
				for _, b := range blocked {
					fmt.Fprintf(os.Stderr, "  - %s\n", b)
				}
				fmt.Fprintf(os.Stderr, "Allowed sources: %s\n", strings.Join(cfg.Enterprise.AllowedSources, ", "))
				os.Exit(1)
			}
		}
	}

	skipScore, _ := cmd.Flags().GetBool("skip-score")
	minScore, _ := cmd.Flags().GetString("min-score")

	opts := installer.InstallOptions{
		Global:    global,
		Agents:    agents,
		Config:    cfg,
		SkipScore: skipScore,
		MinScore:  minScore,
	}

	// Install each expanded skill
	for _, input := range expandedArgs {
		err := installer.Install(input, opts)
		if err != nil {
			failed = append(failed, input)
			fmt.Fprintf(os.Stderr, "Failed to install %s: %v\n", input, err)
		} else {
			succeeded = append(succeeded, input)
		}
	}

	// Print summary
	if len(expandedArgs) > 1 {
		fmt.Println()
		fmt.Println("Installation Summary:")

		var targetDisplay string
		if len(agents) > 0 {
			targetDisplay = strings.Join(agents, ", ")
		} else if global {
			targetDisplay = "global"
		} else {
			wd, wdErr := os.Getwd()
			detected := []config.ToolTarget{}
			if wdErr == nil {
				detected = config.DetectExistingToolDirs(wd)
			}
			if len(detected) > 0 {
				var names []string
				for _, t := range detected {
					names = append(names, t.Name)
				}
				targetDisplay = strings.Join(names, ", ")
			} else {
				targetDisplay = ".agent/skills"
			}
		}

		if len(succeeded) > 0 {
			fmt.Printf("  ✓ Succeeded: %d (%s) -> to: %s\n", len(succeeded), strings.Join(succeeded, ", "), targetDisplay)
		}
		if len(failed) > 0 {
			fmt.Printf("  ✗ Failed: %d (%s)\n", len(failed), strings.Join(failed, ", "))
		}
	}

	if len(failed) > 0 {
		os.Exit(1)
	}
}

func registerInstallFlags(cmd *cobra.Command) {
	cmd.Flags().StringSliceP("agent", "a", []string{}, "Target agent(s) to install for (e.g. claude, cursor)")
	cmd.Flags().StringP("repo", "r", "", "Install skill(s) from a specific repository")
	cmd.Flags().Bool("skip-score", false, "Skip trust score check before installing")
	cmd.Flags().String("min-score", "D", "Minimum acceptable trust grade (A/B/C/D/F)")
}

func init() {
	skillCmd.AddCommand(installCmd)
	registerInstallFlags(installCmd)
}
