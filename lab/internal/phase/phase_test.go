package phase_test

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/phase"
)

// always is the artifact check for a phase that did its job. Advance's own
// refusal is tested with a check that says no.
func always(string) bool { return true }

func TestEveryLeverRoutesWhereItsTableSaysItDoes(t *testing.T) {
	for _, l := range phase.Levers {
		got, err := phase.Next(l.From, l.Verdict, l.Cycle)
		if err != nil {
			t.Fatalf("%s: %v", l.Name, err)
		}
		if got != l.To {
			t.Errorf("%s: %s emitting %q on cycle %d went to %s, want %s",
				l.Name, l.From, l.Verdict, l.Cycle, got, l.To)
		}
	}
}

// Levers is declared to be the fixed test set, so a re-entry that exists in the
// routing and not in that list silently shrinks the set while every test stays
// green. This walks the graph rather than the table, so the two cannot drift
// together.
func TestEveryReEntryIntoAuthoringIsANamedLever(t *testing.T) {
	named := map[string]bool{}
	for _, l := range phase.Levers {
		named[string(l.From)+"/"+string(l.Verdict)] = true
	}
	for _, p := range phase.Graph {
		// The index hands a fresh repository to the author. That is the way in,
		// not a way back, and it carries no rejection.
		if p.Name == phase.Index {
			continue
		}
		for _, v := range p.Verdicts {
			to, err := phase.Next(p.Name, v, 1)
			if err != nil || to != phase.Author {
				continue
			}
			if !named[string(p.Name)+"/"+string(v)] {
				t.Errorf("%s emitting %q re-enters authoring and no lever names it", p.Name, v)
			}
		}
	}
}

// The ceiling is the reason the transition takes a count at all: the same phase
// and the same verdict route two different ways, and nothing but the count
// separates them.
func TestTheSameVerdictReAuthorsBelowTheCeilingAndParksAtIt(t *testing.T) {
	backLevers := []struct {
		from    phase.Name
		verdict phase.Verdict
	}{
		{phase.Author, phase.NoAnchor},
		{phase.Minibench, phase.Requestion},
		{phase.Minibench, phase.NoAnchor},
		{phase.Expand, phase.Requestion},
		{phase.Validate, phase.DoNotPay},
	}
	for _, l := range backLevers {
		for cycle := 1; cycle < phase.AuthoringCeiling; cycle++ {
			got, err := phase.Next(l.from, l.verdict, cycle)
			if err != nil {
				t.Fatalf("%s/%s cycle %d: %v", l.from, l.verdict, cycle, err)
			}
			if got != phase.Author {
				t.Errorf("%s emitting %q on cycle %d went to %s, want a re-entry into %s",
					l.from, l.verdict, cycle, got, phase.Author)
			}
		}
		got, err := phase.Next(l.from, l.verdict, phase.AuthoringCeiling)
		if err != nil {
			t.Fatalf("%s/%s at the ceiling: %v", l.from, l.verdict, err)
		}
		if got != phase.Handoff {
			t.Errorf("%s emitting %q on cycle %d went to %s, want %s: the repository parks at the ceiling",
				l.from, l.verdict, phase.AuthoringCeiling, got, phase.Handoff)
		}
	}
}

// A DoD failure is a re-entry too, but not an authoring one, so the ceiling must
// not swallow it: a cell whose confirmation checks failed on the sixth cycle is
// still diagnosed rather than parked.
func TestADoDFailureIsDiagnosedEvenAtTheAuthoringCeiling(t *testing.T) {
	got, err := phase.Next(phase.Harvest, phase.DoDFail, phase.AuthoringCeiling)
	if err != nil {
		t.Fatal(err)
	}
	if got != phase.Report {
		t.Errorf("a DoD failure on cycle %d went to %s, want %s", phase.AuthoringCeiling, got, phase.Report)
	}
}

