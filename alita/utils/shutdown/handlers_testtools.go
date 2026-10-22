//go:build testtools

package shutdown

// ExecuteHandlersForTest runs the registered handlers in the same LIFO order
// and with the same panic recovery as shutdown, without exiting the process.
// It exists so tests outside this package can assert the relative order of
// the drains main.go registers.
func (m *Manager) ExecuteHandlersForTest() {
	m.mu.RLock()
	handlers := make([]func() error, len(m.handlers))
	copy(handlers, m.handlers)
	m.mu.RUnlock()

	for i := len(handlers) - 1; i >= 0; i-- {
		_ = m.executeHandler(handlers[i], i)
	}
}
