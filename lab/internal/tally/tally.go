// Package tally turns the runs of one cell into a result.
//
// A run is not a result. Two runs of the same arm on the same cell disagree,
// and the recorded same-cell spreads are 0.077, 0.154, 0.154 and 0.250. Against
// a floor of +0.50 the largest is half the bar, so any rule that treats a
// single run as a measurement is reading noise.
//
// There is a second problem and the old tree paid for it in a published report:
// what ranks the board decides what the bench is about. Ranked by a blind
// composite, a +0.44 recall win rendered as a −0.018 tie and Sense appeared to
// tie on 12 of 13 repositories. The prose said the right thing the whole time.
// The code did not, and the code is what produced the table.
//
// So the headline is objective cited recall, and it ranks everything.
package tally

import (
	"errors"
	"fmt"
	"sort"
)

// Bar is the margin a cell must clear to be a win.
const Bar = 0.50

// observedSpreads are the same-cell spreads measured on the recorded corpus.
// The confirmation band is derived from them rather than chosen, so it moves
// only when a new measurement moves it.
var observedSpreads = []float64{0.077, 0.154, 0.154, 0.250}

// ConfirmBand is how close to the bar a result has to be before it is a
// confirm-band result rather than a clean call: the largest spread the corpus
// has actually shown.
//
// It is conservative on purpose, and the reason is worth stating rather than
// leaving to be inferred. Judging is per run, so a cell's observed spread
// carries judge noise and arm noise together. A band presented as arm variance
// would be a claim nobody measured; the stability probe is what separates them.
var ConfirmBand = maxOf(observedSpreads)

// The B-score weights.
//
// They are INHERITED, not derived. They come from the old tree's judging
// contract and no fitting produced them. They are carried across because
// changing them would make every banked number incomparable, which is a much
// larger cost than any improvement a re-weighting could buy.
//
// Changing them requires re-scoring the whole corpus and saying so. That is the
// only thing that justifies touching them.
//
// There is no efficiency term. Efficiency is not a correctness axis and folding
// it in dilutes the thing being measured; it is reported separately, at held
// recall. There is no blind quality term either, and there is not going to be
// one.
const (
	weightCitedRecall       = 0.55
	weightRelatedRecall     = 0.25
	weightGroundedPrecision = 0.20
)

// Run is one replicate's scores, and which gold rows it reached.
type Run struct {
	// ID is the run directory, so a flipped row can be traced back to it.
	ID string
	// CitedRecall is the headline: objective, and not an opinion.
	CitedRecall float64
	// RelatedRecall and GroundedPrecision come from the reference-aware judge.
	RelatedRecall     float64
	GroundedPrecision float64
	// Rows are the gold row ids this run reached.
	Rows []string
	// Parked says this run was replaced by a retry and is never scored.
	Parked bool
	// ParkedWhy records what it was replaced for.
	ParkedWhy string
}

// BScore is the blended figure that accompanies the headline. Every term is
// objective or reference-aware.
func (r Run) BScore() float64 {
	return weightCitedRecall*r.CitedRecall +
		weightRelatedRecall*r.RelatedRecall +
		weightGroundedPrecision*r.GroundedPrecision
}

// Measured is an aggregate together with the spread of the runs behind it.
//
// They travel as one value rather than as two fields, because a consumer that
// reads the aggregate without the spread is reading half a result. Two runs at
// 0.30 and 0.70 are not one run at 0.50, and the type is what stops a caller
// rendering them as though they were.
type Measured struct {
	Value  float64
	Spread float64
}

// String renders the aggregate with its spread, always.
func (m Measured) String() string { return fmt.Sprintf("%.3f ±%.3f", m.Value, m.Spread) }

// Cell is one job's replicates: the same everything, run more than once per arm.
type Cell struct {
	Sense    []Run
	Baseline []Run
}

// Result is what a cell says.
type Result struct {
	// Cell is the runs it was computed from, every one of them, so a reader can
	// see the disagreement rather than only its average.
	Cell Cell
	// SenseRecall and BaselineRecall are the headline, each with its spread.
	SenseRecall    Measured
	BaselineRecall Measured
	// Margin is sense minus baseline on the headline, with the larger of the
	// two arms' spreads.
	Margin Measured
	// SenseB and BaselineB are the blended figure. They accompany the headline
	// and never rank on their own.
	SenseB    Measured
	BaselineB Measured
	// Open says the cell is not a measurement yet: an arm with fewer than two
	// scored runs cannot be published.
	Open bool
	// OpenWhy says what is missing.
	OpenWhy string
	// InConfirmBand says the margin is close enough to the bar that the
	// measured spread could account for the side it landed on.
	InConfirmBand bool
	// FlippedVerdict says the paired replicates landed on different sides of
	// the bar. This is what buys a third run.
	FlippedVerdict bool
	// FlippedRows are the gold rows one replicate reached and another did not.
	// They are diagnostic and they NEVER trigger spend: rows flip between runs
	// as ordinary variance, and paying for a third run every time one did would
	// be paying for noise.
	FlippedRows []string
}

// minReplicates is what a cell needs before it says anything. Two, because one
// run is a sample of a distribution whose spread is half the bar.
const minReplicates = 2

// Of computes a cell's result.
func Of(c Cell) Result {
	r := Result{Cell: c}
	sense, baseline := scored(c.Sense), scored(c.Baseline)

	r.SenseRecall = measure(sense, func(x Run) float64 { return x.CitedRecall })
	r.BaselineRecall = measure(baseline, func(x Run) float64 { return x.CitedRecall })
	r.SenseB = measure(sense, Run.BScore)
	r.BaselineB = measure(baseline, Run.BScore)

	r.Margin = Measured{
		Value:  r.SenseRecall.Value - r.BaselineRecall.Value,
		Spread: max(r.SenseRecall.Spread, r.BaselineRecall.Spread),
	}

	r.Open, r.OpenWhy = openness(sense, baseline)
	r.InConfirmBand = !r.Open && abs(r.Margin.Value-Bar) <= ConfirmBand
	r.FlippedVerdict = flippedVerdict(sense, baseline)
	r.FlippedRows = flippedRows(sense, baseline)
	return r
}

