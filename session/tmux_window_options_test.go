package session

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// remain-on-exit and automatic-rename are WINDOW options, and tmux needs -w.
//
// Without it the option lands on the session, where neither exists. tmux
// reports no error, so nothing looks wrong — until a pane's process exits and
// the whole window disappears with it instead of staying as a dead pane. The
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
