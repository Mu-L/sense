// Package position answers the question the crank has to answer before it does
// anything: what is the next phase for this repository, and how many authoring
// cycles has it already spent.
//
// It answers it from two inputs, and which input a fact comes from is the whole
// design.
//
// The run tree holds which runs exist, what they wrote, and the authoring
// cycle — which is already a directory name under `<runs>/<repo>/<cycle>/`.
// Those are DERIVED, every time, and never stored. A counter beside the fact it
// counts is a counter that drifts from it, and the drift is invisible.
//
// The attempt records hold the verdict of each attempt, the table that rejected
// it, and the anchor. Those are RECORDED, once, and never re-derived: a verdict
// is a judgment, and no amount of reading a tree recovers one.
//
// The old driver held the same answer in a JSON map of repository to phase,
// edited by whoever was debugging. That is a decision anyone can change by hand,
// which made it the one number in the system with no evidence behind it.
//
// # It reports and never repairs
//
// Nothing here re-scores, re-judges, or tidies a run tree it finds odd. An odd
// tree is reported as odd. There is no clock, no network and no subprocess, so
// the same tree gives the same answer on any machine on any day.
package position

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/luuuc/sense/lab/internal/phase"
)

// attemptsDir is where a repository's judgments live, beside its cycles rather
// than inside one: an attempt names its cycle, and a reader that had to walk
// every cycle directory to collect them would miss the ones whose cycle
// directory does not exist.
const attemptsDir = "attempts"

// Attempt is one phase attempt and what it decided.
//
// It carries the judgment and nothing derivable. There is no timestamp: the
// order of attempts is (cycle, phase, try), which the tree and the graph
// already fix, and a clock in a record is one more thing to be wrong on a
// machine whose clock is.
type Attempt struct {
	Cycle   int           `json:"cycle"`
	Phase   phase.Name    `json:"phase"`
	Verdict phase.Verdict `json:"verdict"`
	// Try distinguishes two attempts at one phase in one cycle, which the graph
	// allows: a DoD FAIL sends a harvest back to report inside the same cycle.
	// It is 1-based and it is the caller's, because only the caller knows
	// whether this is the same attempt being written twice or a new one.
	Try int `json:"try"`
	// Step is where this attempt sits in the order its cycle happened in,
	// 1-based, assigned when it is recorded.
	//
	// It exists because a cycle is not a walk down the graph. A re-entry sends
	// work BACK: cycle 1 can hold an author, then a mini-bench that re-questions
	// it, then the author again — and ordering those by where their phases sit
	// in the graph puts the mini-bench last, which makes the position route from
	// the verdict that sent work back rather than from the attempt that
	// answered it. Measured: the loop then re-ran the author on every turn,
	// forever, at whatever an agent costs.
	//
	// It is recorded rather than derived because there is nothing to derive it
	// from. The order judgments happened in is known only to whoever was there,
	// and this package refuses a clock.
	Step int `json:"step"`
	// Table is what rejected this attempt: the mini-bench read, the credit
	// table that refused to pay, the reason a draft had no anchor. It is what
	// the next attempt has to answer, and a re-entry without one is a fresh
	// guess wearing the previous attempt's number.
	Table  string `json:"table,omitempty"`
	Anchor string `json:"anchor,omitempty"`
	// Outcome is how the attempt ended when it did not end in a verdict. Empty
	// is the ordinary case: the phase decided, and Verdict says what.
	//
	// The two that are not empty are the two ways an agent fails to produce a
	// judgment, and they are told apart because they are diagnosed differently.
	Outcome Outcome `json:"outcome,omitempty"`
	// Plan, VerdictDoc, Artifact and Log are what this attempt READ and left
	// behind, as paths. They are recorded rather than derived because a park
	// opened six months later should show the chain that produced it rather
	// than a phase name and a date.
	//
	// Artifact is empty when the phase wrote none, which is a fact about the
	// attempt and not a gap in the record.
	Plan       string `json:"plan,omitempty"`
	VerdictDoc string `json:"verdict_doc,omitempty"`
	Artifact   string `json:"artifact,omitempty"`
	Log        string `json:"log,omitempty"`
}

// Outcome is how an attempt ended when it produced no verdict.
type Outcome string

const (
	// Stalled: the agent ran past its wall. Cannot-finish-at-budget is a
	// result, so it is recorded as one rather than waited on.
	Stalled Outcome = "stalled"
	// Refused: the agent finished and what it wrote is not a verdict for this
	// phase. An agent that misbehaved, against an agent that died.
	Refused Outcome = "refused"
)

// Name is the attempt's file, and it is the attempt's identity.
func (a Attempt) Name() string { return fmt.Sprintf("%d-%s-%d.json", a.Cycle, a.Phase, a.Try) }

