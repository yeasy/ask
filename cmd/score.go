package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/git"
	"github.com/yeasy/ask/internal/github"
	"github.com/yeasy/ask/internal/skill"
)

var scoreCmd = &cobra.Command{
	Use:   "score <path-or-url>",
	Short: "Compute a trust score for a skill",
	Long: `Compute a comprehensive trust score for a skill, evaluating:
  - Security: static analysis for secrets, malware, and dangerous commands
  - Quality: SKILL.md metadata, README, prompts structure
  - Publisher: GitHub account reputation, stars, organization status
  - Transparency: data exfiltration patterns and obfuscated code

The score ranges from 0-100 with grades A/B/C/D/F.
Similar to Snyk or Socket.dev for the agent skill ecosystem.

Use --batch to scan all skills in a directory or repository source.`,
	Example: `  # Score a local skill directory
  ask score ./my-skill

  # Score with JSON output
  ask score ./my-skill --json

  # Score a remote skill (cloned to temp dir)
  ask score anthropics/browser-use

  # Batch score all skills in a directory
  ask score --batch ./skills-dir

  # Batch score a remote repo with multiple skills
  ask score --batch anthropics/skills/skills

  # Batch score with JSON output
  ask score --batch ./skills-dir --json`,
	Args: cobra.ExactArgs(1),
	Run:  runScore,
}

func runScore(cmd *cobra.Command, args []string) {
	batch, _ := cmd.Flags().GetBool("batch")
	if batch {
		runBatchScore(cmd, args)
		return
	}

	target := args[0]
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Resolve target path
	skillPath := target
	var publisher *skill.PublisherInfo

	// Check if target is a local directory
	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		// Try as a GitHub reference — clone to temp
		fmt.Printf("Resolving %s...\n", target)
		cloneRoot, clonedPath, cloneErr := cloneForScore(target)
		if cloneErr != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot resolve '%s' as local path or GitHub repo: %v\n", target, cloneErr)
			os.Exit(1)
		}
		defer func() { _ = os.RemoveAll(cloneRoot) }()
		skillPath = clonedPath

		// Fetch publisher info from GitHub
		publisher = fetchPublisherInfo(target)
	}

	// Run scoring
	result, err := skill.ScoreSkill(skillPath, publisher)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error computing score: %v\n", err)
		os.Exit(1)
	}

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if encErr := encoder.Encode(result); encErr != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", encErr)
		}
		return
	}

	// Pretty print
	printScoreResult(result)
}

// BatchScoreResult holds results for batch scoring
type BatchScoreResult struct {
	Source string              `json:"source"`
	Total  int                 `json:"total"`
	Scores []skill.ScoreResult `json:"scores"`
	Stats  BatchStats          `json:"stats"`
}

// BatchStats summarizes batch scoring statistics
type BatchStats struct {
	Average float64        `json:"average"`
	Grades  map[string]int `json:"grades"`
}

func runBatchScore(cmd *cobra.Command, args []string) {
	target := args[0]
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Resolve target to a local directory
	baseDir := target
	var cloneRoot string
	var publisher *skill.PublisherInfo

	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		// Clone remote repo
		fmt.Printf("Cloning %s for batch scoring...\n", target)
		root, clonedPath, cloneErr := cloneForScore(target)
		if cloneErr != nil {
			fmt.Fprintf(os.Stderr, "Error: cannot resolve '%s': %v\n", target, cloneErr)
			os.Exit(1)
		}
		cloneRoot = root
		baseDir = clonedPath
		publisher = fetchPublisherInfo(target)
	}
	if cloneRoot != "" {
		defer func() { _ = os.RemoveAll(cloneRoot) }()
	}

	// Find all skill subdirectories (those containing SKILL.md)
	skillDirs := discoverSkillDirs(baseDir)
	if len(skillDirs) == 0 {
		fmt.Printf("No skills found in %s\n", target)
		os.Exit(0)
	}

	fmt.Printf("Found %d skills. Scoring...\n\n", len(skillDirs))

	batchResult := BatchScoreResult{
		Source: target,
		Total:  len(skillDirs),
		Stats: BatchStats{
			Grades: make(map[string]int),
		},
	}

	// Score each skill
	var totalScore float64
	for _, dir := range skillDirs {
		result, scoreErr := skill.ScoreSkill(dir, publisher)
		if scoreErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to score %s: %v\n", filepath.Base(dir), scoreErr)
			continue
		}
		batchResult.Scores = append(batchResult.Scores, *result)
		totalScore += result.TotalScore
		batchResult.Stats.Grades[string(result.Grade)]++
	}

	if len(batchResult.Scores) > 0 {
		batchResult.Stats.Average = totalScore / float64(len(batchResult.Scores))
	}

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if encErr := encoder.Encode(batchResult); encErr != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", encErr)
		}
		return
	}

	// Pretty print batch results as table
	printBatchResult(&batchResult)
}

func discoverSkillDirs(baseDir string) []string {
	var dirs []string

	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return dirs
	}

	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		subDir := filepath.Join(baseDir, e.Name())
		// Check if this subdirectory looks like a skill (has SKILL.md)
		if skill.FindSkillMD(subDir) {
			dirs = append(dirs, subDir)
		}
	}

	return dirs
}

