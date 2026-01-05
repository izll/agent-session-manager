package session

import (
	"os/exec"
	"strings"
)

// SessionActivity represents the activity state of a session
type SessionActivity int

const (
	ActivityIdle    SessionActivity = iota // No activity, no prompt
	ActivityBusy                           // Agent is working
	ActivityWaiting                        // Agent needs user input/permission
)

// Busy patterns (case sensitive)
var busyPatterns = []string{
	"esc to interrupt",
	"tokens",
}

// Waiting patterns (case insensitive)
var waitingPatterns = []string{
	"allow once",
	"allow always",
	"yes, allow",
	"no, and tell",
	"? for shortcuts",
	"esc to cancel",
	"do you want to proceed",
	"waiting for user",
	"waiting for tool",
	"apply this change",
}

// Spinner characters (braille dots)
var spinners = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}


// DetectActivity analyzes tmux pane content to determine session activity
func (i *Instance) DetectActivity() SessionActivity {
	if !i.IsAlive() {
		return ActivityIdle
	}

	sessionName := i.TmuxSessionName()
	cmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p", "-S", "-50")
	output, err := cmd.Output()
	if err != nil {
		return ActivityIdle
	}

	lines := strings.Split(string(output), "\n")

	// For Claude: use the area between horizontal separator lines
	if i.Agent == AgentClaude || i.Agent == "" {
		return detectClaudeActivity(lines)
	}

	// For other agents: simple pattern check on last lines
	return detectGenericActivity(lines)
}

// detectClaudeActivity uses Claude Code's UI structure (horizontal separators)
func detectClaudeActivity(lines []string) SessionActivity {
	// Find separator line positions
	var separatorIndices []int
	for idx, line := range lines {
		cleanLine := strings.TrimSpace(stripANSIForDetect(line))
		sepCount := strings.Count(cleanLine, "─") + strings.Count(cleanLine, "━")
		if sepCount > 20 {
			separatorIndices = append(separatorIndices, idx)
		}
	}

	var inputAreaLines []string

	if len(separatorIndices) >= 2 {
		// Normal mode: 2 separators, check between them
		topSepIdx := separatorIndices[len(separatorIndices)-2]
		bottomSepIdx := separatorIndices[len(separatorIndices)-1]

		for idx := topSepIdx + 1; idx < bottomSepIdx; idx++ {
			cleanLine := strings.TrimSpace(stripANSIForDetect(lines[idx]))
			if cleanLine != "" {
				inputAreaLines = append(inputAreaLines, cleanLine)
			}
		}
	} else if len(separatorIndices) == 1 {
		// Permission dialog: only 1 separator, check lines below it
		sepIdx := separatorIndices[0]
		for idx := sepIdx + 1; idx < len(lines); idx++ {
			cleanLine := strings.TrimSpace(stripANSIForDetect(lines[idx]))
			if cleanLine != "" {
				inputAreaLines = append(inputAreaLines, cleanLine)
			}
		}
	} else {
		// No separators - check last lines
		for j := len(lines) - 1; j >= 0 && j >= len(lines)-10; j-- {
			cleanLine := strings.TrimSpace(stripANSIForDetect(lines[j]))
			if cleanLine != "" {
				inputAreaLines = append(inputAreaLines, cleanLine)
			}
		}
	}

	// Check input area for patterns
	// First pass: check for waiting patterns (higher priority)
	for _, line := range inputAreaLines {
		lineLower := strings.ToLower(line)
		for _, pattern := range waitingPatterns {
			if strings.Contains(lineLower, pattern) {
				return ActivityWaiting
			}
		}
	}

	// Second pass: check for busy patterns
	for _, line := range inputAreaLines {
		for _, pattern := range busyPatterns {
			if strings.Contains(line, pattern) {
				return ActivityBusy
			}
		}
		for _, s := range spinners {
			if strings.Contains(line, s) {
				return ActivityBusy
			}
		}
	}

	return ActivityIdle
}

// detectGenericActivity checks last lines for other agents
func detectGenericActivity(lines []string) SessionActivity {
	// First pass: check for waiting patterns (higher priority)
	for j := len(lines) - 1; j >= 0 && j >= len(lines)-15; j-- {
		line := strings.TrimSpace(stripANSIForDetect(lines[j]))
		if line == "" {
			continue
		}
		lineLower := strings.ToLower(line)
		for _, pattern := range waitingPatterns {
			if strings.Contains(lineLower, pattern) {
				return ActivityWaiting
			}
		}
	}

	// Second pass: check for busy patterns
	for j := len(lines) - 1; j >= 0 && j >= len(lines)-15; j-- {
		line := strings.TrimSpace(stripANSIForDetect(lines[j]))
		if line == "" {
			continue
		}
		for _, pattern := range busyPatterns {
			if strings.Contains(line, pattern) {
				return ActivityBusy
			}
		}
		for _, s := range spinners {
			if strings.Contains(line, s) {
				return ActivityBusy
			}
		}
	}

	return ActivityIdle
}

// stripANSIForDetect removes ANSI escape sequences (uses stripANSI from instance.go)
func stripANSIForDetect(s string) string {
	return stripANSI(s)
}