// Publishable reports whether the cell may be published, and why not when it
// may not. An open cell is enforced rather than noted: a single-run cell
// published as a result is the noise this package exists to keep out.
func (r Result) Publishable() error {
	if r.Open {
		return fmt.Errorf("this cell is open and unpublishable: %s", r.OpenWhy)
	}
	return nil
}

// NeedsThirdRun reports whether a third replicate is bought.
//
// A third run is bought by a flipped VERDICT and never by a flipped row. This
// is the rule that is easy to get backwards: individual gold rows flip between
// runs all the time and that is ordinary variance. What justifies paying is the
// two paired replicates landing on different sides of the bar.
func (r Result) NeedsThirdRun() bool { return r.FlippedVerdict }

// Headline is the one line a board carries. The aggregate never appears
// without its spread, on any path.
func (r Result) Headline() string {
	line := fmt.Sprintf("cited recall: sense %s, baseline %s, margin %s", r.SenseRecall, r.BaselineRecall, r.Margin)
	switch {
	case r.Open:
		return line + " (open: " + r.OpenWhy + ")"
	case r.InConfirmBand:
		return line + " (confirm band)"
	default:
		return line
	}
}

// Park replaces one run with a retry.
//
// One retry, never a loop, for infrastructure failure only. The replaced run is
// parked and never scored: keeping it would let a cell be re-read until it said
// the right thing, and that is the failure the bound exists to prevent.
func (c *Cell) Park(runID, why string) error {
	if why == "" {
		return errors.New("a parked run must say what it was replaced for")
	}
	for _, arm := range [][]Run{c.Sense, c.Baseline} {
		for i := range arm {
			if arm[i].ID != runID {
				continue
			}
			if parkedIn(arm) > 0 {
				return fmt.Errorf("this arm already parked a run; one retry, never a loop")
			}
			arm[i].Parked = true
			arm[i].ParkedWhy = why
			return nil
		}
	}
	return fmt.Errorf("no run %q in this cell", runID)
}

// scored is the runs that count. A parked run is not one of them.
func scored(arm []Run) []Run {
	var out []Run
	for _, r := range arm {
		if !r.Parked {
			out = append(out, r)
		}
	}
	return out
}

func parkedIn(arm []Run) int {
	n := 0
	for _, r := range arm {
		if r.Parked {
			n++
		}
	}
	return n
}

// measure is an arm's mean of a metric, with the spread of the runs behind it.
func measure(arm []Run, of func(Run) float64) Measured {
	if len(arm) == 0 {
		return Measured{}
	}
	sum, low, high := 0.0, of(arm[0]), of(arm[0])
	for _, r := range arm {
		v := of(r)
		sum += v
		low = min(low, v)
		high = max(high, v)
	}
	return Measured{Value: sum / float64(len(arm)), Spread: high - low}
}

// openness reports whether the cell is a measurement yet.
func openness(sense, baseline []Run) (bool, string) {
	switch {
	case len(sense) < minReplicates && len(baseline) < minReplicates:
		return true, fmt.Sprintf("both arms have fewer than %d scored runs", minReplicates)
	case len(sense) < minReplicates:
		return true, fmt.Sprintf("the sense arm has %d scored run(s), and a cell needs %d", len(sense), minReplicates)
	case len(baseline) < minReplicates:
		return true, fmt.Sprintf("the baseline arm has %d scored run(s), and a cell needs %d", len(baseline), minReplicates)
	}
	return false, ""
}

// flippedVerdict reports whether the paired replicates landed on different
// sides of the bar.
//
// Replicates are paired by position, which is what they are: the first sense
// run and the first baseline run were the two arms of one cell, launched
// together, and pairing them any other way would compare runs that were never
// a pair.
func flippedVerdict(sense, baseline []Run) bool {
	pairs := min(len(sense), len(baseline))
	if pairs < minReplicates {
		return false
	}
	first := sense[0].CitedRecall-baseline[0].CitedRecall >= Bar
	for i := 1; i < pairs; i++ {
		if (sense[i].CitedRecall-baseline[i].CitedRecall >= Bar) != first {
			return true
		}
	}
	return false
}

// flippedRows are the gold rows one replicate reached and another did not,
// across both arms, in a stable order.
func flippedRows(sense, baseline []Run) []string {
	var flipped []string
	for _, arm := range [][]Run{sense, baseline} {
		flipped = append(flipped, disagreements(arm)...)
	}
	sort.Strings(flipped)
	return dedupe(flipped)
}

func disagreements(arm []Run) []string {
	if len(arm) < minReplicates {
		return nil
	}
	count := map[string]int{}
	for _, r := range arm {
		for _, row := range unique(r.Rows) {
			count[row]++
		}
	}
	var flipped []string
	for row, n := range count {
		if n != len(arm) {
			flipped = append(flipped, row)
		}
	}
	return flipped
}

func unique(rows []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range rows {
		if !seen[r] {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

func dedupe(sorted []string) []string {
	var out []string
	for i, s := range sorted {
		if i == 0 || s != sorted[i-1] {
			out = append(out, s)
		}
	}
	return out
}

func maxOf(values []float64) float64 {
	high := values[0]
	for _, v := range values[1:] {
		high = max(high, v)
	}
	return high
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
