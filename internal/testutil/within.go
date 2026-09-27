package testutil

import (
	"testing"
	"time"
)

// Deadline is how long Within waits before failing the test.
const Deadline = 10 * time.Second

// Within runs f and fails the test if it has not returned within Deadline.
// Opening a named pipe blocks until a writer appears, so a test that checks a
// FIFO is refused rather than opened would hang the suite on a regression
// instead of failing. what names the call in the failure. f runs on another
// goroutine, so it must report through t.Error rather than t.Fatal.
func Within(t testing.TB, what string, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(Deadline):
		t.Fatalf("%s blocked instead of returning", what)
	}
}