// The loop cannot record a loss: a benched cell that is not a win is handed up
// with its numbers, never quietly re-authored and never boarded.
func TestABenchedCellThatIsNotAWinIsHandedUp(t *testing.T) {
	got, err := phase.Next(phase.Report, phase.Diagnosis, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got != phase.Handoff {
		t.Errorf("a diagnosis went to %s, want %s", got, phase.Handoff)
	}
}

func TestAWinWalksFromAnIndexToTheBoard(t *testing.T) {
	walk := []struct {
		at      phase.Name
		verdict phase.Verdict
		next    phase.Name
	}{
		{phase.Index, phase.Auto, phase.Author},
		{phase.Author, phase.Draft, phase.Minibench},
		{phase.Minibench, phase.Proceed, phase.Expand},
		{phase.Expand, phase.Auto, phase.Preflight},
		{phase.Preflight, phase.Auto, phase.Validate},
		{phase.Validate, phase.Pay, phase.Bench},
		{phase.Bench, phase.Auto, phase.Report},
		{phase.Report, phase.Win, phase.Harvest},
		{phase.Harvest, phase.WinConfirmed, phase.Board},
		{phase.Board, phase.Auto, phase.Done},
	}
	at := phase.Index
	for _, s := range walk {
		if at != s.at {
			t.Fatalf("the walk reached %s, want %s", at, s.at)
		}
		next, err := phase.Advance(phase.Outcome{Phase: at, Verdict: s.verdict, Cycle: 1}, always)
		if err != nil {
			t.Fatalf("%s: %v", at, err)
		}
		if next != s.next {
			t.Fatalf("%s emitting %q went to %s, want %s", at, s.verdict, next, s.next)
		}
		at = next
	}
}

func TestAPhaseCannotEmitAVerdictThatIsNotInItsEnum(t *testing.T) {
	// PROCEED is a real verdict, and it belongs to the mini-bench. A draft that
	// emitted it would route to expansion without a probe ever running.
	_, err := phase.Next(phase.Author, phase.Proceed, 1)
	if err == nil {
		t.Fatal("the author was allowed to emit PROCEED")
	}
	if !strings.Contains(err.Error(), "cannot emit") {
		t.Errorf("refusal does not say what was refused: %v", err)
	}
}

func TestAnUnknownPhaseIsRefusedRatherThanRoutedFrom(t *testing.T) {
	if _, err := phase.Next("triage", phase.Auto, 1); err == nil {
		t.Fatal("routed from a phase the graph does not declare")
	}
	if _, err := phase.Advance(phase.Outcome{Phase: "triage", Verdict: phase.Auto, Cycle: 1}, always); err == nil {
		t.Fatal("advanced from a phase the graph does not declare")
	}
}

// The count is the ceiling's only input, so a zero is not a harmless default: it
// is a repository whose cycles are not being counted at all.
func TestACycleCountBelowOneIsRefused(t *testing.T) {
	for _, cycle := range []int{0, -1} {
		if _, err := phase.Next(phase.Author, phase.NoAnchor, cycle); err == nil {
			t.Errorf("cycle %d was accepted", cycle)
		}
	}
}

// The artifact is the fact. A phase that reported success and wrote nothing has
// not finished, and the graph is where that is caught rather than at the one
// phase where it last bit.
func TestAPhaseThatWroteNothingCannotAdvance(t *testing.T) {
	for _, p := range phase.Graph {
		verdict := p.Verdicts[0]
		_, err := phase.Advance(phase.Outcome{Phase: p.Name, Verdict: verdict, Cycle: 1},
			func(string) bool { return false })
		if err == nil {
			t.Errorf("%s advanced without writing %s", p.Name, p.Writes)
			continue
		}
		if !strings.Contains(err.Error(), p.Writes) {
			t.Errorf("%s: refusal does not name the missing artifact: %v", p.Name, err)
		}
	}
}

// Advance asks about the phase's own output, not some other phase's. Checking
// the wrong artifact would let a phase ride on its predecessor's file.
func TestAdvanceChecksThePhasesOwnOutputArtifact(t *testing.T) {
	var asked []string
	_, err := phase.Advance(phase.Outcome{Phase: phase.Minibench, Verdict: phase.Proceed, Cycle: 1},
		func(a string) bool { asked = append(asked, a); return true })
	if err != nil {
		t.Fatal(err)
	}
	want, _ := phase.Lookup(phase.Minibench)
	if len(asked) != 1 || asked[0] != want.Writes {
		t.Errorf("asked about %v, want exactly [%s]", asked, want.Writes)
	}
}

// A phase emits a verdict and the graph decides. If a verdict could name a
// phase, a phase could choose its own successor by saying its name, and the
// separation between the phase that writes a scenario and the phase that
// measures it would be a convention rather than a shape.
func TestNoVerdictNamesAPhase(t *testing.T) {
	for _, p := range phase.Graph {
		for _, v := range p.Verdicts {
			if _, ok := phase.Lookup(phase.Name(v)); ok {
				t.Errorf("%s can emit %q, which is also a phase name", p.Name, v)
			}
		}
	}
}

// A verdict with nowhere to go is a phase that stalls a campaign at three in the
// morning, and it is only discoverable by emitting it.
func TestEveryVerdictOfEveryPhaseRoutesSomewhereDeclared(t *testing.T) {
	for _, p := range phase.Graph {
		for _, v := range p.Verdicts {
			to, err := phase.Next(p.Name, v, 1)
			if err != nil {
				t.Errorf("%s emitting %q: %v", p.Name, v, err)
				continue
			}
			if to == phase.Done {
				continue
			}
			if _, ok := phase.Lookup(to); !ok {
				t.Errorf("%s emitting %q goes to %q, which is not a phase", p.Name, v, to)
			}
		}
	}
}

// Every phase has to be arrivable, or it is a plan file nobody will ever run.
func TestEveryPhaseIsReachedBySomeTransition(t *testing.T) {
	reached := map[phase.Name]bool{phase.Index: true}
	for _, p := range phase.Graph {
		for _, v := range p.Verdicts {
			for cycle := 1; cycle <= phase.AuthoringCeiling; cycle++ {
				if to, err := phase.Next(p.Name, v, cycle); err == nil {
					reached[to] = true
				}
			}
		}
	}
	for _, p := range phase.Graph {
		if !reached[p.Name] {
			t.Errorf("no verdict leads to %s", p.Name)
		}
	}
}

// The plan files in 05-03 are written against these rows, so a phase with no
// declared artifacts is a plan with no stated contract.
func TestEveryPhaseDeclaresItsArtifactsAndAtLeastOneVerdict(t *testing.T) {
	for _, p := range phase.Graph {
		if p.Reads == "" || p.Writes == "" {
			t.Errorf("%s declares reads=%q writes=%q", p.Name, p.Reads, p.Writes)
		}
		if len(p.Verdicts) == 0 {
			t.Errorf("%s declares no verdict; a phase that says nothing cannot be routed from", p.Name)
		}
	}
}

func TestLookupReportsAnUndeclaredPhaseAsAbsent(t *testing.T) {
	if _, ok := phase.Lookup("triage"); ok {
		t.Error("the graph claims to declare a phase it does not have")
	}
}

func TestEmitsIsFalseForAVerdictThePhaseDoesNotDeclare(t *testing.T) {
	p, ok := phase.Lookup(phase.Board)
	if !ok {
		t.Fatal("the board is not in the graph")
	}
	if p.Emits(phase.DoDFail) {
		t.Error("the board claims it can emit a DoD failure")
	}
	if !p.Emits(phase.Auto) {
		t.Error("the board cannot emit the only verdict it has")
	}
}
