// Command mkicon renders the app's mark at every size Windows asks for: a blue
// rounded plate holding an open book. The book is drawn as two quadrilaterals
// rather than traced from a thin-line icon glyph, because a 1.6 px stroke
// disappears once a title bar scales the icon to 16 px.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	out := os.Args[1]
	for _, n := range []int{16, 20, 24, 32, 40, 48, 64, 128, 256} {
		f, err := os.Create(fmt.Sprintf("%s/icon_%d.png", out, n))
		if err != nil {
			panic(err)
		}
		if err := png.Encode(f, Render(n)); err != nil {
			panic(err)
		}
		f.Close()
	}
	fmt.Println("ok")
}

const ss = 8 // supersampling factor

var (
	plate = color.NRGBA{0x1a, 0x93, 0xff, 0xff}
	page  = color.NRGBA{0xff, 0xff, 0xff, 0xff}
)

// pt is a vertex in the drawing's own coordinates, scaled to the canvas.
type pt struct{ x, y float64 }

// Render draws the mark at n pixels.
func Render(n int) image.Image {
	big := n * ss
	img := image.NewRGBA(image.Rect(0, 0, big, big))
	unit := float64(big) / 100

	// Plate.
	inset := 0.0
	if n <= 24 {
		inset = 1.0 // 1 px so the corner radius survives the scale-down
	}
	x0, y0 := inset*unit, inset*unit
	side := float64(big) - 2*x0
	fillRound(img, x0, y0, side, side, side*0.26, plate)

	// The book: two pages hinged at a gutter, the outer edges shorter than
	// the spine side, so the silhouette reads as an open book.
	gx, gy := 18.0, 27.0 // gutter's left edge, page tops
	gw, gh := 64.0, 46.0 // total width, height
	gutter := 3.2        // gap between the pages
	if n <= 24 {
		gutter = 4.0
	}
	cx := gx + gw/2
	// Each page is a quadrilateral: spine top, spine bottom, outer bottom,
	// outer top, with the outer edge inset a little at both ends.
	left := []pt{
		{cx - gutter/2, gy},
		{cx - gutter/2, gy + gh},
		{gx, gy + gh - 3},
		{gx, gy + 4},
	}
	right := []pt{
		{cx + gutter/2, gy},
		{cx + gutter/2, gy + gh},
		{gx + gw, gy + gh - 3},
		{gx + gw, gy + 4},
	}
	for _, poly := range [][]pt{left, right} {
		fillPoly(img, scalePoly(poly, unit), page)
	}

	return downsample(img, n)
}

func scalePoly(p []pt, unit float64) []pt {
	out := make([]pt, len(p))
	for i, v := range p {
		out[i] = pt{v.x * unit, v.y * unit}
	}
	return out
}

// fillPoly fills a polygon by the even-odd rule.
func fillPoly(img *image.RGBA, poly []pt, c color.NRGBA) {
	ys := make([]float64, 0, len(poly)*2)
	for _, v := range poly {
		ys = append(ys, v.y)
	}
	lo, hi := ys[0], ys[0]
	for _, y := range ys {
		lo, hi = min(lo, y), max(hi, y)
	}
	for py := int(lo); py <= int(hi)+1; py++ {
		fy := float64(py) + 0.5
		var xs []float64
		for i := range poly {
			a, b := poly[i], poly[(i+1)%len(poly)]
			if (a.y <= fy) == (b.y <= fy) {
				continue
			}
			t := (fy - a.y) / (b.y - a.y)
			xs = append(xs, a.x+t*(b.x-a.x))
		}
		if len(xs) < 2 {
			continue
		}
		// Sort the crossings and fill between each pair.
		for i := range xs {
			for j := i + 1; j < len(xs); j++ {
				if xs[j] < xs[i] {
					xs[i], xs[j] = xs[j], xs[i]
				}
			}
		}
		for i := 0; i+1 < len(xs); i += 2 {
			for px := int(xs[i]); px <= int(xs[i+1]); px++ {
				if px >= 0 && py >= 0 && px < img.Bounds().Dx() && py < img.Bounds().Dy() {
					img.Set(px, py, c)
				}
			}
		}
	}
}

func fillRound(img *image.RGBA, x, y, w, h, r float64, c color.NRGBA) {
	for py := int(y); py < int(y+h)+1; py++ {
		for px := int(x); px < int(x+w)+1; px++ {
			if px < 0 || py < 0 || px >= img.Bounds().Dx() || py >= img.Bounds().Dy() {
				continue
			}
			fx, fy := float64(px)+0.5, float64(py)+0.5
			if fx < x || fy < y || fx > x+w || fy > y+h {
				continue
			}
			cx := min(max(fx, x+r), x+w-r)
			cy := min(max(fy, y+r), y+h-r)
			if (fx-cx)*(fx-cx)+(fy-cy)*(fy-cy) <= r*r {
				img.Set(px, py, c)
			}
		}
	}
}

func downsample(img *image.RGBA, n int) image.Image {
	small := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var r, g, b, a uint32
			for dy := 0; dy < ss; dy++ {
				for dx := 0; dx < ss; dx++ {
					c := img.RGBAAt(x*ss+dx, y*ss+dy)
					w := uint32(c.A)
					r += uint32(c.R) * w / 255
					g += uint32(c.G) * w / 255
					b += uint32(c.B) * w / 255
					a += w
				}
			}
			cnt := uint32(ss * ss)
			if a == 0 {
				continue
			}
			avgA := a / cnt
			if avgA == 0 {
				continue
			}
			small.Set(x, y, color.NRGBA{
				R: uint8(min(255, r*255/a)),
				G: uint8(min(255, g*255/a)),
				B: uint8(min(255, b*255/a)),
				A: uint8(avgA),
			})
		}
	}
	return small
}
