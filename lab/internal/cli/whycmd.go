package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/luuuc/sense/lab/internal/position"
)

// whyRepo prints the whole record behind a repository's position: every verdict
// it has emitted, oldest first, unabridged.
//
// It exists so that the page a person meets can be six lines. The same string
// used to serve two readers with opposite needs — the phase agent, which is
// handed every prior rejection because reading only the latest is how six
// attempts oscillated between two failures, and the operator at a terminal, who
// needs to know where they stand. The agent still gets all of it, in the prompt
// the crank composes. This is where a person asks for the same thing on
// purpose.
func whyRepo(args []string, stdout, stderr io.Writer) int {
	var runs string
	fs := flag.NewFlagSet("why", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&runs, "runs", defaultRuns, "the root the repositories' run trees live under")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if fs.NArg() != 1 {
		_, _ = fmt.Fprint(stderr, "sense-lab why: name exactly one repository\n")
		return exitUsage
	}

	at, err := position.Read(runs, fs.Arg(0))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab why: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprint(stdout, position.Render(at))
	return exitOK
}
