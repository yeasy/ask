// Package skill provides core skill manipulation and security checking logic.
package skill

import (
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// verifyWeights checks that score category weights sum to 1.0.
// Called from tests to catch configuration errors at test time.
func verifyWeights() error {
	sum := weightSecurity + weightQuality + weightPublisher + weightTransparency
	if math.Abs(sum-1.0) > 0.001 {
		return fmt.Errorf("score category weights must sum to 1.0, got %f", sum)
	}
	return nil
}

// ScoreGrade represents the trust grade of a skill
type ScoreGrade string

// Score grades from A (best) to F (worst)
const (
	GradeA ScoreGrade = "A" // 90-100: Excellent
	GradeB ScoreGrade = "B" // 80-89: Good
	GradeC ScoreGrade = "C" // 70-79: Acceptable
	GradeD ScoreGrade = "D" // 60-69: Poor
	GradeF ScoreGrade = "F" // 0-59: Fail
)

// Category weights (must sum to 1.0)
const (
	weightSecurity     = 0.40
	weightQuality      = 0.30
	weightPublisher    = 0.20
	weightTransparency = 0.10
)

// Grade score thresholds
const (
	thresholdA = 90
	thresholdB = 80
	thresholdC = 70
	thresholdD = 60
)

// Security scoring parameters
const (
	criticalDeductPer = 25.0  // Points deducted per critical finding
	criticalDeductMax = 100.0 // Maximum deduction for critical findings
	warningDeductPer  = 5.0   // Points deducted per warning
	warningDeductMax  = 40.0  // Maximum deduction for warnings
	infoDeductMax     = 10.0  // Maximum deduction for info findings
)

// Publisher scoring parameters
const (
	publisherBaseScore  = 50.0 // Starting score for publisher category
	orgBonus            = 15.0 // Bonus for organization accounts
	starBonusDivisor    = 50.0 // Stars divided by this for bonus
	starBonusMax        = 20.0 // Maximum star bonus
	accountAgeBonusMax  = 10.0 // Maximum account age bonus (1 point per year)
	licensePresentBonus = 5.0  // Bonus for having a license

	// maxSkillFileSize is the maximum file size to read during scoring (1MB)
	maxSkillFileSize = 1024 * 1024
)

// ScoreCategory represents a scored dimension
type ScoreCategory struct {
	Name    string      `json:"name"`
	Score   float64     `json:"score"`   // 0-100
	Weight  float64     `json:"weight"`  // 0-1.0
	Details string      `json:"details"` // Human-readable explanation
	Deducts []Deduction `json:"deductions,omitempty"`
}

// Deduction represents a single score deduction
type Deduction struct {
	Reason string  `json:"reason"`
	Points float64 `json:"points"`
}

// ScoreResult contains the full trust score for a skill
type ScoreResult struct {
	SkillName  string          `json:"skill_name"`
	TotalScore float64         `json:"total_score"` // 0-100 weighted
	Grade      ScoreGrade      `json:"grade"`
	Categories []ScoreCategory `json:"categories"`
	Summary    string          `json:"summary"`
	ScoredAt   time.Time       `json:"scored_at"`
}

// PublisherInfo holds metadata about the skill publisher
type PublisherInfo struct {
	Owner      string
	IsOrg      bool
	RepoStars  int
	AccountAge int // years
	HasLicense bool
	RepoForks  int
}

// ScoreSkill computes a comprehensive trust score for a skill directory
func ScoreSkill(skillPath string, publisher *PublisherInfo) (*ScoreResult, error) {
	result := &ScoreResult{
		SkillName: filepath.Base(skillPath),
		ScoredAt:  time.Now(),
	}

	// Category 1: Security Analysis (40% weight)
	secCat := scoreSecurityCategory(skillPath)
	result.Categories = append(result.Categories, secCat)

	// Category 2: Skill Quality (30% weight)
	qualCat := scoreQualityCategory(skillPath)
	result.Categories = append(result.Categories, qualCat)

	// Category 3: Publisher Trust (20% weight)
	pubCat := scorePublisherCategory(publisher)
	result.Categories = append(result.Categories, pubCat)

	// Category 4: Transparency (10% weight)
	transCat := scoreTransparencyCategory(skillPath)
	result.Categories = append(result.Categories, transCat)

	// Calculate weighted total
	var total float64
	for _, cat := range result.Categories {
		total += cat.Score * cat.Weight
	}
	result.TotalScore = math.Round(total*10) / 10
	result.Grade = gradeFromScore(result.TotalScore)
	result.Summary = generateSummary(result)

	return result, nil
}

// scoreSecurityCategory runs the security scanner and scores based on findings
func scoreSecurityCategory(skillPath string) ScoreCategory {
	cat := ScoreCategory{
		Name:   "Security",
		Weight: weightSecurity,
		Score:  100,
	}

	checkResult, err := CheckSafety(skillPath)
	if err != nil {
		cat.Score = 0
		cat.Details = fmt.Sprintf("Security scan failed: %v", err)
		return cat
	}

	critCount, warnCount, infoCount := 0, 0, 0
	for _, f := range checkResult.Findings {
		switch f.Severity {
		case SeverityCritical:
			critCount++
		case SeverityWarning:
			warnCount++
		case SeverityInfo:
			infoCount++
		}
	}

	// Critical findings: deduct per finding (capped)
	if critCount > 0 {
		deduct := math.Min(float64(critCount)*criticalDeductPer, criticalDeductMax)
		cat.Score -= deduct
		cat.Deducts = append(cat.Deducts, Deduction{
			Reason: fmt.Sprintf("%d critical finding(s): secrets, dangerous commands, or malware indicators", critCount),
			Points: deduct,
		})
	}

	// Warnings: deduct per finding (capped)
	if warnCount > 0 {
		deduct := math.Min(float64(warnCount)*warningDeductPer, warningDeductMax)
		cat.Score -= deduct
		cat.Deducts = append(cat.Deducts, Deduction{
			Reason: fmt.Sprintf("%d warning(s): suspicious patterns or network calls", warnCount),
			Points: deduct,
		})
	}

	// Info: -1 each (capped)
	if infoCount > 0 {
		deduct := math.Min(float64(infoCount), infoDeductMax)
		cat.Score -= deduct
		cat.Deducts = append(cat.Deducts, Deduction{
			Reason: fmt.Sprintf("%d informational finding(s)", infoCount),
			Points: deduct,
		})
	}

	cat.Score = math.Max(cat.Score, 0)

	if len(checkResult.Findings) == 0 {
		cat.Details = "No security issues detected"
	} else {
		cat.Details = fmt.Sprintf("Found %d issue(s): %d critical, %d warning, %d info",
			len(checkResult.Findings), critCount, warnCount, infoCount)
	}

	return cat
}

// scoreQualityCategory checks skill structure and metadata completeness
func scoreQualityCategory(skillPath string) ScoreCategory {
	cat := ScoreCategory{
		Name:   "Quality",
		Weight: weightQuality,
		Score:  100,
	}

	// Check SKILL.md exists and has frontmatter
	hasSkillMD := FindSkillMD(skillPath)
	if !hasSkillMD {
		cat.Score -= 40
		cat.Deducts = append(cat.Deducts, Deduction{
			Reason: "Missing SKILL.md",
			Points: 40,
		})
	} else {
		meta, err := ParseSkillMD(skillPath)
		if err != nil || meta == nil {
			cat.Score -= 20
			cat.Deducts = append(cat.Deducts, Deduction{
				Reason: "SKILL.md exists but has no valid frontmatter",
				Points: 20,
			})
		} else {
			if meta.Description == "" {
				cat.Score -= 10
				cat.Deducts = append(cat.Deducts, Deduction{
					Reason: "Missing description in SKILL.md frontmatter",
					Points: 10,
				})
			}
			if meta.Version == "" {
				cat.Score -= 5
				cat.Deducts = append(cat.Deducts, Deduction{
					Reason: "Missing version in SKILL.md frontmatter",
					Points: 5,
				})
			}
			if meta.Author == "" {
				cat.Score -= 5
				cat.Deducts = append(cat.Deducts, Deduction{
					Reason: "Missing author in SKILL.md frontmatter",
					Points: 5,
				})
			}
		}
	}

	// Check README
	hasReadme := fileExists(filepath.Join(skillPath, "README.md")) ||
		fileExists(filepath.Join(skillPath, "readme.md"))
	if !hasReadme {
		cat.Score -= 15
		cat.Deducts = append(cat.Deducts, Deduction{
			Reason: "Missing README.md",
			Points: 15,
		})
	}

	// Check prompts directory
	promptsDir := filepath.Join(skillPath, "prompts")
	if info, err := os.Stat(promptsDir); err != nil || !info.IsDir() {
		cat.Score -= 10
		cat.Deducts = append(cat.Deducts, Deduction{
			Reason: "Missing prompts/ directory",
			Points: 10,
		})
	} else {
		// Check if prompts dir has any .md files
		entries, readErr := os.ReadDir(promptsDir)
		if readErr != nil {
			entries = nil
		}
		hasMD := false
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
				hasMD = true
				break
			}
		}
		if !hasMD {
			cat.Score -= 5
			cat.Deducts = append(cat.Deducts, Deduction{
				Reason: "No .md files in prompts/ directory",
				Points: 5,
			})
		}
	}

	cat.Score = math.Max(cat.Score, 0)

	if len(cat.Deducts) == 0 {
		cat.Details = "Complete metadata, documentation, and prompt files"
	} else {
		cat.Details = fmt.Sprintf("%d quality issue(s) found", len(cat.Deducts))
	}

	return cat
}

