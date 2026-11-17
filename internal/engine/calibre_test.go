package engine

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// stubCalibre writes a fake ebook-convert into the component directory: a
// shell script that copies its input to its output while printing the
// progress lines Calibre prints. It lets the bridge that spawns the real
// converter be checked on any machine.
func stubCalibre(t *testing.T, body string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stub is a shell script")
	}
	dir := CalibreDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := CalibreExe()
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(exe) })
}

func TestCalibreBridgeReadsProgress(t *testing.T) {
	stubCalibre(t, `
echo "1% Converting input to HTML..."
echo "50% Parsing HTML..."
echo "99% Writing output..."
cp "$1" "$2"
echo "100% Output saved"
`)
	m := NewManager(nil)
	defer m.Close()
	p := write(t, "book.txt", "hello\n")
	m.Add([]string{p}, MOBI)
	var seen int
	for i := 0; i < 400; i++ {
		st := m.Tasks()[0].Snapshot()
		if st.Percent > seen {
			seen = st.Percent
		}
		switch st.State {
		case Done:
			if seen < 99 {
				t.Fatalf("progress stopped at %d: %v", seen, st.Msgs)
			}
			if _, err := os.Stat(m.Tasks()[0].Dst); err != nil {
				t.Fatal(err)
			}
			return
		case Fail, Stop:
			if strings.Contains(strings.Join(msgTexts(st.Msgs), " "), "not installed") {
				t.Skip("calibre is not installed on this machine")
			}
			t.Fatalf("failed: %v", msgTexts(st.Msgs))
		}
		m.Pump()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out")
}

func TestCalibreBridgeReportsFailure(t *testing.T) {
	stubCalibre(t, `
echo "1% Starting..."
echo "Traceback (most recent call last):" >&2
echo "error: cannot read the source" >&2
exit 3
`)
	m := NewManager(nil)
	defer m.Close()
	p := write(t, "book.txt", "hello\n")
	m.Add([]string{p}, MOBI)
	for i := 0; i < 400; i++ {
		st := m.Tasks()[0].Snapshot()
		switch st.State {
		case Fail:
			joined := strings.Join(msgTexts(st.Msgs), " ")
			if !strings.Contains(joined, "cannot read the source") && !strings.Contains(joined, "exit status") {
				t.Fatalf("failure not explained: %q", joined)
			}
			if _, err := os.Stat(m.Tasks()[0].Dst); err == nil {
				t.Fatal("a failed conversion left an output file")
			}
			return
		case Done:
			t.Fatal("a failing converter reported success")
		case Stop:
			t.Skip("calibre is not installed on this machine")
		}
		m.Pump()
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out")
}

func TestCancelKillsTheConverter(t *testing.T) {
	stubCalibre(t, `
echo "1% Starting..."
sleep 30
cp "$1" "$2"
`)
	m := NewManager(nil)
	defer m.Close()
	p := write(t, "book.txt", "hello\n")
	m.Add([]string{p}, MOBI)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		st := m.Tasks()[0].Snapshot()
		if st.State == Stop {
			t.Skip("calibre is not installed on this machine")
		}
		if st.State == Process {
			break
		}
		m.Pump()
		time.Sleep(20 * time.Millisecond)
	}
	if m.Tasks()[0].Snapshot().State != Process {
		t.Skip("the converter never started")
	}
	m.Cancel(m.Tasks()[0])
	for i := 0; i < 300; i++ {
		if st := m.Tasks()[0].Snapshot(); st.State == Stop {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("cancel did not stop the task")
}

// msgTexts flattens a task's log for assertions.
func msgTexts(msgs []Msg) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.Text
	}
	return out
}
