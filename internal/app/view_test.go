package app

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/lucide"

	"github.com/lllll081926i/inkferry/internal/engine"
	"github.com/lllll081926i/inkferry/internal/i18n"
)

// newTestState is a state with no manager, so a frame can be built without
// touching the disk.
func newTestState() *State {
	return &State{Route: RouteMain, Lang: i18n.ZH, Target: engine.EPUB}
}

type fakeWindow struct {
	minimized, maximized, closed int
}

func (w *fakeWindow) Minimize()       { w.minimized++ }
func (w *fakeWindow) ToggleMaximize() { w.maximized++ }
func (w *fakeWindow) Close()          { w.closed++ }

// textsOf collects everything the view drew as text, for assertions on the
// layout rather than on pixels.
func textsOf(t *testing.T, s *State) string {
	t.Helper()
	tt := ui.NewTester(s.View, 900, 640)
	tt.Frame()
	var sb strings.Builder
	for _, a := range tt.AccessAll() {
		if a.Name != "" {
			sb.WriteString(a.Name)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func TestMainViewDrawsChinese(t *testing.T) {
	s := newTestState()
	body := textsOf(t, s)
	for _, want := range []string{"InkFerry", "选择电子书文件", "转码任务列表", "输出格式"} {
		if !strings.Contains(body, want) {
			t.Errorf("view is missing %q:\n%s", want, body)
		}
	}
	// The board carries no explanatory copy: the welcome line, the format
	// hint, the output-location notes and the install note are all gone.
	for _, gone := range []string{"欢迎使用", "保存到原目录", "转码后", "添加后立即转换", "EPUB · MOBI", "需要 Calibre", "内置引擎", "Calibre", "安装 Calibre"} {
		if strings.Contains(body, gone) {
			t.Errorf("explanatory copy is still on the board (%q):\n%s", gone, body)
		}
	}
}

func TestSixFormatButtons(t *testing.T) {
	s := newTestState()
	body := textsOf(t, s)
	for _, f := range engine.Targets {
		if !strings.Contains(body, f.Upper()) {
			t.Errorf("missing the %s button:\n%s", f, body)
		}
	}
}

func TestHeaderUsesTheAppMark(t *testing.T) {
	// The mark is a bitmap: decoding it proves the PNG the exe carries is
	// valid, and the view falls back to a plain box without it.
	var c *ui.Context
	tt := ui.NewTester(func(ctx *ui.Context) {
		c = ctx
		s := newTestState()
		s.View(ctx)
	}, 900, 640)
	tt.Frame()
	if c == nil {
		t.Fatal("the view did not build")
	}
	if s := newTestState(); s.mark(c) == nil {
		t.Fatal("the header mark did not decode")
	}
}

func TestAboutModal(t *testing.T) {
	s := newTestState()
	s.About = true
	body := textsOf(t, s)
	for _, want := range []string{"关于", "InkFerry dev (Go)", "MIT License", "github.com/lllll081926i/inkferry"} {
		if !strings.Contains(body, want) {
			t.Errorf("about dialog is missing %q:\n%s", want, body)
		}
	}
	tt := ui.NewTester(s.View, 360, 640)
	tt.Frame()
	// 弹窗有入场动画：推进时间到动画结束再截图，避免截到不可见的起始帧。
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	for _, label := range []string{"关于", "OK", "InkFerry dev (Go)"} {
		a, ok := tt.Access(label)
		if !ok || a.Bounds.X < 0 || a.Bounds.X+a.Bounds.W > 361 {
			t.Errorf("窄屏关于弹窗横向溢出: %s: %+v", label, a)
		}
	}
	if dir := os.Getenv("NC_UI_SNAPSHOT_DIR"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "about-phone.png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}

func TestEmptyBoardOffersThePicker(t *testing.T) {
	s := newTestState()
	body := textsOf(t, s)
	if !strings.Contains(body, "选择电子书文件") {
		t.Fatalf("the empty board hides the picker:\n%s", body)
	}
}

func TestEnglishToggle(t *testing.T) {
	s := newTestState()
	s.Lang = i18n.EN
	body := textsOf(t, s)
	if !strings.Contains(body, "Choose ebook files") {
		t.Fatalf("the English view did not render:\n%s", body)
	}
}

func TestCustomWindowControls(t *testing.T) {
	for _, size := range [][2]int{{360, 640}, {900, 640}} {
		t.Run(fmt.Sprintf("%dx%d", size[0], size[1]), func(t *testing.T) {
			s := newTestState()
			w := &fakeWindow{}
			s.SetWindow(w)
			tt := ui.NewTester(s.View, size[0], size[1])
			tt.Frame()
			for _, label := range []string{s.Tr("最小化"), s.Tr("最大化或还原"), s.Tr("关闭窗口")} {
				a, ok := tt.Access(label)
				if !ok || a.Offscreen || a.Bounds.X < 0 || a.Bounds.X+a.Bounds.W > float32(size[0]) {
					t.Fatalf("窗口按钮超出窄屏顶栏: %s: %+v", label, a)
				}
			}
			if err := tt.Click(s.Tr("最小化")); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click(s.Tr("最大化或还原")); err != nil {
				t.Fatal(err)
			}
			tt.Frame()
			if err := tt.Click(s.Tr("关闭窗口")); err != nil {
				t.Fatal(err)
			}
			if w.minimized != 1 || w.maximized != 1 || w.closed != 1 {
				t.Fatalf("窗口操作未正确转发: %+v", w)
			}
		})
	}
}

func TestTargetButtonSelects(t *testing.T) {
	s := newTestState()
	tt := ui.NewTester(s.View, 900, 640)
	tt.Frame()
	// The AZW3 row is the second of the six; its label carries the key.
	body := textsOf(t, s)
	if !strings.Contains(body, "AZW3") {
		t.Fatal("AZW3 row missing")
	}
	if s.TargetFormat() != engine.EPUB {
		t.Fatalf("default target %v", s.TargetFormat())
	}
	if err := tt.Click("AZW3"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if s.TargetFormat() != engine.AZW3 {
		t.Fatal("点击目标格式未更新选择")
	}
	if a, ok := tt.Access("AZW3"); !ok || !a.Checked {
		t.Fatalf("目标格式未向辅助技术报告选中状态: %+v", a)
	}
}

func TestMainControlsFitWindow(t *testing.T) {
	for _, size := range [][2]int{{360, 640}, {390, 844}, {768, 1024}, {760, 560}, {900, 640}, {1200, 800}} {
		for _, lang := range []i18n.Lang{i18n.ZH, i18n.EN} {
			t.Run(fmt.Sprintf("%dx%d-%s", size[0], size[1], lang), func(t *testing.T) {
				s := newTestState()
				s.Lang = lang
				tt := ui.NewTester(s.View, size[0], size[1])
				tt.Frame()
				for _, label := range []string{"EPUB", "TXT", s.Tr("选择文件")} {
					a, ok := tt.Access(label)
					if !ok || a.Bounds.W <= 0 || a.Bounds.H <= 0 || a.Bounds.X < 0 || a.Bounds.X+a.Bounds.W > float32(size[0])+1 {
						t.Errorf("控件横向超出窗口: %s: %+v", label, a)
					}
					if size[1] >= 800 && (a.Offscreen || a.Bounds.Y+a.Bounds.H > float32(size[1])) {
						t.Errorf("控件纵向超出窗口: %s: %+v", label, a)
					}
				}
				if size[0] < 720 && size[1] < 800 {
					tt.Scroll(float32(size[0])/2, 180, 0, 600)
					tt.Frame()
					a, ok := tt.Access(s.Tr("选择文件"))
					if !ok || a.Offscreen || a.Bounds.Y < 0 || a.Bounds.Y+a.Bounds.H > float32(size[1]) {
						t.Errorf("窄屏滚动后仍无法操作选择文件按钮: %+v", a)
					}
				}
			})
		}
	}
}

func TestUnsupportedDropShowsFeedback(t *testing.T) {
	s := newTestState()
	tt := ui.NewTester(s.View, 900, 640)
	tt.Frame()
	a, ok := tt.Access(s.Tr("拖放文件"))
	if !ok {
		t.Fatal("拖放区不可访问")
	}
	if !tt.DropFiles(a.Bounds.X+a.Bounds.W/2, a.Bounds.Y+a.Bounds.H/2, "unsupported.jpg") {
		t.Fatal("拖放区未接受文件事件")
	}
	tt.Frame()
	if !tt.HasText(s.Tr("不支持的文件")) || len(s.Tasks()) != 0 {
		t.Fatal("不支持的文件未显示提示，或产生了无效任务")
	}
}

func TestPickingDisablesDuplicateDialog(t *testing.T) {
	s := newTestState()
	s.picking = true
	tt := ui.NewTester(s.View, 900, 640)
	tt.Frame()
	if a, ok := tt.Access(s.Tr("正在选择文件")); !ok || !a.Disabled {
		t.Fatalf("选择文件时未禁用重复打开: %+v", a)
	}
}

func TestTaskActionsAndSnapshots(t *testing.T) {
	s := newTestState()
	s.SetWindow(&fakeWindow{})
	s.mgr = engine.NewManager(nil)
	s.mgr.Close()
	dir := t.TempDir()
	src := filepath.Join(dir, "book.txt")
	broken := filepath.Join(dir, "broken.epub")
	if err := os.WriteFile(src, []byte("用于验证界面状态的真实转换正文。"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken, []byte("invalid epub"), 0o644); err != nil {
		t.Fatal(err)
	}
	tasks := s.mgr.Add([]string{src, broken}, engine.EPUB)
	s.mgr.Pump()
	s.mgr.Pump()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if tasks[0].Snapshot().State == engine.Done && tasks[1].Snapshot().State == engine.Fail {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if tasks[0].Snapshot().State != engine.Done || tasks[1].Snapshot().State != engine.Fail {
		t.Fatal("真实转换未产生预期状态")
	}
	tt := ui.NewTester(s.View, 900, 640)
	tt.Frame()
	if !tt.HasText("100%") || !tt.HasText(s.Tr("已保存")+"(NEW)book.epub") {
		t.Fatal("任务列表未展示进度或实际输出文件名")
	}
	if dir := os.Getenv("NC_UI_SNAPSHOT_DIR"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "tasks.png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
		phone := ui.NewTester(s.View, 390, 844)
		phone.Frame()
		phone.Scroll(195, 180, 0, 600)
		phone.Frame()
		for _, label := range []string{s.Tr("转码任务列表"), s.Tr("清空"), s.Tr("运行日志") + ": " + tasks[0].Title} {
			a, ok := phone.Access(label)
			if !ok || a.Bounds.X < 0 || a.Bounds.X+a.Bounds.W > 391 {
				t.Errorf("窄屏任务列表横向溢出: %s: %+v", label, a)
			}
		}
		f, err = os.Create(filepath.Join(dir, "tasks-phone.png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, phone.Image())
		closeErr = f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	// 在两个任务间直接切换日志，不能将第二个任务的日志误关闭；第一条
	// 日志展开后第二条可能进入滚动区，先滚动到它再点击。
	for i, task := range tasks {
		if i > 0 {
			tt.Scroll(600, 400, 0, 260)
			tt.Frame()
		}
		if err := tt.Click(s.Tr("运行日志") + ": " + task.Title); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if !s.ShowLog || s.LogTask != task {
			t.Fatal("切换任务后日志未保持展开")
		}
	}
	if err := tt.Click(s.Tr("清空")); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if len(s.Tasks()) != 0 {
		t.Fatal("已结束任务未清空")
	}
}

// 设置 NC_UI_SNAPSHOT_DIR 后导出真实界面帧，供人工核验。
func TestUISnapshots(t *testing.T) {
	dir := os.Getenv("NC_UI_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("未指定界面截图目录")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name          string
		width, height int
		dark          bool
		lang          i18n.Lang
	}{
		{"main", 900, 640, false, i18n.ZH},
		{"compact", 760, 560, false, i18n.ZH},
		{"phone", 390, 844, false, i18n.ZH},
		{"phone-english", 360, 640, false, i18n.EN},
		{"tablet", 768, 1024, false, i18n.ZH},
		{"dark", 900, 640, true, i18n.ZH},
		{"english", 760, 560, false, i18n.EN},
	} {
		s := newTestState()
		s.SetWindow(&fakeWindow{})
		s.Lang = tc.lang
		tt := ui.NewTesterWith(s.View, tc.width, tc.height, ui.TesterOptions{Dark: tc.dark})
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, tc.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}

func TestPickerResultQueuesOnNextFrame(t *testing.T) {
	s := newTestState()
	dir := t.TempDir()
	p := filepath.Join(dir, "book.txt")
	if err := os.WriteFile(p, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.QueueOnFirstFrame([]string{p})
	tt := ui.NewTester(s.View, 900, 640)
	tt.Frame()
	if got := len(s.Tasks()); got != 1 {
		t.Fatalf("queue produced %d tasks", got)
	}
	if s.Route != RouteMain {
		t.Fatalf("route %v", s.Route)
	}
}

func TestInstallPageShowsProgress(t *testing.T) {
	s := newTestState()
	s.Route = RoutePrepare
	s.setInstall(func(st *installState) { st.page = PageDownloading; st.pct = 42; st.msg = "下载组件 - 42%" })
	body := textsOf(t, s)
	for _, want := range []string{"正在检查可选组件", "下载组件 - 42%"} {
		if !strings.Contains(body, want) {
			t.Errorf("installer page is missing %q:\n%s", want, body)
		}
	}
	tt := ui.NewTester(s.View, 360, 640)
	tt.Frame()
	for _, label := range []string{s.Tr("开始"), "下载组件 - 42%"} {
		a, ok := tt.Access(label)
		if !ok || a.Bounds.X < 0 || a.Bounds.X+a.Bounds.W > 361 {
			t.Errorf("窄屏安装页横向溢出: %s: %+v", label, a)
		}
	}
	if dir := os.Getenv("NC_UI_SNAPSHOT_DIR"); dir != "" {
		f, err := os.Create(filepath.Join(dir, "prepare-phone.png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		closeErr := f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}

var _ = lucide.BookOpen
