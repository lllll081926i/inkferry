package app

import (
	"os"

	"github.com/egoist/mygo/ui"
)

// uiFamily is the family the interface asks text for. It stays empty so the
// system's own fonts serve: every Windows ships Microsoft YaHei and every
// macOS ships PingFang, both with the real bold face the header's headings
// need. A single-weight embedded face would have to be smeared to fake
// bold, which is worse than letting the platform pick.
var uiFamily string

// InstallFont registers a font file as the interface's family, for a machine
// whose system fonts carry no Chinese at all. An empty path leaves the
// system's fonts in charge.
func InstallFont(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if err := ui.RegisterFont(data, ""); err != nil {
		return ""
	}
	for _, f := range ui.FontFamilies() {
		if f.Registered {
			uiFamily = f.Name
			return uiFamily
		}
	}
	return ""
}
