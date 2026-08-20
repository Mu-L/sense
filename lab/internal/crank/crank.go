package crank

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/plans"
	"github.com/luuuc/sense/lab/internal/position"
)

// Job is one phase to run: everything a spawner needs and nothing it could
// decide with.
type Job struct {
	Repo  string
	Cycle int
	Phase phase.Name
	// Dir is where this phase's artifact and verdict belong.
	Dir string
	// Checkout is the repository under study, at its pin.
	Checkout string
	// Prompt is the plan, with the facts of this attempt in front of it.
	Prompt string
	// Wall is how long the agent gets, read from the plan's own header.
	Wall time.Duration
	// Try is which attempt at this phase, in this cycle, this is. It is here so
	// a spawner can keep a second attempt's evidence beside the first's rather
	// than on top of it: a run directory is created fresh or not at all.
	Try int
}

// Ran is what a spawner reports back: whether the agent finished inside its
// wall, and where what it said was written.
//
// It carries no verdict. What a phase decided is read from the document it
// wrote, never from the process that ran it, because an exit code is a claim.
type Ran struct {
	Finished bool
	Log      string
}

// Spawner runs one phase agent. The real one lives at the edge, beside the
// other things that reach a process; the crank never imports one.
type Spawner func(ctx context.Context, j Job) (Ran, error)

// Crank is the wiring, assembled once. It is a value rather than parameters
// because [Crank.Advance] takes a repository and a context and nothing else:
// the moment a phase needs something the others do not, it belongs in that
// phase's plan or in the spawner, never in this signature.
type Crank struct {
	// Runs is the root the repositories' run trees live under.
	Runs string
	// Plans are the declared plans, already checked against the graph.
	Plans []plans.Plan
	// Checkout is the repository under study, at its pin.
	Checkout string
	Spawn    Spawner
}

// Result is what one turn of the crank did.
type Result struct {
	// Before and After are the position on either side of it. A caller reads
	// After for the exit code, and the pair is what says whether anything moved.
	Before, After position.Position
	// Ran is the phase this turn dispatched, empty when it dispatched none.
	Ran phase.Name
	// Note is what happened, in a sentence, for whoever is reading a loop that
	// stopped.
	Note string
}

// Advance runs the one phase that comes next for a repository.
//
// It reads a position, dispatches at most one phase, records what came back and
// returns. It never loops: `-until` is a loop in main around this call, so
// nothing in here knows which of the two it is in and no mode is threaded
// through it.
func (c Crank) Advance(ctx context.Context, repo string) (Result, error) {
	before, err := position.Read(c.Runs, repo)
	if err != nil {
		return Result{}, err
	}
	r := Result{Before: before, After: before}

	// The automation stops at the pay call, and it stops the same way whichever
	// side of the verdict it is on: a PAY that has been recorded, and a phase
	// that spends being next, are one situation. The crank prints the command
	// and does not run it. The pain this cycle is aimed at is authoring, and an
	// unattended crank that can reach a paid cell is a different product with a
	// different risk.
	if _, spends := spendCommand(before, repo); before.Standing != position.Ready || spends {
		if spends {
			r.After.Standing = position.Waiting
		}
		r.Note = note(r.After, repo)
		return r, nil
	}

	p, ok := planFor(c.Plans, before.Awaiting)
	if !ok {
		return r, fmt.Errorf("no plan for phase %s; a phase with no plan is a phase nobody can run", before.Awaiting)
	}
	j := c.job(before, repo, p, c.try(repo, before.Cycle, p.Phase))
	r.Ran = j.Phase

	ran, err := c.Spawn(ctx, j)
	if err != nil {
		return r, fmt.Errorf("run %s for %s: %w", j.Phase, repo, err)
	}
	if err := c.record(before, j, p, ran); err != nil {
		return r, err
	}
	after, err := position.Read(c.Runs, repo)
	if err != nil {
		return r, err
	}
	r.After = after
	r.Note = note(after, repo)
	return r, nil
}

// note is what a caller prints when the crank stops, and it carries the
// hand-run command whenever the next phase is one that spends.
//
// It is built in one place because the crank stops at the pay call from two
// directions — arriving at it, and finding itself already there — and a command
// printed on only one of them is a loop that stops silently every other time.
func note(at position.Position, repo string) string {
	line := fmt.Sprintf("%s is %s: %s", repo, at.Standing, at.Because)
	if command, spends := spendCommand(at, repo); spends {
		line += fmt.Sprintf("\n\n%s is where the spending starts. Run it by hand:\n\n    %s\n",
			at.Awaiting, command)
	}
	return line
}

