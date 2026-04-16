package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/skill"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check [skill-path]",
	Short: "Check a skill for security issues",
	Long: `Analyze a skill directory for potential security risks, 
including hardcoded secrets, dangerous commands, and network activity.

If skill-path is not provided, the current directory is checked.
If the current directory is not a skill, all installed skills across all agents are checked.`,
	Args: cobra.MaximumNArgs(1),
	Run:  runCheck,
}

var outputFile string

func init() {
	checkCmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write report to file (supports .md, .html/.htm, .json, .sarif)")
	checkCmd.Flags().Bool("ci", false, "CI mode: exit with non-zero code on any findings at or above severity threshold")
	checkCmd.Flags().String("severity", "warning", "Minimum severity to report: info, warning, critical")
	checkCmd.Flags().String("format", "", "Output format: console (default), json, html, markdown, sarif")
	checkCmd.Flags().Bool("watch", false, "Watch mode: re-check on file changes")
}

func filterBySeverity(result *skill.CheckResult, minSeverity string) *skill.CheckResult {
	filtered := &skill.CheckResult{
		SkillName:      result.SkillName,
		ScannedModules: result.ScannedModules,
	}
	for _, f := range result.Findings {
		switch minSeverity {
		case "critical":
			if f.Severity == skill.SeverityCritical {
				filtered.Findings = append(filtered.Findings, f)
			}
		case "warning":
			if f.Severity == skill.SeverityCritical || f.Severity == skill.SeverityWarning {
				filtered.Findings = append(filtered.Findings, f)
			}
		default: // "info"
			filtered.Findings = append(filtered.Findings, f)
		}
	}
	return filtered
}

func runCheck(cmd *cobra.Command, args []string) {
	var targetPath string
	var err error

	// Determine target path
	if len(args) > 0 {
		targetPath, err = filepath.Abs(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
			os.Exit(1)
		}
	} else {
		targetPath, err = os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
			os.Exit(1)
		}
	}

	// Get new flags
	ciMode, _ := cmd.Flags().GetBool("ci")
	severityFilter, _ := cmd.Flags().GetString("severity")
	format, _ := cmd.Flags().GetString("format")
	watchMode, _ := cmd.Flags().GetBool("watch")

	// Watch mode
	if watchMode {
		runWatchMode(targetPath, severityFilter)
		return
	}

	// Check if the target itself is a skill
	if skill.FindSkillMD(targetPath) {
		if len(args) == 0 {
			fmt.Printf("Checking skill in current directory...\n")
		}

		result, err := checkSingleSkill(targetPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error checking skill: %v\n", err)
			os.Exit(1)
		}

		// Ensure module name is set
		for i := range result.Findings {
			if result.Findings[i].Module == "" {
				result.Findings[i].Module = result.SkillName
			}
		}

		// Filter by severity
		result = filterBySeverity(result, severityFilter)

		// Handle output based on format
		if format == "sarif" {
			content, err := skill.GenerateSARIFReport(result, Version)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error generating SARIF report: %v\n", err)
				os.Exit(1)
			}
			if outputFile != "" {
				if err := os.WriteFile(outputFile, []byte(content), 0600); err != nil {
					fmt.Fprintf(os.Stderr, "Error writing report: %v\n", err)
					os.Exit(1)
				}
				fmt.Fprintf(os.Stderr, "SARIF report saved to %s\n", outputFile)
			} else {
				fmt.Println(content)
			}
		} else if outputFile != "" {
			handleReport(result, outputFile)
		} else {
			printReport(result)
		}

		// Handle CI mode
		if ciMode && len(result.Findings) > 0 {
			os.Exit(1)
		}

		// Default behavior: exit on critical issues
		if !ciMode && hasCriticalIssues(result) {
			os.Exit(1)
		}
		return
	}

	// Not a skill? Scan as a project
	label := "project skills"
	if len(args) > 0 {
		label = args[0]
	} else {
		fmt.Println("Current directory is not a skill. Scanning project skills...")
	}

	scanProject(targetPath, label)
}

func formatPathForDisplay(path string) string {
	cwd, err := os.Getwd()
	if err == nil {
		rel, err := filepath.Rel(cwd, path)
		if err == nil && !strings.HasPrefix(rel, "..") {
			return rel
		}
	}

	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}

	return path
}

func checkSingleSkill(absPath string) (*skill.CheckResult, error) {
	return skill.CheckSafety(absPath)
}

func hasCriticalIssues(result *skill.CheckResult) bool {
	for _, f := range result.Findings {
		if f.Severity == skill.SeverityCritical {
			return true
		}
	}
	return false
}

