package github

import (
	"testing"
)

func TestParseBrowserURL(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantRepoURL   string
		wantBranch    string
		wantSubDir    string
		wantSkillName string
		wantOK        bool
	}{
		{
			name:          "full URL with subdirectory",
			input:         "https://github.com/anthropics/skills/tree/main/skills/mcp-builder",
			wantRepoURL:   "https://github.com/anthropics/skills.git",
			wantBranch:    "main",
			wantSubDir:    "skills/mcp-builder",
			wantSkillName: "mcp-builder",
			wantOK:        true,
		},
		{
			name:          "URL with different branch",
			input:         "https://github.com/owner/repo/tree/develop/path/to/skill",
			wantRepoURL:   "https://github.com/owner/repo.git",
			wantBranch:    "develop",
			wantSubDir:    "path/to/skill",
			wantSkillName: "skill",
			wantOK:        true,
		},
		{
			name:          "URL without subdirectory - just branch",
			input:         "https://github.com/owner/repo/tree/main",
			wantRepoURL:   "https://github.com/owner/repo.git",
			wantBranch:    "main",
			wantSubDir:    "",
			wantSkillName: "repo",
			wantOK:        true,
		},
		{
			name:          "URL with trailing slash",
			input:         "https://github.com/anthropics/skills/tree/main/skills/mcp-builder/",
			wantRepoURL:   "https://github.com/anthropics/skills.git",
			wantBranch:    "main",
			wantSubDir:    "skills/mcp-builder",
			wantSkillName: "mcp-builder",
			wantOK:        true,
		},
		{
			name:          "http URL upgraded to https",
			input:         "http://github.com/owner/repo/tree/main/skills/test",
			wantRepoURL:   "https://github.com/owner/repo.git",
			wantBranch:    "main",
			wantSubDir:    "skills/test",
			wantSkillName: "test",
			wantOK:        true,
		},
		{
			name:          "URL with .git suffix already present",
			input:         "https://github.com/owner/repo.git/tree/main/skills/test",
			wantRepoURL:   "https://github.com/owner/repo.git",
			wantBranch:    "main",
			wantSubDir:    "skills/test",
			wantSkillName: "test",
			wantOK:        true,
		},
		{
			name:   "non-tree URL (regular git URL)",
			input:  "https://github.com/owner/repo.git",
			wantOK: false,
		},
		{
			name:   "shorthand format - not a browser URL",
			input:  "owner/repo/path/to/skill",
			wantOK: false,
		},
		{
			name:   "empty string",
			input:  "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRepoURL, gotBranch, gotSubDir, gotSkillName, gotOK := ParseBrowserURL(tt.input)

			if gotOK != tt.wantOK {
				t.Errorf("ParseBrowserURL() ok = %v, want %v", gotOK, tt.wantOK)
				return
			}

			if !tt.wantOK {
				return // No need to check other fields if we expected failure
			}

			if gotRepoURL != tt.wantRepoURL {
				t.Errorf("ParseBrowserURL() repoURL = %v, want %v", gotRepoURL, tt.wantRepoURL)
			}
			if gotBranch != tt.wantBranch {
				t.Errorf("ParseBrowserURL() branch = %v, want %v", gotBranch, tt.wantBranch)
			}
			if gotSubDir != tt.wantSubDir {
				t.Errorf("ParseBrowserURL() subDir = %v, want %v", gotSubDir, tt.wantSubDir)
			}
			if gotSkillName != tt.wantSkillName {
				t.Errorf("ParseBrowserURL() skillName = %v, want %v", gotSkillName, tt.wantSkillName)
			}
		})
	}
}

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "simple owner/repo",
			input:     "owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "https url",
			input:     "https://github.com/owner/repo",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "https url with .git",
			input:     "https://github.com/owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "git ssh url",
			input:     "git@github.com:owner/repo.git",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:    "non-github ssh host",
			input:   "git@gitlab.com:owner/repo.git",
			wantErr: true,
		},
		{
			name:      "trailing slash",
			input:     "https://github.com/owner/repo/",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "url with subpath",
			input:     "https://github.com/owner/repo/tree/main",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "url with deep link",
			input:     "https://github.com/owner/repo/blob/master/README.md",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "short format with subpath",
			input:     "owner/repo/path/to/skill",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:      "whitespace",
			input:     "  owner/repo  ",
			wantOwner: "owner",
			wantRepo:  "repo",
			wantErr:   false,
		},
		{
			name:    "invalid format",
			input:   "repoonly",
			wantErr: true,
		},
		{
			name:    "invalid https",
			input:   "https://google.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOwner, gotRepo, err := ParseRepoURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRepoURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if gotOwner != tt.wantOwner {
					t.Errorf("ParseRepoURL() owner = %v, want %v", gotOwner, tt.wantOwner)
				}
				if gotRepo != tt.wantRepo {
					t.Errorf("ParseRepoURL() repo = %v, want %v", gotRepo, tt.wantRepo)
				}
			}
		})
	}
}
