package cmd

import (
	"strings"
	"testing"

	"github.com/yeasy/ask/internal/skill"
)

// ---------------------------------------------------------------------------
// filterBySeverity (check.go)
// ---------------------------------------------------------------------------

func TestFilterBySeverity(t *testing.T) {
	base := &skill.CheckResult{
		SkillName:      "test-skill",
		ScannedModules: []string{"mod-a"},
		Findings: []skill.Finding{
			{RuleID: "R1", Severity: skill.SeverityCritical, Description: "crit"},
			{RuleID: "R2", Severity: skill.SeverityWarning, Description: "warn"},
			{RuleID: "R3", Severity: skill.SeverityInfo, Description: "info"},
		},
	}

	tests := []struct {
		name        string
		minSeverity string
		wantCount   int
		wantIDs     []string
	}{
		{
			name:        "critical only",
			minSeverity: "critical",
			wantCount:   1,
			wantIDs:     []string{"R1"},
		},
		{
			name:        "warning includes critical and warning",
			minSeverity: "warning",
			wantCount:   2,
			wantIDs:     []string{"R1", "R2"},
		},
		{
			name:        "info includes all",
			minSeverity: "info",
			wantCount:   3,
			wantIDs:     []string{"R1", "R2", "R3"},
		},
		{
			name:        "unknown severity defaults to info (all)",
			minSeverity: "unknown",
			wantCount:   3,
			wantIDs:     []string{"R1", "R2", "R3"},
		},
		{
			name:        "empty severity defaults to info (all)",
			minSeverity: "",
			wantCount:   3,
			wantIDs:     []string{"R1", "R2", "R3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterBySeverity(base, tt.minSeverity)

			if result.SkillName != base.SkillName {
				t.Errorf("SkillName = %q, want %q", result.SkillName, base.SkillName)
			}
			if len(result.Findings) != tt.wantCount {
				t.Fatalf("got %d findings, want %d", len(result.Findings), tt.wantCount)
			}
			for i, id := range tt.wantIDs {
				if result.Findings[i].RuleID != id {
					t.Errorf("Findings[%d].RuleID = %q, want %q", i, result.Findings[i].RuleID, id)
				}
			}
		})
	}
}

func TestFilterBySeverity_EmptyFindings(t *testing.T) {
	base := &skill.CheckResult{
		SkillName: "empty-skill",
		Findings:  []skill.Finding{},
	}
	for _, sev := range []string{"critical", "warning", "info"} {
		result := filterBySeverity(base, sev)
		if len(result.Findings) != 0 {
			t.Errorf("severity=%q: expected 0 findings, got %d", sev, len(result.Findings))
		}
	}
}

// ---------------------------------------------------------------------------
// hasCriticalIssues (check.go)
// ---------------------------------------------------------------------------

