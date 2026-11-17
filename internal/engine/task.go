// Package engine runs a conversion: it decides which converter handles a
// pair of formats, keeps the task list and streams progress to the UI.
package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// State is where a task stands.
type State int

const (
	Wait State = iota
	Process
	Done
	Fail
	Stop
)

// Name is the state as the UI writes it.
func (s State) Name() string {
	switch s {
	case Process:
		return "process"
	case Done:
		return "done"
	case Fail:
		return "fail"
	case Stop:
		return "stop"
	}
	return "wait"
}

// MsgKind classes a log line the way the original UI colored it.
type MsgKind int

const (
	MsgLog MsgKind = iota
	MsgError
	MsgDone
	MsgFail
)

// Msg is one line of a task's log.
type Msg struct {
	Kind MsgKind
	Text string
}

// Task is one conversion.
type Task struct {
	ID       string
	Src      string
	SrcFmt   Format
	DstFmt   Format
	Dst      string
	SrcIsPDF bool
	Warn     bool

	Title    string
	State    State
	mu       sync.Mutex
	Percent  int
	Started  time.Time
	Elapsed  time.Duration
	Msgs     []Msg
	Engine   string
	proc     *childProc
	finished bool
	ctx      context.Context
	cancel   context.CancelFunc
}

// Log appends a line. It takes the task's lock, so the converters may call it
// from their own goroutines while the UI reads snapshots.
func (t *Task) Log(kind MsgKind, format string, args ...any) {
	t.mu.Lock()
	t.logLocked(kind, format, args...)
	t.mu.Unlock()
}

// logLocked appends a line to a task the caller already holds the lock of.
func (t *Task) logLocked(kind MsgKind, format string, args ...any) {
	t.Msgs = append(t.Msgs, Msg{kind, fmt.Sprintf(format, args...)})
}

// lastLineLocked is the newest log line; the caller holds the lock.
func (t *Task) lastLineLocked() string {
	if len(t.Msgs) == 0 {
		return ""
	}
	return t.Msgs[len(t.Msgs)-1].Text
}

// Progress is what the UI needs to draw a task, copied under the lock.
type Progress struct {
	ID      string
	Dst     string
	State   State
	Percent int
	Elapsed time.Duration
	Line    string
	Msgs    []Msg
}

// Snapshot copies the task's public fields.
func (t *Task) Snapshot() Progress {
	t.mu.Lock()
	defer t.mu.Unlock()
	msgs := make([]Msg, len(t.Msgs))
	copy(msgs, t.Msgs)
	return Progress{ID: t.ID, Dst: t.Dst, State: t.State, Percent: t.Percent, Elapsed: t.Elapsed, Line: t.lastLineLocked(), Msgs: msgs}
}

func (t *Task) context() context.Context {
	if t.ctx == nil {
		return context.Background()
	}
	return t.ctx
}

// Manager owns the task list and the three conversion slots the original app
// allowed to run at once.
const slots = 3

type Manager struct {
	mu     sync.Mutex
	tasks  []*Task
	ticker *time.Ticker
	stop   chan struct{}
	once   sync.Once
	notify func() // wakes the UI
	seq    atomic.Int64
}

// NewManager returns a manager; notify is called after any change.
func NewManager(notify func()) *Manager {
	m := &Manager{notify: notify, stop: make(chan struct{})}
	if notify == nil {
		m.notify = func() {}
	}
	go m.loop()
	return m
}

// Pump runs one scheduling pass: it starts queued tasks while a slot is
// free. The ticker calls it every 500 ms; tests call it directly.
func (m *Manager) Pump() { m.pump() }

// Close stops the ticker.
func (m *Manager) Close() { m.once.Do(func() { close(m.stop) }) }

// Tasks returns a copy of the list for drawing.
func (m *Manager) Tasks() []*Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Task, len(m.tasks))
	copy(out, m.tasks)
	return out
}

// Len is the number of tasks.
func (m *Manager) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.tasks)
}

// ClearDone drops the finished tasks.
func (m *Manager) ClearDone() {
	m.mu.Lock()
	kept := m.tasks[:0]
	for _, t := range m.tasks {
		t.mu.Lock()
		st, finished := t.State, t.finished
		t.mu.Unlock()
		if st == Done || st == Fail || (st == Stop && finished) {
			continue
		}
		kept = append(kept, t)
	}
	m.tasks = kept
	m.mu.Unlock()
	m.notify()
}

// Add queues conversions of paths to dst. It returns the tasks it made.
func (m *Manager) Add(paths []string, dst Format) []*Task {
	var made []*Task
	m.mu.Lock()
	for _, p := range paths {
		src := Parse(p)
		if src == "" {
			continue
		}
		name := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		// A title is the file's own name without the extension, not the
		// scratch copy's id.
		title := name
		dir := filepath.Dir(p)
		t := &Task{
			ID:     fmt.Sprintf("t%d", m.seq.Add(1)),
			Src:    p,
			SrcFmt: src,
			DstFmt: dst,
			Dst:    filepath.Join(dir, "(NEW)"+name+"."+string(dst)),
			Title:  title,
			Warn:   WarnQuality(src, dst),
			State:  Wait,
		}
		t.ctx, t.cancel = context.WithCancel(context.Background())
		if Builtin(src, dst) {
			t.Engine = "builtin"
		} else {
			t.Engine = "calibre"
			// The component folder is the original's; a machine that has not
			// installed it gets a clear failure instead of a spawn error.
			if !CalibreReady() {
				t.Engine = "calibre-missing"
			}
		}
		t.Log(MsgLog, "[log] %s → %s", src.Upper(), dst.Upper())
		m.tasks = append(m.tasks, t)
		made = append(made, t)
	}
	m.mu.Unlock()
	m.notify()
	return made
}

