// Package assets carries the app's mark: the .ico Windows puts on the exe and
// the taskbar, and the PNG the window sets at startup.
package assets

import _ "embed"

// IconICO is the multi-size Windows icon (16, 20, 24, 32, 40, 48, 64, 128,
// 256 px) that the resource script embeds in the executable.
//
//go:embed app.ico
var IconICO []byte

// IconPNG is the 256 px mark the window sets with SetIcon. MyGo takes a PNG
// there on every platform, and Windows scales it for the title bar and the
// taskbar.
//
//go:embed icon.png
var IconPNG []byte
