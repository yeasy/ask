package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/cache"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/skill"
)

// DoctorResult represents the result of a health check
type DoctorResult struct {
	Category string      `json:"category"`
	Status   string      `json:"status"` // "ok", "warning", "error"
	Message  string      `json:"message"`
	Details  []string    `json:"details,omitempty"`
	Children []CheckItem `json:"children,omitempty"`
}

// CheckItem represents a single check result
type CheckItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // "ok", "warning", "error"
	Message string `json:"message"`
}

// DoctorReport represents the complete health check report
type DoctorReport struct {
	Version string         `json:"version"`
	Results []DoctorResult `json:"results"`
	Summary DoctorSummary  `json:"summary"`
}

// DoctorSummary provides overall statistics
type DoctorSummary struct {
	TotalChecks   int `json:"total_checks"`
	PassedChecks  int `json:"passed_checks"`
	WarningChecks int `json:"warning_checks"`
	FailedChecks  int `json:"failed_checks"`
}

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose and report on ASK health",
	Long: `Run health checks on your ASK installation and project configuration.

This command validates:
- Configuration files (ask.yaml, ask.lock)
- Skills directories and installed skills
- Repository cache status
- System dependencies (git)
- Agent directory detection`,
	Example: `  ask doctor           # Run all health checks
  ask doctor --json    # Output results in JSON format`,
	Run: runDoctor,
}

var doctorJSON bool

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "output results in JSON format")
}

func runDoctor(_ *cobra.Command, _ []string) {
	report := DoctorReport{
		Version: "1.0",
		Results: []DoctorResult{},
	}

	// Run all checks
	report.Results = append(report.Results, checkConfiguration())
	report.Results = append(report.Results, checkSkillsDirectory())
	report.Results = append(report.Results, checkRepositoryCache())
	report.Results = append(report.Results, checkSystem())
	report.Results = append(report.Results, checkAgentDirectories())

	// Calculate summary
	for _, result := range report.Results {
		for _, child := range result.Children {
			report.Summary.TotalChecks++
			switch child.Status {
			case "ok":
				report.Summary.PassedChecks++
			case "warning":
				report.Summary.WarningChecks++
			case "error":
				report.Summary.FailedChecks++
			}
		}
	}

	if doctorJSON {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		}
		return
	}

	// Text output
	printDoctorReport(report)
}

func printDoctorReport(report DoctorReport) {
	fmt.Println("ASK Doctor - Health Check")
	fmt.Println(strings.Repeat("─", 40))
	fmt.Println()

	for _, result := range report.Results {
		statusIcon := getStatusIcon(result.Status)
		fmt.Printf("%s %s\n", statusIcon, result.Category)

		for _, child := range result.Children {
			childIcon := getStatusIcon(child.Status)
			fmt.Printf("  %s %s\n", childIcon, child.Message)
		}
		fmt.Println()
	}

	// Summary
	fmt.Println(strings.Repeat("─", 40))
	fmt.Printf("Summary: %d passed", report.Summary.PassedChecks)
	if report.Summary.WarningChecks > 0 {
		fmt.Printf(", %d warnings", report.Summary.WarningChecks)
	}
	if report.Summary.FailedChecks > 0 {
		fmt.Printf(", %d errors", report.Summary.FailedChecks)
	}
	fmt.Println()

	if report.Summary.FailedChecks > 0 {
		os.Exit(1)
	}
}

func getStatusIcon(status string) string {
	switch status {
	case "ok":
		return "✓"
	case "warning":
		return "⚠"
	case "error":
		return "✗"
	default:
		return "?"
	}
}

func checkConfiguration() DoctorResult {
	result := DoctorResult{
		Category: "Configuration",
		Status:   "ok",
		Children: []CheckItem{},
	}

	// Check ask.yaml
	if _, err := os.Stat("ask.yaml"); err == nil {
		cfg, err := config.LoadConfig()
		if err != nil {
			result.Children = append(result.Children, CheckItem{
				Name:    "ask.yaml",
				Status:  "error",
				Message: fmt.Sprintf("ask.yaml found but invalid: %v", err),
			})
			result.Status = "error"
		} else {
			result.Children = append(result.Children, CheckItem{
				Name:    "ask.yaml",
				Status:  "ok",
				Message: fmt.Sprintf("ask.yaml found (version: %s)", cfg.Version),
			})
		}
	} else {
		result.Children = append(result.Children, CheckItem{
			Name:    "ask.yaml",
			Status:  "warning",
			Message: "ask.yaml not found - run 'ask init' to create",
		})
		if result.Status == "ok" {
			result.Status = "warning"
		}
	}

	// Check ask.lock
	if _, err := os.Stat("ask.lock"); err == nil {
		lockFile, err := config.LoadLockFile()
		if err != nil {
			result.Children = append(result.Children, CheckItem{
				Name:    "ask.lock",
				Status:  "warning",
				Message: fmt.Sprintf("ask.lock found but invalid: %v", err),
			})
			if result.Status == "ok" {
				result.Status = "warning"
			}
		} else {
			result.Children = append(result.Children, CheckItem{
				Name:    "ask.lock",
				Status:  "ok",
				Message: fmt.Sprintf("ask.lock found (%d entries)", len(lockFile.Skills)),
			})
		}
	} else {
		result.Children = append(result.Children, CheckItem{
			Name:    "ask.lock",
			Status:  "ok",
			Message: "ask.lock not found (will be created on first install)",
		})
	}

	return result
}

