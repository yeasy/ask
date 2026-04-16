package skill

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// ScanResult represents a found skill on disk
type ScanResult struct {
	Path string `json:"path"`
	Meta *Meta  `json:"meta"`
}

// ScanDirectory recursively scans a directory for skills (directories containing SKILL.md)
// limitDepth prevents infinite recursion. Default recommendation: 3-5
func ScanDirectory(root string, limitDepth int) ([]ScanResult, error) {
	var results []ScanResult

	// Use WalkDir for efficiency
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if path == root {
				return err // Propagate errors for the root path itself
			}
			return nil // Skip permission errors for subdirectories
		}

		// Check depth
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		depth := strings.Count(rel, string(filepath.Separator)) + 1
		if rel == "." {
			depth = 0
		}
		if depth > limitDepth && d.IsDir() {
			return fs.SkipDir
		}

		// Security: Skip symlinks
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}

		// Verify if it is a skill
		if d.IsDir() {
			// Skip hidden directories (like .git, .agent)
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return fs.SkipDir
			}

			if FindSkillMD(path) {
				meta, err := ParseSkillMD(path)
				if err == nil {
					results = append(results, ScanResult{
						Path: path,
						Meta: meta,
					})
				}
				// If we found a skill, do we want to search INSIDE it for more skills?
				// Usually skills aren't nested. But let's allow shallow nesting just in case,
				// or valid "collections".
				// For now, let's continue walking.
			}
		}
		return nil
	})

	return results, err
}
