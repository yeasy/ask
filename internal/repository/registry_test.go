package repository

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yeasy/ask/internal/config"
)

// validRegistryIndex returns a sample RegistryIndex for testing
func validRegistryIndex() RegistryIndex {
	return RegistryIndex{
		Version: "1.0",
		Skills: []RegistrySkill{
			{
				Name:        "code-review",
				Source:      "alice",
				URL:         "https://github.com/alice/code-review",
				Description: "Automated code review skill",
				Category:    "development",
				Tags:        []string{"review", "lint"},
				Stars:       42,
				Featured:    true,
				InstallCmd:  "alice/code-review",
			},
			{
				Name:        "docker-helper",
				Source:      "bob",
				URL:         "https://github.com/bob/docker-helper",
				Description: "Docker container management",
				Category:    "devops",
				Tags:        []string{"docker", "containers"},
				Stars:       15,
				Featured:    false,
				InstallCmd:  "bob/docker-helper",
			},
			{
				Name:        "sql-tuner",
				Source:      "carol",
				URL:         "https://github.com/carol/sql-tuner",
				Description: "SQL query optimization",
				Category:    "database",
				Tags:        []string{"sql", "performance"},
				Stars:       28,
				Featured:    false,
				InstallCmd:  "carol/sql-tuner",
			},
		},
	}
}

func setupTestServer(handler http.HandlerFunc) (*httptest.Server, func()) {
	server := httptest.NewServer(handler)
	origBase := rawBaseURL
	rawBaseURL = server.URL
	cleanup := func() {
		server.Close()
		rawBaseURL = origBase
	}
	return server, cleanup
}

func TestFetchSkillsFromRegistry_ValidJSON(t *testing.T) {
	config.SetOffline(false)

	index := validRegistryIndex()
	data, err := json.Marshal(index)
	if err != nil {
		t.Fatalf("failed to marshal test data: %v", err)
	}

	_, cleanup := setupTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	defer cleanup()

	results, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify mapping from RegistrySkill to github.Repository
	r := results[0]
	if r.Name != "code-review" {
		t.Errorf("expected name 'code-review', got %q", r.Name)
	}
	if r.FullName != "alice/code-review" {
		t.Errorf("expected full name 'alice/code-review', got %q", r.FullName)
	}
	if r.Description != "Automated code review skill" {
		t.Errorf("expected description 'Automated code review skill', got %q", r.Description)
	}
	if r.HTMLURL != "https://github.com/alice/code-review" {
		t.Errorf("expected HTMLURL 'https://github.com/alice/code-review', got %q", r.HTMLURL)
	}
	if r.StargazersCount != 42 {
		t.Errorf("expected 42 stars, got %d", r.StargazersCount)
	}
}

func TestFetchSkillsFromRegistry_MasterBranchFallback(t *testing.T) {
	config.SetOffline(false)

	index := validRegistryIndex()
	data, _ := json.Marshal(index)

	_, cleanup := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		// Registry repo whose default branch is "master": "main" must 404,
		// and the fetch should fall back to "master".
		if contains(r.URL.Path, "/main/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	defer cleanup()

	results, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results via master fallback, got %d", len(results))
	}
}

func TestFetchSkillsFromRegistry_HTTPError404(t *testing.T) {
	config.SetOffline(false)

	_, cleanup := setupTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	defer cleanup()

	_, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "")
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if got := err.Error(); got != "registry returned status 404" {
		t.Errorf("expected 'registry returned status 404', got %q", got)
	}
}

func TestFetchSkillsFromRegistry_HTTPError500(t *testing.T) {
	config.SetOffline(false)

	_, cleanup := setupTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	defer cleanup()

	_, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "")
	if err == nil {
		t.Fatal("expected error for 500 response, got nil")
	}
	if got := err.Error(); got != "registry returned status 500" {
		t.Errorf("expected 'registry returned status 500', got %q", got)
	}
}

