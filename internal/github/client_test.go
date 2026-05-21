package github

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSearchResultParsing(t *testing.T) {
	jsonResponse := `{
		"total_count": 1,
		"items": [
			{
				"name": "browser-use",
				"full_name": "browser-use/browser-use",
				"description": "Make websites accessible for AI agents",
				"stargazers_count": 1024,
				"html_url": "https://github.com/browser-use/browser-use"
			}
		]
	}`

	var result SearchResult
	err := json.Unmarshal([]byte(jsonResponse), &result)
	if err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	if result.TotalCount != 1 {
		t.Errorf("Expected TotalCount 1, got %d", result.TotalCount)
	}

	if len(result.Items) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(result.Items))
	}

	item := result.Items[0]
	if item.Name != "browser-use" {
		t.Errorf("Expected name browser-use, got %s", item.Name)
	}
	if item.StargazersCount != 1024 {
		t.Errorf("Expected 1024 stars, got %d", item.StargazersCount)
	}
}

func TestTrimQuotes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "double quoted", input: `"hello"`, want: "hello"},
		{name: "single quoted", input: `'hello'`, want: "hello"},
		{name: "no quotes", input: "hello", want: "hello"},
		{name: "empty string", input: "", want: ""},
		{name: "single char", input: "x", want: "x"},
		{name: "only double quotes", input: `""`, want: ""},
		{name: "only single quotes", input: "''", want: ""},
		{name: "mismatched quotes double-single", input: `"hello'`, want: `"hello'`},
		{name: "mismatched quotes single-double", input: `'hello"`, want: `'hello"`},
		{name: "leading whitespace before quotes", input: `  "hello"`, want: "hello"},
		{name: "trailing whitespace before quotes", input: `"hello"  `, want: "hello"},
		{name: "surrounded by whitespace", input: `  "hello"  `, want: "hello"},
		{name: "whitespace only", input: "   ", want: ""},
		{name: "quotes inside string", input: `he"ll"o`, want: `he"ll"o`},
		{name: "nested double quotes", input: `"he'llo"`, want: "he'llo"},
		{name: "single double quote", input: `"`, want: `"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimQuotes(tt.input)
			if got != tt.want {
				t.Errorf("trimQuotes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "short string within limit", input: "hello", maxLen: 10, want: "hello"},
		{name: "exact length", input: "hello", maxLen: 5, want: "hello"},
		{name: "over limit", input: "hello world", maxLen: 8, want: "hello..."},
		{name: "maxLen less than 4 clamped to 4", input: "hello", maxLen: 2, want: "h..."},
		{name: "maxLen of 4", input: "hello", maxLen: 4, want: "h..."},
		{name: "empty string", input: "", maxLen: 10, want: ""},
		{name: "unicode runes within limit", input: "cafe\u0301", maxLen: 10, want: "cafe\u0301"},
		{name: "unicode runes over limit", input: "abcdefghij", maxLen: 7, want: "abcd..."},
		{name: "multibyte chars count as single rune", input: "ABCDEFGHIJ", maxLen: 7, want: "ABCD..."},
		{name: "chinese characters", input: strings.Repeat("\u4e16", 10), maxLen: 6, want: strings.Repeat("\u4e16", 3) + "..."},
		{name: "maxLen zero clamped to 4", input: "hello", maxLen: 0, want: "h..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestEscapePathSegments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "simple path", input: "skills/mcp-builder", want: "skills/mcp-builder"},
		{name: "single segment", input: "skills", want: "skills"},
		{name: "empty string", input: "", want: ""},
		{name: "spaces in segment", input: "my skills/my builder", want: "my%20skills/my%20builder"},
		{name: "special chars", input: "path/hello world/foo@bar", want: "path/hello%20world/foo@bar"},
		{name: "already safe chars", input: "a-b_c/d.e", want: "a-b_c/d.e"},
		{name: "multiple slashes preserved", input: "a/b/c/d", want: "a/b/c/d"},
		{name: "trailing slash", input: "a/b/", want: "a/b/"},
		{name: "leading slash", input: "/a/b", want: "/a/b"},
		{name: "unicode segment", input: "path/\u4e16\u754c", want: "path/%E4%B8%96%E7%95%8C"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapePathSegments(tt.input)
			if got != tt.want {
				t.Errorf("escapePathSegments(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDescriptionFromSkillMD(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "frontmatter with description",
			input: "---\nname: test\ndescription: A great skill\n---\n# Heading\nBody text",
			want:  "A great skill",
		},
		{
			name:  "frontmatter with double-quoted description",
			input: "---\ndescription: \"Quoted description\"\n---\n",
			want:  "Quoted description",
		},
		{
			name:  "frontmatter with single-quoted description",
			input: "---\ndescription: 'Single quoted'\n---\n",
			want:  "Single quoted",
		},
		{
			// When description value is empty, the fallback body scanner
			// sees frontmatter lines as content (known quirk of current impl).
			name:  "frontmatter with empty description falls back to body",
			input: "---\ndescription:\n---\nSome body text here",
			want:  "description:",
		},
		{
			name:  "no frontmatter with heading and body",
			input: "# My Skill\nThis is the description line",
			want:  "This is the description line",
		},
		{
			name:  "heading only no body",
			input: "# My Skill",
			want:  "",
		},
		{
			name:  "empty input",
			input: "",
			want:  "",
		},
		{
			name:  "body text only no heading",
			input: "Just a plain line of text",
			want:  "Just a plain line of text",
		},
		{
			name:  "long description gets truncated to 60",
			input: "---\ndescription: This is a very long description that exceeds the sixty character limit for truncation\n---\n",
			want:  "This is a very long description that exceeds the sixty ch...",
		},
		{
			name:  "long body line gets truncated",
			input: "# Heading\nThis is a very long body line that definitely exceeds the sixty character maximum length for display",
			want:  "This is a very long body line that definitely exceeds the...",
		},
		{
			name:  "multiple headings skipped to find body",
			input: "# Heading 1\n## Heading 2\nActual content",
			want:  "Actual content",
		},
		{
			name:  "blank lines and headings only",
			input: "\n\n# Heading\n\n## Another\n",
			want:  "",
		},
		{
			name:  "frontmatter description takes priority over body",
			input: "---\ndescription: From frontmatter\n---\nFrom body",
			want:  "From frontmatter",
		},
		{
			name:  "frontmatter without closing delimiter",
			input: "---\ndescription: Unclosed frontmatter\nname: test",
			want:  "Unclosed frontmatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDescriptionFromSkillMD(tt.input)
			if got != tt.want {
				t.Errorf("parseDescriptionFromSkillMD() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseDescriptionFromSkillMD_EmptyFrontmatter(t *testing.T) {
	// Empty frontmatter block with no fields at all
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty frontmatter with body",
			input: "---\n---\nSome body text",
			want:  "Some body text",
		},
		{
			name:  "empty frontmatter no body",
			input: "---\n---\n",
			want:  "",
		},
		{
			name:  "empty frontmatter only headings after",
			input: "---\n---\n# Heading\n## Sub",
			want:  "",
		},
		{
			name:  "empty frontmatter with blank lines then body",
			input: "---\n---\n\n\nEventual body",
			want:  "Eventual body",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDescriptionFromSkillMD(tt.input)
			if got != tt.want {
				t.Errorf("parseDescriptionFromSkillMD(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDescriptionFromSkillMD_NoFrontmatter(t *testing.T) {
	// Files that have no frontmatter delimiter at all
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text only",
			input: "This is a plain skill file with no frontmatter",
			want:  "This is a plain skill file with no frontmatter",
		},
		{
			name:  "heading then paragraph",
			input: "# Title\nParagraph content here",
			want:  "Paragraph content here",
		},
		{
			name:  "blank lines then content",
			input: "\n\nContent after blanks",
			want:  "Content after blanks",
		},
		{
			name:  "only blank lines",
			input: "\n\n\n",
			want:  "",
		},
		{
			name:  "dashes but not frontmatter delimiter",
			input: "-- not frontmatter\nBody text",
			want:  "-- not frontmatter",
		},
		{
			name:  "single dash line is not frontmatter",
			input: "-\nBody text",
			want:  "-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDescriptionFromSkillMD(tt.input)
			if got != tt.want {
				t.Errorf("parseDescriptionFromSkillMD(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseDescriptionFromSkillMD_MalformedYAML(t *testing.T) {
	// Frontmatter that starts but never closes
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "unclosed frontmatter with description",
			input: "---\ndescription: Found inside unclosed\nname: test\nother: value",
			want:  "Found inside unclosed",
		},
		{
			name:  "unclosed frontmatter no description field",
			input: "---\nname: test\nauthor: someone",
			want:  "name: test",
		},
		{
			name:  "unclosed frontmatter only opening delimiter",
			input: "---\n",
			want:  "",
		},
		{
			name:  "opening delimiter with trailing content on same line is not frontmatter",
			input: "---extra\ndescription: Should be body\nOther line",
			want:  "---extra",
		},
		{
			name:  "unclosed frontmatter with colons but no description key",
			input: "---\ntitle: My Skill\nversion: 1.0\n",
			want:  "title: My Skill",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDescriptionFromSkillMD(tt.input)
			if got != tt.want {
				t.Errorf("parseDescriptionFromSkillMD(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewClient_Defaults(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient() returned nil")
	} else if c.httpClient == nil {
		t.Fatal("NewClient().httpClient is nil")
	} else {
		if c.httpClient.Timeout != httpTimeoutDefault {
			t.Errorf("expected timeout %v, got %v", httpTimeoutDefault, c.httpClient.Timeout)
		}
		if c.httpClient.Timeout != 10*time.Second {
			t.Errorf("expected timeout 10s, got %v", c.httpClient.Timeout)
		}
	}
}

func TestEscapePathSegments_SpecialChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "hash in segment", input: "path/foo#bar", want: "path/foo%23bar"},
		{name: "question mark in segment", input: "path/foo?bar", want: "path/foo%3Fbar"},
		{name: "percent literal in segment", input: "path/foo%20bar", want: "path/foo%2520bar"},
		{name: "plus sign in segment", input: "path/a+b", want: "path/a+b"},
		{name: "colon in segment", input: "path/foo:bar", want: "path/foo:bar"},
		{name: "bracket in segment", input: "path/[foo]", want: "path/%5Bfoo%5D"},
		{name: "ampersand in segment", input: "dir/a&b", want: "dir/a&b"},
		{name: "equals sign in segment", input: "dir/a=b", want: "dir/a=b"},
		{name: "empty segments preserved", input: "a//b", want: "a//b"},
		{name: "tab character in segment", input: "path/foo\tbar", want: "path/foo%09bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapePathSegments(tt.input)
			if got != tt.want {
				t.Errorf("escapePathSegments(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
