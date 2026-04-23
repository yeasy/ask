package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/skill"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create <name>",
	Short: "Create a new skill from a template",
	Long: `Create a new skill directory with a standardized structure.
This will generate a SKILL.md and necessary subdirectories.

In interactive mode (default), you'll be prompted for description,
author, version, and compatible agents. Use --yes to skip prompts.`,
	Example: `  # Interactive mode
  ask skill create my-skill

  # Non-interactive mode with defaults
  ask skill create my-skill --yes

  # Specify details via flags
  ask skill create my-skill --description "My awesome skill" --author "Alice"`,
	Args: cobra.ExactArgs(1),
	Run:  runCreate,
}

func runCreate(cmd *cobra.Command, args []string) {
	name := args[0]

	// Validate name
	match, _ := regexp.MatchString("^[a-zA-Z0-9][a-zA-Z0-9-]*$", name)
	if !match {
		fmt.Fprintln(os.Stderr, "Error: Skill name must start with an alphanumeric character and contain only alphanumeric characters and dashes.")
		os.Exit(1)
	}

	// Check if directory already exists
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current working directory: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stat(filepath.Join(cwd, name)); err == nil {
		fmt.Fprintf(os.Stderr, "Error: Directory '%s' already exists.\n", name)
		os.Exit(1)
	}

	yes, _ := cmd.Flags().GetBool("yes")

	data := skill.TemplateData{
		Name:        name,
		Description: "A new skill for AI Agents",
		Author:      skill.GetGitAuthor(),
		Version:     "0.1.0",
		Tags:        []string{"agent-skill"},
	}

	// Override from flags if provided
	if desc, _ := cmd.Flags().GetString("description"); desc != "" {
		data.Description = desc
	}
	if author, _ := cmd.Flags().GetString("author"); author != "" {
		data.Author = author
	}

	if !yes {
		// Interactive mode
		err := runCreateInteractive(&data)
		if err != nil {
			fmt.Printf("Canceled.\n")
			return
		}
	}

	fmt.Printf("Creating skill '%s'...\n", name)

	if err := skill.CreateSkillTemplateWithData(data, cwd); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating skill: %v\n", err)
		os.Exit(1)
	}

	// Create .askcheck.yaml
	checkContent := "# ASK security check configuration\nignore: []\nignore_paths:\n  - assets/**\nrules: []\n"
	if err := os.WriteFile(filepath.Join(cwd, name, ".askcheck.yaml"), []byte(checkContent), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to create .askcheck.yaml: %v\n", err)
	}

	fmt.Printf("\n✓ Created skill '%s'!\n\n", name)
	fmt.Printf("  %s/\n", name)
	fmt.Println("  ├── SKILL.md")
	fmt.Println("  ├── prompts/")
	fmt.Println("  ├── scripts/")
	fmt.Println("  ├── references/")
	fmt.Println("  ├── assets/")
	fmt.Println("  ├── README.md")
	fmt.Println("  ├── .askcheck.yaml")
	fmt.Println("  └── .env.example")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. cd %s\n", name)
	fmt.Println("  2. Edit prompts/example.md with your skill logic")
	fmt.Printf("  3. Run: ask check %s/\n", name)
	fmt.Printf("  4. Publish: ask skill publish %s/\n", name)
}

func runCreateInteractive(data *skill.TemplateData) error {
	// Get supported agent names for the agent selection
	agentNames := config.GetSupportedAgentNames()
	var agentOptions []huh.Option[string]
	for _, name := range agentNames {
		agentOptions = append(agentOptions, huh.NewOption(name, name))
	}

	var selectedAgents []string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Description").
				Description("What does this skill do?").
				Value(&data.Description),
			huh.NewInput().
				Title("Author").
				Description("Your name or GitHub username").
				Value(&data.Author),
			huh.NewInput().
				Title("Version").
				Description("Initial version (semver)").
				Value(&data.Version),
		),
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Compatible agents").
				Description("Which agents should this skill work with?").
				Options(agentOptions...).
				Value(&selectedAgents),
		),
	)

	if err := form.Run(); err != nil {
		return err
	}

	// Build tags from agents
	data.Tags = []string{"agent-skill"}
	data.Tags = append(data.Tags, selectedAgents...)

	return nil
}

func init() {
	skillCmd.AddCommand(createCmd)
	createCmd.Flags().Bool("yes", false, "Non-interactive mode, use defaults")
	createCmd.Flags().String("description", "", "Skill description")
	createCmd.Flags().String("author", "", "Skill author")
}
