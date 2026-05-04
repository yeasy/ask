package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/skill"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed skills",
	Long: `List all skills currently installed.
Use --global to show global skills, --all to show both project and global skills.
Use --agent (-a) to list skills for specific agents (checks agent directories).`,
	Run: runList,
}

func runList(cmd *cobra.Command, _ []string) {
	global, _ := cmd.Flags().GetBool("global")
	all, _ := cmd.Flags().GetBool("all")
	agents, _ := cmd.Flags().GetStringSlice("agent")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	// Ensure project is initialized for non-global operations
	if !global && !all && len(agents) == 0 {
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

	var collectedItems []SkillListItem

	if len(agents) > 0 {
		// Show skills for specific agents by checking directories
		for _, agentName := range agents {
			agentType, _ := config.ResolveAgentType(agentName)

			if all || (!global) {
				// Project level
				dir, _ := config.GetAgentSkillsDir(agentType, false)
				items := showAgentSkills(agentName, dir, "Project", jsonOutput)
				if jsonOutput {
					collectedItems = append(collectedItems, items...)
				}
			}

			if all || global {
				// Global level
				dir, _ := config.GetAgentSkillsDir(agentType, true)
				items := showAgentSkills(agentName, dir, "Global", jsonOutput)
				if jsonOutput {
					collectedItems = append(collectedItems, items...)
				}
			}
		}
	} else {
		if all {
			// Show both project and global skills from config
			items := showSkills("Project", false, jsonOutput)
			if jsonOutput {
				collectedItems = append(collectedItems, items...)
			}
			if !jsonOutput {
				fmt.Println()
			}
			itemsGlobal := showSkills("Global", true, jsonOutput)
			if jsonOutput {
				collectedItems = append(collectedItems, itemsGlobal...)
			}
		} else {
			scope := "Project"
			if global {
				scope = "Global"
			}
			items := showSkills(scope, global, jsonOutput)
			if jsonOutput {
				collectedItems = append(collectedItems, items...)
			}
		}
	}

	if jsonOutput {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(collectedItems); err != nil {
			fmt.Fprintf(os.Stderr, "Error encoding JSON: %v\n", err)
		}
	}
}

// SkillListItem represents a skill in the list output
type SkillListItem struct {
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	URL         string   `json:"url,omitempty"`
	Scope       string   `json:"scope"`
	Agent       string   `json:"agent,omitempty"`
	Agents      []string `json:"agents,omitempty"`
	Path        string   `json:"path,omitempty"`
}

func showAgentSkills(agentName, dir, scope string, jsonOutput bool) []SkillListItem {
	var items []SkillListItem

	if !jsonOutput {
		fmt.Printf("%s Skills for %s (%s):\n\n", scope, agentName, dir)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if !jsonOutput {
			if os.IsNotExist(err) {
				fmt.Println("  (directory not created)")
			} else {
				fmt.Fprintf(os.Stderr, "  Error reading directory: %v\n", err)
			}
			fmt.Println()
		}
		return nil
	}

	if len(entries) == 0 {
		if !jsonOutput {
			fmt.Println("  (none)")
			fmt.Println()
		}
		return nil
	}

	type agentRow struct {
		name    string
		version string
		desc    string
	}
	var rows []agentRow

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillPath := filepath.Join(dir, entry.Name())
		item := SkillListItem{
			Name:  entry.Name(),
			Scope: scope,
			Agent: agentName,
			Path:  skillPath,
		}

		version := ""
		desc := ""
		if skill.FindSkillMD(skillPath) {
			if meta, parseErr := skill.ParseSkillMD(skillPath); parseErr == nil && meta != nil {
				desc = meta.Description
				version = meta.Version
				item.Description = desc
				item.Version = version
			}
		}

		if jsonOutput {
			items = append(items, item)
		} else {
			rows = append(rows, agentRow{name: entry.Name(), version: version, desc: desc})
		}
	}

	if !jsonOutput {
		if len(rows) == 0 {
			fmt.Println("  (none)")
		} else {
			nameW, verW := 4, 7
			for _, r := range rows {
				if len(r.name) > nameW {
					nameW = len(r.name)
				}
				if len(r.version) > verW {
					verW = len(r.version)
				}
			}
			header := fmt.Sprintf("  %-*s  %-*s  DESCRIPTION", nameW, "NAME", verW, "VERSION")
			fmt.Println(header)
			fmt.Println("  " + strings.Repeat("-", len(header)-2))
			for _, r := range rows {
				ver := r.version
				if ver == "" {
					ver = "-"
				}
				d := r.desc
				if d == "" {
					d = "-"
				}
				d = truncateStr(d, 50)
				fmt.Printf("  %-*s  %-*s  %s\n", nameW, r.name, verW, ver, d)
			}
		}
		fmt.Println()
	}
	return items
}

