package engine

import _ "embed"

// CJKFontData is Noto Sans SC cut down to the GB2312 set plus Latin, the
// symbols the UI uses and the punctuation a book needs, so both the built-in
// PDF writer and the interface can set Chinese text. It costs about 2 MB of
// the binary, where the alternative was a 189 MB Calibre install before the
// first conversion.
//
//go:embed font/cjk.ttf
var CJKFontData []byte

// CJKFont returns the embedded font's bytes.
func CJKFont() []byte { return CJKFontData }
