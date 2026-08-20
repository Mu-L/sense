package loop_test

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/loop"
	"github.com/luuuc/sense/lab/internal/phase"
)

func wrote(string) bool { return true }

// The three verdicts that send work back, with the table each one carries.
var levers = []struct {
	from    phase.Name
	verdict phase.Verdict
	table   string
}{
	{phase.Author, phase.NoAnchor, "no group in this repo has a question the anchor could carry"},
	{phase.Minibench, phase.Requestion, "baseline reached 14 of 18 rows in nine tool calls"},
	{phase.Expand, phase.Requestion, "gold row d:notifier does not hold at its line"},
	{phase.Validate, phase.DoNotPay, "credit table: 4 free rows floor the baseline at 0.286"},
}

func TestEveryLeverCarriesTheTableThatRejectedItIntoTheNextAttempt(t *testing.T) {
	for _, l := range levers {
		s := loop.Start("mastodon", "the notification pipeline").Drafted("draft-1.yaml")
		next, at, err := s.Advance(l.from, l.verdict, loop.Rejection{Table: l.table}, wrote)
		if err != nil {
			t.Fatalf("%s/%s: %v", l.from, l.verdict, err)
		}
		if at != phase.Author {
			t.Fatalf("%s/%s routed to %s, want a re-entry into the author", l.from, l.verdict, at)
		}
		if len(next.Carried) != 1 {
			t.Fatalf("%s/%s carried %d rejections, want 1", l.from, l.verdict, len(next.Carried))
		}
		got := next.Carried[0]
		if got.Table != l.table || got.Phase != l.from || got.Verdict != l.verdict {
			t.Errorf("%s/%s carried %+v, want the phase, the verdict and the table that refused it", l.from, l.verdict, got)
		}
	}
}

// A re-entry that carries nothing is a fresh guess wearing the previous
// attempt's number, and it is the failure this whole package exists against.
func TestAReEntryWithNoTableIsRefused(t *testing.T) {
	s := loop.Start("mastodon", "the notification pipeline")
	_, _, err := s.Advance(phase.Minibench, phase.Requestion, loop.Rejection{}, wrote)
	if err == nil {
		t.Fatal("a re-entry carrying no rejection was allowed")
	}
	if !strings.Contains(err.Error(), "fresh guess") {
		t.Errorf("refusal does not say what is wrong: %v", err)
	}
}

// A forward transition is not a re-entry, so it neither demands a table nor
// files an attempt.
func TestAForwardVerdictNeitherCarriesNorFiles(t *testing.T) {
	s := loop.Start("mastodon", "the notification pipeline").Drafted("draft-1.yaml")
	next, at, err := s.Advance(phase.Author, phase.Draft, loop.Rejection{}, wrote)
	if err != nil {
		t.Fatal(err)
	}
	if at != phase.Minibench {
		t.Fatalf("a draft went to %s, want %s", at, phase.Minibench)
	}
	if next.Cycle != 1 || len(next.Attempts) != 0 || len(next.Carried) != 0 {
		t.Errorf("a forward step changed the loop position: %+v", next)
	}
}

// Six cycles, three different levers, and at the end everything the first
// attempt knew is still there. This is the property the rewrite is most likely
// to lose, because losing it is invisible.
func TestNothingIsDeletedAcrossSixCycles(t *testing.T) {
	s := loop.Start("mastodon", "the notification pipeline").Drafted("draft-1.yaml")
	for cycle := 1; cycle < phase.AuthoringCeiling; cycle++ {
		l := levers[(cycle-1)%len(levers)]
		var err error
		s, _, err = s.Advance(l.from, l.verdict, loop.Rejection{Table: l.table}, wrote)
		if err != nil {
			t.Fatalf("cycle %d: %v", cycle, err)
		}
		if s.NeedsAnchor {
			if s, err = s.ReAnchor("the notification pipeline, from the delivery side"); err != nil {
				t.Fatalf("cycle %d: %v", cycle, err)
			}
		}
		s = s.Drafted("draft-" + string(rune('1'+cycle)) + ".yaml")
	}

	if s.Cycle != phase.AuthoringCeiling {
		t.Fatalf("after five re-entries the loop is on cycle %d, want %d", s.Cycle, phase.AuthoringCeiling)
	}
	if len(s.Attempts) != phase.AuthoringCeiling-1 {
		t.Errorf("kept %d attempts, want every one of the %d that were rejected",
			len(s.Attempts), phase.AuthoringCeiling-1)
	}
	if len(s.Carried) != phase.AuthoringCeiling-1 {
		t.Errorf("carried %d rejections, want all of them: the next attempt answers every one",
			len(s.Carried))
	}
	for i, a := range s.Attempts {
		if a.Cycle != i+1 {
			t.Errorf("attempt %d is filed as cycle %d", i, a.Cycle)
		}
		if a.Draft == "" || a.Anchor == "" || a.Rejected.Table == "" {
			t.Errorf("attempt %d lost something: %+v", i, a)
		}
	}
	if s.Anchor == "" {
		t.Error("the anchor was lost")
	}
}

