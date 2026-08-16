package gate_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/gate"
)

// Each gate gets its own test asserting the refusal. The test for a gate is not
// "does it exist" but "does it refuse on its condition".

func TestABaselineThatAlreadyAssemblesTheSetIsRefused(t *testing.T) {
	// There is nothing left for the treatment to find, so whatever the sense
	// arm scores, the cell cannot show a difference that means anything.
	err := gate.BaselineAssemblesTheSet(12, 12)

	if err == nil {
		t.Fatal("a baseline that reached every row was allowed to be paid for")
	}
	if !strings.Contains(err.Error(), "12 of 12") {
		t.Errorf("the refusal reads %q, which does not say what the baseline reached", err)
	}
}

func TestABaselineThatLeavesRowsToFindIsNotRefused(t *testing.T) {
	if err := gate.BaselineAssemblesTheSet(4, 12); err != nil {
		t.Errorf("a baseline at 4 of 12 was refused: %v", err)
	}
}

func TestAGroupWithNoRowsIsNotTreatedAsAssembled(t *testing.T) {
	// Zero of zero is not a baseline that found everything; it is a group that
	// has nothing in it, and refusing it here would hide that.
	if err := gate.BaselineAssemblesTheSet(0, 0); err != nil {
		t.Errorf("an empty group was refused as assembled: %v", err)
	}
}

func TestABaselineAboveTheArithmeticCeilingIsRefused(t *testing.T) {
	// Recall caps at 1.00, so a baseline of 0.55 leaves at most 0.45 of margin
	// and a +0.50 bar is unreachable. Paying to discover that is paying for
	// arithmetic.
	err := gate.ArithmeticCeiling(0.55, true)

	if err == nil {
		t.Fatal("a cell that cannot reach the bar was allowed to be paid for")
	}
	if !strings.Contains(err.Error(), "0.450") {
		t.Errorf("the refusal reads %q, which does not say how much margin is left", err)
	}
}

func TestABaselineExactlyAtTheCeilingIsNotRefused(t *testing.T) {
	// 0.50 leaves exactly 0.50, which is the bar. Refusing it would be refusing
	// a cell that can, arithmetically, still clear.
	if err := gate.ArithmeticCeiling(0.50, true); err != nil {
		t.Errorf("a baseline at exactly the ceiling was refused: %v", err)
	}
}

func TestAFreshCellWithNoRecordedBaselineIsNotGated(t *testing.T) {
	// For a fresh cell there is no baseline until a mini-bench has run. A
	// version that estimated one would be a screen rather than a gate, which is
	// why this check was kept out of the planner.
	if err := gate.ArithmeticCeiling(0.99, false); err != nil {
		t.Errorf("a cell with no recorded baseline was gated on an assumed one: %v", err)
	}
}

func TestThePayPathWithoutAMiniBenchIsRefused(t *testing.T) {
	err := gate.MiniBenchFirst(false)

	if err == nil {
		t.Fatal("the pay path was open with no mini-bench")
	}
	if gate.MiniBenchFirst(true) != nil {
		t.Error("a cell with a mini-bench was refused")
	}
}

func TestThePayPathWithoutAValidationRunIsRefused(t *testing.T) {
	err := gate.ValidationRun(gate.Validation{})

	if err == nil {
		t.Fatal("the pay path was open with no validation run")
	}
	if !strings.Contains(err.Error(), "unscored") {
		t.Errorf("the refusal reads %q, which does not say what a validation run is", err)
	}
}

func TestAScoredValidationRunIsRefused(t *testing.T) {
	// It is unscored by law, because a number from it reads as a result.
	err := gate.ValidationRun(gate.Validation{
		Ran: true, Scored: true, Wall: 8 * time.Minute, RealWall: 8 * time.Minute,
	})

	if err == nil {
		t.Fatal("a scored validation run was accepted")
	}
}

func TestAValidationAtAShorterWallIsRefused(t *testing.T) {
	// A validation at a shorter wall proves nothing about the run that matters:
	// the thing it is meant to rule out is a session that cannot finish.
	err := gate.ValidationRun(gate.Validation{
		Ran: true, Wall: 2 * time.Minute, RealWall: 8 * time.Minute,
	})

	if err == nil {
		t.Fatal("a validation at a quarter of the real wall was accepted")
	}
	if !strings.Contains(err.Error(), "2m0s") {
		t.Errorf("the refusal reads %q, which does not say what wall it ran at", err)
	}
}

