package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/crank"
	"github.com/luuuc/sense/lab/internal/isolate"
	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/position"
	"github.com/luuuc/sense/lab/internal/probe"
)

// soundPair and voidPair state what the two arms a phase reads came out as,
// without running any. The later call wins, so a test that wants a void pair
// states one after the world is built.
func soundPair(t *testing.T) *[]crank.Cell { return statedPair(t, "") }

func voidPair(t *testing.T, why string) *[]crank.Cell { return statedPair(t, why) }

func statedPair(t *testing.T, unsound string) *[]crank.Cell {
	t.Helper()
	var ran []crank.Cell
	was := phaseProbe
	phaseProbe = func(_ context.Context, _ repoFlags, c crank.Cell) (crank.Pair, error) {
		ran = append(ran, c)
		dir := filepath.Join(c.Dir, cellName)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return crank.Pair{}, err
		}
		if unsound != "" {
			return crank.Pair{Dir: dir, Note: unsound}, nil
		}
		return crank.Pair{Sound: true, Dir: dir}, nil
	}
	t.Cleanup(func() { phaseProbe = was })
	return &ran
}

// phaseAgent stands in for the agent a phase is run by. It does what a phase
// agent does and nothing else: it reads where its artifact and its verdict
// belong out of what it was handed, writes both, and records the login it was
// given so a test can read what reached it.
//
// The verdict is chosen by phase from the plan's own declarations, so this one
// script can walk a whole authoring cycle.
const phaseAgentScript = `#!/bin/sh
set -e
in=$(cat)
art=$(printf '%s\n' "$in" | sed -n 's/^artifact: //p' | head -1)
ver=$(printf '%s\n' "$in" | sed -n 's/^verdict: //p' | head -1)
phase=$(printf '%s\n' "$in" | sed -n 's/^phase: //p' | head -1)
repo=$(printf '%s\n' "$in" | sed -n 's/^repository: //p' | head -1)
cycle=$(printf '%s\n' "$in" | sed -n 's/^cycle: \([0-9]*\).*/\1/p' | head -1)
case "$phase" in
  author) verdict=DRAFT ;;
  minibench) verdict=PROCEED ;;
  expand|preflight) verdict=AUTO ;;
  validate) verdict=PAY ;;
  *) verdict=AUTO ;;
esac
mkdir -p "$(dirname "$art")"
pwd > "$(dirname "$art")/given-cwd.txt"
if [ -n "$PHASE_AGENT_SLEEP" ]; then sleep "$PHASE_AGENT_SLEEP"; fi
if [ -z "$PHASE_AGENT_NO_ARTIFACT" ]; then printf 'what %s wrote\n' "$phase" > "$art"; fi
printf '{"phase":"%s","repo":"%s","cycle":%s,"verdict":"%s","anchor":"Category"}\n' \
  "$phase" "$repo" "$cycle" "$verdict" > "$ver"
# What the phase was actually handed, kept where the test can read it.
cp "$FAKE_CONFIG_DIR/.credentials.json" "$(dirname "$art")/given-credential.json" 2>/dev/null || true
printf '%s\n' "$in" > "$(dirname "$art")/given-prompt.txt"
`

// crankWorld is a repository admitted, scanned, and ready for its first
// authoring phase, with an agent that can run one.
type crankWorld struct {
	admission
	repo     string
	runs     string
	checkout string
}

