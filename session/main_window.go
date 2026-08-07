package session

import (
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// mainWindowCache holds the resolved index briefly.
//
// Identification costs a tmux fork, and the answer is needed on every path
// that reads or writes a pane: the status line of every session on every tick,
// the follow checks for every window in a list, the activity poll. Without
// this, hardening those call sites against the wrong-window bug would have
// replaced it with a performance one — measured at over a hundred forks a
// second on a workspace of seventeen sessions.
//
// Two seconds because the answer only changes when a window is created or
// closed, which is a user action; a stale answer for that long costs one tick
// of a status line pointing at the previous window.
var mainWindowCache sync.Map // map[string]mainWindowEntry

type mainWindowEntry struct {
	index   int
	ok      bool
	expires time.Time
}

const mainWindowCacheTTL = 2 * time.Second

// InvalidateMainWindow drops the cached answer for a session, for the moments
// when the window set changes and waiting out the TTL would show the wrong
// window in between.
func InvalidateMainWindow(sessionName string) {
	mainWindowCache.Delete(sessionName)
}

// Which tmux window holds the session's own agent.
//
// It is not always 0, and assuming it was is a family of bugs rather than one:
// tmux does not renumber when a window is closed, so a session whose first
// window was killed and recreated has its agent somewhere else entirely.
// Everything that reads or writes a pane by index — status lines, activity,
// resume ids, YOLO, notes, deletion guards — then targets the wrong window.
//
// The identification is deliberately in two parts. A marker stored on the tmux
// window object survives renumbering and move-window, and is authoritative
// when present. Where it is absent or meaningless, the main window is the one
// live window that is not a followed tab.

// GetMainWindowIndex reports the main window's index, falling back to 0.
//
// For presentational callers only — a status line drawn from the wrong window
// is a cosmetic error. Anything that kills, respawns or writes must use
// getMainWindowIndex and refuse to act when it cannot tell.
func (i *Instance) GetMainWindowIndex() int {
	index, ok := i.getMainWindowIndex()
	if !ok {
		return 0
	}
	return index
}

// getMainWindowIndex reports the main window's index and whether it could be
// determined at all.
//
// Returns false rather than guessing when the session is not running, when
// tmux cannot be asked, or when the answer is ambiguous. A refused
// identification degrades a feature; a wrong one kills the user's agent.
func (i *Instance) getMainWindowIndex() (int, bool) {
	if i.Status != StatusRunning {
		return 0, false
	}

	sessionName := i.TmuxSessionName()
	if cached, hit := mainWindowCache.Load(sessionName); hit {
		entry := cached.(mainWindowEntry)
		if time.Now().Before(entry.expires) {
			return entry.index, entry.ok
		}
	}

	output, err := exec.Command("tmux", "list-windows", "-t", sessionName,
		"-F", "#{window_index}\t#{@asmgr_main}").Output()
	if err != nil {
		return rememberMainWindow(sessionName, 0, false)
	}

	index, ok, anyMarked := identifyMainWindow(output, i.FollowedWindows)
	if !ok {
		return rememberMainWindow(sessionName, 0, false)
	}

	// Backfill the marker for a session created before it existed, but only
	// when nothing is marked yet. Backfilling unconditionally made the marked
	// set grow until it covered every window — observed on a live session with
	// eight windows, all marked — at which point identification correctly
	// refuses to choose and features that need it stop working.
	if !anyMarked {
		target := fmt.Sprintf("%s:%d", sessionName, index)
		_ = exec.Command("tmux", "set-option", "-w", "-t", target, "@asmgr_main", "1").Run()
	}
	return rememberMainWindow(sessionName, index, true)
}

func rememberMainWindow(sessionName string, index int, ok bool) (int, bool) {
	mainWindowCache.Store(sessionName, mainWindowEntry{
		index:   index,
		ok:      ok,
		expires: time.Now().Add(mainWindowCacheTTL),
	})
	return index, ok
}

// identifyMainWindowIndex picks the main window out of tmux's window list.
//
// Pure, so it can be tested without a running multiplexer — which matters,
// because every branch here encodes an incident.
func identifyMainWindowIndex(output []byte, followedWindows []FollowedWindow) (int, bool) {
	index, ok, _ := identifyMainWindow(output, followedWindows)
	return index, ok
}

// identifyMainWindow also reports whether any window carries the marker, so
// the backfill and the identification cannot disagree about what "marked"
// means — they did when the backfill matched a raw substring and the parser
// required an exact field.
func identifyMainWindow(output []byte, followedWindows []FollowedWindow) (index int, ok bool, anyMarked bool) {
	var live []int
	var marked []int
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		var index int
		if _, err := fmt.Sscanf(parts[0], "%d", &index); err != nil {
			continue
		}
		live = append(live, index)
		if len(parts) == 2 && strings.TrimSpace(parts[1]) == "1" {
			marked = append(marked, index)
		}
	}

	anyMarked = len(marked) > 0

	if len(marked) == 1 {
		return marked[0], true, anyMarked
	}

	// Every window marked means the marker carries no information: some
	// multiplexers store a -w user option globally, so one write tags the whole
	// session and the value cannot be unset. Treat it as absent and fall
	// through to identifying the window by what it is.
	//
	// A PARTIAL set of marks is different: that is a session where marking did
	// work and then went wrong, and guessing between them could kill the
	// agent's own window. That still fails closed.
	if len(marked) > 1 && len(marked) != len(live) {
		return 0, false, anyMarked
	}

	followed := make(map[int]struct{}, len(followedWindows))
	for _, window := range followedWindows {
		followed[window.Index] = struct{}{}
	}
	var candidates []int
	for _, index := range live {
		if _, isFollowed := followed[index]; !isFollowed {
			candidates = append(candidates, index)
		}
	}
	if len(candidates) != 1 {
		return 0, false, anyMarked
	}
	return candidates[0], true, anyMarked
}

// tmuxWindowExists reports whether a window index is really there.
//
// Mandatory before kill-window and respawn-pane: tmux silently falls back to
// the CURRENT window for a missing numeric target and exits 0, so a stale
// index does not fail — it destroys whatever the user is looking at, which is
// usually the agent.
func tmuxWindowExists(sessionName string, windowIdx int) bool {
	output, err := exec.Command("tmux", "list-windows", "-t", sessionName,
		"-F", "#{window_index}").Output()
	if err != nil {
		return false
	}
	return tmuxWindowIndexListed(output, windowIdx)
}

// tmuxWindowIndexListed matches an index exactly, never as a substring: "1"
// must not match the "12" in a window list.
func tmuxWindowIndexListed(output []byte, windowIdx int) bool {
	want := fmt.Sprintf("%d", windowIdx)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if strings.TrimSpace(line) == want {
			return true
		}
	}
	return false
}

// soleTmuxWindowIndex returns the index of a session's only window.
//
// For a session that has just been created, where the one window is the
// agent's, whatever number base-index gave it. Refuses when there is more than
// one, rather than marking the wrong window as main.
func soleTmuxWindowIndex(sessionName string) (int, bool) {
	output, err := exec.Command("tmux", "list-windows", "-t", sessionName,
		"-F", "#{window_index}").Output()
	if err != nil {
		return 0, false
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 1 {
		return 0, false
	}
	var index int
	if _, err := fmt.Sscanf(strings.TrimSpace(lines[0]), "%d", &index); err != nil {
		return 0, false
	}
	return index, true
}
