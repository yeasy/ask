package ui

import (
	"strings"
	"testing"
)

func TestNewSpinner(t *testing.T) {
	description := "Testing spinner..."
	spinner := NewSpinner(description)

	if spinner == nil {
		t.Fatal("NewSpinner returned nil")
	}

	// Verify spinner was created (can't easily test actual rendering)
	// Just verify it doesn't panic
	_ = spinner.Finish()
}

func TestNewProgressBar(t *testing.T) {
	total := 100
	description := "Testing progress"
	bar := NewProgressBar(total, description)

	if bar == nil {
		t.Fatal("NewProgressBar returned nil")
	}

	// Test updating progress
	for i := 0; i < 10; i++ {
		_ = bar.Add(1)
	}

	_ = bar.Finish()
}

func TestUpdateDescription(_ *testing.T) {
	bar := NewProgressBar(10, "Initial description")

	newDescription := "Updated description"
	UpdateDescription(bar, newDescription)

	// Verify it doesn't panic when updating description
	_ = bar.Add(1)
	_ = bar.Finish()
}

func TestProgressBarWithZeroTotal(t *testing.T) {
	// Test edge case with zero total
	bar := NewProgressBar(0, "Empty progress")

	if bar == nil {
		t.Fatal("NewProgressBar returned nil for zero total")
	}

	_ = bar.Finish()
}

func TestSpinnerWithEmptyDescription(t *testing.T) {
	// Test edge case with empty description
	spinner := NewSpinner("")

	if spinner == nil {
		t.Fatal("NewSpinner returned nil for empty description")
	}

	_ = spinner.Finish()
}

func TestProgressBarSequence(_ *testing.T) {
	// Test a realistic progress bar usage sequence
	total := 5
	bar := NewProgressBar(total, "Processing items")

	// Simulate processing
	for i := 0; i < total; i++ {
		UpdateDescription(bar, strings.Repeat("Processing item ", i+1))
		_ = bar.Add(1)
	}

	_ = bar.Finish()
}

func TestMultipleProgressBars(t *testing.T) {
	// Test creating multiple progress bars
	bar1 := NewProgressBar(10, "First task")
	bar2 := NewProgressBar(20, "Second task")

	if bar1 == nil || bar2 == nil {
		t.Fatal("Failed to create multiple progress bars")
	}

	// Update both
	_ = bar1.Add(5)
	_ = bar2.Add(10)

	_ = bar1.Finish()
	_ = bar2.Finish()
}

func TestSpinnerAndProgressBar(_ *testing.T) {
	// Test using both spinner and progress bar
	spinner := NewSpinner("Initializing...")
	_ = spinner.Add(1)
	_ = spinner.Finish()

	bar := NewProgressBar(3, "Processing")
	for i := 0; i < 3; i++ {
		_ = bar.Add(1)
	}
	_ = bar.Finish()
}

func TestSetupAllLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "error", "unknown_fallback"}
	for _, level := range levels {
		Setup(level)
		if Log == nil {
			t.Fatalf("Setup(%q) resulted in nil Log", level)
		}
	}
}

func TestSetupCaseInsensitive(t *testing.T) {
	// Verify that Setup handles mixed-case level strings
	for _, level := range []string{"DEBUG", "Info", "WARN", "Error"} {
		Setup(level)
		if Log == nil {
			t.Fatalf("Setup(%q) resulted in nil Log", level)
		}
	}
}

func TestDebugLogDoesNotPanic(t *testing.T) {
	Setup("debug")
	// Debug should not panic, even with extra args
	Debug("test debug message")
	Debug("test with args", "key", "value")
	if Log == nil {
		t.Fatal("Log should not be nil after Debug call")
	}
}

func TestInfoWarnErrorLogDoNotPanic(t *testing.T) {
	Setup("debug")
	Info("info message")
	Info("info with args", "count", 42)
	Warn("warn message")
	Warn("warn with args", "issue", "low disk")
	Error("error message")
	Error("error with args", "err", "connection refused")
	if Log == nil {
		t.Fatal("Log should not be nil after logging calls")
	}
}

func TestSetupDefaultFallback(t *testing.T) {
	// Any unrecognized level string should fall back to info without error
	Setup("nonexistent")
	if Log == nil {
		t.Fatal("Setup with unknown level should still create a logger")
	}
	Setup("")
	if Log == nil {
		t.Fatal("Setup with empty string should still create a logger")
	}
}
