package session_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/isolate"
	"github.com/luuuc/sense/lab/internal/run"
	"github.com/luuuc/sense/lab/internal/session"
)

// parentRepo is a one-commit repository to take a worktree from.
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
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "first")
	return dir, git("rev-parse", "HEAD")
}

// fakeSense stands in for the product binary: `scan` builds an index and
// `setup` writes the registration a real setup would.
func fakeSense(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	const script = `#!/bin/sh
set -e
case "$1" in
scan)  mkdir -p "$3/.sense"; printf 'index' > "$3/.sense/index.db" ;;
setup)
  printf '{"mcpServers":{"sense":{"command":"sense","args":["mcp"],"type":"stdio"}}}' > .mcp.json
  printf '# guidance\n' > CLAUDE.md
  ;;
esac
`
	path := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// agent is a stand-in for the agent CLI: it says something and stops.
func agent(t *testing.T, says string) (name string, args []string) {
	t.Helper()
	return "/bin/sh", []string{"-c", "cat > /dev/null; echo " + says}
}

func spec(t *testing.T, arm isolate.Arm) session.Spec {
	t.Helper()
	parent, commit := parentRepo(t)
	name, args := agent(t, "answered")
	return session.Spec{
		Root:     filepath.Join(t.TempDir(), "run"),
		Arm:      arm,
		Parent:   parent,
		Commit:   commit,
		Prompt:   "list the dependents of Category",
		Command:  name,
		Args:     args,
		SenseBin: fakeSense(t),
		LabBin:   "/opt/sense-lab",
		HostPath: os.Getenv("PATH"),
		Wall:     30 * time.Second,
		Grace:    200 * time.Millisecond,
	}
}

func TestASenseArmLeavesTheRunTreeItsResultWillBeReadFrom(t *testing.T) {
	s := spec(t, isolate.Sense)

	res, err := session.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.Meta.Outcome != run.Completed {
		t.Fatalf("outcome = %s", res.Meta.Outcome)
	}
	// raw/ is what cycle 02's canonical transcript reads. Nothing here
	// normalises it.
	for _, path := range []string{
		filepath.Join(res.Env.Root, "session", "raw", "stdout"),
		filepath.Join(res.Env.Root, "session", "run-meta.json"),
		filepath.Join(res.Env.Repo, "app.go"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
	if said := readFile(t, filepath.Join(res.Env.Root, "session", "raw", "stdout")); !strings.Contains(said, "answered") {
		t.Errorf("stdout = %q", said)
	}
}

func TestTheWorktreeIsAtThePinnedCommitAndTheSessionRunsInIt(t *testing.T) {
	s := spec(t, isolate.Baseline)

	res, err := session.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	at := strings.TrimSpace(gitOut(t, res.Env.Repo, "rev-parse", "HEAD"))
	if at != s.Commit {
		t.Errorf("the session ran against %.12s, want the pinned %.12s", at, s.Commit)
	}
}

func TestTheSenseArmsMcpRegistrationPointsAtTheCapture(t *testing.T) {
	res, err := session.Run(context.Background(), spec(t, isolate.Sense))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	entry := mcpEntry(t, filepath.Join(res.Env.Repo, ".mcp.json"))
	if entry["command"] != "/opt/sense-lab" {
		t.Errorf("the registration spawns %v, want the capture shim", entry["command"])
	}
	args := strings.Join(strs(entry["args"]), " ")
	if !strings.Contains(args, session.LogPath(res.Env)) {
		t.Errorf("the registration logs to %q, want %q", args, session.LogPath(res.Env))
	}
	// The server actually spawned is still the one the product named. The bench
	// does not hand-roll a registration, because then it measures a
	// configuration no user has.
	if !strings.HasSuffix(args, "-- sense mcp") {
		t.Errorf("the registration ends %q, want the product's own command", args)
	}
}

func TestARegistrationKeyTheBenchDoesNotKnowAboutSurvives(t *testing.T) {
	// The registration is rewritten as a generic document. A round trip through
	// a type the bench declares would drop whatever `sense setup` adds next,
	// and the sense arm would quietly lose part of its configured surface.
	res, err := session.Run(context.Background(), spec(t, isolate.Sense))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if entry := mcpEntry(t, filepath.Join(res.Env.Repo, ".mcp.json")); entry["type"] != "stdio" {
		t.Errorf("the registration lost its type key: %v", entry)
	}
}

func TestTheBaselineArmsRepositoryIsNeverConfigured(t *testing.T) {
	res, err := session.Run(context.Background(), spec(t, isolate.Baseline))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, rel := range []string{".mcp.json", "CLAUDE.md", ".sense"} {
		if _, err := os.Stat(filepath.Join(res.Env.Repo, rel)); !os.IsNotExist(err) {
			t.Errorf("the baseline arm's worktree holds %s", rel)
		}
	}
	if len(res.Meta.SenseSetup) != 0 {
		t.Errorf("the baseline arm recorded a setup: %v", res.Meta.SenseSetup)
	}
}

func TestABaselineArmWithNoSenseBinaryLeavesThePathAlone(t *testing.T) {
	// There is no Sense binary to keep off the baseline's PATH, and the empty
	// name must not be read as the working directory: that is the repository
	// under study, and putting it on PATH would let a scenario's own files be
	// executed.
	s := spec(t, isolate.Baseline)
	s.SenseBin = ""

	res, err := session.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	for _, dir := range filepath.SplitList(res.Meta.Path) {
		if dir == "." || dir == "" {
			t.Errorf("the session's PATH is %q, which searches the repository under study", res.Meta.Path)
		}
	}
	if res.Meta.Path == "" {
		t.Error("the session was given no PATH at all")
	}
}

func TestTheBaselineArmHasNoCaptureAtAll(t *testing.T) {
	// It has no MCP server to capture. An empty sense-io.jsonl would be
	// indistinguishable from a capture failure; its absence is the signal.
	res, err := session.Run(context.Background(), spec(t, isolate.Baseline))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Stat(session.LogPath(res.Env)); !os.IsNotExist(err) {
		t.Errorf("the baseline arm left a capture file: %v", err)
	}
	if _, present, err := session.Capture(res.Env); err != nil || present {
		t.Errorf("Capture reported present=%v err=%v for the baseline arm", present, err)
	}
}

func TestAKilledSessionStillLeavesWhatItSaidBeforeTheKill(t *testing.T) {
	s := spec(t, isolate.Baseline)
	s.Args = []string{"-c", "echo the answer so far; sleep 30"}
	s.Wall = 400 * time.Millisecond

	res, err := session.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if res.Meta.Outcome != run.CannotFinishAtBudget {
		t.Errorf("outcome = %s, want %s", res.Meta.Outcome, run.CannotFinishAtBudget)
	}
	said := readFile(t, filepath.Join(res.Env.Root, "session", "raw", "stdout"))
	if !strings.Contains(said, "the answer so far") {
		t.Errorf("stdout = %q, want everything captured up to the kill", said)
	}
}

func TestTheCaptureStatusIsReadBackWhenThereIsOne(t *testing.T) {
	res, err := session.Run(context.Background(), spec(t, isolate.Sense))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// The stand-in agent never launches the MCP server, so the shim never runs.
	// Written by hand here, because what is under test is reading it back.
	writeJSON(t, filepath.Join(res.Env.Artifacts, "capture.json"),
		map[string]any{"capturing": true, "frames": 12, "dropped": 0})

	status, present, err := session.Capture(res.Env)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if !present || !status.Complete() || status.Frames != 12 {
		t.Errorf("Capture = %+v present=%v, want a complete capture of 12 frames", status, present)
	}
}

func TestADroppedFrameMakesTheCaptureIncomplete(t *testing.T) {
	// A partial file that nothing marks as partial reads as "Sense was barely
	// called", which is a finding about the product rather than about the disk.
	res, err := session.Run(context.Background(), spec(t, isolate.Sense))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	writeJSON(t, filepath.Join(res.Env.Artifacts, "capture.json"),
		map[string]any{"capturing": true, "frames": 12, "dropped": 4})

	status, _, err := session.Capture(res.Env)
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if status.Complete() {
		t.Errorf("Capture = %+v reports a complete record although frames were dropped", status)
	}
}

func TestAnUnreadableCaptureStatusIsReported(t *testing.T) {
	res, err := session.Run(context.Background(), spec(t, isolate.Sense))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := os.WriteFile(filepath.Join(res.Env.Artifacts, "capture.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, err := session.Capture(res.Env); err == nil {
		t.Fatal("Capture accepted an unreadable status")
	}
}

func TestASenseArmWithNoRegistrationIsRefusedRatherThanRunBlind(t *testing.T) {
	// A sense arm that ran without Sense configured is the worst possible
	// result: it looks like a measurement and it is not one.
	s := spec(t, isolate.Sense)
	s.SenseBin = fakeSenseWriting(t, `printf '{"mcpServers":{}}' > .mcp.json`)

	_, err := session.Run(context.Background(), s)

	if err == nil {
		t.Fatal("Run accepted a sense arm with no Sense server registered")
	}
	if !strings.Contains(err.Error(), "no sense server") {
		t.Errorf("error = %v, want it to name the missing registration", err)
	}
}

func TestARegistrationWithNoCommandIsRefused(t *testing.T) {
	s := spec(t, isolate.Sense)
	s.SenseBin = fakeSenseWriting(t, `printf '{"mcpServers":{"sense":{}}}' > .mcp.json`)

	if _, err := session.Run(context.Background(), s); err == nil {
		t.Fatal("Run accepted a registration with no command")
	}
}

func TestAnUnreadableRegistrationIsReported(t *testing.T) {
	s := spec(t, isolate.Sense)
	s.SenseBin = fakeSenseWriting(t, `printf 'not json' > .mcp.json`)

	if _, err := session.Run(context.Background(), s); err == nil {
		t.Fatal("Run accepted an unreadable registration")
	}
}

func TestAMissingRegistrationIsReported(t *testing.T) {
	s := spec(t, isolate.Sense)
	s.SenseBin = fakeSenseWriting(t, `printf '# guidance\n' > CLAUDE.md`)

	if _, err := session.Run(context.Background(), s); err == nil {
		t.Fatal("Run accepted a sense arm with no registration file at all")
	}
}

func TestARunRootThatAlreadyExistsIsRefused(t *testing.T) {
	s := spec(t, isolate.Baseline)
	s.Root = t.TempDir()

	if _, err := session.Run(context.Background(), s); err == nil {
		t.Fatal("Run reused an existing run directory")
	}
}

func TestACommitTheParentDoesNotHaveIsRefused(t *testing.T) {
	s := spec(t, isolate.Baseline)
	s.Commit = "0000000000000000000000000000000000000000"

	if _, err := session.Run(context.Background(), s); err == nil {
		t.Fatal("Run accepted a commit the parent does not have")
	}
}

func TestAFailingSubjectPreparationStopsBeforeTheAgentSpawns(t *testing.T) {
	s := spec(t, isolate.Sense)
	s.SenseBin = filepath.Join(t.TempDir(), "not-a-binary")

	if _, err := session.Run(context.Background(), s); err == nil {
		t.Fatal("Run spawned the agent although the subject could not be prepared")
	}
}

// The environment comes back WITH the error, and that contract is load-bearing:
// the caller releases what the failed attempt took, and it cannot do that from a
// zero Result. Stated here, where it is defined, rather than only through the
// cell-level test that depends on it.
func TestAFailedArmStillHandsBackTheEnvironmentItTook(t *testing.T) {
	s := spec(t, isolate.Sense)
	s.SenseBin = fakeSenseWriting(t, "printf '{}' > .mcp.json")

	res, err := session.Run(context.Background(), s)

	if err == nil {
		t.Fatal("an arm with no registration ran")
	}
	if res.Env.Root == "" {
		t.Fatal("the failed arm reported no environment, so its caller has nothing to release")
	}
	if res.Env.Repo == "" || res.Env.Config == "" {
		t.Errorf("the environment came back incomplete: %+v", res.Env)
	}
}

func TestAnAgentThatCannotBeSpawnedIsReported(t *testing.T) {
	s := spec(t, isolate.Baseline)
	s.Command = filepath.Join(t.TempDir(), "no-such-agent")
	s.Args = nil

	if _, err := session.Run(context.Background(), s); err == nil {
		t.Fatal("Run reported success although the agent could not be spawned")
	}
}

func TestCleanupRemovesTheWorktreeAndTheEnvironment(t *testing.T) {
	s := spec(t, isolate.Baseline)
	res, err := session.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if err := session.Cleanup(context.Background(), s.Parent, res.Env); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if _, err := os.Stat(res.Env.Root); !os.IsNotExist(err) {
		t.Errorf("Stat(%s) = %v after cleanup", res.Env.Root, err)
	}
	if list := gitOut(t, s.Parent, "worktree", "list"); strings.Contains(list, res.Env.Repo) {
		t.Errorf("the parent still lists the worktree:\n%s", list)
	}
}

func TestCleanupReportsAWorktreeItCouldNotRemove(t *testing.T) {
	s := spec(t, isolate.Baseline)
	res, err := session.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := session.Cleanup(context.Background(), s.Parent, res.Env); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if err := session.Cleanup(context.Background(), s.Parent, res.Env); err == nil {
		t.Fatal("Cleanup reported success on a worktree that is not there")
	}
}

// fakeSenseWriting is a stand-in whose setup runs the given shell.
func fakeSenseWriting(t *testing.T, setup string) string {
	t.Helper()
	script := "#!/bin/sh\nset -e\ncase \"$1\" in\nscan) mkdir -p \"$3/.sense\" ;;\nsetup)\n" + setup + "\n;;\nesac\n"
	path := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func mcpEntry(t *testing.T, path string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &doc); err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	servers, _ := doc["mcpServers"].(map[string]any)
	entry, ok := servers["sense"].(map[string]any)
	if !ok {
		t.Fatalf("%s registers no sense server", path)
	}
	return entry
}

func strs(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		s, _ := item.(string)
		out = append(out, s)
	}
	return out
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

// The whole chain, end to end: the registration the product wrote, rewritten to
// pass through the shim, spawning the server it named, with every frame landing
// in the run's capture file.

func TestTheRegistrationTheAgentWouldUseProducesTheCapture(t *testing.T) {
	labBin := buildLabBinary(t)
	s := spec(t, isolate.Sense)
	s.LabBin = labBin
	// A stand-in Sense whose `mcp` answers frames, so there is something real
	// on the far side of the shim.
	s.SenseBin = fakeSenseServing(t)

	res, err := session.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Invoked exactly as the registration says, which is what the agent CLI
	// would do. Nothing here reconstructs the command.
	entry := mcpEntry(t, filepath.Join(res.Env.Repo, ".mcp.json"))
	cmd := exec.Command(entry["command"].(string), strs(entry["args"])...)
	cmd.Stdin = strings.NewReader("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\"}\n")
	var spoke strings.Builder
	cmd.Stdout = &spoke
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("the registered server: %v", err)
	}

	if !strings.Contains(spoke.String(), `"result"`) {
		t.Errorf("the client received %q, want the server's answer through the shim", spoke.String())
	}
	captured := readCaptured(t, session.LogPath(res.Env))
	if len(captured) != 2 {
		t.Fatalf("captured %d frames from %s, want the request and the response", len(captured), session.LogPath(res.Env))
	}
	if captured[0]["dir"] != "c2s" || captured[1]["dir"] != "s2c" {
		t.Errorf("captured directions %v and %v", captured[0]["dir"], captured[1]["dir"])
	}

	status, present, err := session.Capture(res.Env)
	if err != nil || !present {
		t.Fatalf("Capture present=%v err=%v", present, err)
	}
	if !status.Complete() {
		t.Errorf("capture status = %+v, want a complete record", status)
	}
}

// buildLabBinary builds this binary so a test can invoke it the way a run's
// registration does.
func buildLabBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "sense-lab")
	build := exec.Command("go", "build", "-o", bin, "github.com/luuuc/sense/lab/cmd/sense-lab")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build sense-lab: %v: %s", err, out)
	}
	return bin
}