// scorePublisherCategory evaluates the trustworthiness of the skill publisher
func scorePublisherCategory(publisher *PublisherInfo) ScoreCategory {
	cat := ScoreCategory{
		Name:   "Publisher",
		Weight: weightPublisher,
		Score:  publisherBaseScore, // Start at base, earn points for trust signals
	}

	if publisher == nil {
		cat.Details = "No publisher information available (local skill)"
		return cat
	}

	// Organization accounts are more trustworthy
	if publisher.IsOrg {
		cat.Score += orgBonus
		cat.Details = fmt.Sprintf("Organization: %s", publisher.Owner)
	}

	// Stars indicate community trust
	starBonus := math.Min(float64(publisher.RepoStars)/starBonusDivisor, starBonusMax)
	cat.Score += starBonus

	// Account age (1 point per year, capped)
	ageBonus := math.Min(float64(publisher.AccountAge), accountAgeBonusMax)
	cat.Score += ageBonus

	// License present (bonus for having one, no bonus without)
	if publisher.HasLicense {
		cat.Score += licensePresentBonus
	} else {
		cat.Deducts = append(cat.Deducts, Deduction{
			Reason: "No license detected in repository (missing bonus)",
			Points: 0, // Not a deduction; license is a bonus, not a baseline
		})
	}

	cat.Score = math.Min(cat.Score, 100)

	if cat.Details == "" {
		cat.Details = fmt.Sprintf("Publisher: %s (stars: %d, age: %d years)",
			publisher.Owner, publisher.RepoStars, publisher.AccountAge)
	}

	return cat
}

