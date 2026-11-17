package app

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/display"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/layout"
	"github.com/lobehub/lobe-mygo/pkg/lucide"
	"github.com/lobehub/lobe-mygo/pkg/motion"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/lllll081926i/inkferry/internal/engine"
	"github.com/lllll081926i/inkferry/internal/i18n"
)

const compactBreakpoint = float32(720)

var colorTransition = ui.ElementTransition{
	Duration: 160 * time.Millisecond,
	Ease:     motion.EaseOut,
	Colors:   true,
}

// pressFeedback gives custom buttons the same immediate, physical response
// as the component library's standard buttons. AnimatePose honors the
// operating system's reduced-motion preference.
func pressFeedback(e ui.Element) {
	scale := float32(1)
	if e.Pressed() {
		scale = 0.97
	}
	e.AnimatePose(ui.Pose{Scale: scale}, motion.Snappy(140*time.Millisecond))
}

// View is the window's content function.
func (s *State) View(c *ui.Context) {
	theme.Apply(c)
	s.drainPending(c)
	// Lobe MyGo's tokens carry no font family, so the registered face is
	// put on the theme here, once per frame.
	if uiFamily != "" {
		if th := c.Theme(); th != nil {
			font := uiFamily
			th.Font = font
			c.SetTheme(th)
		}
	}
	s.mu.Lock()
	s.ctx = c
	s.mu.Unlock()
	tk := theme.TokensOf(c)
	col := ui.Column(c).Fill().Background(tk.BgLayout)
	col.Children(func() {
		s.header(c, tk)
		layout.Divider(c, layout.DividerProps{})
		switch s.Route {
		case RoutePrepare:
			s.preparePage(c, tk)
		case RouteUpdate:
			s.updatePage(c, tk)
		default:
			s.mainPage(c, tk)
		}
		feedback.ToastHost(c, feedback.ToastHostProps{})
		if s.About {
			modalAbout(c, s)
		}
		if s.Detail && s.DetailT != nil {
			m := s.DetailT
			if modalError(c, s, m) {
				s.Detail = false
				s.DetailT = nil
			}
		}
	})
}

// header is the draggable custom title bar. Its right side keeps the
// existing language/about actions and replaces the native window buttons.
func (s *State) header(c *ui.Context, tk *theme.Tokens) {
	width, _ := c.Size()
	gutter := float32(20)
	if width < compactBreakpoint {
		gutter = 12
	}
	bar := ui.Row(c).Height(56).Padding(0, gutter).Gap(10).AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Background(tk.BgContainer).DragWindow()
	bar.Children(func() {
		ui.Row(c).Gap(9).AlignItems(ui.Center).MinWidth(0).Children(func() {
			if m := s.mark(c); m != nil {
				ui.Image(c, m).Size(26, 26).Radius(7).Shrink(0)
			} else {
				ui.Box(c).Size(26, 26).Radius(7).Background(tk.Info).Shrink(0)
			}
			if width >= 480 {
				ui.Text(c, AppName).FontSize(15).FontWeight(600).TextColor(tk.Text).SingleLine().Shrink(0)
			}
		})
		ui.Row(c).Gap(4).AlignItems(ui.Center).Shrink(0).Children(func() {
			if s.LangTag() == i18n.ZH {
				if action.Button(c, "EN", action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
					s.SetLang(i18n.EN)
				}
			} else {
				if action.Button(c, "中文", action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
					s.SetLang(i18n.ZH)
				}
			}
			if action.ActionIcon(c, lucide.Info(), action.ActionIconProps{BlockSize: 30, GlyphSize: 16, Title: s.Tr("关于")}).Clicked() {
				s.About = true
			}
			if w := s.windowController(); w != nil {
				ui.Box(c).Width(1).Height(16).MarginX(4).Background(tk.BorderSecondary).Shrink(0)
				windowButton(c, s.Tr("最小化"), lucide.Minus(), false, w.Minimize)
				windowButton(c, s.Tr("最大化或还原"), lucide.Square(), false, w.ToggleMaximize)
				windowButton(c, s.Tr("关闭窗口"), lucide.X(), true, w.Close)
			}
		})
	})
}

func windowButton(c *ui.Context, label string, icon *ui.SVG, danger bool, clicked func()) {
	b := action.ActionIcon(c, icon, action.ActionIconProps{BlockSize: 30, GlyphSize: 15, Danger: danger, Title: label}).Shrink(0)
	if b.Clicked() {
		clicked()
	}
}

