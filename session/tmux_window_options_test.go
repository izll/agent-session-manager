package session

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// remain-on-exit and automatic-rename are WINDOW options, and every call says so
// with -w.
//
// Not because tmux needs telling — measured on 3.4, it routes a window option to
// the window either way. The reason is the reader: without -w the target looks
// like a session and the scope has to be inferred.
//
// What breaks without the per-window setting is scope, not syntax: a window
// opened after a session-wide setting does not inherit it, and a pane exiting
// there takes the whole window with it rather than staying as a dead pane. The
// UI reads #{pane_dead} to mark a tab stopped and to respawn it, so both stop
// working, silently.
//
// Found in the desktop version first; the same code is here.
func TestWindowOptionsAreSetWithW(t *testing.T) {
	data, err := os.ReadFile("instance.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(data)

	windowOptions := []string{"remain-on-exit", "automatic-rename"}
	calls := regexp.MustCompile(`TmuxCommand\("set-option"([^)]*)\)`)

	for _, call := range calls.FindAllStringSubmatch(src, -1) {
		args := call[1]
		for _, opt := range windowOptions {
			if !strings.Contains(args, `"`+opt+`"`) {
				continue
			}
			if !strings.Contains(args, `"-w"`) {
				t.Errorf("set-option for %s is missing -w:\n  %s", opt, strings.TrimSpace(call[0]))
			}
		}
	}
}
