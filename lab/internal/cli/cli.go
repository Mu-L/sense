// Package cli is the functional core of the sense-lab binary: it takes an
// argument list and two writers and returns a process exit code. Everything the
// binary decides lives here rather than in main, so the whole command surface is
// an ordinary function call in a test.
//
// sense-lab is a development instrument, not part of the Sense product. It may
// reach Sense only through the MCP server, never by importing sense/internal —
// a depguard rule enforces that boundary.
package cli

import (
	"fmt"
	"io"
)

// Version is set at build time via ldflags. The symbol is sense-lab's own,
// because lab may not import sense/internal/version — but the Makefile stamps
// it with the same value it stamps into sense. The separation is of symbols,
// not of version numbers, and nothing should be built on the latter.
var Version = "0.0.0-dev"

const usage = `sense-lab — the bench instrument for Sense

Usage: sense-lab <command>

Commands:
  version   Print version
`

// Exit codes. sense-lab keeps its own table rather than sense's: it is a
// separate binary with a separate surface, and sense already spends 2 on a
// symbol issue.
const (
	exitOK    = 0
	exitUsage = 2
)

// Run dispatches args to a subcommand and returns the process exit code.
// args excludes the program name. Anything that is not a known command prints
// the usage text to stderr and reports a usage error.
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_, _ = fmt.Fprint(stderr, usage)
		return exitUsage
	}
	switch args[0] {
	case "version":
		_, _ = fmt.Fprintln(stdout, Version)
		return exitOK
	case "help", "-h", "--help":
		_, _ = fmt.Fprint(stdout, usage)
		return exitOK
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		_, _ = fmt.Fprint(stderr, usage)
		return exitUsage
	}
}
