package skill

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// TemplateData holds data for the skill template
type TemplateData struct {
	Name        string
	Description string
	Author      string
	Version     string
	Tags        []string
}

const skillMDTemplate = `---
name: "{{yamlEscape .Name}}"
description: "{{yamlEscape .Description}}"
version: "{{yamlEscape .Version}}"
author: "{{yamlEscape .Author}}"
tags:
{{- range .Tags}}
  - "{{yamlEscape .}}"
{{- end}}
---

# {{.Name}}

{{.Description}}

## Usage
Explain how to use this skill here.

## Scripts
- **hello**: Example script

## Resources
- [Reference](references/ref.md)
`

// GetGitAuthor returns the git author name from git config.
// Falls back to "User" if git config is unavailable.
func GetGitAuthor() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "config", "user.name")
	output, err := cmd.Output()
	if err != nil {
		return "User"
	}
	author := strings.TrimSpace(string(output))
	if author == "" {
		return "User"
	}
	return author
}

// CreateSkillTemplateWithData creates a new skill directory using provided template data
func CreateSkillTemplateWithData(data TemplateData, destDir string) error {
	if data.Version == "" {
		data.Version = "0.1.0"
	}
	if len(data.Tags) == 0 {
		data.Tags = []string{"agent-skill"}
	}
	return createSkillDir(data, destDir)
}

// CreateSkillTemplate creates a new skill directory with template files
func CreateSkillTemplate(name, destDir string) error {
	data := TemplateData{
		Name:        name,
		Description: "A new skill for AI Agents",
		Author:      GetGitAuthor(),
		Version:     "0.1.0",
		Tags:        []string{"agent-skill"},
	}
	return createSkillDir(data, destDir)
}

func createSkillDir(data TemplateData, destDir string) error {
	name := data.Name
	skillDir := filepath.Join(destDir, name)

	// 1. Create directory structure
	dirs := []string{
		skillDir,
		filepath.Join(skillDir, "prompts"),
		filepath.Join(skillDir, "scripts"),
		filepath.Join(skillDir, "references"),
		filepath.Join(skillDir, "assets"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// 2. Create SKILL.md
	funcMap := template.FuncMap{
		"yamlEscape": func(s string) string {
			s = strings.ReplaceAll(s, `\`, `\\`)
			s = strings.ReplaceAll(s, `"`, `\"`)
			s = strings.ReplaceAll(s, "\n", `\n`)
			s = strings.ReplaceAll(s, "\r", `\r`)
			s = strings.ReplaceAll(s, "\t", `\t`)
			s = strings.ReplaceAll(s, "\x00", "")
			// Escape Unicode line breaks that YAML treats as newlines
			s = strings.ReplaceAll(s, "\u0085", `\x85`)
			s = strings.ReplaceAll(s, "\u2028", `\u2028`)
			s = strings.ReplaceAll(s, "\u2029", `\u2029`)
			return s
		},
	}
	tmpl, err := template.New("skill").Funcs(funcMap).Parse(skillMDTemplate)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	f, err := os.Create(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return fmt.Errorf("failed to create SKILL.md: %w", err)
	}
	closeSkillMD := true
	defer func() {
		if closeSkillMD {
			_ = f.Close()
		}
	}()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Close explicitly to catch write errors (deferred close would discard them)
	closeSkillMD = false
	if err := f.Close(); err != nil {
		return fmt.Errorf("failed to write SKILL.md: %w", err)
	}

	// 3. Create example script (use single quotes to prevent shell injection)
	scriptContent := fmt.Sprintf("#!/bin/bash\necho 'Hello from %s skill!'\n", strings.ReplaceAll(name, "'", "'\\''"))
	scriptFile := filepath.Join(skillDir, "scripts", "hello.sh")
	if err := os.WriteFile(scriptFile, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to create example script: %w", err)
	}

	// 4. Create example reference
	refContent := `# Reference
This is an example reference file.
`
	if err := os.WriteFile(filepath.Join(skillDir, "references", "ref.md"), []byte(refContent), 0644); err != nil {
		return fmt.Errorf("failed to create example reference: %w", err)
	}

	// 5. Create example prompt
	promptContent := `# Example Prompt

This is an example prompt file. Replace this with your actual prompt content.

## Usage

Describe how this prompt should be used by the AI agent.

## Variables

- Variable1: Description of variable 1
- Variable2: Description of variable 2
`
	if err := os.WriteFile(filepath.Join(skillDir, "prompts", "example.md"), []byte(promptContent), 0644); err != nil {
		return fmt.Errorf("failed to create example prompt: %w", err)
	}

	// 6. Create README.md
	readmeContent := fmt.Sprintf(`# %s

A skill for AI Agents to [describe what this skill does].

## Description

[Add a detailed description of your skill here. Explain what it does, when to use it, and how it benefits users.]

## Features

- [Feature 1]
- [Feature 2]
- [Feature 3]

## Installation

`+"`ask install <your-github-username>/%s`"+`

## Usage

[Explain how to use this skill. Provide examples if applicable.]

### Example

[Provide a concrete example of how to use the skill]

## Requirements

[List any external dependencies, API keys, or environment variables needed]

## Configuration

[Explain any configuration options or environment variables]

### Environment Variables

See `+"`.env.example`"+` for required environment variables.

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Add tests if applicable
4. Submit a pull request

## License

[Specify your license, e.g., MIT, Apache 2.0, etc.]

## Support

For issues and questions, please open an issue on GitHub.
`, name, name)
	if err := os.WriteFile(filepath.Join(skillDir, "README.md"), []byte(readmeContent), 0644); err != nil {
		return fmt.Errorf("failed to create README.md: %w", err)
	}

	// 7. Create .env.example
	envExampleContent := `# Environment variables for ` + name + `

# Example configuration - copy to .env and fill in your values

# API Keys
# API_KEY=your_api_key_here

# Configuration
# DEBUG=false
# TIMEOUT=30
`
	if err := os.WriteFile(filepath.Join(skillDir, ".env.example"), []byte(envExampleContent), 0644); err != nil {
		return fmt.Errorf("failed to create .env.example: %w", err)
	}

	return nil
}
