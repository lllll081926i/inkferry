package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/lllll081926i/inkferry/internal/engine"
	"github.com/lllll081926i/inkferry/internal/i18n"
)

// TestBoardGeometry pins the layout rules the redesign asked for: the two
// wide-screen columns are equally tall, the drop zone absorbs the free
// height, and the task cards are thin enough that a short list never needs
// scrolling.
func TestBoardGeometry(t *testing.T) {
	s := newTestState()
	s.mgr = engine.NewManager(nil)
	s.mgr.Close()
	dir := t.TempDir()
	src := filepath.Join(dir, "book.txt")
	if err := os.WriteFile(src, []byte("用于验证两栏等高与任务行紧凑的真实正文。"), 0o644); err != nil {
		t.Fatal(err)
	}
	tasks := s.mgr.Add([]string{src}, engine.EPUB)
	s.mgr.Pump()
	s.mgr.Pump()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && tasks[0].Snapshot().State != engine.Done {
		time.Sleep(5 * time.Millisecond)
	}
	if tasks[0].Snapshot().State != engine.Done {
		t.Fatal("真实转换未完成，无法测量布局")
	}
	tt := ui.NewTester(s.View, 900, 640)
	tt.Frame()

	// The two columns start on the same line and end on the same line.
	drop, ok := tt.Access(s.Tr("拖放文件"))
	if !ok {
		t.Fatal("拖放区不可访问")
	}
	head, ok := tt.Access(s.Tr("转码任务列表"))
	if !ok {
		t.Fatal("任务列表标题不可访问")
	}
	// Text baselines sit a pixel or two apart inside equal line boxes; a
	// real misalignment would be much larger.
	if abs32(drop.Bounds.Y-head.Bounds.Y) > 3 {
		t.Errorf("两栏顶部未对齐: 拖放区 %+v, 标题 %+v", drop.Bounds, head.Bounds)
	}
	picker, ok := tt.Access(s.Tr("选择文件"))
	if !ok {
		t.Fatal("选择文件按钮不可访问")
	}
	if picker.Bounds.Y+picker.Bounds.H > 620 || picker.Bounds.Y+picker.Bounds.H < 595 {
		t.Errorf("选择文件按钮未贴在左栏底部: %+v", picker.Bounds)
	}

	// The drop zone absorbs the column's free height instead of leaving a
	// blank band above the picker.
	if drop.Bounds.H < 200 {
		t.Errorf("拖放区未随等高伸展（高 %.1f），左栏中段会留白", drop.Bounds.H)
	}

	// Both finished rows, their stats and the header fit the panel without
	// the list scrolling: the two-line cards are thin enough.
	second, ok := tt.Access(s.Tr("运行日志") + ": " + tasks[0].Title)
	if !ok || second.Offscreen || second.Bounds.Y+second.Bounds.H > 620 {
		t.Errorf("任务行过高，列表需要滚动才能放下单条任务: %+v", second)
	}
}

// TestCompactBoardKeepsDropZone checks the phone layout: the page is
// content-sized, so the drop zone stays at its minimum height and nothing
// stretches into a blank band.
func TestCompactBoardKeepsDropZone(t *testing.T) {
	s := newTestState()
	s.Lang = i18n.ZH
	tt := ui.NewTester(s.View, 390, 844)
	tt.Frame()
	drop, ok := tt.Access(s.Tr("拖放文件"))
	if !ok {
		t.Fatal("拖放区不可访问")
	}
	if drop.Bounds.H > 130 || drop.Bounds.H < 90 {
		t.Errorf("窄屏拖放区高度异常: %.1f", drop.Bounds.H)
	}
	if drop.Bounds.X < 0 || drop.Bounds.X+drop.Bounds.W > 391 {
		t.Errorf("窄屏拖放区横向溢出: %+v", drop.Bounds)
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
