package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/izll/agent-session-manager/session"
)

// isGradientColor checks if a color name is a gradient
func isGradientColor(color string) bool {
	_, exists := gradients[color]
	return exists
}

// configureTmuxStatusBar sets up the tmux status bar to display tabs
func configureTmuxStatusBar(sessionName, instanceName, fgColor, bgColor string, autoYes bool) {
	target := sessionName + ":"

	// Enable status bar
	exec.Command("tmux", "set-option", "-t", target, "status", "on").Run()

	// Status bar style - dark background
	exec.Command("tmux", "set-option", "-t", target, "status-style", "bg=#1a1a2e,fg=#888888").Run()

	// Check if there are multiple windows
	windowCountOutput, _ := exec.Command("tmux", "list-windows", "-t", sessionName, "-F", "x").Output()
	windowCount := len(strings.Split(strings.TrimSpace(string(windowCountOutput)), "\n"))

	// Left side: session name with colors (gradient or solid)
	formattedName := formatTmuxSessionName(instanceName, fgColor, bgColor)
	var statusLeft string
	if windowCount > 1 {
		// Multiple windows - add separator
		statusLeft = fmt.Sprintf("#[default,bg=#1a1a2e] %s #[default,nobold]#[fg=#555555,bg=#1a1a2e]│ ", formattedName)
	} else {
		// Single window - no separator, but show YOLO if enabled
		if autoYes {
			statusLeft = fmt.Sprintf("#[default,bg=#1a1a2e] %s #[fg=#FFA500,bold]YOLO !#[default] ", formattedName)
		} else {
			statusLeft = fmt.Sprintf("#[default,bg=#1a1a2e] %s ", formattedName)
		}
	}
	exec.Command("tmux", "set-option", "-t", target, "status-left", statusLeft).Run()
	// Increase length to accommodate gradient (each char has color codes)
	exec.Command("tmux", "set-option", "-t", target, "status-left-length", "200").Run()

	// Right side: key hints
	exec.Command("tmux", "set-option", "-t", target, "status-right", "#[default]#[fg=#555555,bg=#1a1a2e] Alt+</>: tabs | Ctrl+Q: detach ").Run()
	exec.Command("tmux", "set-option", "-t", target, "status-right-length", "40").Run()

	// Window options for tabs - only show if multiple windows
	if windowCount > 1 {
		// Inactive: gray text
		exec.Command("tmux", "set-option", "-t", sessionName, "window-status-format", "#[fg=#888888]#{?pane_dead,○ ,}#W #[fg=#555555]│ ").Run()
		// Active: white bold text, with ! if YOLO mode
		if autoYes {
			exec.Command("tmux", "set-option", "-t", sessionName, "window-status-current-format", "#[fg=#FAFAFA,bold]#{?pane_dead,○ ,}#W #[fg=#FFA500]!#[fg=#555555,nobold]│ ").Run()
		} else {
			exec.Command("tmux", "set-option", "-t", sessionName, "window-status-current-format", "#[fg=#FAFAFA,bold]#{?pane_dead,○ ,}#W #[fg=#555555,nobold]│ ").Run()
		}
	} else {
		// Hide window list when only 1 window
		exec.Command("tmux", "set-option", "-t", sessionName, "window-status-format", "").Run()
		exec.Command("tmux", "set-option", "-t", sessionName, "window-status-current-format", "").Run()
	}

	// No separator
	exec.Command("tmux", "set-option", "-t", sessionName, "window-status-separator", "").Run()

	// Key bindings for tab switching: Alt+Left/Right (global, -n = no prefix needed)
	exec.Command("tmux", "bind-key", "-n", "M-Left", "previous-window").Run()
	exec.Command("tmux", "bind-key", "-n", "M-Right", "next-window").Run()
}

