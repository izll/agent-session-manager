package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// colorPickerView renders the color picker dialog
func (m Model) colorPickerView() string {
	var b strings.Builder

	// Title based on what we're editing
	if m.editingGroup != nil {
		b.WriteString(titleStyle.Render(" Group Color "))
	} else {
		b.WriteString(titleStyle.Render(" Session Color "))
	}
	b.WriteString("\n\n")

	// Editing a group
	if m.editingGroup != nil {
		group := m.editingGroup

		// Get preview colors (current cursor selection for active mode)
		previewFg := m.previewFg
		previewBg := m.previewBg
		filteredColors := m.getFilteredColorOptions()
		if m.colorCursor < len(filteredColors) {
			selected := filteredColors[m.colorCursor]
			if m.colorMode == 0 {
				previewFg = selected.Color
			} else {
				previewBg = selected.Color
			}
		}

		// Show group name with preview colors
		styledName := group.Name
		nameStyle := lipgloss.NewStyle().Bold(true)
		hasBg := previewBg != "" && previewBg != "none"
		if hasBg {
			nameStyle = nameStyle.Background(lipgloss.Color(previewBg))
		}
		if previewFg != "" && previewFg != "none" && previewFg != "auto" {
			nameStyle = nameStyle.Foreground(lipgloss.Color(previewFg))
		} else if hasBg {
			// Auto or empty foreground with background - use contrast
			nameStyle = nameStyle.Foreground(lipgloss.Color(getContrastColor(previewBg)))
		}
		styledName = nameStyle.Render(group.Name)

		b.WriteString(fmt.Sprintf("  Group: 📁 %s\n", styledName))

		// Show current colors
		fgDisplay := "none"
		if group.Color != "" {
			fgDisplay = group.Color
		}
		bgDisplay := "none"
		if group.BgColor != "" {
			bgDisplay = group.BgColor
		}
		fullRowDisplay := "OFF"
		if group.FullRowColor {
			fullRowDisplay = "ON"
		}

		// Highlight active mode
		if m.colorMode == 0 {
			b.WriteString(fmt.Sprintf("  [Text: %s]  Background: %s\n", fgDisplay, bgDisplay))
		} else {
			b.WriteString(fmt.Sprintf("   Text: %s  [Background: %s]\n", fgDisplay, bgDisplay))
		}
		b.WriteString(fmt.Sprintf("  Full row: %s (f)\n", fullRowDisplay))
		b.WriteString(dimStyle.Render("  TAB: switch | f: full row"))
		b.WriteString("\n\n")
	} else if inst := m.getSelectedInstance(); inst != nil {
		// Get preview colors (current cursor selection for active mode)
		previewFg := m.previewFg
		previewBg := m.previewBg
		filteredColors := m.getFilteredColorOptions()
		if m.colorCursor < len(filteredColors) {
			selected := filteredColors[m.colorCursor]
			if m.colorMode == 0 {
				previewFg = selected.Color
			} else {
				previewBg = selected.Color
			}
		}

		// Show session name with preview colors
		styledName := inst.Name
		nameStyle := lipgloss.NewStyle()
		if previewBg != "" {
			nameStyle = nameStyle.Background(lipgloss.Color(previewBg))
		}
		if previewFg != "" {
			if previewFg == "auto" && previewBg != "" {
				nameStyle = nameStyle.Foreground(lipgloss.Color(getContrastColor(previewBg)))
			} else if _, isGradient := gradients[previewFg]; isGradient {
				if previewBg != "" {
					styledName = applyGradientWithBg(inst.Name, previewFg, previewBg)
				} else {
					styledName = applyGradient(inst.Name, previewFg)
				}
			} else {
				nameStyle = nameStyle.Foreground(lipgloss.Color(previewFg))
			}
		} else if previewBg != "" {
			nameStyle = nameStyle.Foreground(lipgloss.Color(getContrastColor(previewBg)))
		}
		if styledName == inst.Name {
			styledName = nameStyle.Render(inst.Name)
		}

		b.WriteString(fmt.Sprintf("  Session: %s\n", styledName))

		// Show current colors
		fgDisplay := "none"
		if inst.Color != "" {
			fgDisplay = inst.Color
		}
		bgDisplay := "none"
		if inst.BgColor != "" {
			bgDisplay = inst.BgColor
		}
		fullRowDisplay := "OFF"
		if inst.FullRowColor {
			fullRowDisplay = "ON"
		}

		// Highlight active mode
		if m.colorMode == 0 {
			b.WriteString(fmt.Sprintf("  [Text: %s]  Background: %s\n", fgDisplay, bgDisplay))
		} else {
			b.WriteString(fmt.Sprintf("   Text: %s  [Background: %s]\n", fgDisplay, bgDisplay))
		}
		b.WriteString(fmt.Sprintf("  Full row: %s (f)\n", fullRowDisplay))
		b.WriteString(dimStyle.Render("  TAB: switch | f: full row"))
		b.WriteString("\n\n")
	}

	// Calculate max items based on mode
	maxItems := m.getMaxColorItems()

	// Calculate visible window
	maxVisible := m.height - ColorPickerHeader
	if maxVisible < MinColorPickerRows {
		maxVisible = MinColorPickerRows
	}

	startIdx := 0
	if m.colorCursor >= maxVisible {
		startIdx = m.colorCursor - maxVisible + 1
	}
	endIdx := startIdx + maxVisible
	if endIdx > maxItems {
		endIdx = maxItems
	}

	// Show scroll indicator at top
	if startIdx > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more\n", startIdx)))
	}

	// Get filtered list of color options for current mode
	filteredOptions := m.getFilteredColorOptions()

	for displayIdx := startIdx; displayIdx < endIdx && displayIdx < len(filteredOptions); displayIdx++ {
		c := filteredOptions[displayIdx]

		// Create color preview
		var colorPreview string
		if c.Color == "" {
			if m.colorMode == 0 {
				colorPreview = "      none"
			} else {
				colorPreview = "       none" // Extra space for background mode
			}
		} else if c.Color == "auto" {
			colorPreview = " ✨   auto"
		} else if _, isGradient := gradients[c.Color]; isGradient {
			// Show gradient preview
			colorPreview = " " + applyGradient("████", c.Color) + " " + c.Name
		} else {
			style := lipgloss.NewStyle()
			if m.colorMode == 0 {
				style = style.Foreground(lipgloss.Color(c.Color))
				colorPreview = style.Render(" ████ ") + c.Name
			} else {
				style = style.Background(lipgloss.Color(c.Color))
				// For background, show solid block with contrast text
				textColor := getContrastColor(c.Color)
				style = style.Foreground(lipgloss.Color(textColor))
				colorPreview = style.Render("      ") + " " + c.Name
			}
		}

		if displayIdx == m.colorCursor {
			b.WriteString(fmt.Sprintf("  ❯%s\n", colorPreview))
		} else {
			b.WriteString(fmt.Sprintf("   %s\n", colorPreview))
		}
	}

	// Show scroll indicator at bottom
	remaining := maxItems - endIdx
	if remaining > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more\n", remaining)))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  enter: select  esc: cancel"))

	return b.String()
}
