package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	lpdf "github.com/ledongthuc/pdf"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/unicode"
)

// ConvertBuiltin converts without Calibre: text extraction from EPUB, MOBI,
// AZW3, AZW, PDF, TXT and DOCX, then writing TXT, EPUB, DOCX or PDF from what
// was extracted. It is the reason the app no longer needs a 60 MB download
// before its first conversion.
func ConvertBuiltin(t *Task, in, out string, report func(int)) error {
	if err := t.context().Err(); err != nil {
		return err
	}
	text, title, err := extractTextContext(t.context(), in, t.SrcFmt)
	if err != nil {
		return err
	}
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("no text could be read from the source file")
	}
	report(45)
	if err := t.context().Err(); err != nil {
		return err
	}
	switch t.DstFmt {
	case TXT:
		return writeText(out, text)
	case EPUB:
		return writeEPUB(t.context(), out, title, text)
	case DOCX:
		return writeDOCX(t.context(), out, title, text)
	case PDF:
		return writePDFContext(t.context(), out, title, text)
	}
	return fmt.Errorf("the built-in engine cannot write %s", t.DstFmt.Upper())
}

// extractText returns the readable text of any supported source.
func extractText(path string, f Format) (text, title string, err error) {
	return extractTextContext(context.Background(), path, f)
}

func extractTextContext(ctx context.Context, path string, f Format) (text, title string, err error) {
	if err := ctx.Err(); err != nil {
		return "", "", err
	}
	switch f {
	case TXT:
		b, err := readFileContext(ctx, path)
		if err != nil {
			return "", "", err
		}
		text, err := decodeText(b)
		return normalizeText(text), baseTitle(path), err
	case EPUB:
		return readEPUB(ctx, path)
	case MOBI, AZW3, AZW:
		return readMOBI(ctx, path)
	case PDF:
		return readPDF(ctx, path)
	case DOCX:
		return readDOCX(ctx, path)
	}
	return "", "", fmt.Errorf("unsupported source format %s", f)
}

// decodeText 支持 UTF-8、带 BOM 的 UTF-16 和 GB18030；拒绝有损解码。
func decodeText(b []byte) (string, error) {
	if bytes.HasPrefix(b, []byte{0xff, 0xfe}) || bytes.HasPrefix(b, []byte{0xfe, 0xff}) {
		if len(b)%2 != 0 {
			return "", fmt.Errorf("UTF-16 文本不完整，请检查源文件编码")
		}
		endian := unicode.LittleEndian
		if b[0] == 0xfe {
			endian = unicode.BigEndian
		}
		encoding := unicode.UTF16(endian, unicode.UseBOM)
		decoded, err := encoding.NewDecoder().Bytes(b)
		roundTrip, encodeErr := encoding.NewEncoder().Bytes(decoded)
		if err != nil || encodeErr != nil || !bytes.Equal(roundTrip, b) {
			return "", fmt.Errorf("UTF-16 文本包含无效字符，请检查源文件编码")
		}
		return string(decoded), nil
	}
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if utf8.Valid(b) {
		return string(b), nil
	}
	decoded, err := simplifiedchinese.GB18030.NewDecoder().Bytes(b)
	if bytes.Contains(decoded, []byte("\ufffd")) {
		roundTrip, encodeErr := simplifiedchinese.GB18030.NewEncoder().Bytes(decoded)
		if encodeErr != nil || !bytes.Equal(roundTrip, b) {
			return "", fmt.Errorf("无法识别文本编码，请将源文件保存为 UTF-8 后重试")
		}
	}
	if err != nil {
		return "", fmt.Errorf("无法识别文本编码，请将源文件保存为 UTF-8 后重试")
	}
	return string(decoded), nil
}

func baseTitle(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

func normalizeText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "\ufffd")
	}
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t\u00a0")
	}
	return strings.Join(lines, "\n")
}

var wsRun = regexp.MustCompile("[ \t\u00a0]+")

