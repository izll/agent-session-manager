package session

import (
	"os/exec"
	"sync"
)

// Every multiplexer invocation in the app is built here, so the binary is named
// in exactly one place.
//
// tmux has no native Windows build. psmux is a native, tmux-compatible
// multiplexer answering the same subcommands, so a Windows build looks for that
// instead — which is the whole reason this indirection exists. Before it, 117
// calls named "tmux" directly, and a Windows build compiled cleanly and then
// failed at every one of them.
//
// The name is a variable rather than a constant so it can be overridden at
// runtime: someone running tmux through WSL or Cygwin, or from an unusual
// location, has no other way to say so.
//
// The mutex is not about contention — the name is written at most once, during
// start-up — but about the race detector: commands are built from the UI and
// the status-polling goroutines at the same time.
var (
	tmuxBinaryMu sync.RWMutex
	tmuxBinary   = defaultTmuxBinary
)

// SetTmuxBinary overrides the multiplexer executable, by name (resolved via
// PATH) or absolute path. An empty string restores the default, so a cleared
// setting cannot leave the app unable to find any binary at all.
func SetTmuxBinary(name string) {
	tmuxBinaryMu.Lock()
	defer tmuxBinaryMu.Unlock()
	if name == "" {
		name = defaultTmuxBinary
	}
	tmuxBinary = name
}

// TmuxBinary reports the executable TmuxCommand will run.
func TmuxBinary() string {
	tmuxBinaryMu.RLock()
	defer tmuxBinaryMu.RUnlock()
	return tmuxBinary
}

// TmuxCommand builds a multiplexer invocation. It is a drop-in for
// exec.Command("tmux", args...): the caller still owns Env, Dir, Stdin and
// whether the error is checked.
func TmuxCommand(args ...string) *exec.Cmd {
	return exec.Command(TmuxBinary(), args...)
}
