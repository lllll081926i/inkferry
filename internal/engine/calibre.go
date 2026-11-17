package engine

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// ConvertCalibre hands the pair to Calibre's ebook-convert, which is what the
// original app did for every conversion it could not do itself. Arguments go
// through exec rather than a shell string, so a path with spaces or quotes is
// passed as one argument and cannot inject anything.
func ConvertCalibre(t *Task, in, out string, report func(int)) error {
	exe := CalibreExecutable()
	if exe == "" {
		return errors.New("calibre is not installed yet")
	}
	cmd := exec.Command(exe, in, out)
	cmd.Dir = filepath.Dir(in)
	setChildEnv(cmd)
	p := &childProc{}
	t.setProc(p)
	defer t.setProc(nil)
	var lastErr strings.Builder
	err := runStream(cmd, p, func(line string) {
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			return
		}
		if strings.Contains(strings.ToLower(line), "error") {
			lastErr.WriteString(line + "\n")
		}
		// Calibre's progress lines start with the percentage.
		if f := strings.Fields(line); len(f) > 0 {
			if pct, convErr := strconv.Atoi(strings.TrimSuffix(f[0], "%")); convErr == nil && pct > 0 && pct <= 100 {
				report(pct)
			}
		}
		t.Log(MsgLog, "[log] %s", line)
	})
	if err != nil {
		if detail := strings.TrimSpace(lastErr.String()); detail != "" {
			return fmt.Errorf("%v: %s", err, firstLine(detail))
		}
		return err
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// setChildEnv gives the child the modules path and code page options the
// original set, so console output stays readable on Windows.
func setChildEnv(cmd *exec.Cmd) {
	env := os.Environ()
	if runtime.GOOS == "windows" {
		env = append(env, "PYTHONIOENCODING=utf-8", "PYTHONUTF8=1")
	}
	cmd.Env = env
}

func (t *Task) setProc(p *childProc) {
	t.mu.Lock()
	t.proc = p
	stopped := t.State == Stop
	t.mu.Unlock()
	if stopped && p != nil {
		p.Kill()
	}
}
