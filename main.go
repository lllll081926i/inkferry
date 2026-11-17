// Command inkferry is a small native ebook converter. MyGo draws the UI
// directly on the GPU while the Go engine reads and writes the supported
// ebook formats in-process.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lllll081926i/inkferry/assets"
	"github.com/lllll081926i/inkferry/internal/app"
	"github.com/lllll081926i/inkferry/internal/engine"
	"github.com/lllll081926i/inkferry/internal/i18n"
)

// version is replaced at build time with -ldflags "-X main.version=v1.0.0".
var version = "dev"

func main() {
	lang := flag.String("lang", "zh-cn", "UI language: zh-cn or en-us (default zh-cn)")
	font := flag.String("font", "", "font file for a system whose own fonts carry no Chinese")
	to := flag.String("to", "", "queue these files for FORMAT on start (epub, mobi, azw3, pdf, txt, docx)")
	flag.Parse()

	mygo.App.WhenReady(func() {
		// No menu bar: the app draws its own header, and a default menu
		// would put File/Edit/View above it.
		mygo.App.SetMenu(nil)

		// The interface uses the system's fonts, which carry Han glyphs and
		// real bold faces on every Windows and macOS install. -font is for
		// a machine that has none.
		if fam := app.InstallFont(*font); fam != "" {
			fmt.Fprintln(os.Stderr, "ui font:", fam)
		}

		app.Version = version
		state := app.NewState()
		state.DetectLang(*lang)
		i18n.Set(state.Lang)
		if f := engine.Parse("." + *to); *to != "" && f != "" {
			state.SetTarget(f)
		}
		state.SetCalibre(engine.CalibreReady())

		w := mygo.NewWindow(mygo.WindowOptions{
			Title:             app.AppName,
			Width:             900,
			Height:            640,
			MinWidth:          360,
			MinHeight:         600,
			TitleBarStyle:     mygo.TitleBarHidden,
			TitleBarHeight:    56,
			HideWindowButtons: true,
			BackgroundColor:   "light-dark(#f8f8f8, #1a1a1a)",
			Content:           ui.View(state.View),
		})
		state.SetWindow(w)
		// The mark the header draws, on the title bar and the taskbar.
		if err := w.SetIcon(assets.IconPNG); err != nil {
			fmt.Fprintln(os.Stderr, "icon:", err)
		}
		if *to != "" && flag.NArg() > 0 {
			// A command-line queue makes the app scriptable, and is how the
			// rewrite is checked without a mouse: the files join the board
			// on the first frame.
			state.QueueOnFirstFrame(flag.Args())
		}
	})

	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		log.Fatal(err)
	}
}
