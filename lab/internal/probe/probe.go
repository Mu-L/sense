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
	"slices"
	"strings"
	"time"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/cell"
	"github.com/luuuc/sense/lab/internal/channels"
	"github.com/luuuc/sense/lab/internal/isolate"
	"github.com/luuuc/sense/lab/internal/run"
	"github.com/luuuc/sense/lab/internal/session"
	"github.com/luuuc/sense/lab/internal/transcript"
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
	// Command and Args spawn the agent tool, AgentEnv is what it declares, and
	// SetupTool is its catalog id, which is what `sense setup` configures for,
	// and ConfigDir is the directory it keeps per-user state in, which the
	// contamination checks look inside.
	Command  string
	Args     []string
	AgentEnv []string
	// Agent is the tool's catalog entry, carried so each arm can be told its
	// own wall in the words that tool understands.
	Agent      catalog.Agent
	SetupTool  string
	ConfigDirs []string
	// Credential is what both arms are given to reach a model. One value, not
	// two: an asymmetry in what the arms could reach would be an asymmetry in
	// what they could do, and nothing in a score would show it.
	Credential isolate.Credential
	// Route is how that credential reaches the agent tool, from the catalog.
	Route isolate.Route
	// SenseBin is the Sense binary and LabBin is this one.
	SenseBin string
	LabBin   string
	// HostPath is the PATH both arms derive from.
	HostPath string
	// Wall is the sense arm's budget. The baseline's derives from what the
	// sense arm ELAPSED, so it is not knowable here.
	Wall  time.Duration
	Grace time.Duration
}

// Arms, named once so a directory name and a check read the same.
const (
	senseArm    = "sense"
	baselineArm = "baseline"
)

// baselineWallRatio is what the baseline gets, as a multiple of what the sense
// arm actually spent.
//
// Above one deliberately. The baseline has to find by reading what the sense
// arm can look up, so an equal budget is not an equal question, and the corpus
// this instrument has to stay comparable with was banked at 1.2.
const baselineWallRatio = 1.2

// BaselineWall is the baseline arm's budget, derived from what the sense arm
// ELAPSED — not from what it was allowed.
//
// The distinction is the whole function and it is easy to lose. A sense arm
// given 480 seconds that finishes in 404 leaves the baseline 485, not 576: the
// pairing is against the work the sense arm actually did. Deriving from the
// budget instead silently hands the baseline more clock every time the sense
// arm finishes early, which shrinks every margin in the same direction without
// appearing in any of them.
//
// That is not hypothetical. This function returned its argument unchanged until
// 2026-08-17, when replaying the banked mastodon cell showed the banked run had
// derived 485s from a 404s sense arm while this derived 480s from its ceiling.
// The two agreed only because that live sense arm ran out of time.
//
// It is a function rather than an assignment because the relationship is easy
// to get backwards, and getting it backwards is not visible in a result. The
// two walls are not independently chosen: the baseline's derives from the sense
// arm it is paired with, which is also why a burned sense arm can never be
// paired with a later baseline.
func BaselineWall(senseElapsed time.Duration) time.Duration {
	wall := time.Duration(float64(senseElapsed) * baselineWallRatio).Round(time.Second)
	// A derived wall below a second is not a budget, it is an immediate kill,
	// and the arm it kills is indistinguishable in a score from a baseline that
	// simply had nothing to say. Rounding makes it worse: 1.2 times a few
	// milliseconds rounds to exactly zero.
	//
	// It happens whenever the sense arm returns almost at once, which is not
	// hypothetical — a session that cannot authenticate exits in about a second
	// having done nothing. Such a pair is already unsound and is refused on the
	// sense arm, which is the real fault; this only stops it being reported as a
	// baseline that could not finish.
	if wall < time.Second {
		return time.Second
	}
	return wall
}

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
	// SenseCalls and BaselineCalls are how many tool calls each arm made and how
	// many of them reached Sense.
	//
	// Reported, never gated. An earlier instrument drove cloud models through a
	// CLI that technically accepted them and they ignored Sense almost
	// entirely: 2 Sense calls against 97 native ones, in a session that
	// COMPLETED and passed every other check. So a completed run is not
	// evidence of a working arm, and the tell is in the counts.
	//
	// There is deliberately no threshold. 2 against 97 is obvious; 15 against
	// 40 is a judgement nobody has the data to automate, and a number invented
	// here would be a screen wearing a check's clothes. A human reads it.
	SenseCalls    Calls
	BaselineCalls Calls
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

