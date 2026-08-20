package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/plan"
)

// planCmd prints what a repository's bench would run and every rejection with
// its reason.
//
// Printing is for humans. Later cycles call the planner as a library and take
// the job list as values; nothing anywhere parses this table.
func planCmd(args []string, stdout, stderr io.Writer) int {
	var dir, id string
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configFlag(fs, &dir)
	fs.StringVar(&id, "repo", "", "catalog repo id whose bench to expand (required)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if id == "" {
		_, _ = fmt.Fprintln(stderr, "sense-lab plan: -repo is required")
		return exitUsage
	}

	c, err := catalog.Load(dir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab plan: %v\n", err)
		return exitError
	}
	b, err := loadBench(dir, id)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab plan: %v\n", err)
		return exitError
	}
	res, err := plan.Expand(c, b)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab plan: %v\n", err)
		return exitError
	}

	printPlan(stdout, b, res)

	// A plan that rejects something is not a failure — the whole point is to
	// find it here rather than in a session — but it exits non-zero so an
	// unattended caller cannot proceed as though the matrix were whole.
	if len(res.Rejected) > 0 {
		return exitIncomplete
	}
	return exitOK
}

func printPlan(w io.Writer, b plan.Bench, res plan.Result) {
	_, _ = fmt.Fprintf(w, "repo  %s\njudge %s (pinned, not an arm)\n\n", b.Repo, b.Judge)

	_, _ = fmt.Fprintf(w, "WILL RUN: %d cells, %d sessions\n", res.Cells(), res.Runs())
	for _, j := range res.Jobs {
		_, _ = fmt.Fprintf(w, "  %-12s %s\n", j.Role, j)
	}
	if len(res.Rejected) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "\nWILL NOT RUN: %d\n", len(res.Rejected))
	for _, r := range res.Rejected {
		_, _ = fmt.Fprintf(w, "  %s\n", r)
	}
}

// loadBench reads benches/<id>.json. Unknown fields are refused for the same
// reason the catalog refuses them: a typo'd key that parses silently is a
// setting that looks applied and is not.
//
// It is its own file rather than a block in repos/<id>.json because the two
// have different lifetimes and different authors: what a repository IS is
// written by admission, and how it is measured is written by a person.
func loadBench(dir, id string) (plan.Bench, error) {
	path := filepath.Join(dir, "benches", id+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return plan.Bench{}, fmt.Errorf("read bench: %w", err)
	}
	var b plan.Bench
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return plan.Bench{}, fmt.Errorf("%s: %w", path, err)
	}
	if b.Repo != id {
		return plan.Bench{}, fmt.Errorf("%s declares repo %q but is the bench for %q", path, b.Repo, id)
	}
	return b, nil
}
