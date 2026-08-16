// Package probe runs both arms of one job and proves they differed in exactly
// one thing.
//
// A one-arm run measures nothing. The number that matters is a difference,
// sense minus baseline, and the difference is only meaningful if the two arms
// were the same agent, on the same repository, at the same commit, under the
// same budget, differing in Sense access and nothing else.
//
// That claim is what the whole corpus rests on, so it is checked with an output
// rather than believed because the setup code looks right.
package probe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/luuuc/sense/lab/internal/cell"
	"github.com/luuuc/sense/lab/internal/channels"
	"github.com/luuuc/sense/lab/internal/isolate"
	"github.com/luuuc/sense/lab/internal/run"
	"github.com/luuuc/sense/lab/internal/session"
)

// Spec is one job: the same everything, run twice.
type Spec struct {
	// Root is the cell directory to create, holding one run per arm.
	Root string
	// Parent is the repository each worktree is taken from, at Commit.
	Parent string
	Commit string
	// Prompt is what both arms are asked. It is one field, not two, because
	// there is no version of this where the arms are asked different things:
	// an asymmetry is fixed in the environment, never in the question.
	Prompt string
	// Command and Args spawn the agent tool, and AgentEnv is what it declares.
	Command  string
	Args     []string
	AgentEnv []string
	// SenseBin is the Sense binary and LabBin is this one.
	SenseBin string
	LabBin   string
	// HostPath is the PATH both arms derive from.
	HostPath string
	// Wall is the sense arm's budget. The baseline's is derived from it.
	Wall  time.Duration
	Grace time.Duration
}

// Arms, named once so a directory name and a check read the same.
const (
	senseArm    = "sense"
	baselineArm = "baseline"
)

// BaselineWall is the baseline arm's budget, derived from the sense arm's.
//
// It is a function rather than an assignment because the relationship is easy
// to get backwards, and getting it backwards is not visible in a result. The
// two walls are not independently chosen: the baseline's derives from the sense
// arm it is paired with, which is also why a burned sense arm can never be
// paired with a later baseline.
func BaselineWall(senseWall time.Duration) time.Duration { return senseWall }

// Report is what the pair proved.
type Report struct {
	// Sense and Baseline are the two arms' environments and metadata.
	Sense    session.Result
	Baseline session.Result
	// SenseMissing names the routes the sense arm was supposed to have and does
	// not. Empty is the only acceptable value: an arm configured for Sense that
	// is missing one is a weakened treatment, and nothing in a score shows it.
	SenseMissing []string
	// BaselineReached names the routes the baseline arm could still use. Empty
	// is the only acceptable value.
	BaselineReached []string
	// MemoryReached names persisted state either arm could read. It is
	// contamination rather than treatment, so it must be empty for BOTH arms:
	// the sense arm reading a memory directory is measuring a previous run.
	MemoryReached []string
	// BaselineUsed names every sign the baseline's transcript used Sense.
	// Empty is the only acceptable value.
	BaselineUsed []string
	// SenseUsed names every sign the sense arm's transcript used Sense. Empty
	// here is a different failure: an arm that had Sense and never touched it.
	SenseUsed []string
	// Frames is how many MCP frames the sense arm's capture holds.
	Frames int
	// BaselineCaptured says whether the baseline arm left a capture file. It
	// should not have one at all: an empty file would be indistinguishable
	// from a capture failure, and its absence is the signal.
	BaselineCaptured bool
	// Differences names every way the two arms differ other than Sense access.
	// Empty is the only acceptable value.
	Differences []string
}

// Sound reports whether the pair is a measurement.
func (r Report) Sound() bool {
	return len(r.SenseMissing) == 0 &&
		len(r.MemoryReached) == 0 &&
		len(r.BaselineReached) == 0 &&
		len(r.BaselineUsed) == 0 &&
		!r.BaselineCaptured &&
		len(r.Differences) == 0 &&
		len(r.SenseUsed) > 0 &&
		r.Frames > 0
}

// Run runs both arms in sequence and checks the claim the corpus rests on.
//
// The arms run under this process, in order, so an interruption between them
// cannot produce a half-pair: lab/internal/cell owns that, and this is the
// caller that needs it.
func Run(ctx context.Context, s Spec) (Report, error) {
	var sense, baseline session.Result
	arms := []cell.Arm{
		{Name: senseArm, Run: s.armRunner(isolate.Sense, s.Wall, &sense)},
		{Name: baselineArm, Run: s.armRunner(isolate.Baseline, BaselineWall(s.Wall), &baseline)},
	}

	rec, err := cell.Run(ctx, s.Root, arms)
	if err != nil {
		return Report{}, err
	}
	if !rec.Complete {
		// The finished arm is paid for and can never be paired, because the
		// baseline's budget derives from the sense arm it ran with. The cell
		// record names it so a later pass refuses it rather than quietly
		// pairing it with something else.
		return Report{}, fmt.Errorf("the cell is incomplete: %v has no result, and %v can never be paired",
			rec.Unusable, rec.Burned)
	}

	return s.check(ctx, sense, baseline)
}

// armRunner is one side of the pair, as the supervisor sees it: a function that
// produces a run in the directory it is handed, and leaves its result where the
// checks can read it.
func (s Spec) armRunner(arm isolate.Arm, wall time.Duration, into *session.Result) func(context.Context, string) (run.Meta, error) {
	return func(ctx context.Context, dir string) (run.Meta, error) {
		res, err := session.Run(ctx, s.arm(dir, arm, wall))
		if err != nil {
			return run.Meta{}, err
		}
		*into = res
		return res.Meta, nil
	}
}

