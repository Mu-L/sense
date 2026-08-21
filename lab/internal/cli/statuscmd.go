package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/luuuc/sense/lab/internal/status"
)

// defaultCeiling is how many paid runs a repository gets over its lifetime. It
// is here rather than in the status package because a ceiling is a decision
// about a repository, and the page only reports it.
//
// One constant, read by the page and by the refusal in `probe`. A flag that let
// the page state a ceiling the refusal does not enforce would put a number in
// front of a person that nothing is holding.
const defaultCeiling = 40

// statusCmd prints where every repository stands.
//
// It reads the run tree, builds a position and hands it to a printer. It writes
// nothing, decides nothing and takes no verdict: everything on the page is
// derived, so there is no hand-maintained file here to go stale.
func statusCmd(args []string, stdout, stderr io.Writer) int {
	var dir string
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&dir, "runs", defaultRuns, "the root the repositories' run trees live under")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}

	at, err := status.Read(dir, defaultCeiling)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab status: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprint(stdout, status.Render(at, command))
	return exitOK
}