// Record writes one attempt, and refuses to write over one.
//
// The refusal is the open flag rather than a check, because append-only as a
// convention survives until a bad night and append-only as an O_EXCL survives
// the bad night. A verdict that has been recorded is history, and history is
// lost on purpose or not at all.
func Record(repoDir string, a Attempt) error {
	if err := a.valid(); err != nil {
		return err
	}
	// The step is assigned here, from what is already recorded, because this is
	// the moment the order is known. A caller cannot be trusted to hold a
	// counter: one that did would drift the first time an attempt was recorded
	// by anything else.
	recorded, err := Attempts(repoDir)
	if err != nil {
		return err
	}
	a.Step = nextStep(recorded, a.Cycle)
	dir := filepath.Join(repoDir, attemptsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("prepare %s: %w", dir, err)
	}
	b, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return fmt.Errorf("render the attempt: %w", err)
	}
	path := filepath.Join(dir, a.Name())
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("record %s %s try %d: %w. A recorded verdict is not rewritten: "+
			"if this is a new attempt it is a new try", a.Phase, a.Verdict, a.Try, err)
	}
	defer func() { _ = f.Close() }()
	_, err = f.Write(append(b, '\n'))
	return err
}

// valid refuses an attempt that could not be read back as a judgment.
func (a Attempt) valid() error {
	switch {
	case a.Cycle < 1:
		return fmt.Errorf("attempt at %s is in cycle %d; cycles are 1-based", a.Phase, a.Cycle)
	case a.Try < 1:
		return fmt.Errorf("attempt at %s is try %d; tries are 1-based", a.Phase, a.Try)
	case a.Phase == "":
		return fmt.Errorf("an attempt with no phase: nothing could say what was decided")
	case a.Verdict == "" && a.Outcome == "":
		return fmt.Errorf("attempt at %s emitted nothing and says nothing about how it ended; "+
			"a phase that decided nothing did not finish, and that is itself a result to record", a.Phase)
	}
	return nil
}

// nextStep is where the next attempt in a cycle sits in that cycle's order.
func nextStep(recorded []Attempt, cycle int) int {
	next := 1
	for _, a := range recorded {
		if a.Cycle == cycle && a.Step >= next {
			next = a.Step + 1
		}
	}
	return next
}

// NextTry is the try number an attempt at this phase and cycle should carry.
//
// It is a pure function of what is already recorded rather than a counter, and
// it is separate from [Record] on purpose: it says what the next attempt would
// be, and [Record] still refuses if something took that number in between.
func NextTry(attempts []Attempt, cycle int, p phase.Name) int {
	next := 1
	for _, a := range attempts {
		if a.Cycle == cycle && a.Phase == p && a.Try >= next {
			next = a.Try + 1
		}
	}
	return next
}

// Attempts reads every attempt recorded for a repository, in the order they
// happened: by cycle, then by the step recorded with each attempt.
//
// The order comes from the record rather than from a timestamp, so it is the
// same order on a machine whose clock is wrong — and rather than from the
// graph, because a cycle that re-enters a phase did not walk the graph in order.
func Attempts(repoDir string) ([]Attempt, error) {
	entries, err := os.ReadDir(filepath.Join(repoDir, attemptsDir))
	if err != nil {
		if os.IsNotExist(err) {
			// A repository nothing has decided about yet. That is a position,
			// not a failure.
			return nil, nil
		}
		return nil, fmt.Errorf("read the attempts of %s: %w", repoDir, err)
	}
	var out []Attempt
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(repoDir, attemptsDir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var a Attempt
		if err := json.Unmarshal(b, &a); err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		out = append(out, a)
	}
	slices.SortFunc(out, order)
	return out, nil
}

// order sorts attempts into the order they happened in.
//
// The step decides. The graph is the tiebreak, for a record written by hand
// with no step in it: a forward walk is in graph order, so that is the best
// guess available and it is only ever a guess.
func order(a, b Attempt) int {
	if a.Cycle != b.Cycle {
		return a.Cycle - b.Cycle
	}
	if a.Step != b.Step {
		return a.Step - b.Step
	}
	if at, bt := graphIndex(a.Phase), graphIndex(b.Phase); at != bt {
		return at - bt
	}
	return a.Try - b.Try
}

// graphIndex is where a phase sits in the declared order. A name the graph does
// not know sorts last rather than first, so an unreadable record cannot displace
// the phase that actually ran.
func graphIndex(n phase.Name) int {
	for i, p := range phase.Graph {
		if p.Name == n {
			return i
		}
	}
	return len(phase.Graph)
}
