package probe_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/probe"
	"github.com/luuuc/sense/lab/internal/run"
)

// parentRepo is the repository under study: one commit, something to index.
func parentRepo(t *testing.T) (dir, commit string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=bench", "GIT_AUTHOR_EMAIL=bench@example.com",
			"GIT_COMMITTER_NAME=bench", "GIT_COMMITTER_EMAIL=bench@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package app\n\nfunc New() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "first")
	return dir, git("rev-parse", "HEAD")
}

// fakeSense stands in for the product: it indexes, configures, and answers an
// MCP tool list. The tool names are read off it rather than written down, so a
// stand-in that offered different tools would change what the checks look for.
func fakeSense(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	bin := filepath.Join(t.TempDir(), "sense")
	script := `#!/bin/sh
set -e
case "$1" in
scan) mkdir -p "$3/.sense"; printf 'index' > "$3/.sense/index.db" ;;
setup)
  printf '{"mcpServers":{"sense":{"command":"` + bin + `","args":["mcp"]}}}' > .mcp.json
  printf '# guidance\n' > CLAUDE.md
  mkdir -p .claude/skills
  printf '{"hooks":{}}' > .claude/settings.json
  printf '# explore\n' > .claude/skills/sense-explore.md
  ;;
mcp)
  while IFS= read -r line; do
    case "$line" in
      *'"tools/list"'*) printf '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"sense_graph"},{"name":"sense_blast"}]}}\n' ;;
      *'"id"'*) printf '{"jsonrpc":"2.0","id":1,"result":{"symbols":[]}}\n' ;;
    esac
  done
  ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// labBinary builds this binary so the capture shim can be spawned the way a
// run's registration spawns it.
func labBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "sense-lab")
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/luuuc/sense/lab/cmd/sense-lab").CombinedOutput(); err != nil {
		t.Fatalf("build sense-lab: %v: %s", err, out)
	}
	return bin
}

// agentThatUsesSense stands in for an agent CLI that reaches the MCP server,
// records the call in its own output the way Claude Code does, and answers.
const agentThatUsesSense = `cat > /dev/null
if [ -f .mcp.json ]; then
  flat=$(tr -d ' \n' < .mcp.json)
  server=$(printf '%s' "$flat" | sed 's/.*"command":"\([^"]*\)".*/\1/')
  args=$(printf '%s' "$flat" | sed 's/.*"args":\[\([^]]*\)\].*/\1/' | tr -d '"' | tr ',' ' ')
  printf '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"sense_graph"}}\n' | $server $args > /dev/null
  printf '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__sense__sense_graph"}]}}\n'
fi
printf '{"type":"result","result":"Category has three dependents"}\n'
`

// agentThatCannotUseSense is the same agent with nothing to reach.
const agentThatCannotUseSense = `cat > /dev/null
printf '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Grep"},{"type":"tool_use","name":"Bash","input":{"command":"grep -rn Category ."}}]}}\n'
printf '{"type":"result","result":"Category is referenced in three files"}\n'
`

func spec(t *testing.T) probe.Spec {
	t.Helper()
	parent, commit := parentRepo(t)
	return probe.Spec{
		Root:       filepath.Join(t.TempDir(), "cell"),
		Parent:     parent,
		Commit:     commit,
		Prompt:     "what depends on Category?",
		Command:    "/bin/sh",
		Args:       []string{"-c", agentThatUsesSense + agentThatCannotUseSense},
		SetupTool:  "claude-code",
		ConfigDirs: []string{".claude"},
		SenseBin:   fakeSense(t),
		LabBin:     labBinary(t),
		HostPath:   os.Getenv("PATH"),
		Wall:       60 * time.Second,
		Grace:      200 * time.Millisecond,
	}
}

// bothArmsRan is the precondition every check below rests on. An arm that never
// started leaves no transcript, and a check that finds nothing in no transcript
// reports the arm CLEAN — which is how a baseline killed before it could fork
// passed three contamination checks at once.
func bothArmsRan(t *testing.T, r probe.Report) {
	t.Helper()
	for _, a := range []struct {
		name string
		meta run.Meta
	}{{"sense", r.Sense.Meta}, {"baseline", r.Baseline.Meta}} {
		if a.meta.Outcome != run.Completed {
			t.Fatalf("the %s arm did not run: %s after %.3fs on a %vs wall; every check here would read it as clean",
				a.name, a.meta.Outcome, a.meta.TookSeconds, a.meta.WallSeconds)
		}
	}
}

func TestBothArmsRunAndTheDifferenceIsOnlySenseAccess(t *testing.T) {
	s := spec(t)

	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if report.Sense.Meta.Outcome != run.Completed || report.Baseline.Meta.Outcome != run.Completed {
		t.Fatalf("outcomes: sense %s, baseline %s", report.Sense.Meta.Outcome, report.Baseline.Meta.Outcome)
	}
	if !report.Sound() {
		t.Fatalf("the pair is not a measurement: %+v", report)
	}
	if len(report.Differences) != 0 {
		t.Errorf("the arms differ in %v", report.Differences)
	}
}

func TestTheSenseArmHasEveryRouteItWasSupposedToHave(t *testing.T) {
	// A sense arm missing one route is a weakened treatment, and nothing in a
	// score would show it.
	report, err := probe.Run(context.Background(), spec(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	bothArmsRan(t, report)

	if len(report.SenseMissing) != 0 {
		t.Errorf("the sense arm is missing %v", report.SenseMissing)
	}
	if report.Frames == 0 {
		t.Error("the sense arm's capture is empty; either Sense was never reached or capture never worked")
	}
	if len(report.SenseUsed) == 0 {
		t.Error("the sense arm's transcript shows no Sense tool call")
	}
}

func TestTheBaselineArmReachesNoRouteAndUsesNone(t *testing.T) {
	// The claim the whole corpus rests on. Configuration says what was set up;
	// the transcript says what was used; neither alone is the claim.
	report, err := probe.Run(context.Background(), spec(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	bothArmsRan(t, report)

	if len(report.BaselineReached) != 0 {
		t.Errorf("the baseline arm reaches %v", report.BaselineReached)
	}
	if len(report.BaselineUsed) != 0 {
		t.Errorf("the baseline arm's transcript shows %v", report.BaselineUsed)
	}
	if report.BaselineCaptured {
		t.Error("the baseline arm left a capture file; it has no MCP server, so its absence is the signal")
	}
}

func TestNeitherArmCanReadPersistedMemory(t *testing.T) {
	// Persisted memory is not a treatment. A sense arm that reads a memory
	// directory is measuring a previous session, which is worse than an
	// asymmetry because it moves both arms in the same direction.
	report, err := probe.Run(context.Background(), spec(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	bothArmsRan(t, report)

	if len(report.MemoryReached) != 0 {
		t.Errorf("persisted memory was reachable: %v", report.MemoryReached)
	}
}

func TestABaselineThatReachedSenseIsCaught(t *testing.T) {
	// The check that catches what configuration checks cannot: an arm that
	// found the binary some other way. There is no configuration trace at all,
	// and only the transcript shows it.
	s := spec(t)
	s.Args = []string{"-c", `cat > /dev/null
printf '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"sense graph -symbol Category"}}]}}\n'
` + agentThatUsesSense}

	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	bothArmsRan(t, report)

	if len(report.BaselineUsed) == 0 {
		t.Fatal("a baseline arm that invoked the sense binary was reported clean")
	}
	if report.Sound() {
		t.Error("the pair reports itself sound although the baseline arm reached Sense")
	}
}

func TestABaselineThatNamedASenseToolIsCaught(t *testing.T) {
	s := spec(t)
	s.Args = []string{"-c", `cat > /dev/null
printf '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__sense__sense_graph"}]}}\n'
` + agentThatUsesSense}

	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	bothArmsRan(t, report)

	if len(report.BaselineUsed) == 0 {
		t.Fatal("a baseline arm that called a Sense tool was reported clean")
	}
}

func TestABaselineWorktreeCarryingARouteIsCaught(t *testing.T) {
	// The other half. A transcript that happens not to use Sense in one run
	// says nothing about whether the arm could have.
	s := spec(t)
	// A parent repository that already carries the registration, so the
	// baseline arm's worktree inherits it.
	if err := os.WriteFile(filepath.Join(s.Parent, ".mcp.json"), []byte(`{"mcpServers":{"sense":{}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	commitAll(t, s.Parent)
	s.Commit = head(t, s.Parent)

	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(report.BaselineReached) == 0 {
		t.Fatal("a baseline worktree carrying the MCP registration was reported clean")
	}
	if report.Sound() {
		t.Error("the pair reports itself sound although the baseline arm carried a route")
	}
}

func TestTheBaselineIsPairedAgainstWhatTheSenseArmSpentNotWhatItWasAllowed(t *testing.T) {
	// The banked pairing, which is the number this has to keep reproducing: a
	// sense arm ALLOWED 480s that SPENT 404s left its baseline 485s. Deriving
	// from the allowance instead gives 576s, and the gap is clock handed to the
	// baseline for free, shrinking every margin in the same direction.
	if got := probe.BaselineWall(404 * time.Second); got != 485*time.Second {
		t.Errorf("BaselineWall(404s spent) = %s, want 485s", got)
	}
	// The failure this replaces: for as long as the derivation was the
	// identity, a sense arm's allowance and its baseline's budget were equal.
	if got := probe.BaselineWall(480 * time.Second); got == 480*time.Second {
		t.Error("BaselineWall is returning its argument, which is the defect the replay found")
	}

	// The one test that pays for real elapsed time. The stand-ins answer in
	// under half a second, which derives a wall the one-second floor would have
	// produced anyway, so without this beat the test cannot tell a derivation
	// from a hardcoded constant.
	s := spec(t)
	s.Args = []string{"-c", `cat > /dev/null
if [ -f .mcp.json ]; then sleep 2; fi
` + agentThatUsesSense + agentThatCannotUseSense}

	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	bothArmsRan(t, report)

	// Read off what was recorded, not off what was configured: the two are the
	// same only when nothing went wrong.
	spent := time.Duration(report.Sense.Meta.TookSeconds * float64(time.Second))
	want := probe.BaselineWall(spent)
	// Without this the test goes vacuous on a fast machine: both sides collapse
	// onto the floor and an implementation returning a constant second passes.
	if want <= time.Second {
		t.Fatalf("the sense arm spent only %s, so its baseline got the floor and this proves nothing", spent)
	}
	if got := time.Duration(report.Baseline.Meta.WallSeconds) * time.Second; got != want {
		t.Errorf("the sense arm spent %s so the baseline should have had %s; it had %s", spent, want, got)
	}
	if report.Sense.Meta.Command != report.Baseline.Meta.Command {
		t.Errorf("the arms ran %q and %q", report.Sense.Meta.Command, report.Baseline.Meta.Command)
	}
}

// TestEachArmIsToldTheWallThatWillActuallyCutIt is the check whose absence cost
// a paid pair.
//
// Both arms of the 2026-08-17 mastodon replay ran to their ceiling and were cut
// mid-answer, because nothing told them what the ceiling was. Every hermetic
// test was green through it: they all asked what an arm could REACH, and none
// asked what it was TOLD. The number in the note has to be the number the
// supervisor enforces, or the note is a lie that costs the arm its answer.
func TestEachArmIsToldTheWallThatWillActuallyCutIt(t *testing.T) {
	s := spec(t)
	s.Agent = catalog.Agent{
		WallNoteFlag:     "--append-system-prompt",
		WallNoteDelivery: catalog.WallNoteFlagged,
		WallNote:         "stopped hard after {{seconds}} seconds of real time",
	}

	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, arm := range []struct {
		name string
		meta run.Meta
	}{
		{"sense", report.Sense.Meta},
		{"baseline", report.Baseline.Meta},
	} {
		told, ok := secondsInWallNote(arm.meta.Args, "--append-system-prompt")
		if !ok {
			t.Errorf("the %s arm was never told its wall; it ran with %v", arm.name, arm.meta.Args)
			continue
		}
		if enforced := int(arm.meta.WallSeconds); told != enforced {
			t.Errorf("the %s arm was told %ds and cut at %ds", arm.name, told, enforced)
		}
	}

	// And the two notes differ, because the two walls do. An implementation
	// that told both arms the sense arm's number would pass every check above.
	senseNote, _ := secondsInWallNote(report.Sense.Meta.Args, "--append-system-prompt")
	baseNote, _ := secondsInWallNote(report.Baseline.Meta.Args, "--append-system-prompt")
	if senseNote == baseNote {
		t.Errorf("both arms were told %ds, but their walls are not the same number", senseNote)
	}
}

// secondsInWallNote reads back the number an arm was actually told.
func secondsInWallNote(args []string, flag string) (int, bool) {
	for i, a := range args {
		if a != flag || i+1 >= len(args) {
			continue
		}
		for _, field := range strings.Fields(args[i+1]) {
			if n, err := strconv.Atoi(field); err == nil {
				return n, true
			}
		}
	}
	return 0, false
}

// TestAPairCarryingItsOwnWallNotesIsStillSound guards the fix from its own
// side effect: the arms are now SUPPOSED to differ in one argument, and a
// difference check that did not know it would report every correct pair as
// asymmetric.
func TestAPairCarryingItsOwnWallNotesIsStillSound(t *testing.T) {
	s := spec(t)
	s.Agent = catalog.Agent{WallNoteFlag: "--append-system-prompt", WallNote: "{{seconds}} seconds"}

	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.Differences) != 0 {
		t.Errorf("a correct pair was reported as differing: %v", report.Differences)
	}
	if !report.Sound() {
		t.Error("a pair whose only asymmetry is each arm's own wall note is not a measurement")
	}
}

func TestArmsThatDifferInAnythingElseAreNamed(t *testing.T) {
	// TookSeconds is what the baseline's budget derives from, so a fixture
	// without it cannot exercise the budget check at all. 400s spent pairs with
	// a 480s baseline.
	same := run.Meta{Command: "claude", Args: []string{"-p"}, WallSeconds: 480, TookSeconds: 400, WallStartsAt: "spawn"}

	if got := probe.Differences(same, same, ""); len(got) != 0 {
		t.Errorf("two identical arms differ in %v", got)
	}

	for _, tc := range []struct {
		what     string
		baseline run.Meta
	}{
		{"a different agent tool", run.Meta{Command: "codex", Args: []string{"-p"}, WallSeconds: 480, WallStartsAt: "spawn"}},
		{"different arguments", run.Meta{Command: "claude", Args: []string{"-p", "--verbose"}, WallSeconds: 480, WallStartsAt: "spawn"}},
		{"a halved budget", run.Meta{Command: "claude", Args: []string{"-p"}, WallSeconds: 240, WallStartsAt: "spawn"}},
		// The defect itself: a baseline handed 1.2x the sense arm's ALLOWANCE
		// (480s) rather than 1.2x what it SPENT (400s). Ninety-six seconds of
		// free clock, in the baseline's favour, invisible in any score.
		{"a budget derived from the allowance instead of the spend", run.Meta{Command: "claude", Args: []string{"-p"}, WallSeconds: 576, WallStartsAt: "spawn"}},
		{"a different wall start", run.Meta{Command: "claude", Args: []string{"-p"}, WallSeconds: 480, WallStartsAt: "first event"}},
	} {
		got := probe.Differences(same, tc.baseline, "")
		if len(got) == 0 {
			t.Errorf("%s went unreported", tc.what)
		}
	}
}

func TestAnInterruptionAfterTheFirstArmRefusesToPair(t *testing.T) {
	// The half-pair. The finished arm is paid for and can never be paired,
	// because the baseline's budget derives from the sense arm it ran with.
	s := spec(t)
	// The sense arm answers and stops; the baseline arm is still running when
	// the interruption lands, which is the moment that burns a paid-for arm.
	//
	// The cancellation waits for the baseline to SAY it started rather than for
	// a stopwatch to run down. Guessing at the schedule is what made this test
	// need ever-longer sleeps every time the timing underneath it moved, and a
	// guess that lands early tests nothing while a guess that lands late is a
	// flake nobody can reproduce.
	//
	// The sense arm still takes a beat, because the window to cancel INSIDE the
	// baseline is that arm's derived wall, and an instant sense arm leaves only
	// the one-second floor to aim at.
	started := filepath.Join(t.TempDir(), "baseline-started")
	s.Args = []string{"-c", `cat > /dev/null
if [ -f .mcp.json ]; then sleep 2; echo answered; else touch ` + started + `; sleep 30; fi`}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		// Bounded, so a baseline that never starts fails the assertion below
		// rather than hanging the package.
		for range 1000 {
			if _, err := os.Stat(started); err == nil {
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()

	_, err := probe.Run(ctx, s)

	if err == nil {
		t.Fatal("Run returned a report from an interrupted cell")
	}
	if !strings.Contains(err.Error(), "sense") {
		t.Errorf("error = %v, want it to name the burned arm", err)
	}
	// The record is on disk, so a later pass refuses the burned run rather than
	// quietly pairing it with something else.
	if _, statErr := os.Stat(filepath.Join(s.Root, "cell-meta.json")); statErr != nil {
		t.Errorf("no cell record was written: %v", statErr)
	}
	// And the checkouts still go back. This is the case that produced three
	// stale registrations in one afternoon: an interrupted cell is exactly the
	// one nobody cleans up, and git then refuses the retry at that path.
	if list := worktreeList(t, s.Parent); strings.Contains(list, s.Root) {
		t.Errorf("an interrupted cell left a registration behind:\n%s", list)
	}
}

// The channel probe is a scratch project and a scratch HOME, built to read back
// what `sense setup` writes. It holds no evidence and must not survive the cell.
func TestTheChannelProbeDirectoryDoesNotSurviveTheCell(t *testing.T) {
	s := spec(t)

	if _, err := probe.Run(context.Background(), s); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(filepath.Join(s.Root, "channel-probe")); !os.IsNotExist(err) {
		t.Errorf("the channel probe directory is still there: %v", err)
	}
}

// worktreeList is what the parent repository still names.
func worktreeList(t *testing.T, parent string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", parent, "worktree", "list").CombinedOutput()
	if err != nil {
		t.Fatalf("git worktree list: %v: %s", err, out)
	}
	return string(out)
}

func TestASenseArmThatCannotBePreparedStopsTheCell(t *testing.T) {
	s := spec(t)
	s.SenseBin = filepath.Join(t.TempDir(), "not-a-binary")

	if _, err := probe.Run(context.Background(), s); err == nil {
		t.Fatal("Run reported success although the sense arm could not be prepared")
	}

	// A cell that could not prepare an arm has still taken a checkout, and it
	// must give it back: a leaked registration makes git refuse the retry.
	if list := worktreeList(t, s.Parent); strings.Contains(list, s.Root) {
		t.Errorf("a refused cell left a registration behind:\n%s", list)
	}
}

func TestAServerThatOffersNoToolsIsRefused(t *testing.T) {
	// An empty tool list makes every transcript check pass, which is the shape
	// of a leak that reads as a clean bill of health.
	s := spec(t)
	s.SenseBin = senseWithNoTools(t)

	_, err := probe.Run(context.Background(), s)

	if err == nil {
		t.Fatal("Run accepted a server that offered no tools")
	}
	if !strings.Contains(err.Error(), "no tools") {
		t.Errorf("error = %v, want it to name the empty tool list", err)
	}
}

func TestAServerThatNeverAnswersTheToolListIsRefused(t *testing.T) {
	s := spec(t)
	s.SenseBin = senseThatIgnoresTheHandshake(t)

	if _, err := probe.Run(context.Background(), s); err == nil {
		t.Fatal("Run accepted a server that never answered the tool list")
	}
}

// senseWithNoTools answers the handshake with an empty tool list.
func senseWithNoTools(t *testing.T) string {
	return senseServing(t, `printf '{"jsonrpc":"2.0","id":2,"result":{"tools":[]}}\n'`)
}

// senseThatIgnoresTheHandshake answers nothing at all.
func senseThatIgnoresTheHandshake(t *testing.T) string {
	return senseServing(t, `:`)
}

func senseServing(t *testing.T, reply string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "sense")
	script := `#!/bin/sh
set -e
case "$1" in
scan) mkdir -p "$3/.sense"; printf 'index' > "$3/.sense/index.db" ;;
setup) printf '{"mcpServers":{"sense":{"command":"` + bin + `","args":["mcp"]}}}' > .mcp.json ;;
mcp) ` + reply + ` ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func commitAll(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", "carry the registration"}} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=bench", "GIT_AUTHOR_EMAIL=bench@example.com",
			"GIT_COMMITTER_NAME=bench", "GIT_COMMITTER_EMAIL=bench@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

func head(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestASenseArmThatNeverTouchedSenseIsNotAMeasurement(t *testing.T) {
	// An arm that had Sense and never used it is a different failure from one
	// that could not reach it, and a score cannot tell them apart. Zero frames
	// on a sense arm means either Sense was never reached or capture never
	// worked, and the run says so rather than reporting a number.
	s := spec(t)
	s.Args = []string{"-c", agentThatCannotUseSense}

	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	bothArmsRan(t, report)

	if report.Frames != 0 {
		t.Fatalf("Frames = %d, want none from an agent that never called Sense", report.Frames)
	}
	if len(report.SenseUsed) != 0 {
		t.Errorf("SenseUsed = %v", report.SenseUsed)
	}
	if report.Sound() {
		t.Error("the pair reports itself sound although the sense arm never touched Sense")
	}
}

func TestARouteTheSenseArmDoesNotHaveIsNamed(t *testing.T) {
	// The product gained a route between the arm being prepared and the routes
	// being derived. That is the shape of a sense arm running with a weakened
	// treatment, and nothing in a score would show it.
	s := spec(t)
	s.SenseBin = senseThatGainsARoute(t)

	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(report.SenseMissing) == 0 {
		t.Fatal("a sense arm missing a route was reported complete")
	}
	if report.Sound() {
		t.Error("the pair reports itself sound although the sense arm is missing a route")
	}
}

func TestASetupThatWritesNothingAtDerivationTimeIsRefused(t *testing.T) {
	// An empty derived list makes every absence check pass, which is the shape
	// of a leak that reads as a clean bill of health.
	s := spec(t)
	s.SenseBin = senseThatStopsConfiguring(t)

	if _, err := probe.Run(context.Background(), s); err == nil {
		t.Fatal("Run accepted a derivation that found no routes")
	}
}

// senseThatGainsARoute writes one more file the second time it configures a
// project, which is when the routes are derived.
func senseThatGainsARoute(t *testing.T) string {
	return senseCounting(t, `printf '{}' > .cursor-route.json`, "")
}

// senseThatStopsConfiguring writes nothing the second time.
func senseThatStopsConfiguring(t *testing.T) string {
	return senseCounting(t, "", "skip")
}

// senseCounting is a stand-in whose setup behaves differently on its second
// call: the first is the sense arm's, the second is the derivation's.
func senseCounting(t *testing.T, extra, mode string) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "sense")
	base := `printf '{"mcpServers":{"sense":{"command":"` + bin + `","args":["mcp"]}}}' > .mcp.json
  printf '# guidance\n' > CLAUDE.md`
	second := base + "\n  " + extra
	if mode == "skip" {
		second = ":"
	}
	script := `#!/bin/sh
set -e
count=` + filepath.Join(dir, "calls") + `
case "$1" in
scan) mkdir -p "$3/.sense"; printf 'index' > "$3/.sense/index.db" ;;
setup)
  printf 'x' >> "$count"
  if [ "$(wc -c < "$count" | tr -d ' ')" -eq 1 ]; then
    ` + base + `
  else
    ` + second + `
  fi
  ;;
mcp)
  while IFS= read -r line; do
    case "$line" in
      *'"tools/list"'*) printf '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"sense_graph"}]}}\n' ;;
      *'"id"'*) printf '{"jsonrpc":"2.0","id":1,"result":{}}\n' ;;
    esac
  done
  ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// A cell whose agent is not identified cannot have its contamination checked:
