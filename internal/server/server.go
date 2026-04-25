// Package server provides an embedded HTTP server for the ask web UI.
package server

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/yeasy/ask/internal/ui"
)

//go:embed web/*
var webFS embed.FS

// Server represents the HTTP server
type Server struct {
	port    int
	server  *http.Server
	mu      sync.Mutex
	cwdMu   sync.RWMutex // protects os.Chdir; write-lock for Chdir, read-lock for Getwd
	version string
}

// New creates a new Server instance
func New(port int, version string) *Server {
	return &Server{
		port:    port,
		version: version,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	mux := s.setupRoutes()

	// Static file serving
	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		return fmt.Errorf("failed to create sub filesystem: %w", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webContent)))

	server := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", s.port),
		Handler:           securityHeadersMiddleware(corsMiddleware(mux)),
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}

	s.mu.Lock()
	s.server = server
	s.mu.Unlock()

	ui.Info(fmt.Sprintf("Starting server on http://127.0.0.1:%d", s.port))
	return server.ListenAndServe()
}

// setupRoutes returns the API mux
func (s *Server) setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/skills", s.handleSkills)
	mux.HandleFunc("/api/skills/search", s.handleSkillSearch)
	mux.HandleFunc("/api/skills/install", s.handleSkillInstall)
	mux.HandleFunc("/api/skills/uninstall", s.handleSkillUninstall)

	// New SkillsLM API routes
	mux.HandleFunc("/api/skills/scan", s.handleSkillScan)
	mux.HandleFunc("/api/skills/import", s.handleSkillImport)
	mux.HandleFunc("/api/skills/files", s.handleSkillFiles) // ?path=...

	mux.HandleFunc("/api/repos", s.handleRepos)
	mux.HandleFunc("/api/repos/add", s.handleRepoAdd)
	mux.HandleFunc("/api/repos/remove", s.handleRepoRemove)
	mux.HandleFunc("/api/repos/sync", s.handleRepoSync)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/config/update", s.handleConfigUpdate)
	mux.HandleFunc("/api/cache/clear", s.handleCacheClear)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/skills/readme", s.handleSkillReadme)

	return mux
}

// Handler returns the HTTP handler for the server (exported for Wails integration)
func (s *Server) Handler() http.Handler {
	return securityHeadersMiddleware(corsMiddleware(s.setupRoutes()))
}

// Stop gracefully stops the server
func (s *Server) Stop(ctx context.Context) error {
	s.mu.Lock()
	server := s.server
	s.mu.Unlock()

	if server != nil {
		return server.Shutdown(ctx)
	}
	return nil
}

// OpenBrowser opens the default browser to the server URL
func OpenBrowser(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http and https URLs are supported")
	}

	// Reject URLs with characters that could be interpreted as shell metacharacters
	// on Windows (cmd.exe). url.Parse is lenient and allows characters like |, >, <
	// which cmd.exe would interpret as command operators. Also reject () to prevent
	// command grouping, ^ (cmd.exe escape char), ! (delayed expansion), and %
	// (environment variable expansion).
	safeURL := u.String()
	if strings.ContainsAny(safeURL, "|><\"'`()^!%") {
		return fmt.Errorf("URL contains unsafe characters")
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", safeURL)
	case "linux":
		cmd = exec.Command("xdg-open", safeURL)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", strings.ReplaceAll(safeURL, "&", "^&"))
	default:
		return fmt.Errorf("unsupported platform")
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }() // reap child to avoid zombie process
	return nil
}

// validateSkillName checks if a skill name is safe (alphanumeric, -, _, .)
func validateSkillName(name string) error {
	if name == "" {
		return fmt.Errorf("skill name is required")
	}
	// Allow alphanumeric, dash, underscore, dot, slash (for repo/path)
	// But disallow characters that could be used for shell injection like ; & | $ ` > <
	// Actually, strictly allow only a safe set.
	for _, r := range name {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' && r != '_' && r != '.' && r != '/' && r != '@' {
			return fmt.Errorf("invalid character in skill name: %c", r)
		}
	}
	// Check for directory traversal
	if strings.Contains(name, "..") {
		return fmt.Errorf("directory traversal not allowed")
	}
	return nil
}

