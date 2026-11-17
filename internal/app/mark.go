package app

import (
	"bytes"
	"image"
	_ "image/png"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/lllll081926i/inkferry/assets"
)

var (
	markOnce sync.Once
	markImg  *ui.Bitmap
)

// mark is the app's logo in the header: the same artwork the exe carries,
// decoded once and reused by every frame.
func (s *State) mark(c *ui.Context) *ui.Bitmap {
	markOnce.Do(func() {
		img, _, err := image.Decode(bytes.NewReader(assets.IconPNG))
		if err != nil {
			return
		}
		markImg = ui.NewBitmap(img)
	})
	return markImg
}
