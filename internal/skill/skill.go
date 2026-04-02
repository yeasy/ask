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

// ParseSkillMD parses a SKILL.md file and extracts frontmatter metadata.
// Uses Lstat pre-check for symlinks, then open-then-fstat for size validation.
func ParseSkillMD(skillPath string) (*Meta, error) {
	skillMDPath := filepath.Join(skillPath, "SKILL.md")

	// Pre-check for symlinks (Lstat does not follow symlinks)
	linfo, err := os.Lstat(skillMDPath)
	if err != nil {
		return nil, err
	}
	if linfo.Mode()&os.ModeSymlink != 0 || !linfo.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file: %s", skillMDPath)
	}

	file, err := os.Open(skillMDPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file: %s", skillMDPath)
	}
	if fi.Size() > maxSkillFileSize {
		return nil, fmt.Errorf("skill file too large: %d bytes (max %d)", fi.Size(), maxSkillFileSize)
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), int(maxSkillFileSize))
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
// Uses Lstat pre-check, then opens the file and stats via the fd to avoid TOCTOU.
func parseFromContent(skillMDPath string) (*Meta, error) {
	// Pre-check for symlinks before opening (Lstat does not follow symlinks)
	linfo, err := os.Lstat(skillMDPath)
	if err != nil {
		return nil, err
	}
	if linfo.Mode()&os.ModeSymlink != 0 || !linfo.Mode().IsRegular() {
		return nil, fmt.Errorf("refusing to read non-regular file: %s", skillMDPath)
	}

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
	// Reject symlinks and non-regular files
	return fi.Mode()&os.ModeSymlink == 0 && fi.Mode().IsRegular()
}
