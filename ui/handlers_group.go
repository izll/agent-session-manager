package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/izll/agent-session-manager/session"
)

// handleNewGroupKeys handles keyboard input in the new group dialog
func (m Model) handleNewGroupKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = stateList
		return m, nil
	case "enter":
		if m.groupInput.Value() != "" {
			group, err := m.storage.AddGroup(m.groupInput.Value())
			if err != nil {
				m.err = err
			} else {
				m.groups = append(m.groups, group)
				m.buildVisibleItems()
			}
			m.state = stateList
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.groupInput, cmd = m.groupInput.Update(msg)
	return m, cmd
}

// handleRenameGroupKeys handles keyboard input in the rename group dialog
func (m Model) handleRenameGroupKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.state = stateList
		return m, nil
	case "enter":
		if m.groupInput.Value() != "" {
			m.buildVisibleItems()
			if m.cursor >= 0 && m.cursor < len(m.visibleItems) {
				item := m.visibleItems[m.cursor]
				if item.isGroup {
					if err := m.storage.RenameGroup(item.group.ID, m.groupInput.Value()); err != nil {
						m.err = err
					} else {
						item.group.Name = m.groupInput.Value()
					}
				}
			}
			m.state = stateList
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.groupInput, cmd = m.groupInput.Update(msg)
	return m, cmd
}

// handleSelectGroupKeys handles keyboard input in the group selection dialog
func (m Model) handleSelectGroupKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	maxIdx := len(m.groups) // 0 = ungrouped, 1+ = groups

	switch msg.String() {
	case "esc":
		m.state = stateList
		return m, nil

	case "up", "k":
		if m.groupCursor > 0 {
			m.groupCursor--
		}

	case "down", "j":
		if m.groupCursor < maxIdx {
			m.groupCursor++
		}

	case "enter":
		// Find current session (works in both grouped and ungrouped modes)
		var inst *session.Instance
		if len(m.groups) > 0 {
			m.buildVisibleItems()
			if m.cursor >= 0 && m.cursor < len(m.visibleItems) {
				item := m.visibleItems[m.cursor]
				if !item.isGroup {
					inst = item.instance
				}
			}
		} else if len(m.instances) > 0 && m.cursor < len(m.instances) {
			inst = m.instances[m.cursor]
		}

		if inst != nil {
			var groupID string
			if m.groupCursor > 0 && m.groupCursor <= len(m.groups) {
				groupID = m.groups[m.groupCursor-1].ID
			}
			inst.GroupID = groupID
			m.storage.UpdateInstance(inst)
			m.buildVisibleItems()
		}
		m.state = stateList
		return m, nil
	}

	return m, nil
}
