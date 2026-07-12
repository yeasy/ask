//go:build !desktop

package cmd

import (
	"fmt"
	"os"
)

// startGUI is the stub used by the standard CLI build, which is compiled
// without the Wails `desktop` build tag (and therefore without any Wails
// dependency). The native desktop window can't run here, so guide the user to
// the web UI, which works in every build.
func startGUI() {
	fmt.Fprintln(os.Stderr, "The desktop UI isn't available in this build of ask.")
	fmt.Fprintln(os.Stderr, "Run `ask serve` to open the interface in your browser,")
	fmt.Fprintln(os.Stderr, "or build the desktop app locally with `make build-desktop`")
	fmt.Fprintln(os.Stderr, "(requires the wails CLI and CGO).")
	os.Exit(1)
}
