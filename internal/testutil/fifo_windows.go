//go:build windows

// Package testutil holds helpers shared by tests in more than one package.
package testutil

import "errors"

// Mkfifo reports that named pipes cannot be created this way on Windows, so
// tests that need one skip there.
func Mkfifo(string) error {
	return errors.New("named pipes are not created this way on Windows")
}
