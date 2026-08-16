package run

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Every test here spawns a real process, and none of them touch a model or the
// network. The wall and the supervision are the assumptions most likely to be
// wrong, and they are the ones a fake would not test at all: the failure mode
// that has already cost a run is a child that keeps running after the parent is
// signalled, which only a real process can demonstrate.

func readMeta(t *testing.T, dir string) Meta {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "run-meta.json"))
	if err != nil {
		t.Fatalf("run-meta.json: %v", err)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode run-meta.json: %v", err)
	}
	return m
}

func readRaw(t *testing.T, dir, stream string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "raw", stream))
	if err != nil {
		t.Fatalf("raw/%s: %v", stream, err)
	}
	return string(b)
}

func TestAProcessThatSucceedsIsRecordedAsCompleted(t *testing.T) {
	dir := t.TempDir()

	m, err := Session(context.Background(), dir, Spec{
		Name: "sh", Args: []string{"-c", "echo hello; echo trouble >&2"},
		Wall: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	if m.Outcome != Completed {
		t.Errorf("outcome = %q, want %q", m.Outcome, Completed)
	}
	if m.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", m.ExitCode)
	}
	if got := readRaw(t, dir, "stdout"); !strings.Contains(got, "hello") {
		t.Errorf("raw/stdout = %q, want the process's stdout", got)
	}
	// stderr is captured separately: an agent CLI's diagnostics must not be
	// interleaved into the stream the scorer reads.
	if got := readRaw(t, dir, "stderr"); !strings.Contains(got, "trouble") {
		t.Errorf("raw/stderr = %q, want the process's stderr", got)
	}

	// The record has to say what was run and when, or two runs of different
	// things are indistinguishable on disk afterwards.
	rec := readMeta(t, dir)
	if rec.Command != "sh" {
		t.Errorf("command = %q, want the command that was run", rec.Command)
	}
	if len(rec.Args) == 0 || rec.Args[0] != "-c" {
		t.Errorf("args = %v, want the arguments the process was given", rec.Args)
	}
	if _, err := time.Parse(time.RFC3339, rec.StartedAt); err != nil {
		t.Errorf("started_at = %q, not a timestamp: %v", rec.StartedAt, err)
	}
}

// A recorded run is paid for and cannot be recreated. Pointing a second run at
// the same directory must fail rather than quietly destroy the first.
func TestARecordedRunIsNeverOverwritten(t *testing.T) {
	dir := t.TempDir()
	if _, err := Session(context.Background(), dir, Spec{
		Name: "sh", Args: []string{"-c", "echo the-original"},
		Stdin: "the original prompt", Wall: 10 * time.Second,
	}); err != nil {
		t.Fatalf("first Session: %v", err)
	}

	_, err := Session(context.Background(), dir, Spec{
		Name: "sh", Args: []string{"-c", "echo the-replacement"},
		Stdin: "the replacement prompt", Wall: 10 * time.Second,
	})

	if err == nil {
		t.Fatal("the second run overwrote a recorded run")
	}
	if got := readRaw(t, dir, "stdout"); !strings.Contains(got, "the-original") {
		t.Errorf("raw/stdout = %q, want the first run's output intact", got)
	}
	// The refusal has to come before anything is written, not merely before the
	// record is. A guard that fires after the prompt is overwritten leaves a
	// paid, unrepeatable run whose recorded output no longer matches its
	// recorded input — which misattributes silently instead of failing loudly.
	b, err := os.ReadFile(filepath.Join(dir, "prompt.txt"))
	if err != nil {
		t.Fatalf("prompt.txt: %v", err)
	}
	if string(b) != "the original prompt" {
		t.Errorf("prompt.txt = %q, want the first run's prompt untouched", b)
	}
}

// A run directory that does not contain its own input records what happened but
// not what was asked, so it can be neither reproduced nor invalidated.
func TestTheRunDirectoryKeepsThePromptItWasGiven(t *testing.T) {
	dir := t.TempDir()
	prompt := "Step 1: trace the path.\nStep 2: audit the dependents.\n"

	if _, err := Session(context.Background(), dir, Spec{
		Name: "cat", Stdin: prompt, Wall: 10 * time.Second,
	}); err != nil {
		t.Fatalf("Session: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "prompt.txt"))
	if err != nil {
		t.Fatalf("prompt.txt: %v", err)
	}
	if string(b) != prompt {
		t.Errorf("prompt.txt = %q, want the prompt the agent was given", b)
	}
}

func TestAProcessThatFailsIsRecordedWithItsExitCode(t *testing.T) {
	dir := t.TempDir()

	m, err := Session(context.Background(), dir, Spec{
		Name: "sh", Args: []string{"-c", "exit 3"},
		Wall: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	if m.Outcome != Failed {
		t.Errorf("outcome = %q, want %q", m.Outcome, Failed)
	}
	if m.ExitCode != 3 {
		t.Errorf("exit code = %d, want 3 — the process's own code, not a flattened 1", m.ExitCode)
	}
}

// The wall case is the whole reason this package exists first. A run that
// exceeds its budget is a result, and it must be recorded as one: the pressure
// this defends against is raising the wall to rescue a stalled arm, which is
// only resistible if "cannot finish at budget" is a normal thing to read.
func TestAProcessThatOverrunsItsWallIsRecordedAsCannotFinishAtBudget(t *testing.T) {
	dir := t.TempDir()

	m, err := Session(context.Background(), dir, Spec{
		Name: "sleep", Args: []string{"60"},
		Wall: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	if m.Outcome != CannotFinishAtBudget {
		t.Errorf("outcome = %q, want %q", m.Outcome, CannotFinishAtBudget)
	}
	// Two-sided on purpose. A one-sided "under 30s" check passes for a
	// hardcoded zero, and TookSeconds is the field that makes 481-against-480
	// legible later; an unmeasured one would read as a run that finished fast.
	if m.TookSeconds < 0.2 || m.TookSeconds > 5 {
		t.Errorf("took %.3fs, want at least the wall (0.2s) and well under 5s", m.TookSeconds)
	}
	if m.ExitCode != 124 {
		t.Errorf("exit code = %d, want 124 — what timeout(1) reports for a killed process", m.ExitCode)
	}
	// The wall that was set is part of run identity. Without it, a run at 481
	// seconds is unreadable: you cannot tell a fast wall from a slow agent.
	if m.WallSeconds != 0.2 {
		t.Errorf("wall = %.3fs, want the wall that was set (0.2)", m.WallSeconds)
	}
}

// run-meta.json is what every later query answers from, so it has to exist for
// the killed run too. The instrument being replaced left incomplete artifacts
// behind on exactly this path, and an arm with no record can never be paired.
func TestAKilledRunStillLeavesItsRecordOnDisk(t *testing.T) {
	dir := t.TempDir()

	if _, err := Session(context.Background(), dir, Spec{
		Name: "sh", Args: []string{"-c", "echo started; sleep 60"},
		Wall: 200 * time.Millisecond,
	}); err != nil {
		t.Fatalf("Session: %v", err)
	}

	if m := readMeta(t, dir); m.Outcome != CannotFinishAtBudget {
		t.Errorf("run-meta.json outcome = %q, want %q", m.Outcome, CannotFinishAtBudget)
	}
	// Output written before the kill survives: capture streams to disk rather
	// than buffering, so a killed run is still worth reading.
	if got := readRaw(t, dir, "stdout"); !strings.Contains(got, "started") {
		t.Errorf("raw/stdout = %q, want what the process said before it was killed", got)
	}
}

// The failure that has actually cost a run: killing the parent and leaving its
// children alive holding the pipe. A grandchild that outlives the wall means the
// wall does not really exist.
func TestTheWallKillsTheChildsChildrenToo(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "grandchild-still-running")

	// sh spawns a background grandchild that would create the marker two
	// seconds from now, then waits. Killing only sh leaves the grandchild.
	_, err := Session(context.Background(), dir, Spec{
		Name: "sh",
		Args: []string{"-c", "(sleep 2; touch " + marker + ") & echo spawned; wait"},
		Wall: 200 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	// Wait past the point the grandchild would have fired.
	time.Sleep(3 * time.Second)

	if _, err := os.Stat(marker); err == nil {
		t.Fatal("the grandchild outlived the wall: only the direct child was killed, " +
			"so an agent CLI's subprocesses would survive their budget")
	}
}

func TestAPromptIsHandedToTheProcessOnStdin(t *testing.T) {
	dir := t.TempDir()

	if _, err := Session(context.Background(), dir, Spec{
		Name: "cat",
		// A prompt carries newlines, quotes and backticks. Passing it as an
		// argument is what mangled a model id into a whole lost arm.
		Stdin: "line one\n`backtick` and \"quotes\"\n",
		Wall:  10 * time.Second,
	}); err != nil {
		t.Fatalf("Session: %v", err)
	}

	got := readRaw(t, dir, "stdout")
	if !strings.Contains(got, "`backtick` and \"quotes\"") {
		t.Errorf("raw/stdout = %q, want the prompt to arrive unmangled", got)
	}
}

// Env adds to the host environment rather than replacing it: an agent CLI
// needs PATH and HOME to run at all, and this cycle has no isolation to give it
// its own.
func TestEnvIsAddedToTheHostEnvironmentNotSubstitutedForIt(t *testing.T) {
	dir := t.TempDir()

	if _, err := Session(context.Background(), dir, Spec{
		Name: "sh", Args: []string{"-c", "echo added=$LAB_TEST_MARKER path=${PATH:+set}"},
		Env:  []string{"LAB_TEST_MARKER=present"},
		Wall: 10 * time.Second,
	}); err != nil {
		t.Fatalf("Session: %v", err)
	}

	got := readRaw(t, dir, "stdout")
	if !strings.Contains(got, "added=present") {
		t.Errorf("stdout = %q, want the added variable", got)
	}
	if !strings.Contains(got, "path=set") {
		t.Errorf("stdout = %q, want PATH inherited from the host", got)
	}
}

func TestTheSessionRunsInTheDirectoryItWasGiven(t *testing.T) {
	dir := t.TempDir()
	target := t.TempDir()

	if _, err := Session(context.Background(), dir, Spec{
		Dir: target, Name: "pwd", Wall: 10 * time.Second,
	}); err != nil {
		t.Fatalf("Session: %v", err)
	}

	// On macOS /var is a symlink to /private/var and the two sides disagree
	// about which form to print, so compare fully resolved paths.
	want, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatalf("resolve %s: %v", target, err)
	}
	got, err := filepath.EvalSymlinks(strings.TrimSpace(readRaw(t, dir, "stdout")))
	if err != nil {
		t.Fatalf("resolve the reported directory: %v", err)
	}
	if got != want {
		t.Errorf("ran in %q, want %q", got, want)
	}
}

// A command that does not exist is not a run that failed — it is a run that
// never happened, and recording it as an ordinary failure would put a scoreable
// artifact on disk for a session that produced nothing.
func TestACommandThatCannotBeStartedIsAnErrorNotAFailedRun(t *testing.T) {
	dir := t.TempDir()

	_, err := Session(context.Background(), dir, Spec{
		Name: "sense-lab-no-such-binary", Wall: 10 * time.Second,
	})

	if err == nil {
		t.Fatal("Session succeeded for a binary that does not exist")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "run-meta.json")); statErr == nil {
		t.Error("run-meta.json was written for a session that never started")
	}
}

// A cancelled parent context must stop the whole tree, not merely return. An
// earlier version of this test checked only that Session came back quickly, and
// it passed while every grandchild kept running and kept billing.
func TestCancellingTheContextKillsTheWholeTree(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(t.TempDir(), "grandchild-still-running")
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	started := time.Now()
	m, err := Session(ctx, dir, Spec{
		Name: "sh",
		Args: []string{"-c", "(sleep 2; touch " + marker + ") & echo spawned; wait"},
		Wall: time.Minute,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	if took := time.Since(started); took > 30*time.Second {
		t.Errorf("took %s — cancelling the context did not stop the run", took)
	}

	time.Sleep(3 * time.Second)
	if _, err := os.Stat(marker); err == nil {
		t.Error("the grandchild outlived the cancellation: an interrupted campaign " +
			"would leave agent sessions running and spending")
	}

	// An interrupt is not a budget failure. Recording it as one would put a row
	// on disk that reads exactly like an arm that stalled against its wall.
	if m.Outcome != Interrupted {
		t.Errorf("outcome = %q, want %q", m.Outcome, Interrupted)
	}
}

// A run that cannot be recorded must not report success. Silently losing the
// artifact is the worst outcome available here: the session is spent, and a
// caller that trusted the nil error would score nothing and call it a zero.
func TestASessionThatCannotBeRecordedReportsAnError(t *testing.T) {
	for _, tc := range []struct {
		name    string
		arrange func(t *testing.T, dir string)
		want    string
	}{
		{
			name: "the run directory cannot be made",
			arrange: func(t *testing.T, dir string) {
				// A file where raw/ needs to be.
				if err := os.WriteFile(filepath.Join(dir, "raw"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
			},
			want: "prepare run directory",
		},
		{
			name: "the capture files cannot be opened",
			arrange: func(t *testing.T, dir string) {
				// A directory where raw/stdout needs to be a file.
				if err := os.MkdirAll(filepath.Join(dir, "raw", "stdout"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "create stdout capture",
		},
		{
			name: "only stderr cannot be opened",
			arrange: func(t *testing.T, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, "raw", "stderr"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "create stderr capture",
		},
		{
			name: "the prompt cannot be written",
			arrange: func(t *testing.T, dir string) {
				if err := os.MkdirAll(filepath.Join(dir, "prompt.txt"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: "write prompt",
		},
		{
			name: "the record cannot be written",
			arrange: func(t *testing.T, dir string) {
				// Everything the run creates before the record already exists,
				// so the only thing left needing a new directory entry is
				// run-meta.json itself. A read-only directory still permits
				// rewriting the files inside it.
				if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
					t.Fatal(err)
				}
				for _, f := range []string{"prompt.txt", "raw/stdout", "raw/stderr"} {
					if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Chmod(dir, 0o500); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
			},
			want: "write run-meta",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.arrange(t, dir)

			_, err := Session(context.Background(), dir, Spec{
				Name: "true", Wall: 10 * time.Second,
			})

			if err == nil {
				t.Fatal("Session reported success for a run it could not record")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to name %q so the failure is diagnosable", err, tc.want)
			}
		})
	}
}

// A wall so short it expires before the process is even started still has to
// produce an honest record rather than a crash: the group-kill runs with no
// process to signal.
func TestAWallThatExpiresBeforeTheProcessStartsIsStillRecorded(t *testing.T) {
	dir := t.TempDir()

	m, err := Session(context.Background(), dir, Spec{
		Name: "sleep", Args: []string{"60"}, Wall: time.Nanosecond,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}

	if m.Outcome != CannotFinishAtBudget {
		t.Errorf("outcome = %q, want %q", m.Outcome, CannotFinishAtBudget)
	}
}

func TestOutcomeOfPrefersWhatEndedTheSessionOverTheExitCode(t *testing.T) {
	// A process killed from outside can still exit 0, if it finished in the
	// window between the signal and the reap. What killed it is what happened.
	if got := outcomeOf(0, endedByWall); got != CannotFinishAtBudget {
		t.Errorf("outcomeOf(0, endedByWall) = %q, want %q", got, CannotFinishAtBudget)
	}
	if got := outcomeOf(0, endedByCancel); got != Interrupted {
		t.Errorf("outcomeOf(0, endedByCancel) = %q, want %q", got, Interrupted)
	}
}

// A session that produced nothing is not the same as a session that ran and
// failed, but on disk they look identical: same outcome, same exit code. The
// measured case is an arm whose model id resolved to nothing — zero bytes,
// exit 1, and a campaign before anyone noticed. The byte count is what makes a
// broken spawn legible without reading transcripts.
func TestTheRecordSaysHowMuchTheSessionActuallySaid(t *testing.T) {
	t.Run("a session that said something", func(t *testing.T) {
		dir := t.TempDir()
		m, err := Session(context.Background(), dir, Spec{
			Name: "sh", Args: []string{"-c", "echo hello there"}, Wall: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("Session: %v", err)
		}
		if m.StdoutBytes != int64(len("hello there\n")) {
			t.Errorf("stdout_bytes = %d, want %d", m.StdoutBytes, len("hello there\n"))
		}
	})

	t.Run("a session that said nothing and failed", func(t *testing.T) {
		dir := t.TempDir()
		m, err := Session(context.Background(), dir, Spec{
			Name: "sh", Args: []string{"-c", "exit 1"}, Wall: 10 * time.Second,
		})
		if err != nil {
			t.Fatalf("Session: %v", err)
		}
		if m.Outcome != Failed {
			t.Fatalf("outcome = %q, want %q", m.Outcome, Failed)
		}
		// Zero, and recorded rather than absent: this is the shape that has to
		// be distinguishable from a real failure afterwards.
		if m.StdoutBytes != 0 {
			t.Errorf("stdout_bytes = %d, want 0", m.StdoutBytes)
		}
	})
}
