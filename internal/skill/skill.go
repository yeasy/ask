package skill

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Meta represents metadata parsed from SKILL.md
type Meta struct {
	Name          string            `yaml:"name"`
	Description   string            `yaml:"description"`
	Version       string            `yaml:"version"`
	Author        string            `yaml:"author"`
	Dependencies  []string          `yaml:"dependencies"`
	Tags          []string          `yaml:"tags"`
	License       string            `yaml:"license"`
	Compatibility string            `yaml:"compatibility"`
	Metadata      map[string]string `yaml:"metadata"`
	AllowedTools  []string          `yaml:"allowed-tools"`
}

// ParseSkillMD parses a SKILL.md file and extracts frontmatter metadata
func ParseSkillMD(skillPath string) (*Meta, error) {
	skillMDPath := filepath.Join(skillPath, "SKILL.md")

	// Reject symlinks to prevent path traversal
	if info, err := os.Lstat(skillMDPath); err != nil {
		return nil, err
	} else if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symlink rejected: %s", skillMDPath)
	}

	file, err := os.Open(skillMDPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	var frontmatter []string
	inFrontmatter := false
	lineCount := 0
	const maxFrontmatterLines = 500 // Prevent unbounded reads from unclosed frontmatter

	for scanner.Scan() {
		line := scanner.Text()
		lineCount++

		// Check for frontmatter delimiters
		if strings.TrimSpace(line) == "---" {
			if !inFrontmatter && lineCount <= 2 {
				inFrontmatter = true
				continue
			} else if inFrontmatter {
				// End of frontmatter
				break
			}
		}

		if inFrontmatter {
			frontmatter = append(frontmatter, line)
			if len(frontmatter) > maxFrontmatterLines {
				break // Frontmatter too large, likely unclosed
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if len(frontmatter) == 0 {
		// No frontmatter found, try to extract info from file content
		return parseFromContent(skillMDPath)
	}

	var meta Meta
	yamlContent := strings.Join(frontmatter, "\n")
	if err := yaml.Unmarshal([]byte(yamlContent), &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

// parseFromContent attempts to extract metadata from SKILL.md content.
// Opens the file then stats via the fd to avoid TOCTOU between check and read.
func parseFromContent(skillMDPath string) (*Meta, error) {
	f, err := os.Open(skillMDPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file: %s", skillMDPath)
	}
	if info.Size() > maxSkillFileSize {
		return nil, fmt.Errorf("skill file too large: %d bytes (max %d)", info.Size(), maxSkillFileSize)
	}
	content, err := io.ReadAll(io.LimitReader(f, maxSkillFileSize))
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(content), "\n")
	meta := &Meta{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for title (first H1)
		if strings.HasPrefix(line, "# ") && meta.Name == "" {
			meta.Name = strings.TrimPrefix(line, "# ")
		}
		// Look for description (first paragraph after title)
		if meta.Name != "" && meta.Description == "" && !strings.HasPrefix(line, "#") && line != "" {
			meta.Description = line
			break
		}
	}

	return meta, nil
}

// FindSkillMD checks if a skill has a SKILL.md file.
// Uses Lstat to avoid following symlinks.
func FindSkillMD(skillPath string) bool {
	skillMDPath := filepath.Join(skillPath, "SKILL.md")
	fi, err := os.Lstat(skillMDPath)
	if err != nil {
		return false
	}
	// Reject symlinks
	return fi.Mode()&os.ModeSymlink == 0
}
