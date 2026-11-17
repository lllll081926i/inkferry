package app

import (
	"strings"

	"github.com/egoist/mygo"

	"github.com/lllll081926i/inkferry/internal/engine"
)

// Filters is the picker's filter, one entry per format so the dialog shows
// "EPUB" and so on rather than a single long list.
func Filters(lang string) []mygo.FileFilter {
	var exts []string
	for _, f := range engine.Sources {
		exts = append(exts, string(f))
	}
	var out []mygo.FileFilter
	out = append(out, mygo.FileFilter{Name: label(lang, "电子书"), Extensions: exts})
	for _, f := range engine.Sources {
		out = append(out, mygo.FileFilter{Name: f.Upper(), Extensions: []string{string(f)}})
	}
	out = append(out, mygo.FileFilter{Name: label(lang, "所有文件"), Extensions: []string{"*"}})
	return out
}

// PickFiles opens the system file dialog and returns the chosen paths.
func PickFiles(lang string) ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Title:    label(lang, "选择文件"),
		Filters:  Filters(lang),
		Multiple: true,
	})
}

// label is the tiny lookup the picker needs before the view is built; the
// dialog runs on its own goroutine, where no ui.Context exists.
func label(lang, key string) string {
	if strings.HasPrefix(strings.ToLower(lang), "zh") {
		return zhLabel[key]
	}
	return enLabel[key]
}

var zhLabel = map[string]string{
	"电子书": "电子书", "选择文件": "选择电子书文件", "所有文件": "所有文件",
}

var enLabel = map[string]string{
	"电子书": "Ebooks", "选择文件": "Choose ebook files", "所有文件": "All files",
}