// Calls is how a single arm spent its tool calls.
type Calls struct {
	// Sense is how many of them reached Sense.
	Sense int
	// Total is how many it made altogether.
	Total int
}

// String is the one line a human reads.
func (c Calls) String() string {
	return fmt.Sprintf("%d of %d tool calls reached Sense", c.Sense, c.Total)
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
	// A cell whose agent is unknown cannot have its contamination checked: an
	// empty config dir joins to the disposable HOME itself, which exists for
	// both arms, so every arm reads as contaminated. Refused here rather than
	// producing a report nobody can act on.
	if s.SetupTool == "" || len(s.ConfigDirs) == 0 {
		return Report{}, fmt.Errorf("the agent is not identified: setup tool %q, config dirs %v; "+
			"both come from the catalog and the contamination checks cannot run without them",
			s.SetupTool, s.ConfigDirs)
	}

	var sense, baseline session.Result
	// The baseline's wall is resolved when the baseline starts, not here,
	// because it derives from what the sense arm ELAPSED and the sense arm has
	// not run yet. The arms run in sequence, so by the time this closure is
	// called `sense` is filled in.
	arms := []cell.Arm{
		{Name: senseArm, Run: s.armRunner(isolate.Sense, func() time.Duration { return s.Wall }, &sense)},
		{Name: baselineArm, Run: s.armRunner(isolate.Baseline, func() time.Duration { return s.baselineWall(sense) }, &baseline)},
	}

	rec, err := cell.Run(ctx, s.Root, arms)

	// Whatever happened, the checkouts go back — but only after the checks have
	// read them, because the repository routes are proved by looking inside the
	// worktree and a released one would read as an arm that never had Sense.
	//
	// This is the call that was missing: the removal has been correct and
	// unreachable from the binary since cycle 03, and one mastodon pair left
	// 230MB and about 19,700 files behind. It runs on every path, because an
	// interrupted or refused attempt leaks exactly the registration a finished
	// one would — three of those accumulated in a single afternoon.
	report, checkErr := s.report(ctx, rec, err, sense, baseline)
	return report, errors.Join(checkErr, s.release(ctx, sense, baseline))
}

