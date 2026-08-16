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
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// Version is set at build time via ldflags. The symbol is sense-lab's own,
// because lab may not import sense/internal/version — but the Makefile stamps
// it with the same value it stamps into sense. The separation is of symbols,
// not of version numbers, and nothing should be built on the latter.
var Version = "0.0.0-dev"

const usage = `sense-lab — the bench instrument for Sense

Usage: sense-lab <command> [flags]

Commands:
  catalog   Show the subjects, agents, models, repositories and executors in the config
  plan      Show what a campaign would run, and every rejection with its reason
  run       Run one scenario against one repository
  score     Score a recorded run against a scenario's gold
  validate  Audit a scenario's gold and report what would be quarantined
  rescore   Recompute every recorded score and name the cause of each difference
  version   Print version
`

// Exit codes. sense-lab keeps its own table rather than sense's: it is a
// separate binary with a separate surface, and sense already spends 2 on a
// symbol issue.
//
// A run that failed or could not finish inside its budget exits 1: it is a
// result, and reporting it as success would let a stalled arm pass unnoticed.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
	// exitCannotFinish is its own code so a caller can tell a bankable result
	// from a broken binary without parsing JSON: a run that hit its wall left a
	// record on disk, a run that exited 1 may not have.
	exitCannotFinish = 3
	// exitBelowFloor is its own code for the same reason: a run that scored
	// and came up short is a result, and a caller must be able to tell it from
	// a typo'd path or an unreadable transcript.
	exitBelowFloor = 4
	// exitIncomplete: the campaign is well-formed and part of its matrix cannot
	// run. A caller must not proceed on a partial matrix, but "your config is
	// broken" and "three arms cannot run" are opposite actions.
	exitIncomplete = 5
	// exitProvisional: the transcript this was scored from is known incomplete,
	// so the number is neither a pass nor a failure. Reporting it as either
	// would be the exact misreading the provisional mark exists to prevent.
	exitProvisional = 6
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
	case "plan":
		return planCmd(args[1:], stdout, stderr)
	case "run":
		// A run must die with the binary. Without this the cancel path the
		// runner carefully distinguishes can never fire in production:
		// interrupting a campaign would leave the agent running, unattended,
		// unowned and still spending, with no record on disk — and because the
		// session is in its own process group, a second Ctrl-C cannot reach it
		// either.
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return runSession(ctx, args[1:], stdout, stderr)
	case "catalog":
		return catalogCmd(args[1:], stdout, stderr)
	case "score":
		return scoreRun(args[1:], stdout, stderr)
	case "validate":
		return validateGold(args[1:], stdout, stderr)
	case "rescore":
		return rescoreRuns(args[1:], stdout, stderr)
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
