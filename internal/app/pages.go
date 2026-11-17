package app

import (
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/lllll081926i/inkferry/internal/engine"
)

// preparePage shows the optional Calibre component installer used for
// formats that the built-in converter cannot emit.
func (s *State) preparePage(c *ui.Context, tk *theme.Tokens) {
	width, _ := c.Size()
	contentWidth := min(float32(440), width-48)
	progressWidth := min(float32(320), width-64)
	ui.Column(c).Fill().Center().Padding(24).Gap(16).Children(func() {
		ui.Box(c).Size(60, 60).Radius(30).Background(tk.InfoFillTertiary).Center().Children(func() {
			ui.Icon(c, lucide.Sparkles()).FontSize(27).TextColor(tk.Info)
		})
		ui.Text(c, s.Tr("检测组件")).FontSize(18).FontWeight(600).TextColor(tk.Text).TextAlign(ui.Center)
		ui.Text(c, s.Tr("组件说明")).FontSize(12).LineHeight(1.6).TextColor(tk.TextTertiary).MaxWidth(contentWidth).TextAlign(ui.Center)

		st := s.Installer()
		switch st.page {
		case PageDownloading, PageInstalling:
			bar := feedback.Progress(c, float32(st.pct), feedback.ProgressProps{Size: theme.Middle, Status: feedback.ProgressActive, Color: tk.Info})
			bar.Width(progressWidth)
			ui.Text(c, st.msg).FontSize(12).TextColor(tk.TextSecondary).MaxWidth(contentWidth).TextAlign(ui.Center)
		case PageFailed:
			feedback.Alert(c, s.Tr("请求失败"), feedback.AlertProps{Type: feedback.Error, Description: st.msg}).MaxWidth(contentWidth)
		case PageReady:
			display.Tag(c, s.Tr("组件安装完成"), display.TagProps{Color: display.TagSuccess, Variant: display.TagFilled, Size: theme.Middle})
		}
		ui.Row(c).Gap(10).Wrap().Justify(ui.Center).Children(func() {
			if action.Button(c, s.Tr("开始"), action.ButtonProps{Variant: action.Primary}).Clicked() {
				s.Route = RouteMain
			}
			if st.page == PageIdle || st.page == PageFailed {
				if action.Button(c, s.Tr("安装Calibre"), action.ButtonProps{Variant: action.Default}).Clicked() {
					go s.installCalibre()
				}
			}
		})
	})
}

// installCalibre downloads and unpacks the component archive in-process
// with a pure-Go 7z reader, so no external extractor is needed.
func (s *State) installCalibre() {
	// The label is looked up before the lock: Tr takes the same mutex, and
	// the callbacks run inside it.
	label := s.Tr("下载组件")
	s.mu.Lock()
	if s.install.page == PageDownloading || s.install.page == PageInstalling {
		s.mu.Unlock()
		return
	}
	s.install.page = PageDownloading
	s.mu.Unlock()
	s.setInstall(func(st *installState) {
		st.page = PageDownloading
		st.pct = 0
		st.msg = label + "0%"
	})
	err := engine.InstallCalibre(func(pct int, note string) {
		s.setInstall(func(st *installState) {
			st.pct = float64(pct)
			if note != "" {
				st.page = PageInstalling
				st.msg = note
				return
			}
			st.msg = label + strconv.Itoa(pct) + "%"
		})
	})
	if err != nil {
		s.setInstall(func(st *installState) {
			st.page = PageFailed
			st.msg = err.Error()
		})
		return
	}
	s.mu.Lock()
	s.Calibre = true
	s.mu.Unlock()
	s.setInstall(func(st *installState) { st.page = PageReady })
}

func (s *State) invalidate() {
	s.mu.Lock()
	ctx := s.ctx
	s.mu.Unlock()
	if ctx != nil {
		ctx.Invalidate()
	}
}

