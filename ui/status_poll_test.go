package ui

import (
	"testing"

	"github.com/izll/agent-session-manager/session"
)

// A poll's results are applied in one step, and what it did not measure it
// must not overwrite. A session that was skipped this round — non-selected
// sessions only refresh every fifth tick — has to keep the state it had, or
// the list would blink between real values and blanks.
func TestApplyingAPollLeavesUnpolledSessionsAlone(t *testing.T) {
	m := newTestModel()
	m.lastLines["untouched"] = "still here"
	m.activityState["untouched"] = session.ActivityBusy

	m.applyStatusPoll(statusPollResultMsg{
		lastLines: map[string]string{"polled": "fresh"},
		activity:  map[string]session.SessionActivity{"polled": session.ActivityWaiting},
		stopped:   map[string]bool{"polled": false},
	})

	if got := m.lastLines["untouched"]; got != "still here" {
		t.Errorf("an unpolled session's line became %q", got)
	}
	if got := m.activityState["untouched"]; got != session.ActivityBusy {
		t.Errorf("an unpolled session's activity became %v", got)
	}
	if got := m.lastLines["polled"]; got != "fresh" {
		t.Errorf("the polled session's line = %q, want it updated", got)
	}
}

// isActive means "the output changed since we last looked", and it is compared
// before the new line is stored — storing first would make every session look
// unchanged forever.
func TestChangedOutputMarksASessionActive(t *testing.T) {
	m := newTestModel()
	m.prevContent["s"] = "before"

	m.applyStatusPoll(statusPollResultMsg{
		lastLines: map[string]string{"s": "after"},
		stopped:   map[string]bool{"s": false},
	})

	if !m.isActive["s"] {
		t.Error("output changed but the session was not marked active")
	}
	if m.prevContent["s"] != "after" {
		t.Error("the new line was not remembered for the next comparison")
	}

	// Same line next time round: no longer active.
	m.applyStatusPoll(statusPollResultMsg{
		lastLines: map[string]string{"s": "after"},
		stopped:   map[string]bool{"s": false},
	})
	if m.isActive["s"] {
		t.Error("unchanged output still counted as activity")
	}
}

// A session with no previous content has nothing to compare against. Treating
// the first line as a change would light up every session on startup.
func TestFirstSightingIsNotActivity(t *testing.T) {
	m := newTestModel()

	m.applyStatusPoll(statusPollResultMsg{
		lastLines: map[string]string{"s": "first line"},
		stopped:   map[string]bool{"s": false},
	})

	if m.isActive["s"] {
		t.Error("a session's first line counted as activity")
	}
}

// A stopped session keeps no window state. Its agent can come back at a
// different index, and a leftover entry would be read against the old one.
func TestStoppingASessionDropsItsWindowState(t *testing.T) {
	m := newTestModel()
	m.windowActivityState["s"] = map[int]session.SessionActivity{3: session.ActivityBusy}
	m.mainWindowIndex["s"] = 3
	m.activityState["s"] = session.ActivityBusy

	m.applyStatusPoll(statusPollResultMsg{
		lastLines: map[string]string{"s": "stopped"},
		stopped:   map[string]bool{"s": true},
	})

	if m.windowActivityState["s"] != nil {
		t.Error("window activity survived the session stopping")
	}
	if _, present := m.mainWindowIndex["s"]; present {
		t.Error("the main window index survived the session stopping")
	}
	if m.activityState["s"] != session.ActivityIdle {
		t.Error("a stopped session should read as idle")
	}
	if m.isActive["s"] {
		t.Error("a stopped session should not read as active")
	}
}

// The main window's index is recorded per session, because it is not
// necessarily 0 and the views need it without asking tmux on every frame.
func TestPollRecordsWhereEachAgentLives(t *testing.T) {
	m := newTestModel()

	m.applyStatusPoll(statusPollResultMsg{
		lastLines:  map[string]string{"a": "x", "b": "y"},
		stopped:    map[string]bool{"a": false, "b": false},
		mainWindow: map[string]int{"a": 4, "b": 0},
		windowActivity: map[string]map[int]session.SessionActivity{
			"a": {4: session.ActivityWaiting},
			"b": {0: session.ActivityIdle},
		},
	})

	if m.mainWindowIndex["a"] != 4 {
		t.Errorf("main window index = %d, want 4", m.mainWindowIndex["a"])
	}
	if got := m.windowActivityState["a"][4]; got != session.ActivityWaiting {
		t.Errorf("activity for window 4 = %v, want waiting", got)
	}
}

func newTestModel() *Model {
	return &Model{
		lastLines:           map[string]string{},
		prevContent:         map[string]string{},
		isActive:            map[string]bool{},
		activityState:       map[string]session.SessionActivity{},
		windowActivityState: map[string]map[int]session.SessionActivity{},
		mainWindowIndex:     map[string]int{},
	}
}
