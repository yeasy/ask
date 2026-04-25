package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSkillName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid simple name", "my-skill", false},
		{"valid with underscore", "my_skill", false},
		{"valid with dot", "skill.v1", false},
		{"valid with slash", "owner/repo", false},
		{"valid with at", "skill@v1.0.0", false},
		{"empty name", "", true},
		{"directory traversal", "../secret", true},
		{"shell injection semicolon", "skill;rm -rf", true},
		{"shell injection ampersand", "skill && echo", true},
		{"shell injection pipe", "skill | cat", true},
		{"shell injection dollar", "skill$HOME", true},
		{"shell injection backtick", "skill`whoami`", true},
		{"shell injection greater than", "skill > file", true},
		{"shell injection less than", "skill < file", true},
		{"contains space", "my skill", true},
		{"newline injection", "skill\necho", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSkillName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSkillName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestCorsMiddleware(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name           string
		origin         string
		method         string
		wantAllowed    bool
		wantStatusCode int
	}{
		{"localhost origin", "http://localhost:8080", "GET", true, http.StatusOK},
		{"127.0.0.1 origin", "http://127.0.0.1:8080", "GET", true, http.StatusOK},
		{"app://wails.localhost origin", "app://wails.localhost", "GET", true, http.StatusOK},
		{"app://ask origin", "app://ask", "GET", true, http.StatusOK},
		{"unknown app:// origin rejected", "app://myapp", "GET", false, http.StatusOK},
		{"external origin", "https://evil.com", "GET", false, http.StatusOK},
		{"no origin", "", "GET", true, http.StatusOK},
		{"options preflight localhost", "http://localhost:8080", "OPTIONS", true, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/test", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatusCode)
			}

			corsHeader := rr.Header().Get("Access-Control-Allow-Origin")
			if tt.wantAllowed && corsHeader != tt.origin && tt.origin != "" {
				t.Errorf("CORS header = %q, want %q", corsHeader, tt.origin)
			}
			if !tt.wantAllowed && corsHeader != "" {
				t.Errorf("CORS header should be empty for external origin, got %q", corsHeader)
			}
		})
	}
}

func TestJsonResponse(t *testing.T) {
	t.Run("map of strings", func(t *testing.T) {
		rr := httptest.NewRecorder()
		data := map[string]string{"status": "ok", "message": "test"}
		jsonResponse(rr, data)

		if rr.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
		}

		var result map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if result["status"] != "ok" {
			t.Errorf("status = %q, want ok", result["status"])
		}
		if result["message"] != "test" {
			t.Errorf("message = %q, want test", result["message"])
		}
	})

	t.Run("slice of strings", func(t *testing.T) {
		rr := httptest.NewRecorder()
		data := []string{"a", "b", "c"}
		jsonResponse(rr, data)

		if rr.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
		}

		var result []string
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if len(result) != 3 || result[0] != "a" || result[1] != "b" || result[2] != "c" {
			t.Errorf("got %v, want [a b c]", result)
		}
	})

	t.Run("struct", func(t *testing.T) {
		rr := httptest.NewRecorder()
		type resp struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		}
		data := resp{Name: "test-skill", Count: 42}
		jsonResponse(rr, data)

		if rr.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
		}

		var result resp
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if result.Name != "test-skill" {
			t.Errorf("name = %q, want test-skill", result.Name)
		}
		if result.Count != 42 {
			t.Errorf("count = %d, want 42", result.Count)
		}
	})

	t.Run("nil value", func(t *testing.T) {
		rr := httptest.NewRecorder()
		jsonResponse(rr, nil)

		if rr.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
		}

		body := strings.TrimSpace(rr.Body.String())
		if body != "null" {
			t.Errorf("body = %q, want null", body)
		}
	})

	t.Run("empty map", func(t *testing.T) {
		rr := httptest.NewRecorder()
		jsonResponse(rr, map[string]string{})

		if rr.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
		}

		var result map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})

	t.Run("nested structure", func(t *testing.T) {
		rr := httptest.NewRecorder()
		data := map[string]interface{}{
			"skills": []string{"a", "b"},
			"count":  2,
		}
		jsonResponse(rr, data)

		var result map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		skills, ok := result["skills"].([]interface{})
		if !ok || len(skills) != 2 {
			t.Errorf("skills = %v, want [a b]", result["skills"])
		}
	})

	t.Run("boolean value", func(t *testing.T) {
		rr := httptest.NewRecorder()
		jsonResponse(rr, map[string]bool{"success": true})

		var result map[string]bool
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if !result["success"] {
			t.Error("success = false, want true")
		}
	})
}

