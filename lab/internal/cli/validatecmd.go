package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/luuuc/sense/lab/internal/goldcheck"
	"github.com/luuuc/sense/lab/internal/ground"
	"github.com/luuuc/sense/lab/internal/scenario"
)

type validateFlags struct {
	scenario string
	checkout string
	commit   string
	results  string
}

func parseValidateFlags(args []string, stderr io.Writer) (validateFlags, error) {
	var f validateFlags
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.scenario, "scenario", "", "path to the scenario file (required)")
	fs.StringVar(&f.checkout, "checkout", "", "repository checkout to resolve gold against")
	fs.StringVar(&f.commit, "commit", "", "commit the checkout is pinned at (required with -checkout)")
	fs.StringVar(&f.results, "results", "", "recorded results tree, to list the runs a quarantine moves")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	if f.scenario == "" {
		return f, fmt.Errorf("-scenario is required")
	}
	if f.checkout != "" && f.commit == "" {
		return f, fmt.Errorf("-checkout needs -commit: resolving gold against an unpinned tree would " +
			"check it against whatever happens to be checked out")
	}
	return f, nil
}

// validateGold audits a scenario's gold and reports what it found.
//
// It REPORTS. Quarantine and report; a validator that also edits gold is a
// validator nobody can check, and fixing a row is a decision to take with the
// scenario in front of you.
func validateGold(args []string, stdout, stderr io.Writer) int {
	f, err := parseValidateFlags(args, stderr)
	if err != nil {
		if err != flag.ErrHelp {
			_, _ = fmt.Fprintf(stderr, "sense-lab validate: %v\n", err)
		}
		return exitUsage
	}
	set, err := scenario.LoadPath(f.scenario)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab validate: %v\n", err)
		return exitError
	}

	var checkout *ground.Checkout
	if f.checkout != "" {
		if checkout, err = ground.Open(f.checkout, f.commit); err != nil {
			_, _ = fmt.Fprintf(stderr, "sense-lab validate: resolving skipped: %v\n", err)
			checkout = nil
		}
	}
	// A nil *Checkout must go in as a nil INTERFACE, or the audit sees a
	// non-nil interface holding a nil pointer and believes it can resolve.
	report := goldcheck.Audit(set, resolverOrNil(checkout), grepperOrNil(checkout))
	_, _ = fmt.Fprint(stdout, report.String())

	if f.results != "" && len(report.Quarantined) > 0 {
		printAffectedRuns(set, f.results, stdout, stderr)
	}
	if len(report.Quarantined) > 0 {
		return exitBelowFloor
	}
	return exitOK
}

func resolverOrNil(c *ground.Checkout) goldcheck.Resolver {
	if c == nil {
		return nil
	}
	return c
}

func grepperOrNil(c *ground.Checkout) goldcheck.Grepper {
	if c == nil {
		return nil
	}
	return c
}

// printAffectedRuns lists the recorded runs whose scores a quarantine moves.
//
// Removing a row from a group changes that group's recall in every run that
// scored against it. The rescore proof lists "gold change" as a cause of a
// difference, and it should meet that cause as a prediction made HERE rather
// than as a surprise found there.
func printAffectedRuns(set scenario.Set, root string, stdout, stderr io.Writer) {
	runs, err := scoredRuns(root, set.Scenario.Repo)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab validate: could not list recorded runs: %v\n", err)
		return
	}
	_, _ = fmt.Fprintf(stdout, "\n  the quarantine moves the recorded score of %d runs:\n", len(runs))
	for _, run := range runs {
		_, _ = fmt.Fprintf(stdout, "    %s\n", run)
	}
}

// scoredRuns finds the recorded runs for one repo that carry a score.
func scoredRuns(root, repo string) ([]string, error) {
	var out []string
	err := filepath.Walk(root, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() || fi.Name() != "scored.json" {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/"+repo+"/") {
			return nil
		}
		// Rel cannot fail here: path came from walking root, so it is under it.
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		out = append(out, rel)
		return nil
	})
	sort.Strings(out)
	return out, err
}
