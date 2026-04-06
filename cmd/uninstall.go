package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/filesystem"
	"github.com/yeasy/ask/internal/ui"
)

// uninstallCmd represents the uninstall command
var uninstallCmd = &cobra.Command{
	Use:   "uninstall [skill-name]",
	Short: "Uninstall a skill",
	Long: `Remove a skill from the skills directory and update configuration.
Use --global to uninstall from global installation (~/.ask/skills).

Use --agent (-a) to specify target agents (e.g., claude, cursor, codex).
If no agent is specified, uninstalls from agent directories only (keeps source).

Use --all to remove both symlinks AND the source files in .agent/skills/.`,
	Example: `  ask skill uninstall pdf
  ask skill uninstall --global pdf
  ask skill uninstall pdf --agent claude --agent cursor
  ask skill uninstall pdf --all  # Removes source and all symlinks`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		skillName := args[0]
		global, _ := cmd.Flags().GetBool("global")
		agents, _ := cmd.Flags().GetStringSlice("agent")
		removeAll, _ := cmd.Flags().GetBool("all")

		// Ensure project is initialized for non-global operations
		if !global && len(agents) == 0 {
			if !ensureInitialized() {
				return
			}
		}

		// Validate agent names
		for _, agent := range agents {
			if !config.IsValidAgent(agent) {
				fmt.Fprintf(os.Stderr, "Error: Unknown agent '%s'. Supported agents: %s\n",
					agent, strings.Join(config.GetSupportedAgentNames(), ", "))
				os.Exit(1)
			}
		}

		targetName := filepath.Base(skillName)

		// Validate skill name to prevent path traversal
		if targetName == "." || targetName == ".." || targetName == "" ||
			strings.ContainsAny(targetName, `/\`) {
			fmt.Fprintf(os.Stderr, "Error: Invalid skill name '%s'\n", skillName)
			os.Exit(1)
		}

		// Determine target directories
		var targetDirs []string
		var scopeLabel string

		if len(agents) > 0 {
			// Uninstall from specific agent directories
			for _, agentName := range agents {
				agentType, ok := config.ResolveAgentType(agentName)
				if !ok {
					fmt.Fprintf(os.Stderr, "Error: cannot resolve agent '%s'\n", agentName)
					continue
				}
				dir, err := config.GetAgentSkillsDir(agentType, global)
				if err != nil {
					ui.Debug(fmt.Sprintf("Error getting skills dir for agent %s: %v", agentName, err))
					continue
				}
				targetDirs = append(targetDirs, dir)
			}
			scopeLabel = strings.Join(agents, ", ")
			if global {
				scopeLabel += " (global)"
			}
		} else {
			if global {
				gsd, gsdErr := config.GetSkillsDirByScope(true)
				if gsdErr != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", gsdErr)
					os.Exit(1)
				}
				targetDirs = []string{gsd}
				scopeLabel = "global"
			} else {
				// Use active/detected directories
				cfg, cfgErr := config.LoadConfig()
				if cfgErr != nil {
					ui.Debug(fmt.Sprintf("Failed to load config: %v", cfgErr))
				}
				if cfg != nil {
					wd, wdErr := os.Getwd()
					if wdErr != nil {
						fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", wdErr)
						os.Exit(1)
					}
					targetDirs = cfg.GetActiveSkillsDirs(wd)
				}
				scopeLabel = "detected targets"
			}
		}

		// Central storage location
		centralDir := config.DefaultSkillsDir
		if global {
			gsd, gsdErr := config.GetSkillsDirByScope(true)
			if gsdErr != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", gsdErr)
				os.Exit(1)
			}
			centralDir = gsd
		}
		centralPath := filepath.Join(centralDir, targetName)

		removedCount := 0

		// Remove from agent directories (symlinks or copies)
		for _, dir := range targetDirs {
			skillPath := filepath.Join(dir, targetName)

			// Skip central storage unless --all is specified
			if skillPath == centralPath && !removeAll {
				continue
			}

			if _, err := os.Stat(skillPath); err == nil {
				// Check if the path is a symlink
				if filesystem.IsSymlink(skillPath) {
					ui.Debug(fmt.Sprintf("Removing symlink %s...", skillPath))
					// If it's a symlink, just remove it
					err := os.Remove(skillPath)
					if err != nil {
						ui.Warn(fmt.Sprintf("Failed to remove symlink %s: %v", skillPath, err))
					} else {
						removedCount++
					}
				} else {
					ui.Debug(fmt.Sprintf("Removing %s...", skillPath))
					// If it's a directory (copied), remove entire directory
					err := os.RemoveAll(skillPath)
					if err != nil {
						ui.Warn(fmt.Sprintf("Failed to remove skill directory %s: %v", skillPath, err))
					} else {
						removedCount++
					}
				}
			} else if !os.IsNotExist(err) {
				ui.Warn(fmt.Sprintf("Cannot access %s: %v", skillPath, err))
			}
		}

		// Remove source from central storage if --all is specified
		if removeAll {
			if _, err := os.Stat(centralPath); err == nil {
				ui.Debug(fmt.Sprintf("Removing source %s...", centralPath))
				if err := os.RemoveAll(centralPath); err != nil {
					ui.Warn(fmt.Sprintf("Failed to remove source: %v", err))
				} else {
					removedCount++
				}
			} else if !os.IsNotExist(err) {
				ui.Warn(fmt.Sprintf("Cannot access %s: %v", centralPath, err))
			}

			// Also update configuration when removing source
			cfg, err := config.LoadConfigByScope(global)
			if err == nil {
				cfg.RemoveSkill(targetName)
				cfg.RemoveSkillInfo(targetName)
				if saveErr := cfg.SaveByScope(global); saveErr != nil {
					ui.Warn(fmt.Sprintf("Failed to save config: %v", saveErr))
				}
			}

			// Update lock file
			lockFile, err := config.LoadLockFileByScope(global)
			if err == nil && lockFile != nil {
				lockFile.RemoveEntry(targetName)
				if saveErr := lockFile.SaveByScope(global); saveErr != nil {
					ui.Warn(fmt.Sprintf("Failed to save lock file: %v", saveErr))
				}
			}
		}

		if removedCount > 0 {
			if removeAll {
				fmt.Printf("Successfully uninstalled %s (source + %d links)\n", targetName, removedCount-1)
			} else {
				fmt.Printf("Successfully uninstalled %s from %s (%d removed)\n", targetName, scopeLabel, removedCount)
			}
		} else {
			fmt.Printf("Skill %s not found in any target directories\n", targetName)
		}
	},
}

func init() {
	skillCmd.AddCommand(uninstallCmd)
	uninstallCmd.Flags().StringSliceP("agent", "a", []string{}, "target agent(s) for uninstallation")
	uninstallCmd.Flags().BoolP("global", "g", false, "uninstall globally")
	uninstallCmd.Flags().Bool("all", false, "remove source and all symlinks (complete removal)")

	// Register installed skill name completion
	uninstallCmd.ValidArgsFunction = completeInstalledSkills

	// Register agent flag completion
	_ = uninstallCmd.RegisterFlagCompletionFunc("agent", completeAgentNames)
}