// scoreTransparencyCategory checks for data exfiltration and hidden behavior
func scoreTransparencyCategory(skillPath string) ScoreCategory {
	cat := ScoreCategory{
		Name:   "Transparency",
		Weight: weightTransparency,
		Score:  100,
	}

	// Scan for data exfiltration patterns
	exfilPatterns := []struct {
		name    string
		pattern string
		deduct  float64
	}{
		{"curl/wget with POST data", `(?i)(curl|wget).*(-d|--data|--post)`, 30},
		{"fetch with body payload", `(?i)fetch\s*\(.*body\s*:`, 25},
		{"base64 encode + send", `(?i)(btoa|base64.*encode).*?(fetch|curl|http)`, 35},
		{"hidden environment read", `(?i)(process\.env|os\.environ|ENV\[).*?(send|post|fetch|curl)`, 30},
		{"obfuscated code", `(?i)(eval|exec)\s*\(\s*(atob|Buffer\.from|decodeURI)`, 40},
		{"steganographic data hiding", `(?i)(canvas|image).*?(toDataURL|getImageData).*?(fetch|send)`, 25},
	}

	err := filepath.WalkDir(skillPath, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsPermission(walkErr) {
				return nil
			}
			return walkErr
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

		ext := strings.ToLower(filepath.Ext(path))
		if isBinaryExt(ext) {
			return nil
		}

		// Skip files larger than 1MB to prevent memory exhaustion
		info, infoErr := d.Info()
		if infoErr != nil || info.Size() > maxSkillFileSize {
			return nil
		}

		// Open file and check via fd to avoid TOCTOU race.
		// Use a helper closure so defer fires per file instead of accumulating.
		content, readErr := func() ([]byte, error) {
			f, fErr := os.Open(path)
			if fErr != nil {
				return nil, fErr
			}
			defer func() { _ = f.Close() }()

			fi, fErr := f.Stat()
			if fErr != nil || !fi.Mode().IsRegular() || fi.Size() > maxSkillFileSize {
				return nil, nil
			}
			return io.ReadAll(io.LimitReader(f, maxSkillFileSize))
		}()
		if readErr != nil || content == nil {
			return nil
		}

		text := string(content)
		relPath, relErr := filepath.Rel(skillPath, path)
		if relErr != nil {
			return nil
		}

		for _, p := range exfilPatterns {
			matched, _ := matchPattern(text, p.pattern)
			if matched {
				cat.Score -= p.deduct
				cat.Deducts = append(cat.Deducts, Deduction{
					Reason: fmt.Sprintf("%s detected in %s", p.name, relPath),
					Points: p.deduct,
				})
			}
		}
		return nil
	})

	if err != nil {
		cat.Score = 0
		cat.Details = fmt.Sprintf("Transparency scan failed: %v", err)
		return cat
	}

	cat.Score = math.Max(cat.Score, 0)

	if len(cat.Deducts) == 0 {
		cat.Details = "No data exfiltration or obfuscation patterns detected"
	} else {
		cat.Details = fmt.Sprintf("%d suspicious transparency pattern(s) found", len(cat.Deducts))
	}

	return cat
}