// arm builds one side of the pair. Everything except the arm and its wall is
// shared by construction rather than by two call sites that must be kept the
// same: an asymmetry between the arms is the one failure that invalidates every
// number without being visible in any of them.
func (s Spec) arm(root string, arm isolate.Arm, wall time.Duration) session.Spec {
	return session.Spec{
		Root:     root,
		Arm:      arm,
		Parent:   s.Parent,
		Commit:   s.Commit,
		Prompt:   s.Prompt,
		Command:  s.Command,
		Args:     s.Args,
		AgentEnv: s.AgentEnv,
		SenseBin: s.SenseBin,
		LabBin:   s.LabBin,
		HostPath: s.HostPath,
		Wall:     wall,
		Grace:    s.Grace,
	}
}

// check runs every proof against the pair that just ran.
func (s Spec) check(ctx context.Context, sense, baseline session.Result) (Report, error) {
	r := Report{Sense: sense, Baseline: baseline}

	derived, err := channels.Derive(ctx, s.SenseBin, filepath.Join(s.Root, "channel-probe"))
	if err != nil {
		return Report{}, err
	}
	tools, err := channels.ToolNames(ctx, s.SenseBin, sense.Env.Repo)
	if err != nil {
		return Report{}, err
	}
	binary := filepath.Base(s.SenseBin)

	// The routes split in two. The repository files and the binary on PATH are
	// the treatment: the sense arm must have every one and the baseline none.
	// Persisted memory is not a treatment at all — it is state carried between
	// runs — so it must be absent from both, and an arm that has it is
	// measuring a previous session rather than this one.
	treatment, contamination := split(derived)
	senseWorld, baselineWorld := armWorld(sense, binary), armWorld(baseline, binary)

	r.SenseMissing = missing(treatment, channels.Absent(treatment, senseWorld))
	r.BaselineReached = channels.Absent(treatment, baselineWorld)
	r.MemoryReached = append(
		channels.Absent(contamination, senseWorld),
		channels.Absent(contamination, baselineWorld)...)

	senseSaid, err := transcript(sense)
	if err != nil {
		return Report{}, err
	}
	baselineSaid, err := transcript(baseline)
	if err != nil {
		return Report{}, err
	}
	r.SenseUsed = channels.UsedBy(senseSaid, tools, binary)
	r.BaselineUsed = channels.UsedBy(baselineSaid, tools, binary)

	frames, err := countFrames(session.LogPath(sense.Env))
	if err != nil {
		return Report{}, err
	}
	r.Frames = frames
	_, r.BaselineCaptured, err = session.Capture(baseline.Env)
	if err != nil {
		return Report{}, err
	}
	if _, err := os.Stat(session.LogPath(baseline.Env)); err == nil {
		r.BaselineCaptured = true
	}

	r.Differences = Differences(sense.Meta, baseline.Meta)
	return r, nil
}

// split separates the routes that are the treatment from the one that is
// contamination.
func split(all []channels.Channel) (treatment, contamination []channels.Channel) {
	for _, c := range all {
		if c.Kind == channels.Home {
			contamination = append(contamination, c)
			continue
		}
		treatment = append(treatment, c)
	}
	return treatment, contamination
}

// missing names the routes that were not reached, given the ones that were.
func missing(all []channels.Channel, reached []string) []string {
	var absent []string
	for _, c := range all {
		found := false
		for _, r := range reached {
			if strings.HasPrefix(r, c.Name+":") {
				found = true
				break
			}
		}
		if !found {
			absent = append(absent, c.Name)
		}
	}
	return absent
}

// armWorld is where one arm's channels would be if it had them.
func armWorld(res session.Result, binary string) channels.Arm {
	return channels.Arm{
		Repo:        res.Env.Repo,
		Home:        res.Env.Home,
		PathValue:   res.Meta.Path,
		SenseBinary: binary,
	}
}

// Differences names every way the two arms differ other than Sense access.
//
// Read off what was recorded rather than off what was configured: the two are
// the same only when nothing went wrong, and the case worth catching is the one
// where something did.
func Differences(sense, baseline run.Meta) []string {
	var found []string
	compare := func(what, a, b string) {
		if a != b {
			found = append(found, fmt.Sprintf("%s: the sense arm had %q and the baseline had %q", what, a, b))
		}
	}
	compare("the agent tool", sense.Command, baseline.Command)
	compare("the arguments", fmt.Sprint(sense.Args), fmt.Sprint(baseline.Args))
	compare("where the wall starts", sense.WallStartsAt, baseline.WallStartsAt)
	if sense.WallSeconds != baseline.WallSeconds {
		found = append(found, fmt.Sprintf("the budget: the sense arm had %.0fs and the baseline had %.0fs, "+
			"but the baseline's derives from the sense arm's", sense.WallSeconds, baseline.WallSeconds))
	}
	return found
}

// transcript is what an arm actually said, unnormalised. The canonical form is
// cycle 02's and is built from the same bytes.
func transcript(res session.Result) ([]byte, error) {
	path := filepath.Join(res.Env.Root, "session", "raw", "stdout")
	b, err := os.ReadFile(path) // #nosec G304 -- the run's own capture
	if err != nil {
		return nil, fmt.Errorf("read the %s arm's transcript: %w", res.Meta.Arm, err)
	}
	return b, nil
}

// countFrames is how much the sense arm's capture holds. Zero on a sense arm is
// a finding: either Sense was never reached or the capture never worked, and
// the run cannot tell those apart on its own.
func countFrames(path string) (int, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- the run's own capture
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read the capture: %w", err)
	}
	n := 0
	for _, line := range bytes.Split(b, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			n++
		}
	}
	return n, nil
}
