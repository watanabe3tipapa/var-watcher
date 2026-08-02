package engine

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os/exec"
	"strings"
	"sync"
	"syscall"
)

type State int

const (
	StateStopped State = iota
	StateRunning
	StateError
)

func (s State) String() string {
	switch s {
	case StateRunning:
		return "running"
	case StateError:
		return "error"
	default:
		return "stopped"
	}
}

var (
	ErrNotInstalled = errors.New("command not installed")
	ErrNotPermitted = errors.New("permission denied")
)

type Watcher struct {
	Name    string
	Command string
	Args    []string
	InitIn  []byte
	Check   []string // 起動に必要な追加バイナリ(例: entr が sh 内で使われる場合)

	bus       *Bus
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	enabled   bool
	state     State
	errMsg    string
	pid       int
	Installed bool
}

func NewWatcher(name, command string, args []string, bus *Bus) *Watcher {
	w := &Watcher{
		Name:    name,
		Command: command,
		Args:    args,
		bus:     bus,
		state:   StateStopped,
	}
	w.Installed = w.checkInstalled()
	return w
}

// checkInstalled は Command と Check の全バイナリが存在するかを確認する。
func (w *Watcher) checkInstalled() bool {
	seen := map[string]bool{}
	check := func(bin string) bool {
		if seen[bin] {
			return true
		}
		seen[bin] = true
		_, err := exec.LookPath(bin)
		return err == nil
	}
	if !check(w.Command) {
		return false
	}
	for _, b := range w.Check {
		if !check(b) {
			return false
		}
	}
	return true
}

func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.enabled {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()

	path, err := exec.LookPath(w.Command)
	if err != nil || !w.checkInstalled() {
		missing := w.Command
		if err == nil {
			for _, b := range w.Check {
				if _, lerr := exec.LookPath(b); lerr != nil {
					missing = b
					break
				}
			}
		}
		w.setError(ErrNotInstalled.Error() + ": " + missing)
		w.Installed = false
		return ErrNotInstalled
	}
	w.Installed = true
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, path, w.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		w.setError(err.Error())
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		w.setError(err.Error())
		return err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		w.setError(err.Error())
		return err
	}

	if err := cmd.Start(); err != nil {
		cancel()
		w.setError(err.Error())
		return err
	}

	w.mu.Lock()
	w.ctx, w.cancel = ctx, cancel
	w.enabled, w.state, w.pid = true, StateRunning, cmd.Process.Pid
	w.mu.Unlock()

	if len(w.InitIn) > 0 {
		go func() {
			_, _ = stdin.Write(w.InitIn)
			_ = stdin.Close()
		}()
	} else {
		_ = stdin.Close()
	}

	go w.readPipe("stdout", stdout)
	go w.readPipe("stderr", stderr)
	go func() {
		werr := cmd.Wait()
		w.mu.Lock()
		w.enabled, w.state = false, StateStopped
		if werr != nil && ctx.Err() == nil {
			w.state = StateError
			w.errMsg = werr.Error()
		}
		w.mu.Unlock()
	}()

	w.publish(LogLine{Source: w.Name, Level: "info", Message: "started (pid " + itoa(cmd.Process.Pid) + ")"})
	return nil
}

func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.enabled || w.cancel == nil {
		w.mu.Unlock()
		return
	}
	cancel, pid := w.cancel, w.pid
	w.enabled = false
	w.mu.Unlock()

	cancel()
	if pid > 0 {
		_ = syscall.Kill(-pid, syscall.SIGTERM)
	}
	w.publish(LogLine{Source: w.Name, Level: "info", Message: "stopped"})
}

func (w *Watcher) readPipe(kind string, r io.ReadCloser) {
	defer r.Close()
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		w.publish(LogLine{Source: w.Name, Level: kind, Message: line})
	}
}

func (w *Watcher) publish(line LogLine) {
	if w.bus != nil {
		w.bus.Publish(line)
	}
}

func (w *Watcher) setError(msg string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.state, w.errMsg = StateError, msg
}

func (w *Watcher) Info() WatcherInfo {
	w.mu.Lock()
	defer w.mu.Unlock()
	return WatcherInfo{
		Name:      w.Name,
		Command:   w.Command,
		Args:      append([]string(nil), w.Args...),
		Enabled:   w.enabled,
		State:     w.state.String(),
		Error:     w.errMsg,
		Installed: w.Installed,
		Pid:       w.pid,
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