// record reads what the phase left behind, guards it, and writes the attempt.
//
// Every path through here records something, and that is deliberate: an
// invocation that dispatched a phase and recorded nothing leaves the position
// exactly as it found it, and an unattended loop then runs that phase again,
// forever, at whatever it costs.
func (c Crank) record(at position.Position, j Job, p plans.Plan, ran Ran) error {
	a := position.Attempt{
		Cycle: j.Cycle, Phase: j.Phase, Try: j.Try,
		Anchor: at.Last.Anchor, Plan: p.Path, Log: ran.Log,
	}

	if !ran.Finished {
		// Out of clock. Cannot-finish-at-budget is a result, and it is recorded
		// as one rather than waited on: the alternative is a crank that holds
		// on a hung agent until somebody notices.
		a.Outcome = position.Stalled
		return position.Record(filepath.Join(c.Runs, j.Repo), a)
	}

	v, err := readVerdict(j.Dir)
	if err == nil {
		err = guard(v, j, p)
	}
	if err != nil {
		// The phase said something that is not a verdict for this phase. It is
		// recorded as refused, with the reason, and the loop stops there.
		a.Outcome, a.Table = position.Refused, err.Error()
		return position.Record(filepath.Join(c.Runs, j.Repo), a)
	}

	a.Verdict, a.Table = v.Verdict, v.Table
	if v.Anchor != "" {
		// A phase that decided an anchor says so; one that did not leaves the
		// previous attempt's in place. A re-entry that dropped the anchor would
		// be a fresh guess rather than a second attempt at the same question.
		a.Anchor = v.Anchor
	}
	a.VerdictDoc = filepath.Join(j.Dir, VerdictFile)
	if wrote(j.Dir, p) {
		// The artifact is the fact, so it is recorded as accepted only when it
		// is there. When it is not, the verdict stands and the position reads
		// the missing artifact for itself.
		a.Artifact = filepath.Join(j.Dir, p.Writes)
	}
	return position.Record(filepath.Join(c.Runs, j.Repo), a)
}

// job is the phase to dispatch, assembled from the position and the plan.
func (c Crank) job(at position.Position, repo string, p plans.Plan, try int) Job {
	dir := filepath.Join(c.Runs, repo, strconv.Itoa(at.Cycle), string(p.Phase))
	return Job{
		Repo: repo, Cycle: at.Cycle, Phase: p.Phase, Dir: dir, Try: try,
		Checkout: c.Checkout, Wall: p.Wall, Prompt: prompt(at, repo, dir, p),
	}
}

// try is which attempt at this phase, in this cycle, is about to happen. It is
// worked out before the phase runs so the spawner can be told, and again from
// the same records when the attempt is recorded.
func (c Crank) try(repo string, cycle int, name phase.Name) int {
	attempts, err := position.Attempts(filepath.Join(c.Runs, repo))
	if err != nil {
		// An unreadable record set is the recorder's to report; here it means
		// only that nothing is known about earlier tries.
		return 1
	}
	return position.NextTry(attempts, cycle, name)
}

// prompt is the plan, with the facts of this attempt in front of it.
//
// Facts only: where things are, which cycle this is, and every rejection so far.
// Not a sentence of method — that all lives in the plan and is reviewed by
// reading a file. A binary that adds guidance here is a binary holding opinions
// nobody can review.
//
// Every rejection, oldest first, because reading only the latest is how six
// attempts oscillated between two failures without landing in between.
func prompt(at position.Position, repo, dir string, p plans.Plan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "repository: %s\ncycle: %d of %d\nphase: %s\nartifact: %s\nverdict: %s\n",
		repo, at.Cycle, phase.AuthoringCeiling, p.Phase, filepath.Join(dir, p.Writes), filepath.Join(dir, VerdictFile))
	if at.Last.Anchor != "" {
		fmt.Fprintf(&b, "anchor: %s\n", at.Last.Anchor)
	}
	for _, a := range at.Answer {
		fmt.Fprintf(&b, "rejected in cycle %d by %s (%s): %s\n", a.Cycle, a.Phase, a.Verdict, a.Table)
	}
	b.WriteString("\n")
	b.Write(p.Body)
	return b.String()
}

// spending is every phase that costs money, and the crank runs none of them.
var spending = map[phase.Name]bool{phase.Bench: true, phase.Report: true, phase.Harvest: true, phase.Board: true}

// spendCommand reports whether the next phase spends, and what to run by hand.
func spendCommand(at position.Position, repo string) (string, bool) {
	if !spending[at.Awaiting] {
		return "", false
	}
	if at.Awaiting != phase.Bench {
		return fmt.Sprintf("sense-lab %s (by hand; see lab/plans/%s.md)", at.Awaiting, at.Awaiting), true
	}
	return fmt.Sprintf("sense-lab probe -repo %s -scenario lab/scenarios/%s/%s.yaml -out <cell> "+
		"-agent <agent> -model <model> -checkout <clone> -sense ./bin/sense", repo, repo, repo), true
}

// planFor is the declared plan for a phase.
func planFor(loaded []plans.Plan, name phase.Name) (plans.Plan, bool) {
	for _, p := range loaded {
		if p.Phase == name {
			return p, true
		}
	}
	return plans.Plan{}, false
}
