package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/luuuc/sense/lab/internal/run"
	"github.com/luuuc/sense/lab/internal/scenario"
	"github.com/luuuc/sense/lab/internal/score"
	"github.com/luuuc/sense/lab/internal/transcript"
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

	tr, err := transcript.ReadClaudeCode(filepath.Join(f.run, "raw", "stdout"))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab score: %v\n", err)
		return exitError
	}
	src := scoredRun{tr: tr, why: runWasNotCompleted(f.run)}

	_, _ = fmt.Fprintf(stdout, "repo       %s\nscenario   %s\n", s.Repo, s.Name)
	// The transcript goes in, not its text, so the provisional mark travels
	// with the number rather than depending on this caller to copy it.
	result := score.Group(f.group, rows, src, f.floor)
	_, _ = fmt.Fprint(stdout, result)

	// A provisional number is not a verdict either way, so it never reports as
	// a clean pass or a clean failure.
	if result.Provisional() {
		return exitProvisional
	}
	if result.Verdict != score.AtOrAboveFloor {
		return exitBelowFloor
	}
	return exitOK
}

// scoredRun is the transcript plus what the RUNNER recorded about the session.
//
// The runner knows whether it killed the session; the transcript can only be
// re-inferred from bytes, and that inference is weaker. A run recorded as
// having hit its wall or been interrupted is provisional whatever its capture
// looks like, and the record was sitting unread beside it.
type scoredRun struct {
	tr  transcript.Transcript
	why string
}

func (s scoredRun) Answer() string { return s.tr.Answer() }

// ProvisionalWhy prefers the runner's own account, because it is a fact rather
// than an inference.
func (s scoredRun) ProvisionalWhy() string {
	if s.why != "" {
		return s.why
	}
	return s.tr.ProvisionalWhy()
}

// runWasNotCompleted reads the run's own record and reports why it is not a
// finished session, or "" when it is — or when there is no record to read,
// which is the cycle-00 shape and not itself evidence of anything.
func runWasNotCompleted(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "run-meta.json"))
	if err != nil {
		return ""
	}
	var m struct {
		Outcome     string `json:"outcome"`
		StdoutBytes int64  `json:"stdout_bytes"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return "the run record beside this capture could not be read"
	}
	if m.Outcome != "" && m.Outcome != string(run.Completed) {
		return "the runner recorded this session as " + m.Outcome
	}
	return ""
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
