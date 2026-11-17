package engine

import (
	"context"
	"fmt"
	"strings"

	"github.com/signintech/gopdf"
)

// PDF writer geometry: A4 in points, the text block the app lays out.
const (
	pageW    = 595.28
	pageH    = 841.89
	marginX  = 56.7
	marginY  = 56.7
	fontSize = 11.0
	lineGap  = 3.2
)

// writePDF lays the text out on A4 pages. It embeds the CJK font the binary
// carries, so Chinese text renders instead of blank boxes.
func writePDF(path, title, text string) error {
	return writePDFContext(context.Background(), path, title, text)
}

func writePDFContext(ctx context.Context, path, title, text string) error {
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4, Unit: gopdf.UnitPT})
	if err := pdf.AddTTFFontData("body", CJKFont()); err != nil {
		return err
	}
	if err := pdf.SetFont("body", "", fontSize); err != nil {
		return err
	}
	pdf.AddPage()
	pdf.SetXY(marginX, marginY)
	paragraphs := append([]string{title}, splitParagraphs(text)...)
	for _, para := range paragraphs {
		if err := ctx.Err(); err != nil {
			return err
		}
		lines, err := wrapLine(para, pageW-2*marginX, func(line string) (float64, error) {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
			return pdf.MeasureTextWidth(line)
		})
		if err != nil {
			return err
		}
		for _, line := range lines {
			if err := ctx.Err(); err != nil {
				return err
			}
			if pdf.GetY()+fontSize+lineGap > pageH-marginY {
				pdf.AddPage()
				pdf.SetXY(marginX, marginY)
			}
			pdf.SetX(marginX)
			if err := pdf.Cell(nil, line); err != nil {
				return err
			}
			pdf.Br(fontSize + lineGap)
		}
		pdf.Br(lineGap)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return pdf.WritePdf(path)
}

// wrapLine 按实际字形宽度换行，并优先保留完整的英文单词。
func wrapLine(s string, width float64, measure func(string) (float64, error)) ([]string, error) {
	if width <= 0 {
		return nil, fmt.Errorf("PDF 正文宽度必须大于零")
	}
	runes := []rune(s)
	var out []string
	for len(runes) > 0 {
		lo, hi := 0, 1
		for {
			w, err := measure(string(runes[:hi]))
			if err != nil {
				return nil, err
			}
			if w > width {
				break
			}
			lo = hi
			if hi == len(runes) {
				break
			}
			hi = min(hi*2, len(runes))
		}
		for lo < hi {
			mid := lo + (hi-lo+1)/2
			w, err := measure(string(runes[:mid]))
			if err != nil {
				return nil, err
			}
			if w <= width {
				lo = mid
			} else {
				hi = mid - 1
			}
		}
		if lo == 0 {
			return nil, fmt.Errorf("PDF 字符宽度超出正文区域")
		}
		n := lo
		cut := n
		if n < len(runes) {
			for i := n - 1; i > n/2; i-- {
				if runes[i] == ' ' {
					cut = i
					break
				}
			}
		}
		if line := strings.TrimRight(string(runes[:cut]), " "); line != "" {
			out = append(out, line)
		}
		for cut < len(runes) && runes[cut] == ' ' {
			cut++
		}
		runes = runes[cut:]
	}
	return out, nil
}

// splitParagraphs divides text into blocks at blank lines.
func splitParagraphs(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	blocks := strings.Split(text, "\n\n")
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		b = strings.TrimSpace(strings.ReplaceAll(b, "\n", " "))
		if b != "" {
			out = append(out, b)
		}
	}
	if len(out) == 0 {
		if t := strings.TrimSpace(text); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// langTag guesses the document language from its script, for EPUB metadata.
func langTag(text string) string {
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			return "zh"
		}
	}
	return "en"
}
