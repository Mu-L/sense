package budget_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/budget"
)

// paidRun writes a finished run directory: a capture directory and a terminal
// record, which is what run.Session leaves behind.
func paidRun(t *testing.T, dir string) {
	t.Helper()
	startedRun(t, dir)
	if err := os.WriteFile(filepath.Join(dir, "run-meta.json"), []byte(`{"outcome":"completed"}`), 0o644); err != nil {
		t.Fatal(err)
	}
}

// startedRun writes a run directory that spawned and never recorded a terminal
// state, which is what a reboot or a power loss leaves behind.
func startedRun(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestSpendIsEveryBenchRunInTheTree(t *testing.T) {
	camp := t.TempDir()
	paidRun(t, filepath.Join(camp, "mastodon", "1", "bench", "cell-0", "sense"))
	paidRun(t, filepath.Join(camp, "mastodon", "1", "bench", "cell-0", "untreated"))
	paidRun(t, filepath.Join(camp, "chatwoot", "3", "bench", "cell-0", "sense"))

	s, err := budget.Read(camp)
	if err != nil {
		t.Fatal(err)
	}
	if s.Runs() != 3 {
		t.Errorf("counted %d paid runs, want 3: %+v", s.Runs(), s)
	}
}

// A mini-bench and a validation are unscored and unpaid by law. Counting them
// would refuse a campaign for the runs it does in order to avoid spending.
func TestUnpaidPhasesAreNotSpend(t *testing.T) {
	camp := t.TempDir()
	paidRun(t, filepath.Join(camp, "mastodon", "1", "minibench", "cell-0", "sense"))
	paidRun(t, filepath.Join(camp, "mastodon", "1", "validate", "cell-0", "sense"))
	paidRun(t, filepath.Join(camp, "mastodon", "1", "bench", "cell-0", "sense"))

	s, err := budget.Read(camp)
	if err != nil {
		t.Fatal(err)
	}
	if s.Runs() != 1 {
		t.Errorf("counted %d paid runs, want 1: %+v", s.Runs(), s)
	}
}

// A bench run that started and never wrote a terminal record still spent its
// money. Dropping it undercounts the ceiling by exactly the runs nobody can
// account for.
func TestABenchRunWithNoTerminalRecordStillCountsAndIsNamed(t *testing.T) {
	camp := t.TempDir()
	paidRun(t, filepath.Join(camp, "mastodon", "1", "bench", "cell-0", "sense"))
	startedRun(t, filepath.Join(camp, "mastodon", "1", "bench", "cell-0", "untreated"))

	s, err := budget.Read(camp)
	if err != nil {
		t.Fatal(err)
	}
	if s.Runs() != 2 {
		t.Errorf("counted %d paid runs, want 2", s.Runs())
	}
	if len(s.Orphaned) != 1 || !strings.HasSuffix(s.Orphaned[0], "untreated") {
		t.Errorf("the orphaned run is not named: %+v", s.Orphaned)
	}
	if !strings.Contains(s.String(), "orphaned") {
		t.Errorf("the summary hides the orphan: %q", s.String())
	}
}

// The cell and phase directories above a run are not runs. Counting a parent
// would multiply every run by its depth in the tree.
func TestOnlyRunDirectoriesAreCounted(t *testing.T) {
	camp := t.TempDir()
	paidRun(t, filepath.Join(camp, "mastodon", "1", "bench", "cell-0", "sense"))

	s, err := budget.Read(camp)
	if err != nil {
		t.Fatal(err)
	}
	if s.Runs() != 1 {
		t.Errorf("counted %d, want 1: %+v", s.Runs(), s)
	}
}

// The whole reason spend is read rather than held: a process that restarts must
// see the same total, and reading twice must not add to it.
func TestSpendIsTheSameAfterARestartAndIsNeverDoubleCounted(t *testing.T) {
	camp := t.TempDir()
	for _, arm := range []string{"sense", "untreated"} {
		paidRun(t, filepath.Join(camp, "mastodon", "1", "bench", "cell-0", arm))
	}

	before, err := budget.Read(camp)
	if err != nil {
		t.Fatal(err)
	}
	// A restart is a second Read with nothing in between, which is exactly what
	// a resumed campaign does.
	after, err := budget.Read(camp)
	if err != nil {
		t.Fatal(err)
	}
	if before.Runs() != after.Runs() {
		t.Fatalf("spend moved from %d to %d without a run happening", before.Runs(), after.Runs())
	}

	paidRun(t, filepath.Join(camp, "mastodon", "2", "bench", "cell-0", "sense"))
	next, err := budget.Read(camp)
	if err != nil {
		t.Fatal(err)
	}
	if next.Runs() != before.Runs()+1 {
		t.Errorf("one run took the total from %d to %d", before.Runs(), next.Runs())
	}
}

// The ceiling is checked before the first run as well as after the hundredth,
// so a campaign with nothing on disk is a spend of zero rather than a failure.
func TestACampaignThatHasRunNothingHasSpentNothing(t *testing.T) {
	s, err := budget.Read(filepath.Join(t.TempDir(), "never-started"))
	if err != nil {
		t.Fatalf("an unstarted campaign was an error: %v", err)
	}
	if s.Runs() != 0 {
		t.Errorf("an unstarted campaign has spent %d", s.Runs())
	}
	if s.String() != "0 paid runs" {
		t.Errorf("summary reads %q", s.String())
	}
}

// It refuses rather than warns: a ceiling that warns is passed at the moment
// someone believes the next run will work.
func TestTheCeilingRefusesAtItsLimitRatherThanAboveIt(t *testing.T) {
	camp := t.TempDir()
	for _, arm := range []string{"sense", "untreated"} {
		paidRun(t, filepath.Join(camp, "mastodon", "1", "bench", "cell-0", arm))
	}
	s, err := budget.Read(camp)
	if err != nil {
		t.Fatal(err)
	}
	if err := budget.Ceiling(s, 3); err != nil {
		t.Errorf("2 runs against a ceiling of 3 was refused: %v", err)
	}
	err = budget.Ceiling(s, 2)
	if err == nil {
		t.Fatal("2 runs against a ceiling of 2 was allowed; the next run would pass it")
	}
	if !strings.Contains(err.Error(), "lifetime") {
		t.Errorf("the refusal does not name the window it applies over: %v", err)
	}
}

func TestAnUnreadableCampaignTreeIsAnErrorRatherThanAZero(t *testing.T) {
	camp := t.TempDir()
	locked := filepath.Join(camp, "bench")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	if _, err := budget.Read(camp); err == nil {
		t.Fatal("a campaign tree that could not be read reported a spend of zero")
	}
}