// updatePage is the original's forced-upgrade screen, kept because its
// strings are part of the app's behaviour; it only shows when the release
// endpoint says this build is too old.
func (s *State) updatePage(c *ui.Context, tk *theme.Tokens) {
	ui.Column(c).Fill().Center().Padding(24).Gap(10).Children(func() {
		feedback.Result(c, feedback.ResultProps{
			Status:   feedback.ResultWarning,
			Title:    s.Tr("版本过低标题"),
			SubTitle: s.Tr("版本过低内容"),
			Extra: func() {
				if action.Button(c, s.Tr("前往下载"), action.ButtonProps{Variant: action.Primary}).Clicked() {
					mygo.Shell.OpenExternal(releaseURL())
				}
			},
		})
		ui.Text(c, s.Tr("给您带来不便")).FontSize(12).TextColor(tk.TextTertiary)
	})
}

func dialogWidth(c *ui.Context, preferred float32) float32 {
	width, _ := c.Size()
	if width > 0 {
		return min(preferred, width-32)
	}
	return preferred
}

// modalAbout is the about dialog.
func modalAbout(c *ui.Context, s *State) {
	tk := theme.TokensOf(c)
	open := true
	if feedback.Modal(c, &open, feedback.ModalProps{Title: s.Tr("关于"), Width: dialogWidth(c, 440), OkText: "OK", NoFooter: false}, func() {
		ui.Column(c).Gap(12).Children(func() {
			ui.Text(c, VersionLabel()).FontSize(16).FontWeight(600).TextColor(tk.Text)
			ui.Text(c, s.Tr("关于说明")).FontSize(12).LineHeight(1.55).TextColor(tk.TextSecondary)
			layout.Divider(c, layout.DividerProps{})
			ui.Text(c, "MIT License").FontSize(12).TextColor(tk.TextTertiary)
			if action.Button(c, "github.com/lllll081926i/inkferry", action.ButtonProps{Variant: action.Link, Size: theme.Small}).AlignSelf(ui.Start).Clicked() {
				mygo.Shell.OpenExternal(projectURL())
			}
		})
	}) {
		s.About = false
	}
	if !open {
		s.About = false
	}
}

// modalError explains a failed conversion and shows the task's own log.
func modalError(c *ui.Context, s *State, t *engine.Task) bool {
	tk := theme.TokensOf(c)
	open := true
	ok := feedback.Modal(c, &open, feedback.ModalProps{Title: s.Tr("转码失败提示"), Width: dialogWidth(c, 560), OkText: "OK", CancelText: ""}, func() {
		p := progressOf(t)
		ui.Column(c).Gap(10).Children(func() {
			ui.Text(c, s.Tr("失败原因解释")).FontSize(12).LineHeight(1.5).TextColor(tk.TextSecondary)
			ui.Text(c, s.Tr("失败原因1")).FontSize(12).LineHeight(1.5).TextColor(tk.TextTertiary)
			ui.Text(c, s.Tr("失败原因2")).FontSize(12).LineHeight(1.5).TextColor(tk.TextTertiary)
			ui.Text(c, s.Tr("失败原因3")).FontSize(12).LineHeight(1.5).TextColor(tk.TextTertiary)
			ui.Text(c, s.Tr("失败后做法")).FontSize(12).LineHeight(1.5).TextColor(tk.TextTertiary)
			layout.Divider(c, layout.DividerProps{Label: s.Tr("错误信息输出")})
			ui.Column(c).Gap(3).Padding(10).Radius(8).Background(tk.FillQuaternary).Children(func() {
				for _, m := range p.Msgs {
					ui.Text(c, m.Text).FontSize(11).LineHeight(1.4).TextColor(msgColor(tk, m.Kind))
				}
			})
		})
	})
	if !open {
		return true
	}
	return ok
}

func projectURL() string {
	return "https://github.com/lllll081926i/inkferry"
}

func releaseURL() string {
	return projectURL() + "/releases/latest"
}
