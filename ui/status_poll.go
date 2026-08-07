package ui

import (
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/izll/agent-session-manager/session"
)

// Reading each session's state off the UI thread.
//
// Detection is expensive in a way that is easy to miss: every window costs a
// `tmux capture-pane`, and a window showing a spinner costs a second one after
// a 60ms sleep, because the only way to tell a live spinner from one frozen in
// scrollback is to look twice. Run inline that arithmetic lands on the UI
// thread — five sessions of three busy tabs is nearly two seconds of sleeping
// per tick, against a 100ms tick. The queue never catches up and the interface
// stops answering the keyboard.
//
// So the work happens in a tea.Cmd, which Bubble Tea runs in its own goroutine,
// and the sessions are probed concurrently with each other. What comes back is
// a message applied in one step, which is also what keeps the model free of
// locks: nothing here touches the Model.

// statusPollResultMsg carries a completed poll back to the update loop.
type statusPollResultMsg struct {
	// Keyed by session ID. Sessions that were not polled are absent rather
	// than zeroed, so a partial poll cannot blank the rest of the list.
	lastLines      map[string]string
	activity       map[string]session.SessionActivity
	windowActivity map[string]map[int]session.SessionActivity
	mainWindow     map[string]int
	stopped        map[string]bool
}

// sessionPoll is one session's worth of results, before merging.
type sessionPoll struct {
	id             string
	lastLine       string
	activity       session.SessionActivity
	windowActivity map[int]session.SessionActivity
	mainWindow     int
	stopped        bool
}

// statusPollCmd probes the given sessions and reports what it found.
//
// The instances are used read-only here apart from UpdateStatus, which is why
// the results are returned rather than written: the model is owned by the
// update loop, and writing to it from this goroutine would be a data race
// however carefully it were done.
func statusPollCmd(instances []*session.Instance) tea.Cmd {
	if len(instances) == 0 {
		return nil
	}

	return func() tea.Msg {
		results := make([]sessionPoll, len(instances))
		var wg sync.WaitGroup

		for idx, inst := range instances {
			wg.Add(1)
			go func(idx int, inst *session.Instance) {
				defer wg.Done()
				results[idx] = pollSession(inst)
			}(idx, inst)
		}
		wg.Wait()

		msg := statusPollResultMsg{
			lastLines:      make(map[string]string, len(results)),
			activity:       make(map[string]session.SessionActivity, len(results)),
			windowActivity: make(map[string]map[int]session.SessionActivity, len(results)),
			mainWindow:     make(map[string]int, len(results)),
			stopped:        make(map[string]bool, len(results)),
		}
		for _, result := range results {
			if result.id == "" {
				continue
			}
			msg.lastLines[result.id] = result.lastLine
			msg.stopped[result.id] = result.stopped
			if result.stopped {
				continue
			}
			msg.activity[result.id] = result.activity
			msg.windowActivity[result.id] = result.windowActivity
			msg.mainWindow[result.id] = result.mainWindow
		}
		return msg
	}
}

// pollSession reads one session's state. Runs in its own goroutine.
func pollSession(inst *session.Instance) sessionPoll {
	inst.UpdateStatus()

	result := sessionPoll{id: inst.ID, lastLine: inst.GetLastLine()}
	if inst.Status != session.StatusRunning {
		result.stopped = true
		return result
	}

	result.mainWindow = inst.GetMainWindowIndex()
	result.windowActivity = make(map[int]session.SessionActivity)

	// The session's state is the strongest of its windows': one tab asking for
	// an answer is what the user needs to see about the whole session. Derived
	// from the per-window pass rather than asked for separately, because
	// DetectAggregatedActivity walks the same windows and probes each again.
	aggregate := inst.DetectActivityForWindow(result.mainWindow)
	result.windowActivity[result.mainWindow] = aggregate

	for _, fw := range inst.FollowedWindows {
		if fw.Index == result.mainWindow {
			continue
		}
		windowActivity := inst.DetectActivityForWindow(fw.Index)
		result.windowActivity[fw.Index] = windowActivity
		if windowActivity > aggregate {
			aggregate = windowActivity
		}
	}
	result.activity = aggregate
	return result
}

// applyStatusPoll merges a completed poll into the model.
func (m *Model) applyStatusPoll(msg statusPollResultMsg) {
	for id, line := range msg.lastLines {
		// isActive is "the output changed since we last looked", which the
		// views use as a fallback indicator. Compared before storing, since
		// storing is what makes them equal.
		previous := m.prevContent[id]
		m.isActive[id] = !msg.stopped[id] && line != previous && previous != ""
		m.prevContent[id] = line
		m.lastLines[id] = line
	}

	for id, stopped := range msg.stopped {
		if !stopped {
			continue
		}
		m.activityState[id] = session.ActivityIdle
		m.windowActivityState[id] = nil
		// Dropped with the activity: on restart the agent can land on a
		// different index, and a leftover entry would be read against the old
		// one.
		delete(m.mainWindowIndex, id)
	}

	for id, activity := range msg.activity {
		m.activityState[id] = activity
	}
	for id, windows := range msg.windowActivity {
		m.windowActivityState[id] = windows
	}
	for id, index := range msg.mainWindow {
		m.mainWindowIndex[id] = index
	}
}
