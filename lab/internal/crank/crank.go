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
	// Cell is where the pair this phase reads landed, for a phase that reads
	// one. Empty for every phase whose input is an artifact.
	Cell string
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

// Cell is the two-arm pair a phase reads, for a phase that reads one.
//
// It carries where the pair belongs and what to run it against, and nothing a
// prober could decide with. Which directory inside Dir the arms land in is the
// prober's: a pair is created fresh or not at all, so the name has to be chosen
// against what is already on disk, and that is a question only the side that
// reaches the disk can answer.
type Cell struct {
	Repo  string
	Cycle int
	Phase phase.Name
	// Dir is the phase directory the pair belongs under.
	Dir string
	// Scenario is the file the pair is run against, derived from the phase the
	// graph says wrote it.
	Scenario string
	// Checkout is the repository under study, at its pin.
	Checkout string
}

// Pair is what a prober reports: whether the two arms are a measurement.
//
// Not a measurement is not an error. The arms ran, they cost what they cost,
// and what came back cannot be compared — which is a result to record and stop
// on, not a broken invocation.
type Pair struct {
	Sound bool
	// Dir is where the arms landed, sound or not.
	Dir string
	// Note is why it is not a measurement, in a sentence, and it is what the
	// attempt record carries.
	Note string
}

// Prober runs one cell. Like [Spawner] the real one lives at the edge: this
// package decides that a pair is owed and never reaches a process to produce
// one.
type Prober func(ctx context.Context, c Cell) (Pair, error)

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
	// Probe runs the pair a phase reads, for the phases that read one. A crank
	// assembled without it can still turn every phase that reads an artifact.
	Probe Prober
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
	//
	// The mini-bench pair is not that cell. It is unscored and unpaid by law —
	// budget counts what is under the bench phase — and it is the gate that
	// exists to avoid reaching one, so the crank runs it. See probes.
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

	pair, err := c.pair(ctx, j, p)
	if err != nil {
		return r, err
	}
	// The prompt is composed after the pair, because where the pair landed is
	// one of the facts it carries. A judge left to infer that from the artifact
	// path it was given would be guessing at a directory name.
	j.Cell = pair.Dir
	j.Prompt = prompt(before, j, p)
	if !pair.Sound {
		// The arms ran and what came back cannot be compared. The judge is not
		// spawned on a pair it may not read, and the refusal is recorded rather
		// than returned: an unrecorded stop leaves the position exactly as it
		// was found, and the next turn runs another pair, at whatever two arms
		// cost.
		if err := c.refuse(before, j, p, pair.Dir, pair.Note); err != nil {
			return r, err
		}
		return c.settled(r, repo)
	}

	ran, err := c.Spawn(ctx, j)
	if err != nil {
		return r, fmt.Errorf("run %s for %s: %w", j.Phase, repo, err)
	}
	if err := c.record(before, j, p, ran); err != nil {
		return r, err
	}
	return c.settled(r, repo)
}

// settled is the position after the turn, and the sentence a caller prints. It
// is read back from disk rather than reasoned about: what the turn did is
// whatever it left behind.
func (c Crank) settled(r Result, repo string) (Result, error) {
	after, err := position.Read(c.Runs, repo)
	if err != nil {
		return r, err
	}
	r.After = after
	r.Note = note(after, repo)
	return r, nil
}

// pair runs the cell this phase reads, for a phase that reads one.
//
// A phase whose input is an artifact is dispatched as it always has been. A
// phase whose input is a two-arm pair is dispatched only after the binary has
// produced one: the mini-bench plan reads a cell, and a crank that spawned that
// judge without producing one handed it an empty directory to rule on. Measured
// on mastodon cycle 1 — the judge wrote no verdict, correctly, and the
// repository parked on a defect in here.
func (c Crank) pair(ctx context.Context, j Job, p plans.Plan) (Pair, error) {
	if !probes[j.Phase] {
		return Pair{Sound: true}, nil
	}
	if c.Probe == nil {
		return Pair{}, fmt.Errorf("%s reads a two-arm cell and this crank was assembled without a prober; "+
			"dispatching it would hand the judge an empty directory to rule on", j.Phase)
	}
	scenario, err := c.reads(j, p)
	if err != nil {
		return Pair{}, err
	}
	got, err := c.Probe(ctx, Cell{Repo: j.Repo, Cycle: j.Cycle, Phase: j.Phase, Dir: j.Dir,
		Scenario: scenario, Checkout: c.Checkout})
	if err != nil {
		return Pair{}, fmt.Errorf("run the pair %s reads for %s: %w", j.Phase, j.Repo, err)
	}
	return got, nil
}

// reads is the file this phase reads, where the graph says it was written.
//
// Derived rather than named here: a path to another phase's artifact written
// out in Go is a path that keeps pointing at the old name after somebody
// renames the artifact in the graph.
func (c Crank) reads(j Job, p plans.Plan) (string, error) {
	for _, g := range phase.Graph {
		if g.Writes == p.Reads {
			return filepath.Join(c.Runs, j.Repo, strconv.Itoa(j.Cycle), string(g.Name), p.Reads), nil
		}
	}
	return "", fmt.Errorf("%s reads %s and no phase in the graph writes it", j.Phase, p.Reads)
}

// refuse records an attempt that produced no verdict because what it was owed
// was not one. It is the record a misbehaving agent leaves, for the same
// reason: the loop stops where it is, and a person reads why.
func (c Crank) refuse(at position.Position, j Job, p plans.Plan, log, why string) error {
	return position.Record(filepath.Join(c.Runs, j.Repo), position.Attempt{
		Cycle: j.Cycle, Phase: j.Phase, Try: j.Try, Anchor: at.Last.Anchor,
		Plan: p.Path, Log: log, Outcome: position.Refused, Table: why,
	})
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
		return c.refuse(at, j, p, ran.Log, err.Error())
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
	return Job{
		Repo: repo, Cycle: at.Cycle, Phase: p.Phase, Try: try,
		Dir:      filepath.Join(c.Runs, repo, strconv.Itoa(at.Cycle), string(p.Phase)),
		Checkout: c.Checkout, Wall: p.Wall,
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
func prompt(at position.Position, j Job, p plans.Plan) string {
	var b strings.Builder
	fmt.Fprintf(&b, "repository: %s\ncycle: %d of %d\nphase: %s\nartifact: %s\nverdict: %s\n",
		j.Repo, j.Cycle, phase.AuthoringCeiling, p.Phase,
		filepath.Join(j.Dir, p.Writes), filepath.Join(j.Dir, VerdictFile))
	if j.Cell != "" {
		// Where the pair is, for a phase that reads one. A fact about this
		// attempt, like every other line here: what to do with the arms is the
		// plan's, and this says nothing about it.
		fmt.Fprintf(&b, "cell: %s\n", j.Cell)
	}
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

// probes is every phase whose input is a two-arm pair rather than an artifact,
// which the binary runs before the phase's agent is spawned.
//
// Two phases judge a pair: the mini-bench rules on the draft's discriminator
// step, validate rules on the full seven-step scenario. Both were measured
// spawning onto an empty directory — validate on mastodon cycle 3, which
// refused, correctly, and named this table. It is a table rather than a
// comparison for the same reason spending is: what a phase reads is a property
// of the phase, and a caller reading `if j.Phase == phase.Minibench` in the
// middle of Advance learns nothing about why.
var probes = map[phase.Name]bool{phase.Minibench: true, phase.Validate: true}

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
