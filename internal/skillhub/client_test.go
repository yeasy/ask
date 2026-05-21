package skillhub

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	c := NewClient()
	if c == nil {
		t.Fatal("NewClient returned nil")
	} else if c.HTTPClient == nil {
		t.Fatal("HTTPClient is nil")
	}
}

func TestSearch_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/search/quick") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		q := r.URL.Query().Get("q")
		if q == "" {
			t.Error("expected non-empty query parameter")
		}
		resp := searchResponse{
			Skills: []Skill{
				{ID: "1", Name: "browser-use", Slug: "browser-use", Description: "Browser automation"},
				{ID: "2", Name: "web-search", Slug: "web-search", Description: "Web search"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}

	skills, err := c.Search("browser")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	if skills[0].Name != "browser-use" {
		t.Errorf("expected first skill name 'browser-use', got %q", skills[0].Name)
	}
}

func TestSearch_EmptyQuery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q != "agent" {
			t.Errorf("expected default query 'agent' for empty input, got %q", q)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(searchResponse{Skills: []Skill{}})
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	_, err := c.Search("")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
}

func TestSearch_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	_, err := c.Search("test")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected error to mention status 500, got: %v", err)
	}
}

func TestResolve_Success(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		wantURL string
		wantErr bool
	}{
		{
			name:    "href pattern",
			body:    `<a href="https://github.com/owner/repo">GitHub</a>`,
			wantURL: "https://github.com/owner/repo",
		},
		{
			name:    "repoUrl JSON pattern",
			body:    `{"repoUrl":"https://github.com/owner/repo2"}`,
			wantURL: "https://github.com/owner/repo2",
		},
		{
			name:    "escaped repoUrl pattern",
			body:    `\"repoUrl\":\"https://github.com/owner/repo3\"`,
			wantURL: "https://github.com/owner/repo3",
		},
		{
			name:    "href with fragment",
			body:    `<a href="https://github.com/owner/repo#readme">GitHub</a>`,
			wantURL: "https://github.com/owner/repo",
		},
		{
			name:    "no GitHub URL",
			body:    `<html><body>No links here</body></html>`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if !strings.HasPrefix(r.URL.Path, "/skills/") {
					t.Errorf("unexpected path: %s", r.URL.Path)
				}
				w.Header().Set("Content-Type", "text/html")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
			gotURL, err := c.Resolve("test-skill")

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got URL %q", gotURL)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve failed: %v", err)
			}
			if gotURL != tt.wantURL {
				t.Errorf("got URL %q, want %q", gotURL, tt.wantURL)
			}
		})
	}
}

func TestValidateResolvedURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "valid GitHub URL", url: "https://github.com/owner/repo", wantErr: false},
		{name: "valid GitHub URL with path", url: "https://github.com/owner/repo/tree/main/path", wantErr: false},
		{name: "HTTP not HTTPS", url: "http://github.com/owner/repo", wantErr: true},
		{name: "not github.com", url: "https://evil.com/owner/repo", wantErr: true},
		{name: "no repo path", url: "https://github.com/owner", wantErr: true},
		{name: "empty path", url: "https://github.com/", wantErr: true},
		{name: "path traversal", url: "https://github.com/owner/../secret", wantErr: true},
		{name: "invalid URL", url: "://not-a-url", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateResolvedURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateResolvedURL(%q) error = %v, wantErr %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestResolve_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := &Client{HTTPClient: srv.Client(), BaseURL: srv.URL}
	_, err := c.Resolve("nonexistent")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}
