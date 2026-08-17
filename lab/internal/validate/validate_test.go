package validate_test

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/validate"
)

// The four banked cells, with the surfaces their runs exercised. Four is what
// exists, and the thinness is the point of half the tests below.
func corpus(after ...float64) []validate.Regression {
	banked := []validate.Banked{
		{Cell: "bitwarden-server", Model: "claude-opus-5", Margin: 0.625, Spread: 0.077, Surfaces: []string{"blast"}},
		{Cell: "mastodon", Model: "claude-opus-5", Margin: 0.530, Spread: 0.125, Surfaces: []string{"blast"}},
		{Cell: "discourse", Model: "claude-opus-5", Margin: 1.000, Spread: 0.250, Surfaces: []string{"blast", "graph"}},
		{Cell: "chatwoot", Model: "gpt-5.6-sol", Margin: 0.775, Spread: 0.100, Surfaces: []string{"graph"}},
	}
	out := make([]validate.Regression, len(banked))
	for i, b := range banked {
		held := b.Margin
		if i < len(after) {
			held = after[i]
		}
		out[i] = validate.Regression{Banked: b, After: held}
	}
	return out
}

// improves is a candidate that helps the cell its hypothesis named, on two
// models, touching a surface the corpus exercises.
func improves() validate.Candidate {
	return validate.Candidate{
		Surface: "blast",
		Targets: []validate.Target{
			{Cell: "aspnetcore", Model: "claude-opus-5", Before: 0.310, After: 0.620, Predict: validate.Up},
			{Cell: "aspnetcore", Model: "gpt-5.6-sol", Before: 0.280, After: 0.540, Predict: validate.Up},
		},
		Regressions: corpus(),
	}
}