// mainPage keeps the wide two-column workflow and stacks it on narrow
// screens. In the wide layout both cards stretch to the same height. The
// compact page scrolls as one surface so every control remains reachable
// even on a short phone-sized window.
func (s *State) mainPage(c *ui.Context, tk *theme.Tokens) {
	width, _ := c.Size()
	if width < compactBreakpoint {
		ui.Scroll(c.Key("compact-page")).Grow(1).Basis(0).MinHeight(0).Children(func() {
			ui.Column(c).Padding(16).Gap(14).AlignItems(ui.Stretch).Children(func() {
				s.heroCard(c, tk, 0)
				s.taskColumn(c, tk, true)
			})
		})
		return
	}
	ui.Row(c).Grow(1).Basis(0).MinHeight(0).Padding(20).Gap(20).AlignItems(ui.Stretch).Children(func() {
		s.heroCard(c, tk, 296)
		s.taskColumn(c, tk, false)
	})
}

// heroCard holds only the three conversion controls: the drop zone, the
// target formats and the picker. width > 0 fixes the card's column width.
func (s *State) heroCard(c *ui.Context, tk *theme.Tokens, width float32) {
	card := layout.Block(c, layout.BlockProps{Variant: layout.BlockOutlined, Shadow: true}).Radius(14).Padding(16).Gap(12).AlignSelf(ui.Stretch)
	if width > 0 {
		card.Width(width).Shrink(0)
	}
	card.Children(func() {
		s.dropZone(c, tk)
		ui.Text(c, s.Tr("输出格式")).FontSize(12).FontWeight(600).TextColor(tk.TextSecondary)
		ui.Column(c).Gap(8).AlignSelf(ui.Stretch).Children(func() {
			for row := 0; row < 2; row++ {
				ui.Row(c).Gap(8).AlignItems(ui.Stretch).Children(func() {
					for _, f := range engine.Targets[row*3 : row*3+3] {
						s.targetButton(c, tk, f)
					}
				})
			}
		})
		s.mu.Lock()
		picking := s.picking
		s.mu.Unlock()
		pickLabel := s.Tr("选择文件")
		if picking {
			pickLabel = s.Tr("正在选择文件")
		}
		if action.Button(c, pickLabel, action.ButtonProps{Variant: action.Primary, Size: theme.Large}).Disabled(picking).AlignSelf(ui.Stretch).Clicked() {
			s.pickAndStart(c)
		}
	})
}

// dropZone accepts files dropped on the window, the fastest path from a
// folder to a task.
func (s *State) dropZone(c *ui.Context, tk *theme.Tokens) {
	z := ui.ButtonBase(c.Key("drop")).Column().AlignItems(ui.Stretch).Justify(ui.Center).Gap(8).
		Grow(1).MinHeight(96).Padding(10).Radius(10).Border(1, tk.Border).BorderStyle(ui.BorderDashed).
		Background(tk.FillQuaternary).Label(s.Tr("拖放文件")).Cursor(ui.CursorPointer)
	if z.FileDragOver() || z.Hovered() {
		z.Background(tk.InfoFillTertiary).BorderColor(tk.InfoBorder)
	}
	z.Transition(colorTransition)
	pressFeedback(z)
	z.Children(func() {
		ui.Row(c).Center().Children(func() {
			lift := float32(0)
			if z.FileDragOver() || z.Hovered() {
				lift = -3
			}
			ui.Icon(c, lucide.FileUp()).FontSize(22).TextColor(ifColor(z.FileDragOver(), tk.Info, tk.TextTertiary)).
				AnimatePose(ui.Pose{Y: lift}, motion.Smooth(180*time.Millisecond))
		})
		ui.Text(c, s.Tr("拖放文件")).FontSize(12).FontWeight(500).LineHeight(1.4).TextColor(tk.TextSecondary).TextAlign(ui.Center)
	})
	if files := z.DroppedFiles(); len(files) > 0 {
		s.AddPaths(c, files)
	}
	if z.Clicked() {
		s.pickAndStart(c)
	}
}

// targetButton is one of the six "任意格式 → X" rows.
func (s *State) targetButton(c *ui.Context, tk *theme.Tokens, f engine.Format) {
	active := s.TargetFormat() == f
	b := ui.ButtonBase(c.Key("to-" + string(f))).Row().Grow(1).Basis(0).MinWidth(0).Height(40).Gap(4).AlignItems(ui.Center).Justify(ui.Center).
		Padding(6).Radius(8).Cursor(ui.CursorPointer).Label(f.Upper()).Checked(active).Background(tk.BgContainer)
	border := tk.BorderSecondary
	if active {
		b.Background(tk.InfoFillTertiary)
		border = tk.InfoBorder
	} else if b.Hovered() {
		b.Background(tk.InfoFillQuaternary)
		border = tk.InfoBorder
	}
	b.Border(1, border).Transition(colorTransition)
	pressFeedback(b)
	b.Children(func() {
		ui.Text(c, f.Upper()).FontSize(12).FontWeight(600).TextColor(ifColor(active, tk.Info, tk.Text))
		if active {
			ui.Icon(c, lucide.Check()).FontSize(13).TextColor(tk.Info).Shrink(0)
		}
	})
	if b.Clicked() {
		s.SetTarget(f)
	}
}