func newCrankWorld(t *testing.T, env ...string) crankWorld {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in agent is a shell script")
	}
	a, id := admitted(t)
	bin := filepath.Join(t.TempDir(), "phase-agent")
	if err := os.WriteFile(bin, []byte(phaseAgentScript), 0o755); err != nil {
		t.Fatal(err)
	}
	artifact(t, filepath.Join(a.config, "agents", "phase", "agent.json"),
		`{"id":"phase","binary":"`+bin+`","setup_tool":"fake-cli","transcript_format":"assistant-events",`+
			`"model_flag":"--model","config_dirs":[".fake"],"config_dir_var":"FAKE_CONFIG_DIR",`+
			`"keychain_service":"Fake-credentials","credential_file":".credentials.json",`+
			`"credential_fields":["fakeOauth.accessToken","fakeOauth.scopes"],`+
			`"credential_expiry":"ms:fakeOauth.expiresAt",`+
			`"headless_args":["-c"],"env":`+quoted(t, env)+`,"supports_mcp":true,"auth_modes":["subscription"]}`)
	artifact(t, filepath.Join(a.config, "models", "m1.json"),
		`{"id":"m1","provider":"acme","available_under":["subscription"],"agents":["phase"]}`)
	// The plans are config, so the world points at the shipped ones.
	linkPlans(t, a.config)
	// A host that holds a seat, stated rather than logged in: CI has none.
	hostHolding(t, aSeatWithNoise(time.Hour), nil)
	// The pair the mini-bench reads, stated rather than run. It is driven end
	// to end in TestThePairAPhaseReadsRunsBothArms; a crank test that ran one
	// would spawn two agents against a real checkout to assert something about
	// routing.
	soundPair(t)
	for _, name := range isolate.Credentials() {
		t.Setenv(name, "")
	}
	return crankWorld{admission: a, repo: id, runs: a.runs, checkout: checkoutOf(t, a, id)}
}

func (w crankWorld) run(t *testing.T, extra ...string) (int, string, string) {
	t.Helper()
	args := []string{"repo", "-config", w.config, "-runs", w.runs, "-checkouts", w.checkouts,
		"-sense", w.sense, "-agent", "phase", "-model", "m1"}
	return dispatch(t, append(append(args, extra...), w.repo)...)
}

func (w crankWorld) phaseDir(name string) string {
	return filepath.Join(w.runs, w.repo, "1", name)
}

