package judge

import "sort"

// Stability is what repeated gradings of the same answer produced.
//
// It exists because the judge is a model call inside a measurement, which makes
// it the one component that can quietly change what every past number meant.
// The probe converts "is the judge stable enough" from an argument into a
// number that can be rechecked after any model or harness change.
//
// It reports and it decides nothing. A stability threshold that automatically
// invalidated cells would let judge noise silently delete results; if the
// number is bad, a person decides what to do about it.
type Stability struct {
	// Gradings is how many times the same answer was graded.
	Gradings int
	// RecallSpread and PrecisionSpread are the highest minus the lowest value
	// each metric took. They are what a margin is compared against.
	RecallSpread    float64
	PrecisionSpread float64
	// Unstable names the gold rows that did not land in the same state every
	// time, in a stable order.
	//
	// It is reported separately from the metric spreads because it is the
	// number that actually threatens a result: the tri-state is decided by the
	// model too, and a flaky `contradicted` call moves grounded_precision and
	// therefore the blended score.
	Unstable []string
	// FlipRate is the share of graded rows that were unstable.
	FlipRate float64
}

// Grading is one repeat of the probe: what the judge said and what it meant.
//
// The two travel together rather than as two slices, so a probe cannot compare
// one grading's verdict against another's numbers.
type Grading struct {
	Verdict Verdict
	Result  Result
}

// Spread reports what repeated gradings of one answer did.
//
// The gradings must be of the SAME answer against the SAME gold: comparing two
// answers would measure the arms, which is the thing the judge is supposed to
// be measuring, and the noise would be indistinguishable from the signal.
func Spread(gradings []Grading) Stability {
	s := Stability{Gradings: len(gradings)}
	if len(gradings) == 0 {
		return s
	}

	s.RecallSpread = spreadOf(gradings, func(r Result) float64 { return r.RelatedRecall })
	s.PrecisionSpread = spreadOf(gradings, func(r Result) float64 { return r.GroundedPrecision })

	// A row is unstable if it did not land in the same state every time. A row
	// claimed in one grading and absent from another counts: silence and a
	// claim are different judgements, and treating the absence as "no opinion"
	// would hide exactly the flakiness being measured.
	//
	// Two passes, because a row first claimed in the third grading has to be
	// counted absent from the first two, and a single pass would only ever
	// compare a row against the gradings that came after it.
	graded := map[string]bool{}
	for _, g := range gradings {
		for _, c := range g.Verdict.Claims {
			graded[c.ID] = true
		}
	}
	states := map[string]map[State]bool{}
	for _, g := range gradings {
		said := map[string]State{}
		for _, c := range g.Verdict.Claims {
			said[c.ID] = c.State
		}
		for id := range graded {
			if states[id] == nil {
				states[id] = map[State]bool{}
			}
			states[id][said[id]] = true
		}
	}
	for id, seen := range states {
		if len(seen) > 1 {
			s.Unstable = append(s.Unstable, id)
		}
	}
	sort.Strings(s.Unstable)
	if len(graded) > 0 {
		s.FlipRate = float64(len(s.Unstable)) / float64(len(graded))
	}
	return s
}

// spreadOf is the highest minus the lowest value a metric took.
func spreadOf(gradings []Grading, of func(Result) float64) float64 {
	low, high := of(gradings[0].Result), of(gradings[0].Result)
	for _, g := range gradings[1:] {
		v := of(g.Result)
		if v < low {
			low = v
		}
		if v > high {
			high = v
		}
	}
	return high - low
}

// Threatens reports whether the judge's own noise is large enough to account
// for a margin being called a win.
//
// It is a question, not a gate. A true answer means the direct-API question
// reopens with evidence attached, and it means a person looks — never that a
// cell is deleted.
func (s Stability) Threatens(margin float64) bool {
	return s.RecallSpread >= margin || s.PrecisionSpread >= margin
}