// htmlToText turns markup into paragraphs the writers can lay out.
func htmlToText(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	s := string(b)
	for _, tag := range []string{"script", "style", "head"} {
		re := regexp.MustCompile(`(?is)<` + tag + `(?:\s[^>]*)?>.*?</` + tag + `\s*>`)
		s = re.ReplaceAllString(s, "")
	}
	s = regexp.MustCompile(`<br\s*/?>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`</(p|div|h[1-6]|li|tr|blockquote)>`).ReplaceAllString(s, "\n\n")
	s = regexp.MustCompile(`<(h[1-6])[^>]*>`).ReplaceAllString(s, "\n\n")
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = normalizeText(s)
	// Collapse the runs of blank lines the tag replacement left.
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s) + "\n", nil
}

func readEPUB(ctx context.Context, path string) (string, string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", "", fmt.Errorf("not a readable EPUB: %w", err)
	}
	defer zr.Close()

	title := baseTitle(path)
	var order []string
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".opf") {
			if t := readOPFTitle(f); t != "" {
				title = t
			}
			order = append(order, opfSpine(f, zr.File)...)
			break
		}
	}
	var docs []string
	if len(order) > 0 {
		for _, name := range order {
			if f := findZip(zr.File, name); f != nil {
				docs = append(docs, f.Name)
			}
		}
	}
	if len(docs) == 0 {
		for _, f := range zr.File {
			n := strings.ToLower(f.Name)
			if strings.HasSuffix(n, ".xhtml") || strings.HasSuffix(n, ".html") || strings.HasSuffix(n, ".htm") {
				docs = append(docs, f.Name)
			}
		}
	}
	var sb strings.Builder
	for i, name := range docs {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		f := findZip(zr.File, name)
		if f == nil {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		text, err := htmlToText(contextReader{ctx, rc})
		rc.Close()
		if err != nil || strings.TrimSpace(text) == "" {
			continue
		}
		if i > 0 && sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(text)
	}
	return strings.TrimSpace(sb.String()) + "\n", title, nil
}

func findZip(files []*zip.File, name string) *zip.File {
	name = strings.TrimPrefix(name, "./")
	for _, f := range files {
		if f.Name == name || strings.EqualFold(f.Name, name) {
			return f
		}
	}
	base := filepath.Base(name)
	for _, f := range files {
		if strings.EqualFold(filepath.Base(f.Name), base) {
			return f
		}
	}
	return nil
}

func readOPFTitle(f *zip.File) string {
	rc, err := f.Open()
	if err != nil {
		return ""
	}
	defer rc.Close()
	b, err := io.ReadAll(io.LimitReader(rc, 1<<20))
	if err != nil {
		return ""
	}
	re := regexp.MustCompile(`<dc:title[^>]*>([^<]+)</dc:title>`)
	if m := re.FindSubmatch(b); len(m) == 2 {
		return strings.TrimSpace(html.UnescapeString(string(m[1])))
	}
	return ""
}

func opfSpine(f *zip.File, files []*zip.File) []string {
	rc, err := f.Open()
	if err != nil {
		return nil
	}
	b, err := io.ReadAll(io.LimitReader(rc, 4<<20))
	rc.Close()
	if err != nil {
		return nil
	}
	dir := filepath.ToSlash(filepath.Dir(f.Name))
	manifest := map[string]string{}
	reItem := regexp.MustCompile(`<item[^>]*id="([^"]+)"[^>]*href="([^"]+)"`)
	reItemAlt := regexp.MustCompile(`<item[^>]*href="([^"]+)"[^>]*id="([^"]+)"`)
	for _, m := range reItem.FindAllStringSubmatch(string(b), -1) {
		manifest[m[1]] = m[2]
	}
	for _, m := range reItemAlt.FindAllStringSubmatch(string(b), -1) {
		if _, ok := manifest[m[2]]; !ok {
			manifest[m[2]] = m[1]
		}
	}
	var order []string
	reRef := regexp.MustCompile(`<itemref[^>]*idref="([^"]+)"`)
	for _, m := range reRef.FindAllStringSubmatch(string(b), -1) {
		href, ok := manifest[m[1]]
		if !ok {
			continue
		}
		p := href
		if dir != "." && dir != "" && !strings.HasPrefix(href, "/") {
			p = dir + "/" + href
		}
		order = append(order, filepath.ToSlash(filepath.Clean(strings.TrimPrefix(p, "./"))))
	}
	return order
}

func readMOBI(ctx context.Context, path string) (string, string, error) {
	// MOBI and AZW3 are either raw PDB files or ZIP containers (AZW3 is
	// KF8, which may be zipped). Try the archive first, then the PDB.
	if zr, err := zip.OpenReader(path); err == nil {
		defer zr.Close()
		var sb strings.Builder
		title := baseTitle(path)
		for _, f := range zr.File {
			if err := ctx.Err(); err != nil {
				return "", "", err
			}
			n := strings.ToLower(f.Name)
			if !strings.HasSuffix(n, ".html") && !strings.HasSuffix(n, ".xhtml") && !strings.HasSuffix(n, ".htm") {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			text, _ := htmlToText(contextReader{ctx, rc})
			rc.Close()
			if strings.TrimSpace(text) != "" {
				sb.WriteString(text)
				sb.WriteString("\n\n")
			}
		}
		if sb.Len() > 0 {
			return strings.TrimSpace(sb.String()) + "\n", title, nil
		}
	}
	return readPDBContext(ctx, path)
}

func readPDF(ctx context.Context, path string) (string, string, error) {
	f, r, err := lpdf.Open(path)
	if err != nil {
		return "", "", fmt.Errorf("not a readable PDF: %w", err)
	}
	defer f.Close()
	var sb strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(normalizeText(text))
		sb.WriteString("\n\n")
	}
	title := ""
	if info := r.Trailer().Key("Info"); !info.IsNull() {
		if t := info.Key("Title"); !t.IsNull() {
			title = strings.TrimSpace(t.Text())
		}
	}
	if title == "" {
		title = baseTitle(path)
	}
	return strings.TrimSpace(sb.String()) + "\n", title, nil
}

func readDOCX(ctx context.Context, path string) (string, string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", "", fmt.Errorf("not a readable DOCX: %w", err)
	}
	defer zr.Close()
	var doc *zip.File
	for _, f := range zr.File {
		if strings.EqualFold(f.Name, "word/document.xml") {
			doc = f
			break
		}
	}
	if doc == nil {
		return "", "", fmt.Errorf("the DOCX has no document.xml")
	}
	rc, err := doc.Open()
	if err != nil {
		return "", "", err
	}
	defer rc.Close()
	text, err := docxXMLToText(contextReader{ctx, rc})
	if err != nil {
		return "", "", err
	}
	return text, baseTitle(path), nil
}

func docxXMLToText(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	s := string(b)
	s = regexp.MustCompile(`<w:p[ >]`).ReplaceAllString(s, "\n<w:p ")
	s = regexp.MustCompile(`<w:br[^>]*/>`).ReplaceAllString(s, "\n")
	s = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	s = normalizeText(s)
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s) + "\n", nil
}
