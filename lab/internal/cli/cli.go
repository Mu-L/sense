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
	"os"
)

// Version is set at build time via ldflags. The symbol is sense-lab's own,
// because lab may not import sense/internal/version — but the Makefile stamps
// it with the same value it stamps into sense. The separation is of symbols,
// not of version numbers, and nothing should be built on the latter.
var Version = "0.0.0-dev"

const usage = `sense-lab — the bench instrument for Sense

Usage: sense-lab <command> [flags]

Commands:
  next      Do the next thing for a repository, and say what comes after
  why       The whole record behind a repository's position
  pay       Run the paid cells a repository's bench declares — the only command that spends
  catalog   Show the subjects, agents, models, repositories and executors in the config
  plan      Show what a repository's bench would run, and every rejection with its reason
  probe     Run both arms of one cell and prove they differed only in Sense access
  score     Score a recorded run against a scenario's gold
  validate  Audit a scenario's gold and report what would be quarantined
  rescore   Recompute every recorded score and name the cause of each difference
  status    Show where every repository stands, derived from its run tree
  gate      Read a pay decision on stdin and refuse it, or not
  help      concepts — the words this instrument uses, and what each one means
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

	// What a READING command answers with: score, plan, gate. These share
	// numbers with the standing codes below and do not collide, because no
	// command answers with both — a caller reads an exit code with the command
	// it ran in hand, and `score` never parks.
	//
	// exitBelowFloor is its own code for the same reason: a run that scored
	// and came up short is a result, and a caller must be able to tell it from
	// a typo'd path or an unreadable transcript.
	exitBelowFloor = 4
	// exitIncomplete: the bench is well-formed and part of its matrix cannot
	// run. A caller must not proceed on a partial matrix, but "your config is
	// broken" and "three arms cannot run" are opposite actions.
	exitIncomplete = 5
	// exitRefused: a gate refused. It is its own code because "a gate says no"
	// and "the binary broke" are opposite situations for whoever is reading:
	// one is the instrument working, and it is the answer to the question that
	// was asked.
	exitRefused = 7
	// exitProvisional: the transcript this was scored from is known incomplete,
	// so the number is neither a pass nor a failure. Reporting it as either
	// would be the exact misreading the provisional mark exists to prevent.
	exitProvisional = 6

	// The standing codes. They carry where a repository stands, so a shell
	// loop around `next` stops on a park, a PAY, a block or either refusal
	// rather than spinning on any of them, and a park (the method failing here)
	// is told from a PAY (the method working and waiting on a wallet).
	exitFinished = 3
	exitParked   = 4
	exitWaiting  = 5
	exitUnusable = 6
	exitMissing  = 7
	// exitBlocked: a phase reported that the loop cannot go on until a person
	// changes something outside it. Its own code because the act it asks for is
	// its own: not read a transcript, not re-run anything, but edit a file and
	// come back.
	exitBlocked = 9
)

// repoFlags is where a repository is admitted from and where the evidence of it
// lands.
type repoFlags struct {
	config    string
	runs      string
	checkouts string
	senseBin  string
	agent     string
	model     string
	until     bool
	name      string
}

// exitNotIndexed is a scan that produced no index. It is told apart from a
// broken invocation because the artifact naming the shortfall is on disk and
// worth printing, which is a different thing for a caller to say.
const exitNotIndexed = 8

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
	case "probe":
		return probeSignals(args[1:], stdout, stderr)
	case "next":
		// The whole flow in one verb. It clones, scans and spawns, so it dies
		// with the binary like everything else that reaches a process.
		return nextSignals(args[1:], os.Stdin, stdout, stderr)
	case "why":
		return whyRepo(args[1:], stdout, stderr)
	case "pay":
		// The one command that spends. It dies with the binary for the same
		// reason a run does, and for one more: an interrupt that did not reach
		// the arms would leave a cell half run, with money spent on an arm
		// nothing can ever pair.
		return paySignals(args[1:], os.Stdin, stdout, stderr)
	case "tee":
		// Deliberately absent from the usage text. It is what a run's
		// .mcp.json points at instead of the Sense server, not a verb a person
		// types.
		return teeServer(args[1:], os.Stdin, stdout, stderr)
	case "gate":
		return gateCmd(args[1:], os.Stdin, stdout, stderr)
	case "status":
		return statusCmd(args[1:], stdout, stderr)
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
		return helpCmd(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "unknown command: %s\n\n", args[0])
		_, _ = fmt.Fprint(stderr, usage)
		return exitUsage
	}
}