// an empty config dir joins to the disposable HOME itself, which exists for
// both arms, so every arm would read as contaminated. Refused before anything
// spawns, because a check that cannot run must not run wrong.
func TestACellWhoseAgentIsNotIdentifiedIsRefusedBeforeSpawning(t *testing.T) {
	for _, tc := range []struct {
		name, setup string
		dirs        []string
	}{
		{"no setup tool", "", []string{".claude"}},
		{"no config dirs", "claude-code", nil},
		{"neither", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := spec(t)
			s.SetupTool, s.ConfigDirs = tc.setup, tc.dirs
			if _, err := probe.Run(context.Background(), s); err == nil {
				t.Fatal("the cell ran with an unidentified agent")
			}
			if _, err := os.Stat(s.Root); err == nil {
				t.Error("a cell directory was created for a cell that cannot be checked")
			}
		})
	}
}

func TestABaselineIsNeverGivenAWallItCannotStartIn(t *testing.T) {
	// Deriving from elapsed time has a failure mode that deriving from a budget
	// did not: a sense arm that returns almost at once leaves its baseline
	// almost nothing, and 1.2 times a few milliseconds ROUNDS TO ZERO. The
	// baseline is then killed before it runs, and an arm that never ran is
	// indistinguishable in a score from one that had nothing to say.
	//
	// It is not hypothetical. A session that cannot authenticate exits in about
	// a second having done nothing.
	for _, spent := range []time.Duration{
		0, time.Millisecond, 10 * time.Millisecond, 100 * time.Millisecond, time.Second,
	} {
		if got := probe.BaselineWall(spent); got < time.Second {
			t.Errorf("a sense arm that spent %s leaves its baseline %s, which it cannot start in", spent, got)
		}
	}

	// And the floor does not disturb a real pairing.
	if got := probe.BaselineWall(404 * time.Second); got != 485*time.Second {
		t.Errorf("BaselineWall(404s) = %s, want the banked 485s", got)
	}
}

