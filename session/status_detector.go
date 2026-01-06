package session

import (
	"fmt"
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

// AgentPatterns holds detection patterns for a specific agent
type AgentPatterns struct {
	WaitingPatterns []string // Patterns that indicate waiting for user input
	BusyPatterns    []string // Patterns that indicate agent is working
	Spinners        []string // Spinner characters
}

// Default spinner characters (braille dots)
var defaultSpinners = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Agent-specific patterns
var agentPatterns = map[AgentType]AgentPatterns{
	AgentClaude: {
		WaitingPatterns: []string{
			"allow once",
			"allow always",
			"yes, allow",
			"no, and tell",
			"esc to cancel",
			"do you want to proceed",
			"waiting for user",
			"waiting for tool",
			"apply this change",
			"? for shortcuts",
		},
		BusyPatterns: []string{
			"esc to interrupt",
			"tokens",
			"Generating",
		},
		Spinners: defaultSpinners,
	},
	AgentGemini: {
		WaitingPatterns: []string{
			"allow once",
			"allow always",
			"waiting for user",
			"do you want to proceed",
		},
		BusyPatterns: []string{
			"Generating",
			"esc to cancel",
		},
		Spinners: append(defaultSpinners, "∴", "∵", "⋮", "⋯"),
	},
	AgentAider: {
		WaitingPatterns: []string{
			"allow once",
			"allow always",
			"do you want to proceed",
			"waiting for user",
		},
		BusyPatterns: []string{
			"Generating",
			"tokens",
		},
		Spinners: defaultSpinners,
	},
	AgentCodex: {
		WaitingPatterns: []string{
			"allow once",
			"allow always",
			"do you want to proceed",
			"waiting for user",
		},
		BusyPatterns: []string{
			"Generating",
		},
		Spinners: defaultSpinners,
	},
	AgentAmazonQ: {
		WaitingPatterns: []string{
			"allow once",
			"allow always",
			"do you want to proceed",
			"waiting for user",
		},
		BusyPatterns: []string{
			"Generating",
		},
		Spinners: defaultSpinners,
	},
	AgentOpenCode: {
		WaitingPatterns: []string{
			"allow once",
			"allow always",
			"do you want to proceed",
			"waiting for user",
		},
		BusyPatterns: []string{
			"Generating",
		},
		Spinners: defaultSpinners,
	},
	AgentCustom: {
		WaitingPatterns: []string{
			"allow once",
			"allow always",
			"do you want to proceed",
			"waiting for user",
		},
		BusyPatterns: []string{
			"Generating",
		},
		Spinners: defaultSpinners,
	},
}

// getAgentPatterns returns patterns for the given agent type
func getAgentPatterns(agent AgentType) AgentPatterns {
	if patterns, ok := agentPatterns[agent]; ok {
		return patterns
	}
	// Default to Claude patterns
	return agentPatterns[AgentClaude]
}

// DetectActivity analyzes tmux pane content to determine session activity
// This always checks window 0 (the main agent window)
func (i *Instance) DetectActivity() SessionActivity {
	return i.DetectActivityForWindow(0)
}

// DetectActivityForWindow analyzes a specific tmux window to determine activity
func (i *Instance) DetectActivityForWindow(windowIdx int) SessionActivity {
	if !i.IsAlive() {
		return ActivityIdle
	}

	sessionName := i.TmuxSessionName()
	target := fmt.Sprintf("%s:%d", sessionName, windowIdx)

	// Determine agent type for this window
	agent := i.Agent
	if agent == "" {
		agent = AgentClaude
	}
	if windowIdx > 0 {
		for _, fw := range i.FollowedWindows {
			if fw.Index == windowIdx {
				agent = fw.Agent
				break
			}
		}
	}

	cmd := exec.Command("tmux", "capture-pane", "-t", target, "-p", "-S", "-50")
	output, err := cmd.Output()
	if err != nil {
		return ActivityIdle
	}

	lines := strings.Split(string(output), "\n")
	patterns := getAgentPatterns(agent)

	// Claude uses special UI structure detection
	if agent == AgentClaude {
		return detectClaudeActivity(lines, patterns)
	}

	// Gemini needs spinner-first detection
	if agent == AgentGemini {
		return detectGeminiActivity(lines, patterns)
	}

	// Other agents use generic detection
	return detectGenericActivity(lines, patterns)
}

// DetectAggregatedActivity checks all followed windows and returns highest priority activity
// Priority: Waiting > Busy > Idle
func (i *Instance) DetectAggregatedActivity() SessionActivity {
	if !i.IsAlive() {
		return ActivityIdle
	}

	// Always check window 0
	windowsToCheck := []int{0}

	// Add followed windows
	for _, fw := range i.FollowedWindows {
		if fw.Index != 0 { // 0 is already added
			windowsToCheck = append(windowsToCheck, fw.Index)
		}
	}

	highestActivity := ActivityIdle

	for _, winIdx := range windowsToCheck {
		activity := i.DetectActivityForWindow(winIdx)
		// Waiting has highest priority
		if activity == ActivityWaiting {
			return ActivityWaiting
		}
		// Busy is higher than Idle
		if activity == ActivityBusy && highestActivity == ActivityIdle {
			highestActivity = ActivityBusy
		}
	}

	return highestActivity
}

// detectClaudeActivity uses Claude Code's UI structure (horizontal separators)
func detectClaudeActivity(lines []string, patterns AgentPatterns) SessionActivity {
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
	var aboveSeparatorLines []string // Lines above top separator (for thinking state)

	if len(separatorIndices) >= 2 {
		// Normal mode: 2 separators, check between them
		topSepIdx := separatorIndices[len(separatorIndices)-2]
		bottomSepIdx := separatorIndices[len(separatorIndices)-1]

		// Count non-empty lines between separators
		contentCount := 0
		for idx := topSepIdx + 1; idx < bottomSepIdx; idx++ {
			cleanLine := strings.TrimSpace(stripANSIForDetect(lines[idx]))
			if cleanLine != "" {
				inputAreaLines = append(inputAreaLines, cleanLine)
				contentCount++
			}
		}

		// If only prompt line (or empty), check content ABOVE top separator
		// This is where Claude shows spinner and "esc to interrupt" during thinking
		if contentCount <= 1 {
			for j := topSepIdx - 1; j >= 0 && j >= topSepIdx-15; j-- {
				cleanLine := strings.TrimSpace(stripANSIForDetect(lines[j]))
				if cleanLine != "" {
					// Skip UI elements and tips
					if strings.HasPrefix(cleanLine, "╭") || strings.HasPrefix(cleanLine, "╰") ||
						strings.HasPrefix(cleanLine, "└") || strings.HasPrefix(cleanLine, "Tip:") {
						continue
					}
					aboveSeparatorLines = append(aboveSeparatorLines, cleanLine)
				}
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

	// Combine lines to check - input area has priority, then above separator
	allLinesToCheck := append(inputAreaLines, aboveSeparatorLines...)

	// First pass: check for waiting patterns (higher priority)
	for _, line := range allLinesToCheck {
		lineLower := strings.ToLower(line)
		for _, pattern := range patterns.WaitingPatterns {
			if strings.Contains(lineLower, pattern) {
				return ActivityWaiting
			}
		}
	}

	// Second pass: check for busy patterns (case-insensitive)
	for _, line := range allLinesToCheck {
		lineLower := strings.ToLower(line)
		for _, pattern := range patterns.BusyPatterns {
			if strings.Contains(lineLower, strings.ToLower(pattern)) {
				return ActivityBusy
			}
		}
		for _, s := range patterns.Spinners {
			if strings.Contains(line, s) {
				return ActivityBusy
			}
		}
	}

	return ActivityIdle
}

// detectGeminiActivity checks for Gemini's spinner when working
func detectGeminiActivity(lines []string, patterns AgentPatterns) SessionActivity {
	hasSpinner := false
	hasWaitingPattern := false

	// Count non-empty lines, not indices
	nonEmptyCount := 0
	for j := len(lines) - 1; j >= 0 && nonEmptyCount < 15; j-- {
		line := stripANSIForDetect(lines[j])
		if line == "" {
			continue
		}
		nonEmptyCount++
		lineLower := strings.ToLower(line)

		// Check for waiting patterns
		for _, pattern := range patterns.WaitingPatterns {
			if strings.Contains(lineLower, pattern) {
				hasWaitingPattern = true
				break
			}
		}

		// Check for spinner characters
		for _, s := range patterns.Spinners {
			if strings.Contains(line, s) {
				hasSpinner = true
				break
			}
		}

		// Check for busy patterns (case-insensitive)
		for _, pattern := range patterns.BusyPatterns {
			if strings.Contains(lineLower, strings.ToLower(pattern)) {
				hasSpinner = true // treat busy patterns like spinners
				break
			}
		}
	}

	// Waiting pattern takes priority (even if spinner is present)
	if hasWaitingPattern {
		return ActivityWaiting
	}

	// Spinner without waiting pattern = busy
	if hasSpinner {
		return ActivityBusy
	}

	return ActivityIdle
}

// detectGenericActivity checks last lines for other agents
func detectGenericActivity(lines []string, patterns AgentPatterns) SessionActivity {
	// First pass: check for waiting patterns (higher priority)
	nonEmptyCount := 0
	for j := len(lines) - 1; j >= 0 && nonEmptyCount < 15; j-- {
		line := strings.TrimSpace(stripANSIForDetect(lines[j]))
		if line == "" {
			continue
		}
		nonEmptyCount++
		lineLower := strings.ToLower(line)
		for _, pattern := range patterns.WaitingPatterns {
			if strings.Contains(lineLower, pattern) {
				return ActivityWaiting
			}
		}
	}

	// Second pass: check for busy patterns (case-insensitive)
	nonEmptyCount = 0
	for j := len(lines) - 1; j >= 0 && nonEmptyCount < 15; j-- {
		line := strings.TrimSpace(stripANSIForDetect(lines[j]))
		if line == "" {
			continue
		}
		nonEmptyCount++
		lineLower := strings.ToLower(line)
		for _, pattern := range patterns.BusyPatterns {
			if strings.Contains(lineLower, strings.ToLower(pattern)) {
				return ActivityBusy
			}
		}
		for _, s := range patterns.Spinners {
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
