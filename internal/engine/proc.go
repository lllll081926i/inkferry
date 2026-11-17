package engine

import (
	"bufio"
	"context"
	"os/exec"
	"sync"
)

// Process is a converter child process the task can kill.
type childProc struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	killed bool
}

// Kill stops the child process.
func (p *childProc) Kill() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.killed = true
	if p.cmd == nil || p.cmd.Process == nil {
		return
	}
	_ = p.cmd.Process.Kill()
}

// runStream starts cmd, reads its combined output line by line into onLine,
// and reports the exit error.
func runStream(cmd *exec.Cmd, p *childProc, onLine func(string)) error {
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer pipe.Close()
	cmd.Stderr = cmd.Stdout
	p.mu.Lock()
	if p.killed {
		p.mu.Unlock()
		return context.Canceled
	}
	p.cmd = cmd
	err = cmd.Start()
	p.mu.Unlock()
	if err != nil {
		return err
	}
	r := bufio.NewReaderSize(pipe, 64<<10)
	for {
		line, readErr := r.ReadString('\n')
		if line != "" {
			onLine(line)
		}
		if readErr != nil {
			break
		}
	}
	return cmd.Wait()
}
