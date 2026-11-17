package engine

import (
	"archive/zip"
	"path/filepath"
	"testing"
	"time"
)

// openZip lists an archive's entry names, for the writers' tests.
func openZip(path string) (map[string]bool, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	out := map[string]bool{}
	for _, f := range zr.File {
		out[f.Name] = true
	}
	return out, nil
}

// waitDone lets a just-started task finish before the next check.
func waitDone(m *Manager) { time.Sleep(50 * time.Millisecond) }

// isolate points the scratch and component folders at a per-test directory,
// so a test never writes beside the real one. The app is portable: outside a
// component install it touches nothing but the temp folder.
func isolate(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("INKFERRY_TMP", filepath.Join(dir, "temp"))
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
}
