package run_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/run"
)

// out is a fresh run directory. Session refuses to write over a recorded run,
// so every session in a test needs its own.
func out(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "run")
}

func TestASleeperIsKilledAtItsWallAndTheRunSaysSo(t *testing.T) {
	dir := out(t)

	m, err := run.Session(context.Background(), dir, run.Spec{
		Name:  "/bin/sh",
		Args:  []string{"-c", `echo the answer so far; sleep 30`},
		Wall:  300 * time.Millisecond,
		Grace: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	// A run that exceeds its budget is a result, not an error. The wall is
	// never raised to rescue it.
	if m.Outcome != run.CannotFinishAtBudget {
		t.Errorf("outcome = %s, want %s", m.Outcome, run.CannotFinishAtBudget)
	}
	// The partial answer is evidence about where the arm spent its budget. One
	// recorded run spent 269 of 480 seconds writing a final message that was cut
	// mid-sentence, and that is only visible because the output was kept.
	if said := readFile(t, filepath.Join(dir, "raw", "stdout")); !strings.Contains(said, "the answer so far") {
		t.Errorf("stdout = %q, want the output captured up to the kill", said)
	}
	if m.StdoutBytes == 0 {
		t.Error("run-meta reports no output although the session spoke before its wall")
	}
}

func TestTheWallStartPointIsRecordedAndIsTheSameForBothArms(t *testing.T) {
	// The sense arm pays for MCP server startup. A wall counted from first
	// streamed event would charge that to one arm only, and the arms would have
	// different effective budgets while appearing to share one.
	metas := map[string]run.Meta{}
	for _, arm := range []string{"sense", "baseline"} {
		m, err := run.Session(context.Background(), out(t), run.Spec{
			Name: "/bin/sh", Args: []string{"-c", "true"}, Arm: arm, Wall: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("Session(%s): %v", arm, err)
		}
		metas[arm] = m
	}

	if metas["sense"].WallStartsAt == "" {
		t.Fatal("run-meta does not record where the wall is counted from")
	}
	if metas["sense"].WallStartsAt != metas["baseline"].WallStartsAt {
		t.Errorf("the arms count their walls from %q and %q", metas["sense"].WallStartsAt, metas["baseline"].WallStartsAt)
	}
}

func TestAChildIsAskedToStopBeforeItIsKilled(t *testing.T) {
	// Ten seconds of graceful cleanup is normal behaviour for an agent CLI. A
	// supervisor that sends only SIGKILL never lets the flush start, and the
	// output it was about to write is lost.
	dir := out(t)

	// Catches the signal, says so, and leaves on its own. If the supervisor
	// went straight to SIGKILL, nothing would be said.
	script := `trap 'echo flushed on request; exit 0' TERM
	while :; do sleep 0.05; done`
	m, err := run.Session(context.Background(), dir, run.Spec{
		Name:  "/bin/sh",
		Args:  []string{"-c", script},
		Wall:  300 * time.Millisecond,
		Grace: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	if m.Outcome != run.CannotFinishAtBudget {
		t.Errorf("outcome = %s, want %s", m.Outcome, run.CannotFinishAtBudget)
	}
	if said := readFile(t, filepath.Join(dir, "raw", "stdout")); !strings.Contains(said, "flushed on request") {
		t.Errorf("stdout = %q; the session was never asked to stop, it was killed", said)
	}
}

func TestAChildThatIgnoresTheRequestIsKilledAnyway(t *testing.T) {
	// The other half of the escalation. A supervisor that asks and then waits
	// forever has turned the wall into a suggestion.
	dir := out(t)
	pidFile := filepath.Join(t.TempDir(), "pid")

	script := `trap '' TERM
	echo $$ > ` + pidFile + `
	while :; do sleep 0.05; done`
	m, err := run.Session(context.Background(), dir, run.Spec{
		Name:  "/bin/sh",
		Args:  []string{"-c", script},
		Wall:  300 * time.Millisecond,
		Grace: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	if m.Outcome != run.CannotFinishAtBudget {
		t.Errorf("outcome = %s, want %s", m.Outcome, run.CannotFinishAtBudget)
	}
	if m.TookSeconds > 5 {
		t.Errorf("the session took %.1fs; an ignored request must not hold the wall open", m.TookSeconds)
	}
	assertGone(t, pidFile)
}

func TestKillingTheSessionLeavesNoOrphanBehind(t *testing.T) {
	// Checked by looking for the orphan, not by watching a terminal. An agent
	// CLI spawns children of its own, and killing only the direct child leaves
	// them alive holding the capture files and still spending.
	dir := out(t)
	pidFile := filepath.Join(t.TempDir(), "grandchild-pid")

	// A child that spawns its own child and then exits, so the survivor is not
	// the process the supervisor is waiting on.
	script := `sh -c 'echo $$ > ` + pidFile + `; sleep 30' &
	sleep 30`
	if _, err := run.Session(context.Background(), dir, run.Spec{
		Name:  "/bin/sh",
		Args:  []string{"-c", script},
		Wall:  400 * time.Millisecond,
		Grace: 200 * time.Millisecond,
	}); err != nil {
		t.Fatalf("Session: %v", err)
	}

	assertGone(t, pidFile)
}

func TestAnInterruptedSessionIsNotABudgetFailure(t *testing.T) {
	// An operator pressing Ctrl-C says nothing about whether the session could
	// have finished, and a row that conflates the two reads as a stalled arm.
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	m, err := run.Session(ctx, out(t), run.Spec{
		Name:  "/bin/sh",
		Args:  []string{"-c", "sleep 30"},
		Wall:  30 * time.Second,
		Grace: 100 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	if m.Outcome != run.Interrupted {
		t.Errorf("outcome = %s, want %s", m.Outcome, run.Interrupted)
	}
}

func TestARunWithNoTerminalStateIsInvalidOnDiscovery(t *testing.T) {
	// Signals cover an interrupt. Nothing covers a reboot or a power loss, so
	// the gap is closed by a rule: such a directory is never resumed, because
	// resuming it would silently pair a run whose conditions nobody can
	// reconstruct.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "raw", "stdout"), []byte("half an answer"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := run.Read(dir)

	if err == nil {
		t.Fatal("Read accepted a run directory with no terminal state")
	}
	if !strings.Contains(err.Error(), "no terminal state") {
		t.Errorf("Read error = %v, want it to name the missing terminal state", err)
	}
}

func TestARecordWithoutAnOutcomeIsAlsoInvalid(t *testing.T) {
	// A metadata file that exists but records nothing terminal is the same
	// failure wearing a more convincing costume.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "run-meta.json"), []byte(`{"exit_code":0}`), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := run.Read(dir); err == nil {
		t.Fatal("Read accepted metadata with no outcome")
	}
}

func TestUnreadableRecordsAreReportedRatherThanTreatedAsAbsent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "run-meta.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := run.Read(dir); err == nil {
		t.Fatal("Read accepted unreadable metadata")
	}
}

func TestARecordThatCannotBeReadIsNotTreatedAsAbsent(t *testing.T) {
	// "I could not read it" and "there is nothing there" lead to opposite
	// actions: one is a broken machine, the other is a run that never finished.
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "run-meta.json"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := run.Read(dir)

	if err == nil {
		t.Fatal("Read reported success on metadata it could not read")
	}
	if strings.Contains(err.Error(), "no terminal state") {
		t.Errorf("Read error = %v, which reports an unreadable record as a run that never finished", err)
	}
}

func TestARecordedRunReadsBackAsItWasWritten(t *testing.T) {
	dir := out(t)
	wrote, err := run.Session(context.Background(), dir, run.Spec{
		Name: "/bin/sh", Args: []string{"-c", "echo hello"}, Arm: "sense", Wall: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	read, err := run.Read(dir)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if read.Outcome != wrote.Outcome || read.Arm != wrote.Arm || read.StdoutBytes != wrote.StdoutBytes {
		t.Errorf("read back %+v, want %+v", read, wrote)
	}
}

// assertGone waits briefly for the recorded process to disappear and fails if
// it is still alive. Briefly, because the kill and this check race by design.
func assertGone(t *testing.T, pidFile string) {
	t.Helper()
	pid := readPID(t, pidFile)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(pid, 0) != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = syscall.Kill(pid, syscall.SIGKILL)
	t.Fatalf("process %d survived the session that spawned it", pid)
}

func readPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(b))) > 0 {
			pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
			if err != nil {
				t.Fatalf("pid file holds %q", b)
			}
			return pid
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("the session never recorded a pid at %s", path)
	return 0
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// The supervision rules the old shell driver worked around are meant to be
// gone, not merely unused.

func TestNoShellDetachmentTrickSurvivesInTheLabTree(t *testing.T) {
	// A Go binary supervising its own children is most of the argument for the
	// rewrite at this layer. screen, nohup, disown and setsid are the symptoms
	// of not owning them, and one of them already cost a 312-second run to a
	// launcher that looked detached and was not.
	//
	// The tree is walked rather than grepped through git, so a file that is not
	// yet committed is covered too. Test files are skipped: this one names the
	// tricks in order to forbid them.
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	tricks := []string{"screen -dmS", "nohup", "disown", "setsid"}

	err = filepath.WalkDir(filepath.Join(root, "lab"), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, trick := range tricks {
			if strings.Contains(string(b), trick) {
				t.Errorf("%q appears in %s; the binary owns its children", trick, path)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk the lab tree: %v", err)
	}
}

func TestTheRecordedMetadataIsReadableJSON(t *testing.T) {
	dir := out(t)
	if _, err := run.Session(context.Background(), dir, run.Spec{
		Name: "/bin/sh", Args: []string{"-c", "true"}, Wall: 10 * time.Second,
	}); err != nil {
		t.Fatalf("Session: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(dir, "run-meta.json"))), &raw); err != nil {
		t.Fatalf("run-meta is not readable JSON: %v", err)
	}
	if _, ok := raw["wall_starts_at"]; !ok {
		t.Error("run-meta does not carry wall_starts_at")
	}
}
