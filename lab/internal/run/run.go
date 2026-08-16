// Package run spawns one agent session under a wall and records what happened.
//
// It is the effectful half of the walking skeleton and it is deliberately thin:
// it spawns a command, streams its output to disk, enforces a wall, and writes a
// run directory. It decides nothing about scenarios, subjects or scores.
//
// Everything here is provisional. Cycle 03 replaces it with the isolated
// executor, and the run directory layout is settled by cycle 02 once the whole
// corpus has been read through it.
package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Outcome is how a session ended. The wall case is a first-class result, not an
// error: a run that exceeds its budget has told you something, and the honest
// record of it is what stops anyone raising the wall to rescue a stalled arm.
type Outcome string

const (
	// Completed: the process exited on its own with status 0.
	Completed Outcome = "completed"
	// Failed: the process exited on its own with a non-zero status.
	Failed Outcome = "failed"
	// CannotFinishAtBudget: the wall expired and the process was killed.
	CannotFinishAtBudget Outcome = "cannot finish at budget"
	// Interrupted: the caller's context was cancelled and the process was
	// killed. This is NOT a budget failure and must never be read as one: an
	// operator pressing Ctrl-C says nothing about whether the session could
	// have finished, and a row that conflates the two reads as a stalled arm.
	Interrupted Outcome = "interrupted"
)

// Spec is one session to run. Paths are absolute.
type Spec struct {
	// Dir is the working directory the command is spawned in.
	Dir string
	// Name and Args are the command to spawn.
	Name string
	Args []string
	// Stdin is handed to the process on its standard input. Agent CLIs take
	// their prompt this way; a prompt beginning with a dash is read as an
	// option when passed as an argument, which has already killed a spawn.
	Stdin string
	// Env is the session's complete environment, as KEY=VALUE. It is complete
	// rather than additive: a measured run is handed a scrubbed environment by
	// lab/internal/isolate and inherits nothing it was not given. A caller that
	// genuinely wants the host's environment composes it at the call site,
	// where the decision is visible.
	//
	// A nil Env leaves the session with the parent's environment, which is only
	// ever right for a test.
	Env []string
	// Arm is which side of a cell this session is, recorded so a run can say so
	// without anyone re-deriving it from a directory name.
	Arm string
	// SenseSetup is what the subject's setup wrote into the repository, as path
	// to content hash. Empty for the baseline arm, which is the point.
	SenseSetup map[string]string
	// Wall is how long the process may take. It is part of run identity and is
	// never raised to rescue a stalled arm: a wall that can be lifted at the
	// moment of frustration is not a measurement.
	Wall time.Duration
	// Grace is how long a session gets to stop on its own after being asked,
	// before it is killed. Zero means defaultGrace.
	Grace time.Duration
}

// Meta is the self-describing record left in every run directory. It is what a
// later query answers from, so it records the wall that was set as well as the
// time actually taken: a run at 481 seconds against a 480 second wall reads
// very differently from one that finished in 30.
type Meta struct {
	Outcome     Outcome `json:"outcome"`
	ExitCode    int     `json:"exit_code"`
	WallSeconds float64 `json:"wall_seconds"`
	// WallStartsAt is the point the wall is counted from, recorded because it is
	// a property of the measurement rather than of the code.
	//
	// The sense arm pays for MCP server startup, which is slow to first output.
	// A wall counted from first streamed event would charge that to the sense
	// arm alone, and the two arms would have different effective budgets while
	// appearing to share one. It is spawn, for both arms, and it is written down
	// so a later reader does not have to take that on trust.
	WallStartsAt string   `json:"wall_starts_at"`
	TookSeconds  float64  `json:"took_seconds"`
	Command      string   `json:"command"`
	Args         []string `json:"args"`
	StartedAt    string   `json:"started_at"`
	// StdoutBytes is how much the session actually said.
	//
	// It is here because of a specific failure: an arm whose model id resolved
	// to nothing produced zero bytes and exited 1, and that is byte-identical
	// in shape to a session that ran and legitimately failed. A run with a
	// real wall and no output is not a result, it is a broken spawn, and this
	// is what makes the difference visible without reading transcripts.
	StdoutBytes int64 `json:"stdout_bytes"`

	// Arm, Home and Path record the isolation the session actually ran under.
	//
	// Path is here because it is one of the six channels Sense reaches an agent
	// through, and it is the one that differs by arm. Recording it is what makes
	// "the baseline arm could not reach the Sense binary" a fact on disk rather
	// than a claim about what the code intended. Home is recorded beside it so
	// a run says which disposable HOME it saw.
	//
	// Nothing else from the environment is recorded: it carries credentials.
	Arm  string `json:"arm,omitempty"`
	Home string `json:"home,omitempty"`
	Path string `json:"path,omitempty"`

	// SenseSetup is what the subject's setup wrote into the repository, as path
	// to content hash.
	//
	// A change to `sense setup` changes what the sense arm sees, which changes
	// the measurement, and nothing about that is visible in a result months
	// later. Recorded per run, two runs far apart are comparable on what was
	// installed rather than on an assumption that it was the same.
	SenseSetup map[string]string `json:"sense_setup,omitempty"`
}

