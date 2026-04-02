package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/ui"
)

// updateCmd represents the update command
var updateCmd = &cobra.Command{
	Use:   "update [skill-name]",
	Short: "Update installed skills to latest version",
	Long: `Update one or all installed skills to their latest versions.
If no skill name is provided, updates all installed skills.
Use --global to update global skills.`,
	Example: `  # Update all installed skills
  ask skill update
  
  # Update a specific skill
  ask skill update pdf
  
  # Update global skills
  ask skill update --global`,
	Run: func(cmd *cobra.Command, args []string) {
		global, _ := cmd.Flags().GetBool("global")

		// Ensure project is initialized for non-global operations
		if !global {
			if !ensureInitialized() {
				return
			}
		}

		cfg, err := config.LoadConfigByScope(global)
		if err != nil {
			ui.Debug(fmt.Sprintf("Error loading config: %v", err))
			os.Exit(1)
		}

		if len(cfg.Skills) == 0 && len(cfg.SkillsInfo) == 0 {
			scopeLabel := "project"
			if global {
				scopeLabel = "global"
			}
			fmt.Printf("No %s skills installed.\n", scopeLabel)
			return
		}

		// Build combined deduplicated skills list
		allSkills := cfg.GetAllSkillNames()

		// Determine which skills to update
		var skillsToUpdate []string
		if len(args) > 0 {
			// Update specific skill
			skillName := args[0]
			found := false
			for _, s := range cfg.Skills {
				if s == skillName {
					found = true
					break
				}
			}
			if !found {
				for _, si := range cfg.SkillsInfo {
					if si.Name == skillName {
						found = true
						break
					}
				}
			}
			if !found {
				fmt.Printf("Skill '%s' is not installed.\n", skillName)
				os.Exit(1)
			}
			skillsToUpdate = []string{skillName}
		} else {
			// Update all skills
			skillsToUpdate = allSkills
		}

		skillsDir, err := config.GetSkillsDirByScope(global)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		for _, skillName := range skillsToUpdate {
			skillPath := filepath.Join(skillsDir, skillName)

			// Check if it's a git repository
			gitDir := filepath.Join(skillPath, ".git")
			if _, err := os.Stat(gitDir); os.IsNotExist(err) {
				ui.Debug(fmt.Sprintf("Skipping %s (not a git repository)", skillName))
				continue
			}

			ui.Debug(fmt.Sprintf("Updating %s...", skillName))

			// Run git pull (use --ff-only to avoid leaving broken rebase state)
			pullCtx, pullCancel := context.WithTimeout(context.Background(), 60*time.Second)
			gitCmd := exec.CommandContext(pullCtx, "git", "pull", "--ff-only")
			gitCmd.Dir = skillPath
			gitCmd.Stdout = os.Stdout
			gitCmd.Stderr = os.Stderr

			runErr := gitCmd.Run()
			pullCancel()
			if runErr != nil {
				ui.Warn(fmt.Sprintf("  Failed to update %s: %v", skillName, runErr))
				continue
			}

			ui.Debug(fmt.Sprintf("  Updated %s successfully!", skillName))
		}
	},
}

func init() {
	skillCmd.AddCommand(updateCmd)
}
