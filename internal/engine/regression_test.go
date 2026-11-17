package engine

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/signintech/gopdf"
	"golang.org/x/text/encoding/simplifiedchinese"
)

func isolatedManager(t *testing.T, notify func()) *Manager {
	t.Helper()
	isolate(t)
	m := NewManager(notify)
	m.Close()
	return m
}

func runTask(m *Manager, task *Task) {
	task.mu.Lock()
	task.State = Process
	task.Started = time.Now()
	task.mu.Unlock()
	m.run(task)
}

func TestOutputPreservesExistingFiles(t *testing.T) {
	m := isolatedManager(t, nil)
	src := write(t, "book.txt", "新的正文")
	base := filepath.Join(filepath.Dir(src), "(NEW)book.txt")
	second := filepath.Join(filepath.Dir(src), "(NEW)book (2).txt")
	for _, p := range []string{base, second} {
		if err := os.WriteFile(p, []byte("已有结果"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	task := m.Add([]string{src}, TXT)[0]
	runTask(m, task)
	if p := task.Snapshot(); p.State != Done || filepath.Base(p.Dst) != "(NEW)book (3).txt" {
		t.Fatalf("输出未正确避让: %+v", p)
	}
	for _, p := range []string{base, second} {
		b, err := os.ReadFile(p)
		if err != nil || string(b) != "已有结果" {
			t.Fatalf("已有文件被修改: %s: %q, %v", p, b, err)
		}
	}
}

func TestConcurrentOutputsHaveUniqueNames(t *testing.T) {
	m := isolatedManager(t, nil)
	src := write(t, "book.txt", "并发输出正文")
	tasks := m.Add([]string{src, src, src}, TXT)
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func() { defer wg.Done(); runTask(m, task) }()
	}
	wg.Wait()
	seen := map[string]bool{}
	for _, task := range tasks {
		p := task.Snapshot()
		if p.State != Done || seen[p.Dst] {
			t.Fatalf("输出路径冲突或任务失败: %+v", p)
		}
		seen[p.Dst] = true
		b, err := os.ReadFile(p.Dst)
		if err != nil || string(b) != "并发输出正文" {
			t.Fatalf("输出内容错误: %q, %v", b, err)
		}
	}
}

func TestSameNameTasksHaveSeparateScratch(t *testing.T) {
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	m := isolatedManager(t, nil)
	sources := []string{write(t, "book.txt", "甲书正文"), write(t, "book.txt", "乙书正文")}
	tasks := m.Add(sources, TXT)
	var reports atomic.Int32
	m.notify = func() {
		if reports.Add(1) <= 2 {
			ready <- struct{}{}
			<-release
		}
	}
	var wg sync.WaitGroup
	for _, task := range tasks {
		wg.Add(1)
		go func() { defer wg.Done(); runTask(m, task) }()
	}
	defer wg.Wait()
	defer close(release)
	for range tasks {
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
			t.Fatal("转换未到达屏障")
		}
	}
	root, err := TempDir()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 2 {
		t.Fatalf("任务未使用独立目录: %v, %v", entries, err)
	}
	contents := map[string]bool{}
	for _, entry := range entries {
		b, err := os.ReadFile(filepath.Join(root, entry.Name(), "book.txt"))
		if err != nil {
			t.Fatal(err)
		}
		contents[string(b)] = true
	}
	if !contents["甲书正文"] || !contents["乙书正文"] {
		t.Fatalf("临时输入发生串扰: %v", contents)
	}
}

func TestCancelBuiltinBeforeOutput(t *testing.T) {
	for _, dst := range []Format{TXT, EPUB, DOCX, PDF} {
		t.Run(string(dst), func(t *testing.T) {
			m := isolatedManager(t, nil)
			src := write(t, "cancel.txt", "取消后不应输出")
			task := m.Add([]string{src}, dst)[0]
			var canceled atomic.Bool
			m.notify = func() {
				if task.Snapshot().Percent == 45 && canceled.CompareAndSwap(false, true) {
					m.Cancel(task)
				}
			}
			runTask(m, task)
			if p := task.Snapshot(); p.State != Stop {
				t.Fatalf("取消状态被覆盖: %+v", p)
			}
			if _, err := os.Stat(task.Dst); !os.IsNotExist(err) {
				t.Fatalf("取消后仍产生输出: %v", err)
			}
			root, _ := TempDir()
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("取消后临时文件未清理: %v, %v", entries, err)
			}
		})
	}
}

func TestMalformedMOBIReturnsError(t *testing.T) {
	valid := buildTestMOBI([]byte("正文"))
	cases := map[string]func([]byte) []byte{
		"零记录":     func(b []byte) []byte { binary.BigEndian.PutUint16(b[76:], 0); return b },
		"偏移越界":    func(b []byte) []byte { binary.BigEndian.PutUint32(b[78:], uint32(len(b)+1)); return b },
		"偏移进入记录表": func(b []byte) []byte { binary.BigEndian.PutUint32(b[78:], 1); return b },
		"偏移逆序":    func(b []byte) []byte { binary.BigEndian.PutUint32(b[86:], 94); return b },
		"正文数量越界": func(b []byte) []byte {
			off := binary.BigEndian.Uint32(b[78:])
			binary.BigEndian.PutUint16(b[off+8:], 2)
			return b
		},
		"标头截断": func(b []byte) []byte {
			off := binary.BigEndian.Uint32(b[78:])
			binary.BigEndian.PutUint32(b[86:], off+180)
			return b
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			p := write(t, "broken.mobi", string(mutate(append([]byte(nil), valid...))))
			if _, _, err := readPDB(p); err == nil {
				t.Fatal("畸形 MOBI 未返回错误")
			}
		})
	}
}