// A sub-floor cell is a question the anchor could not carry, not necessarily a
// bad anchor. Re-questioning must leave the anchor exactly where it was.
func TestReQuestioningNeverChangesTheAnchor(t *testing.T) {
	const anchor = "the notification pipeline"
	s := loop.Start("mastodon", anchor).Drafted("draft-1.yaml")
	s, _, err := s.Advance(phase.Minibench, phase.Requestion, loop.Rejection{Table: "the probe read"}, wrote)
	if err != nil {
		t.Fatal(err)
	}
	if s.Anchor != anchor {
		t.Errorf("the anchor became %q after a re-question", s.Anchor)
	}
	if s.NeedsAnchor {
		t.Error("a re-question asked for a new anchor; that is a separate decision")
	}
	if _, err := s.ReAnchor("something else"); err == nil {
		t.Error("re-anchoring was allowed without a verdict asking for it")
	}
}

// NO-ANCHOR records that an anchor is owed. It does not pick one, and it does
// not throw away the one that failed.
func TestANoAnchorVerdictAsksForAnAnchorRatherThanDiscardingOne(t *testing.T) {
	const anchor = "the notification pipeline"
	s := loop.Start("mastodon", anchor).Drafted("draft-1.yaml")
	s, _, err := s.Advance(phase.Minibench, phase.NoAnchor, loop.Rejection{Table: "no row discriminated"}, wrote)
	if err != nil {
		t.Fatal(err)
	}
	if !s.NeedsAnchor {
		t.Fatal("NO-ANCHOR did not record that a re-anchoring is owed")
	}
	if s.Anchor != anchor {
		t.Errorf("the failed anchor was discarded before a replacement was chosen: %q", s.Anchor)
	}
	if _, err := s.ReAnchor(""); err == nil {
		t.Error("re-anchoring to nothing was accepted")
	}
	s, err = s.ReAnchor("the delivery side")
	if err != nil {
		t.Fatal(err)
	}
	if s.Anchor != "the delivery side" || s.NeedsAnchor {
		t.Errorf("after re-anchoring: anchor %q, still owed %v", s.Anchor, s.NeedsAnchor)
	}
	if s.Attempts[0].Anchor != anchor {
		t.Errorf("the history lost the anchor that was tried: %q", s.Attempts[0].Anchor)
	}
}

// atTheCeiling drives a repository to its last authoring cycle through real
// re-entries.
//
// Assigning the cycle by hand would assert the arithmetic of the ceiling while
// proving nothing about whether five rejections actually reach it: a reEnter
// that stopped counting would leave every ceiling test green.
func atTheCeiling(t *testing.T) loop.State {
	t.Helper()
	s := loop.Start("mastodon", "the notification pipeline").Drafted("draft-1.yaml")
	for s.Cycle < phase.AuthoringCeiling {
		var err error
		s, _, err = s.Advance(phase.Minibench, phase.Requestion,
			loop.Rejection{Table: "the probe read"}, wrote)
		if err != nil {
			t.Fatalf("cycle %d: %v", s.Cycle, err)
		}
		s = s.Drafted("draft.yaml")
	}
	if s.Parked {
		t.Fatalf("the loop parked before reaching cycle %d", phase.AuthoringCeiling)
	}
	return s
}

func TestTheSixthCycleParksInsteadOfReAuthoring(t *testing.T) {
	s := atTheCeiling(t)
	s, at, err := s.Advance(phase.Validate, phase.DoNotPay, loop.Rejection{Table: "the credit table"}, wrote)
	if err != nil {
		t.Fatal(err)
	}
	if at != phase.Handoff {
		t.Fatalf("the ceiling routed to %s, want %s", at, phase.Handoff)
	}
	if !s.Parked {
		t.Fatal("the ceiling did not park the repository")
	}
	if !strings.Contains(s.Because, "ceiling") {
		t.Errorf("the park does not say why: %q", s.Because)
	}
	if len(s.Attempts) != phase.AuthoringCeiling || s.Attempts[len(s.Attempts)-1].Rejected.Table == "" {
		t.Errorf("filed %d attempts; the one that hit the ceiling is filed like every other", len(s.Attempts))
	}
}

