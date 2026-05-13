//go:build !windows

package internal

import "syscall"

// ExecClaude replaces the current process with claude (Unix) or
// spawns it as a child and propagates exit code (Windows).
// Exported because main.go (different package) calls it.
func ExecClaude(claudePath string, args []string, env []string) error {
	return syscall.Exec(claudePath, args, env)
}