func printBatchResult(batch *BatchScoreResult) {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow)
	red := color.New(color.FgRed)

	_, _ = bold.Printf("\n  Batch Score Report: %s\n", batch.Source)
	fmt.Printf("  %d skills scored\n\n", len(batch.Scores))

	// Table header
	_, _ = bold.Printf("  %-30s  %6s  %5s  %s\n", "SKILL", "SCORE", "GRADE", "ISSUES")
	fmt.Printf("  %s\n", strings.Repeat("─", 70))

	// Sort by score (worst first for visibility)
	for _, r := range batch.Scores {
		grade := string(r.Grade)
		var gradeStr string
		switch r.Grade {
		case skill.GradeA, skill.GradeB:
			gradeStr = green.Sprint(grade)
		case skill.GradeC:
			gradeStr = yellow.Sprint(grade)
		default:
			gradeStr = red.Sprint(grade)
		}

		issues := 0
		for _, cat := range r.Categories {
			issues += len(cat.Deducts)
		}

		name := r.SkillName
		if len(name) > 30 {
			name = name[:27] + "..."
		}
		fmt.Printf("  %-30s  %5.1f   %s      %d\n", name, r.TotalScore, gradeStr, issues)
	}

	// Summary
	fmt.Printf("\n  %s\n", strings.Repeat("─", 70))
	fmt.Printf("  Average: %.1f/100", batch.Stats.Average)

	if len(batch.Stats.Grades) > 0 {
		fmt.Printf("  |  ")
		for _, g := range []string{"A", "B", "C", "D", "F"} {
			if count, ok := batch.Stats.Grades[g]; ok && count > 0 {
				fmt.Printf("%s:%d  ", g, count)
			}
		}
	}
	fmt.Printf("\n\n")
}

func printScoreResult(result *skill.ScoreResult) {
	bold := color.New(color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)
	red := color.New(color.FgRed, color.Bold)

	fmt.Println()
	_, _ = bold.Printf("  Trust Score: %s %s\n", result.SkillName, formatGrade(result))
	fmt.Println()

	// Score bar
	barLen := 40
	filled := int(result.TotalScore / 100 * float64(barLen))
	if filled > barLen {
		filled = barLen
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", barLen-filled)
	fmt.Printf("  [%s] %.1f/100\n\n", bar, result.TotalScore)

	// Category breakdown
	for _, cat := range result.Categories {
		icon := "✓"
		printer := green
		if cat.Score < 70 {
			icon = "⚠"
			printer = yellow
		}
		if cat.Score < 50 {
			icon = "✗"
			printer = red
		}

		_, _ = printer.Printf("  %s %-14s", icon, cat.Name)
		fmt.Printf("  %5.1f / 100  (weight: %.0f%%)\n", cat.Score, cat.Weight*100)

		if cat.Details != "" {
			fmt.Printf("    %s\n", cat.Details)
		}
		for _, d := range cat.Deducts {
			_, _ = red.Printf("    -%.*f  ", 0, d.Points)
			fmt.Printf("%s\n", d.Reason)
		}
		fmt.Println()
	}

	// Summary
	fmt.Printf("  %s\n\n", result.Summary)
}

func formatGrade(result *skill.ScoreResult) string {
	green := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)
	red := color.New(color.FgRed, color.Bold)

	grade := string(result.Grade)
	switch result.Grade {
	case skill.GradeA, skill.GradeB:
		return green.Sprint(grade)
	case skill.GradeC:
		return yellow.Sprint(grade)
	default:
		return red.Sprint(grade)
	}
}

func cloneForScore(target string) (cloneRoot, scorePath string, err error) {
	tmpDir, err := os.MkdirTemp("", "ask-score-*")
	if err != nil {
		return "", "", err
	}

	url := target
	subDir := ""
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		parts := strings.SplitN(target, "/", 3)
		if len(parts) >= 2 {
			url = "https://github.com/" + parts[0] + "/" + parts[1]
			if len(parts) > 2 {
				subDir = parts[2]
			}
		} else {
			url = "https://github.com/" + target
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cloneErr := git.Clone(ctx, url, tmpDir)
	if cloneErr != nil {
		_ = os.RemoveAll(tmpDir)
		return "", "", cloneErr
	}

	if subDir != "" {
		targetPath := filepath.Join(tmpDir, filepath.Clean(subDir))
		if !strings.HasPrefix(targetPath, tmpDir+string(filepath.Separator)) {
			_ = os.RemoveAll(tmpDir)
			return "", "", fmt.Errorf("invalid path: traversal not allowed in %s", subDir)
		}
		if _, err := os.Stat(targetPath); err != nil {
			_ = os.RemoveAll(tmpDir)
			return "", "", fmt.Errorf("subdirectory %s not found in repository", subDir)
		}
		return tmpDir, targetPath, nil
	}
	return tmpDir, tmpDir, nil
}

func fetchPublisherInfo(target string) *skill.PublisherInfo {
	// Extract owner from target like "owner/repo" or "owner/repo/path"
	parts := strings.Split(strings.TrimPrefix(target, "https://github.com/"), "/")
	if len(parts) < 2 {
		return nil
	}
	owner := parts[0]
	repo := parts[1]

	info := &skill.PublisherInfo{
		Owner: owner,
	}

	// Fetch repo metadata via GitHub API
	client := github.NewClient()
	repoInfo, err := client.GetRepoInfo(owner, repo)
	if err == nil {
		info.RepoStars = repoInfo.Stars
		info.IsOrg = repoInfo.IsOrg
		info.HasLicense = repoInfo.HasLicense
		info.AccountAge = repoInfo.OwnerAge
		info.RepoForks = repoInfo.Forks
	}

	return info
}

func init() {
	skillCmd.AddCommand(scoreCmd)
	scoreCmd.Flags().Bool("json", false, "Output score as JSON")
	scoreCmd.Flags().Bool("batch", false, "Batch score all skills in a directory")
}
