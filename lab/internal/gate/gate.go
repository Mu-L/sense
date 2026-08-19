// Package gate is where the laws stop being prose.
//
// Every law tagged `gate` is code that refuses, or it is decoration. The old
// tree learned this twice and both were expensive.
//
// A rule believed to be law, enforced by nothing: "RUNS=2 binds the headline
// arm only" was quoted as a constraint. One of the two files it came from was
// never executed at all; in the other, the line lived inside a function that
// renders a sentence. A live script that only builds a report string
// constrains nothing, and nobody knew.
//
// And prose losing to code: the manifesto said objective recall is the headline
// and the blind composite must never lead. The reporter ranked by the blind
// composite anyway and published a +0.44 win as a tie. The document was right
// for months while the table was wrong.
//
// So the test for a gate here is not "does it exist" but "does it refuse on its
// condition, and does a test assert the refusal".
//
// # Six gates, six functions
//
// Each is its own function with its own test, and there is deliberately no
// aggregate that runs them in a fixed order and returns the first failure.
// Which gate fired is information; an aggregate destroys it while quietly
// making the order significant. Refusals reports all of them.
//
// # No bypass
//
// There is no flag, no environment variable and no config field that turns a
// gate off. A gate with an override is a suggestion, and the moment of
// frustration is exactly when it would be used.
//
// The sanctioned path when a gate is wrong mid-cycle is named instead,
// because with no route the pressure lands on quietly editing the gate, which
// is worse and less visible: fix the gate, or record a ruling that exempts that
// one cell with the reason attached to it. A ruling is a decision someone
// signed and a future reader can find.
package gate

import (
	"fmt"
	"time"
)

// Bar is the margin a cell must clear. It is here as well as in the tally
// because the ceiling is arithmetic about it.
const Bar = 0.50

// BaselineAssemblesTheSet refuses a cell whose baseline already reaches every
// row of the discriminator group.
//
// There is nothing left for the treatment to find, so whatever the sense arm
// scores, the cell cannot show a difference that means anything.
func BaselineAssemblesTheSet(reached, total int) error {
	if total > 0 && reached >= total {
		return fmt.Errorf("the baseline already assembles the set: %d of %d rows, so there is nothing left to find",
			reached, total)
	}
	return nil
}

// ArithmeticCeiling refuses a cell whose recorded baseline is high enough that
// the bar is unreachable.
//
// Recall caps at 1.00, so a margin of Bar requires a baseline of at most
// 1 − Bar. A baseline above that cannot clear the bar however good the sense
// arm is, and paying to discover that is paying for arithmetic.
//
// It applies to a RECORDED baseline only. For a fresh cell there is no baseline
// until a mini-bench has run, and a version that estimated one would be a
// screen rather than a gate — which is why this check was kept out of the
// planner and lands here, at pay time.
func ArithmeticCeiling(baseline float64, recorded bool) error {
	if !recorded {
		return nil
	}
	if ceiling := 1 - Bar; baseline > ceiling {
		return fmt.Errorf("the recorded baseline is %.3f and recall caps at 1.00, so the largest possible margin is %.3f: +%.2f cannot be reached",
			baseline, 1-baseline, Bar)
	}
	return nil
}

// MiniBenchFirst refuses the pay path for a cell that has never been run.
//
// A mini-bench is both arms, once, unpaid. Paying without one is paying to find
// out whether the scenario works at all.
func MiniBenchFirst(ran bool) error {
	if !ran {
		return fmt.Errorf("no mini-bench has run for this cell; both arms run once before anything is paid for")
	}
	return nil
}

// Validation is the unscored run that proves the cell can be run at all.
type Validation struct {
	// Ran says a validation cell exists.
	Ran bool
	// Scored says it carries a score. It must not: a validation run is
	// unscored by law, because a number from it would be read as a result.
	Scored bool
	// Wall is the wall it ran at, and RealWall is the wall the paid run will
	// use. A validation at a shorter wall proves nothing about the run that
	// matters.
	Wall     time.Duration
	RealWall time.Duration
}

// ValidationRun refuses the pay path without an unscored validation cell at the
// real wall.
func ValidationRun(v Validation) error {
	switch {
	case !v.Ran:
		return fmt.Errorf("no validation run for this cell; the pay path needs one, unscored, at the real wall")
	case v.Scored:
		return fmt.Errorf("the validation run carries a score; it is unscored by law, because a number from it reads as a result")
	case v.Wall < v.RealWall:
		return fmt.Errorf("the validation ran at %s against a real wall of %s; a shorter wall proves nothing about the run that matters",
			v.Wall, v.RealWall)
	}
	return nil
}

// SingleRunCell refuses to publish a cell that is not a measurement yet.
//
// The recorded same-cell spreads reach 0.250 against a bar of 0.50, so one run
// is a sample of a distribution whose spread is half the bar.
func SingleRunCell(senseRuns, baselineRuns int) error {
	const replicates = 2
	if senseRuns < replicates || baselineRuns < replicates {
		return fmt.Errorf("this cell has %d sense and %d baseline scored runs, and a published cell needs %d of each",
			senseRuns, baselineRuns, replicates)
	}
	return nil
}

// RetryBound refuses a second retry, and refuses to score a replaced run.
//
// One retry, never a loop. Keeping a parked run scorable would let a cell be
// re-read until it said the right thing.
func RetryBound(retries int, scoredAParkedRun bool) error {
	switch {
	case retries > 1:
		return fmt.Errorf("this cell has been retried %d times; one retry, never a loop", retries)
	case scoredAParkedRun:
		return fmt.Errorf("a parked run was scored; a replaced run is never scored")
	}
	return nil
}

// Decision is everything the gates are asked about one cell.
//
// Cost is absent, and its absence is the point: costing more is a product
// finding, not a stopper, so a cell that wins its discriminator while spending
// more still wins. There is nowhere here for a cost to be put.
type Decision struct {
	// BaselineReached and GroupTotal are the discriminator group's rows.
	//
	// The DISCRIMINATOR group, not every group. A recorded win has groups whose
	// baseline sits well above the ceiling — one banked cell carries a contract
	// group at 0.75 — and gating on any group would refuse a cell that was paid
	// for and published.
	BaselineReached int     `json:"baseline_reached"`
	GroupTotal      int     `json:"group_total"`
	BaselineRecall  float64 `json:"baseline_recall"`
	// BaselineRecorded says the baseline above is measured rather than assumed.
	BaselineRecorded bool `json:"baseline_recorded"`
	MiniBenchRan     bool `json:"mini_bench_ran"`
	Validation       Validation
	SenseRuns        int  `json:"sense_runs"`
	BaselineRuns     int  `json:"baseline_runs"`
	Retries          int  `json:"retries"`
	ScoredAParkedRun bool `json:"scored_a_parked_run"`
}

// Refusals runs every gate and reports all of them that fired, by name.
//
// All of them, not the first: which gate fired is information, and an operator
// who fixes one thing only to be refused by the next has learned nothing about
// the cell. The order here is presentation and nothing depends on it.
func Refusals(d Decision) []error {
	var refused []error
	for _, err := range []error{
		BaselineAssemblesTheSet(d.BaselineReached, d.GroupTotal),
		ArithmeticCeiling(d.BaselineRecall, d.BaselineRecorded),
		MiniBenchFirst(d.MiniBenchRan),
		ValidationRun(d.Validation),
		SingleRunCell(d.SenseRuns, d.BaselineRuns),
		RetryBound(d.Retries, d.ScoredAParkedRun),
	} {
		if err != nil {
			refused = append(refused, err)
		}
	}
	return refused
}
