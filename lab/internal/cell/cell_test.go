package cell_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/cell"
	"github.com/luuuc/sense/lab/internal/run"
)

// arms is a cell of two sessions, each running the script it is given.
func arms(senseScript, baselineScript string) []cell.Arm {
	spec := func(name, script string) run.Spec {
		return run.Spec{
			Name: "/bin/sh", Args: []string{"-c", script}, Arm: name,
			Wall: 30 * time.Second, Grace: 100 * time.Millisecond,
		}
	}
	return []cell.Arm{
		{Name: "sense", Spec: spec("sense", senseScript)},
		{Name: "baseline", Spec: spec("baseline", baselineScript)},
	}
}

// bothFinish is the ordinary cell: two sessions that end on their own.
func bothFinish() []cell.Arm { return arms("echo sense answer", "echo baseline answer") }

// cutAfterTheFirstArm is the failure this package exists to prevent: the sense
// arm finishes and is paid for, and the interruption lands while the baseline
// arm is still running.
func cutAfterTheFirstArm() []cell.Arm { return arms("echo sense answer", "sleep 30") }

func TestBothArmsRunUnderOneSupervisorAndTheCellIsComplete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cell")

	rec, err := cell.Run(context.Background(), dir, bothFinish())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !rec.Complete {
		t.Fatalf("cell is incomplete: %+v", rec)
	}
	if len(rec.Burned) != 0 {
		t.Errorf("a complete cell burned %v", rec.Burned)
	}
	for _, arm := range []string{"sense", "baseline"} {
		if _, err := run.Read(rec.Arms[arm]); err != nil {
			t.Errorf("arm %s: %v", arm, err)
		}
	}
}

func TestAnInterruptionBetweenTheArmsNamesTheBurnedRun(t *testing.T) {
	// The failure this package exists to prevent. The finished arm can never be
	// paired, because the baseline's budget derives from its paired sense wall,
	// and money was spent on it. Naming it is what lets a later pass refuse it
	// rather than quietly pair it with something else.
	dir := filepath.Join(t.TempDir(), "cell")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	rec, err := cell.Run(ctx, dir, cutAfterTheFirstArm())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rec.Complete {
		t.Fatal("an interrupted cell reported itself complete")
	}
	if !slices.Contains(rec.Unusable, "baseline") {
		t.Errorf("Unusable = %v, want the baseline arm named as having no result", rec.Unusable)
	}
	if !slices.Contains(rec.Burned, "sense") {
		t.Errorf("Burned = %v, want the sense arm named", rec.Burned)
	}
}

func TestAnInterruptedCellRefusesToBePaired(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cell")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()
	if _, err := cell.Run(ctx, dir, cutAfterTheFirstArm()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	_, err := cell.Pair(dir)

	if err == nil {
		t.Fatal("Pair accepted an incomplete cell; the burned arm would be measured against a run it was never a cell with")
	}
	if !strings.Contains(err.Error(), "sense") {
		t.Errorf("Pair error = %v, want it to name the burned arm", err)
	}
}

func TestACompleteCellPairsItsArms(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cell")
	if _, err := cell.Run(context.Background(), dir, bothFinish()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	metas, err := cell.Pair(dir)
	if err != nil {
		t.Fatalf("Pair: %v", err)
	}

	if len(metas) != 2 {
		t.Fatalf("Pair returned %d arms, want 2", len(metas))
	}
	if metas["sense"].Arm != "sense" || metas["baseline"].Arm != "baseline" {
		t.Errorf("Pair returned %+v", metas)
	}
}

func TestAnAlreadyCancelledCellStartsNothing(t *testing.T) {
	// An interrupt that arrives while the previous arm is being recorded must
	// not start another session that is certain to be killed: that is a second
	// arm's worth of spend for a cell that can never be paired.
	dir := filepath.Join(t.TempDir(), "cell")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	rec, err := cell.Run(ctx, dir, bothFinish())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if rec.Complete {
		t.Fatal("a cancelled cell reported itself complete")
	}
	if len(rec.Burned) != 0 {
		t.Errorf("Burned = %v; nothing had run, so nothing was burned", rec.Burned)
	}
	if !slices.Contains(rec.Unusable, "sense") || !slices.Contains(rec.Unusable, "baseline") {
		t.Errorf("Unusable = %v, want both arms", rec.Unusable)
	}
	if _, err := os.Stat(filepath.Join(dir, "sense")); !os.IsNotExist(err) {
		t.Errorf("a session directory was created for an arm that never ran: %v", err)
	}
}

func TestACellWithNoRecordIsInvalidOnDiscovery(t *testing.T) {
	// A reboot leaves a cell directory with no record. Resuming it would pair
	// runs whose conditions nobody can reconstruct.
	dir := t.TempDir()

	if _, err := cell.Pair(dir); err == nil {
		t.Fatal("Pair accepted a cell directory with no record")
	}
}

func TestAnUnreadableCellRecordIsReported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "cell-meta.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := cell.ReadRecord(dir); err == nil {
		t.Fatal("ReadRecord accepted an unreadable record")
	}
}

func TestACompleteCellWhoseArmLostItsRecordIsRefused(t *testing.T) {
	// The cell says complete and the arm on disk does not agree. Trusting the
	// cell record alone would pair a run that never reached a terminal state.
	dir := filepath.Join(t.TempDir(), "cell")
	if _, err := cell.Run(context.Background(), dir, bothFinish()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, "sense", "run-meta.json")); err != nil {
		t.Fatal(err)
	}

	if _, err := cell.Pair(dir); err == nil {
		t.Fatal("Pair accepted a cell whose arm has no terminal state")
	}
}

func TestACellWithNoArmsIsRefused(t *testing.T) {
	if _, err := cell.Run(context.Background(), t.TempDir(), nil); err == nil {
		t.Fatal("Run accepted a cell with no arms")
	}
}

func TestACellDirectoryThatCannotBeCreatedIsReported(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := cell.Run(context.Background(), filepath.Join(blocked, "cell"), bothFinish()); err == nil {
		t.Fatal("Run succeeded with an unusable cell directory")
	}
}

func TestAnArmThatCannotBeRecordedStopsTheCell(t *testing.T) {
	// A cell directory that already holds a recorded run for one arm. Runs are
	// immutable, so this must fail loudly rather than write over paid-for work.
	dir := filepath.Join(t.TempDir(), "cell")
	if _, err := cell.Run(context.Background(), dir, bothFinish()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := cell.Run(context.Background(), dir, bothFinish()); err == nil {
		t.Fatal("Run wrote over a cell that already holds recorded runs")
	}
}

func TestACellRecordThatCannotBeWrittenIsReported(t *testing.T) {
	// Without the record there is nothing on disk saying which arm was burned,
	// and a later pass would pair it with whatever ran next.
	dir := filepath.Join(t.TempDir(), "cell")
	// A directory where the record file belongs: the arms run and finish, and
	// only the record fails.
	if err := os.MkdirAll(filepath.Join(dir, "cell-meta.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := cell.Run(context.Background(), dir, bothFinish())

	if err == nil {
		t.Fatal("Run reported success although the cell record could not be written")
	}
}

func TestACellRecordThatCannotBeReadIsReported(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission that makes the read fail")
	}
	dir := t.TempDir()
	record := filepath.Join(dir, "cell-meta.json")
	if err := os.WriteFile(record, []byte(`{"complete":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(record, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(record, 0o600) })

	if _, err := cell.ReadRecord(dir); err == nil {
		t.Fatal("ReadRecord reported success on a record it could not read")
	}
}
