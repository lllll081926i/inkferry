// Command ferry converts ebooks without the UI. It drives the same engine
// as the desktop app, which makes it useful in terminals and scripts.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lllll081926i/inkferry/internal/engine"
)

func main() {
	to := flag.String("to", "epub", "target format: epub, mobi, azw3, pdf, txt, docx")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: %s [-to FORMAT] FILE...\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	dst := engine.Parse("x." + *to)
	if dst == "" {
		fmt.Fprintf(os.Stderr, "unknown target format %q\n", *to)
		os.Exit(2)
	}
	m := engine.NewManager(nil)
	defer m.Close()
	m.Add(flag.Args(), dst)
	for i := 0; i < 120; i++ {
		done := 0
		for _, t := range m.Tasks() {
			p := t.Snapshot()
			switch p.State {
			case engine.Done, engine.Fail, engine.Stop:
				done++
			}
		}
		if done == m.Len() && m.Len() > 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	bad := 0
	for _, t := range m.Tasks() {
		p := t.Snapshot()
		last := ""
		if len(p.Msgs) > 0 {
			last = p.Msgs[len(p.Msgs)-1].Text
		}
		fmt.Printf("%-9s %s  %s\n", strings.ToUpper(p.State.Name()), t.Src, last)
		if p.State != engine.Done {
			bad++
		}
	}
	if bad > 0 {
		os.Exit(1)
	}
}
