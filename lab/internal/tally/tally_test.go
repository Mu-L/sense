package tally_test

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/tally"
)

func run(id string, recall float64, rows ...string) tally.Run {
	return tally.Run{ID: id, CitedRecall: recall, RelatedRecall: recall, GroundedPrecision: 1, Rows: rows}
}

// cell is two arms of two replicates each, at the recalls given.
func cell(sense, baseline [2]float64) tally.Cell {
	return tally.Cell{
		Sense:    []tally.Run{run("s1", sense[0]), run("s2", sense[1])},
		Baseline: []tally.Run{run("b1", baseline[0]), run("b2", baseline[1])},
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestTheHeadlineIsObjectiveCitedRecall(t *testing.T) {
	// What ranks the board decides what the bench is about. Ranked by a blind
	// composite, a +0.44 recall win once rendered as a −0.018 tie.
	got := tally.Of(cell([2]float64{0.80, 0.80}, [2]float64{0.20, 0.20}))

	if !near(got.SenseRecall.Value, 0.80) || !near(got.BaselineRecall.Value, 0.20) {
		t.Errorf("recalls = %s and %s", got.SenseRecall, got.BaselineRecall)
	}
	if !near(got.Margin.Value, 0.60) {
		t.Errorf("Margin = %s, want 0.600", got.Margin)
	}
	if !strings.HasPrefix(got.Headline(), "cited recall:") {
		t.Errorf("the headline leads with %q", got.Headline())
	}
}

func TestTheBlendedFigureIsExactlyItsThreeTerms(t *testing.T) {
	// The weights are inherited, not derived, and changing them requires
	// re-scoring the whole corpus. A test that recomputed them from the code
	// would agree with any weights at all, so they are stated here.
	r := tally.Run{CitedRecall: 1, RelatedRecall: 0, GroundedPrecision: 0}
	if !near(r.BScore(), 0.55) {
		t.Errorf("cited recall weighs %.3f, want 0.55", r.BScore())
	}
	r = tally.Run{CitedRecall: 0, RelatedRecall: 1, GroundedPrecision: 0}
	if !near(r.BScore(), 0.25) {
		t.Errorf("related recall weighs %.3f, want 0.25", r.BScore())
	}
	r = tally.Run{CitedRecall: 0, RelatedRecall: 0, GroundedPrecision: 1}
	if !near(r.BScore(), 0.20) {
		t.Errorf("grounded precision weighs %.3f, want 0.20", r.BScore())
	}
	// And nothing else is in there. A perfect run scores exactly 1.
	r = tally.Run{CitedRecall: 1, RelatedRecall: 1, GroundedPrecision: 1}
	if !near(r.BScore(), 1) {
		t.Errorf("a perfect run scores %.3f, want 1; there is a fourth term", r.BScore())
	}
}

func TestNoEfficiencyOrQualityTermReachesTheScore(t *testing.T) {
	// Efficiency is not a correctness axis and folding it in dilutes the thing
	// being measured. Two runs identical on the three terms score identically,
	// whatever else differed about them.
	slow := tally.Run{ID: "slow", CitedRecall: 0.6, RelatedRecall: 0.6, GroundedPrecision: 0.9}
	fast := tally.Run{ID: "fast", CitedRecall: 0.6, RelatedRecall: 0.6, GroundedPrecision: 0.9}

	if slow.BScore() != fast.BScore() {
		t.Errorf("two runs with the same three terms scored %.3f and %.3f", slow.BScore(), fast.BScore())
	}
}

func TestTheAggregateNeverAppearsWithoutItsSpread(t *testing.T) {
	// Two runs at 0.30 and 0.70 are not one run at 0.50, and a consumer that
	// reads the aggregate without the spread is reading half a result.
	got := tally.Of(cell([2]float64{0.30, 0.70}, [2]float64{0.10, 0.10}))

	if !near(got.SenseRecall.Value, 0.50) {
		t.Fatalf("SenseRecall = %s", got.SenseRecall)
	}
	if !near(got.SenseRecall.Spread, 0.40) {
		t.Errorf("Spread = %.3f, want 0.400", got.SenseRecall.Spread)
	}
	// Rendered anywhere, the spread comes with it.
	if !strings.Contains(got.SenseRecall.String(), "±0.400") {
		t.Errorf("the aggregate renders as %q with no spread", got.SenseRecall)
	}
	if !strings.Contains(got.Headline(), "±") {
		t.Errorf("the headline renders as %q with no spread", got.Headline())
	}
}

func TestTheMarginCarriesTheLargerOfTheTwoArmsSpreads(t *testing.T) {
	// A margin computed from two disagreeing arms is no steadier than the
	// noisier of them.
	got := tally.Of(cell([2]float64{0.80, 0.80}, [2]float64{0.05, 0.35}))

	if !near(got.Margin.Spread, 0.30) {
		t.Errorf("Margin spread = %.3f, want the baseline arm's 0.300", got.Margin.Spread)
	}
}

func TestASingleRunCellIsOpenAndCannotBePublished(t *testing.T) {
	// Enforced rather than noted. The recorded same-cell spreads reach 0.250
	// against a bar of 0.50, so one run is a sample of a distribution whose
	// spread is half the bar.
	c := tally.Cell{Sense: []tally.Run{run("s1", 0.9)}, Baseline: []tally.Run{run("b1", 0.1), run("b2", 0.1)}}

	got := tally.Of(c)

	if !got.Open {
		t.Fatal("a one-run arm was reported as a measurement")
	}
	if err := got.Publishable(); err == nil {
		t.Fatal("an open cell was publishable")
	}
	if !strings.Contains(got.OpenWhy, "sense") {
		t.Errorf("OpenWhy = %q, want it to name the arm that is short", got.OpenWhy)
	}
	if !strings.Contains(got.Headline(), "open") {
		t.Errorf("the headline %q does not say the cell is open", got.Headline())
	}
}

func TestACellWithBothArmsShortSaysSo(t *testing.T) {
	got := tally.Of(tally.Cell{Sense: []tally.Run{run("s1", 0.9)}, Baseline: []tally.Run{run("b1", 0.1)}})

	if !got.Open || !strings.Contains(got.OpenWhy, "both") {
		t.Errorf("OpenWhy = %q, want it to say both arms are short", got.OpenWhy)
	}
}

func TestACompleteCellIsPublishable(t *testing.T) {
	if err := tally.Of(cell([2]float64{0.8, 0.8}, [2]float64{0.1, 0.1})).Publishable(); err != nil {
		t.Errorf("a two-run cell is not publishable: %v", err)
	}
}

// The third run.

func TestAThirdRunIsBoughtByAFlippedVerdict(t *testing.T) {
	// The two paired replicates landed on different sides of the bar: one at
	// +0.70 and one at +0.30 against a floor of +0.50. That is what justifies
	// paying.
	got := tally.Of(cell([2]float64{0.80, 0.40}, [2]float64{0.10, 0.10}))

	if !got.FlippedVerdict {
		t.Fatal("paired replicates on different sides of the bar did not buy a third run")
	}
	if !got.NeedsThirdRun() {
		t.Error("NeedsThirdRun disagrees with FlippedVerdict")
	}
}

func TestAThirdRunIsNeverBoughtByAFlippedRow(t *testing.T) {
	// This is the rule that is easy to get backwards. Individual gold rows flip
	// between runs all the time and that is ordinary variance; paying for a
	// third run every time one did would be paying for noise.
	c := tally.Cell{
		Sense: []tally.Run{
			run("s1", 0.80, "d:one", "d:two", "d:three", "d:four"),
			run("s2", 0.80, "d:one", "d:two", "d:three", "d:five"),
		},
		Baseline: []tally.Run{run("b1", 0.10, "d:one"), run("b2", 0.10, "d:two")},
	}

	got := tally.Of(c)

	if got.NeedsThirdRun() {
		t.Fatal("flipped rows bought a third run")
	}
	// And they are reported anyway, because they are the diagnostic.
	if !slices.Equal(got.FlippedRows, []string{"d:five", "d:four", "d:one", "d:two"}) {
		t.Errorf("FlippedRows = %v, want every row that came and went", got.FlippedRows)
	}
}

func TestTwoRunsOnTheSameSideOfTheBarBuyNothing(t *testing.T) {
	got := tally.Of(cell([2]float64{0.80, 0.75}, [2]float64{0.10, 0.10}))

	if got.NeedsThirdRun() {
		t.Errorf("two runs both clearing the bar bought a third: margins %.2f and %.2f", 0.70, 0.65)
	}
}

func TestTwoRunsBothBelowTheBarBuyNothing(t *testing.T) {
	got := tally.Of(cell([2]float64{0.40, 0.30}, [2]float64{0.10, 0.10}))

	if got.NeedsThirdRun() {
		t.Error("two runs both short of the bar bought a third")
	}
}

func TestAnOpenCellBuysNoThirdRun(t *testing.T) {
	// One replicate cannot disagree with anything, so there is no flip to pay
	// for; what it needs is its second run, not its third.
	got := tally.Of(tally.Cell{Sense: []tally.Run{run("s1", 0.9)}, Baseline: []tally.Run{run("b1", 0.1)}})

	if got.NeedsThirdRun() {
		t.Error("a one-run cell bought a third run")
	}
}

// The confirmation band.

func TestAMarginNearTheBarIsLabelledConfirmBand(t *testing.T) {
	// The band is the largest spread the corpus has actually shown, not a
	// chosen constant.
	got := tally.Of(cell([2]float64{0.60, 0.60}, [2]float64{0.10, 0.10}))

	if !near(got.Margin.Value, 0.50) {
		t.Fatalf("Margin = %s", got.Margin)
	}
	if !got.InConfirmBand {
		t.Error("a margin exactly at the bar is not a clean call")
	}
	if !strings.Contains(got.Headline(), "confirm band") {
		t.Errorf("the headline %q does not say it is a confirm-band result", got.Headline())
	}
}

func TestAMarginWellClearOfTheBarIsACleanCall(t *testing.T) {
	got := tally.Of(cell([2]float64{0.95, 0.95}, [2]float64{0.05, 0.05}))

	if got.InConfirmBand {
		t.Errorf("a margin of %s was called a confirm-band result", got.Margin)
	}
	// And the headline says nothing extra: a clean call is a clean call.
	line := got.Headline()
	if strings.Contains(line, "confirm band") || strings.Contains(line, "open") {
		t.Errorf("the headline for a clean call reads %q", line)
	}
}

func TestABaselineArmThatIsShortLeavesTheCellOpen(t *testing.T) {
	// Named per arm, because "this cell is open" does not say which run to buy.
	got := tally.Of(tally.Cell{
		Sense:    []tally.Run{run("s1", 0.8), run("s2", 0.8)},
		Baseline: []tally.Run{run("b1", 0.1)},
	})

	if !got.Open {
		t.Fatal("a one-run baseline arm was reported as a measurement")
	}
	if !strings.Contains(got.OpenWhy, "baseline") {
		t.Errorf("OpenWhy = %q, want it to name the baseline arm", got.OpenWhy)
	}
}

func TestTheBandComesFromTheMeasuredSpreads(t *testing.T) {
	// Derived rather than chosen, so it moves only when a new measurement moves
	// it. The recorded spreads are 0.077, 0.154, 0.154 and 0.250.
	if !near(tally.ConfirmBand, 0.250) {
		t.Errorf("ConfirmBand = %.3f, want the largest observed spread 0.250", tally.ConfirmBand)
	}
}

func TestAnOpenCellIsNotGivenABandLabelItHasNotEarned(t *testing.T) {
	got := tally.Of(tally.Cell{Sense: []tally.Run{run("s1", 0.6)}, Baseline: []tally.Run{run("b1", 0.1)}})

	if got.InConfirmBand {
		t.Error("an open cell was labelled a confirm-band result")
	}
}

// The retry.

func TestAParkedRunIsNeverScored(t *testing.T) {
	// Keeping it would let a cell be re-read until it said the right thing.
	c := tally.Cell{
		Sense:    []tally.Run{run("s1", 0.10), run("s2", 0.80), run("s3", 0.80)},
		Baseline: []tally.Run{run("b1", 0.10), run("b2", 0.10)},
	}
	if err := c.Park("s1", "the agent CLI failed to authenticate"); err != nil {
		t.Fatalf("Park: %v", err)
	}

	got := tally.Of(c)

	if !near(got.SenseRecall.Value, 0.80) {
		t.Errorf("SenseRecall = %s, want the parked run excluded", got.SenseRecall)
	}
	if !near(got.SenseRecall.Spread, 0) {
		t.Errorf("Spread = %.3f, want the parked run excluded from it too", got.SenseRecall.Spread)
	}
}

func TestASecondRetryOnTheSameArmIsRefused(t *testing.T) {
	// One retry, never a loop.
	c := tally.Cell{Sense: []tally.Run{run("s1", 0.1), run("s2", 0.2), run("s3", 0.8)}}
	if err := c.Park("s1", "infrastructure"); err != nil {
		t.Fatalf("Park: %v", err)
	}

	err := c.Park("s2", "infrastructure again")

	if err == nil {
		t.Fatal("a second retry was allowed on the same arm")
	}
	if !strings.Contains(err.Error(), "never a loop") {
		t.Errorf("error = %v, want it to say why", err)
	}
}

func TestAParkedRunMustSayWhatItWasReplacedFor(t *testing.T) {
	// A retry with no reason is indistinguishable from a run somebody did not
	// like the look of.
	c := tally.Cell{Sense: []tally.Run{run("s1", 0.1), run("s2", 0.8)}}

	if err := c.Park("s1", ""); err == nil {
		t.Fatal("a run was parked with no reason given")
	}
}

func TestParkingARunThatIsNotInTheCellIsRefused(t *testing.T) {
	c := tally.Cell{Sense: []tally.Run{run("s1", 0.1)}}

	if err := c.Park("s9", "infrastructure"); err == nil {
		t.Fatal("a run outside the cell was parked")
	}
}

func TestParkingLeavesTheCellOpenWhenItRunsShort(t *testing.T) {
	c := tally.Cell{
		Sense:    []tally.Run{run("s1", 0.1), run("s2", 0.8)},
		Baseline: []tally.Run{run("b1", 0.1), run("b2", 0.1)},
	}
	if err := c.Park("s1", "the model id resolved to nothing"); err != nil {
		t.Fatalf("Park: %v", err)
	}

	got := tally.Of(c)

	if !got.Open {
		t.Fatal("a cell left with one scored run reported itself as a measurement")
	}
	if err := got.Publishable(); err == nil {
		t.Error("it was publishable anyway")
	}
}

func TestAnEmptyCellSaysNothingRatherThanZero(t *testing.T) {
	got := tally.Of(tally.Cell{})

	if !got.Open {
		t.Error("an empty cell reported itself as a measurement")
	}
	if got.SenseRecall.Value != 0 || got.SenseRecall.Spread != 0 {
		t.Errorf("SenseRecall = %s for a cell with no runs", got.SenseRecall)
	}
}

func TestEveryRunsScoreIsKeptOnTheResult(t *testing.T) {
	// A reader has to be able to see the disagreement, not only its average.
	c := cell([2]float64{0.30, 0.70}, [2]float64{0.10, 0.10})

	got := tally.Of(c)

	if len(got.Cell.Sense) != 2 || got.Cell.Sense[0].CitedRecall != 0.30 {
		t.Errorf("the result does not carry each run: %+v", got.Cell.Sense)
	}
}
