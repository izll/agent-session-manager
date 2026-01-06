package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/izll/agent-session-manager/session"
)

// buildPreviewPane builds the right pane containing the preview
func (m Model) buildPreviewPane(contentHeight int) string {
	var rightPane strings.Builder

	// Header with Preview title on left and version on right
	previewWidth := m.calculatePreviewWidth()
	title := titleStyle.Render(" Preview ")

	// Add update indicator if available
	versionText := fmt.Sprintf("%s v%s", AppName, AppVersion)
	if m.updateAvailable != "" {
		updateIcon := lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorYellow)).
			Render(" ↑")
		versionText = versionText + updateIcon
	}
	version := dimStyle.Render(versionText + " ")

	titleLen := lipgloss.Width(title)
	versionLen := lipgloss.Width(version)
	spacing := previewWidth - titleLen - versionLen
	if spacing < 1 {
		spacing = 1
	}
	rightPane.WriteString(title + strings.Repeat(" ", spacing) + version)
	rightPane.WriteString("\n")
	rightPane.WriteString(dimStyle.Render(strings.Repeat("─", previewWidth)))
	rightPane.WriteString("\n")

	// Get selected instance (handles both grouped and ungrouped modes)
	var inst *session.Instance
	if len(m.groups) > 0 {
		m.buildVisibleItems()
		if m.cursor >= 0 && m.cursor < len(m.visibleItems) {
			item := m.visibleItems[m.cursor]
			if !item.isGroup {
				inst = item.instance
			} else {
				// Group selected - show group info
				rightPane.WriteString(dimStyle.Render(fmt.Sprintf("  Group: %s", item.group.Name)))
				rightPane.WriteString("\n")
				sessionCount := len(m.getSessionsInGroup(item.group.ID))
				rightPane.WriteString(dimStyle.Render(fmt.Sprintf("  Sessions: %d", sessionCount)))
				rightPane.WriteString("\n\n")
				rightPane.WriteString(dimStyle.Render("  Press Enter to toggle collapse"))
				return rightPane.String()
			}
		}
	} else if len(m.instances) > 0 && m.cursor < len(m.instances) {
		inst = m.instances[m.cursor]
	}

	if inst == nil {
		return rightPane.String()
	}

	// Instance info with styled labels and values
	rightPane.WriteString("  " + projectLabelStyle.Render("Path: ") + projectNameStyle.Render(inst.Path))
	rightPane.WriteString("\n")

	// Show agent type
	agentName := "Claude Code"
	switch inst.Agent {
	case session.AgentGemini:
		agentName = "Gemini"
	case session.AgentAider:
		agentName = "Aider"
	case session.AgentCodex:
		agentName = "Codex CLI"
	case session.AgentAmazonQ:
		agentName = "Amazon Q"
	case session.AgentOpenCode:
		agentName = "OpenCode"
	case session.AgentCustom:
		agentName = "Custom"
	}
	// Show yolo mode for Claude on same line
	if (inst.Agent == session.AgentClaude || inst.Agent == "") && inst.AutoYes {
		yoloStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrange))
		rightPane.WriteString("  " + projectLabelStyle.Render("Agent: ") + projectNameStyle.Render(agentName) + yoloStyle.Render(" ! YOLO"))
	} else {
		rightPane.WriteString("  " + projectLabelStyle.Render("Agent: ") + projectNameStyle.Render(agentName))
	}
	rightPane.WriteString("\n")

	// Calculate actual header height: Initial(1) + Title(1) + blank(1) + Path(1) + Agent(1) + separator(1) = 6 base + 1 buffer
	headerLines := 7

	if inst.Agent == session.AgentCustom && inst.CustomCommand != "" {
		rightPane.WriteString("  " + projectLabelStyle.Render("Command: ") + projectNameStyle.Render(inst.CustomCommand))
		rightPane.WriteString("\n")
		headerLines++
	}

	if inst.ResumeSessionID != "" {
		rightPane.WriteString("  " + projectLabelStyle.Render("Resume: ") + projectNameStyle.Render(inst.ResumeSessionID[:8]))
		rightPane.WriteString("\n")
		headerLines++
	}

	// Display notes if any (truncated to fit)
	if inst.Notes != "" {
		// Show first line of notes or truncate if too long
		notesPreview := inst.Notes
		if idx := strings.Index(notesPreview, "\n"); idx != -1 {
			notesPreview = notesPreview[:idx] + "…"
		}
		maxNotesLen := previewWidth - 12 // "  Notes: " prefix + some margin
		if len([]rune(notesPreview)) > maxNotesLen {
			notesPreview = truncateRunes(notesPreview, maxNotesLen)
		}
		notesStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorYellow)).Italic(true)
		rightPane.WriteString("  " + projectLabelStyle.Render("Notes: ") + notesStyle.Render(notesPreview))
		rightPane.WriteString("\n")
		headerLines++
	}

	// Horizontal separator
	rightPane.WriteString(dimStyle.Render(strings.Repeat("─", previewWidth)))
	rightPane.WriteString("\n")
	headerLines++

	// Preview content
	if m.preview == "" {
		rightPane.WriteString(dimStyle.Render("  (no output yet)"))
		return rightPane.String()
	}

	// Use scrollContent if scrolling, otherwise use preview
	content := m.preview
	if m.previewScroll > 0 && m.scrollContent != "" {
		content = m.scrollContent
	}
	lines := strings.Split(content, "\n")
	maxLines := contentHeight - headerLines
	if maxLines < MinPreviewLines {
		maxLines = MinPreviewLines
	}

	// When scrolling, always reserve 2 lines for indicators to prevent layout shift
	if m.previewScroll > 0 {
		maxLines -= 2
	}

	// Apply scroll offset
	endIdx := len(lines) - m.previewScroll
	if endIdx < maxLines {
		endIdx = maxLines
	}
	if endIdx > len(lines) {
		endIdx = len(lines)
	}
	startIdx := endIdx - maxLines
	if startIdx < 0 {
		startIdx = 0
	}

	// Show scroll indicator at top if scrolled and not at beginning
	if m.previewScroll > 0 && startIdx > 0 {
		rightPane.WriteString(dimStyle.Render("   ↑ more"))
		rightPane.WriteString("\n")
	} else if m.previewScroll > 0 {
		// Empty line to keep layout stable
		rightPane.WriteString("\n")
	}

	for i := startIdx; i < endIdx; i++ {
		line := lines[i]
		// Truncate to available width (previewWidth - 2 for left margin)
		maxWidth := previewWidth - 2
		if displayWidth(line) > maxWidth {
			line = truncateToWidth(line, maxWidth)
		}
		rightPane.WriteString("  " + line + "\x1b[0m\n")
	}

	// Show scroll indicator at bottom if scrolled
	if m.previewScroll > 0 {
		rightPane.WriteString(dimStyle.Render(fmt.Sprintf("   ↓ more (%d lines)", m.previewScroll)))
		rightPane.WriteString("\n")
	}

	// Truncate to exactly contentHeight lines to prevent layout shift
	result := rightPane.String()
	resultLines := strings.Split(result, "\n")
	if len(resultLines) > contentHeight {
		resultLines = resultLines[:contentHeight]
	}
	return strings.Join(resultLines, "\n")
}

