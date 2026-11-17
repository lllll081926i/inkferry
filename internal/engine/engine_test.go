package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func convert(t *testing.T, src string, dst Format) string {
	t.Helper()
	m := NewManager(nil)
	defer m.Close()
	text := "第一章 标题\n\n这是中文段落，包含标点：，。！\n\nAn English paragraph with numbers 123.\n"
	p := write(t, "book.txt", text)
	m.Add([]string{p}, dst)
	for i := 0; i < 200; i++ {
		st := m.Tasks()[0].Snapshot()
		switch st.State {
		case Done:
			return m.Tasks()[0].Dst
		case Fail, Stop:
			t.Fatalf("conversion failed: %v", st.Msgs)
		}
		m.Pump()
		waitDone(m)
	}
	t.Fatal("conversion timed out")
	return m.Tasks()[0].Dst
}

func TestTxtToEPUB(t *testing.T) {
	out := convert(t, "x", EPUB)
	if !strings.HasSuffix(out, "(NEW)book.epub") {
		t.Fatalf("output name %q", out)
	}
	zr, err := openZip(out)
	if err != nil {
		t.Fatal(err)
	}
	if !zr["mimetype"] || !zr["OEBPS/chapter.xhtml"] {
		t.Fatalf("epub entries: %v", zr)
	}
}

func TestTxtToDOCX(t *testing.T) {
	out := convert(t, "x", DOCX)
	zr, err := openZip(out)
	if err != nil {
		t.Fatal(err)
	}
	if !zr["word/document.xml"] {
		t.Fatalf("docx entries: %v", zr)
	}
}

func TestTxtToPDF(t *testing.T) {
	out := convert(t, "x", PDF)
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "%PDF") {
		t.Fatalf("not a pdf: %q", b[:min(20, len(b))])
	}
}

func TestWarnQuality(t *testing.T) {
	cases := []struct {
		src, dst Format
		warn     bool
	}{
		{EPUB, TXT, false},
		{PDF, TXT, false},
		{TXT, EPUB, true},
		{TXT, TXT, false},
		{EPUB, MOBI, false},
		{MOBI, AZW3, false},
		{PDF, EPUB, true},
		{PDF, TXT, false},
		{DOCX, MOBI, true},
		{DOCX, TXT, false},
		{MOBI, PDF, false},
	}
	for _, c := range cases {
		if got := WarnQuality(c.src, c.dst); got != c.warn {
			t.Errorf("%s→%s: warn=%v want %v", c.src, c.dst, got, c.warn)
		}
	}
}

func TestParse(t *testing.T) {
	if Parse("a.epub") != EPUB || Parse("b.MOBI") != MOBI || Parse("c.rtf") != "" {
		t.Fatal("parse")
	}
}

// TestMobiExtraction reads a MOBI the test writes in the same layout the
// reader expects: PalmDOC header, MOBI header, one text record.
func TestMobiExtraction(t *testing.T) {
	rec0 := buildTestMOBI([]byte("<p>第一章</p><p>body text</p>"))
	p := write(t, "gen.mobi", "")
	if err := os.WriteFile(p, rec0, 0o644); err != nil {
		t.Fatal(err)
	}
	text, _, err := readPDB(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "第一章") || !strings.Contains(text, "body text") {
		t.Fatalf("text = %q", text)
	}
}

func TestPDBRejectsGarbage(t *testing.T) {
	p := write(t, "bad.mobi", "this is not a mobi file at all")
	if _, _, err := readPDB(p); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestBuiltinCoversReaders(t *testing.T) {
	for _, src := range Sources {
		for _, dst := range []Format{TXT, EPUB, DOCX, PDF} {
			if !Builtin(src, dst) {
				t.Errorf("%s→%s should be builtin", src, dst)
			}
		}
		if Builtin(src, MOBI) {
			t.Errorf("%s→MOBI needs calibre", src)
		}
	}
}