// sanitizeAndRestrictPath resolves a raw path to an absolute path and restricts it
// to be within the user's home directory or current working directory.
// Symlinks are resolved when possible to prevent bypasses via symlinked path components.
func sanitizeAndRestrictPath(rawPath string) (string, error) {
	cleanPath, err := filepath.Abs(filepath.Clean(rawPath))
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}
	// Resolve symlinks to compare real paths (best-effort; path may not exist yet)
	if resolved, err := filepath.EvalSymlinks(cleanPath); err == nil {
		cleanPath = resolved
	}

	homeDir, homeErr := os.UserHomeDir()
	if homeErr == nil {
		if resolved, err := filepath.EvalSymlinks(homeDir); err == nil {
			homeDir = resolved
		}
	}
	cwd, cwdErr := os.Getwd()
	if cwdErr == nil {
		if resolved, err := filepath.EvalSymlinks(cwd); err == nil {
			cwd = resolved
		}
	}
	if homeErr != nil && cwdErr != nil {
		return "", fmt.Errorf("cannot determine safe base directory")
	}

	inHome := homeErr == nil && (cleanPath == homeDir || strings.HasPrefix(cleanPath, homeDir+string(filepath.Separator)))
	inCwd := cwdErr == nil && (cleanPath == cwd || strings.HasPrefix(cleanPath, cwd+string(filepath.Separator)))
	if !inHome && !inCwd {
		return "", fmt.Errorf("path must be within home directory or project directory")
	}

	return cleanPath, nil
}

// securityHeadersMiddleware adds security headers to prevent clickjacking and other attacks.
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "frame-ancestors 'none'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers for development, restricted to localhost
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := false
		if origin != "" {
			// Allow localhost/127.0.0.1 (strict prefix to prevent localhost.evil.com)
			if strings.HasPrefix(origin, "http://localhost:") ||
				strings.HasPrefix(origin, "http://localhost/") ||
				origin == "http://localhost" ||
				strings.HasPrefix(origin, "http://127.0.0.1:") ||
				strings.HasPrefix(origin, "http://127.0.0.1/") ||
				origin == "http://127.0.0.1" ||
				origin == "app://wails.localhost" || origin == "app://ask" { // Allow only known app origins
				allowed = true
			}
		} else {
			// No origin usually means same origin or direct request
			allowed = true
		}

		// Always set Vary: Origin since response varies by origin header
		w.Header().Set("Vary", "Origin")

		if allowed && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

const (
	// maxRequestBodySize limits the maximum size of request bodies (1MB)
	maxRequestBodySize = 1 << 20 // 1MB

	serverReadHeaderTimeout = 10 * time.Second
	serverReadTimeout       = 30 * time.Second
	serverWriteTimeout      = 60 * time.Second
	serverIdleTimeout       = 120 * time.Second
)

// limitRequestBody is a helper to limit request body size for POST handlers
func limitRequestBody(w http.ResponseWriter, r *http.Request) {
	if r.Body != nil {
		r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
	}
}

// requireJSONContentType checks that POST requests have Content-Type: application/json.
// This prevents blind CSRF attacks because cross-origin requests with non-simple
// content types trigger a CORS preflight that our CORS policy will reject.
// Returns true if the request is valid; writes an error response and returns false otherwise.
func requireJSONContentType(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		return true
	}
	ct := strings.TrimSpace(r.Header.Get("Content-Type"))
	// Accept "application/json" optionally followed by parameters (e.g., "; charset=utf-8")
	if ct != "application/json" && !strings.HasPrefix(ct, "application/json;") {
		jsonError(w, "Content-Type must be application/json", http.StatusUnsupportedMediaType)
		return false
	}
	return true
}

// getExecutable returns the path to the current executable, writing an error
// response and returning false if the lookup fails.
func getExecutable(w http.ResponseWriter) (string, bool) {
	exe, err := os.Executable()
	if err != nil {
		jsonError(w, "Failed to get executable path", http.StatusInternalServerError)
		return "", false
	}
	return exe, true
}

// JSON response helpers.
// Marshals to buffer first to avoid partial writes on encoding errors.
func jsonResponse(w http.ResponseWriter, data interface{}) {
	buf, err := json.Marshal(data)
	if err != nil {
		jsonError(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	buf = append(buf, '\n')
	_, _ = w.Write(buf)
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	buf, err := json.Marshal(map[string]string{"error": message})
	if err != nil {
		_, _ = w.Write([]byte(`{"error":"internal error"}` + "\n"))
		return
	}
	_, _ = w.Write(buf)
	_, _ = w.Write([]byte("\n"))
}
