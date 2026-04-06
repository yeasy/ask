// Package cache manages the local caching of repositories and skills.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/yeasy/ask/internal/filesystem"
)

// maxCacheFileSize limits cache file reads to prevent OOM from malformed files
const maxCacheFileSize = 5 * 1024 * 1024 // 5MB

// Cache provides file-based caching with TTL
type Cache struct {
	dir string
	ttl time.Duration
}

// Entry represents a cached item with metadata
type Entry struct {
	Data      json.RawMessage `json:"data"`
	CreatedAt time.Time       `json:"created_at"`
}

// DefaultTTL is the default cache time-to-live
const DefaultTTL = 5 * time.Minute

// New creates a new cache instance
func New(cacheDir string, ttl time.Duration) (*Cache, error) {
	if cacheDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home dir: %w", err)
		}
		cacheDir = filepath.Join(homeDir, ".ask", "cache")
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}

	if ttl == 0 {
		ttl = DefaultTTL
	}

	return &Cache{
		dir: cacheDir,
		ttl: ttl,
	}, nil
}

// hashKey creates a consistent hash for the cache key
func hashKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

// Get retrieves a value from the cache if it exists and is not expired
func (c *Cache) Get(key string, v interface{}) bool {
	filename := filepath.Join(c.dir, hashKey(key)+".json")

	f, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, maxCacheFileSize+1))
	if err != nil || int64(len(data)) > maxCacheFileSize {
		return false
	}

	var entry Entry
	if err := json.Unmarshal(data, &entry); err != nil {
		return false
	}

	// Check expiration
	if time.Since(entry.CreatedAt) > c.ttl {
		_ = os.Remove(filename) // Clean up expired entry
		return false
	}

	if err := json.Unmarshal(entry.Data, v); err != nil {
		return false
	}

	return true
}

// Set stores a value in the cache
func (c *Cache) Set(key string, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	entry := Entry{
		Data:      data,
		CreatedAt: time.Now(),
	}

	entryData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal entry: %w", err)
	}

	filename := filepath.Join(c.dir, hashKey(key)+".json")
	return filesystem.AtomicWriteFile(filename, entryData, 0600)
}

// Clear removes all cached entries
func (c *Cache) Clear() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".json" {
			_ = os.Remove(filepath.Join(c.dir, entry.Name()))
		}
	}

	return nil
}
