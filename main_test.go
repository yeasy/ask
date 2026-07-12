package main

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestReleaseBuildBehavior builds the binary exactly as the release pipeline
// does (plain `go build`, no Wails desktop build tags) and asserts the
// user-facing behavior of the standard CLI build.
func TestReleaseBuildBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping build+run integration test in -short mode")
	}

	bin := filepath.Join(t.TempDir(), "ask")
	if out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("go build failed: %v\n%s", err, out)
	}

	// Reproduces the original bug: running `ask` with no arguments must show CLI
	// help, not crash with "Wails applications will not build without the correct
	// build tags."
	t.Run("bare invocation shows CLI help", func(t *testing.T) {
		out, err := exec.Command(bin).CombinedOutput()
		if err != nil {
			t.Fatalf("bare `ask` exited with error: %v\noutput:\n%s", err, out)
		}
		got := string(out)
		if strings.Contains(got, "build tags") {
			t.Fatalf("bare `ask` still hits the Wails GUI path:\n%s", got)
		}
		if !strings.Contains(got, "Usage:") {
			t.Errorf("bare `ask` should print CLI help; got:\n%s", got)
		}
	})

	// `ask gui` in the CLI build must fail gracefully and guide the user to the
	// web UI, never leaking the raw Wails build-tags error.
	t.Run("gui subcommand guides to the web UI", func(t *testing.T) {
		out, err := exec.Command(bin, "gui").CombinedOutput()
		if err == nil {
			t.Error("expected `ask gui` to exit non-zero in the CLI build")
		}
		got := string(out)
		if strings.Contains(got, "build tags") {
			t.Fatalf("`ask gui` leaked the raw Wails error:\n%s", got)
		}
		if !strings.Contains(got, "ask serve") {
			t.Errorf("`ask gui` should suggest `ask serve`; got:\n%s", got)
		}
	})
}
