package ui

// showError displays an error in a dialog and remembers the current state to return to
func (m *Model) showError(err error) {
	m.previousState = m.state
	m.err = err
	m.state = stateError
}