// buildSplitPreviewPane builds split view with two preview panes
func (m Model) buildSplitPreviewPane(contentHeight int) string {
	var result strings.Builder
	previewWidth := m.calculatePreviewWidth()

	// Get selected instance
	selectedInst := m.getSelectedInstance()

	// Get marked instance
	var markedInst *session.Instance
	if m.markedSessionID != "" {
		for _, inst := range m.instances {
			if inst.ID == m.markedSessionID {
				markedInst = inst
				break
			}
		}
	}

	// Calculate heights for each pane
	halfHeight := (contentHeight - 1) / 2 // -1 for separator

	// Top pane: marked session (pinned)
	topFocused := m.splitFocus == 1
	topScroll := 0
	if topFocused {
		topScroll = m.previewScroll
	}
	if markedInst != nil {
		result.WriteString("\n") // Add spacing at top
		result.WriteString(m.buildMiniPreview(markedInst, halfHeight, previewWidth, "Pinned", topFocused, topScroll))
	} else {
		result.WriteString("\n")
		result.WriteString(dimStyle.Render("  Press 'm' to pin a session"))
		result.WriteString("\n")
	}

	// Separator
	result.WriteString(dimStyle.Render(strings.Repeat("─", previewWidth-2)))
	result.WriteString("\n")

	// Bottom pane: selected session
	bottomFocused := m.splitFocus == 0
	bottomScroll := 0
	if bottomFocused {
		bottomScroll = m.previewScroll
	}
	if selectedInst != nil && (markedInst == nil || selectedInst.ID != markedInst.ID) {
		result.WriteString(m.buildMiniPreview(selectedInst, halfHeight, previewWidth, "Selected", bottomFocused, bottomScroll))
	} else if selectedInst != nil {
		result.WriteString(dimStyle.Render("  (same as pinned)"))
	}

	return result.String()
}