func TestJsonError(t *testing.T) {
	tests := []struct {
		name       string
		message    string
		code       int
		wantStatus int
	}{
		{
			name:       "bad request",
			message:    "test error",
			code:       http.StatusBadRequest,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not found",
			message:    "resource not found",
			code:       http.StatusNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "internal server error",
			message:    "something went wrong",
			code:       http.StatusInternalServerError,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "method not allowed",
			message:    "method not allowed",
			code:       http.StatusMethodNotAllowed,
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "empty message",
			message:    "",
			code:       http.StatusBadRequest,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "message with special characters",
			message:    "error: invalid <input> & \"data\"",
			code:       http.StatusBadRequest,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			jsonError(rr, tt.message, tt.code)

			if rr.Code != tt.wantStatus {
				t.Errorf("status code = %d, want %d", rr.Code, tt.wantStatus)
			}

			if rr.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
			}

			var result map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if result["error"] != tt.message {
				t.Errorf("error = %q, want %q", result["error"], tt.message)
			}
		})
	}
}

func TestSanitizeAndRestrictPath(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("cannot get cwd: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		wantErr bool
		wantDir string // if non-empty, the result must start with this prefix
	}{
		{
			name:    "home directory itself",
			input:   homeDir,
			wantErr: false,
			wantDir: homeDir,
		},
		{
			name:    "subdirectory of home",
			input:   filepath.Join(homeDir, "projects", "test"),
			wantErr: false,
			wantDir: homeDir,
		},
		{
			name:    "current working directory",
			input:   cwd,
			wantErr: false,
		},
		{
			name:    "subdirectory of cwd",
			input:   filepath.Join(cwd, "subdir"),
			wantErr: false,
		},
		{
			name:    "dot-dot traversal to root",
			input:   "/tmp/../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "absolute path outside home and cwd",
			input:   "/etc/passwd",
			wantErr: true,
		},
		{
			name:    "root path",
			input:   "/",
			wantErr: true,
		},
		{
			name:    "path with dot-dot in middle",
			input:   filepath.Join(homeDir, "a", "..", "..", "..", "etc", "passwd"),
			wantErr: true,
		},
		{
			name:    "relative path within cwd",
			input:   ".",
			wantErr: false,
		},
		{
			name:    "relative path with subdirectory",
			input:   "./subdir",
			wantErr: false,
		},
		{
			name:    "tmp directory",
			input:   "/tmp",
			wantErr: true,
		},
		{
			name:    "usr directory",
			input:   "/usr/local/bin",
			wantErr: true,
		},
		{
			name:    "home with trailing separator",
			input:   homeDir + string(filepath.Separator),
			wantErr: false,
			wantDir: homeDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := sanitizeAndRestrictPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeAndRestrictPath(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if result == "" {
					t.Error("expected non-empty result for valid path")
				}
				if tt.wantDir != "" && !strings.HasPrefix(result, tt.wantDir) {
					t.Errorf("result %q does not start with %q", result, tt.wantDir)
				}
				// Result should always be an absolute clean path
				if !filepath.IsAbs(result) {
					t.Errorf("result %q is not absolute", result)
				}
				if result != filepath.Clean(result) {
					t.Errorf("result %q is not clean (clean = %q)", result, filepath.Clean(result))
				}
			}
		})
	}
}

func TestLimitRequestBody(t *testing.T) {
	// Create a request with a body larger than the limit
	largeBody := strings.Repeat("x", 2<<20) // 2MB, larger than 1MB limit
	req := httptest.NewRequest("POST", "/api/test", bytes.NewBufferString(largeBody))

	w := httptest.NewRecorder()
	limitRequestBody(w, req)

	// Try to read the body - should fail after limit
	buf := make([]byte, 2<<20)
	_, err := req.Body.Read(buf)

	// Reading past the limit should return an error
	if err == nil {
		// Read more to trigger the limit
		for {
			_, err = req.Body.Read(buf)
			if err != nil {
				break
			}
		}
	}

	// The error should be http: request body too large or EOF
	if err == nil {
		t.Error("Expected error when reading oversized body")
	}
}