// handleEnterSession starts (if needed) and attaches to the selected session
func (m *Model) handleEnterSession() tea.Cmd {
	var inst *session.Instance

	// In split view with focus on pinned, attach to pinned session
	if m.splitView && m.splitFocus == 1 && m.markedSessionID != "" {
		for _, i := range m.instances {
			if i.ID == m.markedSessionID {
				inst = i
				break
			}
		}
	} else {
		inst = m.getSelectedInstance()
	}

	if inst == nil {
		return nil
	}
	if inst.Status != session.StatusRunning {
		// Check if command exists before starting
		if err := session.CheckAgentCommand(inst); err != nil {
			m.err = err
			m.previousState = stateList
			m.state = stateError
			return nil
		}
		if err := inst.Start(); err != nil {
			m.err = err
			m.previousState = stateList
			m.state = stateError
			return nil
		}
		m.storage.UpdateInstance(inst)
	} else {
		// Session is running - check if active tab is dead and respawn it
		windows := inst.GetWindowList()
		for _, w := range windows {
			if w.Active && w.Dead {
				inst.RespawnWindow(w.Index)
				break
			}
		}
	}
	sessionName := inst.TmuxSessionName()
	// Configure tmux for proper terminal resize following (ignore errors - non-critical)
	exec.Command("tmux", "set-option", "-t", sessionName, "window-size", "largest").Run()
	exec.Command("tmux", "set-option", "-t", sessionName, "aggressive-resize", "on").Run()
	// Enable focus events for hooks to work
	exec.Command("tmux", "set-option", "-t", sessionName, "focus-events", "on").Run()
	// Set up hook to resize window on focus gain (fixes Konsole tab switch issue)
	exec.Command("tmux", "set-hook", "-t", sessionName, "client-focus-in", "resize-window -A").Run()
	exec.Command("tmux", "set-hook", "-t", sessionName, "pane-focus-in", "resize-window -A").Run()

	// Update window name (removes any old " ! " prefix from YOLO mode)
	exec.Command("tmux", "rename-window", "-t", sessionName, inst.Name).Run()

	// Configure tmux status bar to show tabs
	configureTmuxStatusBar(sessionName, inst.Name, inst.Color, inst.BgColor, inst.AutoYes)

	// Set up Ctrl+Q to resize to preview size before detach
	tmuxWidth, tmuxHeight := m.calculateTmuxDimensions()
	inst.UpdateDetachBinding(tmuxWidth, tmuxHeight)
	cmd := exec.Command("tmux", "attach-session", "-t", sessionName)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return reattachMsg{}
	})
}

// handleResumeSession shows Claude sessions for the current instance
func (m *Model) handleResumeSession() error {
	inst := m.getSelectedInstance()
	if inst == nil {
		return nil
	}
	// List sessions based on agent type
	var sessions []session.AgentSession
	var err error

	switch inst.Agent {
	case session.AgentGemini:
		sessions, err = session.ListGeminiSessions(inst.Path)
	case session.AgentCodex:
		sessions, err = session.ListCodexSessions(inst.Path)
	case session.AgentOpenCode:
		sessions, err = session.ListOpenCodeSessions(inst.Path)
	case session.AgentAmazonQ:
		sessions, err = session.ListAmazonQSessions(inst.Path)
	default:
		// Claude and others
		sessions, err = session.ListAgentSessions(inst.Path)
	}

	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		return fmt.Errorf("no previous %s sessions found", inst.Agent)
	}
	m.agentSessions = sessions
	m.sessionCursor = 1 // Start with first session selected (0 is "new session")
	m.state = stateSelectAgentSession
	return nil
}

// handleStartSession starts the selected session without attaching
func (m *Model) handleStartSession() {
	inst := m.getSelectedInstance()
	if inst == nil {
		return
	}
	if inst.Status != session.StatusRunning {
		// Check if command exists before starting
		if err := session.CheckAgentCommand(inst); err != nil {
			m.err = err
			m.previousState = stateList
			m.state = stateError
			return
		}
		if err := inst.Start(); err != nil {
			m.err = err
			m.previousState = stateList
			m.state = stateError
		} else {
			m.storage.UpdateInstance(inst)
		}
	}
}

// handleStopSession shows confirmation dialog for stopping the selected session
func (m *Model) handleStopSession() {
	inst := m.getSelectedInstance()
	if inst == nil {
		return
	}
	if inst.Status == session.StatusRunning {
		m.stopTarget = inst
		m.state = stateConfirmStop
	}
}

// handleRenameSession opens the rename dialog for the selected session
func (m *Model) handleRenameSession() tea.Cmd {
	inst := m.getSelectedInstance()
	if inst == nil {
		return nil
	}
	m.nameInput.SetValue(inst.Name)
	m.nameInput.Focus()
	m.state = stateRename
	return textinput.Blink
}