func TestFetchSkillsFromRegistry_MalformedJSON(t *testing.T) {
	config.SetOffline(false)

	_, cleanup := setupTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{not valid json`))
	})
	defer cleanup()

	_, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "")
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
	if got := err.Error(); !contains(got, "failed to parse registry index") {
		t.Errorf("expected error containing 'failed to parse registry index', got %q", got)
	}
}

func TestFetchSkillsFromRegistry_KeywordFilterByName(t *testing.T) {
	config.SetOffline(false)

	index := validRegistryIndex()
	data, _ := json.Marshal(index)

	_, cleanup := setupTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	defer cleanup()

	results, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "docker")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result for keyword 'docker', got %d", len(results))
	}
	if results[0].Name != "docker-helper" {
		t.Errorf("expected 'docker-helper', got %q", results[0].Name)
	}
}

func TestFetchSkillsFromRegistry_KeywordFilterByDescription(t *testing.T) {
	config.SetOffline(false)

	index := validRegistryIndex()
	data, _ := json.Marshal(index)

	_, cleanup := setupTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	defer cleanup()

	results, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "optimization")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result for keyword 'optimization', got %d", len(results))
	}
	if results[0].Name != "sql-tuner" {
		t.Errorf("expected 'sql-tuner', got %q", results[0].Name)
	}
}

func TestFetchSkillsFromRegistry_KeywordFilterByTag(t *testing.T) {
	config.SetOffline(false)

	index := validRegistryIndex()
	data, _ := json.Marshal(index)

	_, cleanup := setupTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	defer cleanup()

	results, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "lint")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result for keyword 'lint', got %d", len(results))
	}
	if results[0].Name != "code-review" {
		t.Errorf("expected 'code-review', got %q", results[0].Name)
	}
}

func TestFetchSkillsFromRegistry_KeywordCaseInsensitive(t *testing.T) {
	config.SetOffline(false)

	index := validRegistryIndex()
	data, _ := json.Marshal(index)

	_, cleanup := setupTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	defer cleanup()

	results, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "DOCKER")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result for keyword 'DOCKER', got %d", len(results))
	}
	if results[0].Name != "docker-helper" {
		t.Errorf("expected 'docker-helper', got %q", results[0].Name)
	}
}

func TestFetchSkillsFromRegistry_KeywordNoMatch(t *testing.T) {
	config.SetOffline(false)

	index := validRegistryIndex()
	data, _ := json.Marshal(index)

	_, cleanup := setupTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	defer cleanup()

	results, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for keyword 'nonexistent', got %d", len(results))
	}
}

func TestFetchSkillsFromRegistry_InvalidURLFormat(t *testing.T) {
	config.SetOffline(false)

	_, err := FetchSkillsFromRegistry("invalid-no-slashes", "")
	if err == nil {
		t.Fatal("expected error for invalid URL format, got nil")
	}
	if got := err.Error(); !contains(got, "invalid registry URL format") {
		t.Errorf("expected error containing 'invalid registry URL format', got %q", got)
	}
}

func TestFetchSkillsFromRegistry_OfflineMode(t *testing.T) {
	config.SetOffline(true)
	defer config.SetOffline(false)

	_, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "")
	if err == nil {
		t.Fatal("expected error in offline mode, got nil")
	}
	if got := err.Error(); !contains(got, "offline mode") {
		t.Errorf("expected error containing 'offline mode', got %q", got)
	}
}

func TestFetchSkillsFromRegistry_EmptySkillsList(t *testing.T) {
	config.SetOffline(false)

	index := RegistryIndex{Version: "1.0", Skills: []RegistrySkill{}}
	data, _ := json.Marshal(index)

	_, cleanup := setupTestServer(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	defer cleanup()

	results, err := FetchSkillsFromRegistry("owner/repo/registry/index.json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results for empty registry, got %d", len(results))
	}
}

func TestFetchSkillsFromRegistry_RequestPath(t *testing.T) {
	config.SetOffline(false)

	var requestedPath string
	_, cleanup := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		index := RegistryIndex{Version: "1.0", Skills: []RegistrySkill{}}
		data, _ := json.Marshal(index)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	defer cleanup()

	_, err := FetchSkillsFromRegistry("myowner/myrepo/path/to/index.json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "/myowner/myrepo/main/path/to/index.json"
	if requestedPath != expected {
		t.Errorf("expected request path %q, got %q", expected, requestedPath)
	}
}

func TestFetchSkillsFromRegistry_UserAgentHeader(t *testing.T) {
	config.SetOffline(false)

	var userAgent string
	_, cleanup := setupTestServer(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		index := RegistryIndex{Version: "1.0", Skills: []RegistrySkill{}}
		data, _ := json.Marshal(index)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(data)
	})
	defer cleanup()

	_, err := FetchSkillsFromRegistry("owner/repo/index.json", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if userAgent != "ask-cli" {
		t.Errorf("expected User-Agent 'ask-cli', got %q", userAgent)
	}
}

// contains is a simple helper to check substring presence
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
