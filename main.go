// Command incrmit parses files, discovers semantic version strings, and
// increments them. See doc/DEVELOPMENT.md for the full design.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "incrmit:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	_ = args
	return nil
}
