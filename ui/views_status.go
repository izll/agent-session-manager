package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/izll/agent-session-manager/session"
)

// buildStatusBar builds the status bar at the bottom
func (m Model) buildStatusBar() string {
	// Styles for status bar
	keyStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#1a1a2e")).
		Background(lipgloss.Color(ColorPurple)).
		Bold(true).
		Padding(0, 1)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorLightGray))

	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444"))

	onStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorGreen)).
		Bold(true)

	offStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorGray))

	sep := separatorStyle.Render(" │ ")

	// Priority groups (high to low)
	// P1: Essential - always shown
	p1 := []string{
		keyStyle.Render("n") + descStyle.Render(" new"),
		keyStyle.Render("enter") + descStyle.Render(" attach"),
		keyStyle.Render("?") + descStyle.Render(" help"),
		keyStyle.Render("q") + descStyle.Render(" quit"),
	}

	// P2: Common actions
	p2 := []string{
		keyStyle.Render("s") + descStyle.Render(" start"),
		keyStyle.Render("a") + descStyle.Render(" replace/start"),
		keyStyle.Render("x") + descStyle.Render(" stop"),
		keyStyle.Render("d") + descStyle.Render(" delete"),
		keyStyle.Render("p") + descStyle.Render(" prompt"),
	}

	// P3: Less common
	p3 := []string{
		keyStyle.Render("r") + descStyle.Render(" resume"),
		keyStyle.Render("e") + descStyle.Render(" rename"),
		keyStyle.Render("c") + descStyle.Render(" color"),
	}

	// P4: Group management
	p4 := []string{
		keyStyle.Render("g") + descStyle.Render(" group"),
		keyStyle.Render("G") + descStyle.Render(" assign"),
	}

	// P5: Toggles
	compactStatus := offStyle.Render("OFF")
	if m.compactList {
		compactStatus = onStyle.Render("ON")
	}
	statusLinesStatus := onStyle.Render("ON")
	if m.hideStatusLines {
		statusLinesStatus = offStyle.Render("OFF")
	}
	autoYesStatus := offStyle.Render("OFF")
	if inst := m.getSelectedInstance(); inst != nil && inst.AutoYes {
		autoYesStatus = onStyle.Render("ON")
	}
	splitStatus := offStyle.Render("OFF")
	if m.splitView {
		splitStatus = onStyle.Render("ON")
	}
	iconsStatus := offStyle.Render("OFF")
	if m.showAgentIcons {
		iconsStatus = onStyle.Render("ON")
	}
	p5 := []string{
		keyStyle.Render("l") + descStyle.Render(" compact ") + compactStatus,
		keyStyle.Render("o") + descStyle.Render(" output ") + statusLinesStatus,
		keyStyle.Render("I") + descStyle.Render(" icons ") + iconsStatus,
		keyStyle.Render("^Y") + descStyle.Render(" yolo ") + autoYesStatus,
		keyStyle.Render("v") + descStyle.Render(" split ") + splitStatus,
	}

	// Calculate widths and determine what fits
	sepWidth := 3 // " │ "

	// Try adding priority groups until it doesn't fit
	var items []string
	items = append(items, p1...)

	testWidth := func(newItems []string) int {
		total := 0
		for i, item := range newItems {
			total += len(stripANSI(item))
			if i < len(newItems)-1 {
				total += sepWidth
			}
		}
		return total
	}

	availableWidth := m.width - 4 // margin

	// Add P2 if fits
	testItems := append(items, p2...)
	if testWidth(testItems) <= availableWidth {
		items = testItems
		// Add P3 if fits
		testItems = append(items, p3...)
		if testWidth(testItems) <= availableWidth {
			items = testItems
			// Add P4 if fits
			testItems = append(items, p4...)
			if testWidth(testItems) <= availableWidth {
				items = testItems
				// Add P5 if fits
				testItems = append(items, p5...)
				if testWidth(testItems) <= availableWidth {
					items = testItems
				}
			}
		}
	}

	// Add search indicator at the left if filter is active
	if m.searchActive && m.searchQuery != "" {
		searchStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorYellow)).
			Bold(true)
		query := m.searchQuery
		if len(query) > 20 {
			query = query[:19] + "…"
		}
		searchIndicator := searchStyle.Render("🔍 " + query)
		escHint := descStyle.Render(" [ESC]")

		statusText := strings.Join(items, sep)
		rightPart := lipgloss.PlaceHorizontal(m.width-len(query)-10, lipgloss.Center, statusText)
		return "\n " + searchIndicator + escHint + rightPart
	}

	statusText := strings.Join(items, sep)
	return "\n" + lipgloss.PlaceHorizontal(m.width, lipgloss.Center, statusText)
}

