// Package say turns the numbers this instrument produces into the sentences a
// person reads.
//
// The measurement is a pair of recalls and a floor, and printed as itself it
// reads `sense 0.84 · baseline 0.24 · delta +0.60`. Every one of those asks the
// reader to remember something: which direction is good, what the bar is, and
// that a delta is a subtraction rather than a score. The same three numbers as
// a sentence ask nothing.
//
// It takes floats rather than a [score.Result] so that the mini-bench, validate
// and the paid run all reach the same words. What was scored is the caller's;
// how it reads is here.
package say

import (
	"fmt"
	"math"
)

// Pair is one comparison as a reader needs it: what each arm reached, and the
// gap the method asks for before it is worth paying for.
//
// All three are recalls in the range 0 to 1, the form the scorer produces, and
// none of them is ever printed in that form.
type Pair struct {
	Sense    float64
	Baseline float64
	// Floor is the gap the method requires. Zero means no bar was stated, and
	// the sentence then reports the gap without judging it.
	Floor float64
}

// points is the three numbers as they will be printed.
//
// The gap is the difference of the ROUNDED percentages rather than the rounded
// difference, because the sentence prints all three and a reader subtracts the
// two it can see. 0.874 against 0.204 is 87 and 20, and a gap of 67: reporting
// 66 from the unrounded difference would be arithmetic that fails in front of
// the person it is explaining itself to.
func (p Pair) points() (sense, baseline, gap int) {
	sense, baseline = percent(p.Sense), percent(p.Baseline)
	return sense, baseline, sense - baseline
}

// Won reports whether the gap clears the floor, judged on the same numbers the
// sentence prints.
func (p Pair) Won() bool {
	_, _, gap := p.points()
	return p.Floor > 0 && gap >= percent(p.Floor)
}

// Sentence is the pair in words: what each arm reached, and what that means
// against the bar.
//
// Three readings, because there are three things that can have happened and
// they are not degrees of one another. Sense reached further and it was worth
// paying for; Sense reached further and it was not; Sense reached no further at
// all, which is not a small win but a different result, and the sentence says
// so rather than printing a gap of 2 and leaving the reader to notice.
func (p Pair) Sentence() string {
	sense, baseline, gap := p.points()
	switch {
	case gap <= 0:
		return fmt.Sprintf("Sense gave no advantage: %d%% with it, %d%% without.", sense, baseline)
	case p.Won():
		return fmt.Sprintf("With Sense the agent found %d%% of what we hid. Without it, %d%%.\n%s This is %d.",
			sense, baseline, p.bar(), gap)
	default:
		return fmt.Sprintf("With Sense %d%%, without %d%%. A %d-point gap, %s",
			sense, baseline, gap, p.short())
	}
}

// bar is the requirement, stated where a win is being claimed. A win against an
// unnamed bar is a number the reader has to take on trust.
//
// It is only ever reached with a floor, because [Pair.Won] is false without
// one: an unbarred pair cannot have cleared a bar.
func (p Pair) bar() string {
	return fmt.Sprintf("We need a %d-point gap before it is worth paying for.", percent(p.Floor))
}

// short says what a gap below the bar means, in the same terms the bar was
// stated in.
func (p Pair) short() string {
	if p.Floor <= 0 {
		return "with no bar set for it."
	}
	return fmt.Sprintf("short of the %d we need.", percent(p.Floor))
}

// percent rounds a recall to whole points. Half rounds away from zero, which is
// what [math.Round] does and what a reader expects: 0.865 is 87%, not 86%.
func percent(v float64) int { return int(math.Round(v * 100)) }

// Gap is the difference in points, for a caller that needs the number on its
// own rather than in a sentence.
func (p Pair) Gap() int {
	_, _, gap := p.points()
	return gap
}

// Mean is the pair a set of runs amounts to.
//
// A cell is run more than once because the recorded spread within one cell
// reaches 0.250 against a bar of 0.50, so one run is a sample of a distribution
// whose spread is half the bar. The mean is what may be reported; a caller
// handed one run says so itself, with [Runs].
func Mean(runs []Pair) Pair {
	if len(runs) == 0 {
		return Pair{}
	}
	var m Pair
	for _, r := range runs {
		m.Sense += r.Sense
		m.Baseline += r.Baseline
	}
	n := float64(len(runs))
	return Pair{Sense: m.Sense / n, Baseline: m.Baseline / n, Floor: runs[0].Floor}
}

// Runs is how many runs a number stands on, so a gap never appears without the
// count that produced it.
func Runs(n int) string {
	if n == 1 {
		return "on a single run"
	}
	return fmt.Sprintf("averaged over %d runs", n)
}