// Parking is where the loop stops. A parked repository that any transition can
// re-enter is a loop that quietly spends six more cycles.
func TestAParkedRepositoryIsNotReEnteredByATransition(t *testing.T) {
	s, _, err := atTheCeiling(t).Advance(phase.Author, phase.NoAnchor, loop.Rejection{Table: "no anchor"}, wrote)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.Advance(phase.Author, phase.Draft, loop.Rejection{}, wrote); err == nil {
		t.Fatal("a parked repository advanced")
	}
	if _, err := loop.Start("chatwoot", "an anchor").Resume(); err == nil {
		t.Fatal("a repository that was never parked was resumed")
	}
}

// The fresh budget a resumed repository gets is a fresh cycle count, and
// nothing else: the six cycles already spent are what the seventh learns from.
func TestResumingHandsAFreshCycleCountAndKeepsTheHistory(t *testing.T) {
	s, _, err := atTheCeiling(t).Advance(phase.Validate, phase.DoNotPay, loop.Rejection{Table: "the credit table"}, wrote)
	if err != nil {
		t.Fatal(err)
	}
	carried, attempts, anchor := len(s.Carried), len(s.Attempts), s.Anchor

	s, err = s.Resume()
	if err != nil {
		t.Fatal(err)
	}
	if s.Parked || s.Because != "" {
		t.Error("the repository is still parked after a deliberate resume")
	}
	if s.Cycle != 1 {
		t.Errorf("resumed on cycle %d, want a fresh count starting at 1", s.Cycle)
	}
	if len(s.Carried) != carried || len(s.Attempts) != attempts || s.Anchor != anchor {
		t.Error("the resume threw away the history it exists to learn from")
	}
}

// A phase that reported success and wrote nothing has not finished, and there
// is one way out of a phase rather than two.
func TestAPhaseThatWroteNothingDoesNotMoveTheLoop(t *testing.T) {
	s := loop.Start("mastodon", "the notification pipeline").Drafted("draft.yaml")
	next, _, err := s.Advance(phase.Minibench, phase.Requestion, loop.Rejection{Table: "the read"},
		func(string) bool { return false })
	if err == nil {
		t.Fatal("the loop advanced past a phase that wrote nothing")
	}
	if next.Cycle != 1 || len(next.Carried) != 0 {
		t.Error("a refused transition still moved the loop")
	}
}

// A diagnosis and a ceiling both land on handoff and mean opposite things: one
// is a swap handed up with its numbers, the other is a repository parked.
func TestABenchedDiagnosisIsHandedUpWithoutParkingTheRepository(t *testing.T) {
	s := loop.Start("mastodon", "the notification pipeline").Drafted("draft.yaml")
	s, at, err := s.Advance(phase.Report, phase.Diagnosis, loop.Rejection{}, wrote)
	if err != nil {
		t.Fatal(err)
	}
	if at != phase.Handoff {
		t.Fatalf("a diagnosis went to %s, want %s", at, phase.Handoff)
	}
	if s.Parked {
		t.Error("a diagnosis parked the repository; only the authoring ceiling parks")
	}
}

func TestManualStopsAtItsPhaseAndNamesTheArtifactItAwaits(t *testing.T) {
	m, err := loop.NewManual("expand")
	if err != nil {
		t.Fatal(err)
	}
	msg, halted := m.Halt(phase.Expand)
	if !halted {
		t.Fatal("--manual expand did not stop at the expand phase")
	}
	p, _ := phase.Lookup(phase.Expand)
	if !strings.Contains(msg, p.Reads) || !strings.Contains(msg, p.Writes) {
		t.Errorf("the stop message does not name the artifacts: %q", msg)
	}
	if _, halted := m.Halt(phase.Bench); halted {
		t.Error("--manual expand stopped at the bench phase")
	}
}

func TestAnUnsetManualNeverStops(t *testing.T) {
	var m loop.Manual
	for _, p := range phase.Graph {
		if _, halted := m.Halt(p.Name); halted {
			t.Errorf("a loop with no --manual stopped at %s", p.Name)
		}
	}
}

// A typo stops the loop at the flag rather than never stopping it at all,
// which is the failure mode of a stop-at that silently matches nothing.
func TestManualRefusesAPhaseTheGraphDoesNotDeclare(t *testing.T) {
	if _, err := loop.NewManual("minibensh"); err == nil {
		t.Fatal("--manual accepted a phase that does not exist")
	}
}

func TestARejectionReadsAsThePhaseTheVerdictAndTheTable(t *testing.T) {
	r := loop.Rejection{Phase: phase.Validate, Verdict: phase.DoNotPay, Table: "4 free rows"}
	got := r.String()
	for _, want := range []string{"validate", "DO-NOT-PAY", "4 free rows"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q does not name %q", got, want)
		}
	}
}
