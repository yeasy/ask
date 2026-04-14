package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/installer"
	"github.com/yeasy/ask/internal/skill"
)

var lockInstallCmd = &cobra.Command{
	Use:   "lock-install",
	Short: "Install exact versions from ask.lock",
	Long: `Install skills at the exact versions specified in ask.lock.
Similar to 'npm ci', this ensures reproducible installations.

This command reads the lock file and installs each skill using the
recorded URL and version, ensuring consistent environments across
team members and CI/CD pipelines.`,
	Example: `  # Install from project lock file
  ask lock-install

  # Install from global lock file
  ask lock-install --global`,
	Run: func(cmd *cobra.Command, _ []string) {
		global, _ := cmd.Flags().GetBool("global")
		agents, _ := cmd.Flags().GetStringSlice("agent")
		check, _ := cmd.Flags().GetBool("check")

		lockFile, err := config.LoadLockFileByScope(global)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading lock file: %v\n", err)
			os.Exit(1)
		}

		if len(lockFile.Skills) == 0 {
			fmt.Println("No skills found in lock file.")
			return
		}

		// Check enterprise config enforcement
		cfg, err := config.LoadConfig()
		if err != nil {
			def := config.DefaultConfig()
			cfg = &def
		}

		if cfg.Enterprise != nil && cfg.Enterprise.RequireLock {
			// Validate lock file is not empty (already checked above)
			fmt.Println("Enterprise mode: lock file required ✓")
		}

		opts := installer.InstallOptions{
			Global: global,
			Agents: agents,
			Config: cfg,
		}

		fmt.Printf("Installing %d skills from ask.lock...\n\n", len(lockFile.Skills))

		var succeeded, failed int
		for _, entry := range lockFile.Skills {
			// Check allowed sources if enterprise config enforces it
			if cfg.Enterprise != nil && len(cfg.Enterprise.AllowedSources) > 0 {
				if !config.IsSourceAllowed(entry.URL, cfg.Enterprise.AllowedSources) {
					fmt.Printf("  %s %s: blocked by enterprise policy (source not allowed)\n",
						color.RedString("✗"), entry.Name)
					failed++
					continue
				}
			}

			// Use commit hash for exact version pinning when available
			input := entry.URL
			if input == "" {
				input = entry.Name
			}
			if entry.Commit != "" {
				// Use commit-based install for exact reproducibility
				input = input + "@" + entry.Commit
			} else if entry.Version != "" {
				input = input + "@" + entry.Version
			}

			err := installer.Install(input, opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  %s %s: %v\n", color.RedString("✗"), entry.Name, err)
				failed++
			} else {
				version := entry.Version
				if version == "" && entry.Commit != "" {
					version = entry.Commit[:min(7, len(entry.Commit))]
				}
				if version != "" {
					fmt.Printf("  %s %s (%s)\n", color.GreenString("✓"), entry.Name, version)
				} else {
					fmt.Printf("  %s %s\n", color.GreenString("✓"), entry.Name)
				}

				// Run security check if --check flag is set or enterprise requires it
				if check || (cfg.Enterprise != nil && cfg.Enterprise.RequireCheck) {
					skillsDir, sdErr := config.GetSkillsDirByScope(global)
					if sdErr != nil {
						fmt.Fprintf(os.Stderr, "Error: %v\n", sdErr)
						os.Exit(1)
					}
					skillPath := filepath.Join(skillsDir, entry.Name)
					if skill.FindSkillMD(skillPath) {
						result, checkErr := skill.CheckSafety(skillPath)
						if checkErr == nil && hasCriticalIssues(result) {
							fmt.Printf("    %s security check: critical issues found\n", color.RedString("!"))
						}
					}
				}
				succeeded++
			}
		}

		fmt.Printf("\nDone: %d installed, %d failed.\n", succeeded, failed)

		if failed > 0 {
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(lockInstallCmd)
	lockInstallCmd.Flags().StringSliceP("agent", "a", []string{}, "Target agent(s)")
	lockInstallCmd.Flags().Bool("check", false, "Run security check after installing each skill")
}
