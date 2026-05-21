package server

import (
	"context"
	"net/http"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	port := 8080
	s := New(port, "test-version")
	if s == nil {
		t.Fatal("New() returned nil")
	} else if s.port != port {
		t.Errorf("New() port = %d; want %d", s.port, port)
	}
}

func TestServer_StartStop(t *testing.T) {
	s := New(0, "test-version") // 0 to pick a random available port

	// Start server in a goroutine
	go func() {
		err := s.Start()
		if err != nil && err != http.ErrServerClosed {
			t.Logf("Server failed: %v", err)
		}
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check if we can stop it
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := s.Stop(ctx); err != nil {
		t.Errorf("Stop() failed: %v", err)
	}
}
