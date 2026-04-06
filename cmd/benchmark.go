// Package cmd provides the command line interface logic for ask.
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/config"
	"github.com/yeasy/ask/internal/github"
)

// benchmarkCmd represents the benchmark command
var benchmarkCmd = &cobra.Command{
	Use:   "benchmark",
	Short: "Run performance benchmarks",
	Long:  `Measure the performance of key CLI operations like search, list, and info.`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("Running benchmarks...")
		fmt.Println()

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "OPERATION\tTIME\tNOTES")

		// 1. Search (Cold) - Use a temporary cache directory for benchmarking
		tmpCacheDir, err := os.MkdirTemp("", "ask-bench-cache-*")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating temp cache: %v\n", err)
			return
		}
		defer func() { _ = os.RemoveAll(tmpCacheDir) }()

		start := time.Now()
		// We simulate search by calling the internal function directly to avoid printing to stdout
		// But since searchCmd prints to stdout, we might just want to measure the internal call
		// For a real benchmark, we should call the internal functions

		// Load config for search
		cfg, _ := config.LoadConfig()
		if cfg == nil {
			def := config.DefaultConfig()
			cfg = &def
		}

		// Mock search execution (Cold)
		// We'll search for "browser" which should trigger network requests
		if len(cfg.Repos) == 0 {
			fmt.Println("No repos configured. Skipping search benchmarks.")
			_ = w.Flush()
			fmt.Println()
			fmt.Println("Done.")
			return
		}
		repo := cfg.Repos[0]
		if repo.Type == "topic" {
			_, _ = github.SearchTopic(repo.URL, "browser")
		}
		duration := time.Since(start)
		_, _ = fmt.Fprintf(w, "Search (Cold)\t%v\tFirst repo only\n", duration.Round(time.Millisecond))

		// 2. Search (Hot) - Should be cached
		start = time.Now()
		if repo.Type == "topic" {
			_, _ = github.SearchTopic(repo.URL, "browser")
		}
		duration = time.Since(start)
		_, _ = fmt.Fprintf(w, "Search (Hot)\t%v\tCached\n", duration.Round(time.Millisecond))

		// 3. List - Local operation
		start = time.Now()
		// Simulate list parsing
		_, _ = config.LoadConfig()
		duration = time.Since(start)
		_, _ = fmt.Fprintf(w, "List\t%v\tConfig load\n", duration.Round(time.Millisecond))

		_ = w.Flush()
		fmt.Println()
		fmt.Println("Done.")
	},
}

func init() {
	rootCmd.AddCommand(benchmarkCmd)
}