// matchPattern checks if text matches a regex pattern
func matchPattern(text, pattern string) (bool, error) {
	re, err := compilePattern(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(text), nil
}

// patternCache caches compiled regex patterns with synchronization
var (
	patternCache   = make(map[string]*regexp.Regexp)
	patternCacheMu sync.RWMutex
)

func compilePattern(pattern string) (*regexp.Regexp, error) {
	patternCacheMu.RLock()
	if cached, ok := patternCache[pattern]; ok {
		patternCacheMu.RUnlock()
		return cached, nil
	}
	patternCacheMu.RUnlock()

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	patternCacheMu.Lock()
	// Re-check after acquiring write lock to avoid redundant compilation
	if cached, ok := patternCache[pattern]; ok {
		patternCacheMu.Unlock()
		return cached, nil
	}
	patternCache[pattern] = re
	patternCacheMu.Unlock()
	return re, nil
}

// gradeRank returns numerical rank for comparison (lower is better)
func gradeRank(g ScoreGrade) int {
	switch g {
	case GradeA:
		return 1
	case GradeB:
		return 2
	case GradeC:
		return 3
	case GradeD:
		return 4
	default:
		return 5
	}
}

// GradeBelowThreshold returns true if grade is worse than the threshold
func GradeBelowThreshold(grade, threshold ScoreGrade) bool {
	return gradeRank(grade) > gradeRank(threshold)
}

func gradeFromScore(score float64) ScoreGrade {
	switch {
	case score >= thresholdA:
		return GradeA
	case score >= thresholdB:
		return GradeB
	case score >= thresholdC:
		return GradeC
	case score >= thresholdD:
		return GradeD
	default:
		return GradeF
	}
}

func generateSummary(result *ScoreResult) string {
	switch result.Grade {
	case GradeA:
		return "Excellent trust score. This skill passes all security checks with strong publisher credentials."
	case GradeB:
		return "Good trust score. Minor issues detected but generally safe to use."
	case GradeC:
		return "Acceptable trust score. Some concerns found — review details before installing."
	case GradeD:
		return "Poor trust score. Significant issues detected — use with caution."
	default:
		return "Failed trust assessment. Critical security or transparency issues found — not recommended."
	}
}

func fileExists(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	// Reject symlinks and non-regular files
	return fi.Mode()&os.ModeSymlink == 0 && fi.Mode().IsRegular()
}
