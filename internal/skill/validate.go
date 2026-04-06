// Package skill provides core skill manipulation, validation, and security checking logic.
package skill

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// ValidationError represents a validation issue with a SKILL.md file
type ValidationError struct {
	Field    string
	Message  string
	Severity Severity
}

// nameRegex validates the name field per Agent Skills spec
// - 1-64 chars, lowercase a-z and hyphens only
// - no leading/trailing/consecutive hyphens
var nameRegex = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// ValidateMeta validates the Meta struct against the Agent Skills specification
// https://agentskills.io/specification
func ValidateMeta(meta *Meta, dirName string) []ValidationError {
	var validationErrors []ValidationError

	// Validate name (required)
	if meta.Name == "" {
		validationErrors = append(validationErrors, ValidationError{
			Field:    "name",
			Message:  "name is required",
			Severity: SeverityCritical,
		})
	} else {
		// Check length (1-64 chars)
		if len(meta.Name) > 64 {
			validationErrors = append(validationErrors, ValidationError{
				Field:    "name",
				Message:  fmt.Sprintf("name must be 1-64 characters, got %d", len(meta.Name)),
				Severity: SeverityCritical,
			})
		}

		// Check for uppercase letters
		for _, r := range meta.Name {
			if unicode.IsUpper(r) {
				validationErrors = append(validationErrors, ValidationError{
					Field:    "name",
					Message:  "name must be lowercase (a-z and hyphens only)",
					Severity: SeverityCritical,
				})
				break
			}
		}

		// Check for consecutive hyphens
		if strings.Contains(meta.Name, "--") {
			validationErrors = append(validationErrors, ValidationError{
				Field:    "name",
				Message:  "name must not contain consecutive hyphens (--)",
				Severity: SeverityCritical,
			})
		}

		// Check for leading/trailing hyphens
		if strings.HasPrefix(meta.Name, "-") || strings.HasSuffix(meta.Name, "-") {
			validationErrors = append(validationErrors, ValidationError{
				Field:    "name",
				Message:  "name must not start or end with a hyphen",
				Severity: SeverityCritical,
			})
		}

		// Check pattern match (lowercase alphanumeric with hyphens)
		if !nameRegex.MatchString(meta.Name) && len(meta.Name) > 0 {
			// Only report if not already caught by above checks
			hasOther := false
			for _, r := range meta.Name {
				if !unicode.IsLower(r) && !unicode.IsDigit(r) && r != '-' {
					hasOther = true
					break
				}
			}
			if hasOther {
				validationErrors = append(validationErrors, ValidationError{
					Field:    "name",
					Message:  "name may only contain lowercase letters (a-z), digits, and hyphens",
					Severity: SeverityCritical,
				})
			}
		}

		// Check name matches directory name (warning)
		if dirName != "" && meta.Name != dirName {
			validationErrors = append(validationErrors, ValidationError{
				Field:    "name",
				Message:  fmt.Sprintf("name '%s' should match directory name '%s'", meta.Name, dirName),
				Severity: SeverityWarning,
			})
		}
	}

	// Validate description (required)
	if meta.Description == "" {
		validationErrors = append(validationErrors, ValidationError{
			Field:    "description",
			Message:  "description is required",
			Severity: SeverityCritical,
		})
	} else if len(meta.Description) > 1024 {
		validationErrors = append(validationErrors, ValidationError{
			Field:    "description",
			Message:  fmt.Sprintf("description must be 1-1024 characters, got %d", len(meta.Description)),
			Severity: SeverityCritical,
		})
	}

	// Validate compatibility (optional, 1-500 chars if provided)
	if meta.Compatibility != "" && len(meta.Compatibility) > 500 {
		validationErrors = append(validationErrors, ValidationError{
			Field:    "compatibility",
			Message:  fmt.Sprintf("compatibility must be 1-500 characters, got %d", len(meta.Compatibility)),
			Severity: SeverityWarning,
		})
	}

	return validationErrors
}
