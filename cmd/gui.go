package cmd

import (
	"github.com/spf13/cobra"
)

// guiCmd launches the native desktop interface. The actual implementation is
// build-tag gated: the real Wails-backed GUI is compiled only into desktop
// builds (see gui_desktop.go, built via `make build-desktop`). The standard
// CLI release uses the stub in gui_default.go, which points users at the web
// UI (`ask serve`). This keeps Wails out of the CLI binary entirely.
var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the desktop UI (desktop builds only; use `ask serve` for the web UI)",
	Long: "Launch the ask desktop interface in a native window.\n\n" +
		"The desktop UI is only available in desktop builds produced by " +
		"`make build-desktop`. In the standard CLI build, run `ask serve` to open " +
		"the same interface in your browser.",
	Run: func(_ *cobra.Command, _ []string) {
		startGUI()
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
}
