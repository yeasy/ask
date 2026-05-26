package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/yeasy/ask/internal/app"
	"github.com/yeasy/ask/internal/server"
	"github.com/yeasy/ask/internal/server/web"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the ask desktop interface",
	Long:  "Launch the ask desktop interface in a native window.",
	Run: func(_ *cobra.Command, _ []string) {
		startGUI()
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}

// ExecuteGUI starts the GUI application
func ExecuteGUI() {
	startGUI()
}

func startGUI() {
	// Create an instance of the app structure
	app := app.NewApp()

	// Create and configure server for API handling (port 0 as we only use the handler)
	srv := server.New(0, Version)

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "Ask",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets:  web.Assets,
			Handler: srv.Handler(),
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.Startup,
		Bind: []any{
			app,
		},
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting application: %v\n", err)
		os.Exit(1)
	}
}