func ifColor(cond bool, a, b ui.Color) ui.Color {
	if cond {
		return a
	}
	return b
}

// taskColumn groups the summary chips and the task cards on one surface.
// The compact layout gives it a bounded height inside the page's scroll.
func (s *State) taskColumn(c *ui.Context, tk *theme.Tokens, compact bool) {
	tasks := s.Tasks()
	counts := map[engine.State]int{}
	for _, t := range tasks {
		counts[t.Snapshot().State]++
	}
	panel := layout.Block(c, layout.BlockProps{Variant: layout.BlockOutlined, Shadow: !compact}).Radius(14).Padding(16).Gap(12).AlignSelf(ui.Stretch).MinWidth(0)
	if compact {
		panel.Height(360)
	} else {
		panel.Grow(1).Basis(0).MinHeight(0)
	}
	panel.Children(func() {
		ui.Row(c).Justify(ui.SpaceBetween).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).MinWidth(0).Children(func() {
				ui.Text(c, s.Tr("转码任务列表")).FontSize(15).FontWeight(600).TextColor(tk.Text).SingleLine()
				ui.Row(c).Height(20).MinWidth(22).PaddingX(6).Radius(6).Background(tk.FillTertiary).Center().Children(func() {
					ui.Text(c, strconv.Itoa(len(tasks))).FontSize(11).FontWeight(500).FontFeatures("tnum").TextColor(tk.TextSecondary)
				})
			})
			if len(tasks) > 0 {
				clearable := counts[engine.Done]+counts[engine.Fail]+counts[engine.Stop] > 0
				if action.Button(c, s.Tr("清空"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Disabled(!clearable).Clicked() {
					s.ClearDone()
				}
			}
		})
		if len(tasks) > 0 {
			ui.Row(c).Gap(8).Children(func() {
				for _, st := range []engine.State{engine.Process, engine.Wait, engine.Done, engine.Fail} {
					ui.Row(c).Grow(1).Basis(0).MinWidth(0).Gap(6).Padding(4, 8).Radius(8).
						Background(tk.BgContainerSecondary).Border(1, tk.BorderSecondary).
						AlignItems(ui.Center).Justify(ui.Center).Children(func() {
						ui.Text(c, strconv.Itoa(counts[st])).FontSize(14).FontWeight(600).FontFeatures("tnum").TextColor(stateColor(tk, st)).Shrink(0)
						ui.Text(c, stateLabel(s, st)).FontSize(11).TextColor(tk.TextTertiary).Shrink(0)
					})
				}
			})
		}
		if len(tasks) == 0 {
			ui.Column(c).Grow(1).Basis(0).MinHeight(0).Center().Transition(motion.Collapse()).Children(func() {
				display.Empty(c, display.EmptyProps{
					Icon:  lucide.FileUp(),
					Title: s.Tr("等待添加文件"),
				})
			})
			return
		}
		// The indicators stay hidden: a task list rarely needs them, and a
		// visible bar over short content reads as a bug.
		ui.Scroll(c.Key("tasks")).Grow(1).Basis(0).MinHeight(0).Scrollbars(false).Children(func() {
			ui.Column(c).Gap(8).AlignSelf(ui.Stretch).Children(func() {
				for _, t := range tasks {
					s.taskRow(c.Key("task-"+t.ID), tk, t)
				}
			})
		})
	})
}

