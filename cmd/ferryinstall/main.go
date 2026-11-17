// Command ferryinstall installs the optional Calibre bridge into InkFerry's
// data folder. It uses the same installer as the desktop app.
package main

import (
	"fmt"
	"os"

	"github.com/lllll081926i/inkferry/internal/engine"
)

func main() {
	last := -1
	err := engine.InstallCalibre(func(pct int, note string) {
		if pct != last {
			last = pct
			fmt.Fprintf(os.Stderr, "\r%d%% %s", pct, note)
		}
	})
	fmt.Fprintln(os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "install:", err)
		os.Exit(1)
	}
	fmt.Println("calibre ready:", engine.CalibreExe())
}
