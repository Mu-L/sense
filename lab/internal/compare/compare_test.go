package compare_test

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/compare"
)

func aReport(subjects ...compare.Subject) compare.Report {
	return compare.Report{
		Scenario: "locate the run supervisor", Repo: "selfcheck", Group: "supervisor",
		Model: "claude-opus-5", Wall: "6m0s", Subjects: subjects,
	}
}

// Every delta is against the untreated arm, and a table without one must not
// print deltas against whatever happened to come first.
func TestEveryDeltaIsAgainstTheUntreatedArm(t *testing.T) {
	out := aReport(
		compare.Subject{ID: "untreated", Baseline: true, Recall: 0.25, Runs: 2, Executor: "isolated-home"},
		compare.Subject{ID: "sense-main", Recall: 0.75, Runs: 2, Executor: "isolated-home"},
		compare.Subject{ID: "archmcp", Recall: 0.50, Runs: 2, Executor: "isolated-home"},
	).Render()

	if !strings.Contains(out, "+0.5000") {
		t.Errorf("the treated arm's delta against the baseline is missing:\n%s", out)
	}
	if !strings.Contains(out, "+0.2500") {
		t.Errorf("the competitor's delta against the baseline is missing:\n%s", out)
	}
}

// A comparison with no untreated arm has nothing to be a delta from, and
// printing one anyway would invent a number.
func TestAComparisonWithNoBaselineSaysSoRatherThanInventingDeltas(t *testing.T) {
	out := aReport(
		compare.Subject{ID: "sense-main", Recall: 0.75, Runs: 2, Executor: "isolated-home"},
		compare.Subject{ID: "archmcp", Recall: 0.50, Runs: 2, Executor: "isolated-home"},
	).Render()

	if !strings.Contains(out, "NO BASELINE") {
		t.Errorf("a comparison with nothing to compare against read as a normal one:\n%s", out)
	}
	if strings.Contains(out, "+0.") || strings.Contains(out, "-0.") {
		t.Errorf("deltas were printed with no baseline to take them from:\n%s", out)
	}
}

// The rule that does not soften when the other name is a rival's: a subject
// that loses or ties is in the table, in its own place.
func TestASubjectThatLosesIsInTheTable(t *testing.T) {
	out := aReport(
		compare.Subject{ID: "untreated", Baseline: true, Recall: 0.60, Runs: 2, Executor: "isolated-home"},
		compare.Subject{ID: "sense-main", Recall: 0.40, Runs: 2, Executor: "isolated-home"},
		compare.Subject{ID: "archmcp", Recall: 0.60, Runs: 2, Executor: "isolated-home"},
	).Render()

	if !strings.Contains(out, "-0.2000") {
		t.Errorf("a subject that lost to the baseline is not reported as losing:\n%s", out)
	}
	if !strings.Contains(out, "+0.0000") {
		t.Errorf("a subject that tied the baseline is not reported as tying:\n%s", out)
	}
}

// Container setup lands inside the measured wall, so a comparison in which one
// subject is containerised and another is not carries a difference that is
// about isolation rather than about code intelligence. A reader must not have
// to infer it.
func TestTheTableStatesWhichSubjectsPaidContainerOverhead(t *testing.T) {
	out := aReport(
		compare.Subject{ID: "untreated", Baseline: true, Recall: 0.25, Runs: 2, Executor: "isolated-home"},
		compare.Subject{ID: "rival", Recall: 0.50, Runs: 2, Executor: "container", Containerised: true},
	).Render()

	if !strings.Contains(out, "CONTAINER OVERHEAD") {
		t.Errorf("a mixed comparison does not say who paid container overhead:\n%s", out)
	}
	if !strings.Contains(out, "paid container overhead inside the wall") {
		t.Errorf("the containerised subject's row does not say so:\n%s", out)
	}
}