func showSkills(scope string, global bool, jsonOutput bool) []SkillListItem {
	var items []SkillListItem
	cfg, err := config.LoadConfigByScope(global)
	if err != nil {
		if !jsonOutput {
			if os.IsNotExist(err) && !global {
				fmt.Printf("%s Skills: No ask.yaml found. Run 'ask init' first.\n", scope)
				return nil
			}
			if !global {
				fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			}
		}
		return nil
	}

	if len(cfg.Skills) == 0 && len(cfg.SkillsInfo) == 0 {
		if !jsonOutput {
			fmt.Printf("%s Skills: (none)\n", scope)
		}
		return nil
	}

	// Collect all skill data first
	type skillRow struct {
		name    string
		version string
		agents  []string
		item    SkillListItem
	}
	var rows []skillRow

	// Load lock file for version info
	lockFile, _ := config.LoadLockFile()
	lockVersions := make(map[string]string)
	if lockFile != nil {
		for _, entry := range lockFile.Skills {
			lockVersions[entry.Name] = entry.Version
		}
	}

	shown := make(map[string]bool)
	for _, s := range cfg.SkillsInfo {
		shown[s.Name] = true
		version := lockVersions[s.Name]
		agentList := detectSkillAgents(s.Name)
		item := SkillListItem{
			Name:        s.Name,
			Version:     version,
			Description: s.Description,
			URL:         s.URL,
			Scope:       scope,
			Agents:      agentList,
		}
		items = append(items, item)
		rows = append(rows, skillRow{name: s.Name, version: version, agents: agentList, item: item})
	}

	for _, s := range cfg.Skills {
		if !shown[s] {
			version := lockVersions[s]
			agentList := detectSkillAgents(s)
			item := SkillListItem{
				Name:    s,
				Version: version,
				Scope:   scope,
				Agents:  agentList,
			}
			items = append(items, item)
			rows = append(rows, skillRow{name: s, version: version, agents: agentList, item: item})
		}
	}

	if !jsonOutput {
		fmt.Printf("%s Skills:\n\n", scope)

		// Calculate column widths
		nameW, verW := 4, 7 // minimum: "NAME", "VERSION"
		for _, r := range rows {
			if len(r.name) > nameW {
				nameW = len(r.name)
			}
			if len(r.version) > verW {
				verW = len(r.version)
			}
		}

		// Print header
		header := fmt.Sprintf("  %-*s  %-*s  AGENTS", nameW, "NAME", verW, "VERSION")
		fmt.Println(header)
		fmt.Println("  " + strings.Repeat("-", len(header)-2))

		// Print rows
		for _, r := range rows {
			ver := r.version
			if ver == "" {
				ver = "-"
			}
			agentStr := "-"
			if len(r.agents) > 0 {
				agentStr = strings.Join(r.agents, ", ")
			}
			fmt.Printf("  %-*s  %-*s  %s\n", nameW, r.name, verW, ver, agentStr)
		}
		fmt.Println()
	}

	return items
}

func registerListFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("all", false, "show both project and global skills")
	cmd.Flags().StringSliceP("agent", "a", []string{}, "list skills for specific agent(s)")
	cmd.Flags().Bool("json", false, "output results in JSON format")
}

// detectSkillAgents checks which agent directories contain a given skill
func detectSkillAgents(skillName string) []string {
	var agents []string
	cwd, err := os.Getwd()
	if err != nil {
		return nil
	}

	for _, agentName := range config.GetSupportedAgentNames() {
		agentType, ok := config.ResolveAgentType(agentName)
		if !ok {
			continue
		}
		agentCfg := config.SupportedAgents[agentType]
		skillPath := filepath.Join(cwd, agentCfg.ProjectDir, skillName)
		if _, err := os.Stat(skillPath); err == nil {
			agents = append(agents, agentName)
		}
	}

	return agents
}

func init() {
	skillCmd.AddCommand(listCmd)
	registerListFlags(listCmd)

	// Register agent flag completion
	_ = listCmd.RegisterFlagCompletionFunc("agent", completeAgentNames)
}