// handleColorPicker opens the color picker for the selected session
func (m *Model) handleColorPicker() {
	inst := m.getSelectedInstance()
	if inst == nil {
		return
	}
	// Initialize preview colors
	m.previewFg = inst.Color
	m.previewBg = inst.BgColor
	m.colorMode = 0
	m.editingGroup = nil
	// Find current color index in filtered list
	m.colorCursor = 0
	filteredColors := m.getFilteredColorOptions()
	for i, c := range filteredColors {
		if c.Color == inst.Color || c.Name == inst.Color {
			m.colorCursor = i
			break
		}
	}
	m.state = stateColorPicker
}

// handleGroupColorPicker opens the color picker for a group
func (m *Model) handleGroupColorPicker(group *session.Group) {
	m.editingGroup = group
	m.previewFg = group.Color
	m.previewBg = group.BgColor
	m.colorMode = 0
	// Find current color index in filtered list
	m.colorCursor = 0
	filteredColors := m.getFilteredColorOptions()
	for i, c := range filteredColors {
		if c.Color == group.Color || c.Name == group.Color {
			m.colorCursor = i
			break
		}
	}
	m.state = stateColorPicker
}

// handleSendPrompt opens the prompt input for the selected session
func (m *Model) handleSendPrompt() {
	inst := m.getSelectedInstance()
	if inst == nil {
		return
	}
	if inst.Status != session.StatusRunning {
		m.err = fmt.Errorf("session not running")
		m.previousState = stateList
		m.state = stateError
		return
	}
	m.promptInput.SetValue("")
	inputWidth := PromptMinWidth
	if m.width > 80 {
		inputWidth = m.width/2 - 10
	}
	if inputWidth > PromptMaxWidth {
		inputWidth = PromptMaxWidth
	}
	m.promptInput.Width = inputWidth
	m.promptInput.Focus()

	// Get suggestion from agent
	m.promptSuggestion = inst.GetSuggestion()

	m.state = statePrompt
}

// handleForceResize forces resize of the selected pane
func (m *Model) handleForceResize() {
	inst := m.getSelectedInstance()
	if inst == nil {
		return
	}
	tmuxWidth, tmuxHeight := m.calculateTmuxDimensions()
	if err := inst.ResizePane(tmuxWidth, tmuxHeight); err != nil {
		m.err = fmt.Errorf("failed to resize pane: %w", err)
		m.previousState = stateList
		m.state = stateError
	}
}

// handleToggleAutoYes toggles the auto-yes flag on the selected session
// Returns a tea.Cmd to attach to the session if it was restarted
func (m *Model) handleToggleAutoYes() tea.Cmd {
	inst := m.getSelectedInstance()
	if inst == nil {
		return nil
	}

	// Get agent type (empty string means Claude for backward compatibility)
	agentType := inst.Agent
	if agentType == "" {
		agentType = session.AgentClaude
	}

	// Special handling for Gemini - send Ctrl+Y keystroke instead
	if agentType == session.AgentGemini {
		if inst.Status == session.StatusRunning {
			if err := inst.SendKeys("C-y"); err != nil {
				m.err = fmt.Errorf("failed to send Ctrl+Y: %w", err)
				m.previousState = stateList
				m.state = stateError
			}
		}
		return nil
	}

	// Check if agent supports AutoYes
	config := session.AgentConfigs[agentType]
	if !config.SupportsAutoYes {
		m.err = fmt.Errorf("yolo mode not supported for %s agent", agentType)
		m.previousState = stateList
		m.state = stateError
		return nil
	}

	// Toggle AutoYes
	wasRunning := inst.Status == session.StatusRunning
	inst.AutoYes = !inst.AutoYes
	m.storage.UpdateInstance(inst)

	// If running, restart with new flag (no auto-attach in list view)
	if wasRunning {
		inst.Stop()
		if err := inst.Start(); err != nil {
			m.err = fmt.Errorf("failed to restart session: %w", err)
			m.previousState = stateList
			m.state = stateError
			return nil
		}
		m.storage.UpdateInstance(inst)
	}

	return nil
}