// envValue reads one KEY=VALUE out of an environment slice. The last entry
// wins, which is what exec does with a duplicate.
func envValue(env []string, key string) string {
	value := ""
	for _, kv := range env {
		if name, v, ok := strings.Cut(kv, "="); ok && name == key {
			value = v
		}
	}
	return value
}

// defaultGrace is how long a session gets to stop on its own. An agent CLI
// traps its first signal and spends seconds flushing, and ten seconds of
// graceful cleanup is normal behaviour for the thing being measured.
const defaultGrace = 10 * time.Second

func (s Spec) grace() time.Duration {
	if s.Grace <= 0 {
		return defaultGrace
	}
	return s.Grace
}

// exitCodeKilled is what Meta records when the process was killed from outside.
// Its real code is meaningless once we sent the signal, and 124 is what
// timeout(1) uses.
const exitCodeKilled = 124

// wallStartsAtSpawn is the one wall start point, for both arms.
const wallStartsAtSpawn = "spawn"

// metaFile is the run's self-describing record. Its presence marks a directory
// as holding a run that already happened.
const metaFile = "run-meta.json"

// Session runs one Spec to completion, to failure, or to its wall, streaming
// stdout and stderr to files under dir/raw/ as they arrive. It returns the Meta
// it wrote. An error means the run could not be recorded at all; a process that
// failed or hit its wall is a Meta, not an error.
//
// One honest limit: a child that exits cleanly having left its own children
// running is reaped immediately, and those orphans still hold the capture files
// and can write into them after run-meta.json lands. Every path that kills the
// session signals the whole group, so this only affects a session that ended by
// itself. Cycle 03's isolation is what closes it properly.
func Session(ctx context.Context, dir string, s Spec) (Meta, error) {
	// A recorded run is paid for and can never be recreated: the model is
	// nondeterministic and the spend is already gone. Refuse to write over one
	// rather than silently destroying it, which is what a repeated command with
	// the same -out would otherwise do.
	if _, err := os.Stat(filepath.Join(dir, metaFile)); err == nil {
		return Meta{}, fmt.Errorf("%s already holds a recorded run; runs are immutable, use a new directory", dir)
	}
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		return Meta{}, fmt.Errorf("prepare run directory: %w", err)
	}

	// The prompt is the run's input. Without it on disk the directory records
	// what happened but not what was asked, so it can be neither reproduced nor
	// invalidated.
	if err := os.WriteFile(filepath.Join(dir, "prompt.txt"), []byte(s.Stdin), 0o644); err != nil {
		return Meta{}, fmt.Errorf("write prompt: %w", err)
	}

	stdout, stderr, err := rawFiles(dir)
	if err != nil {
		return Meta{}, err
	}
	defer func() { _ = stdout.Close(); _ = stderr.Close() }()

	started := time.Now()
	code, ended, err := spawn(ctx, s, stdout, stderr)
	if err != nil {
		return Meta{}, err
	}

	said, _ := stdout.Seek(0, io.SeekCurrent)

	m := Meta{
		Outcome:      outcomeOf(code, ended),
		StdoutBytes:  said,
		ExitCode:     code,
		WallSeconds:  s.Wall.Seconds(),
		WallStartsAt: wallStartsAtSpawn,
		TookSeconds:  time.Since(started).Seconds(),
		Command:      s.Name,
		Args:         s.Args,
		StartedAt:    started.UTC().Format(time.RFC3339),
		Arm:          s.Arm,
		SenseSetup:   s.SenseSetup,
		Home:         envValue(s.Env, "HOME"),
		Path:         envValue(s.Env, "PATH"),
	}
	if err := writeMeta(dir, m); err != nil {
		return Meta{}, err
	}
	return m, nil
}

