package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/skill"
)

// infoCmd represents the info command
var infoCmd = &cobra.Command{
	Use:   "info [skill-name]",
	Short: "Show detailed information about a skill",
	Long: `Display detailed metadata about an installed skill.
Reads from the SKILL.md file if available.
Use --global to check global skills.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		global, _ := cmd.Flags().GetBool("global")
		jsonOutput, _ := cmd.Flags().GetBool("json")
		skillName := args[0]

		// Ensure project is initialized for non-global operations
		if !global {
			if !ensureInitialized() {
				return
			}
		}

		skillsDir, err := config.GetSkillsDirByScope(global)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		skillPath := filepath.Join(skillsDir, skillName)

		// Check if skill exists
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			scopeLabel := "project"
			if global {
				scopeLabel = "global"
			}
			fmt.Printf("Skill '%s' is not installed (%s).\n", skillName, scopeLabel)
			os.Exit(1)
		}

		scopeLabel := "Project"
		if global {
			scopeLabel = "Global"
		}

		type SkillInfo struct {
			Name         string   `json:"name"`
			Description  string   `json:"description,omitempty"`
			Version      string   `json:"version,omitempty"`
			Author       string   `json:"author,omitempty"`
			Dependencies []string `json:"dependencies,omitempty"`
			Tags         []string `json:"tags,omitempty"`
			Path         string   `json:"path"`
			Scope        string   `json:"scope"`
			Files        []string `json:"files,omitempty"`
		}

		info := SkillInfo{
			Name:  skillName,
			Path:  skillPath,
			Scope: scopeLabel,
		}

		// Try to parse SKILL.md
		if skill.FindSkillMD(skillPath) {
			meta, err := skill.ParseSkillMD(skillPath)
			if err != nil {
				if !jsonOutput {
					fmt.Fprintf(os.Stderr, "Warning: Could not parse SKILL.md: %v\n", err)
				}
			} else {
				info.Name = meta.Name // Use name from metadata if available
				info.Description = meta.Description
				info.Version = meta.Version
				info.Author = meta.Author
				info.Dependencies = meta.Dependencies
				info.Tags = meta.Tags
			}
		}

		// List files in skill directory
		entries, err := os.ReadDir(skillPath)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					info.Files = append(info.Files, entry.Name()+"/")
				} else {
					info.Files = append(info.Files, entry.Name())
				}
			}
		}

		if jsonOutput {
			encoder := json.NewEncoder(os.Stdout)
			encoder.SetIndent("", "  ")
			if err := encoder.Encode(info); err != nil {
				fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
			}
			return
		}

		// Text Output
		fmt.Printf("Skill: %s (%s)\n", info.Name, info.Scope)
		fmt.Printf("Path:  %s\n", info.Path)
		fmt.Println()

		if info.Description != "" {
			fmt.Printf("  Name:        %s\n", info.Name)
			fmt.Printf("  Description: %s\n", info.Description)
			if info.Version != "" {
				fmt.Printf("  Version:     %s\n", info.Version)
			}
			if info.Author != "" {
				fmt.Printf("  Author:      %s\n", info.Author)
			}
			if len(info.Dependencies) > 0 {
				fmt.Printf("  Dependencies:\n")
				for _, dep := range info.Dependencies {
					fmt.Printf("    - %s\n", dep)
				}
			}
			if len(info.Tags) > 0 {
				fmt.Printf("  Tags: %v\n", info.Tags)
			}
		} else {
			if !skill.FindSkillMD(skillPath) {
				fmt.Println("  No SKILL.md found.")
			}
		}

		fmt.Println()
		fmt.Println("Files:")
		for _, file := range info.Files {
			if len(file) > 0 && file[len(file)-1] == '/' {
				fmt.Printf("  📁 %s\n", file)
			} else {
				fmt.Printf("  📄 %s\n", file)
			}
		}
	},
}

func init() {
	skillCmd.AddCommand(infoCmd)
	infoCmd.Flags().Bool("global", false, "check global skills")
	infoCmd.Flags().Bool("json", false, "output results in JSON format")

	// Register installed skill name completion
	infoCmd.ValidArgsFunction = completeInstalledSkills
}
