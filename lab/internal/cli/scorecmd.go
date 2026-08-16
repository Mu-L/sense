package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/luuuc/sense/lab/internal/ground"
	"github.com/luuuc/sense/lab/internal/run"
	"github.com/luuuc/sense/lab/internal/scenario"
	"github.com/luuuc/sense/lab/internal/score"
	"github.com/luuuc/sense/lab/internal/transcript"
)

// The skeleton scores one gold group against one floor. The floor is a constant
// here for the same reason the run target is: cycle 04 owns the verdict
// vocabulary, and cycle 02 owns the real matcher.
//
// The GROUP is not a constant. It defaults to the scenario's own declared
// discriminator, because a hardcoded "dependents" here would mean a gold file
// naming a different discriminator was silently ignored — the exact failure the
// field was added to end, with a YAML key in front of it.
const defaultFloor = 0.50

// allGroups is what -group takes to score every gold group instead of one.
const allGroups = "all"

type scoreFlags struct {
	scenario string
	run      string
	group    string
	floor    float64
	checkout string
	commit   string
}

func parseScoreFlags(args []string, stderr io.Writer) (scoreFlags, error) {
	var f scoreFlags
	fs := flag.NewFlagSet("score", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.scenario, "scenario", "", "path to the scenario file (required)")
	fs.StringVar(&f.run, "run", "", "path to the run directory to score (required)")
	fs.StringVar(&f.group, "group", "", "gold group to score, or \"all\" (default: the scenario's discriminator)")
	fs.Float64Var(&f.floor, "floor", defaultFloor, "recall at or above which the run passes")
	fs.StringVar(&f.checkout, "checkout", "", "repository checkout to verify citations against")
	fs.StringVar(&f.commit, "commit", "", "commit the checkout is pinned at (required with -checkout)")
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
	// A missing checkout is a deferral. A checkout given WITHOUT its commit is
	// a typo, and downgrading it to unverified would exit 0 on a stderr line
	// the operator may well be piping away.
	if f.checkout != "" && f.commit == "" {
		return f, errors.New("-checkout needs -commit: grounding against an unpinned tree would " +
			"check citations against whatever happens to be checked out")
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

	set, err := scenario.LoadPath(f.scenario)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab score: %v\n", err)
		return exitError
	}

	tr, err := transcript.ReadClaudeCode(filepath.Join(f.run, "raw", "stdout"))
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab score: %v\n", err)
		return exitError
	}
	src := scoredRun{tr: tr, why: runWasNotCompleted(f.run)}
	_, _ = fmt.Fprintf(stdout, "repo       %s\nscenario   %s\n", set.Scenario.Repo, set.Scenario.Name)

	if f.group == allGroups {
		return scoreEveryGroup(set, src, f, stdout, stderr)
	}

	group := f.group
	if group == "" {
		group = set.Gold.Discriminator
	}
	rows, err := goldRows(set, group)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab score: %v\n", err)
		return exitError
	}
	if len(rows) == 0 {
		_, _ = fmt.Fprintf(stderr, "sense-lab score: the scenario has no gold group %q\n", group)
		return exitError
	}

	// The transcript goes in, not its text, so the provisional mark travels
	// with the number rather than depending on this caller to copy it.
	result := score.GroupCites(group, rows, score.Scan(src.Answer()), src.ProvisionalWhy(), f.floor)
	result.Grounding = groundReport(src.Answer(), f, rows, stderr).String()
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
func goldRows(set scenario.Set, group string) ([]score.Row, error) {
	gold, err := set.Gold.Group(group)
	if err != nil {
		return nil, err
	}
	var rows []score.Row
	for _, g := range gold {
		rows = append(rows, score.Row{ID: g.ID, Cite: g.Cite()})
	}
	return rows, nil
}

// scoreEveryGroup scores all five gold groups against one run.
//
// The discriminator carries the headline, but a margin that sits entirely in
// one group and a margin spread across all of them are different results, and
// reporting only the discriminator cannot tell them apart. The exit code is the
// discriminator's, because that is still the number the floor applies to.
func scoreEveryGroup(set scenario.Set, src score.Source, f scoreFlags, stdout, stderr io.Writer) int {
	code := exitError
	cites := score.Scan(src.Answer())
	report := groundReport(src.Answer(), f, allRows(set), stderr)
	for _, group := range set.Gold.Groups() {
		rows, err := goldRows(set, group)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "sense-lab score: %v\n", err)
			return exitError
		}
		result := score.GroupCites(group, rows, cites, src.ProvisionalWhy(), f.floor)
		result.Grounding = report.String()
		_, _ = fmt.Fprint(stdout, result)
		if group != set.Gold.Discriminator {
			continue
		}
		switch {
		case result.Provisional():
			code = exitProvisional
		case result.Verdict != score.AtOrAboveFloor:
			code = exitBelowFloor
		default:
			code = exitOK
		}
	}
	return code
}

// groundReport verifies the answer's citations against a checkout when one is
// given.
//
// No checkout is not an error. It downgrades the result to unverified and says
// so: a scorer that refused to score without a git checkout would make the
// whole pure layer depend on the state of a disk, and three of the four benched
// repositories are not cloned on this machine today.
//
// A checkout that is the WRONG one is different, and it is the dangerous case:
// a commit that exists in another repository passes every existence check and
// then resolves nothing, so every citation reads as fabricated and the report
// says verified. The gold rows are the detector — they have known locations at
// the pinned commit — and grounding refuses rather than publishing that.
func groundReport(answer string, f scoreFlags, rows []score.Row, stderr io.Writer) ground.Report {
	if f.checkout == "" {
		return ground.Check(nil, nil)
	}
	c, err := ground.Open(f.checkout, f.commit)
	if err == nil {
		err = ground.CheckGold(locations(rows), c)
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab score: grounding skipped: %v\n", err)
		return ground.Check(nil, nil)
	}
	return ground.Check(score.Scan(answer), c)
}

func locations(rows []score.Row) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.Cite != "" {
			out = append(out, r.Cite)
		}
	}
	return out
}

// allRows is every gold row in the set, used to verify the checkout when all
// groups are scored at once.
func allRows(set scenario.Set) []score.Row {
	var out []score.Row
	for _, g := range set.Gold.Rows {
		if c := g.Cite(); c != "" {
			out = append(out, score.Row{ID: g.ID, Cite: c})
		}
	}
	return out
}
