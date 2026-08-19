package position

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/phase"
)

// A recorded verdict is history. Append-only as a convention survives until a
// bad night; append-only as an open flag survives the bad night, because the
// second write fails rather than replacing a judgment nobody can recover.
func TestARecordedAttemptIsNotWrittenOver(t *testing.T) {
	dir := t.TempDir()
	first := Attempt{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1, Anchor: "BaseItem"}
	if err := Record(dir, first); err != nil {
		t.Fatal(err)
	}

	err := Record(dir, Attempt{Cycle: 1, Phase: phase.Author, Verdict: phase.NoAnchor, Try: 1})

	if err == nil {
		t.Fatal("a second write to one attempt succeeded; a verdict was replaced")
	}
	back, readErr := Attempts(dir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(back) != 1 || back[0].Verdict != phase.Draft {
		t.Errorf("attempts = %+v, want the first verdict intact", back)
	}
}

// The same phase can legitimately run twice in one cycle — a DoD FAIL sends a
// harvest back to report — so a second attempt is a second try rather than a
// collision, and both are kept.
func TestASecondAttemptAtOnePhaseIsANewTry(t *testing.T) {
	dir := t.TempDir()
	if err := Record(dir, Attempt{Cycle: 1, Phase: phase.Report, Verdict: phase.Win, Try: 1}); err != nil {
		t.Fatal(err)
	}

	next := NextTry(mustRead(t, dir), 1, phase.Report)
	if next != 2 {
		t.Fatalf("NextTry = %d, want 2", next)
	}
	if err := Record(dir, Attempt{Cycle: 1, Phase: phase.Report, Verdict: phase.Diagnosis, Try: next}); err != nil {
		t.Fatal(err)
	}

	all := mustRead(t, dir)
	if len(all) != 2 || all[0].Try != 1 || all[1].Try != 2 {
		t.Errorf("attempts = %+v, want both tries in order", all)
	}
}

// NextTry counts only what it is asked about. A phase in another cycle is
// another attempt at another question.
func TestNextTryIsPerCycleAndPerPhase(t *testing.T) {
	recorded := []Attempt{
		{Cycle: 1, Phase: phase.Report, Try: 1},
		{Cycle: 1, Phase: phase.Report, Try: 2},
		{Cycle: 2, Phase: phase.Report, Try: 1},
		{Cycle: 1, Phase: phase.Author, Try: 1},
	}

	for _, tc := range []struct {
		cycle int
		phase phase.Name
		want  int
	}{
		{1, phase.Report, 3},
		{2, phase.Report, 2},
		{1, phase.Author, 2},
		{3, phase.Author, 1},
	} {
		if got := NextTry(recorded, tc.cycle, tc.phase); got != tc.want {
			t.Errorf("NextTry(cycle %d, %s) = %d, want %d", tc.cycle, tc.phase, got, tc.want)
		}
	}
}

// The order attempts are read back in is derived from the graph, not from a
// clock, so it is the same order on a machine whose clock is wrong.
func TestAttemptsComeBackInTheOrderTheyCanOnlyHaveHappenedIn(t *testing.T) {
	dir := t.TempDir()
	// A later cycle recorded first, which is what a hand-repaired tree looks
	// like. Within a cycle the order is the order they were recorded in; across
	// cycles it is the cycle.
	for _, a := range []Attempt{
		{Cycle: 2, Phase: phase.Author, Verdict: phase.Draft, Try: 1},
		{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1},
		{Cycle: 1, Phase: phase.Minibench, Verdict: phase.Requestion, Try: 1, Table: "the baseline reached it"},
	} {
		if err := Record(dir, a); err != nil {
			t.Fatal(err)
		}
	}

	var got []string
	for _, a := range mustRead(t, dir) {
		got = append(got, string(a.Phase))
	}

	if strings.Join(got, ",") != "author,minibench,author" {
		t.Errorf("order = %v, want cycle 1's author, then its mini-bench, then cycle 2's author", got)
	}
	if all := mustRead(t, dir); all[2].Cycle != 2 {
		t.Errorf("last is in cycle %d, want the later cycle last however it was recorded", all[2].Cycle)
	}
}

// A record written by hand carries no step, and those fall back to the graph's
// order — which is right for a forward walk and is the best guess available. A
// phase the graph does not know sorts last rather than first, so an unreadable
// record cannot displace the phase that actually ran.
func TestHandWrittenRecordsFallBackToTheGraphsOrder(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"1-invented-1.json": `{"cycle":1,"phase":"invented","verdict":"AUTO","try":1}`,
		"1-author-1.json":   `{"cycle":1,"phase":"author","verdict":"DRAFT","try":1}`,
	} {
		if err := os.MkdirAll(filepath.Join(dir, attemptsDir), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, attemptsDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	all := mustRead(t, dir)

	if all[0].Phase != phase.Author {
		t.Errorf("first = %s, want the phase the graph knows", all[0].Phase)
	}
}

// A record that could not be read back as a judgment is refused when it is
// written, where the caller can still fix it.
func TestAnAttemptThatIsNotAJudgmentIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name string
		a    Attempt
	}{
		{"no cycle", Attempt{Phase: phase.Author, Verdict: phase.Draft, Try: 1}},
		{"no try", Attempt{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft}},
		{"no phase", Attempt{Cycle: 1, Verdict: phase.Draft, Try: 1}},
		{"no verdict", Attempt{Cycle: 1, Phase: phase.Author, Try: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := Record(t.TempDir(), tc.a); err == nil {
				t.Error("recorded")
			}
		})
	}
}