// buildSessionGroupedView builds the flattened grouped view from agentSessions
func (m *Model) buildSessionGroupedView() {
	m.sessionGroupedView = nil

	if len(m.agentSessions) == 0 {
		return
	}

	// Initialize expanded map if nil
	if m.sessionGroupExpanded == nil {
		m.sessionGroupExpanded = make(map[string]bool)
	}

	// Group sessions by project path
	type projectGroup struct {
		path     string
		name     string
		sessions []session.AgentSession
	}
	groups := make(map[string]*projectGroup)
	var groupOrder []string

	for _, s := range m.agentSessions {
		path := s.ProjectPath
		if path == "" {
			path = "(unknown)"
		}

		if _, exists := groups[path]; !exists {
			// Extract short name from path
			name := path
			parts := strings.Split(path, "/")
			if len(parts) > 0 {
				name = parts[len(parts)-1]
			}
			groups[path] = &projectGroup{path: path, name: name}
			groupOrder = append(groupOrder, path)
		}
		groups[path].sessions = append(groups[path].sessions, s)
	}

	// Build flattened view
	// First item: "Start new session" (index 0 handled separately)

	for _, path := range groupOrder {
		g := groups[path]

		// Add group header
		m.sessionGroupedView = append(m.sessionGroupedView, sessionGroupItem{
			isGroupHeader: true,
			projectPath:   g.path,
			projectName:   g.name,
			sessionCount:  len(g.sessions),
			session:       &g.sessions[0], // First session for display
		})

		// If expanded, add all sessions (skip first since it's shown in header)
		if m.sessionGroupExpanded[path] {
			for i := 1; i < len(g.sessions); i++ {
				s := g.sessions[i]
				m.sessionGroupedView = append(m.sessionGroupedView, sessionGroupItem{
					isGroupHeader: false,
					projectPath:   g.path,
					session:       &s,
				})
			}
		}
	}
}

