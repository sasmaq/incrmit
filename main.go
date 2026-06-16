// Command incrmit parses files, discovers semantic version strings, and
// increments them. See doc/DEVELOPMENT.md for the full design.
package main

import (
	"os"

	"github.com/sasmaq/incrmit/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdout, os.Stderr))
}
