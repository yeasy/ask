package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/installer"
)

// Skill packs for quickstart
var skillPacks = []struct {
	Name        string
	Description string
	Skills      []string
}{
	{
		Name:        "essentials",
		Description: "Essential skills for any agent (browser-use, pdf, filesystem)",
		Skills:      []string{"anthropics/browser-use", "anthropics/pdf", "anthropics/filesystem"},
	},
	{
		Name:        "developer",
		Description: "Developer productivity skills (code-review, git-helper, testing)",
		Skills:      []string{"anthropics/code-review", "anthropics/git-helper", "anthropics/testing"},
	},
}

var quickstartCmd = &cobra.Command{
	Use:   "quickstart [pack-name]",
	Short: "Install recommended skill packs",
	Long: `Quickly install curated collections of skills.

Available packs:
  essentials  - Essential skills for any agent
  developer   - Developer productivity skills

If no pack is specified, lists all available packs.`,
	Example: `  # List available packs
  ask quickstart

  # Install a pack
  ask quickstart essentials`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Println("Available Skill Packs:")
			fmt.Println()
			for _, pack := range skillPacks {
				fmt.Printf("  %-14s %s\n", pack.Name, pack.Description)
				fmt.Printf("  %s  Skills: %s\n", strings.Repeat(" ", 14), strings.Join(pack.Skills, ", "))
				fmt.Println()
			}
			fmt.Println("Usage: ask quickstart <pack-name>")
			return
		}

		packName := args[0]
		var selectedPack *struct {
			Name        string
			Description string
			Skills      []string
		}

		for i := range skillPacks {
			if skillPacks[i].Name == packName {
				selectedPack = &skillPacks[i]
				break
			}
		}

		if selectedPack == nil {
			fmt.Printf("Unknown pack '%s'. Available packs:\n", packName)
			for _, pack := range skillPacks {
				fmt.Printf("  %s - %s\n", pack.Name, pack.Description)
			}
			os.Exit(1)
		}

		// Ensure initialized
		if !ensureInitialized() {
			return
		}

		global, _ := cmd.Flags().GetBool("global")
		agents, _ := cmd.Flags().GetStringSlice("agent")

		cfg, err := config.LoadConfig()
		if err != nil {
			def := config.DefaultConfig()
			cfg = &def
		}

		opts := installer.InstallOptions{
			Global: global,
			Agents: agents,
			Config: cfg,
		}

		fmt.Printf("Installing %s pack (%d skills)...\n\n", selectedPack.Name, len(selectedPack.Skills))

		var succeeded, failed int
		for _, skillInput := range selectedPack.Skills {
			err := installer.Install(skillInput, opts)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  ✗ Failed: %s (%v)\n", skillInput, err)
				failed++
			} else {
				fmt.Printf("  ✓ Installed: %s\n", skillInput)
				succeeded++
			}
		}

		fmt.Printf("\nDone! %d installed, %d failed.\n", succeeded, failed)
		if succeeded > 0 {
			fmt.Println("\nUse 'ask list' to see installed skills.")
		}
	},
}

func init() {
	rootCmd.AddCommand(quickstartCmd)
	quickstartCmd.Flags().StringSliceP("agent", "a", []string{}, "Target agent(s)")
}
