package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/cache"
	"github.com/yeasy/ask/internal/config"
)

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for ASK.

To load completions:

Bash:
  $ source <(ask completion bash)
  
  # To load completions for each session, execute once:
  # Linux:
  $ ask completion bash > /etc/bash_completion.d/ask
  # macOS:
  $ ask completion bash > $(brew --prefix)/etc/bash_completion.d/ask

Zsh:
  # If shell completion is not already enabled in your environment,
  # you will need to enable it. You can execute the following once:
  $ echo "autoload -U compinit; compinit" >> ~/.zshrc
  
  # To load completions for each session, execute once:
  $ ask completion zsh > "${fpath[1]}/_ask"
  
  # You will need to start a new shell for this setup to take effect.

Fish:
  $ ask completion fish | source
  
  # To load completions for each session, execute once:
  $ ask completion fish > ~/.config/fish/completions/ask.fish

PowerShell:
  PS> ask completion powershell | Out-String | Invoke-Expression
  
  # To load completions for every new session, run:
  PS> ask completion powershell > ask.ps1
  # and source this file from your PowerShell profile.
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		switch args[0] {
		case "bash":
			err = cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			err = cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			err = cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			err = cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating completion: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}

// completeSkillNames provides completion for skill names from local cache
// Used by: install command
func completeSkillNames(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var suggestions []string

	// Try to get skills from local cache
	reposCache, err := cache.NewReposCache()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	// Search all cached repos
	skills, err := reposCache.SearchSkills("")
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	toCompleteLower := strings.ToLower(toComplete)
	for _, skill := range skills {
		if toComplete == "" || strings.HasPrefix(strings.ToLower(skill.Name), toCompleteLower) {
			// Add both plain name and repo/name format
			suggestions = append(suggestions, skill.Name)
			fullName := skill.RepoName + "/" + skill.Name
			if toComplete == "" || strings.HasPrefix(strings.ToLower(fullName), toCompleteLower) {
				suggestions = append(suggestions, fullName)
			}
		}
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completeInstalledSkills provides completion for installed skill names
// Used by: uninstall, info commands
func completeInstalledSkills(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var suggestions []string

	// Get installed skills from project config
	cfg, err := config.LoadConfig()
	if err == nil && cfg != nil {
		toCompleteLower := strings.ToLower(toComplete)
		for _, skill := range cfg.SkillsInfo {
			if toComplete == "" || strings.HasPrefix(strings.ToLower(skill.Name), toCompleteLower) {
				suggestions = append(suggestions, skill.Name)
			}
		}
		for _, skill := range cfg.Skills {
			if toComplete == "" || strings.HasPrefix(strings.ToLower(skill), toCompleteLower) {
				suggestions = append(suggestions, skill)
			}
		}
	}

	// Also check global config
	globalCfg, err := config.LoadConfigByScope(true)
	if err == nil && globalCfg != nil {
		toCompleteLower := strings.ToLower(toComplete)
		for _, skill := range globalCfg.SkillsInfo {
			if toComplete == "" || strings.HasPrefix(strings.ToLower(skill.Name), toCompleteLower) {
				suggestions = append(suggestions, skill.Name)
			}
		}
		for _, skill := range globalCfg.Skills {
			if toComplete == "" || strings.HasPrefix(strings.ToLower(skill), toCompleteLower) {
				suggestions = append(suggestions, skill)
			}
		}
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completeAgentNames provides completion for agent names
// Used by: install, uninstall, list --agent flag
func completeAgentNames(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	agents := config.GetSupportedAgentNames()
	var suggestions []string

	toCompleteLower := strings.ToLower(toComplete)
	for _, agent := range agents {
		if toComplete == "" || strings.HasPrefix(strings.ToLower(agent), toCompleteLower) {
			suggestions = append(suggestions, agent)
		}
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}

// completeRepoNames provides completion for repository names
// Used by: repo sync command
func completeRepoNames(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	var suggestions []string

	cfg, err := config.LoadConfig()
	if err != nil {
		def := config.DefaultConfig()
		cfg = &def
	}

	toCompleteLower := strings.ToLower(toComplete)
	for _, repo := range cfg.Repos {
		if toComplete == "" || strings.HasPrefix(strings.ToLower(repo.Name), toCompleteLower) {
			suggestions = append(suggestions, repo.Name)
		}
	}

	return suggestions, cobra.ShellCompDirectiveNoFileComp
}