func TestPalmDOCInstructions(t *testing.T) {
	cases := []struct {
		name    string
		in      []byte
		want    string
		invalid bool
	}{
		{"普通字符", []byte("abc"), "abc", false},
		{"字面量", []byte{3, 0x80, 0x00, 0xff}, string([]byte{0x80, 0x00, 0xff}), false},
		{"空格简写", []byte{0xc1}, " A", false},
		{"回溯距离八", append([]byte("abcdefghABCDEFGH"), 0x80, 0x40), "abcdefghABCDEFGHABC", false},
		{"重叠回溯", []byte{'a', 0x80, 0x0f}, "aaaaaaaaaaa", false},
		{"空回溯", []byte{0x80, 0x08}, "", true},
		{"零距离", []byte{'a', 0x80, 0}, "", true},
		{"距离越界", []byte{'a', 0x80, 0x10}, "", true},
		{"截断回溯", []byte{'a', 0x80}, "", true},
		{"截断字面量", []byte{3, 'a'}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := palmdocDecompress(tc.in)
			if tc.invalid {
				if err == nil {
					t.Fatal("无效压缩数据未报错")
				}
			} else if err != nil || string(got) != tc.want {
				t.Fatalf("解压结果 %q, %v；期望 %q", got, err, tc.want)
			}
		})
	}
}

func TestTextEncodings(t *testing.T) {
	gb18030, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte("中文𠀀"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		in   []byte
		want string
	}{
		{"UTF8", []byte("中文"), "中文"},
		{"UTF8BOM", append([]byte{0xef, 0xbb, 0xbf}, []byte("中文")...), "中文"},
		{"UTF16LE", []byte{0xff, 0xfe, 0x2d, 0x4e, 0x87, 0x65}, "中文"},
		{"UTF16BE", []byte{0xfe, 0xff, 0x4e, 0x2d, 0x65, 0x87}, "中文"},
		{"UTF16合法替换字符", []byte{0xff, 0xfe, 0xfd, 0xff}, "\ufffd"},
		{"GBK", []byte{0xd6, 0xd0, 0xce, 0xc4}, "中文"},
		{"GB18030", gb18030, "中文𠀀"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := write(t, "encoded.txt", string(tc.in))
			got, _, err := extractText(p, TXT)
			if err != nil || got != tc.want {
				t.Fatalf("解码结果 %q, %v", got, err)
			}
		})
	}
	for _, b := range [][]byte{{0xff}, {0xff, 0xfe, 0x01}, {0xff, 0xfe, 0x00, 0xd8}} {
		if _, err := decodeText(b); err == nil {
			t.Errorf("无效编码未报错: %x", b)
		}
	}
}

func TestEPUBRoundTripDoesNotLeakMetadata(t *testing.T) {
	p := filepath.Join(t.TempDir(), "roundtrip.epub")
	if err := writeEPUB(context.Background(), p, "图书标题", "中文正文"); err != nil {
		t.Fatal(err)
	}
	text, _, err := extractText(p, EPUB)
	if err != nil || strings.TrimSpace(text) != "中文正文" {
		t.Fatalf("正文混入元数据: %q, %v", text, err)
	}
	text, err = htmlToText(strings.NewReader("<HEAD>\n<TITLE>标题</TITLE>\n<STYLE>\np { color:red }\n</STYLE></HEAD><p>正文</p>"))
	if err != nil || strings.TrimSpace(text) != "正文" {
		t.Fatalf("多行标头混入正文: %q, %v", text, err)
	}
}

func TestPDFLinesFitActualFont(t *testing.T) {
	pdf := gopdf.GoPdf{}
	pdf.Start(gopdf.Config{PageSize: *gopdf.PageSizeA4, Unit: gopdf.UnitPT})
	if err := pdf.AddTTFFontData("body", CJKFont()); err != nil {
		t.Fatal(err)
	}
	if err := pdf.SetFont("body", "", fontSize); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{strings.Repeat("中文标点，。", 80), strings.Repeat("Wide words WWW and small iii. ", 50), strings.Repeat("中English文", 50)} {
		lines, err := wrapLine(text, pageW-2*marginX, pdf.MeasureTextWidth)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range lines {
			width, err := pdf.MeasureTextWidth(line)
			if err != nil || width > pageW-2*marginX {
				t.Fatalf("行宽超限: %f, %v", width, err)
			}
		}
		strip := func(s string) string { return strings.ReplaceAll(s, " ", "") }
		if strip(strings.Join(lines, "")) != strip(text) {
			t.Fatal("换行丢失正文")
		}
	}
}

func TestConverterProcessHelper(t *testing.T) {
	if os.Getenv("NC_CONVERTER_TEST_HELPER") != "1" {
		return
	}
	println("ready")
	for {
		time.Sleep(time.Second)
	}
}

func TestCancelNativeConverter(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestConverterProcessHelper$")
	cmd.Env = append(os.Environ(), "NC_CONVERTER_TEST_HELPER=1")
	p := &childProc{}
	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runStream(cmd, p, func(line string) {
			if strings.Contains(line, "ready") {
				close(ready)
			}
		})
	}()
	defer p.Kill()
	select {
	case <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("子进程未启动")
	}
	p.Kill()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("取消后子进程未返回错误")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("取消后子进程未退出")
	}
	p = &childProc{}
	p.Kill()
	cmd = exec.Command(os.Args[0])
	if err := runStream(cmd, p, func(string) {}); !errors.Is(err, context.Canceled) || cmd.Process != nil {
		t.Fatalf("启动前取消仍执行了进程: %v", err)
	}
}