// endedBy says what stopped the session, when it was not the process itself.
type endedBy int

const (
	endedByItself endedBy = iota
	endedByWall
	endedByCancel
)

// outcomeOf maps an exit code plus what ended the session onto an Outcome.
// Whatever ended the session wins over the exit code: a process killed from
// outside may exit with any code, including 0 if it happened to finish in the
// window between the signal and the reap.
func outcomeOf(code int, ended endedBy) Outcome {
	switch {
	case ended == endedByWall:
		return CannotFinishAtBudget
	case ended == endedByCancel:
		return Interrupted
	case code == 0:
		return Completed
	default:
		return Failed
	}
}

// rawFiles opens the two capture files. Output is streamed to them as it
// arrives rather than buffered, so a run killed on its wall still leaves
// everything the process managed to say.
func rawFiles(dir string) (stdout, stderr *os.File, err error) {
	stdout, err = os.Create(filepath.Join(dir, "raw", "stdout"))
	if err != nil {
		return nil, nil, fmt.Errorf("create stdout capture: %w", err)
	}
	stderr, err = os.Create(filepath.Join(dir, "raw", "stderr"))
	if err != nil {
		_ = stdout.Close()
		return nil, nil, fmt.Errorf("create stderr capture: %w", err)
	}
	return stdout, stderr, nil
}

// spawn runs the command under the wall and reports its exit code and what
// ended it. It returns an error only when the command could not be started at
// all.
func spawn(ctx context.Context, s Spec, stdout, stderr *os.File) (code int, ended endedBy, err error) {
	parent := ctx
	ctx, cancel := context.WithTimeout(ctx, s.Wall)
	defer cancel()

	cmd := exec.CommandContext(ctx, s.Name, s.Args...)
	cmd.Dir = s.Dir
	// These MUST stay *os.File. os/exec then hands the descriptors straight to
	// the child and starts no copying goroutines, so Wait returns as soon as
	// the direct child is reaped. Wrap either one (io.MultiWriter, a tee) and
	// exec switches to a pipe, Wait blocks until every process holding that
	// pipe closes it, and the wall silently stops existing — the exact failure
	// the process-group kill below exists to prevent, reintroduced from the
	// other side. If a pipe is ever genuinely needed, set cmd.WaitDelay too.
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	cmd.Stdin = strings.NewReader(s.Stdin)
	cmd.Env = s.Env

	// Kill the whole process group, not just the child. An agent CLI spawns its
	// own children, and killing only the parent leaves them holding the pipe:
	// the wall passes, the run does not end, and the cell's other arm is burned.
	//
	// This runs as cmd.Cancel rather than after Run returns, which matters
	// twice: the child is still alive so its process group id is certainly
	// valid (signalling a reaped pid can hit whoever the OS handed it to next),
	// and Cancel fires on the parent's cancellation as well as the wall, so an
	// interrupted campaign does not leave agents running and spending.
	setProcessGroup(cmd)
	cmd.Cancel = func() error { return stopGroup(cmd, s.grace()) }

	runErr := cmd.Run()

	// Being stopped from outside and merely failing are indistinguishable from
	// the error alone, so ask the contexts which one happened. The parent is
	// checked first: an operator interrupt is not a budget failure.
	switch {
	case parent.Err() != nil:
		return exitCodeKilled, endedByCancel, nil
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return exitCodeKilled, endedByWall, nil
	}

	var exitErr *exec.ExitError
	switch {
	case runErr == nil:
		return 0, endedByItself, nil
	case errors.As(runErr, &exitErr):
		return exitErr.ExitCode(), endedByItself, nil
	default:
		return 0, endedByItself, fmt.Errorf("spawn %s: %w", s.Name, runErr)
	}
}

// writeMeta writes the run's self-describing record. This file is what every
// later query answers from, so it is written last: its presence means the run
// was recorded.
func writeMeta(dir string, m Meta) error {
	// Meta is a fixed struct of scalars, so encoding it cannot fail.
	b, _ := json.MarshalIndent(m, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, metaFile), append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write run-meta: %w", err)
	}
	return nil
}
