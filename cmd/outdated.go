package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/ui"
)

// outdatedCmd represents the outdated command
var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Check for outdated skills",
	Long: `Check which installed skills have updates available.
Use --global to check global skills.`,
	Run: func(cmd *cobra.Command, _ []string) {
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

		if len(cfg.Skills) == 0 {
			scopeLabel := "project"
			if global {
				scopeLabel = "global"
			}
			fmt.Printf("No %s skills installed.\n", scopeLabel)
			return
		}

		lockFile, _ := config.LoadLockFileByScope(global)

		scopeLabel := "project"
		if global {
			scopeLabel = "global"
		}
		ui.Debug(fmt.Sprintf("Checking for updates (%s)...", scopeLabel))
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "SKILL\tCURRENT\tLATEST\tSTATUS")

		outdatedCount := 0
		skillsDir := config.GetSkillsDirByScope(global)

		for _, skillName := range cfg.Skills {
			skillPath := filepath.Join(skillsDir, skillName)

			// Check if it's a git repository
			gitDir := filepath.Join(skillPath, ".git")
			if _, err := os.Stat(gitDir); os.IsNotExist(err) {
				_, _ = fmt.Fprintf(w, "%s\t-\t-\tNot a git repo\n", skillName)
				continue
			}

			// Get current commit
			currentCommit := getShortCommit(skillPath)

			// Fetch latest from remote (skip if offline)
			remoteCommit := ""
			status := "✓ Up to date"

			if !config.OfflineMode {
				fetchCmd := exec.Command("git", "fetch", "--quiet")
				fetchCmd.Dir = skillPath
				_ = fetchCmd.Run()

				// Get remote HEAD commit
				remoteCommit = getRemoteHeadCommit(skillPath)
			} else {
				remoteCommit = "?"
				status = "? Unknown (Offline)"
			}

			// Get lock file info
			lockEntry := lockFile.GetEntry(skillName)
			lockedVersion := ""
			if lockEntry != nil && lockEntry.Version != "" {
				lockedVersion = lockEntry.Version
			}

			// Compare
			// Compare
			if !config.OfflineMode {
				status = "✓ Up to date"
				if currentCommit != remoteCommit && remoteCommit != "" {
					status = "⬆ Update available"
					outdatedCount++
				}
			}

			currentDisplay := currentCommit
			if lockedVersion != "" {
				currentDisplay = lockedVersion + " (" + currentCommit + ")"
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", skillName, currentDisplay, remoteCommit, status)
		}

		_ = w.Flush()

		fmt.Println()
		if outdatedCount > 0 {
			updateCmd := "ask skill update"
			if global {
				updateCmd = "ask skill update --global"
			}
			fmt.Printf("%d skill(s) can be updated. Run '%s' to update all.\n", outdatedCount, updateCmd)
		} else {
			fmt.Println("All skills are up to date.")
		}
	},
}

// getShortCommit returns the short commit hash
func getShortCommit(repoPath string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return "-"
	}
	return strings.TrimSpace(string(output))
}

// getRemoteHeadCommit returns the short commit hash of remote HEAD
func getRemoteHeadCommit(repoPath string) string {
	cmd := exec.Command("git", "rev-parse", "--short", "origin/HEAD")
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		// Try origin/main or origin/master
		cmd = exec.Command("git", "rev-parse", "--short", "origin/main")
		cmd.Dir = repoPath
		output, err = cmd.Output()
		if err != nil {
			cmd = exec.Command("git", "rev-parse", "--short", "origin/master")
			cmd.Dir = repoPath
			output, err = cmd.Output()
			if err != nil {
				return "-"
			}
		}
	}
	return strings.TrimSpace(string(output))
}

func init() {
	skillCmd.AddCommand(outdatedCmd)
}