// taskRow is a compact two-line card: formats, name and state on the first
// line, progress or result and the row's actions on the second.
func (s *State) taskRow(c *ui.Context, tk *theme.Tokens, t *engine.Task) {
	p := progressOf(t)
	block := layout.Block(c, layout.BlockProps{Variant: layout.BlockOutlined}).Radius(10).Padding(9, 12).Gap(7).AlignSelf(ui.Stretch).
		Background(tk.BgContainerSecondary).Transition(motion.Collapse())
	block.Children(func() {
		ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
			display.Tag(c, t.SrcFmt.Upper(), display.TagProps{Color: display.TagGray, Size: theme.Small, Variant: display.TagBorderless}).Shrink(0)
			ui.Icon(c, lucide.ChevronRight()).FontSize(12).TextColor(tk.TextQuaternary).Shrink(0)
			display.Tag(c, t.DstFmt.Upper(), display.TagProps{Color: display.TagBlue, Size: theme.Small, Variant: display.TagBorderless}).Shrink(0)
			ui.Text(c, t.Title).FontSize(12).FontWeight(600).TextColor(tk.Text).Grow(1).Basis(0).MinWidth(0).SingleLine()
			if t.Warn {
				ui.Icon(c, lucide.TriangleAlert()).FontSize(13).TextColor(tk.Warning).Shrink(0)
			}
			display.Tag(c, stateLabel(s, p.State), display.TagProps{Color: stateTagColor(p.State), Variant: display.TagFilled, Size: theme.Small}).Shrink(0)
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
			switch p.State {
			case engine.Done:
				ui.Text(c, s.Tr("已保存")+filepath.Base(p.Dst)).FontSize(11).TextColor(tk.TextSecondary).Grow(1).Basis(0).MinWidth(0).SingleLine()
			case engine.Fail:
				ui.Text(c, strings.TrimPrefix(p.Line, "[fail] ")).FontSize(11).TextColor(tk.Error).Grow(1).Basis(0).MinWidth(0).SingleLine()
			default:
				bar := feedback.Progress(c, float32(p.Percent), feedback.ProgressProps{
					Size:   theme.Small,
					Status: progressStatus(p.State),
					Color:  progressColor(tk, p.State),
				})
				bar.Grow(1).Basis(0).MinWidth(60)
			}
			ui.Text(c, strconv.Itoa(p.Percent)+"%").FontSize(11).TextColor(tk.TextSecondary).FontFeatures("tnum").Width(32).TextAlign(ui.End)
			ui.Text(c, durationText(p.Elapsed)).FontSize(11).TextColor(tk.TextTertiary).FontFeatures("tnum").Shrink(0)
			switch p.State {
			case engine.Wait, engine.Process:
				if action.Button(c, s.Tr("终止"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
					s.Stop(t)
				}
			case engine.Done:
				if action.Button(c, s.Tr("定位"), action.ButtonProps{Variant: action.Link, Size: theme.Small}).Clicked() {
					mygo.Shell.ShowItemInFolder(p.Dst)
				}
			case engine.Fail:
				if action.Button(c, s.Tr("失败"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Clicked() {
					s.Detail = true
					s.DetailT = t
				}
			}
			if action.Button(c, s.Tr("运行日志"), action.ButtonProps{Variant: action.Text, Size: theme.Small}).Label(s.Tr("运行日志") + ": " + t.Title).Clicked() {
				s.ShowLog = !s.ShowLog || s.LogTask != t
				s.LogTask = t
			}
		})
		if s.ShowLog && s.LogTask == t {
			ui.Scroll(c.Key("log-" + t.ID)).Height(120).AlignSelf(ui.Stretch).Transition(motion.Collapse()).Children(func() {
				ui.Column(c).Gap(4).Padding(8).Radius(8).Background(tk.BgLayout).Children(func() {
					for _, m := range p.Msgs {
						ui.Text(c, m.Text).FontSize(11).LineHeight(1.4).TextColor(msgColor(tk, m.Kind))
					}
				})
			})
		}
	})
}

func stateTagColor(st engine.State) display.TagColor {
	switch st {
	case engine.Process:
		return display.TagBlue
	case engine.Done:
		return display.TagSuccess
	case engine.Fail:
		return display.TagError
	default:
		return display.TagGray
	}
}

func stateColor(tk *theme.Tokens, s engine.State) ui.Color {
	switch s {
	case engine.Done:
		return tk.Success
	case engine.Fail:
		return tk.Error
	case engine.Stop:
		return tk.Warning
	case engine.Process:
		return tk.Info
	}
	return tk.TextTertiary
}

func progressStatus(s engine.State) feedback.ProgressStatus {
	switch s {
	case engine.Done:
		return feedback.ProgressSuccess
	case engine.Fail:
		return feedback.ProgressException
	}
	return feedback.ProgressNormal
}

func progressColor(tk *theme.Tokens, s engine.State) ui.Color {
	switch s {
	case engine.Done:
		return tk.Success
	case engine.Fail:
		return tk.Error
	case engine.Stop:
		return tk.Warning
	}
	return tk.Info
}

func msgColor(tk *theme.Tokens, k engine.MsgKind) ui.Color {
	switch k {
	case engine.MsgError, engine.MsgFail:
		return tk.Error
	case engine.MsgDone:
		return tk.Success
	}
	return tk.TextTertiary
}

func durationText(d interface{ Seconds() float64 }) string {
	sec := int(d.Seconds())
	if sec < 60 {
		return strconv.Itoa(sec) + "s"
	}
	return strconv.Itoa(sec/60) + "m" + strconv.Itoa(sec%60) + "s"
}