// selectSessionView renders the Claude session selector as an overlay dialog
func (m Model) selectSessionView() string {
	var b strings.Builder

	b.WriteString("\n\n")

	// Calculate visible window
	maxVisible := SessionListMaxItems
	startIdx := 0
	if m.sessionCursor > maxVisible-2 {
		startIdx = m.sessionCursor - maxVisible + 2
	}
	if startIdx < 0 {
		startIdx = 0
	}

	totalItems := len(m.sessionGroupedView) + 1 // +1 for "new session"

	// Option 0: Start new session
	if startIdx == 0 {
		otherCount := len(m.agentSessions)
		suffix := ""
		if otherCount > 0 {
			suffix = fmt.Sprintf(" (+%d sessions)", otherCount)
		}

		if m.sessionCursor == 0 {
			b.WriteString(fmt.Sprintf("  ❯ ▶ Start new session%s\n", suffix))
		} else {
			b.WriteString(fmt.Sprintf("    Start new session%s\n", suffix))
		}
		b.WriteString("\n")
	}

	// List grouped items
	visibleCount := 1
	for i, item := range m.sessionGroupedView {
		itemIdx := i + 1 // +1 because index 0 is "new session"

		if itemIdx < startIdx {
			continue
		}
		if visibleCount >= maxVisible {
			break
		}

		if item.isGroupHeader {
			// This is a group header - show first session with (+N) indicator
			cs := item.session
			prompt := cs.LastPrompt
			if prompt == "" {
				prompt = cs.FirstPrompt
			}
			maxPromptLen := 50
			if len([]rune(prompt)) > maxPromptLen {
				prompt = truncateRunes(prompt, maxPromptLen)
			}

			timeAgo := formatTimeAgo(cs.UpdatedAt)
			msgText := "messages"
			if cs.MessageCount == 1 {
				msgText = "message"
			}

			// Show project name and other sessions count
			expanded := m.sessionGroupExpanded[item.projectPath]
			expandIcon := "▶"
			if expanded {
				expandIcon = "▼"
			}

			otherSuffix := ""
			if item.sessionCount > 1 {
				otherSuffix = fmt.Sprintf(" (+%d other)", item.sessionCount-1)
			}

			projectDisplay := item.projectName
			if len(projectDisplay) > 20 {
				projectDisplay = "..." + projectDisplay[len(projectDisplay)-17:]
			}

			if itemIdx == m.sessionCursor {
				b.WriteString(selectedPromptStyle.Render(fmt.Sprintf("  ❯ %s %s%s", expandIcon, prompt, otherSuffix)))
				b.WriteString("\n")
				b.WriteString(metaStyle.Render(fmt.Sprintf("      %s · %d %s · %s", timeAgo, cs.MessageCount, msgText, projectDisplay)))
				b.WriteString("\n\n")
			} else {
				b.WriteString(fmt.Sprintf("    %s %s%s\n", expandIcon, prompt, otherSuffix))
				b.WriteString(dimStyle.Render(fmt.Sprintf("      %s · %d %s · %s", timeAgo, cs.MessageCount, msgText, projectDisplay)))
				b.WriteString("\n\n")
			}
		} else {
			// This is a child session (expanded from a group)
			cs := item.session
			prompt := cs.LastPrompt
			if prompt == "" {
				prompt = cs.FirstPrompt
			}
			maxPromptLen := 48
			if len([]rune(prompt)) > maxPromptLen {
				prompt = truncateRunes(prompt, maxPromptLen)
			}

			timeAgo := formatTimeAgo(cs.UpdatedAt)
			msgText := "messages"
			if cs.MessageCount == 1 {
				msgText = "message"
			}

			if itemIdx == m.sessionCursor {
				b.WriteString(selectedPromptStyle.Render(fmt.Sprintf("  ❯   ├─ %s", prompt)))
				b.WriteString("\n")
				b.WriteString(metaStyle.Render(fmt.Sprintf("          %s · %d %s", timeAgo, cs.MessageCount, msgText)))
				b.WriteString("\n\n")
			} else {
				b.WriteString(fmt.Sprintf("      ├─ %s\n", prompt))
				b.WriteString(dimStyle.Render(fmt.Sprintf("          %s · %d %s", timeAgo, cs.MessageCount, msgText)))
				b.WriteString("\n\n")
			}
		}
		visibleCount++
	}

	// Show more indicator
	remaining := totalItems - startIdx - maxVisible
	if remaining > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("    ... and %d more items\n", remaining)))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  ↑/↓ navigate • enter select • →/← expand/collapse • esc cancel"))
	b.WriteString("\n")

	// Calculate box width based on content
	boxWidth := 70
	if m.width > 90 {
		boxWidth = 80
	}

	return m.renderOverlayDialog(" Resume Session ", b.String(), boxWidth, ColorPurple)
}

// formatTimeAgo formats a time as a relative string (e.g., "5 min ago")
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	duration := time.Since(t)
	if duration < time.Minute {
		return "just now"
	} else if duration < time.Hour {
		return fmt.Sprintf("%d min ago", int(duration.Minutes()))
	} else if duration < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	} else {
		return fmt.Sprintf("%d days ago", int(duration.Hours()/24))
	}
}