// A tool with no system-prompt flag carries its wall note in the prompt, so the
// two arms hold different prompts by design — and "the arms differ in nothing
// but Sense access" is then a claim about something nothing was checking.
func TestEachArmIsAskedTheScenariosQuestionWithItsOwnWallNoteInFront(t *testing.T) {
	s := spec(t)
	s.Agent = catalog.Agent{
		WallNoteDelivery: catalog.WallNotePrompted,
		WallNote:         "stopped hard after {{seconds}} seconds of real time",
	}

	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, arm := range []struct {
		name string
		root string
		wall float64
	}{
		{"sense", report.Sense.Env.Root, report.Sense.Meta.WallSeconds},
		{"baseline", report.Baseline.Env.Root, report.Baseline.Meta.WallSeconds},
	} {
		asked, err := os.ReadFile(filepath.Join(arm.root, "session", "prompt.txt"))
		if err != nil {
			t.Fatalf("read the %s arm's prompt: %v", arm.name, err)
		}
		want := fmt.Sprintf("stopped hard after %d seconds of real time", int(arm.wall))
		if !strings.HasPrefix(string(asked), want) {
			t.Errorf("the %s arm was not told its own wall in front of the prompt:\n%s", arm.name, asked)
		}
		if !strings.Contains(string(asked), s.Prompt) {
			t.Errorf("the %s arm was not asked the scenario's question:\n%s", arm.name, asked)
		}
	}
	for _, d := range report.Differences {
		if strings.Contains(d, "the prompt") {
			t.Errorf("a correct pair was reported as asking different questions: %s", d)
		}
	}
}

// And the check has to fire when an arm really was asked something else, or it
// is a check that passes on every run including the one that matters.
func TestAnArmAskedADifferentQuestionIsReportedAsADifference(t *testing.T) {
	s := spec(t)
	report, err := probe.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	tampered := s
	tampered.Prompt = "something else entirely"

	if got := tampered.PromptDifferencesForTest(report.Sense, report.Baseline); len(got) != 2 {
		t.Errorf("differences = %v, want both arms reported as asked something else", got)
	}
}