// Cancel stops a running task, or drops a waiting one.
func (m *Manager) Cancel(t *Task) {
	t.mu.Lock()
	var p *childProc
	if t.State == Wait || t.State == Process {
		t.finished = t.State == Wait
		t.State = Stop
		if t.cancel != nil {
			t.cancel()
		}
		p = t.proc
		t.logLocked(MsgLog, "[log] user stop this task")
	}
	t.mu.Unlock()
	if p != nil {
		p.Kill()
	}
	m.notify()
}

// loop ticks the running tasks and starts queued ones, three at a time, as
// the original polled its queue every 500 ms.
func (m *Manager) loop() {
	tk := time.NewTicker(500 * time.Millisecond)
	defer tk.Stop()
	for {
		select {
		case <-m.stop:
			return
		case <-tk.C:
			m.pump()
		case <-m.wake():
		}
	}
}

var wakeCh = make(chan struct{}, 1)

func (m *Manager) wake() <-chan struct{} { return wakeCh }

func (m *Manager) kick() {
	select {
	case wakeCh <- struct{}{}:
	default:
	}
}

func (m *Manager) pump() {
	m.mu.Lock()
	running := 0
	var next *Task
	for _, t := range m.tasks {
		t.mu.Lock()
		switch t.State {
		case Process:
			running++
			t.Elapsed += 500 * time.Millisecond
		case Stop:
			if !t.finished {
				running++
			}
		case Wait:
			if next == nil {
				next = t
			}
		}
		t.mu.Unlock()
	}
	var start *Task
	if running < slots && next != nil {
		next.mu.Lock()
		next.State = Process
		next.Started = time.Now()
		next.mu.Unlock()
		start = next
	}
	m.mu.Unlock()
	m.notify()
	if start != nil {
		go m.run(start)
	}
}

// run performs one task.
func (m *Manager) run(t *Task) {
	defer func() {
		t.mu.Lock()
		t.finished = true
		if t.cancel != nil {
			t.cancel()
		}
		t.mu.Unlock()
		m.notify()
	}()
	ctx := t.context()
	if ctx.Err() != nil {
		return
	}
	src, dst := t.Src, t.Dst
	root, err := TempDir()
	if err != nil {
		m.fail(t, "temp folder: %v", err)
		return
	}
	temp, err := os.MkdirTemp(root, "task-")
	if err != nil {
		m.fail(t, "temp folder: %v", err)
		return
	}
	defer os.RemoveAll(temp)
	work := filepath.Join(temp, t.Title+"."+string(t.SrcFmt))
	if err := copyFileContext(ctx, src, work); err != nil {
		m.fail(t, "001 - failed to copy input file: %v", err)
		return
	}
	t.Log(MsgLog, "[log] copy input file to temp folder")

	out := filepath.Join(temp, "new "+t.Title+"."+string(t.DstFmt))
	var convErr error
	report := func(p int) { m.SetPercent(t, p) }
	switch t.Engine {
	case "builtin":
		convErr = ConvertBuiltin(t, work, out, report)
	case "calibre-missing":
		convErr = fmt.Errorf("calibre is not installed yet")
	default:
		convErr = ConvertCalibre(t, work, out, report)
	}
	if convErr != nil {
		m.fail(t, "002 - %v", convErr)
		return
	}
	if ctx.Err() != nil {
		return
	}
	dst, err = copyUniqueOutput(ctx, out, dst)
	if err != nil {
		m.fail(t, "003 - fail to write output file: %v", err)
		return
	}
	t.mu.Lock()
	if t.State == Stop {
		t.mu.Unlock()
		os.Remove(dst)
		return
	}
	t.Dst = dst
	t.logLocked(MsgLog, "[log] copy output file from temp folder")
	t.State = Done
	t.Percent = 100
	t.Elapsed = time.Since(t.Started)
	t.logLocked(MsgDone, "[done] 0")
	t.mu.Unlock()
	m.notify()
}

func (m *Manager) fail(t *Task, format string, args ...any) {
	t.mu.Lock()
	if t.State == Stop {
		t.mu.Unlock()
		return
	}
	t.State = Fail
	t.logLocked(MsgFail, "[fail] "+format, args...)
	t.mu.Unlock()
	m.notify()
}

// SetPercent records progress from a converter.
func (m *Manager) SetPercent(t *Task, p int) {
	if p < 0 {
		p = 0
	}
	if p > 99 {
		p = 99
	}
	t.mu.Lock()
	if p > t.Percent {
		t.Percent = p
	}
	t.mu.Unlock()
	m.notify()
}
