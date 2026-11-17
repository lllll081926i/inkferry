package engine

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bodgit/sevenzip"
)

// CalibreURL is the optional bridge archive for formats that the built-in
// engine cannot emit. Core EPUB, PDF, DOCX and TXT conversion works without
// downloading anything.
const (
	CalibreURL = "https://neat-reader-app-release.oss-cn-beijing.aliyuncs.com/calibre.7z"
	// CalibreSize is the archive's length, for the progress bar before
	// the server's headers arrive.
	CalibreSize = 62160119
)

// InstallCalibre downloads the archive and unpacks it into the component
// directory. It reports progress as a percentage and a note for the UI.
// Unpacking uses the pure-Go 7z reader: there is no 7za.exe to download and
// no shell command to run.
func InstallCalibre(report func(pct int, note string)) error {
	dir := CalibreDir()
	if dir == "" {
		return fmt.Errorf("no data directory")
	}
	if CalibreReady() {
		report(100, "")
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	archive := filepath.Join(dir, "calibre.7z")
	if err := download(CalibreURL, archive, report); err != nil {
		os.Remove(archive)
		return err
	}
	report(90, "解压完成后即可使用")
	if err := unpack7z(archive, dir, report); err != nil {
		os.Remove(archive)
		return err
	}
	os.Remove(archive)
	// The archive is only worth keeping if the converter it should hold
	// actually landed: a truncated download unpacks to nothing useful.
	if !CalibreReady() {
		return fmt.Errorf("组件解压后没有可运行的转换程序")
	}
	report(100, "")
	return nil
}

// download streams url into path, reporting the percentage it has of the
// server's content length, or of CalibreSize when the server sends none.
func download(url, path string, report func(int, string)) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// No overall deadline: the archive takes minutes on a slow line. The
	// transport still gives up on a server that never answers.
	client := &http.Client{Transport: &http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
	}}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}
	total := resp.ContentLength
	if total <= 0 {
		total = CalibreSize
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 256<<10)
	var done int64
	last := -1
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if pct := int(done * 90 / total); pct != last {
				last = pct
				report(pct, "")
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// unpack7z extracts every entry of archive into dir.
func unpack7z(archive, dir string, report func(int, string)) error {
	r, err := sevenzip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("解压失败: %w", err)
	}
	defer r.Close()
	total := len(r.File)
	for i, f := range r.File {
		name := strings.TrimPrefix(filepath.ToSlash(f.Name), "/")
		// The archive holds "calibre/..." and the component directory is
		// the calibre folder itself.
		name = strings.TrimPrefix(name, "calibre/")
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		dest := filepath.Join(dir, filepath.FromSlash(name))
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(dest, 0o755); err != nil {
				continue
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		out, err := os.Create(dest)
		if err != nil {
			rc.Close()
			continue
		}
		_, err = io.Copy(out, rc)
		out.Close()
		rc.Close()
		if err != nil {
			return err
		}
		if i%20 == 0 {
			report(90+int(float64(i)*10/float64(total)), "")
		}
	}
	return nil
}
