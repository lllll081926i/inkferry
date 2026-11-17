package engine

import (
	"context"
	"encoding/binary"
	"fmt"
	"html"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	regexpTags  = regexp.MustCompile(`<[^>]+>`)
	regexpLines = regexp.MustCompile(`\n{3,}`)
)

func htmlUnescape(s string) string { return html.UnescapeString(s) }

// readPDB reads the text of a Palm database container (.mobi, .azw, .prc),
// record 0's MOBI/KF8 header telling how the text records are laid out.
func readPDB(path string) (string, string, error) {
	return readPDBContext(context.Background(), path)
}

func readPDBContext(ctx context.Context, path string) (string, string, error) {
	data, err := readFileContext(ctx, path)
	if err != nil {
		return "", "", err
	}
	if len(data) < 78 || string(data[60:68]) != "BOOKMOBI" {
		return "", "", fmt.Errorf("not a MOBI file: the header is missing")
	}
	n := int(binary.BigEndian.Uint16(data[76:78]))
	if n == 0 || len(data) < 78+n*8+2 {
		return "", "", fmt.Errorf("the MOBI record table is truncated")
	}
	offsets := make([]uint32, n+1)
	for i := 0; i < n; i++ {
		offsets[i] = binary.BigEndian.Uint32(data[78+i*8 : 78+i*8+4])
		if offsets[i] < uint32(78+n*8+2) || uint64(offsets[i]) > uint64(len(data)) || (i > 0 && offsets[i] < offsets[i-1]) {
			return "", "", fmt.Errorf("MOBI 记录 %d 的偏移无效", i)
		}
	}
	offsets[n] = uint32(len(data))

	// Record 0: PalmDOC header, then the MOBI header.
	r0 := data[offsets[0]:offsets[1]]
	if len(r0) < 16 {
		return "", "", fmt.Errorf("the MOBI record 0 is truncated")
	}
	compression := binary.BigEndian.Uint16(r0[0:2])
	textRecords := int(binary.BigEndian.Uint16(r0[8:10]))
	enc := binary.BigEndian.Uint16(r0[12:14])
	title := ""
	var encoding int
	var extraFlags uint16
	if len(r0) >= 8+16+4 && string(r0[16:20]) == "MOBI" {
		hdrLen := int(binary.BigEndian.Uint32(r0[20:24]))
		if hdrLen > len(r0)-16 {
			return "", "", fmt.Errorf("MOBI 标头不完整")
		}
		if hdrLen >= 0xE8 && len(r0) >= 16+hdrLen {
			// Full name offset and length are relative to record 0.
			nameOff := int(binary.BigEndian.Uint32(r0[84:88]))
			nameLen := int(binary.BigEndian.Uint32(r0[88:92]))
			if nameOff > 0 && nameLen > 0 && nameOff+nameLen <= len(r0) {
				t := r0[nameOff : nameOff+nameLen]
				title = strings.TrimSpace(decodeString(t, enc))
			}
			if hdrLen >= 0xC4 {
				extraFlags = binary.BigEndian.Uint16(r0[0xE2-0x10 : 0xE4-0x10])
			}
		}
	}

	var chunks []string
	remaining := textRecords
	if remaining > n-1 {
		return "", "", fmt.Errorf("MOBI 正文记录数量超出记录表")
	}
	for i := 1; i <= remaining; i++ {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
		start, end := offsets[i], offsets[i+1]
		if end > uint32(len(data)) || start > end {
			break
		}
		rec := data[start:end]
		switch compression {
		case 1:
			chunks = append(chunks, string(rec))
		case 2:
			decoded, derr := palmdocDecompress(rec)
			if derr != nil {
				return "", "", fmt.Errorf("MOBI 正文记录 %d: %w", i, derr)
			}
			chunks = append(chunks, string(decoded))
		case 17480:
			decoded, derr := huffDecompress(rec, data, offsets, i)
			if derr != nil {
				return "", "", derr
			}
			chunks = append(chunks, string(decoded))
		default:
			return "", "", fmt.Errorf("unsupported MOBI compression %d", compression)
		}
	}
	_ = extraFlags
	_ = encoding
	if title == "" {
		title = baseTitle(path)
	}
	text := strings.Join(chunks, "")
	text = mobiToText(text)
	if strings.TrimSpace(text) == "" {
		return "", "", fmt.Errorf("no text could be read from the MOBI records")
	}
	return text, title, nil
}

func decodeString(b []byte, enc uint16) string {
	if enc == 65001 {
		return string(b)
	}
	// Windows-1252: the bytes map directly except for the high range, which
	// MOBI text rarely uses in titles.
	out := make([]rune, 0, len(b))
	for _, c := range b {
		if c >= 0x80 && c < 0xA0 {
			out = append(out, '?')
			continue
		}
		out = append(out, rune(c))
	}
	return string(out)
}

// palmdocDecompress expands a PalmDOC-compressed record.
func palmdocDecompress(rec []byte) ([]byte, error) {
	var out []byte
	for i := 0; i < len(rec); {
		c := int(rec[i])
		i++
		if c >= 1 && c <= 8 {
			if i+c > len(rec) {
				return nil, fmt.Errorf("PalmDOC 字面量不完整")
			}
			out = append(out, rec[i:i+c]...)
			i += c
			continue
		}
		if c < 0x80 {
			out = append(out, byte(c))
			continue
		}
		if c >= 0xC0 {
			out = append(out, ' ', byte(c&0x7F))
			continue
		}
		if i >= len(rec) {
			return nil, fmt.Errorf("PalmDOC 回溯指令不完整")
		}
		d := int(rec[i])
		i++
		pair := c<<8 | d
		dist := (pair >> 3) & 0x7FF
		copyLen := (pair & 0x07) + 3
		start := len(out) - dist
		if dist == 0 || start < 0 {
			return nil, fmt.Errorf("PalmDOC 回溯距离无效: %d", dist)
		}
		for k := 0; k < copyLen; k++ {
			out = append(out, out[start+k])
		}
	}
	return out, nil
}

func huffDecompress(rec, data []byte, offsets []uint32, idx int) ([]byte, error) {
	// Huffman-compressed MOBI needs the HUFF record, which follows the text
	// records; it is rare, so the app reports it plainly.
	return nil, fmt.Errorf("HUFF-compressed MOBI is not supported by the built-in engine")
}

// mobiToText strips the KF8 markup from decoded records.
func mobiToText(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	for _, tag := range []string{"<mbp:pagebreak>", "<mbp:pagebreak/>"} {
		s = strings.ReplaceAll(s, tag, "\n\n")
	}
	// Paragraph and line boundaries carry the structure; strip them after
	// turning them into blank lines, not before.
	for _, tag := range []string{"</p>", "</P>", "</div>", "</DIV>", "</h1>", "</h2>"} {
		s = strings.ReplaceAll(s, tag, "\n\n")
	}
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	s = regexpTags.ReplaceAllString(s, "")
	s = htmlUnescape(s)
	// A MOBI's records end with a null byte.
	s = strings.ReplaceAll(s, "\x00", "")
	s = normalizeText(s)
	s = wsRun.ReplaceAllString(s, " ")
	s = regexpLines.ReplaceAllString(s, "\n\n")
	if !utf8.ValidString(s) {
		s = strings.ToValidUTF8(s, "")
	}
	return strings.TrimSpace(s) + "\n"
}