// fakeSenseServing is a stand-in Sense whose `mcp` answers newline-delimited
// frames, so the shim has a real server to sit in front of.
func fakeSenseServing(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "sense")
	script := `#!/bin/sh
set -e
case "$1" in
scan) mkdir -p "$3/.sense"; printf 'index' > "$3/.sense/index.db" ;;
setup) printf '{"mcpServers":{"sense":{"command":"` + bin + `","args":["mcp"],"type":"stdio"}}}' > .mcp.json ;;
mcp)
  while IFS= read -r line; do
    printf '{"jsonrpc":"2.0","id":1,"result":{"symbols":[]}}\n'
  done
  ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

func readCaptured(t *testing.T, path string) []map[string]any {
	t.Helper()
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(readFile(t, path)), "\n") {
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("capture line %q is not JSON: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

// A measured arm gives back the disk and keeps the evidence. A worktree is a
// full checkout — one mastodon pair put 230MB and about 19,700 files on disk —
// and the transcript beside it is read for months.
func TestAFinishedArmGivesBackItsCheckoutAndKeepsItsRecord(t *testing.T) {
	s := spec(t, isolate.Sense)
	s.Credential = aCredential()
	s.Route = aRoute()
	res, err := session.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if err := session.Finish(context.Background(), s.Parent, res.Env); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if _, err := os.Stat(res.Env.Repo); !os.IsNotExist(err) {
		t.Errorf("the checkout is still on disk: %v", err)
	}
	if list := gitOut(t, s.Parent, "worktree", "list"); strings.Contains(list, res.Env.Repo) {
		t.Errorf("the parent still lists the worktree:\n%s", list)
	}
	if _, err := os.Stat(isolate.CredentialPath(res.Env.Config)); err == nil {
		t.Error("the credential survived, in a directory kept for months")
	}
	// The evidence the scorer, the miner and status all read.
	for _, kept := range []string{
		filepath.Join(res.Env.Root, "session", "run-meta.json"),
		filepath.Join(res.Env.Root, "session", "raw", "stdout"),
	} {
		if _, err := os.Stat(kept); err != nil {
			t.Errorf("Finish destroyed %s, which is what the arm was run to produce", kept)
		}
	}
}

// A directory that holds nothing is a throwaway and goes whole. The shape is a
// property of the directory, not a flag the caller passes.
func TestADirectoryHoldingNoEvidenceIsRemovedEntirely(t *testing.T) {
	s := spec(t, isolate.Baseline)
	res, err := session.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Everything a later reader would want, gone: this is now the shape of an
	// attempt that produced nothing at all.
	for _, dir := range []string{"session", "artifacts", "logs"} {
		if err := os.RemoveAll(filepath.Join(res.Env.Root, dir)); err != nil {
			t.Fatal(err)
		}
	}

	if err := session.Finish(context.Background(), s.Parent, res.Env); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if _, err := os.Stat(res.Env.Root); !os.IsNotExist(err) {
		t.Errorf("a directory holding no evidence was kept: %v", err)
	}
}

// The leak that produced three stale registrations in one afternoon: an attempt
// that fails before the agent spawns has still taken a checkout, and git refuses
// the next attempt at that path until somebody prunes by hand.
func TestAnAttemptThatFailsBeforeSpawningLeaksNoRegistration(t *testing.T) {
	s := spec(t, isolate.Sense)
	// A setup that writes no registration, so the arm is refused after the
	// worktree exists and before anything is spawned.
	s.SenseBin = fakeSenseWriting(t, "printf '{}' > .mcp.json")

	if _, err := session.Run(context.Background(), s); err == nil {
		t.Fatal("an arm with no registration ran")
	}

	if list := gitOut(t, s.Parent, "worktree", "list"); strings.Contains(list, "run/repo") {
		t.Errorf("a refused attempt left a registration behind:\n%s", list)
	}
}

// The registration and the directory come apart, and the entry is what blocks
// the retry. Removing the directory by any other means must still leave the
// parent clean.
func TestAWorktreeWhoseDirectoryVanishedIsStillDeregistered(t *testing.T) {
	s := spec(t, isolate.Baseline)
	res, err := session.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := os.RemoveAll(res.Env.Repo); err != nil {
		t.Fatal(err)
	}

	if err := session.Finish(context.Background(), s.Parent, res.Env); err != nil {
		t.Fatalf("Finish: %v", err)
	}

	if list := gitOut(t, s.Parent, "worktree", "list"); strings.Contains(list, res.Env.Repo) {
		t.Errorf("the parent still lists a worktree whose directory is gone:\n%s", list)
	}
}

// aCredential is a usable seat, and aRoute the catalog names it reaches the tool
// by. Both are the test's own: nothing here is compiled into the lab.
func aCredential() isolate.Credential {
	return isolate.Credential{
		AccessToken: "seat-token",
		ExpiresAt:   time.Now().Add(time.Hour).UnixMilli(),
		Scopes:      []string{"user:inference"},
	}
}

func aRoute() isolate.Route {
	return isolate.Route{ConfigDirVar: "TOOL_CONFIG_DIR", ConfigDir: ".tool", Key: "toolOauth"}
}