func checkSkillsDirectory() DoctorResult {
	result := DoctorResult{
		Category: "Skills Directory",
		Status:   "ok",
		Children: []CheckItem{},
	}

	skillsDir := config.DefaultSkillsDir

	if _, err := os.Stat(skillsDir); err == nil {
		entries, err := os.ReadDir(skillsDir)
		if err != nil {
			result.Children = append(result.Children, CheckItem{
				Name:    "directory",
				Status:  "error",
				Message: fmt.Sprintf("Cannot read %s: %v", skillsDir, err),
			})
			result.Status = "error"
			return result
		}

		// Count skills and check for SKILL.md
		var skillCount int
		var missingSkillMD []string
		for _, entry := range entries {
			if entry.IsDir() {
				skillCount++
				skillPath := filepath.Join(skillsDir, entry.Name())
				if !skill.FindSkillMD(skillPath) {
					missingSkillMD = append(missingSkillMD, entry.Name())
				}
			}
		}

		result.Children = append(result.Children, CheckItem{
			Name:    "directory",
			Status:  "ok",
			Message: fmt.Sprintf("%s exists (%d skills installed)", skillsDir, skillCount),
		})

		if len(missingSkillMD) > 0 {
			result.Children = append(result.Children, CheckItem{
				Name:    "skill_md",
				Status:  "warning",
				Message: fmt.Sprintf("%d skills missing SKILL.md: %s", len(missingSkillMD), strings.Join(missingSkillMD, ", ")),
			})
			if result.Status == "ok" {
				result.Status = "warning"
			}
		}
	} else {
		result.Children = append(result.Children, CheckItem{
			Name:    "directory",
			Status:  "ok",
			Message: fmt.Sprintf("%s not found (will be created on first install)", skillsDir),
		})
	}

	return result
}

func checkRepositoryCache() DoctorResult {
	result := DoctorResult{
		Category: "Repository Cache",
		Status:   "ok",
		Children: []CheckItem{},
	}

	reposCacheDir := cache.GetReposCacheDir()

	if _, err := os.Stat(reposCacheDir); err == nil {
		reposCache, err := cache.NewReposCache()
		if err != nil {
			result.Children = append(result.Children, CheckItem{
				Name:    "cache",
				Status:  "error",
				Message: fmt.Sprintf("Cannot access repos cache: %v", err),
			})
			result.Status = "error"
			return result
		}

		repos := reposCache.GetCachedRepos()
		result.Children = append(result.Children, CheckItem{
			Name:    "cache",
			Status:  "ok",
			Message: fmt.Sprintf("%s exists (%d repos cached)", reposCacheDir, len(repos)),
		})

		if len(repos) == 0 {
			result.Children = append(result.Children, CheckItem{
				Name:    "sync",
				Status:  "warning",
				Message: "No repos synced - run 'ask repo sync' for faster searches",
			})
			if result.Status == "ok" {
				result.Status = "warning"
			}
		}
	} else {
		result.Children = append(result.Children, CheckItem{
			Name:    "cache",
			Status:  "ok",
			Message: fmt.Sprintf("%s not found (run 'ask repo sync' to populate)", reposCacheDir),
		})
	}

	return result
}

func checkSystem() DoctorResult {
	result := DoctorResult{
		Category: "System",
		Status:   "ok",
		Children: []CheckItem{},
	}

	// Check git
	gitCtx, gitCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer gitCancel()
	gitCmd := exec.CommandContext(gitCtx, "git", "--version")
	output, err := gitCmd.Output()
	if err != nil {
		result.Children = append(result.Children, CheckItem{
			Name:    "git",
			Status:  "error",
			Message: "git not found - required for cloning skills",
		})
		result.Status = "error"
	} else {
		version := strings.TrimSpace(string(output))
		result.Children = append(result.Children, CheckItem{
			Name:    "git",
			Status:  "ok",
			Message: version,
		})
	}

	// Check home directory
	home, err := os.UserHomeDir()
	if err != nil {
		result.Children = append(result.Children, CheckItem{
			Name:    "home",
			Status:  "error",
			Message: "Cannot determine home directory",
		})
		result.Status = "error"
	} else {
		globalDir := filepath.Join(home, config.GlobalConfigDirName)
		if _, err := os.Stat(globalDir); err == nil {
			result.Children = append(result.Children, CheckItem{
				Name:    "global_dir",
				Status:  "ok",
				Message: fmt.Sprintf("Global config: %s", globalDir),
			})
		} else {
			result.Children = append(result.Children, CheckItem{
				Name:    "global_dir",
				Status:  "ok",
				Message: fmt.Sprintf("Global config: %s (will be created)", globalDir),
			})
		}
	}

	return result
}

func checkAgentDirectories() DoctorResult {
	result := DoctorResult{
		Category: "Agent Directories",
		Status:   "ok",
		Children: []CheckItem{},
	}

	cwd, err := os.Getwd()
	if err != nil {
		result.Children = append(result.Children, CheckItem{
			Name:    "cwd",
			Status:  "error",
			Message: "Cannot get current directory",
		})
		result.Status = "error"
		return result
	}

	detected := config.DetectExistingToolDirs(cwd)
	if len(detected) > 0 {
		for _, t := range detected {
			result.Children = append(result.Children, CheckItem{
				Name:    t.Name,
				Status:  "ok",
				Message: fmt.Sprintf("%s detected (%s)", t.Name, t.SkillsDir),
			})
		}
	} else {
		result.Children = append(result.Children, CheckItem{
			Name:    "none",
			Status:  "ok",
			Message: "No agent-specific directories detected (using .agent/skills)",
		})
	}

	return result
}
