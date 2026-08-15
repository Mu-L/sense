package cli

import (
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/luuuc/sense/lab/internal/scenario"
	"github.com/luuuc/sense/lab/internal/score"
)

// The skeleton scores one gold group against one floor. The group and the floor
// are constants here for the same reason the run target is: cycle 04 owns the
// verdict vocabulary, and cycle 02 owns the real matcher.
const (
	defaultGroup = "dependents"
	defaultFloor = 0.50
)

type scoreFlags struct {
	scenario string
	run      string
	group    string
	floor    float64
}

func parseScoreFlags(args []string, stderr io.Writer) (scoreFlags, error) {
	var f scoreFlags
	fs := flag.NewFlagSet("score", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.scenario, "scenario", "", "path to the scenario file (required)")
	fs.StringVar(&f.run, "run", "", "path to the run directory to score (required)")
	fs.StringVar(&f.group, "group", defaultGroup, "gold group to score")
	fs.Float64Var(&f.floor, "floor", defaultFloor, "recall at or above which the run passes")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	for _, required := range []struct{ name, value string }{
		{"-scenario", f.scenario},
		{"-run", f.run},
	} {
		if required.value == "" {
			return f, fmt.Errorf("%s is required", required.name)
		}
	}
	return f, nil
}

// scoreRun reads a recorded run and prints a number and a verdict. It spends
// nothing and touches no network, so it can be re-run over the whole recorded
// corpus whenever the matcher changes.
func scoreRun(args []string, stdout, stderr io.Writer) int {
	f, err := parseScoreFlags(args, stderr)
	if err != nil {
		if err != flag.ErrHelp {
			_, _ = fmt.Fprintf(stderr, "sense-lab score: %v\n", err)
		}
		return exitUsage
	}

	s, err := scenario.Load(f.scenario)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab score: %v\n", err)
		return exitError
	}

	rows, err := goldRows(s, f.group)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab score: %v\n", err)
		return exitError
	}
	if len(rows) == 0 {
		_, _ = fmt.Fprintf(stderr, "sense-lab score: the scenario has no gold group %q\n", f.group)
		return exitError
	}

	text, err := score.AnswerText(filepath.Join(f.run, "raw", "stdout"))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab score: %v\n", err)
		return exitError
	}

	_, _ = fmt.Fprintf(stdout, "repo       %s\nscenario   %s\n", s.Repo, s.Name)
	result := score.Group(f.group, rows, text, f.floor)
	_, _ = fmt.Fprint(stdout, result)

	if result.Verdict != score.AtOrAboveFloor {
		return exitBelowFloor
	}
	return exitOK
}

// goldRows converts one gold group into the scorer's rows.
func goldRows(s scenario.Scenario, group string) ([]score.Row, error) {
	gold, err := s.GoldGroup(group)
	if err != nil {
		return nil, err
	}
	var rows []score.Row
	for _, g := range gold {
		rows = append(rows, score.Row{ID: g.ID, Cite: g.Cite()})
	}
	return rows, nil
}