func TestAComparisonWhereNobodyIsContainerisedSaysNothingAboutOverhead(t *testing.T) {
	out := aReport(
		compare.Subject{ID: "untreated", Baseline: true, Recall: 0.25, Runs: 2, Executor: "isolated-home"},
		compare.Subject{ID: "sense-main", Recall: 0.75, Runs: 2, Executor: "isolated-home"},
	).Render()

	if strings.Contains(out, "CONTAINER OVERHEAD") {
		t.Errorf("a note about overhead nobody paid:\n%s", out)
	}
}

// The mark travels. A number derived from a capture that was cut off is a
// provisional result rather than a clean low one, and a table that dropped the
// mark would be the exact failure the mark exists to prevent.
func TestAProvisionalSubjectCarriesItsMarkIntoTheTable(t *testing.T) {
	out := aReport(
		compare.Subject{ID: "untreated", Baseline: true, Recall: 0.25, Runs: 2, Executor: "isolated-home"},
		compare.Subject{ID: "sense-main", Recall: 0.10, Runs: 1, Executor: "isolated-home",
			Why: "the session did not finish; there is no closing result event"},
	).Render()

	if !strings.Contains(out, "PROVISIONAL (sense-main)") {
		t.Errorf("a provisional number reads as a clean low one:\n%s", out)
	}
	if !strings.Contains(out, "did not finish") {
		t.Errorf("the mark does not say what made it provisional:\n%s", out)
	}
}

// The spread is what says whether a delta is bigger than the noise, so it is
// beside every number rather than in a footnote.
func TestTheSpreadIsBesideEveryNumber(t *testing.T) {
	out := aReport(
		compare.Subject{ID: "untreated", Baseline: true, Recall: 0.25, Runs: 3, Spread: 0.12, Executor: "isolated-home"},
		compare.Subject{ID: "sense-main", Recall: 0.75, Runs: 3, Spread: 0.30, Executor: "isolated-home"},
	).Render()

	for _, want := range []string{"0.1200", "0.3000", "spread"} {
		if !strings.Contains(out, want) {
			t.Errorf("the table does not carry %q:\n%s", want, out)
		}
	}
}

// The baseline reads first because that is where a reader looks for it, and two
// subjects that tie must not swap places between renders.
func TestTheOrderIsBaselineFirstThenByRecallAndIsStable(t *testing.T) {
	r := aReport(
		compare.Subject{ID: "zulu", Recall: 0.50, Runs: 1, Executor: "isolated-home"},
		compare.Subject{ID: "sense-main", Recall: 0.75, Runs: 1, Executor: "isolated-home"},
		compare.Subject{ID: "untreated", Baseline: true, Recall: 0.25, Runs: 1, Executor: "isolated-home"},
		compare.Subject{ID: "alpha", Recall: 0.50, Runs: 1, Executor: "isolated-home"},
	)

	out := r.Render()
	order := []string{"untreated", "sense-main", "alpha", "zulu"}
	at := -1
	for _, id := range order {
		i := strings.Index(out, "  "+id+" ")
		if i <= at {
			t.Fatalf("%s is out of order in:\n%s", id, out)
		}
		at = i
	}
	if again := r.Render(); again != out {
		t.Error("two renders of the same report differ, so nobody can diff them")
	}
}

// A comparison describes someone else's product. Publishing it is a separate
// decision, and the output says so rather than relying on whoever reads it to
// remember.
func TestTheTableSaysItIsNotPublished(t *testing.T) {
	out := aReport(compare.Subject{ID: "untreated", Baseline: true, Recall: 0.25, Runs: 1, Executor: "isolated-home"}).Render()

	if !strings.Contains(out, "NOT PUBLISHED") {
		t.Errorf("the comparison does not say it goes no further than this repository:\n%s", out)
	}
}

// Every subject is asked the same question at the same budget, and the table
// says which budget rather than leaving it to a run record nobody opens.
func TestTheBudgetIsOnTheTable(t *testing.T) {
	out := aReport(compare.Subject{ID: "untreated", Baseline: true, Recall: 0.25, Runs: 1, Executor: "isolated-home"}).Render()

	if !strings.Contains(out, "6m0s, the same for every subject") {
		t.Errorf("the budget is missing from the table:\n%s", out)
	}
}