func TestHasCriticalIssues(t *testing.T) {
	tests := []struct {
		name     string
		findings []skill.Finding
		want     bool
	}{
		{
			name:     "no findings",
			findings: nil,
			want:     false,
		},
		{
			name: "only info",
			findings: []skill.Finding{
				{Severity: skill.SeverityInfo},
			},
			want: false,
		},
		{
			name: "only warnings",
			findings: []skill.Finding{
				{Severity: skill.SeverityWarning},
				{Severity: skill.SeverityWarning},
			},
			want: false,
		},
		{
			name: "one critical among others",
			findings: []skill.Finding{
				{Severity: skill.SeverityInfo},
				{Severity: skill.SeverityCritical},
				{Severity: skill.SeverityWarning},
			},
			want: true,
		},
		{
			name: "all critical",
			findings: []skill.Finding{
				{Severity: skill.SeverityCritical},
				{Severity: skill.SeverityCritical},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &skill.CheckResult{Findings: tt.findings}
			got := hasCriticalIssues(result)
			if got != tt.want {
				t.Errorf("hasCriticalIssues() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildRepoURL (sync.go)
// ---------------------------------------------------------------------------

func TestBuildRepoURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "owner/repo shorthand",
			url:  "anthropics/skills",
			want: "https://github.com/anthropics/skills.git",
		},
		{
			name: "owner/repo/path shorthand",
			url:  "anthropics/skills/skills",
			want: "https://github.com/anthropics/skills.git",
		},
		{
			name: "https URL passthrough",
			url:  "https://github.com/foo/bar.git",
			want: "https://github.com/foo/bar.git",
		},
		{
			name: "http URL passthrough",
			url:  "http://github.com/foo/bar.git",
			want: "http://github.com/foo/bar.git",
		},
		{
			name: "git@ URL passthrough",
			url:  "git@github.com:foo/bar.git",
			want: "git@github.com:foo/bar.git",
		},
		{
			name: "single segment",
			url:  "somerepo",
			want: "https://github.com/somerepo.git",
		},
		{
			name: "path traversal owner rejected",
			url:  "../evil",
			want: "",
		},
		{
			name: "path traversal repo rejected",
			url:  "owner/..",
			want: "",
		},
		{
			name: "dot owner rejected",
			url:  "./repo",
			want: "",
		},
		{
			name: "dot repo rejected",
			url:  "owner/.",
			want: "",
		},
		{
			name: "empty owner rejected",
			url:  "/repo",
			want: "",
		},
		{
			name: "empty string rejected",
			url:  "",
			want: "",
		},
		{
			name: "dot-dot single segment rejected",
			url:  "..",
			want: "",
		},
		{
			name: "dot single segment rejected",
			url:  ".",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRepoURL(tt.url)
			if got != tt.want {
				t.Errorf("buildRepoURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildRepoName (sync.go)
// ---------------------------------------------------------------------------

func TestBuildRepoName(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "owner/repo",
			url:  "anthropics/skills",
			want: "anthropics-skills",
		},
		{
			name: "owner/repo/path",
			url:  "anthropics/skills/skills",
			want: "anthropics-skills",
		},
		{
			name: "single segment",
			url:  "myrepo",
			want: "myrepo",
		},
		{
			name: "path traversal stripped",
			url:  "../evil",
			want: "unknown-repo",
		},
		{
			name: "owner with traversal stripped leaving empty",
			url:  "../..",
			want: "unknown-repo",
		},
		{
			name: "empty string",
			url:  "",
			want: "unknown-repo",
		},
		{
			name: "slashes become dashes for single segment with slash",
			url:  "a",
			want: "a",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRepoName(tt.url)
			if got != tt.want {
				t.Errorf("buildRepoName(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// isValidSemver (publish.go)
// ---------------------------------------------------------------------------

func TestIsValidSemver(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{name: "simple version", version: "1.0.0", want: true},
		{name: "zero version", version: "0.0.0", want: true},
		{name: "large numbers", version: "123.456.789", want: true},
		{name: "pre-release alpha", version: "1.0.0-alpha", want: true},
		{name: "pre-release beta.1", version: "1.0.0-beta.1", want: true},
		{name: "pre-release rc.1", version: "2.1.0-rc.1", want: true},
		{name: "missing patch", version: "1.0", want: false},
		{name: "missing minor and patch", version: "1", want: false},
		{name: "v prefix", version: "v1.0.0", want: false},
		{name: "empty string", version: "", want: false},
		{name: "letters", version: "abc", want: false},
		{name: "extra dot segment", version: "1.0.0.0", want: false},
		{name: "trailing dash", version: "1.0.0-", want: false},
		{name: "spaces", version: "1 .0.0", want: false},
		{name: "leading space", version: " 1.0.0", want: false},
		{name: "trailing space", version: "1.0.0 ", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidSemver(tt.version)
			if got != tt.want {
				t.Errorf("isValidSemver(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseOwnerFromRemote (publish.go)
// ---------------------------------------------------------------------------

func TestParseOwnerFromRemote(t *testing.T) {
	tests := []struct {
		name   string
		remote string
		want   string
	}{
		{
			name:   "ssh remote",
			remote: "git@github.com:yeasy/ask.git",
			want:   "yeasy",
		},
		{
			name:   "https remote",
			remote: "https://github.com/yeasy/ask.git",
			want:   "yeasy",
		},
		{
			name:   "https remote without .git",
			remote: "https://github.com/yeasy/ask",
			want:   "yeasy",
		},
		{
			name:   "http remote",
			remote: "http://github.com/yeasy/ask.git",
			want:   "yeasy",
		},
		{
			name:   "ssh remote with org",
			remote: "git@github.com:anthropics/skills.git",
			want:   "anthropics",
		},
		{
			name:   "plain owner/repo no prefix",
			remote: "owner/repo",
			want:   "owner",
		},
		{
			name:   "empty string",
			remote: "",
			want:   "",
		},
		{
			name:   "just a name",
			remote: "solo",
			want:   "solo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOwnerFromRemote(tt.remote)
			if got != tt.want {
				t.Errorf("parseOwnerFromRemote(%q) = %q, want %q", tt.remote, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateSkillMeta (publish.go)
// ---------------------------------------------------------------------------

func TestValidateSkillMeta(t *testing.T) {
	tests := []struct {
		name      string
		meta      *skill.Meta
		wantCount int
		wantErrs  []string
	}{
		{
			name: "valid meta",
			meta: &skill.Meta{
				Name:        "my-skill",
				Description: "A cool skill",
			},
			wantCount: 0,
		},
		{
			name:      "nil meta",
			meta:      nil,
			wantCount: 1,
			wantErrs:  []string{"failed to parse SKILL.md metadata"},
		},
		{
			name: "missing name",
			meta: &skill.Meta{
				Description: "A skill without a name",
			},
			wantCount: 1,
			wantErrs:  []string{"name is required"},
		},
		{
			name: "missing description",
			meta: &skill.Meta{
				Name: "nameless-desc",
			},
			wantCount: 1,
			wantErrs:  []string{"description is required"},
		},
		{
			name:      "missing both name and description",
			meta:      &skill.Meta{},
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateSkillMeta(tt.meta)
			if len(errs) != tt.wantCount {
				t.Fatalf("got %d errors, want %d: %v", len(errs), tt.wantCount, errs)
			}
			for _, substr := range tt.wantErrs {
				found := false
				for _, e := range errs {
					if contains(e, substr) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got %v", substr, errs)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// containsPath (prompt.go)
// ---------------------------------------------------------------------------

func TestContainsPath_TableDriven(t *testing.T) {
	tests := []struct {
		name   string
		paths  []string
		target string
		want   bool
	}{
		{
			name:   "found at start",
			paths:  []string{"/a/b", "/c/d"},
			target: "/a/b",
			want:   true,
		},
		{
			name:   "found at end",
			paths:  []string{"/a/b", "/c/d"},
			target: "/c/d",
			want:   true,
		},
		{
			name:   "not found",
			paths:  []string{"/a/b", "/c/d"},
			target: "/x/y",
			want:   false,
		},
		{
			name:   "empty list",
			paths:  []string{},
			target: "/a",
			want:   false,
		},
		{
			name:   "nil list",
			paths:  nil,
			target: "/a",
			want:   false,
		},
		{
			name:   "empty target in populated list",
			paths:  []string{"/a", "/b"},
			target: "",
			want:   false,
		},
		{
			name:   "empty string matches empty entry",
			paths:  []string{""},
			target: "",
			want:   true,
		},
		{
			name:   "case sensitive",
			paths:  []string{"/A/B"},
			target: "/a/b",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsPath(tt.paths, tt.target)
			if got != tt.want {
				t.Errorf("containsPath(%v, %q) = %v, want %v", tt.paths, tt.target, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// generateRegistryEntry (publish.go)
// ---------------------------------------------------------------------------

func TestGenerateRegistryEntry(t *testing.T) {
	tests := []struct {
		name      string
		meta      *skill.Meta
		skillPath string
		gitRemote string
		wantName  string
		wantCat   string
		wantURL   string
	}{
		{
			name: "basic with no remote",
			meta: &skill.Meta{
				Name:        "my-skill",
				Description: "Does things",
			},
			skillPath: "/home/user/skills/my-skill",
			gitRemote: "",
			wantName:  "my-skill",
			wantCat:   "general",
			wantURL:   "",
		},
		{
			name: "with https remote",
			meta: &skill.Meta{
				Name:        "my-skill",
				Description: "Does things",
			},
			skillPath: "/home/user/skills/my-skill",
			gitRemote: "https://github.com/owner/repo.git",
			wantName:  "my-skill",
			wantCat:   "general",
			wantURL:   "https://github.com/owner/repo",
		},
		{
			name: "with ssh remote",
			meta: &skill.Meta{
				Name:        "my-skill",
				Description: "Does things",
			},
			skillPath: "/home/user/skills/my-skill",
			gitRemote: "git@github.com:owner/repo.git",
			wantName:  "my-skill",
			wantCat:   "general",
			wantURL:   "https://github.com/owner/repo",
		},
		{
			name: "category from tags - security",
			meta: &skill.Meta{
				Name:        "sec-skill",
				Description: "Security stuff",
				Tags:        []string{"agent-skill", "security"},
			},
			skillPath: "/tmp/sec-skill",
			gitRemote: "",
			wantName:  "sec-skill",
			wantCat:   "security",
		},
		{
			name: "category from tags - development",
			meta: &skill.Meta{
				Name:        "dev-skill",
				Description: "Dev stuff",
				Tags:        []string{"development", "productivity"},
			},
			skillPath: "/tmp/dev-skill",
			gitRemote: "",
			wantName:  "dev-skill",
			wantCat:   "productivity", // last matching category wins
		},
		{
			name: "nil tags get default",
			meta: &skill.Meta{
				Name:        "no-tags",
				Description: "No tags",
				Tags:        nil,
			},
			skillPath: "/tmp/no-tags",
			gitRemote: "",
			wantName:  "no-tags",
			wantCat:   "general",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := generateRegistryEntry(tt.meta, tt.skillPath, tt.gitRemote)

			if entry.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", entry.Name, tt.wantName)
			}
			if entry.Category != tt.wantCat {
				t.Errorf("Category = %q, want %q", entry.Category, tt.wantCat)
			}
			if tt.wantURL != "" && entry.URL != tt.wantURL {
				t.Errorf("URL = %q, want %q", entry.URL, tt.wantURL)
			}
			if entry.Featured != false {
				t.Error("Featured should be false")
			}
			if entry.Stars != 0 {
				t.Error("Stars should be 0")
			}
			if len(entry.Tags) == 0 {
				t.Error("Tags should not be empty")
			}
		})
	}
}

func TestGenerateRegistryEntry_DefaultTags(t *testing.T) {
	meta := &skill.Meta{
		Name:        "tagless",
		Description: "No tags set",
		Tags:        nil,
	}
	entry := generateRegistryEntry(meta, "/tmp/tagless", "")
	if len(entry.Tags) != 1 || entry.Tags[0] != "agent-skill" {
		t.Errorf("expected default tags [agent-skill], got %v", entry.Tags)
	}
}

func TestGenerateRegistryEntry_InstallCmd(t *testing.T) {
	meta := &skill.Meta{
		Name:        "test-skill",
		Description: "A test",
	}

	// Without remote: install by name
	entry := generateRegistryEntry(meta, "/tmp/test-skill", "")
	if entry.InstallCmd != "ask install test-skill" {
		t.Errorf("InstallCmd = %q, want %q", entry.InstallCmd, "ask install test-skill")
	}

	// With remote: install by repo path
	entry = generateRegistryEntry(meta, "/tmp/test-skill", "https://github.com/owner/repo.git")
	if entry.InstallCmd != "ask install owner/repo" {
		t.Errorf("InstallCmd = %q, want %q", entry.InstallCmd, "ask install owner/repo")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