func TestHandlerMethodNotAllowed(t *testing.T) {
	s := New(0, "test")

	tests := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		path    string
	}{
		{"skills GET on POST endpoint", s.handleSkillInstall, "GET", "/api/skills/install"},
		{"skills DELETE on POST endpoint", s.handleSkillInstall, "DELETE", "/api/skills/install"},
		{"repos POST on GET endpoint", s.handleRepos, "POST", "/api/repos"},
		{"config POST on GET endpoint", s.handleConfig, "POST", "/api/config"},
		{"skills search POST on GET endpoint", s.handleSkillSearch, "POST", "/api/skills/search"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()

			tt.handler(rr, req)

			if rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
			}
		})
	}
}

func TestHandleSkillInstallValidation(t *testing.T) {
	s := New(0, "test")

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "invalid json",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty name",
			body:       `{"name":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "injection attempt",
			body:       `{"name":"skill;rm -rf /"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "directory traversal",
			body:       `{"name":"../../../etc/passwd"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/skills/install", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			s.handleSkillInstall(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}

func TestHandleSkillScanValidation(t *testing.T) {
	s := New(0, "test")

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "empty path",
			body:       `{"path":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid json",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nonexistent path",
			body:       `{"path":"/nonexistent/path/12345"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/skills/scan", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			s.handleSkillScan(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}

func TestHandleSkillFilesValidation(t *testing.T) {
	s := New(0, "test")

	tests := []struct {
		name       string
		query      string
		wantStatus int
	}{
		{
			name:       "missing skill name",
			query:      "",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "nonexistent skill",
			query:      "?skill=nonexistent-skill-12345",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/skills/files"+tt.query, nil)
			rr := httptest.NewRecorder()

			s.handleSkillFiles(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}

func TestHandleRepoAddValidation(t *testing.T) {
	s := New(0, "test")

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "empty url",
			body:       `{"url":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "invalid json",
			body:       "not json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "http url rejected",
			body:       `{"url":"http://example.com/repo"}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/api/repos/add", bytes.NewBufferString(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()

			s.handleRepoAdd(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}

func TestReadFileNoSymlink(t *testing.T) {
	t.Run("regular file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hello.txt")
		if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
			t.Fatal(err)
		}

		data, err := readFileNoSymlink(path, 1024)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(data) != "hello world" {
			t.Errorf("got %q, want %q", string(data), "hello world")
		}
	})

	t.Run("symlink to file", func(t *testing.T) {
		dir := t.TempDir()
		realPath := filepath.Join(dir, "real.txt")
		if err := os.WriteFile(realPath, []byte("secret"), 0644); err != nil {
			t.Fatal(err)
		}
		linkPath := filepath.Join(dir, "link.txt")
		if err := os.Symlink(realPath, linkPath); err != nil {
			t.Fatal(err)
		}

		_, err := readFileNoSymlink(linkPath, 1024)
		if err == nil {
			t.Fatal("expected error for symlink, got nil")
		}
		if !errors.Is(err, os.ErrPermission) {
			t.Errorf("expected permission error, got: %v", err)
		}
	})

	t.Run("file larger than maxSize", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "big.txt")
		if err := os.WriteFile(path, bytes.Repeat([]byte("x"), 2048), 0644); err != nil {
			t.Fatal(err)
		}

		_, err := readFileNoSymlink(path, 1024)
		if err == nil {
			t.Fatal("expected error for oversized file, got nil")
		}
		if !strings.Contains(err.Error(), "file too large") {
			t.Errorf("expected 'file too large' error, got: %v", err)
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "no-such-file.txt")

		_, err := readFileNoSymlink(path, 1024)
		if err == nil {
			t.Fatal("expected error for nonexistent file, got nil")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "empty.txt")
		if err := os.WriteFile(path, []byte{}, 0644); err != nil {
			t.Fatal(err)
		}

		data, err := readFileNoSymlink(path, 1024)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(data) != 0 {
			t.Errorf("expected empty bytes, got %d bytes", len(data))
		}
	})
}

func TestBuildFileTree(t *testing.T) {
	t.Run("normal directory with files", func(t *testing.T) {
		dir := t.TempDir()
		// Create files and a subdirectory
		if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("one"), 0644); err != nil {
			t.Fatal(err)
		}
		subDir := filepath.Join(dir, "subdir")
		if err := os.Mkdir(subDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(subDir, "file2.txt"), []byte("two"), 0644); err != nil {
			t.Fatal(err)
		}

		root, err := buildFileTree(dir, "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root.Type != "dir" {
			t.Errorf("root type = %q, want %q", root.Type, "dir")
		}
		if len(root.Children) != 2 {
			t.Fatalf("root children count = %d, want 2", len(root.Children))
		}

		// Find children by name
		childMap := make(map[string]*FileNode)
		for _, c := range root.Children {
			childMap[c.Name] = c
		}

		fileNode, ok := childMap["file1.txt"]
		if !ok {
			t.Fatal("file1.txt not found in children")
		}
		if fileNode.Type != "file" {
			t.Errorf("file1.txt type = %q, want %q", fileNode.Type, "file")
		}

		dirNode, ok := childMap["subdir"]
		if !ok {
			t.Fatal("subdir not found in children")
		}
		if dirNode.Type != "dir" {
			t.Errorf("subdir type = %q, want %q", dirNode.Type, "dir")
		}
		if len(dirNode.Children) != 1 {
			t.Fatalf("subdir children count = %d, want 1", len(dirNode.Children))
		}
		if dirNode.Children[0].Name != "file2.txt" {
			t.Errorf("subdir child name = %q, want %q", dirNode.Children[0].Name, "file2.txt")
		}
	})

	t.Run("git directory is skipped", func(t *testing.T) {
		dir := t.TempDir()
		gitDir := filepath.Join(dir, ".git")
		if err := os.Mkdir(gitDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte("git config"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme"), 0644); err != nil {
			t.Fatal(err)
		}

		root, err := buildFileTree(dir, "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		for _, c := range root.Children {
			if c.Name == ".git" {
				t.Error(".git directory should be skipped but was found in children")
			}
		}
		if len(root.Children) != 1 {
			t.Errorf("expected 1 child (README.md), got %d", len(root.Children))
		}
	})

	t.Run("symlinks are labeled and not recursed", func(t *testing.T) {
		dir := t.TempDir()
		// Create a real subdirectory with a file
		realDir := filepath.Join(dir, "real")
		if err := os.Mkdir(realDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(realDir, "secret.txt"), []byte("secret"), 0644); err != nil {
			t.Fatal(err)
		}
		// Create a symlink to the directory
		linkPath := filepath.Join(dir, "link")
		if err := os.Symlink(realDir, linkPath); err != nil {
			t.Fatal(err)
		}

		root, err := buildFileTree(dir, "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		childMap := make(map[string]*FileNode)
		for _, c := range root.Children {
			childMap[c.Name] = c
		}

		linkNode, ok := childMap["link"]
		if !ok {
			t.Fatal("symlink 'link' not found in children")
		}
		if linkNode.Type != "symlink" {
			t.Errorf("symlink type = %q, want %q", linkNode.Type, "symlink")
		}
		if linkNode.Children != nil {
			t.Error("symlink node should not have children")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()

		root, err := buildFileTree(dir, "", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if root.Type != "dir" {
			t.Errorf("root type = %q, want %q", root.Type, "dir")
		}
		if len(root.Children) != 0 {
			t.Errorf("expected 0 children, got %d", len(root.Children))
		}
	})
}

func TestParseGitConfigForRepo(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "empty file",
			content: "",
			want:    "",
		},
		{
			name:    "https origin",
			content: "[core]\n\trepositoryformatversion = 0\n[remote \"origin\"]\n\turl = https://github.com/owner/repo.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n",
			want:    "owner/repo",
		},
		{
			name:    "ssh origin",
			content: "[remote \"origin\"]\n\turl = git@github.com:owner/repo.git\n\tfetch = +refs/heads/*:refs/remotes/origin/*\n",
			want:    "owner/repo",
		},
		{
			name:    "no origin remote",
			content: "[remote \"upstream\"]\n\turl = https://github.com/upstream/repo.git\n",
			want:    "",
		},
		{
			name:    "non-github url",
			content: "[remote \"origin\"]\n\turl = https://gitlab.com/owner/repo.git\n",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), "config")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}
			got := parseGitConfigForRepo(tmpFile)
			if got != tt.want {
				t.Errorf("parseGitConfigForRepo() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseGitConfigForRepo_NonexistentFile(t *testing.T) {
	got := parseGitConfigForRepo("/nonexistent/path/config")
	if got != "" {
		t.Errorf("expected empty string for nonexistent file, got %q", got)
	}
}

func TestHandleSkillUninstall_Validation(t *testing.T) {
	s := New(0, "test")

	tests := []struct {
		name       string
		method     string
		body       string
		ctHeader   string
		wantStatus int
	}{
		{
			name:       "GET method not allowed",
			method:     "GET",
			body:       "",
			ctHeader:   "",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "POST with invalid JSON",
			method:     "POST",
			body:       "not json",
			ctHeader:   "application/json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "POST with empty name",
			method:     "POST",
			body:       `{"name":""}`,
			ctHeader:   "application/json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "POST with path traversal in name",
			method:     "POST",
			body:       `{"name":"../test"}`,
			ctHeader:   "application/json",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, "/api/skills/uninstall", bytes.NewBufferString(tt.body))
			} else {
				req = httptest.NewRequest(tt.method, "/api/skills/uninstall", nil)
			}
			if tt.ctHeader != "" {
				req.Header.Set("Content-Type", tt.ctHeader)
			}
			rr := httptest.NewRecorder()

			s.handleSkillUninstall(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}

func TestHandleSkillImport_Validation(t *testing.T) {
	s := New(0, "test")

	tests := []struct {
		name       string
		method     string
		body       string
		ctHeader   string
		wantStatus int
	}{
		{
			name:       "GET method not allowed",
			method:     "GET",
			body:       "",
			ctHeader:   "",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "POST with invalid JSON",
			method:     "POST",
			body:       "not json",
			ctHeader:   "application/json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "POST with empty src_path",
			method:     "POST",
			body:       `{"src_path":""}`,
			ctHeader:   "application/json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "POST with src_path starting with dash",
			method:     "POST",
			body:       `{"src_path":"-malicious"}`,
			ctHeader:   "application/json",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, "/api/skills/import", bytes.NewBufferString(tt.body))
			} else {
				req = httptest.NewRequest(tt.method, "/api/skills/import", nil)
			}
			if tt.ctHeader != "" {
				req.Header.Set("Content-Type", tt.ctHeader)
			}
			rr := httptest.NewRecorder()

			s.handleSkillImport(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}
		})
	}
}

func TestHandleSkillReadme_Validation(t *testing.T) {
	s := New(0, "test")

	tests := []struct {
		name       string
		method     string
		query      string
		wantStatus int
		wantErr    string
	}{
		{
			name:       "POST method not allowed",
			method:     "POST",
			query:      "",
			wantStatus: http.StatusMethodNotAllowed,
			wantErr:    "Method not allowed",
		},
		{
			name:       "DELETE method not allowed",
			method:     "DELETE",
			query:      "",
			wantStatus: http.StatusMethodNotAllowed,
			wantErr:    "Method not allowed",
		},
		{
			name:       "PUT method not allowed",
			method:     "PUT",
			query:      "",
			wantStatus: http.StatusMethodNotAllowed,
			wantErr:    "Method not allowed",
		},
		{
			name:       "GET without name param",
			method:     "GET",
			query:      "",
			wantStatus: http.StatusBadRequest,
			wantErr:    "Invalid skill name",
		},
		{
			name:       "GET with empty name param",
			method:     "GET",
			query:      "?name=",
			wantStatus: http.StatusBadRequest,
			wantErr:    "Invalid skill name",
		},
		{
			name:       "GET with invalid name containing semicolon",
			method:     "GET",
			query:      "?name=test;cmd",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "GET with path traversal in name",
			method:     "GET",
			query:      "?name=../etc/passwd",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "GET with shell injection pipe",
			method:     "GET",
			query:      "?name=skill|cat",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "GET with shell injection backtick",
			method:     "GET",
			query:      "?name=skill`whoami`",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "GET with nonexistent valid skill name",
			method:     "GET",
			query:      "?name=nonexistent-skill-99999",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/skills/readme"+tt.query, nil)
			rr := httptest.NewRecorder()

			s.handleSkillReadme(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if rr.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
			}

			if tt.wantErr != "" {
				var result map[string]string
				if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if !strings.Contains(result["error"], tt.wantErr) {
					t.Errorf("error = %q, want to contain %q", result["error"], tt.wantErr)
				}
			}
		})
	}
}

func TestHandleCacheClear_Validation(t *testing.T) {
	s := New(0, "test")

	tests := []struct {
		name       string
		method     string
		body       string
		ctHeader   string
		wantStatus int
		wantErr    string
	}{
		{
			name:       "GET method not allowed",
			method:     "GET",
			body:       "",
			ctHeader:   "",
			wantStatus: http.StatusMethodNotAllowed,
			wantErr:    "Method not allowed",
		},
		{
			name:       "DELETE method not allowed",
			method:     "DELETE",
			body:       "",
			ctHeader:   "",
			wantStatus: http.StatusMethodNotAllowed,
			wantErr:    "Method not allowed",
		},
		{
			name:       "PUT method not allowed",
			method:     "PUT",
			body:       "",
			ctHeader:   "",
			wantStatus: http.StatusMethodNotAllowed,
			wantErr:    "Method not allowed",
		},
		{
			name:       "POST without content-type",
			method:     "POST",
			body:       "{}",
			ctHeader:   "",
			wantStatus: http.StatusUnsupportedMediaType,
			wantErr:    "Content-Type must be application/json",
		},
		{
			name:       "POST with text/plain content-type",
			method:     "POST",
			body:       "{}",
			ctHeader:   "text/plain",
			wantStatus: http.StatusUnsupportedMediaType,
			wantErr:    "Content-Type must be application/json",
		},
		{
			name:       "POST with multipart/form-data content-type",
			method:     "POST",
			body:       "{}",
			ctHeader:   "multipart/form-data",
			wantStatus: http.StatusUnsupportedMediaType,
			wantErr:    "Content-Type must be application/json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, "/api/cache/clear", bytes.NewBufferString(tt.body))
			} else {
				req = httptest.NewRequest(tt.method, "/api/cache/clear", nil)
			}
			if tt.ctHeader != "" {
				req.Header.Set("Content-Type", tt.ctHeader)
			}
			rr := httptest.NewRecorder()

			s.handleCacheClear(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if rr.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
			}

			if tt.wantErr != "" {
				var result map[string]string
				if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if !strings.Contains(result["error"], tt.wantErr) {
					t.Errorf("error = %q, want to contain %q", result["error"], tt.wantErr)
				}
			}
		})
	}
}

func TestHandleStats(t *testing.T) {
	s := New(0, "test")

	t.Run("GET returns 200 with valid StatsInfo JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats", nil)
		rr := httptest.NewRecorder()

		s.handleStats(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}

		ct := rr.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		var result StatsInfo
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode JSON response: %v", err)
		}

		// Verify non-negative counts
		if result.InstalledSkills < 0 {
			t.Errorf("installed_skills = %d, want >= 0", result.InstalledSkills)
		}
		if result.ConfiguredRepos < 0 {
			t.Errorf("configured_repos = %d, want >= 0", result.ConfiguredRepos)
		}
		if result.SyncedRepos < 0 {
			t.Errorf("synced_repos = %d, want >= 0", result.SyncedRepos)
		}
	})

	t.Run("POST method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/stats", nil)
		rr := httptest.NewRecorder()

		s.handleStats(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusMethodNotAllowed, rr.Body.String())
		}

		var result map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if !strings.Contains(result["error"], "Method not allowed") {
			t.Errorf("error = %q, want to contain 'Method not allowed'", result["error"])
		}
	})

	t.Run("DELETE method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/stats", nil)
		rr := httptest.NewRecorder()

		s.handleStats(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("PUT method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/stats", nil)
		rr := httptest.NewRecorder()

		s.handleStats(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("response contains expected JSON fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/stats", nil)
		rr := httptest.NewRecorder()

		s.handleStats(rr, req)

		var raw map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&raw); err != nil {
			t.Fatalf("Failed to decode JSON: %v", err)
		}

		expectedFields := []string{"installed_skills", "configured_repos", "synced_repos"}
		for _, field := range expectedFields {
			if _, ok := raw[field]; !ok {
				t.Errorf("missing expected field %q in response", field)
			}
		}
	})
}

func TestHandleConfig(t *testing.T) {
	s := New(0, "test-version")

	t.Run("GET returns 200 with valid ConfigInfo JSON", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/config", nil)
		rr := httptest.NewRecorder()

		s.handleConfig(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusOK, rr.Body.String())
		}

		ct := rr.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		var result ConfigInfo
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode JSON response: %v", err)
		}

		// Verify version matches what was set on the server
		if result.Version != "test-version" {
			t.Errorf("version = %q, want %q", result.Version, "test-version")
		}

		// Verify agents list is populated
		if len(result.Agents) == 0 {
			t.Error("agents list should not be empty")
		}

		// Verify skills_dir is non-empty
		if result.SkillsDir == "" {
			t.Error("skills_dir should not be empty")
		}
	})

	t.Run("POST method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/config", nil)
		rr := httptest.NewRecorder()

		s.handleConfig(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusMethodNotAllowed, rr.Body.String())
		}

		var result map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if !strings.Contains(result["error"], "Method not allowed") {
			t.Errorf("error = %q, want to contain 'Method not allowed'", result["error"])
		}
	})

	t.Run("DELETE method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/config", nil)
		rr := httptest.NewRecorder()

		s.handleConfig(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("PUT method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/config", nil)
		rr := httptest.NewRecorder()

		s.handleConfig(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("response contains expected JSON fields", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/config", nil)
		rr := httptest.NewRecorder()

		s.handleConfig(rr, req)

		var raw map[string]interface{}
		if err := json.NewDecoder(rr.Body).Decode(&raw); err != nil {
			t.Fatalf("Failed to decode JSON: %v", err)
		}

		expectedFields := []string{"version", "skills_dir", "agents", "tool_targets", "global_dir", "initialized"}
		for _, field := range expectedFields {
			if _, ok := raw[field]; !ok {
				t.Errorf("missing expected field %q in response", field)
			}
		}
	})

	t.Run("version reflects server version", func(t *testing.T) {
		s2 := New(0, "v2.0.0-rc1")
		req := httptest.NewRequest("GET", "/api/config", nil)
		rr := httptest.NewRecorder()

		s2.handleConfig(rr, req)

		var result ConfigInfo
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode JSON: %v", err)
		}
		if result.Version != "v2.0.0-rc1" {
			t.Errorf("version = %q, want %q", result.Version, "v2.0.0-rc1")
		}
	})
}

func TestHandleSkillSearch_Validation(t *testing.T) {
	s := New(0, "test")

	t.Run("POST method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/api/skills/search", nil)
		rr := httptest.NewRecorder()

		s.handleSkillSearch(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusMethodNotAllowed, rr.Body.String())
		}

		var result map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if !strings.Contains(result["error"], "Method not allowed") {
			t.Errorf("error = %q, want to contain 'Method not allowed'", result["error"])
		}
	})

	t.Run("DELETE method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("DELETE", "/api/skills/search", nil)
		rr := httptest.NewRecorder()

		s.handleSkillSearch(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("PUT method not allowed", func(t *testing.T) {
		req := httptest.NewRequest("PUT", "/api/skills/search", nil)
		rr := httptest.NewRecorder()

		s.handleSkillSearch(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want %d", rr.Code, http.StatusMethodNotAllowed)
		}
	})

	t.Run("query too long", func(t *testing.T) {
		longQuery := strings.Repeat("a", 256)
		req := httptest.NewRequest("GET", "/api/skills/search?q="+longQuery, nil)
		rr := httptest.NewRecorder()

		s.handleSkillSearch(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
		}

		var result map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if !strings.Contains(result["error"], "Query too long") {
			t.Errorf("error = %q, want to contain 'Query too long'", result["error"])
		}
	})

	t.Run("query exactly at limit is allowed", func(t *testing.T) {
		// 255 characters should be accepted (validation only rejects > 255)
		query := strings.Repeat("a", 255)
		req := httptest.NewRequest("GET", "/api/skills/search?q="+query, nil)
		rr := httptest.NewRecorder()

		s.handleSkillSearch(rr, req)

		// Should not be a 400 for query length
		if rr.Code == http.StatusBadRequest {
			var result map[string]string
			if err := json.NewDecoder(rr.Body).Decode(&result); err == nil {
				if strings.Contains(result["error"], "Query too long") {
					t.Errorf("query of exactly 255 chars should not be rejected as too long")
				}
			}
		}
	})

	t.Run("repo filter too long", func(t *testing.T) {
		longRepo := strings.Repeat("b", 256)
		req := httptest.NewRequest("GET", "/api/skills/search?q=test&repo="+longRepo, nil)
		rr := httptest.NewRecorder()

		s.handleSkillSearch(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("status = %d, want %d, body: %s", rr.Code, http.StatusBadRequest, rr.Body.String())
		}

		var result map[string]string
		if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}
		if !strings.Contains(result["error"], "Repo filter too long") {
			t.Errorf("error = %q, want to contain 'Repo filter too long'", result["error"])
		}
	})

	t.Run("response is JSON on error", func(t *testing.T) {
		longQuery := strings.Repeat("x", 300)
		req := httptest.NewRequest("GET", "/api/skills/search?q="+longQuery, nil)
		rr := httptest.NewRecorder()

		s.handleSkillSearch(rr, req)

		ct := rr.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
	})
}

func TestHandleRepoSync_Validation(t *testing.T) {
	s := New(0, "test")

	tests := []struct {
		name       string
		method     string
		body       string
		ctHeader   string
		wantStatus int
		wantErr    string
	}{
		{
			name:       "GET method not allowed",
			method:     "GET",
			body:       "",
			ctHeader:   "",
			wantStatus: http.StatusMethodNotAllowed,
			wantErr:    "Method not allowed",
		},
		{
			name:       "DELETE method not allowed",
			method:     "DELETE",
			body:       "",
			ctHeader:   "",
			wantStatus: http.StatusMethodNotAllowed,
			wantErr:    "Method not allowed",
		},
		{
			name:       "PUT method not allowed",
			method:     "PUT",
			body:       "",
			ctHeader:   "",
			wantStatus: http.StatusMethodNotAllowed,
			wantErr:    "Method not allowed",
		},
		{
			name:       "POST without content-type",
			method:     "POST",
			body:       "{}",
			ctHeader:   "",
			wantStatus: http.StatusUnsupportedMediaType,
			wantErr:    "Content-Type must be application/json",
		},
		{
			name:       "POST with text/plain content-type",
			method:     "POST",
			body:       "{}",
			ctHeader:   "text/plain",
			wantStatus: http.StatusUnsupportedMediaType,
			wantErr:    "Content-Type must be application/json",
		},
		{
			name:       "POST with form-urlencoded content-type",
			method:     "POST",
			body:       "name=test",
			ctHeader:   "application/x-www-form-urlencoded",
			wantStatus: http.StatusUnsupportedMediaType,
			wantErr:    "Content-Type must be application/json",
		},
		{
			name:       "POST with invalid name containing semicolon",
			method:     "POST",
			body:       `{"name":"test;rm -rf"}`,
			ctHeader:   "application/json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "POST with path traversal in name",
			method:     "POST",
			body:       `{"name":"../etc/passwd"}`,
			ctHeader:   "application/json",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "POST with malformed JSON body",
			method:     "POST",
			body:       `{invalid json`,
			ctHeader:   "application/json",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != "" {
				req = httptest.NewRequest(tt.method, "/api/repos/sync", bytes.NewBufferString(tt.body))
			} else {
				req = httptest.NewRequest(tt.method, "/api/repos/sync", nil)
			}
			if tt.ctHeader != "" {
				req.Header.Set("Content-Type", tt.ctHeader)
			}
			rr := httptest.NewRecorder()

			s.handleRepoSync(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d, body: %s", rr.Code, tt.wantStatus, rr.Body.String())
			}

			if rr.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", rr.Header().Get("Content-Type"))
			}

			if tt.wantErr != "" {
				var result map[string]string
				if err := json.NewDecoder(rr.Body).Decode(&result); err != nil {
					t.Fatalf("Failed to decode response: %v", err)
				}
				if !strings.Contains(result["error"], tt.wantErr) {
					t.Errorf("error = %q, want to contain %q", result["error"], tt.wantErr)
				}
			}
		})
	}
}

func TestRequireJSONContentType(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		ct         string
		wantResult bool
		wantStatus int
	}{
		{
			name:       "non-POST request passes",
			method:     "GET",
			ct:         "",
			wantResult: true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST with application/json passes",
			method:     "POST",
			ct:         "application/json",
			wantResult: true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST with application/json charset passes",
			method:     "POST",
			ct:         "application/json; charset=utf-8",
			wantResult: true,
			wantStatus: http.StatusOK,
		},
		{
			name:       "POST with application/jsonl rejected",
			method:     "POST",
			ct:         "application/jsonl",
			wantResult: false,
			wantStatus: http.StatusUnsupportedMediaType,
		},
		{
			name:       "POST with text/plain fails",
			method:     "POST",
			ct:         "text/plain",
			wantResult: false,
			wantStatus: http.StatusUnsupportedMediaType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/api/test", nil)
			if tt.ct != "" {
				req.Header.Set("Content-Type", tt.ct)
			}
			rr := httptest.NewRecorder()

			result := requireJSONContentType(rr, req)

			if result != tt.wantResult {
				t.Errorf("requireJSONContentType() = %v, want %v", result, tt.wantResult)
			}
			if !tt.wantResult && rr.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tt.wantStatus)
			}
		})
	}
}
