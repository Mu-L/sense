package judge_test

import (
	"slices"
	"testing"

	"github.com/luuuc/sense/lab/internal/judge"
)

// probe grades the same answer several times and reports the spread.
func probe(t *testing.T, repeats ...[]judge.Claim) judge.Stability {
	t.Helper()
	var gradings []judge.Grading
	for _, claims := range repeats {
		v := judge.Verdict{Claims: claims}
		r, err := judge.Grade(mastodonGold(t), v)
		if err != nil {
			t.Fatalf("Grade: %v", err)
		}
		gradings = append(gradings, judge.Grading{Verdict: v, Result: r})
	}
	return judge.Spread(gradings)
}

func TestAJudgeThatSaysTheSameThingTwiceHasNoSpread(t *testing.T) {
	same := []judge.Claim{
		{ID: "d:single-user-mode", State: judge.Covered},
		{ID: "d:extractor-mention-re", State: judge.Contradicted},
	}

	got := probe(t, same, same, same)

	if got.Gradings != 3 {
		t.Errorf("Gradings = %d, want 3", got.Gradings)
	}
	if got.RecallSpread != 0 || got.PrecisionSpread != 0 {
		t.Errorf("spreads = %.3f and %.3f on identical gradings", got.RecallSpread, got.PrecisionSpread)
	}
	if len(got.Unstable) != 0 {
		t.Errorf("Unstable = %v on identical gradings", got.Unstable)
	}
}

func TestARowThatLandsInADifferentStateIsNamed(t *testing.T) {
	// The tri-state is decided by the model too, and a flaky `contradicted`
	// call moves grounded_precision and therefore the blended score. Naming the
	// row is what lets somebody look at it.
	got := probe(t,
		[]judge.Claim{{ID: "d:extractor-mention-re", State: judge.Contradicted}, {ID: "d:single-user-mode", State: judge.Covered}},
		[]judge.Claim{{ID: "d:extractor-mention-re", State: judge.Related}, {ID: "d:single-user-mode", State: judge.Covered}},
	)

	if !slices.Equal(got.Unstable, []string{"d:extractor-mention-re"}) {
		t.Errorf("Unstable = %v, want the flaky row named", got.Unstable)
	}
	if want := 0.5; got.FlipRate != want {
		t.Errorf("FlipRate = %.3f, want %.3f", got.FlipRate, want)
	}
	// And that flip reaches the number: one of the two gradings had a
	// contradiction and the other did not.
	if got.PrecisionSpread == 0 {
		t.Error("PrecisionSpread = 0 although a contradiction appeared and vanished")
	}
}

func TestARowClaimedInOneGradingAndNotAnotherIsUnstable(t *testing.T) {
	// Silence and a claim are different judgements. Treating the absence as "no
	// opinion" would hide exactly the flakiness being measured.
	got := probe(t,
		[]judge.Claim{{ID: "d:single-user-mode", State: judge.Covered}},
		nil,
	)

	if !slices.Equal(got.Unstable, []string{"d:single-user-mode"}) {
		t.Errorf("Unstable = %v, want the row that came and went", got.Unstable)
	}
}

func TestARowFirstSeenInALaterGradingIsStillUnstable(t *testing.T) {
	// The failure a single pass would have: a row first claimed in the third
	// grading would only be compared against the gradings that came after it,
	// and would read as perfectly stable.
	got := probe(t,
		[]judge.Claim{{ID: "d:single-user-mode", State: judge.Covered}},
		[]judge.Claim{{ID: "d:single-user-mode", State: judge.Covered}},
		[]judge.Claim{{ID: "d:single-user-mode", State: judge.Covered}, {ID: "d:admin-audit-scope", State: judge.Related}},
	)

	if !slices.Equal(got.Unstable, []string{"d:admin-audit-scope"}) {
		t.Errorf("Unstable = %v, want the row that only appeared once", got.Unstable)
	}
}

func TestUnstableRowsAreNamedInAStableOrder(t *testing.T) {
	got := probe(t,
		[]judge.Claim{{ID: "d:single-user-mode", State: judge.Covered}, {ID: "d:admin-audit-scope", State: judge.Covered}},
		nil,
	)

	if !slices.Equal(got.Unstable, []string{"d:admin-audit-scope", "d:single-user-mode"}) {
		t.Errorf("Unstable = %v, want them in id order", got.Unstable)
	}
}

func TestJudgeNoiseLargeEnoughToExplainAMarginIsFlagged(t *testing.T) {
	// The threshold with teeth: if judge spread approaches the margins being
	// called wins, the direct-API question reopens with evidence. It is a
	// question and not a gate — a threshold that automatically invalidated
	// cells would let judge noise silently delete results.
	noisy := probe(t,
		[]judge.Claim{{ID: "d:single-user-mode", State: judge.Covered}, {ID: "d:admin-audit-scope", State: judge.Covered},
			{ID: "d:inbox-unknown-actor", State: judge.Covered}},
		nil,
	)

	if !noisy.Threatens(0.50) {
		t.Errorf("a recall spread of %.3f was not flagged against a 0.50 margin", noisy.RecallSpread)
	}

	quiet := probe(t,
		[]judge.Claim{{ID: "d:single-user-mode", State: judge.Covered}},
		[]judge.Claim{{ID: "d:single-user-mode", State: judge.Covered}},
	)
	if quiet.Threatens(0.50) {
		t.Error("a judge that said the same thing twice was flagged as threatening a margin")
	}
}

func TestAProbeWithNothingToCompareReportsNothing(t *testing.T) {
	got := judge.Spread(nil)

	if got.Gradings != 0 || got.RecallSpread != 0 || len(got.Unstable) != 0 {
		t.Errorf("Spread(nil) = %+v", got)
	}
}

func TestASingleGradingHasNoSpreadToReport(t *testing.T) {
	// One grading cannot disagree with anything, and reporting a spread of zero
	// as evidence of stability would be reading a measurement that was never
	// taken. Gradings is what says so.
	got := probe(t, []judge.Claim{{ID: "d:single-user-mode", State: judge.Covered}})

	if got.Gradings != 1 {
		t.Errorf("Gradings = %d, want 1", got.Gradings)
	}
	if got.RecallSpread != 0 || len(got.Unstable) != 0 {
		t.Errorf("a single grading reported a spread: %+v", got)
	}
}
