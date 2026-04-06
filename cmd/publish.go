package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/skill"
)

var publishCmd = &cobra.Command{
	Use:   "publish [skill-path]",
	Short: "Validate and prepare a skill for publishing",
	Long: `Validate a skill and prepare it for publishing to the ASK registry.

This command runs comprehensive checks including:
  - SKILL.md format validation
  - Security scanning
  - Version follows semver
  - Required files verification
  - Git repository status check

After validation passes, it generates a registry entry that can be
submitted to the awesome-agent-skills repository.`,
	Example: `  # Publish skill in current directory
  ask skill publish

  # Publish skill at a specific path
  ask skill publish ./my-skill

  # Generate registry entry to file
  ask skill publish --output registry-entry.json`,
	Args: cobra.MaximumNArgs(1),
	Run:  runPublish,
}

func runPublish(cmd *cobra.Command, args []string) {
	var targetPath string
	var err error

	if len(args) > 0 {
		targetPath, err = filepath.Abs(args[0])
	} else {
		targetPath, err = os.Getwd()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving path: %v\n", err)
		os.Exit(1)
	}

	output, _ := cmd.Flags().GetString("output")

	fmt.Printf("Preparing to publish skill at %s...\n\n", targetPath)

	allPassed := true

	// Step 1: Check SKILL.md exists
	fmt.Print("  Checking SKILL.md... ")
	if !skill.FindSkillMD(targetPath) {
		color.Red("FAIL")
		fmt.Println("    No SKILL.md found. Create one with 'ask skill create'.")
		os.Exit(1)
	}
	color.Green("OK")

	// Step 2: Validate SKILL.md format
	fmt.Print("  Validating SKILL.md format... ")
	meta, err := skill.ParseSkillMD(targetPath)
	if err != nil {
		color.Red("FAIL")
		fmt.Fprintf(os.Stderr, "    %v\n", err)
		os.Exit(1)
	}

	validationErrors := validateSkillMeta(meta)
	if len(validationErrors) > 0 {
		color.Red("FAIL")
		for _, e := range validationErrors {
			fmt.Printf("    - %s\n", e)
		}
		allPassed = false
	} else {
		color.Green("OK")
	}

	// Step 3: Version check (semver)
	fmt.Print("  Checking version (semver)... ")
	if meta.Version == "" {
		color.Yellow("MISSING (add version to SKILL.md frontmatter)")
	} else if !isValidSemver(meta.Version) {
		color.Yellow("WARN (not valid semver: %s)", meta.Version)
	} else {
		color.Green("OK (%s)", meta.Version)
	}

	// Step 4: Security scan
	fmt.Print("  Running security scan... ")
	result, err := skill.CheckSafety(targetPath)
	if err != nil {
		color.Red("FAIL")
		fmt.Fprintf(os.Stderr, "    %v\n", err)
		os.Exit(1)
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
		color.Red("FAIL (%d critical)", criticals)
		for _, f := range result.Findings {
			if f.Severity == skill.SeverityCritical {
				fmt.Printf("    - %s: %s (%s:%d)\n", f.RuleID, f.Description, f.File, f.Line)
			}
		}
		allPassed = false
	} else if warnings > 0 {
		color.Yellow("WARN (%d warnings)", warnings)
	} else {
		color.Green("OK")
	}

	// Step 5: Check for README
	fmt.Print("  Checking README.md... ")
	readmePath := filepath.Join(targetPath, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		color.Yellow("MISSING (recommended)")
	} else {
		color.Green("OK")
	}

	// Step 6: Check for prompt files
	fmt.Print("  Checking prompts/... ")
	promptDir := filepath.Join(targetPath, "prompts")
	if entries, err := os.ReadDir(promptDir); err != nil || len(entries) == 0 {
		color.Yellow("EMPTY (add prompt files)")
	} else {
		color.Green("OK (%d files)", len(entries))
	}

	// Step 7: Check git status
	fmt.Print("  Checking git status... ")
	gitRemote := getGitRemote(targetPath)
	gitCtx, gitCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer gitCancel()
	gitCmd := exec.CommandContext(gitCtx, "git", "-C", targetPath, "status", "--porcelain")
	gitOutput, err := gitCmd.Output()
	if err != nil {
		color.Yellow("SKIP (not a git repo)")
	} else if len(gitOutput) > 0 {
		color.Yellow("WARN (uncommitted changes)")
	} else {
		color.Green("OK")
	}

	// Step 8: Check git tag
	fmt.Print("  Checking git tag... ")
	if meta.Version != "" {
		tagCtx, tagCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer tagCancel()
		tagCmd := exec.CommandContext(tagCtx, "git", "-C", targetPath, "tag", "-l", "v"+meta.Version)
		tagOutput, tagErr := tagCmd.Output()
		if tagErr != nil {
			color.Yellow("SKIP (not a git repo)")
		} else if strings.TrimSpace(string(tagOutput)) == "" {
			color.Yellow("MISSING (run: git tag v%s)", meta.Version)
		} else {
			color.Green("OK (v%s)", meta.Version)
		}
	} else {
		color.Yellow("SKIP (no version)")
	}

	// Summary
	fmt.Println()
	if !allPassed {
		color.Red("✗ Skill has issues that must be fixed before publishing.\n")
		os.Exit(1)
	}

	color.Green("✓ Skill '%s' is ready for publishing!\n", meta.Name)

	// Generate registry entry
	entry := generateRegistryEntry(meta, targetPath, gitRemote)

	if output != "" {
		data, err := json.MarshalIndent(entry, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling registry entry: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(output, data, 0600); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing registry entry: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\nRegistry entry saved to %s\n", output)
	} else {
		fmt.Println()
		fmt.Println("Registry entry (add to awesome-agent-skills/registry/index.json):")
		fmt.Println()
		data, err := json.MarshalIndent(entry, "  ", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling registry entry: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("  %s\n", string(data))
	}

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. Push your skill to a public GitHub repository")
	if meta.Version != "" {
		fmt.Printf("  2. Tag your release: git tag v%s && git push --tags\n", meta.Version)
	}
	fmt.Println("  3. Submit to registry: https://github.com/yeasy/awesome-agent-skills")
}

// registryEntry matches the RegistrySkill struct format
type registryEntry struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	URL         string   `json:"url"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Tags        []string `json:"tags"`
	Stars       int      `json:"stars"`
	Featured    bool     `json:"featured"`
	InstallCmd  string   `json:"install_cmd"`
}

func generateRegistryEntry(meta *skill.Meta, skillPath, gitRemote string) registryEntry {
	source := filepath.Base(skillPath)
	skillURL := ""
	installCmd := "ask install " + meta.Name

	if gitRemote != "" {
		owner := parseOwnerFromRemote(gitRemote)
		if owner != "" {
			source = owner
		}
		skillURL = gitRemote
		skillURL = strings.TrimSuffix(skillURL, ".git")
		if strings.HasPrefix(skillURL, "git@github.com:") {
			skillURL = "https://github.com/" + strings.TrimPrefix(skillURL, "git@github.com:")
		}
		installCmd = "ask install " + strings.TrimPrefix(skillURL, "https://github.com/")
	}

	category := "general"
	if meta.Tags != nil {
		for _, tag := range meta.Tags {
			switch tag {
			case "productivity", "development", "creative", "business", "media", "security":
				category = tag
			}
		}
	}

	tags := meta.Tags
	if tags == nil {
		tags = []string{"agent-skill"}
	}

	return registryEntry{
		Name:        meta.Name,
		Source:      source,
		URL:         skillURL,
		Description: meta.Description,
		Category:    category,
		Tags:        tags,
		Stars:       0,
		Featured:    false,
		InstallCmd:  installCmd,
	}
}

func getGitRemote(path string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", path, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func parseOwnerFromRemote(remote string) string {
	if strings.HasPrefix(remote, "git@github.com:") {
		parts := strings.SplitN(strings.TrimPrefix(remote, "git@github.com:"), "/", 2)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	remote = strings.TrimSuffix(remote, ".git")
	remote = strings.TrimPrefix(remote, "https://github.com/")
	remote = strings.TrimPrefix(remote, "http://github.com/")
	parts := strings.SplitN(remote, "/", 2)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$`)

func isValidSemver(v string) bool {
	return semverRe.MatchString(v)
}

func validateSkillMeta(meta *skill.Meta) []string {
	var validationErrors []string
	if meta == nil {
		return []string{"failed to parse SKILL.md metadata"}
	}
	if meta.Name == "" {
		validationErrors = append(validationErrors, "name is required in SKILL.md frontmatter")
	}
	if meta.Description == "" {
		validationErrors = append(validationErrors, "description is required in SKILL.md frontmatter")
	}
	return validationErrors
}

func init() {
	skillCmd.AddCommand(publishCmd)
	publishCmd.Flags().StringP("output", "o", "", "Write registry entry to file")
}
