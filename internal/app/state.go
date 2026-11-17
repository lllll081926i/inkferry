// Package app is the desktop front end: it draws the task board, the format
// buttons and the component installer with Lobe MyGo components, and drives
// the engine.
package app

import (
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/lobehub/lobe-mygo/pkg/feedback"

	"github.com/lllll081926i/inkferry/internal/engine"
	"github.com/lllll081926i/inkferry/internal/i18n"
)

// Route is one of the pages the original app opened as its own window.
type Route int

const (
	RouteInit Route = iota
	RoutePrepare
	RouteMain
	RouteUpdate
)

// AppName is the product name shown in the title bar and About dialog.
const AppName = "InkFerry"

// Version is replaced at link time for tagged builds.
var Version = "dev"

// VersionLabel is the release string shown in About.
func VersionLabel() string {
	if Version == "" {
		return AppName + " (Go)"
	}
	return AppName + " " + Version + " (Go)"
}

// Page collects what the view needs to know about the component installer.
type Page int

const (
	PageIdle Page = iota
	PageDownloading
	PageInstalling
	PageReady
	PageFailed
)

// WindowController is the part of the desktop window used by the custom
// title bar. Keeping it behind this small interface also lets UI tests run
// without creating a native window.
type WindowController interface {
	Minimize()
	ToggleMaximize()
	Close()
}

// State is the app's whole UI state; the view is a pure function of it.
type State struct {
	Route   Route
	Lang    i18n.Lang
	Target  engine.Format
	Page    Page
	Detail  bool         // an error panel is open
	DetailT *engine.Task // the task it explains
	About   bool         // the about modal is open
	Calibre bool         // calibre is ready
	ShowLog bool
	LogTask *engine.Task

	// install is the component installer's progress. The download runs on
	// its own goroutine, so the view reads a snapshot under mu rather than
	// the fields themselves.
	install installState

	pending      []string
	pendingToast *pendingNotice
	picking      bool
	mu           sync.Mutex
	mgr          *engine.Manager
	ctx          *ui.Context
	window       WindowController
}

// SetWindow connects the custom title bar to its native window.
func (s *State) SetWindow(w WindowController) {
	s.mu.Lock()
	s.window = w
	s.mu.Unlock()
}

func (s *State) windowController() WindowController {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.window
}

var _ WindowController = (*mygo.Window)(nil)

// installState is what the installer page draws.
type installState struct {
	page Page
	pct  float64
	msg  string
}

// Installer returns a copy of the installer's progress, safe to read from
// the view while the download runs.
func (s *State) Installer() installState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.install
}

// setInstall updates the installer's progress under the lock. fn runs while
// the mutex is held, so it must not call Tr or any other accessor that takes
// it: the download reports progress many times a second, and a nested lock
// would be a deadlock rather than a race.
func (s *State) setInstall(fn func(*installState)) {
	s.mu.Lock()
	fn(&s.install)
	s.mu.Unlock()
	s.invalidate()
}

// NewState builds the initial state.
func NewState() *State {
	return &State{Route: RouteInit, Lang: i18n.ZH, Target: engine.EPUB}
}

// Engine reports whether the optional Calibre component is installed.
func (s *State) Engine() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Calibre
}

// SetCalibre records whether the component was found at startup.
func (s *State) SetCalibre(v bool) {
	s.mu.Lock()
	s.Calibre = v
	s.mu.Unlock()
}

// Target returns the format the board converts to.
func (s *State) TargetFormat() engine.Format {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Target
}

// SetTarget picks the format the board converts to.
func (s *State) SetTarget(f engine.Format) {
	s.mu.Lock()
	s.Target = f
	s.mu.Unlock()
}

// Tr looks a message up in the current language.
func (s *State) Tr(key string) string { return i18n.Of(s.LangTag(), key) }

// Manager returns the task manager, creating it on first use.
func (s *State) Manager(c *ui.Context) *engine.Manager {
	if s.mgr != nil {
		return s.mgr
	}
	s.mu.Lock()
	s.ctx = c
	s.mu.Unlock()
	s.mgr = engine.NewManager(s.invalidate)
	return s.mgr
}

// QueueOnFirstFrame queues paths as soon as the window has a context, for
// the command line's -to flag and for a drop that arrives before the first
// frame.
func (s *State) QueueOnFirstFrame(paths []string) {
	s.mu.Lock()
	s.pending = append(s.pending, paths...)
	s.mu.Unlock()
}

// drainPending moves any queued files into the task list and shows the toast
// the picker asked for. It runs on the UI thread, in the frame that follows
// the dialog.
func (s *State) drainPending(c *ui.Context) {
	s.mu.Lock()
	paths := s.pending
	s.pending = nil
	note := s.pendingToast
	s.pendingToast = nil
	s.mu.Unlock()
	if len(paths) > 0 {
		s.AddPaths(c, paths)
	}
	if note != nil {
		t := feedback.ToastSuccess
		if note.kind == toastError {
			t = feedback.ToastError
		}
		feedback.ShowToast(c, feedback.ToastProps{Title: note.title, Description: note.body, Type: t})
	}
}

// Tasks returns the current list.
func (s *State) Tasks() []*engine.Task {
	if s.mgr == nil {
		return nil
	}
	return s.mgr.Tasks()
}

// StartQueue begins conversions of the chosen paths.
func (s *State) StartQueue(c *ui.Context, paths []string) {
	m := s.Manager(c)
	if added := m.Add(paths, s.TargetFormat()); len(added) > 0 {
		// Adding files shows the board and leaves the finished rows of
		// earlier files in place; the header's clear button drops them.
		s.Route = RouteMain
	}
}

// Stop cancels a task.
func (s *State) Stop(t *engine.Task) {
	if s.mgr != nil {
		s.mgr.Cancel(t)
	}
}

// ClearDone drops the finished rows.
func (s *State) ClearDone() {
	if s.mgr != nil {
		s.mgr.ClearDone()
	}
}

// DetectLang picks the language: "zh-cn" or "en-us" as given, or the
// system's when the value is not one of the two.
func (s *State) DetectLang(tag string) {
	s.mu.Lock()
	s.Lang = i18n.Detect(tag)
	s.mu.Unlock()
}

// SetLang changes the language from the header's toggle.
func (s *State) SetLang(l i18n.Lang) {
	s.mu.Lock()
	s.Lang = l
	s.mu.Unlock()
	i18n.Set(l)
}

// LangTag is the language as the picker and the site link want it.
func (s *State) LangTag() i18n.Lang {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Lang
}

// progressOf copies a task for drawing.
func progressOf(t *engine.Task) engine.Progress { return t.Snapshot() }

// stateLabel writes a task's state.
func stateLabel(s *State, st engine.State) string {
	switch st {
	case engine.Process:
		return s.Tr("转换中")
	case engine.Done:
		return s.Tr("已完成")
	case engine.Fail:
		return s.Tr("失败")
	case engine.Stop:
		return s.Tr("已终止")
	}
	return s.Tr("等待")
}
