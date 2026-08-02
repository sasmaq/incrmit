//go:build !windows

// Package testutil holds helpers shared by tests in more than one package.
package testutil

import "syscall"

// Mkfifo creates a named pipe at path. Tests use one to check that incrmit
// refuses to read a non-regular file rather than blocking on it, since opening a
// pipe waits for a writer that never arrives. It returns an error on platforms
// without named pipes so the caller can skip.
func Mkfifo(path string) error {
	return syscall.Mkfifo(path, 0o600)
}