// A repository nothing has decided about yet has no attempts, which is a
// position rather than a failure.
func TestARepositoryWithNoAttemptsReadsAsNone(t *testing.T) {
	all, err := Attempts(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("attempts = %v, want none", all)
	}
}

// A record that is there and cannot be read back is reported rather than
// skipped. A judgment silently dropped is a repository that reads as never
// having reached the phase that produced it.
func TestARecordThatCannotBeReadIsReportedRatherThanSkipped(t *testing.T) {
	for _, tc := range []struct {
		name string
		put  func(t *testing.T, path string)
	}{
		{"it is not a document", func(t *testing.T, path string) {
			if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{"it cannot be opened", func(t *testing.T, path string) {
			if os.Geteuid() == 0 {
				t.Skip("root reads anything")
			}
			if err := os.Chmod(path, 0o000); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			a := Attempt{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1}
			if err := Record(dir, a); err != nil {
				t.Fatal(err)
			}
			tc.put(t, filepath.Join(dir, attemptsDir, a.Name()))

			if _, err := Attempts(dir); err == nil {
				t.Error("an unreadable record was skipped")
			}
		})
	}
}

// Anything that is not a record is not read as one.
func TestOnlyRecordsAreReadOutOfTheAttemptsDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := Record(dir, Attempt{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, attemptsDir, "notes.md"), []byte("# by hand\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, attemptsDir, "olds"), 0o755); err != nil {
		t.Fatal(err)
	}

	if all := mustRead(t, dir); len(all) != 1 {
		t.Errorf("attempts = %+v, want only the record", all)
	}
}

func TestARecordThatCannotBeWrittenIsReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, attemptsDir), []byte("in the way"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Record(dir, Attempt{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1}); err == nil {
		t.Fatal("an attempt was recorded into a file")
	}
}

func mustRead(t *testing.T, dir string) []Attempt {
	t.Helper()
	all, err := Attempts(dir)
	if err != nil {
		t.Fatal(err)
	}
	return all
}

// The order is the one they were recorded in, not the filesystem's. Read back
// alphabetically an expansion comes before the mini-bench that decides whether
// to expand at all, and the last attempt — which is what a position is read
// from — would be the wrong one.
func TestTheOrderIsTheOneTheyHappenedInRatherThanTheAlphabets(t *testing.T) {
	dir := t.TempDir()
	for _, a := range []Attempt{
		{Cycle: 1, Phase: phase.Minibench, Verdict: phase.Proceed, Try: 1},
		{Cycle: 1, Phase: phase.Expand, Verdict: phase.Auto, Try: 1},
	} {
		if err := Record(dir, a); err != nil {
			t.Fatal(err)
		}
	}

	all := mustRead(t, dir)

	if all[0].Phase != phase.Minibench || all[1].Phase != phase.Expand {
		t.Errorf("order = %s then %s, want the mini-bench before the expansion it decides on",
			all[0].Phase, all[1].Phase)
	}
}

// And it is not the graph's either, because a cycle that re-enters a phase did
// not walk the graph in order.
//
// This is the failure that made the loop spin: ordered by the graph, the
// mini-bench that sent work back sorts after the author attempt that answered
// it, so the position routes from the rejection forever and re-runs the author
// on every turn, at whatever an agent costs.
func TestAReEnteredPhaseIsTheLastAttemptRatherThanTheOneThatSentItBack(t *testing.T) {
	dir := t.TempDir()
	for _, a := range []Attempt{
		{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1},
		{Cycle: 1, Phase: phase.Minibench, Verdict: phase.Requestion, Try: 1, Table: "the baseline reached it"},
		{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 2},
	} {
		if err := Record(dir, a); err != nil {
			t.Fatal(err)
		}
	}

	all := mustRead(t, dir)

	last := all[len(all)-1]
	if last.Phase != phase.Author || last.Try != 2 {
		t.Errorf("last = %s try %d, want the re-entered author", last.Phase, last.Try)
	}
}

// The step is what makes that possible, and it is assigned when the attempt is
// recorded rather than held by a caller.
func TestEachAttemptIsRecordedWithItsPlaceInTheCycle(t *testing.T) {
	dir := t.TempDir()
	for _, a := range []Attempt{
		{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1},
		{Cycle: 1, Phase: phase.Minibench, Verdict: phase.Requestion, Try: 1, Table: "no"},
		{Cycle: 2, Phase: phase.Author, Verdict: phase.Draft, Try: 1},
	} {
		if err := Record(dir, a); err != nil {
			t.Fatal(err)
		}
	}

	all := mustRead(t, dir)

	// Steps count within a cycle, so a new cycle starts again at one.
	for i, want := range []int{1, 2, 1} {
		if all[i].Step != want {
			t.Errorf("%s in cycle %d is step %d, want %d", all[i].Phase, all[i].Cycle, all[i].Step, want)
		}
	}
}

// The anchor is one of the three things a record exists to keep — the verdict,
// the table and the anchor — so it survives the round trip.
func TestTheAnchorSurvivesTheRecord(t *testing.T) {
	dir := t.TempDir()
	if err := Record(dir, Attempt{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1,
		Anchor: "MediaBrowser.Controller.Entities.BaseItem"}); err != nil {
		t.Fatal(err)
	}

	if back := mustRead(t, dir); back[0].Anchor != "MediaBrowser.Controller.Entities.BaseItem" {
		t.Errorf("anchor = %q, want the one that was recorded", back[0].Anchor)
	}
}
