//go:build testtools

package jobs

// Registered reports whether the job name is registered on the process
// registry, so other packages' tests can check what they register.
func Registered(name string) bool {
	std.mu.Lock()
	defer std.mu.Unlock()
	_, ok := std.jobs[name]
	return ok
}
