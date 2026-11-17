package app

import (
	"github.com/egoist/mygo/ui"

	"github.com/lllll081926i/inkferry/internal/engine"
)

// pickAndStart opens the picker. The dialog blocks, so it runs on its own
// goroutine; the files it returns wait in pending until the next frame, when
// drainPending turns them into tasks on the UI thread. A ui.Context is valid
// only while its frame builds, so nothing is drawn from the goroutine.
func (s *State) pickAndStart(c *ui.Context) {
	s.mu.Lock()
	if s.picking {
		s.mu.Unlock()
		return
	}
	s.picking = true
	s.mu.Unlock()
	lang := s.LangTag()
	go func() {
		defer func() {
			s.mu.Lock()
			s.picking = false
			s.mu.Unlock()
			s.invalidate()
		}()
		paths, err := PickFiles(string(lang))
		if err != nil {
			s.pushToast(s.Tr("失败"), err.Error(), toastError)
			return
		}
		if len(paths) == 0 {
			return
		}
		s.QueueOnFirstFrame(paths)
	}()
}

// toastKind is the notification type a pending notice asks for.
type toastKind int

const (
	toastSuccess toastKind = iota
	toastError
)

// pendingNotice is a toast the next frame shows.
type pendingNotice struct {
	title, body string
	kind        toastKind
}

// pushToast asks the next frame to show a toast.
func (s *State) pushToast(title, body string, kind toastKind) {
	s.mu.Lock()
	s.pendingToast = &pendingNotice{title: title, body: body, kind: kind}
	s.mu.Unlock()
	s.invalidate()
}

// AddPaths is what a drop handler calls; it runs on the UI thread.
func (s *State) AddPaths(c *ui.Context, paths []string) {
	if len(paths) == 0 {
		return
	}
	ok := make([]string, 0, len(paths))
	for _, p := range paths {
		if engine.Parse(p) != "" {
			ok = append(ok, p)
		}
	}
	if len(ok) == 0 {
		s.pushToast(s.Tr("不支持的文件"), s.Tr("支持格式提示"), toastError)
		return
	}
	s.StartQueue(c, ok)
	if len(ok) < len(paths) {
		s.pushToast(s.Tr("已跳过不支持的文件"), s.Tr("支持格式提示"), toastError)
	}
}
