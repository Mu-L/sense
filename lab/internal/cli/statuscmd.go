package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/luuuc/sense/lab/internal/status"
)

// defaultCeiling is how many paid runs a campaign gets over its lifetime unless
// it is told otherwise. It is here rather than in the status package because a
// ceiling is a decision about a campaign, and the page only reports it.
const defaultCeiling = 40

// statusCmd prints where a campaign stands.
//
// It reads the run tree, builds a position and hands it to a printer. It writes
// nothing, decides nothing and takes no verdict: everything on the page is
// derived, so there is no hand-maintained file here to go stale.
func statusCmd(args []string, stdout, stderr io.Writer) int {
	var dir string
	var ceiling int
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&dir, "campaign", "", "the campaign's run tree")
	fs.IntVar(&ceiling, "ceiling", defaultCeiling, "paid runs this campaign gets over its lifetime")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if dir == "" {
		_, _ = fmt.Fprintln(stderr, "sense-lab status: -campaign names the run tree to report on")
		return exitUsage
	}

	at, err := status.Read(dir, ceiling)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab status: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprint(stdout, status.Render(at))
	return exitOK
}
