package probe_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

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

func TestTheTwoArmsShareTheirBudgetRelationship(t *testing.T) {
	// The baseline's wall is not independently chosen. Getting the relationship
	// backwards is not visible in a result, which is why it is a function with
	// a name rather than an assignment at two call sites.
	if got := probe.BaselineWall(480 * time.Second); got != 480*time.Second {
		t.Errorf("BaselineWall(480s) = %s, want the sense arm's own wall", got)
	}

	report, err := probe.Run(context.Background(), spec(t))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Read off what was recorded, not off what was configured: the two are the
	// same only when nothing went wrong.
	if report.Sense.Meta.WallSeconds != report.Baseline.Meta.WallSeconds {
		t.Errorf("the arms ran at %.0fs and %.0fs", report.Sense.Meta.WallSeconds, report.Baseline.Meta.WallSeconds)
	}
	if report.Sense.Meta.Command != report.Baseline.Meta.Command {
		t.Errorf("the arms ran %q and %q", report.Sense.Meta.Command, report.Baseline.Meta.Command)
	}
}

func TestArmsThatDifferInAnythingElseAreNamed(t *testing.T) {
	same := run.Meta{Command: "claude", Args: []string{"-p"}, WallSeconds: 480, WallStartsAt: "spawn"}

	if got := probe.Differences(same, same); len(got) != 0 {
		t.Errorf("two identical arms differ in %v", got)
	}

	for _, tc := range []struct {
		what     string
		baseline run.Meta
	}{
		{"a different agent tool", run.Meta{Command: "codex", Args: []string{"-p"}, WallSeconds: 480, WallStartsAt: "spawn"}},
		{"different arguments", run.Meta{Command: "claude", Args: []string{"-p", "--verbose"}, WallSeconds: 480, WallStartsAt: "spawn"}},
		{"a halved budget", run.Meta{Command: "claude", Args: []string{"-p"}, WallSeconds: 240, WallStartsAt: "spawn"}},
		{"a different wall start", run.Meta{Command: "claude", Args: []string{"-p"}, WallSeconds: 480, WallStartsAt: "first event"}},
	} {
		got := probe.Differences(same, tc.baseline)
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
	s.Args = []string{"-c", `cat > /dev/null
if [ -f .mcp.json ]; then echo answered; else sleep 30; fi`}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(4 * time.Second)
		cancel()
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
