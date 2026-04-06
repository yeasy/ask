package cmd

import (
	"fmt"
	"os"

	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/ui"
)

var (
	cfgFile  string
	logLevel string
)

// Custom help template with subcommand details at the end
var rootHelpTemplate = `ASK v` + Version + `
{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}
Skill Commands (ask skill <command>):
  search      Search for skills across all sources
  install     Install one or more skills
  uninstall   Remove an installed skill
  list        List installed skills
  info        Show detailed skill information
  update      Update skills to latest versions
  outdated    Check for available updates
  check       Check a skill for security issues
  score       Compute trust score for a skill
  test        Run validation checks on a skill
  create      Create a new skill template
  publish     Validate and prepare skill for publishing
  prompt      Generate XML skill listing for agent prompts

Repository Commands (ask repo <command>):
  list        List configured repositories or skills in a repo
  add         Add a custom skill repository
  remove      Remove a repository
  sync        Clone/update repos to local cache (~/.ask/repos/)

System Commands:
  doctor       Diagnose and report on ASK health
  serve        Start web UI for visual skill management
  audit        Generate security audit report
  lock-install Install exact versions from ask.lock
  init         Initialize ASK project configuration
  benchmark    Run performance benchmarks
  quickstart   Install recommended skill packs
  version      Show current version

Supported Agents: %s
`

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "ask",
	Short: "The package manager for AI Agent skills",
	Long: `The package manager for AI Agent skills.

Discover, install, and manage capabilities for your AI Agents (Claude, Cursor, 
Codex, etc.) with a familiar CLI experience, just like Homebrew or npm.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
	// Version: "1.0.0",
	PersistentPreRun: func(_ *cobra.Command, _ []string) {
		ui.Setup(logLevel)
	},
}

// Version is the current version of the application
const Version = "1.9.5"

// Top-level aliases (Docker-style)
var installRootCmd = &cobra.Command{
	Use:     "install [url...]",
	Aliases: []string{"add", "i"},
	Short:   "Install one or more skills (alias for 'skill install')",
	Long:    "Download and install skills into agent-specific directories.\nThis is a shortcut for 'ask skill install'.",
	Example: installCmd.Example, // Reuse example from original command
	Args:    installCmd.Args,    // Reuse args validation
	Run:     runInstall,
}

var searchRootCmd = &cobra.Command{
	Use:     "search [keyword]",
	Short:   "Search for skills on GitHub (alias for 'skill search')",
	Long:    "Search for skills matching the keyword.\nThis is a shortcut for 'ask skill search'.",
	Example: searchCmd.Example,
	Run:     runSearch,
}

var listRootCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed skills (alias for 'skill list')",
	Long:  "List all skills currently installed.\nThis is a shortcut for 'ask skill list'.",
	Run:   runList,
}

var uninstallRootCmd = &cobra.Command{
	Use:     "uninstall [skill-name]",
	Short:   "Uninstall a skill (alias for 'skill uninstall')",
	Long:    "Remove a skill from the skills directory.\nThis is a shortcut for 'ask skill uninstall'.",
	Example: uninstallCmd.Example,
	Args:    uninstallCmd.Args,
	Run:     uninstallCmd.Run,
}

var checkRootCmd = &cobra.Command{
	Use:   "check [skill-path]",
	Short: "Check a skill for security issues (alias for 'skill check')",
	Long:  "Analyze a skill directory for potential security risks.\nThis is a shortcut for 'ask skill check'.",
	Run:   runCheck,
}

var syncRootCmd = &cobra.Command{
	Use:     "sync [repo-name]",
	Short:   "Sync skill repositories (alias for 'repo sync')",
	Long:    "Clone or update skill repositories to local cache.\nThis is a shortcut for 'ask repo sync'.",
	Example: syncCmd.Example,
	Run:     syncCmd.Run,
}

var updateRootCmd = &cobra.Command{
	Use:     "update [skill-name]",
	Short:   "Update installed skills (alias for 'skill update')",
	Long:    "Update one or all installed skills to their latest versions.\nThis is a shortcut for 'ask skill update'.",
	Example: updateCmd.Example,
	Run:     updateCmd.Run,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Set custom help template to show subcommand details at the end
	// Dynamically generate supported agents list
	agents := config.GetSupportedAgentNames()
	// Join agents with comma
	agentList := strings.Join(agents, ", ")
	rootCmd.SetHelpTemplate(fmt.Sprintf(rootHelpTemplate, agentList))

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ./ask.yaml)")
	rootCmd.PersistentFlags().Bool("offline", false, "run in offline mode (no network requests)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().BoolP("global", "g", false, "use global installation (~/.ask/skills)")

	// Register top-level aliases
	rootCmd.AddCommand(installRootCmd)
	registerInstallFlags(installRootCmd)

	rootCmd.AddCommand(searchRootCmd)
	registerSearchFlags(searchRootCmd)

	rootCmd.AddCommand(listRootCmd)
	registerListFlags(listRootCmd)

	rootCmd.AddCommand(uninstallRootCmd)
	rootCmd.AddCommand(checkRootCmd)
	rootCmd.AddCommand(syncRootCmd)
	rootCmd.AddCommand(updateRootCmd)

	// No specific flags to register for uninstall root shim as it uses uninstallCmd.Run directly?
	// Actually uninstallCmd.Run uses flags so we should share flags definition or re-register.
	// Since uninstallCmd is in another file, we can't easily reuse 'registerUninstallFlags' unless we export it.
	// But uninstallCmd is exported. Let's see how registerListFlags works.
	// It's likely defined in list.go.
	// We should probably just copy flags setup here.
	uninstallRootCmd.Flags().AddFlagSet(uninstallCmd.Flags())
	checkRootCmd.Flags().AddFlagSet(checkCmd.Flags())
	// syncCmd doesn't have specific flags, but it might in future.
	syncRootCmd.ValidArgsFunction = completeRepoNames
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if vid := rootCmd.PersistentFlags().Lookup("offline"); vid != nil && vid.Changed {
		if val, _ := rootCmd.PersistentFlags().GetBool("offline"); val {
			config.SetOffline(true)
		}
	}
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".ask" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".ask")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		ui.Debug("Using config file: " + viper.ConfigFileUsed())
	}
}
