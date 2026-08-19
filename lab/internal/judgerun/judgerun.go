// Package judgerun sends one grading to a model and records what graded it.
//
// The judge is a model call inside a measurement, which makes it the one
// component that can quietly change what every past number meant. Three ways
// that happens, and each has a mechanism here:
//
//   - it sees which arm it is grading, and then it is grading a label
//   - it has tools, so it may verify claims itself, and whether it chooses to
//     varies run to run. That is invisible grading variance and it changes what
//     is measured: a judge that went and looked is grading correctness, one that
//     did not is grading the answer as written
//   - it drifts, and a judge that moves with the headline arm makes every board
//     incomparable
//
// The first two are prevented. The third is only ever made visible: the model
// is pinned and recorded, and the agent tool's version is part of run identity,
// so a harness change produces a new cell instead of silent movement. That is
// detection rather than prevention, and it is the honest amount of protection
// available.
package judgerun

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luuuc/sense/lab/internal/isolate"
	"github.com/luuuc/sense/lab/internal/judge"
	"github.com/luuuc/sense/lab/internal/run"
)

// Spec is one grading to send.
type Spec struct {
	// Root is the directory to record the grading in. It must not exist.
	Root string
	// Rows are the gold rows the answer is graded against.
	Rows []judge.Gold
	// Answer is what the arm said. It is the only thing about the run that
	// reaches the judge.
	Answer string
	// Command and Args spawn the agent tool. Args must be the tool's TOOL-LESS,
	// single-turn form: a judge with tools is a different instrument from one
	// without, and which one graded a run would not be visible in the number.
	Command string
	Args    []string
	// AgentEnv is what the agent tool declares.
	AgentEnv []string
	// Model is the pinned judge model, and ModelFlag selects it. It comes from
	// the repository's bench rather than from the arm, because a judge that moves with
	// the headline arm makes every board incomparable.
	Model     string
	ModelFlag string
	// HostPath is the PATH the grading derives from.
	HostPath string
	// Wall and Grace bound the grading.
	Wall  time.Duration
	Grace time.Duration
}

// Result is one grading and what produced it.
type Result struct {
	// Verdict and Score are what the judge said and what it means.
	Verdict judge.Verdict
	Score   judge.Result
	// Model is what graded this. Recorded per run, because a board is only
	// comparable across runs that were graded by the same thing.
	Model string
	// Root is where the grading was recorded.
	Root string
}

// armLabels are the words that would tell the judge which arm it is grading.
//
// They are checked on the payload rather than on the code that builds it: the
// code is right until somebody adds a helpful line to a prompt, and nothing
// about the resulting grade would look wrong.
var armLabels = []string{
	string(isolate.Sense),
	string(isolate.Baseline),
	"treatment arm",
	"control arm",
}

// Leaks reports every arm label the BENCH would put in front of the judge.
//
// The answer is excluded from the scan on purpose, and this is the limit worth
// recording rather than papering over: an answer that names the tools it used
// identifies its own arm, and no scrubbing removes that without corrupting the
// text being graded. What is checked is everything the bench authored — the
// framing and the gold relations — because that is the part the bench controls
// and the part that would go wrong when somebody adds a helpful line.
//
// It is checked on the payload rather than on the code that builds it. The code
// is right until it is not, and nothing about the resulting grade would look
// wrong.
func Leaks(payload, answer string) []string {
	framing := payload
	if answer != "" {
		framing = strings.ReplaceAll(payload, answer, "")
	}
	lower := strings.ToLower(framing)
	var found []string
	for _, label := range armLabels {
		if strings.Contains(lower, strings.ToLower(label)) {
			found = append(found, label)
		}
	}
	return found
}

// judgeArgs is the tool's tool-less form plus the pinned model.
func judgeArgs(s Spec) []string {
	args := append([]string{}, s.Args...)
	if s.ModelFlag != "" {
		args = append(args, s.ModelFlag)
	}
	return append(args, s.Model)
}

// Run grades one answer and returns the verdict with what produced it.
func Run(ctx context.Context, s Spec) (Result, error) {
	if s.Model == "" {
		// An unpinned judge is a judge that drifts, and the drift is invisible.
		return Result{}, errors.New("no judge model pinned; a judge inherited from the arm makes every board incomparable")
	}

	payload := judge.Instruction(s.Rows, s.Answer)
	if leaked := Leaks(payload, s.Answer); len(leaked) != 0 {
		return Result{}, fmt.Errorf("the grading would tell the judge which arm it is grading: %v", leaked)
	}

	// A disposable environment with an EMPTY working directory. The judge is
	// spawned somewhere with no repository, no MCP registration and no routing
	// guidance, so tool-lessness does not rest on a flag alone: there is
	// nothing to reach even if a tool were enabled.
	env, err := isolate.Prepare(isolate.Spec{
		Root:     s.Root,
		Arm:      isolate.Baseline, // no Sense binary reachable, for the same reason
		HostPath: s.HostPath,
		AgentEnv: s.AgentEnv,
	})
	if err != nil {
		return Result{}, err
	}
	cwd := filepath.Join(env.Root, "grading")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		return Result{}, fmt.Errorf("prepare the grading directory: %w", err)
	}

	m, err := run.Session(ctx, filepath.Join(env.Root, "session"), run.Spec{
		Dir:   cwd,
		Name:  s.Command,
		Args:  judgeArgs(s),
		Stdin: payload,
		Env:   env.Environ,
		Wall:  s.Wall,
		Grace: s.Grace,
	})
	if err != nil {
		return Result{}, err
	}
	if m.Outcome != run.Completed {
		return Result{}, fmt.Errorf("the grading %s", m.Outcome)
	}

	said, err := os.ReadFile(filepath.Join(env.Root, "session", "raw", "stdout")) // #nosec G304 -- the grading's own capture
	if err != nil {
		return Result{}, fmt.Errorf("read the grading: %w", err)
	}
	v, err := judge.Parse(said)
	if err != nil {
		return Result{}, err
	}
	score, err := judge.Grade(s.Rows, v)
	if err != nil {
		return Result{}, err
	}
	return Result{Verdict: v, Score: score, Model: s.Model, Root: env.Root}, nil
}