// report is the cell's outcome, before anything is given back.
func (s Spec) report(ctx context.Context, rec cell.Record, err error, sense, baseline session.Result) (Report, error) {
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

// release gives back both arms' checkouts, in whichever shape each directory
// calls for. An arm that never ran has no environment and nothing to give back.
//
// A release failure is reported rather than swallowed: it means a registration
// or a credential is still there, and the next attempt at that path is refused.
func (s Spec) release(ctx context.Context, arms ...session.Result) error {
	// Detached from the caller's cancellation, deliberately. Release shells out to
	// git, and the case that most needs releasing is the interrupted cell — where
	// the context is already cancelled, so every one of those commands would be
	// killed before it ran and the registration would leak exactly when it costs
	// most. The removals are bounded work on local paths, not something a user
	// waits on.
	ctx = context.WithoutCancel(ctx)

	var errs []error
	for _, a := range arms {
		if a.Env.Root == "" {
			continue
		}
		errs = append(errs, session.Finish(ctx, s.Parent, a.Env))
	}
	return errors.Join(errs...)
}

// armRunner is one side of the pair, as the supervisor sees it: a function that
// produces a run in the directory it is handed, and leaves its result where the
// checks can read it.
func (s Spec) armRunner(arm isolate.Arm, wall func() time.Duration, into *session.Result) func(context.Context, string) (run.Meta, error) {
	return func(ctx context.Context, dir string) (run.Meta, error) {
		res, err := session.Run(ctx, s.arm(dir, arm, wall()))
		// Recorded whether or not the arm succeeded. The environment is what has
		// to be released, and an arm that failed or was cut still took a checkout.
		*into = res
		if err != nil {
			return run.Meta{}, err
		}
		return res.Meta, nil
	}
}

// baselineWall is what the baseline gets, given the sense arm that just ran.
//
// A sense arm that never reached the model reports no elapsed time, and 1.2
// times nothing is a baseline killed the instant it starts — an empty arm that
// reads in a score exactly like a baseline that had nothing to say. So a pair
// whose sense arm did not run falls back to the ceiling the credential
// preflight already budgeted for, and the pair fails its soundness checks on
// the sense arm, which is where the real fault is.
func (s Spec) baselineWall(sense session.Result) time.Duration {
	elapsed := time.Duration(sense.Meta.TookSeconds * float64(time.Second))
	if elapsed <= 0 {
		return BaselineWall(s.Wall)
	}
	return BaselineWall(elapsed)
}

// arm builds one side of the pair. Everything except the arm and its wall is
// shared by construction rather than by two call sites that must be kept the
// same: an asymmetry between the arms is the one failure that invalidates every
// number without being visible in any of them.
func (s Spec) arm(root string, arm isolate.Arm, wall time.Duration) session.Spec {
	return session.Spec{
		Root:       root,
		Arm:        arm,
		Credential: s.Credential,
		Route:      s.Route,
		Parent:     s.Parent,
		Commit:     s.Commit,
		// The wall note reaches an agent as a flag or in front of the prompt,
		// whichever this tool declares. Both are rendered here for the same
		// reason: this is the first place the arm's own number exists.
		Prompt:  s.Agent.PromptWithWall(s.Prompt, wall),
		Command: s.Command,
		// Each arm is told its OWN wall. Appended here rather than by the
		// caller because this is the first place the number exists: the
		// baseline's is not known until the sense arm has finished.
		Args:             append(slices.Clone(s.Args), s.Agent.WallNoteArgs(wall)...),
		AgentEnv:         s.AgentEnv,
		SetupTool:        s.SetupTool,
		TranscriptFormat: s.Agent.TranscriptFormat,
		MCPRegistration:  s.Agent.MCPRegistration,
		SenseBin:         s.SenseBin,
		LabBin:           s.LabBin,
		HostPath:         s.HostPath,
		Wall:             wall,
		Grace:            s.Grace,
	}
}

// check runs every proof against the pair that just ran.
func (s Spec) check(ctx context.Context, sense, baseline session.Result) (rep Report, removed error) {
	r := Report{Sense: sense, Baseline: baseline}

	// The channel probe is a throwaway: a scratch project and a scratch HOME that
	// exist only to read back what `sense setup` writes. It holds no capture, no
	// transcript and no record, so it goes whole, and it goes whether or not the
	// derivation succeeded.
	//
	// Through isolate.Cleanup rather than a bare RemoveAll, so this removal is
	// proven like every other one here. A discarded RemoveAll error is the exact
	// shape of cleanup this package refuses everywhere else.
	probeDir := filepath.Join(s.Root, "channel-probe")
	defer func() {
		removed = errors.Join(removed, isolate.Cleanup(isolate.Env{Layout: isolate.Layout{Root: probeDir}}))
	}()

	derived, err := channels.Derive(ctx, s.SenseBin, s.SetupTool, probeDir)
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
	senseWorld, baselineWorld := armWorld(sense, binary, s.ConfigDirs), armWorld(baseline, binary, s.ConfigDirs)

	r.SenseMissing = missing(treatment, channels.Absent(treatment, senseWorld))
	r.BaselineReached = channels.Absent(treatment, baselineWorld)
	r.MemoryReached = append(
		channels.Absent(contamination, senseWorld),
		channels.Absent(contamination, baselineWorld)...)

	senseSaid, err := rawTranscript(sense)
	if err != nil {
		return Report{}, err
	}
	baselineSaid, err := rawTranscript(baseline)
	if err != nil {
		return Report{}, err
	}
	r.SenseUsed = channels.UsedBy(senseSaid, tools, binary)
	r.BaselineUsed = channels.UsedBy(baselineSaid, tools, binary)

	r.SenseCalls, err = armCalls(sense, s.Agent.TranscriptFormat, tools)
	if err != nil {
		return Report{}, err
	}
	r.BaselineCalls, err = armCalls(baseline, s.Agent.TranscriptFormat, tools)
	if err != nil {
		return Report{}, err
	}

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

	r.Differences = Differences(sense.Meta, baseline.Meta, s.Agent.WallNoteFlag)
	r.Differences = append(r.Differences, s.promptDifferences(sense, baseline)...)
	return r, nil
}

// PromptDifferencesForTest exposes the prompt check to the package's own test,
// which cannot otherwise ask what it reports about a pair it did not run.
func (s Spec) PromptDifferencesForTest(sense, baseline session.Result) []string {
	return s.promptDifferences(sense, baseline)
}

// promptDifferences reports an arm that was asked something other than the
// scenario's question with its own wall note in front of it.
//
// The arguments are compared with the wall note dropped, and the prompt has to
// be compared too now that a tool with no system-prompt flag carries its note
// there: the two arms then legitimately hold different prompts, and "the arms
// differ in nothing but Sense access" would be a claim nothing checked.
//
// It reads the prompt each arm was RECORDED with rather than re-deriving it
// from the spec, because what was constructed and what was sent are the same
// thing only when nothing went wrong in between.
func (s Spec) promptDifferences(sense, baseline session.Result) []string {
	var found []string
	for _, arm := range []struct {
		name string
		res  session.Result
		wall time.Duration
	}{
		{"the sense arm", sense, s.Wall},
		{"the baseline arm", baseline, time.Duration(baseline.Meta.WallSeconds) * time.Second},
	} {
		asked, err := os.ReadFile(filepath.Join(arm.res.Env.Root, "session", "prompt.txt")) // #nosec G304 -- the run's own directory
		if err != nil {
			found = append(found, fmt.Sprintf("the prompt: %s recorded none (%v)", arm.name, err))
			continue
		}
		if want := s.Agent.PromptWithWall(s.Prompt, arm.wall); string(asked) != want {
			found = append(found, fmt.Sprintf("the prompt: %s was asked something other than "+
				"the scenario's question with its own wall note in front of it", arm.name))
		}
	}
	return found
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
func armWorld(res session.Result, binary string, configDirs []string) channels.Arm {
	return channels.Arm{
		Repo:        res.Env.Repo,
		Home:        res.Env.Home,
		ConfigDir:   res.Env.Config,
		PathValue:   res.Meta.Path,
		SenseBinary: binary,
		ConfigDirs:  configDirs,
	}
}

// Differences names every way the two arms differ other than Sense access.
//
// Read off what was recorded rather than off what was configured: the two are
// the same only when nothing went wrong, and the case worth catching is the one
// where something did.
func Differences(sense, baseline run.Meta, wallNoteFlag string) []string {
	var found []string
	compare := func(what, a, b string) {
		if a != b {
			found = append(found, fmt.Sprintf("%s: the sense arm had %q and the baseline had %q", what, a, b))
		}
	}
	compare("the agent tool", sense.Command, baseline.Command)
	// The wall note is dropped before the arguments are compared, because the
	// arms are SUPPOSED to carry different ones: each is told its own wall, and
	// the walls differ by design. Comparing it here would report every correct
	// pair as asymmetric — which is worse than useless, because the check that
	// cries wolf on every run is the check nobody reads on the run that matters.
	// The walls themselves are checked below, where the relationship is known.
	compare("the arguments",
		fmt.Sprint(withoutWallNote(sense.Args, wallNoteFlag)),
		fmt.Sprint(withoutWallNote(baseline.Args, wallNoteFlag)))
	compare("where the wall starts", sense.WallStartsAt, baseline.WallStartsAt)
	found = append(found, budgetDifference(sense, baseline)...)
	return found
}

// budgetDifference checks the one relationship the arms are allowed to differ
// in: the baseline's wall against what the sense arm spent.
//
// Equality is the wrong test and was the one here until 2026-08-17. The walls
// are not meant to match; the baseline's is meant to be 1.2 times what the
// sense arm ELAPSED. Asserting equality passed for as long as it did because
// the derivation it was guarding had quietly become the identity.
func budgetDifference(sense, baseline run.Meta) []string {
	// A sense arm that recorded no time is a pair that has already failed for a
	// louder reason, and its baseline ran on the fallback ceiling rather than on
	// a derivation. Reporting a budget mismatch on top of that would bury the
	// real fault under a second one.
	if sense.TookSeconds <= 0 {
		return nil
	}
	want := BaselineWall(time.Duration(sense.TookSeconds * float64(time.Second)))
	if got := time.Duration(baseline.WallSeconds) * time.Second; got != want {
		return []string{fmt.Sprintf("the budget: the sense arm spent %.0fs so the baseline should have had %s, and it had %s",
			sense.TookSeconds, want, got)}
	}
	return nil
}

// withoutWallNote drops a flag and the value after it, so two arms carrying
// their own wall notes can have the rest of their arguments compared.
func withoutWallNote(args []string, flag string) []string {
	if flag == "" {
		return args
	}
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == flag {
			i++ // and its value
			continue
		}
		out = append(out, args[i])
	}
	return out
}

// transcript is what an arm actually said, unnormalised. The canonical form is
// cycle 02's and is built from the same bytes.
func rawTranscript(res session.Result) ([]byte, error) {
	path := filepath.Join(res.Env.Root, "session", "raw", "stdout")
	b, err := os.ReadFile(path) // #nosec G304 -- the run's own capture
	if err != nil {
		return nil, fmt.Errorf("read the %s arm's transcript: %w", res.Meta.Arm, err)
	}
	return b, nil
}

// armCalls counts what one arm did with its tools, read through the normalizer
// its own agent tool declares.
//
// Through the normalizer rather than by searching the raw bytes, because the
// question is how many CALLS were made and not whether a name appears: a name
// appears in the tool's own registration, in the reply the tool was handed and
// in whatever the repository happens to contain.
func armCalls(res session.Result, format string, senseTools []string) (Calls, error) {
	tr, err := transcript.Read(transcript.FormatOfRun(format),
		filepath.Join(res.Env.Root, "session", "raw", "stdout"))
	if err != nil {
		return Calls{}, fmt.Errorf("read the %s arm's calls: %w", res.Meta.Arm, err)
	}
	c := Calls{Total: len(tr.Calls)}
	for _, call := range tr.Calls {
		if reachesSense(call.Name, senseTools) {
			c.Sense++
		}
	}
	return c, nil
}

// reachesSense reports whether a call by this name went to Sense.
//
// The tool names are asked of the running server rather than written down, and
// the name is matched by CONTAINMENT because every tool namespaces an MCP call
// differently and all of them were measured rather than assumed: one writes
// `mcp__sense__sense_search`, one writes the bare `sense_search`, and one
// writes `sense_sense_search`. Equality found none of the third tool's calls
// and reported an arm that used Sense three times as having used it none —
// which is the exact reading this count exists to prevent, arriving through the
// count itself.
func reachesSense(name string, senseTools []string) bool {
	if strings.Contains(name, channels.MCPPrefix) {
		return true
	}
	for _, tool := range senseTools {
		if strings.Contains(name, tool) {
			return true
		}
	}
	return false
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
