package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

// BaseDir is the folder the optional Calibre component is unpacked into, for
// example %APPDATA%\InkFerry on Windows. The app itself is portable: it never
// creates this folder until the user asks to install the component, and it
// writes no configuration, log or cache files anywhere.
func BaseDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "InkFerry"), nil
}

// TempDir is the scratch folder conversions copy their input to. It lives in
// the system's temporary directory, so a portable run leaves nothing behind
// outside it. INKFERRY_TMP overrides the location for tests.
func TempDir() (string, error) {
	root := os.Getenv("INKFERRY_TMP")
	if root == "" {
		root = filepath.Join(os.TempDir(), "InkFerry")
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	return root, nil
}

// CalibreDir holds the downloaded Calibre.
func CalibreDir() string {
	base, err := BaseDir()
	if err != nil {
		return ""
	}
	return filepath.Join(base, "cmdbin", "calibre")
}

// CalibreExe is the conventional path of the converter the app falls back to
// when its built-in engine cannot handle a pair.
func CalibreExe() string {
	dir := CalibreDir()
	if runtime.GOOS == "windows" {
		return filepath.Join(dir, "ebook-convert.exe")
	}
	return filepath.Join(dir, "ebook-convert")
}

// CalibreExecutable is the converter that is actually runnable here: the
// component the app installs is Calibre's Windows build, as the original's
// was, so only a Windows host can start it. Another host reports no converter
// and the app says so, rather than spawning a binary it cannot execute.
func CalibreExecutable() string {
	if runtime.GOOS != "windows" {
		for _, c := range []string{
			filepath.Join(CalibreDir(), "ebook-convert"),
			filepath.Join(CalibreDir(), "ebook-convert.exe"),
		} {
			if fi, err := os.Stat(c); err == nil && fi.Mode().IsRegular() && isRunnable(c) {
				return c
			}
		}
		return ""
	}
	for _, c := range []string{
		filepath.Join(CalibreDir(), "ebook-convert.exe"),
		filepath.Join(CalibreDir(), "ebook-convert"),
	} {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

// isRunnable reports whether the file carries the executable bit.
func isRunnable(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.Mode()&0o111 != 0
}

// CalibreReady reports whether the converter is installed.
func CalibreReady() bool { return CalibreExecutable() != "" }

// copyFile copies src to dst, keeping the permission bits.
func copyFile(src, dst string) error {
	return copyFileContext(context.Background(), src, dst)
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func readFileContext(ctx context.Context, path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(contextReader{ctx, f})
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func copyFileContext(ctx context.Context, src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	st, err := in.Stat()
	if err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, contextReader{ctx, in}); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(dst, st.Mode().Perm())
}

// copyUniqueOutput 排他创建目标文件，避免并发任务覆盖已有结果。
func copyUniqueOutput(ctx context.Context, src, dst string) (string, error) {
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	ext := filepath.Ext(dst)
	base := dst[:len(dst)-len(ext)]
	for n := 1; ; n++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		candidate := dst
		if n > 1 {
			candidate = fmt.Sprintf("%s (%d)%s", base, n, ext)
		}
		out, err := os.OpenFile(candidate, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if os.IsExist(err) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(out, contextReader{ctx, in})
		closeErr := out.Close()
		if copyErr == nil {
			copyErr = ctx.Err()
		}
		if copyErr != nil || closeErr != nil {
			os.Remove(candidate)
			if copyErr != nil {
				return "", copyErr
			}
			return "", closeErr
		}
		return candidate, nil
	}
}