// linkPlans points the world's config at the shipped plans. They are loaded from
// the config directory, so a world with none would test a crank with no graph.
func linkPlans(t *testing.T, config string) {
	t.Helper()
	shipped, err := filepath.Abs(filepath.Join("..", "..", "plans"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(shipped, filepath.Join(config, "plans")); err != nil {
		t.Fatal(err)
	}
}

// aSeatWithNoise is a host store carrying the login AND things a run has no
// business seeing. What reaches the phase is what the allowlist names.
func aSeatWithNoise(good time.Duration) isolate.Credential {
	c := aSeat(good)
	c.Fields["fakeOauth.refreshToken"] = json.RawMessage(`"the-refresh-token"`)
	c.Fields["otherAccount.accessToken"] = json.RawMessage(`"somebody-elses"`)
	return c
}

// One turn of the crank, for real: a phase agent spawned, its verdict read, its
// artifact checked, the attempt recorded and the position moved.
func TestTheCrankRunsAPhaseAndTheLoopMoves(t *testing.T) {
	w := newCrankWorld(t)

	code, stdout, stderr := w.run(t)

	if code != exitOK {
		t.Fatalf("exit %d: %s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "awaiting: minibench") {
		t.Errorf("stdout = %q, want the loop moved to the next phase", stdout)
	}
	if _, err := os.Stat(filepath.Join(w.phaseDir("author"), "scenario.draft.yaml")); err != nil {
		t.Errorf("the phase's artifact is not there: %v", err)
	}
}

// The credential a phase agent is handed carries no more than a run arm's, and
// it is asserted on the document that reached it rather than on the code that
// wrote one.
func TestAPhaseAgentIsHandedNoMoreThanARunArm(t *testing.T) {
	w := newCrankWorld(t)

	if code, stdout, stderr := w.run(t); code != exitOK {
		t.Fatalf("exit %d: %s%s", code, stdout, stderr)
	}

	b, err := os.ReadFile(filepath.Join(w.phaseDir("author"), "given-credential.json"))
	if err != nil {
		t.Fatalf("the phase was handed no credential document: %v", err)
	}
	given := string(b)
	if !strings.Contains(given, "accessToken") {
		t.Errorf("the phase was handed no login at all: %s", given)
	}
	for _, forbidden := range []string{"refreshToken", "otherAccount", "the-refresh-token"} {
		if strings.Contains(given, forbidden) {
			t.Errorf("the phase was handed %q, which is outside the allowlist: %s", forbidden, given)
		}
	}
}

// `-until` cranks until it stops on its own, and where it stops is the pay call:
// the phases that spend stay hand-run this cycle.
func TestUntilCranksToThePayCallAndStops(t *testing.T) {
	w := newCrankWorld(t)

	code, stdout, stderr := w.run(t, "-until")

	if code != exitWaiting {
		t.Fatalf("exit %d, want it waiting on a human: %s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "sense-lab probe") {
		t.Errorf("stdout = %q, want the command to run by hand", stdout)
	}
	for _, phase := range []string{"author", "minibench", "expand", "preflight", "validate"} {
		if _, err := os.Stat(filepath.Join(w.phaseDir(phase), "verdict.json")); err != nil {
			t.Errorf("%s never ran: %v", phase, err)
		}
	}
	if _, err := os.Stat(w.phaseDir("bench")); err == nil {
		t.Error("the crank reached a phase that spends")
	}
}

// The plan reaches the agent, and the facts of the attempt with it. Read off
// what the agent was handed rather than off what the crank composed.
func TestThePhaseAgentIsHandedItsPlan(t *testing.T) {
	w := newCrankWorld(t)

	if code, _, stderr := w.run(t); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	b, err := os.ReadFile(filepath.Join(w.phaseDir("author"), "given-prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	handed := string(b)
	for _, want := range []string{"repository: " + w.repo, "cycle: 1 of 6", "# author", "## Task"} {
		if !strings.Contains(handed, want) {
			t.Errorf("the agent was not handed %q:\n%s", want, handed)
		}
	}
}

// Nothing is dispatched without something to dispatch it with, and the refusal
// says which flags are missing rather than spawning nothing quietly.
func TestNoPhaseRunsWithoutAnAgentAndAModel(t *testing.T) {
	w := newCrankWorld(t)

	code, stdout, _ := dispatch(t, "repo", "-config", w.config, "-runs", w.runs,
		"-checkouts", w.checkouts, "-sense", w.sense, w.repo)

	if code != exitOK {
		t.Fatalf("exit %d, want the position printed", code)
	}
	if !strings.Contains(stdout, "awaiting: author") {
		t.Errorf("stdout = %q, want the position", stdout)
	}
	if _, err := os.Stat(w.phaseDir("author")); err == nil {
		t.Error("a phase ran with no agent named")
	}
}

func TestAnAgentTheCatalogDoesNotKnowIsRefused(t *testing.T) {
	w := newCrankWorld(t)

	code, _, stderr := dispatch(t, "repo", "-config", w.config, "-runs", w.runs,
		"-checkouts", w.checkouts, "-sense", w.sense, "-agent", "nobody", "-model", "m1", w.repo)

	if code != exitError {
		t.Fatalf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "no agent") {
		t.Errorf("stderr = %q, want it to name what is missing", stderr)
	}
}

// A config with no plans is a graph nobody can run, and it is refused rather
// than dispatched against with an empty prompt.
func TestAConfigWithNoPlansIsRefused(t *testing.T) {
	w := newCrankWorld(t)
	if err := os.Remove(filepath.Join(w.config, "plans")); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := w.run(t)

	if code != exitError {
		t.Fatalf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "plan") {
		t.Errorf("stderr = %q, want it to name what is missing", stderr)
	}
}

// A second attempt at one phase keeps the first's evidence. A run environment is
// created fresh or not at all, and overwriting one destroys the record of why
// there was a second attempt.
func TestASecondAttemptLandsBesideTheFirst(t *testing.T) {
	if got := attemptDir("session", 1); got != "session" {
		t.Errorf("first attempt = %q, want the ordinary name", got)
	}
	if got := attemptDir("session", 2); got != "session-2" {
		t.Errorf("second attempt = %q, want it beside the first", got)
	}
}

// A driver named by half is not a driver, and the refusal says so rather than
// dispatching nothing quietly.
func TestHalfADriverIsRefused(t *testing.T) {
	w := newCrankWorld(t)

	code, _, stderr := dispatch(t, "repo", "-config", w.config, "-runs", w.runs,
		"-checkouts", w.checkouts, "-sense", w.sense, "-agent", "phase", w.repo)

	if code != exitError {
		t.Fatalf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "-agent and -model") {
		t.Errorf("stderr = %q, want it to name both flags", stderr)
	}
}

// A phase agent that could not be given a login does not run. Two arms of a real
// cell once exited in about a second with "Not logged in", and a phase agent
// fails the same way for the same reason.
func TestAPhaseWithNoCredentialDoesNotRun(t *testing.T) {
	w := newCrankWorld(t)
	hostHolding(t, isolate.Credential{}, errors.New("no store on this machine"))

	code, _, stderr := w.run(t)

	if code != exitError {
		t.Fatalf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "credential") {
		t.Errorf("stderr = %q, want it to say what is missing", stderr)
	}
	if _, err := os.Stat(filepath.Join(w.phaseDir("author"), "verdict.json")); err == nil {
		t.Error("a phase ran with no login")
	}
}

// A run environment is created fresh or not at all, so one that is already there
// stops the phase rather than being reused.
func TestAnEnvironmentThatIsAlreadyThereStopsThePhase(t *testing.T) {
	w := newCrankWorld(t)
	artifact(t, filepath.Join(w.phaseDir("author"), "env", "leftover"), "from a previous run\n")

	code, _, stderr := w.run(t)

	if code != exitError {
		t.Fatalf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Errorf("stderr = %q, want it to say the environment was not fresh", stderr)
	}
}

// A repository the catalog does not hold cannot be cranked, whatever a run tree
// says about it.
func TestARepositoryTheCatalogDoesNotHoldIsRefused(t *testing.T) {
	w := newCrankWorld(t)
	if err := os.Remove(w.repoFile(w.repo)); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := w.run(t)

	if code != exitError {
		t.Fatalf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "not an admitted id") && !strings.Contains(stderr, "no repository") {
		t.Errorf("stderr = %q, want it to say the repository is unknown", stderr)
	}
}

// A repository the lab cloned has no checkout recorded, and the phase runs in
// the clone under the lab's own root.
func TestALabOwnedRepositoryIsCrankedInItsOwnClone(t *testing.T) {
	w := newCrankWorld(t)
	clone, _ := sourceRepo(t)
	if err := os.MkdirAll(w.checkouts, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(clone, filepath.Join(w.checkouts, w.repo)); err != nil {
		t.Fatal(err)
	}
	// No checkout recorded is what says the lab made this clone.
	artifact(t, w.repoFile(w.repo), `{"id":"`+w.repo+`","url":"https://example.test/`+w.repo+
		`.git","commit":"`+headOf(t, clone)+`","languages":["go"]}`)

	if code, stdout, stderr := w.run(t); code != exitOK {
		t.Fatalf("exit %d: %s%s", code, stdout, stderr)
	}

	b, err := os.ReadFile(filepath.Join(w.phaseDir("author"), "given-prompt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "repository: "+w.repo) {
		t.Errorf("the phase was not told which repository it was in:\n%s", b)
	}
}

func headOf(t *testing.T, dir string) string {
	t.Helper()
	return git(t, dir, "rev-parse", "HEAD")
}

// No interactive prompt exists anywhere in the run path.
//
// The law is cycle 05's and it binds this cycle hardest, because this is the
// cycle where a confirmation would feel most helpful: the crank is about to
// spawn something. The printed position before the act is the whole affordance,
// and a prompt would turn an unattended loop into one that waits forever on a
// terminal nobody is watching.
//
// The probe is over the source rather than over behaviour, because a prompt
// that is never reached in a test is exactly the one that would be added.
func TestNothingInTheRunPathReadsFromATerminal(t *testing.T) {
	for _, at := range []string{
		filepath.Join("..", "crank"),
		filepath.Join("..", "position"),
		filepath.Join("..", "repo"),
	} {
		entries, err := os.ReadDir(at)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(at, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			for _, forbidden := range []string{"os.Stdin", "bufio.NewScanner(os.", "fmt.Scan"} {
				if strings.Contains(string(b), forbidden) {
					t.Errorf("%s/%s reads a terminal (%s); the run path has no interactive prompt in it",
						at, e.Name(), forbidden)
				}
			}
		}
	}
	// And the one file in this package that turns the crank does not either.
	b, err := os.ReadFile("crankcmd.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "os.Stdin") {
		t.Error("crankcmd.go reads a terminal; an unattended loop cannot answer a question")
	}
}

// The phase agent runs in the repository under study. One pointed anywhere else
// reads nothing, and its draft would be about a repository it never opened.
func TestThePhaseAgentRunsInTheRepositoryUnderStudy(t *testing.T) {
	w := newCrankWorld(t)

	if code, stdout, stderr := w.run(t); code != exitOK {
		t.Fatalf("exit %d: %s%s", code, stdout, stderr)
	}

	b, err := os.ReadFile(filepath.Join(w.phaseDir("author"), "given-cwd.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(w.checkout)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(b)); got != want {
		t.Errorf("the phase ran in %q, want the repository under study (%q)", got, want)
	}
}

// A phase agent that runs past its wall is recorded as out of clock and the
// crank stops, rather than holding on it. The wall comes off the plan, so this
// is driven by editing a declaration.
func TestARealAgentThatRunsPastItsWallIsRecordedAsOutOfClock(t *testing.T) {
	w := newCrankWorld(t, "PHASE_AGENT_SLEEP=5")
	copyPlansWithWall(t, w.config, "author", "1s")

	code, stdout, stderr := w.run(t)

	if code != exitMissing {
		t.Fatalf("exit %d, want the agent recorded as one that did not finish: %s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "past its wall") {
		t.Errorf("stdout = %q, want it to say the phase ran out of clock", stdout)
	}
	recorded := attemptsOf(t, w)
	if len(recorded) != 1 || recorded[0].Outcome != position.Stalled {
		t.Errorf("recorded %+v, want one stalled attempt", recorded)
	}
}

// An exit code is a claim; the artifact is the fact. A real agent that finished,
// said the right thing and wrote nothing does not advance.
func TestARealAgentThatWroteNoArtifactDoesNotAdvance(t *testing.T) {
	w := newCrankWorld(t, "PHASE_AGENT_NO_ARTIFACT=1")

	code, stdout, stderr := w.run(t)

	if code != exitMissing {
		t.Fatalf("exit %d, want it held at the missing artifact: %s%s", code, stdout, stderr)
	}
	if recorded := attemptsOf(t, w); recorded[0].Artifact != "" {
		t.Errorf("recorded artifact %q, want none: nothing was accepted", recorded[0].Artifact)
	}
}

func attemptsOf(t *testing.T, w crankWorld) []position.Attempt {
	t.Helper()
	all, err := position.Attempts(filepath.Join(w.runs, w.repo))
	if err != nil {
		t.Fatal(err)
	}
	return all
}

// copyPlansWithWall replaces the world's symlinked plans with real files, one of
// them carrying a different wall. It is how a wall is changed the way a person
// changes one: by editing the declaration.
func copyPlansWithWall(t *testing.T, config, name, wall string) {
	t.Helper()
	at := filepath.Join(config, "plans")
	if err := os.Remove(at); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(at, 0o755); err != nil {
		t.Fatal(err)
	}
	shipped, err := filepath.Abs(filepath.Join("..", "..", "plans"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(shipped)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(shipped, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if e.Name() == name+".md" {
			b = wallLine.ReplaceAll(b, []byte("wall: "+wall+"\n"))
		}
		if err := os.WriteFile(filepath.Join(at, e.Name()), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

var wallLine = regexp.MustCompile(`(?m)^wall: .*\n`)

// checkoutOf is where the world's repository actually is, read from the record
// the admission wrote rather than assumed.
func checkoutOf(t *testing.T, a admission, id string) string {
	t.Helper()
	var r struct{ Checkout string }
	readJSON(t, a.repoFile(id), &r)
	if r.Checkout == "" {
		return filepath.Join(a.checkouts, id)
	}
	return r.Checkout
}

// quoted renders the agent's declared environment as the catalog spells it. A
// phase agent's environment is what the catalog says it is, so a test that
// wants the stand-in to behave differently declares it there.
func quoted(t *testing.T, env []string) string {
	t.Helper()
	b, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if len(env) == 0 {
		return "[]"
	}
	return string(b)
}

// The pair a phase reads, driven end to end: one call produces both arms,
// isolated, where the phase that reads them will look. This is the step the
// crank was missing, so it is asserted on the arms on disk rather than on the
// call that made them.
func TestThePairAPhaseReadsRunsBothArms(t *testing.T) {
	w := newProbeWorld(t)
	dir := filepath.Join(t.TempDir(), "minibench")

	got, err := liveProbe(context.Background(), repoFlags{
		config: w.config, runs: w.runs, senseBin: w.senseBin, agent: "fake", model: "fake-model",
	}, crank.Cell{Repo: "probe-repo", Cycle: 1, Phase: phase.Minibench,
		Dir: dir, Scenario: w.scenario, Checkout: w.checkout})

	if err != nil {
		t.Fatal(err)
	}
	if !got.Sound {
		t.Errorf("the pair is not a measurement: %s", got.Note)
	}
	if want := filepath.Join(dir, cellName); got.Dir != want {
		t.Errorf("the pair landed in %s, want %s", got.Dir, want)
	}
	// Two arms on disk, in one cell, with the cell's own record beside them.
	for _, rel := range []string{"sense", "baseline", "cell-meta.json"} {
		if _, err := os.Stat(filepath.Join(got.Dir, rel)); err != nil {
			t.Errorf("the cell is missing %s: %v", rel, err)
		}
	}
}

// A second pair lands beside the first. Two arms are created fresh or not at
// all, so a name that is already taken is not re-used: the case that matters is
// a cell interrupted between its arms, which leaves a directory naming a burned
// run and would wedge the phase there for good.
func TestASecondPairLandsBesideTheFirst(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, cellName), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := freeCell(dir)

	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, cellName+"-2"); got != want {
		t.Errorf("the next pair would land in %s, want %s", got, want)
	}
}

// A tree that holds every name there is says so rather than writing over one.
func TestAPhaseFullOfPairsRefusesAnother(t *testing.T) {
	dir := t.TempDir()
	for n := 1; n <= 99; n++ {
		if err := os.MkdirAll(filepath.Join(dir, attemptDir(cellName, n)), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := freeCell(dir); err == nil {
		t.Error("a phase holding 99 pairs was handed a hundredth")
	}
}

// What a void pair is recorded as: the checks that refused it, named. A record
// saying only that the pair was void would send whoever reads it back to the
// transcripts to find out which check fired.
func TestAVoidPairIsRecordedWithTheChecksThatRefusedIt(t *testing.T) {
	for _, tc := range []struct {
		name   string
		report probe.Report
		want   string
	}{
		{"a route the sense arm never got", probe.Report{SenseMissing: []string{".mcp.json"}}, ".mcp.json"},
		{"the baseline reached one", probe.Report{BaselineReached: []string{"CLAUDE.md"}}, "CLAUDE.md"},
		{"memory survived", probe.Report{MemoryReached: []string{"~/.claude"}}, "~/.claude"},
		{"Sense in the baseline", probe.Report{BaselineUsed: []string{"sense_graph"}}, "sense_graph"},
		{"the arms differ", probe.Report{Differences: []string{"budget"}}, "budget"},
		{"the sense arm never used Sense", probe.Report{}, "never used Sense"},
		{"no frames", probe.Report{}, "no MCP frames"},
		{"the baseline left a capture", probe.Report{BaselineCaptured: true}, "left a capture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := notSound(tc.report); !strings.Contains(got, tc.want) {
				t.Errorf("notSound = %q, want it to name %q", got, tc.want)
			}
		})
	}
}

// The mini-bench judge is spawned on a pair, and on nothing else. A void pair
// stops the loop with the reason recorded, which is the shape a binary defect
// has to have: the plan promises its verdicts are always issuable, and they are
// not issuable on arms that cannot be compared.
func TestAVoidPairStopsTheLoopBeforeTheJudge(t *testing.T) {
	w := newCrankWorld(t)
	voidPair(t, "the baseline arm reached the sense server")

	code, stdout, stderr := w.run(t, "-until")

	if code != exitUnusable {
		t.Fatalf("exit %d, want the loop stopped on a pair nobody may rule on: %s%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "the baseline arm reached the sense server") {
		t.Errorf("stdout = %q, want the check that refused the pair", stdout)
	}
	if _, err := os.Stat(filepath.Join(w.phaseDir("minibench"), "verdict.json")); err == nil {
		t.Error("a judge ruled on a pair that is not a measurement")
	}
}

// Where the pair lands: under the phase that reads it, in this cycle, beside
// the environment and session of the attempt that read it.
func TestThePairLandsUnderThePhaseThatReadsIt(t *testing.T) {
	w := newCrankWorld(t)
	ran := soundPair(t)

	if code, stdout, stderr := w.run(t, "-until"); code != exitWaiting {
		t.Fatalf("exit %d: %s%s", code, stdout, stderr)
	}

	// Two, not one: a cycle to the pay call passes both phases that judge a
	// pair, and each reads the artifact the phase before it wrote.
	if len(*ran) != 2 {
		t.Fatalf("ran %d pairs over a whole cycle, want one per phase that reads one", len(*ran))
	}
	for i, want := range []struct{ dir, scenario string }{
		{w.phaseDir("minibench"), filepath.Join(w.phaseDir("author"), "scenario.draft.yaml")},
		{w.phaseDir("validate"), filepath.Join(w.phaseDir("expand"), "scenario.yaml")},
	} {
		if got := (*ran)[i]; got.Dir != want.dir {
			t.Errorf("pair %d landed under %s, want %s", i+1, got.Dir, want.dir)
		}
		if got := (*ran)[i].Scenario; got != want.scenario {
			t.Errorf("pair %d ran against %s, want %s", i+1, got, want.scenario)
		}
	}
}

// A cell that cannot be set up is reported rather than ruled on: the scenario
// the phase before it was supposed to write is not there, or is not a scenario.
func TestAPairThatCannotBeSetUpIsReported(t *testing.T) {
	w := newProbeWorld(t)

	_, err := liveProbe(context.Background(), repoFlags{
		config: w.config, runs: w.runs, senseBin: w.senseBin, agent: "fake", model: "fake-model",
	}, crank.Cell{Repo: "probe-repo", Cycle: 1, Phase: phase.Minibench,
		Dir: t.TempDir(), Scenario: filepath.Join(t.TempDir(), "never-written.yaml"), Checkout: w.checkout})

	if err == nil {
		t.Error("a pair was reported against a scenario that is not there")
	}
}

// A pair whose arms could not be given Sense is not a measurement, and the
// binary says which check refused it instead of handing the judge two arms that
// cannot be compared.
func TestAPairThatCouldNotBeGivenSenseIsNotAMeasurement(t *testing.T) {
	w := newProbeWorld(t)

	got, err := liveProbe(context.Background(), repoFlags{
		config: w.config, runs: w.runs, agent: "fake", model: "fake-model",
		senseBin: filepath.Join(t.TempDir(), "no-such-sense"),
	}, crank.Cell{Repo: "probe-repo", Cycle: 1, Phase: phase.Minibench,
		Dir: t.TempDir(), Scenario: w.scenario, Checkout: w.checkout})

	if err == nil && got.Sound {
		t.Error("a pair whose sense arm never had Sense was reported as a measurement")
	}
}
