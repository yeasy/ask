package cache

import (
	"testing"
	"time"
)

func TestCacheSetGet(t *testing.T) {
	cache, err := New(t.TempDir(), DefaultTTL)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Test set and get
	testData := map[string]string{"key": "value"}
	if err := cache.Set("test-key", testData); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	var result map[string]string
	if !cache.Get("test-key", &result) {
		t.Fatal("expected to get cached value")
	}

	if result["key"] != "value" {
		t.Errorf("expected 'value', got %s", result["key"])
	}
}

func TestCacheExpiry(t *testing.T) {
	// Use a short TTL for testing (500ms avoids flakiness under race detector)
	cache, err := New(t.TempDir(), 500*time.Millisecond)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	testData := "test-value"
	if err := cache.Set("expiry-key", testData); err != nil {
		t.Fatalf("failed to set cache: %v", err)
	}

	// Should get value immediately
	var result string
	if !cache.Get("expiry-key", &result) {
		t.Fatal("expected to get cached value before expiry")
	}

	// Wait for expiry
	time.Sleep(600 * time.Millisecond)

	// Should not get expired value
	if cache.Get("expiry-key", &result) {
		t.Fatal("expected cache to be expired")
	}
}

func TestCacheClear(t *testing.T) {
	cache, err := New(t.TempDir(), DefaultTTL)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	// Set multiple values
	_ = cache.Set("key1", "value1")
	_ = cache.Set("key2", "value2")

	// Clear cache
	if err := cache.Clear(); err != nil {
		t.Fatalf("failed to clear cache: %v", err)
	}

	// Should not get any values
	var result string
	if cache.Get("key1", &result) {
		t.Fatal("expected cache to be cleared")
	}
	if cache.Get("key2", &result) {
		t.Fatal("expected cache to be cleared")
	}
}

func TestCacheMiss(t *testing.T) {
	cache, err := New(t.TempDir(), DefaultTTL)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	var result string
	if cache.Get("nonexistent-key", &result) {
		t.Fatal("expected cache miss for nonexistent key")
	}
}
