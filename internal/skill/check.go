// Package skill provides core skill manipulation and security checking logic.
package skill

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Severity represents the severity of a security finding
type Severity string

const (
	// SeverityInfo indicates informational findings
	SeverityInfo Severity = "INFO"
	// SeverityWarning indicates potential issues
	SeverityWarning Severity = "WARNING"
	// SeverityCritical indicates critical vulnerabilities
	SeverityCritical Severity = "CRITICAL"
)

// Rule represents a security check rule
type Rule struct {
	ID          string
	Description string
	Severity    Severity
	Regex       *regexp.Regexp
	Entropy     float64 // Minimum entropy threshold (0 to ignore)
	Tags        []string
}

// Finding represents a single security issue found in a skill
type Finding struct {
	RuleID      string
	Severity    Severity
	Description string
	Module      string // The skill or module name where this finding occurred
	File        string
	Line        int
	Match       string
}

// CheckResult contains all findings for a skill
type CheckResult struct {
	SkillName      string
	Findings       []Finding
	ScannedModules []string // List of all modules scanned, including clean ones
}

// Rules definition
var defaultRules = []Rule{
	// Secrets
	{
		ID:          "SECRET-AWS-KEY",
		Description: "Potential AWS Access Key ID found",
		Severity:    SeverityCritical,
		Regex:       regexp.MustCompile(`(A3T[A-Z0-9]|AKIA|AGPA|AIDA|AROA|AIPA|ANPA|ANVA|ASIA)[A-Z0-9]{16}`),
		Entropy:     0,
	},
	{
		ID:          "SECRET-PRIVATE-KEY",
		Description: "Private Key found",
		Severity:    SeverityCritical,
		Regex:       regexp.MustCompile(`-----BEGIN (RSA|DSA|EC|OPENSSH|PGP) PRIVATE KEY-----`),
		Entropy:     0,
	},
	{
		ID:          "SECRET-GENERIC-TOKEN",
		Description: "High entropy string assigned to likely secret variable",
		Severity:    SeverityCritical,
		// Looks for "key = '...'" or "token: "..." patterns
		Regex:   regexp.MustCompile(`(?i)(api_?key|access_?token|secret|password|passwd|pwd)[ \t]*[:=][ \t]*['"]([a-zA-Z0-9_\-\.=]{8,})['"]`),
		Entropy: 4.0, // Require some randomness
	},
	{
		ID:          "SECRET-SLACK-TOKEN",
		Description: "Slack Token found",
		Severity:    SeverityCritical,
		Regex:       regexp.MustCompile(`xox[baprs]-([0-9a-zA-Z]{10,48})`),
		Entropy:     0,
	},
	{
		ID:          "SECRET-GOOGLE-API",
		Description: "Google API Key found",
		Severity:    SeverityCritical,
		Regex:       regexp.MustCompile(`AIza[0-9A-Za-z\\-_]{35}`),
		Entropy:     0,
	},

	// Dangerous Commands
	{
		ID:          "CMD-RM-RF",
		Description: "Dangerous use of 'rm -rf' on root-level directories",
		Severity:    SeverityWarning,
		Regex:       regexp.MustCompile(`rm\s+(-[a-zA-Z]*r[a-zA-Z]*f\s+.*|-[a-zA-Z]*f[a-zA-Z]*r\s+.*)`),
		Entropy:     0,
	},
	{
		ID:          "CMD-SUDO",
		Description: "Usage of 'sudo' detected",
		Severity:    SeverityWarning,
		Regex:       regexp.MustCompile(`sudo\s+`),
		Entropy:     0,
	},
	{
		ID:          "CMD-CHMOD-777",
		Description: "Usage of 'chmod 777' is insecure",
		Severity:    SeverityWarning,
		Regex:       regexp.MustCompile(`chmod\s+.*777`),
		Entropy:     0,
	},
	{
		ID:          "CMD-REV-SHELL",
		Description: "Potential reverse shell detected",
		Severity:    SeverityCritical,
		Regex:       regexp.MustCompile(`(nc|netcat)\s+-e|/dev/tcp/|bash\s+-i`),
		Entropy:     0,
	},
	{
		ID:          "CMD-OBFUSCATION",
		Description: "Obfuscated command execution detected",
		Severity:    SeverityWarning,
		Regex:       regexp.MustCompile(`(eval|base64\s+-d|openssl\s+enc\s+-d)`),
		Entropy:     0,
	},

	// Network
	{
		ID:          "NET-HTTP",
		Description: "Insecure HTTP URL detected",
		Severity:    SeverityInfo,
		// Exclude common license and harmless URLs to reduce noise (filtered in scanFile)
		Regex:   regexp.MustCompile(`http://[a-zA-Z0-9\-\.]+\.[a-zA-Z]{2,}`),
		Entropy: 0,
	},
	{
		ID:          "NET-IP-ADDR",
		Description: "Hardcoded IP address detected",
		Severity:    SeverityInfo,
		Regex:       regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`),
		Entropy:     0,
	},
}

// CheckSafety performs security checks on a skill directory.
// It loads .askcheck.yaml (if present) to support custom rules, rule ignoring, and path exclusions.
func CheckSafety(skillPath string) (*CheckResult, error) {
	meta, err := ParseSkillMD(skillPath)
	if err != nil {
		// If SKILL.md is missing or unparseable, proceed with a fallback name
		// derived from the directory. We still want to scan files for security issues.
		meta = &Meta{Name: filepath.Base(skillPath)}
	}

	result := &CheckResult{
		SkillName: meta.Name,
		Findings:  []Finding{},
	}

	// Load .askcheck.yaml if present
	checkCfg, err := LoadCheckConfig(skillPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load .askcheck.yaml: %w", err)
	}

	// Mark as skill-bundled so CRITICAL rules cannot be ignored
	if checkCfg != nil {
		checkCfg.SkillBundled = true
	}

	// Build effective rules (defaults minus ignored, plus custom)
	rules := checkCfg.BuildRules()

	// Build ignored rule set for filtering validation findings too.
	// Respect the same SkillBundled restriction: CRITICAL built-in rules stay enforced.
	ignoreSet := make(map[string]bool)
	if checkCfg != nil {
		protected := protectedRuleIDs()
		for _, id := range checkCfg.Ignore {
			upper := strings.ToUpper(id)
			if checkCfg.SkillBundled && protected[upper] {
				continue
			}
			ignoreSet[upper] = true
		}
	}

	// Validate SKILL.md format per Agent Skills specification
	dirName := filepath.Base(skillPath)
	validationErrors := ValidateMeta(meta, dirName)
	for _, ve := range validationErrors {
		ruleID := "SKILL-FORMAT-" + strings.ToUpper(ve.Field)
		if ignoreSet[ruleID] {
			continue
		}
		result.Findings = append(result.Findings, Finding{
			RuleID:      ruleID,
			Severity:    ve.Severity,
			Description: ve.Message,
			File:        "SKILL.md",
			Line:        0,
			Match:       ve.Field,
		})
	}

	// Walk through the skill directory
	err = filepath.WalkDir(skillPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Skip permission errors and other access issues
			return nil
		}

		// Skip symlinks to prevent following links outside intended directory
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		relPath, relErr := filepath.Rel(skillPath, path)
		if relErr != nil {
			return nil
		}

		// Check path exclusions from .askcheck.yaml
		if checkCfg != nil && checkCfg.IsPathIgnored(relPath) {
			return nil
		}

		// Skip binary files/images based on extension
		ext := strings.ToLower(filepath.Ext(path))
		if isBinaryExt(ext) {
			return nil
		}

		// Skip files that are too large to scan safely
		if info, infoErr := d.Info(); infoErr == nil && info.Size() > maxSkillFileSize {
			return nil
		}

		// Check for suspicious extensions
		if isSuspiciousExt(ext) && !ignoreSet["FILE-SUSPICIOUS-EXT"] {
			result.Findings = append(result.Findings, Finding{
				RuleID:      "FILE-SUSPICIOUS-EXT",
				Severity:    SeverityWarning,
				Description: fmt.Sprintf("Suspicious file extension found: %s", ext),
				File:        relPath,
				Line:        0,
				Match:       filepath.Base(path),
			})
		}

		findings, scanErr := scanFile(path, skillPath, rules)
		if scanErr != nil {
			return fmt.Errorf("failed to scan file %s: %w", path, scanErr)
		}
		result.Findings = append(result.Findings, findings...)

		return nil
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".ico": true, ".svg": true, ".webp": true, ".tiff": true, ".tif": true,
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true,
	".7z": true, ".rar": true, ".zst": true,
	".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true,
	".pyc": true, ".o": true, ".a": true, ".wasm": true,
	".mp3": true, ".mp4": true, ".wav": true, ".avi": true, ".mov": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
}

func isBinaryExt(ext string) bool {
	return binaryExts[ext]
}

var suspiciousExts = map[string]bool{
	".exe": true, ".bin": true, ".dll": true, ".so": true, ".dylib": true,
	".class": true, ".jar": true,
}

func isSuspiciousExt(ext string) bool {
	return suspiciousExts[ext]
}

func scanFile(path, rootPath string, rules []Rule) ([]Finding, error) {
	// Pre-check for symlinks (Lstat does not follow symlinks)
	linfo, err := os.Lstat(path)
	if err != nil {
		if os.IsPermission(err) || os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if linfo.Mode()&os.ModeSymlink != 0 || !linfo.Mode().IsRegular() {
		return nil, nil
	}

	// Open file and verify via fd to avoid TOCTOU race
	file, err := os.Open(path)
	if err != nil {
		if os.IsPermission(err) || os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() { _ = file.Close() }()

	fi, err := file.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > maxSkillFileSize {
		return nil, nil
	}

	var findings []Finding
	relPath, relErr := filepath.Rel(rootPath, path)
	if relErr != nil {
		relPath = filepath.Base(path)
	}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), int(maxSkillFileSize))
	lineNum := 0

	for scanner.Scan() {
		line := scanner.Text()
		lineNum++

		// Max line length check to avoid DOS/Memory issues on minified files
		if len(line) > 10000 {
			continue
		}

		for _, rule := range rules {
			matches := rule.Regex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				fullMatch := match[0]

				// If rule has entropy check, verify validity
				if rule.Entropy > 0 {
					// We check the capture group for entropy if it exists, otherwise the whole match
					checkStr := fullMatch
					if len(match) > 2 {
						// For generic secrets, group 2 is usually the secret value
						checkStr = match[2]
					} else if len(match) > 1 {
						checkStr = match[1]
					}

					entropy := CalculateEntropy(checkStr)
					if entropy < rule.Entropy {
						continue // Skip low entropy matches
					}
				}

				// Special handling for NET-HTTP rule to implement exclusions (since Go regex doesn't support lookarounds)
				if rule.ID == "NET-HTTP" {
					lowerMatch := strings.ToLower(fullMatch)
					// domains to exclude
					exclusions := []string{
						"apache.org", "creativecommons.org", "opensource.org", "github.com", "w3.org",
					}
					excluded := false
					for _, domain := range exclusions {
						if strings.Contains(lowerMatch, domain) {
							excluded = true
							break
						}
					}
					if excluded || strings.Contains(lowerMatch, "license") {
						continue
					}
				}

				// Special handling for NET-IP-ADDR to reduce false positives on
				// version numbers (e.g. "1.2.3.4"), loopback, and link-local addresses.
				if rule.ID == "NET-IP-ADDR" {
					lowerLine := strings.ToLower(line)
					// Skip lines that are clearly version declarations
					if strings.Contains(lowerLine, "version") ||
						strings.Contains(lowerLine, "\"version\"") ||
						strings.Contains(lowerLine, "'version'") {
						continue
					}
					// Skip well-known non-routable addresses
					if fullMatch == "127.0.0.1" || fullMatch == "0.0.0.0" ||
						strings.HasPrefix(fullMatch, "169.254.") ||
						fullMatch == "255.255.255.255" {
						continue
					}
				}

				findings = append(findings, Finding{
					RuleID:      rule.ID,
					Severity:    rule.Severity,
					Description: rule.Description,
					File:        relPath,
					Line:        lineNum,
					Match:       strings.TrimSpace(fullMatch),
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return findings, fmt.Errorf("scanning %s: %w", path, err)
	}
	return findings, nil
}