func TestAValidationAtTheRealWallPasses(t *testing.T) {
	if err := gate.ValidationRun(gate.Validation{
		Ran: true, Wall: 8 * time.Minute, RealWall: 8 * time.Minute,
	}); err != nil {
		t.Errorf("a proper validation run was refused: %v", err)
	}
}

func TestPublishingASingleRunCellIsRefused(t *testing.T) {
	// The recorded same-cell spreads reach 0.250 against a bar of 0.50, so one
	// run is a sample of a distribution whose spread is half the bar.
	err := gate.SingleRunCell(1, 2)

	if err == nil {
		t.Fatal("a one-run arm was published")
	}
	if !strings.Contains(err.Error(), "1 sense") {
		t.Errorf("the refusal reads %q, which does not say which arm is short", err)
	}
	if gate.SingleRunCell(2, 2) != nil {
		t.Error("a two-run cell was refused")
	}
}

func TestASecondRetryIsRefused(t *testing.T) {
	// One retry, never a loop.
	err := gate.RetryBound(2, false)

	if err == nil {
		t.Fatal("a second retry was allowed")
	}
	if gate.RetryBound(1, false) != nil {
		t.Error("a first retry was refused")
	}
}

func TestScoringAParkedRunIsRefused(t *testing.T) {
	// Keeping a replaced run scorable would let a cell be re-read until it said
	// the right thing.
	if err := gate.RetryBound(1, true); err == nil {
		t.Fatal("a parked run was scored")
	}
}

// Every gate that fired, not the first.

func TestEveryGateThatFiredIsReported(t *testing.T) {
	// An operator who fixes one thing only to be refused by the next has
	// learned nothing about the cell, and an aggregate that returns the first
	// failure quietly makes the order significant.
	got := gate.Refusals(gate.Decision{
		BaselineReached: 12, GroupTotal: 12,
		BaselineRecall: 0.9, BaselineRecorded: true,
		MiniBenchRan: false,
		SenseRuns:    1, BaselineRuns: 1,
		Retries: 3, ScoredAParkedRun: true,
	})

	if len(got) != 6 {
		t.Fatalf("Refusals reported %d of 6 gates:\n%v", len(got), got)
	}
}

func TestACleanDecisionIsRefusedByNothing(t *testing.T) {
	if got := gate.Refusals(cleanDecision()); len(got) != 0 {
		t.Errorf("a clean decision was refused by %v", got)
	}
}

func cleanDecision() gate.Decision {
	return gate.Decision{
		BaselineReached: 4, GroupTotal: 12,
		BaselineRecall: 0.21, BaselineRecorded: true,
		MiniBenchRan: true,
		Validation:   gate.Validation{Ran: true, Wall: 8 * time.Minute, RealWall: 8 * time.Minute},
		SenseRuns:    2, BaselineRuns: 2,
		Retries: 0,
	}
}

func TestNoBypassReachesTheGates(t *testing.T) {
	// A gate with an override is a suggestion, and the moment of frustration is
	// exactly when it would be used. The decision has nowhere to put one, and
	// this fails the day somebody adds a field for it.
	d := cleanDecision()
	d.MiniBenchRan = false

	if got := gate.Refusals(d); len(got) != 1 {
		t.Fatalf("Refusals = %v, want the mini-bench gate", got)
	}
}

func TestCostIsAbsentFromEveryVerdictPath(t *testing.T) {
	// Checked by its absence rather than asserted in prose. Costing more is a
	// product finding, not a stopper: a cell that wins its discriminator while
	// spending more still wins, and the way to keep that true is for there to
	// be nowhere to put a cost.
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	money := []string{"cost", "token", "dollar", "price", "spend", "billed", "budget"}

	for _, pkg := range []string{"gate", "tally"} {
		dir := filepath.Join(root, "lab", "internal", pkg)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			for _, decl := range declaredNames(t, filepath.Join(dir, name), b) {
				for _, word := range money {
					if strings.Contains(strings.ToLower(decl), word) {
						t.Errorf("%s/%s declares %q; cost must not be able to reach a verdict", pkg, name, decl)
					}
				}
			}
		}
	}
}

// declaredNames is every identifier a file declares: types, fields, constants,
// variables and functions. Comments are deliberately not searched — the
// prohibition is on cost being computable here, not on the word being written
// down to explain why it is not.
func declaredNames(t *testing.T, path string, src []byte) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, src, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	var names []string
	ast.Inspect(file, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.Ident:
			names = append(names, decl.Name)
		case *ast.Field:
			for _, name := range decl.Names {
				names = append(names, name.Name)
			}
		}
		return true
	})
	return names
}