func handleReport(result *skill.CheckResult, filename string) {
	ext := filepath.Ext(filename)

	var content string
	var err error

	if ext == ".sarif" {
		content, err = skill.GenerateSARIFReport(result, Version)
	} else {
		format := "md"
		switch ext {
		case ".html", ".htm":
			format = "html"
		case ".json":
			format = "json"
		}
		content, err = skill.GenerateReport(result, format)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(filename, []byte(content), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing report to %s: %v\n", filename, err)
		os.Exit(1)
	}

	fmt.Printf("\n%s Security report saved to %s\n", color.GreenString("✓"), filename)

	if hasCriticalIssues(result) {
		fmt.Printf("%s Found critical issues. Please review the report.\n", color.RedString("!"))
	}
}

func printReport(result *skill.CheckResult) {
	fmt.Printf("\nSecurity Report for: %s\n", color.CyanString(result.SkillName))
	fmt.Println("----------------------------------------")

	if len(result.Findings) == 0 {
		color.Green("✓ No issues found. Skill appears safe.\n")
		return
	}

	criticals := 0
	warnings := 0
	infos := 0

	for _, finding := range result.Findings {
		switch finding.Severity {
		case skill.SeverityCritical:
			criticals++
			printFinding(color.RedString("[CRITICAL]"), finding)
		case skill.SeverityWarning:
			warnings++
			printFinding(color.YellowString("[WARNING] "), finding)
		case skill.SeverityInfo:
			infos++
			printFinding(color.BlueString("[INFO]    "), finding)
		}
	}

	fmt.Println("----------------------------------------")
	fmt.Printf("Summary: %d Critical, %d Warning, %d Info\n", criticals, warnings, infos)
}

func printCompactReport(result *skill.CheckResult) {
	if len(result.Findings) == 0 {
		fmt.Printf("  %s %s: No issues\n", color.GreenString("✓"), result.SkillName)
		return
	}

	criticals := 0
	warnings := 0
	for _, f := range result.Findings {
		switch f.Severity {
		case skill.SeverityCritical:
			criticals++
		case skill.SeverityWarning:
			warnings++
		}
	}

	if criticals > 0 {
		fmt.Printf("  %s %s: %d Critical, %d Warnings\n", color.RedString("!"), result.SkillName, criticals, warnings)
		for _, f := range result.Findings {
			if f.Severity == skill.SeverityCritical {
				fmt.Printf("    - %s: %s\n", f.RuleID, f.Description)
			}
		}
	} else if warnings > 0 {
		fmt.Printf("  %s %s: %d Warnings\n", color.YellowString("!"), result.SkillName, warnings)
	} else {
		// Only info
		fmt.Printf("  %s %s: OK (Info only)\n", color.GreenString("✓"), result.SkillName)
	}
}

func printFinding(prefix string, finding skill.Finding) {
	fmt.Printf("%s %s\n", prefix, finding.Description)
	fmt.Printf("  File: %s:%d\n", finding.File, finding.Line)
	fmt.Printf("  Match: %s\n\n", finding.Match)
}

// scanProject scans a project root for skills in known directories
func scanProject(rootDir string, label string) {
	// Define directories to check. Order matters for precedence implicitly if we were deduplicating skills by name,
	// but here we report everything.
	// 1. .agent/skills (Default)
	// 2. <agent> project dirs (e.g. .claude/skills)
	// 3. skills (Generic)
	// 4. . (Root - for flat repos)
	searchDirs := []string{config.DefaultSkillsDir, "skills", "."}
	for _, agentConfig := range config.SupportedAgents {
		searchDirs = append(searchDirs, agentConfig.ProjectDir)
	}

	// Deduplicate search dirs
	uniqueDirs := make(map[string]bool)
	var dirs []string
	for _, d := range searchDirs {
		if !uniqueDirs[d] {
			uniqueDirs[d] = true
			dirs = append(dirs, d)
		}
	}

	foundSkills := 0
	issuesFound := false
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		return
	}

	// Aggregate results
	// Use formatted path for display if it looks like a path
	displayLabel := label
	if filepath.IsAbs(label) || strings.Contains(label, string(os.PathSeparator)) || label == "." {
		displayLabel = formatPathForDisplay(label)
	}

	// If display label resolves to ".", it's confusing in a static report. Use absolute or home-relative path instead.
	if displayLabel == "." {
		if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(rootDir, home) {
			displayLabel = "~" + strings.TrimPrefix(rootDir, home)
		} else {
			displayLabel = rootDir
		}
	}

	aggregatedResult := &skill.CheckResult{
		SkillName: displayLabel,
		Findings:  []skill.Finding{},
	}

	// Track examined paths to avoid duplicate scans if nested dirs overlap
	// (e.g. scanning "." and "skills" - "." covers "skills")
	scannedPaths := make(map[string]bool)

	for _, relDir := range dirs {
		startDir := filepath.Join(rootDir, relDir)

		// If we already scanned this directory (or a parent of it) as part of "." scan, strictly speaking we might double scan.
		// However, "WalkDir" on "." will cover everything.
		// If "." is in the list, it effectively supersedes others if they are subdirs of it.
		// To keep it simple and robust: we will walk each requested dir.
		// Inside walk, we check unique SKILL.md paths.

		if walkErr := filepath.WalkDir(startDir, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			if d.IsDir() {
				// Optimization: Skip large non-skill directories
				switch d.Name() {
				case ".git", "node_modules", "vendor", ".venv", "venv",
					"__pycache__", ".next", ".nuxt", "dist", "build",
					".cache", ".gradle", ".cargo", "target", ".tox":
					return filepath.SkipDir
				}

				// Check if this directory is a skill
				if skill.FindSkillMD(path) {
					// Use absolute path for uniqueness check
					absPath, absErr := filepath.Abs(path)
					if absErr != nil {
						return nil // Skip this path on error
					}
					if scannedPaths[absPath] {
						return filepath.SkipDir // Already scanned this skill
					}
					scannedPaths[absPath] = true

					foundSkills++
					displayPath := formatPathForDisplay(path)
					fmt.Printf("Checking %s...\n", displayPath)

					result, err := checkSingleSkill(path)
					if err != nil {
						fmt.Fprintf(os.Stderr, "  Error checking skill: %v\n", err)
						return nil
					}

					printCompactReport(result)
					if hasCriticalIssues(result) {
						issuesFound = true
					}

					// Add to aggregated results
					relSkillPath, _ := filepath.Rel(cwd, path)
					if relSkillPath == "" || strings.HasPrefix(relSkillPath, "..") {
						relSkillPath, _ = filepath.Rel(rootDir, path)
					}
					if relSkillPath == "" || strings.HasPrefix(relSkillPath, "..") {
						relSkillPath = filepath.Base(path)
					}

					for _, f := range result.Findings {
						if f.Module == "" {
							f.Module = result.SkillName
						}
						f.File = filepath.Join(relSkillPath, f.File)
						aggregatedResult.Findings = append(aggregatedResult.Findings, f)
					}

					// Track scanned module
					aggregatedResult.ScannedModules = append(aggregatedResult.ScannedModules, result.SkillName)

					// If we found a skill, we typically don't scan subdirectories of a skill
					// (nested skills are rare/discouraged), but to be safe we can continue or skip.
					// Let's SkipDir to avoid scanning internal directories of a skill as potential skills.
					return filepath.SkipDir
				}
			}
			return nil
		}); walkErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to scan %s: %v\n", startDir, walkErr)
		}
	}

	if foundSkills == 0 {
		fmt.Printf("No skills found in %s (checked recursively).\n", rootDir)
	} else {
		fmt.Printf("\nScanned %d skills.\n", foundSkills)

		if outputFile != "" {
			handleReport(aggregatedResult, outputFile)
		}
	}

	if issuesFound {
		fmt.Println(color.RedString("\nCritical issues found in one or more skills."))
		os.Exit(1)
	}
}