func run(t *testing.T, c validate.Candidate) validate.Outcome {
	t.Helper()
	o, err := validate.Run(c)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

// The path is proven in both directions. A path that has only ever said yes has
// not been tested.
func TestACandidateThatMovesItsTargetsAndBreaksNothingIsAccepted(t *testing.T) {
	o := run(t, improves())
	if o.Decision != validate.Accepted {
		t.Fatalf("a clean candidate was %s: %s", o.Decision, o.Reason())
	}
	if len(o.Reasons) != 0 {
		t.Errorf("an accepted candidate carries refusals: %v", o.Reasons)
	}
	if !o.CrossModel {
		t.Error("movement on two models was not recorded as cross-model")
	}
}

// The synthetic regression: a deliberate break in something the corpus covers.
// It has to be caught, and caught on the numbers rather than on a judgement.
func TestADeliberateRegressionInACoveredCellIsRejectedOnTheNumbers(t *testing.T) {
	c := improves()
	// bitwarden-server was banked at 0.625 with a spread of 0.077, so 0.400 is
	// outside its own noise by a wide margin.
	c.Regressions = corpus(0.400)

	o := run(t, c)
	if o.Decision != validate.Rejected {
		t.Fatalf("a candidate that broke a banked cell was %s", o.Decision)
	}
	if len(o.Reasons) != 1 {
		t.Fatalf("rejected for %d reasons, want the one cell that fell: %v", len(o.Reasons), o.Reasons)
	}
	for _, want := range []string{"bitwarden-server", "0.625", "0.400"} {
		if !strings.Contains(o.Reasons[0], want) {
			t.Errorf("the refusal does not carry %q: %q", want, o.Reasons[0])
		}
	}
}

// A cell that wobbles inside its own recorded spread has held. A suite that
// demanded an exact number would fail on the instrument's own noise.
func TestACellInsideItsRecordedSpreadHasHeld(t *testing.T) {
	c := improves()
	// Banked at 0.625 ± 0.077. 0.560 is inside; 0.540 is not.
	c.Regressions = corpus(0.560)
	if o := run(t, c); o.Decision != validate.Accepted {
		t.Errorf("a cell inside its own spread was called a regression: %s", o.Reason())
	}

	c.Regressions = corpus(0.540)
	if o := run(t, c); o.Decision != validate.Rejected {
		t.Errorf("a cell outside its own spread was called a hold: %s", o.Reason())
	}
}

// A hypothesis that survives "nothing happened" predicts nothing.
func TestATargetThatDidNotMoveIsARejection(t *testing.T) {
	c := improves()
	c.Targets[0].After = c.Targets[0].Before

	o := run(t, c)
	if o.Decision != validate.Rejected {
		t.Fatalf("a target that did not move was %s", o.Decision)
	}
	if !strings.Contains(o.Reason(), "did not move as predicted") {
		t.Errorf("the refusal does not say what failed: %q", o.Reason())
	}
}

// A candidate that trades one cell for another says so out loud, and is judged
// against what it said rather than against improvement in general.
func TestAPredictedFallIsAMoveAndAnUnpredictedRiseIsNot(t *testing.T) {
	c := improves()
	c.Targets[1].Predict = validate.Down
	c.Targets[1].Before, c.Targets[1].After = 0.540, 0.280
	if o := run(t, c); o.Decision != validate.Accepted {
		t.Errorf("a predicted fall was treated as a regression: %s", o.Reason())
	}

	c.Targets[1].Before, c.Targets[1].After = 0.280, 0.540
	if o := run(t, c); o.Decision != validate.Rejected {
		t.Errorf("a cell that rose when a fall was predicted was accepted: %s", o.Reason())
	}
}

// Every reason, not the first. Someone who fixes one and is refused by the next
// has learned nothing about the change.
func TestEveryFailureIsReportedNotTheFirst(t *testing.T) {
	c := improves()
	c.Targets[0].After = c.Targets[0].Before
	c.Regressions = corpus(0.400, 0.200)

	o := run(t, c)
	if len(o.Reasons) != 3 {
		t.Fatalf("reported %d reasons, want the target and both fallen cells: %v", len(o.Reasons), o.Reasons)
	}
}

// The corpus size travels with the result. A pass reads as "four of four held",
// never as "no regressions".
func TestEveryResultCarriesTheCorpusSize(t *testing.T) {
	o := run(t, improves())
	if o.CorpusSize != 4 {
		t.Errorf("corpus size %d, want 4", o.CorpusSize)
	}
	if !strings.Contains(o.Reason(), "4 of 4 banked cells held") {
		t.Errorf("the result hides its denominator: %q", o.Reason())
	}

	broken := improves()
	broken.Regressions = corpus(0.400)
	if got := run(t, broken).Reason(); !strings.Contains(got, "3 of 4 banked cells held") {
		t.Errorf("a rejection hides its denominator: %q", got)
	}
}

// The blind spot is measured rather than assumed. The corpus catching a break in
// something it covers says nothing about a break in something it does not, and
// with four cells that is the likelier real case.
func TestARegressionInASurfaceTheCorpusDoesNotCoverPassesAndIsRecordedAsABlindSpot(t *testing.T) {
	c := improves()
	c.Surface = "conventions"

	o := run(t, c)
	if o.Decision != validate.Accepted {
		t.Fatalf("the corpus claimed to catch a break in a surface it never exercises: %s", o.Reason())
	}
	if !o.BlindSpot {
		t.Fatal("a pass over a surface the corpus does not cover was not recorded as a blind spot")
	}
	if !strings.Contains(o.Reason(), "none of the changed surface") {
		t.Errorf("the blind spot is not on the result: %q", o.Reason())
	}

	// And a surface the corpus does exercise is not a blind spot, so the mark
	// is not simply always set.
	if run(t, improves()).BlindSpot {
		t.Error("a candidate touching a covered surface was called a blind spot")
	}
}

// A candidate that names no surface cannot have its coverage assessed, and
// silence is not coverage.
func TestACandidateThatNamesNoSurfaceIsBlindByDefinition(t *testing.T) {
	c := improves()
	c.Surface = ""
	if !run(t, c).BlindSpot {
		t.Error("a candidate naming no surface was reported as covered")
	}
}

// A candidate validated on one model is recorded as single-model, never as
// confirmed. Cycle 08 brings the other tools; until then this is a fact about
// the validation, not a footnote.
func TestMovementOnOneModelIsRecordedAsSingleModel(t *testing.T) {
	c := improves()
	c.Targets = c.Targets[:1]

	o := run(t, c)
	if o.CrossModel {
		t.Error("movement on one model was reported as cross-model")
	}
	if len(o.Models) != 1 || o.Models[0] != "claude-opus-5" {
		t.Errorf("models %v, want the one it moved on", o.Models)
	}
	if !strings.Contains(o.Reason(), "single-model") {
		t.Errorf("the result does not say it is single-model: %q", o.Reason())
	}
}

// It is the MOVEMENT that has to be cross-model, not the measurement. Running an
// unchanged cell on five models confirms nothing.
func TestAModelWhereNothingMovedDoesNotMakeAResultCrossModel(t *testing.T) {
	c := improves()
	c.Targets[1].After = c.Targets[1].Before

	o := run(t, c)
	if o.CrossModel {
		t.Errorf("a model where nothing moved counted toward cross-model: %v", o.Models)
	}
}

// A change validated only on the cells its own hypothesis named has been
// measured against its own intent.
func TestACandidateCannotBeValidatedOnlyAgainstTheCellsItTargeted(t *testing.T) {
	own := []validate.Regression{{
		Banked: validate.Banked{Cell: "aspnetcore", Model: "claude-opus-5", Margin: 0.310, Spread: 0.05},
		After:  0.620,
	}}
	c := improves()
	c.Targets = c.Targets[:1]
	c.Regressions = own

	if _, err := validate.Run(c); err == nil {
		t.Fatal("a candidate was validated against nothing but its own targets")
	}

	c.Regressions = nil
	if _, err := validate.Run(c); err == nil {
		t.Fatal("a candidate was validated against no banked cell at all")
	}
}

func TestACandidateThatPredictsNothingIsRefused(t *testing.T) {
	c := improves()
	c.Targets = nil
	if _, err := validate.Run(c); err == nil {
		t.Fatal("a candidate predicting no cell was validated")
	}
}

// A prediction that is not a direction predicts nothing, and must not pass by
// default.
func TestATargetWithNoPredictedDirectionHasNotMoved(t *testing.T) {
	c := improves()
	c.Targets[0].Predict = ""
	if o := run(t, c); o.Decision != validate.Rejected {
		t.Errorf("a target with no predicted direction was accepted: %s", o.Reason())
	}
}
