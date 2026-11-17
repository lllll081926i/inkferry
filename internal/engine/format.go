package engine

import (
	"path/filepath"
	"strings"
)

// Format is an ebook file format the app converts to and from.
type Format string

const (
	EPUB Format = "epub"
	MOBI Format = "mobi"
	AZW3 Format = "azw3"
	AZW  Format = "azw"
	PDF  Format = "pdf"
	TXT  Format = "txt"
	DOCX Format = "docx"
)

// Targets are the formats offered as conversion targets, in the order the
// original app showed its six buttons.
var Targets = []Format{EPUB, AZW3, MOBI, PDF, DOCX, TXT}

// Sources are the formats the file picker accepts.
var Sources = []Format{EPUB, MOBI, AZW3, AZW, PDF, TXT, DOCX}

// Parse turns a file name or extension into a Format, "" when unknown.
func Parse(name string) Format {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), ".")
	switch ext {
	case "epub", "mobi", "azw3", "azw", "pdf", "txt", "docx":
		return Format(ext)
	}
	return ""
}

// Upper is the name a button shows.
func (f Format) Upper() string { return strings.ToUpper(string(f)) }

// Level ranks how much a format can hold, which is what decides how good a
// conversion between two of them can be: 1 is plain text, 3 is a rich
// container. It follows the explanation the original app showed its users.
func (f Format) Level() int {
	switch f {
	case TXT:
		return 1
	case PDF:
		return 2
	case AZW:
		return 2
	}
	return 3
}

// WarnQuality reports whether a pair is worth the warning the original app
// showed. Its rule, kept as it was: a text source into any other format, and
// PDF or DOCX into a reflowable one, are the pairs whose result turns poor.
func WarnQuality(src, dst Format) bool {
	if src == "" || dst == "" || src == dst {
		return false
	}
	if src == TXT {
		return true
	}
	if src == PDF || src == DOCX {
		return dst == EPUB || dst == AZW3 || dst == MOBI
	}
	return false
}

// Builtin reports whether the app's own engine handles this pair without
// Calibre: it reads every source it supports and writes TXT, EPUB, DOCX and
// PDF itself. MOBI and AZW3 output still need Calibre, because those are
// Palm database containers with per-record compression.
func Builtin(src, dst Format) bool {
	if src == "" || dst == "" {
		return false
	}
	if src == dst && (dst == TXT || dst == EPUB || dst == DOCX || dst == PDF) {
		return true
	}
	switch dst {
	case TXT, EPUB, DOCX, PDF:
		return true
	}
	return false
}