func runWatchMode(targetPath, severityFilter string) {
	displayPath := formatPathForDisplay(targetPath)
	fmt.Printf("Watching for changes in %s... (Ctrl+C to stop)\n\n", color.CyanString(displayPath))

	// Run initial check
	printWatchResult("initial scan", targetPath, severityFilter)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	err := skill.WatchAndCheck(ctx, targetPath, func(event string, result *skill.CheckResult, checkErr error) {
		timestamp := time.Now().Format("15:04:05")
		if checkErr != nil {
			fmt.Fprintf(os.Stderr, "[%s] %s error: %v\n", color.YellowString(timestamp), event, checkErr)
			return
		}

		// Filter by severity
		filtered := filterBySeverity(result, severityFilter)

		if len(filtered.Findings) == 0 {
			fmt.Printf("[%s] %s modified → %s\n",
				color.YellowString(timestamp),
				event,
				color.GreenString("0 issues ✓"))
		} else {
			criticals := 0
			warnings := 0
			infos := 0
			for _, f := range filtered.Findings {
				switch f.Severity {
				case skill.SeverityCritical:
					criticals++
				case skill.SeverityWarning:
					warnings++
				case skill.SeverityInfo:
					infos++
				}
			}
			summary := fmt.Sprintf("%d critical, %d warning, %d info",
				criticals, warnings, infos)
			if criticals > 0 {
				fmt.Printf("[%s] %s modified → %s\n",
					color.YellowString(timestamp),
					event,
					color.RedString(summary))
			} else {
				fmt.Printf("[%s] %s modified → %s\n",
					color.YellowString(timestamp),
					event,
					color.YellowString(summary))
			}
			// Show critical findings inline
			for _, f := range filtered.Findings {
				if f.Severity == skill.SeverityCritical {
					fmt.Printf("    %s %s: %s (%s:%d)\n",
						color.RedString("!"),
						f.RuleID,
						f.Description,
						f.File,
						f.Line)
				}
			}
		}
	})

	if err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "Watch error: %v\n", err)
		os.Exit(1)
	}
}

func printWatchResult(event, targetPath, severityFilter string) {
	timestamp := time.Now().Format("15:04:05")
	result, err := skill.CheckSafety(targetPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[%s] %s error: %v\n", color.YellowString(timestamp), event, err)
		return
	}

	filtered := filterBySeverity(result, severityFilter)
	if len(filtered.Findings) == 0 {
		fmt.Printf("[%s] %s → %s\n",
			color.YellowString(timestamp),
			event,
			color.GreenString("0 issues ✓"))
	} else {
		fmt.Printf("[%s] %s → %d issues found\n",
			color.YellowString(timestamp),
			event,
			len(filtered.Findings))
	}
}
