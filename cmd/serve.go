package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/yeasy/ask/internal/server"
	"github.com/yeasy/ask/internal/ui"
)

var (
	servePort int
	noOpen    bool
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve [path]",
	Short: "Start the web UI server",
	Long: `Start a local web server that provides a modern UI for managing skills and repositories.

The web interface allows you to:
  - View installed skills and their details
  - Search and install new skills
  - Manage skill repositories
  - View configuration and statistics

If a path is provided, the server will start in that directory.
Otherwise, it defaults to the current directory.

Examples:
  ask serve              # Start server in current directory
  ask serve ./my-proj    # Start server in ./my-proj directory
  ask serve --port 3000  # Use custom port`,
	Args: cobra.MaximumNArgs(1),
	Run:  runServe,
}

func runServe(_ *cobra.Command, args []string) {
	// Handle project path if provided
	if len(args) > 0 {
		targetDir := args[0]
		info, err := os.Stat(targetDir)
		if err != nil {
			ui.Error("Failed to access project directory", "path", targetDir, "error", err)
			os.Exit(1)
		}
		if !info.IsDir() {
			ui.Error("Project path is not a directory", "path", targetDir)
			os.Exit(1)
		}

		if err := os.Chdir(targetDir); err != nil {
			ui.Error("Failed to change to project directory", "path", targetDir, "error", err)
			os.Exit(1)
		}
		fmt.Printf("📂 Working directory changed to: %s\n", targetDir)
	}

	srv := server.New(servePort, Version)

	// Setup graceful shutdown
	done := make(chan bool, 1)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		ui.Info("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Stop(ctx); err != nil {
			ui.Error("Server shutdown error", "error", err)
		}
		close(done)
	}()

	// Open browser if not disabled
	if !noOpen {
		go func() {
			// Wait a bit for server to start
			time.Sleep(500 * time.Millisecond)
			url := fmt.Sprintf("http://127.0.0.1:%d", servePort)
			if err := server.OpenBrowser(url); err != nil {
				ui.Debug("Failed to open browser", "error", err)
			}
		}()
	}

	fmt.Printf("\n🌐 ASK Web UI starting at http://127.0.0.1:%d\n", servePort)
	fmt.Println("   Press Ctrl+C to stop the server")

	if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		ui.Error("Server error", "error", err)
		os.Exit(1)
	}

	<-done
	fmt.Println("Server stopped.")
}

func init() {
	rootCmd.AddCommand(serveCmd)

	serveCmd.Flags().IntVarP(&servePort, "port", "p", 8125, "Port to run the server on")
	serveCmd.Flags().BoolVar(&noOpen, "no-open", false, "Don't open browser automatically")
}
