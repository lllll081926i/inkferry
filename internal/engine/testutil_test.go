package engine

import (
	"archive/zip"
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
