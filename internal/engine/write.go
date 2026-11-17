package engine

import (
	"archive/zip"
	"bytes"
	"context"
	"html"
	"io"
	"os"
	"strconv"
)

// writeText saves the extracted text with a single trailing newline.
func writeText(path, text string) error {
	return os.WriteFile(path, []byte(text), 0o644)
}

// writeEPUB builds an EPUB 3 container from plain text, one paragraph per
// line, with a title page and a table of contents.
func writeEPUB(ctx context.Context, path, title, text string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)

	add := func(name string, body string, store bool) error {
		var w io.Writer
		var err error
		if store {
			w, err = zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store})
		} else {
			w, err = zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		}
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, body)
		return err
	}

	if err := add("mimetype", "application/epub+zip", true); err != nil {
		return err
	}
	if err := add("META-INF/container.xml", `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles>
</container>`, false); err != nil {
		return err
	}

	uid := "urn:uuid:neat-" + strconv.FormatInt(int64(len(text)), 10)
	opf := `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:title>` + html.EscapeString(title) + `</dc:title>
    <dc:language>` + langTag(text) + `</dc:language>
    <dc:identifier id="bookid">` + uid + `</dc:identifier>
    <meta property="dcterms:modified">1970-01-01T00:00:00Z</meta>
  </metadata>
  <manifest>
    <item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>
    <item id="chap" href="chapter.xhtml" media-type="application/xhtml+xml"/>
  </manifest>
  <spine><itemref idref="chap"/></spine>
</package>`
	if err := add("OEBPS/content.opf", opf, false); err != nil {
		return err
	}
	nav := `<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head><title>` + html.EscapeString(title) + `</title></head>
<body><nav epub:type="toc" id="toc"><h1>` + html.EscapeString(title) + `</h1>
<ul><li><a href="chapter.xhtml">` + html.EscapeString(title) + `</a></li></ul></nav></body></html>`
	if err := add("OEBPS/nav.xhtml", nav, false); err != nil {
		return err
	}

	var body bytes.Buffer
	body.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<html xmlns="http://www.w3.org/1999/xhtml">
<head><title>` + html.EscapeString(title) + `</title>
<style>body{font-family:serif;line-height:1.6;margin:1.4em} p{text-indent:2em;margin:0 0 .5em}</style>
</head>
<body>
`)
	for _, para := range splitParagraphs(text) {
		if err := ctx.Err(); err != nil {
			return err
		}
		body.WriteString("<p>" + html.EscapeString(para) + "</p>\n")
	}
	body.WriteString("</body></html>")
	if err := add("OEBPS/chapter.xhtml", body.String(), false); err != nil {
		return err
	}
	return zw.Close()
}

// writeDOCX builds a minimal Word document, one paragraph per line.
func writeDOCX(ctx context.Context, path, title, text string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	add := func(name, body string) error {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			return err
		}
		_, err = io.WriteString(w, body)
		return err
	}
	if err := add("[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`); err != nil {
		return err
	}
	if err := add("_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`); err != nil {
		return err
	}

	var doc bytes.Buffer
	doc.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>`)
	doc.WriteString(`<w:p><w:pPr><w:pStyle w:val="Title"/></w:pPr><w:r><w:rPr><w:b/><w:sz w:val="36"/></w:rPr><w:t xml:space="preserve">` +
		html.EscapeString(title) + `</w:t></w:r></w:p>`)
	for _, para := range splitParagraphs(text) {
		if err := ctx.Err(); err != nil {
			return err
		}
		doc.WriteString(`<w:p><w:r><w:t xml:space="preserve">` + html.EscapeString(para) + `</w:t></w:r></w:p>`)
	}
	doc.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr></w:body></w:document>`)
	if err := add("word/document.xml", doc.String()); err != nil {
		return err
	}
	return zw.Close()
}