// buildMiniPreview builds a compact preview for split view
func (m Model) buildMiniPreview(inst *session.Instance, height, width int, label string, focused bool, scrollOffset int) string {
	var preview strings.Builder

	if inst == nil {
		preview.WriteString(dimStyle.Render(fmt.Sprintf("  %s: (none)\n", label)))
		return preview.String()
	}

	// Focus indicator
	focusIndicator := " "
	if focused {
		focusIndicator = titleStyle.Render("▶")
	}

	// Header with session name
	nameStyle := titleStyle
	if inst.Status == session.StatusRunning {
		if m.isActive[inst.ID] {
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrange)).Bold(true)
		} else {
			nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorLightGray)).Bold(true)
		}
	} else {
		nameStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorRed)).Bold(true)
	}
	preview.WriteString(focusIndicator + " ")
	preview.WriteString(nameStyle.Render(inst.Name))
	preview.WriteString("\n")

	// Get preview content for this instance
	content := ""
	if inst.Status == session.StatusRunning {
		// Use scrollContent if scrolling, otherwise fetch normal preview
		if scrollOffset > 0 && m.scrollContent != "" {
			content = m.scrollContent
		} else {
			content, _ = inst.GetPreview(PreviewLineCount)
		}
	}

	maxLines := height - 2 // -2 for header and margin
	if maxLines < 2 {
		maxLines = 2
	}

	if content == "" {
		preview.WriteString(dimStyle.Render("  (no output)"))
		preview.WriteString("\n")
		// Fill remaining lines with empty space
		for i := 1; i < maxLines; i++ {
			preview.WriteString("\n")
		}
		return preview.String()
	}

	// Apply scroll offset (similar to buildPreviewPane)
	lines := strings.Split(content, "\n")
	endIdx := len(lines) - scrollOffset
	if endIdx < maxLines {
		endIdx = maxLines
	}
	if endIdx > len(lines) {
		endIdx = len(lines)
	}
	startIdx := endIdx - maxLines
	if startIdx < 0 {
		startIdx = 0
	}

	// Show scroll indicator at top if not at beginning
	if startIdx > 0 {
		preview.WriteString(dimStyle.Render("   ↑ more"))
		preview.WriteString("\n")
		maxLines-- // Account for indicator line
	}

	displayedLines := 0
	for i := startIdx; i < endIdx && displayedLines < maxLines; i++ {
		line := lines[i]
		// Truncate to available width (width - 2 for left margin)
		maxWidth := width - 2
		if displayWidth(line) > maxWidth {
			line = truncateToWidth(line, maxWidth)
		}
		preview.WriteString("  " + line + "\x1b[0m\n")
		displayedLines++
	}

	// Show scroll indicator at bottom if scrolled
	if scrollOffset > 0 {
		preview.WriteString(dimStyle.Render(fmt.Sprintf("   ↓ more (%d lines)", scrollOffset)))
		preview.WriteString("\n")
		displayedLines++
	}

	// Fill remaining lines with empty space
	for i := displayedLines; i < maxLines; i++ {
		preview.WriteString("\n")
	}

	return preview.String()
}
