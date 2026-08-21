package say

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// The reading a win produces: what each arm reached, what the bar is, and what
// this gap is against it, with nothing left for the reader to work out.
func TestAWinNamesTheBarItCleared(t *testing.T) {
	got := Pair{Sense: 1.00, Baseline: 0.40, Floor: 0.50}.Sentence()

	for _, want := range []string{"100%", "40%", "We need a 50-point gap", "This is 60."} {
		if !strings.Contains(got, want) {
			t.Errorf("the sentence does not say %q:\n%s", want, got)
		}
	}
}

// A gap that did not clear the bar is not a small win. It says what it was
// short of, in the same terms the bar was stated in.
func TestAGapBelowTheBarSaysWhatItWasShortOf(t *testing.T) {
	p := Pair{Sense: 0.73, Baseline: 0.33, Floor: 0.50}
	got := p.Sentence()

	if p.Won() {
		t.Errorf("a 40-point gap against a 50-point bar reads as a win")
	}
	for _, want := range []string{"73%", "33%", "40-point gap", "short of the 50 we need"} {
		if !strings.Contains(got, want) {
			t.Errorf("the sentence does not say %q:\n%s", want, got)
		}
	}
}

// No advantage is its own result, not a two-point win. The cycle-2 mini-bench
// on mastodon read sense 0.389 against baseline 0.444 and the finding was that
// the arm never called sense_blast — a sentence reporting a gap of -6 would
// have buried it.
func TestNoAdvantageIsSaidPlainly(t *testing.T) {
	for _, p := range []Pair{
		{Sense: 0.44, Baseline: 0.44, Floor: 0.50},
		{Sense: 0.389, Baseline: 0.444, Floor: 0.50},
	} {
		got := p.Sentence()
		if !strings.Contains(got, "Sense gave no advantage") {
			t.Errorf("Pair%+v reads as a win or a near miss:\n%s", p, got)
		}
		if p.Won() {
			t.Errorf("Pair%+v reads as a win", p)
		}
	}
}

// The gap a reader computes from the two printed percentages is the gap the
// sentence states. 0.874 and 0.204 print as 87 and 20, and the difference of
// the unrounded recalls is 67.0 — but 0.875 and 0.204 print as 88 and 20 while
// their unrounded difference rounds to 67, which is the arithmetic that would
// fail in front of the person it is explaining itself to.
func TestThePrintedGapIsTheDifferenceOfThePrintedPercentages(t *testing.T) {
	digits := regexp.MustCompile(`(\d+)%`)
	for _, p := range []Pair{
		{Sense: 0.875, Baseline: 0.204, Floor: 0.50},
		{Sense: 0.874, Baseline: 0.204, Floor: 0.50},
		{Sense: 0.666, Baseline: 0.333, Floor: 0.30},
		{Sense: 0.735, Baseline: 0.335, Floor: 0.50},
	} {
		got := p.Sentence()
		shown := digits.FindAllStringSubmatch(got, -1)
		if len(shown) != 2 {
			t.Fatalf("Pair%+v printed %d percentages, want two:\n%s", p, len(shown), got)
		}
		sense, baseline := number(t, shown[0][1]), number(t, shown[1][1])
		if !strings.Contains(got, strconv.Itoa(sense-baseline)) {
			t.Errorf("Pair%+v prints %d%% and %d%% and never states %d:\n%s",
				p, sense, baseline, sense-baseline, got)
		}
	}
}

// A pair with no bar reports the gap and does not judge it. A scenario that
// states no floor has not lost, and a sentence claiming it cleared something
// would be inventing the bar it cleared.
func TestAPairWithNoBarIsNotJudged(t *testing.T) {
	p := Pair{Sense: 0.90, Baseline: 0.10}
	got := p.Sentence()

	if p.Won() {
		t.Errorf("a pair with no floor reads as a win")
	}
	if !strings.Contains(got, "80-point gap") || !strings.Contains(got, "no bar set") {
		t.Errorf("the sentence does not report an unbarred gap:\n%s", got)
	}
}

// A gap exactly on the bar is a win. The method's own wording is "at or above",
// and an instrument that read it as "above" would refuse the cell that sits on
// the line it was designed around.
func TestAGapExactlyOnTheBarIsAWin(t *testing.T) {
	if !(Pair{Sense: 0.90, Baseline: 0.40, Floor: 0.50}).Won() {
		t.Errorf("a 50-point gap against a 50-point bar is not read as a win")
	}
}

// No recall reaches the page in the form the scorer produces it.
func TestNoSentenceCarriesARawRecall(t *testing.T) {
	for _, p := range []Pair{
		{Sense: 1.00, Baseline: 0.40, Floor: 0.50},
		{Sense: 0.73, Baseline: 0.33, Floor: 0.50},
		{Sense: 0.44, Baseline: 0.44, Floor: 0.50},
		{Sense: 0.90, Baseline: 0.10},
	} {
		// A digit either side of a point. "This is 60." ends a sentence and is
		// not a recall; 0.60 is one.
		if got := p.Sentence(); regexp.MustCompile(`\d\.\d`).MatchString(got) {
			t.Errorf("Pair%+v prints a decimal:\n%s", p, got)
		}
	}
}

func number(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("%q is not a number: %v", s, err)
	}
	return n
}

// A cell is run more than once because the spread within one cell reaches a
// quarter of the group against a bar of half of it. The mean is what may be
// reported, and it carries the count that produced it.
func TestTheMeanOfSeveralRunsIsWhatMayBeReported(t *testing.T) {
	runs := []Pair{
		{Sense: 0.87, Baseline: 0.20, Floor: 0.50},
		{Sense: 0.80, Baseline: 0.27, Floor: 0.50},
	}

	mean := Mean(runs)
	if got := mean.Gap(); got != 60 {
		t.Errorf("the gap of the mean is %d, want 60", got)
	}
	if mean.Floor != 0.50 {
		t.Errorf("the mean dropped the bar it is measured against: %+v", mean)
	}
	if !mean.Won() {
		t.Errorf("the mean of two clearing runs does not clear: %+v", mean)
	}
}

// A mean of nothing is not a result, and it must not read as one.
func TestTheMeanOfNoRunsIsNotAWin(t *testing.T) {
	mean := Mean(nil)
	if mean.Won() || mean.Gap() != 0 {
		t.Errorf("the mean of no runs reads as %+v", mean)
	}
}

// A gap never appears without the count that produced it: a single run is a
// draw rather than a reading, and the sentence has to say which it is.
func TestAGapCarriesTheCountThatProducedIt(t *testing.T) {
	if got := Runs(1); got != "on a single run" {
		t.Errorf("Runs(1) = %q", got)
	}
	if got := Runs(3); got != "averaged over 3 runs" {
		t.Errorf("Runs(3) = %q", got)
	}
}
