package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/isolate"
)

// The stand-ins are the ones lab/internal/probe is tested with: a Sense that
// indexes, configures and answers a tool list, and an agent that reaches the
// registered server when there is one. They are here so this test drives the
// COMMAND end to end rather than asserting it called something.
const (
	fakeSenseScript = `#!/bin/sh
set -e
case "$1" in
scan) mkdir -p "$3/.sense"; printf 'index' > "$3/.sense/index.db" ;;
setup)
  printf '{"mcpServers":{"sense":{"command":"%s","args":["mcp"]}}}' "$0" > .mcp.json
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

	// The agent reaches the server when a registration exists and falls back to
	// grep when it does not, which is what the two arms are supposed to differ
	// in.
	fakeAgentScript = `#!/bin/sh
cat > /dev/null
if [ -f .mcp.json ]; then
  flat=$(tr -d ' \n' < .mcp.json)
  server=$(printf '%s' "$flat" | sed 's/.*"command":"\([^"]*\)".*/\1/')
  args=$(printf '%s' "$flat" | sed 's/.*"args":\[\([^]]*\)\].*/\1/' | tr -d '"' | tr ',' ' ')
  printf '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"sense_graph"}}\n' | $server $args > /dev/null
  printf '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__sense__sense_graph"}]}}\n'
  printf '{"type":"result","result":"Category has three dependents"}\n'
  exit 0
fi
printf '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Grep"}]}}\n'
printf '{"type":"result","result":"Category is referenced in three files"}\n'
`
)

func writeScript(t *testing.T, path, body string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// parentRepo is the repository the worktrees are taken from.
func parentRepo(t *testing.T) (dir, commit string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=lab", "GIT_AUTHOR_EMAIL=lab@example.com",
			"GIT_COMMITTER_NAME=lab", "GIT_COMMITTER_EMAIL=lab@example.com")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package app\n\ntype Category struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "first")
	return dir, git("rev-parse", "HEAD")
}

// commitInto adds a file and commits it, moving the repository's HEAD.
func commitInto(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "-q", "-m", "later"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=lab", "GIT_AUTHOR_EMAIL=lab@example.com",
			"GIT_COMMITTER_NAME=lab", "GIT_COMMITTER_EMAIL=lab@example.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
}

// probeWorld is a complete catalog, scenario and checkout for one cell.
type probeWorld struct {
	config   string
	scenario string
	checkout string
	senseBin string
	out      string
	runs     string
}

// buildLabBinary builds this binary so the capture shim is spawned the way a
// run's registration spawns it. Under `go test` os.Executable() is the test
// binary, which has no tee.
func buildLabBinary(t *testing.T) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "sense-lab")
	if out, err := exec.Command("go", "build", "-o", bin, "github.com/luuuc/sense/lab/cmd/sense-lab").CombinedOutput(); err != nil {
		t.Fatalf("build sense-lab: %v: %s", err, out)
	}
	was := labBinary
	labBinary = func() (string, error) { return bin, nil }
	t.Cleanup(func() { labBinary = was })
}

func newProbeWorld(t *testing.T) probeWorld {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-ins are shell scripts")
	}
	buildLabBinary(t)
	// A credential the session could reach. The stand-in agent never uses it;
	// what it satisfies is the check that refuses a cell whose arms could not
	// authenticate, and without it every probe test would exercise that refusal
	// instead of the thing it is named for.
	t.Setenv("ANTHROPIC_API_KEY", "a-test-key")
	root := t.TempDir()
	checkout, commit := parentRepo(t)

	config := filepath.Join(root, "config")
	write := func(rel, body string) {
		t.Helper()
		at := filepath.Join(config, rel)
		if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	agentBin := writeScript(t, filepath.Join(root, "bin", "fake-agent"), fakeAgentScript)
	write("repos/probe-repo.json", `{"id":"probe-repo","url":"https://example.com/r.git","commit":"`+commit+`","languages":["go"],"stack":"go"}`)
	write("agents/fake/agent.json", `{"id":"fake","binary":"`+agentBin+`","setup_tool":"fake-cli",`+
		`"transcript_format":"assistant-events","model_flag":"--model",`+
		`"mcp_registration":{"file":".mcp.json","servers_key":"mcpServers","command_style":"command+args"},`+
		`"config_dirs":[".fake"],"headless_args":["-c"],"judge_args":["-c"],"env":[],"supports_mcp":true,`+
		`"auth_modes":["api_key"]}`)
	write("models/fake-model.json", `{"id":"fake-model","provider":"fake","aliases":[],"available_under":["api_key"],"agents":["fake"]}`)
	write("subjects/untreated/subject.json", `{"id":"untreated","kind":"baseline","needs_mcp":false,`+
		`"needs_isolated_config":false,"executor":"isolated-home","agents":["fake"]}`)
	write("subjects/sense-main/subject.json", `{"id":"sense-main","kind":"sense","needs_mcp":true,`+
		`"needs_isolated_config":true,"executor":"isolated-home","agents":["fake"]}`)
	write("executors/isolated-home.json", `{"id":"isolated-home","preserves_auth":["subscription","api_key"],"isolates_global_config":true}`)

	scenarioPath := filepath.Join(root, "probe.yaml")
	if err := os.WriteFile(scenarioPath, []byte(`name: Probe cell
repo: probe-repo
steps:
  - name: Audit every dependent
    prompt: "What depends on Category? Answer with file:line for each."
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "probe.rubric.yaml"), []byte(`audience: |
  An AI coding agent about to rework the Category type.
steps:
  - name: Audit every dependent
    criteria:
      map_quality:
        weight: 0.40
        question: |
          Does the answer name every dependent at an exact file:line?
      specificity:
        weight: 0.25
        question: |
          Is each dependent pinned to a line rather than a file?
      justification:
        weight: 0.20
        question: |
          Does it say how each dependent uses the type?
      uncertainty:
        weight: 0.15
        question: |
          Does it say what it could not establish?
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "probe.gold.yaml"), []byte(`discriminator: dependents
rows:
  - {id: "d:app", group: dependents, match: ["app.go"], relation: "app.go:3 the Category type"}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	return probeWorld{
		config:   config,
		scenario: scenarioPath,
		checkout: checkout,
		senseBin: writeScript(t, filepath.Join(root, "bin", "sense"), fakeSenseScript),
		out:      filepath.Join(root, "cell-0"),
		runs:     filepath.Join(root, "runs"),
	}
}

func (w probeWorld) args(extra ...string) []string {
	return append([]string{"probe",
		"-config", w.config,
		"-scenario", w.scenario,
		"-repo", "probe-repo",
		"-checkout", w.checkout,
		"-agent", "fake",
		"-model", "fake-model",
		"-sense", w.senseBin,
		"-out", w.out,
		"-runs", w.runs,
		"-wall", "20s",
	}, extra...)
}

func runProbe(t *testing.T, args []string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// The whole point of the command: one invocation produces both arms, isolated,
// with the capture in place, and says whether the pair is a measurement.
func TestProbeRunsBothArmsAndReportsASoundPair(t *testing.T) {
	w := newProbeWorld(t)
	code, stdout, stderr := runProbe(t, w.args())
	if code != exitOK {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "SOUND") {
		t.Errorf("the pair was not reported sound:\n%s", stdout)
	}

	// Two arms on disk, in one cell, with the cell's own record beside them.
	for _, rel := range []string{"sense", "baseline", "cell-meta.json"} {
		if _, err := os.Stat(filepath.Join(w.out, rel)); err != nil {
			t.Errorf("the cell is missing %s: %v", rel, err)
		}
	}
	// The capture the failed hand-rolled attempt never produced.
	capture := filepath.Join(w.out, "sense", "artifacts", "sense-io.jsonl")
	info, err := os.Stat(capture)
	if err != nil {
		t.Fatalf("no MCP capture for the sense arm: %v", err)
	}
	if info.Size() == 0 {
		t.Error("the capture is empty, which is indistinguishable from a capture failure")
	}
	// And the baseline left none at all, which is the signal rather than an
	// empty file.
	if _, err := os.Stat(filepath.Join(w.out, "baseline", "artifacts", "sense-io.jsonl")); err == nil {
		t.Error("the baseline arm left a capture; its absence is what says it never reached Sense")
	}
}

// Every check is printed whether or not it fired. A report that only listed
// problems would be indistinguishable from one that ran no checks.
func TestProbeReportsEveryCheckOnACleanPair(t *testing.T) {
	w := newProbeWorld(t)
	_, stdout, _ := runProbe(t, w.args())
	for _, want := range []string{
		"sense arm routes", "all present",
		"baseline arm routes", "none reachable",
		"persisted memory", "unreadable by both arms",
		"Sense in the baseline", "no sign of it",
		"arms differ in", "nothing but Sense access",
		"MCP frames captured",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the report does not carry %q:\n%s", want, stdout)
		}
	}
}

// A pair that is not a measurement gets its own exit code, so a caller can tell
// "the arms were not comparable" from "the binary broke" without parsing output.
func TestAPairThatIsNotAMeasurementIsRefusedWithItsOwnCode(t *testing.T) {
	w := newProbeWorld(t)
	// An agent that never touches Sense, in both arms. The pair runs, and the
	// sense arm having had Sense and ignored it is not a measurement of the
	// baseline.
	writeScript(t, filepath.Join(filepath.Dir(w.senseBin), "fake-agent"), `#!/bin/sh
cat > /dev/null
printf '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Grep"}]}}\n'
printf '{"type":"result","result":"Category is referenced in three files"}\n'
`)
	code, stdout, stderr := runProbe(t, w.args())
	if code != exitRefused {
		t.Fatalf("exit %d, want the refusal code\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "NOT A MEASUREMENT") {
		t.Errorf("the refusal does not say the pair may not be scored:\n%s", stdout)
	}
	if !strings.Contains(stdout, "may not be cited") {
		t.Errorf("the refusal does not say the number may not be cited:\n%s", stdout)
	}
}

// The same pin check `run` makes. A cell against the wrong tree records a
// commit it did not use, and both arms would carry it.
func TestProbeRefusesACheckoutThatIsNotAtThePinnedCommit(t *testing.T) {
	w := newProbeWorld(t)
	// A repository built the same way in the same second hashes identically, so
	// the divergence has to be real.
	other, _ := parentRepo(t)
	commitInto(t, other, "later.go", "package app\n")
	code, _, stderr := runProbe(t, w.args("-checkout", other))
	if code != exitError {
		t.Fatalf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "records a commit it did not use") {
		t.Errorf("the refusal does not say why: %q", stderr)
	}
}

func TestProbeRefusesAnIncompleteInvocation(t *testing.T) {
	w := newProbeWorld(t)
	for _, missing := range []string{"-scenario", "-repo", "-checkout", "-out", "-agent", "-model"} {
		t.Run(missing, func(t *testing.T) {
			args := w.args()
			for i, a := range args {
				if a == missing {
					args[i+1] = ""
				}
			}
			code, _, stderr := runProbe(t, args)
			if code != exitUsage {
				t.Errorf("exit %d without %s, want a usage error", code, missing)
			}
			if !strings.Contains(stderr, missing) {
				t.Errorf("the refusal does not name %s: %q", missing, stderr)
			}
		})
	}
}

// A combination the catalog says cannot work is refused before anything spawns,
// because an unsupported pairing discovered forty minutes into a paid cell is a
// cell wasted.
func TestProbeRefusesAnUnresolvableJobBeforeSpawningAnything(t *testing.T) {
	w := newProbeWorld(t)
	for _, tc := range []struct{ name, flag, value string }{
		{"unknown repo", "-repo", "nowhere"},
		{"unknown agent", "-agent", "nothing"},
		{"unknown model", "-model", "nobody"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := runProbe(t, w.args(tc.flag, tc.value))
			if code != exitError {
				t.Errorf("exit %d, want an error", code)
			}
			if stderr == "" {
				t.Error("the refusal was silent")
			}
			if _, err := os.Stat(w.out); err == nil {
				t.Error("a cell directory was created for a job that cannot run")
			}
		})
	}
}

// The ceiling is the one thing in this binary that stops money being spent, and
// this is the only command that spends: a cell that would take a repository
// past its lifetime's paid runs is refused before anything spawns.
//
// It answers with the refusal code rather than the error code. "This repository
// has spent its budget" and "the binary broke" send whoever is reading to
// opposite places.
func TestACellPastTheRepositorysSpendCeilingIsRefusedBeforeSpawning(t *testing.T) {
	w := newProbeWorld(t)
	for i := 0; i < defaultCeiling; i++ {
		paid := filepath.Join(w.runs, "probe-repo", "1", "bench", fmt.Sprintf("cell-%d", i), "sense", "raw")
		if err := os.MkdirAll(paid, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	code, _, stderr := runProbe(t, w.args())

	if code != exitRefused {
		t.Errorf("exit %d, want the refusal code %d", code, exitRefused)
	}
	if !strings.Contains(stderr, "ceiling") {
		t.Errorf("the refusal does not name the ceiling: %q", stderr)
	}
	if _, err := os.Stat(w.out); err == nil {
		t.Error("a cell directory was created past the ceiling")
	}
}

// The spend that counts is this repository's. A neighbour that burned its whole
// budget is a fact about that neighbour, and a ceiling read across both would
// stop a repository that has spent nothing.
func TestAnotherRepositorysSpendDoesNotCountAgainstThisOne(t *testing.T) {
	w := newProbeWorld(t)
	for i := 0; i < defaultCeiling; i++ {
		paid := filepath.Join(w.runs, "other-repo", "1", "bench", fmt.Sprintf("cell-%d", i), "sense", "raw")
		if err := os.MkdirAll(paid, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// It gets past the ceiling and stops at the next refusal, which is the
	// checkout: what is being asserted is that the ceiling did not fire.
	code, _, stderr := runProbe(t, w.args("-checkout", filepath.Join(t.TempDir(), "nowhere")))

	if code == exitRefused || strings.Contains(stderr, "ceiling") {
		t.Errorf("exit %d: another repository's spend stopped this one: %q", code, stderr)
	}
}

func TestProbeRefusesAScenarioItCannotRead(t *testing.T) {
	w := newProbeWorld(t)
	code, _, stderr := runProbe(t, w.args("-scenario", filepath.Join(t.TempDir(), "nowhere.yaml")))
	if code != exitError {
		t.Errorf("exit %d, want an error", code)
	}
	if stderr == "" {
		t.Error("the refusal was silent")
	}
}

func TestProbeRefusesACheckoutThatIsNotThere(t *testing.T) {
	w := newProbeWorld(t)
	code, _, stderr := runProbe(t, w.args("-checkout", filepath.Join(t.TempDir(), "nowhere")))
	if code != exitError {
		t.Errorf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "checkout") {
		t.Errorf("the refusal does not name the checkout: %q", stderr)
	}
}

// The baseline's wall derives from what the sense arm SPENDS rather than being
// chosen beside it, so before anything spends the header can only state a
// ceiling for it — and it has to read as a ceiling, or somebody plans a session
// around a number the baseline will not get.
func TestProbeStatesBothWallsBeforeItSpends(t *testing.T) {
	w := newProbeWorld(t)
	_, stdout, _ := runProbe(t, w.args())
	for _, want := range []string{"20s sense", "at most 24s baseline"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the header does not state %q:\n%s", want, stdout)
		}
	}
}

// A Sound() failure that is not "the sense arm ignored Sense". The pair ran,
// both arms produced answers, and the baseline named a Sense tool — which makes
// it not a control, whatever the numbers say.
func TestABaselineThatNamedASenseToolIsRefused(t *testing.T) {
	w := newProbeWorld(t)
	// The agent claims a Sense tool call whether or not a server was registered,
	// so the baseline arm carries one too.
	writeScript(t, filepath.Join(filepath.Dir(w.senseBin), "fake-agent"), `#!/bin/sh
cat > /dev/null
if [ -f .mcp.json ]; then
  flat=$(tr -d ' \n' < .mcp.json)
  server=$(printf '%s' "$flat" | sed 's/.*"command":"\([^"]*\)".*/\1/')
  args=$(printf '%s' "$flat" | sed 's/.*"args":\[\([^]]*\)\].*/\1/' | tr -d '"' | tr ',' ' ')
  printf '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"sense_graph"}}\n' | $server $args > /dev/null
fi
printf '{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__sense__sense_graph"}]}}\n'
printf '{"type":"result","result":"Category has three dependents"}\n'
`)
	code, stdout, stderr := runProbe(t, w.args())
	if code != exitRefused {
		t.Fatalf("exit %d, want the refusal code\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "NOT A MEASUREMENT") {
		t.Errorf("the pair was not refused:\n%s", stdout)
	}
	// The report names WHICH check failed, so the reader is not left to guess
	// between six of them.
	if !strings.Contains(stdout, "Sense in the baseline") {
		t.Errorf("the report does not name the failing check:\n%s", stdout)
	}
	if strings.Contains(stdout, "Sense in the baseline  no sign of it") {
		t.Errorf("the failing check was reported clean:\n%s", stdout)
	}
}

// A cell that could not be produced at all is an error, not a refusal: "the
// arms were not comparable" and "the pair never ran" are opposite situations
// for whoever is reading, and only one of them cost money.
func TestACellThatCannotBeProducedIsAnErrorRatherThanARefusal(t *testing.T) {
	w := newProbeWorld(t)
	writeScript(t, w.senseBin, `#!/bin/sh
echo "the index could not be built" >&2
exit 1
`)
	code, stdout, stderr := runProbe(t, w.args())
	if code != exitError {
		t.Fatalf("exit %d, want an error\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "NOT A MEASUREMENT") && stderr == "" {
		t.Error("the failure was silent")
	}
}

// The failure that cost a wall per arm to discover: an arm handed a capture
// shim that answers nothing sits until its budget expires. If this binary
// cannot be located, that is refused before anything spawns.
func TestAProbeThatCannotLocateItsOwnBinaryRefusesBeforeSpawning(t *testing.T) {
	w := newProbeWorld(t)
	was := labBinary
	labBinary = func() (string, error) { return "", errors.New("no such process image") }
	t.Cleanup(func() { labBinary = was })

	code, _, stderr := runProbe(t, w.args())
	if code != exitError {
		t.Fatalf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "capture shim") {
		t.Errorf("the refusal does not say what the binary is needed for: %q", stderr)
	}
	if _, err := os.Stat(w.out); err == nil {
		t.Error("a cell directory was created for a probe that could not run")
	}
}

func TestProbeRejectsAFlagItDoesNotHave(t *testing.T) {
	w := newProbeWorld(t)
	if code, _, _ := runProbe(t, w.args("-subject", "sense-main")); code != exitUsage {
		t.Errorf("exit %d, want a usage error: probe takes no -subject, the arms are the subjects", code)
	}
}

// The failure that produced two empty arms: the disposable HOME carries no
// credential of its own, there was none in the operator's store this run could
// read, and none in the environment to pass through. The executor's
// preserves_auth says what CAN survive isolation, not that anything is there to
// survive.
func TestACellWithNoCredentialAnywhereIsRefusedBeforeSpawning(t *testing.T) {
	w := newProbeWorld(t)
	for _, name := range isolate.Credentials() {
		t.Setenv(name, "")
	}

	code, _, stderr := runProbe(t, w.args())
	if code != exitError {
		t.Fatalf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "no credential") {
		t.Errorf("the refusal does not say what is missing: %q", stderr)
	}
	if !strings.Contains(stderr, "empty runs") {
		t.Errorf("the refusal does not say what would happen: %q", stderr)
	}
	if _, err := os.Stat(w.out); err == nil {
		t.Error("a cell directory was created for a session that could not authenticate")
	}
}

// One is enough: the arms need a credential, not a particular one.
func TestAnyOneCredentialSatisfiesTheCheck(t *testing.T) {
	for _, name := range isolate.Credentials() {
		t.Run(name, func(t *testing.T) {
			w := newProbeWorld(t)
			for _, other := range isolate.Credentials() {
				t.Setenv(other, "")
			}
			t.Setenv(name, "a-value")

			code, stdout, stderr := runProbe(t, w.args())
			if code == exitError && strings.Contains(stderr, "no credential") {
				t.Errorf("%s did not satisfy the credential check:\n%s\n%s", name, stdout, stderr)
			}
		})
	}
}

// hostHolding makes the operator's store answer with the given credential, and
// restores the real read afterwards. It is how a seat-based host is stated
// without a login: CI has none and can never be given one.
func hostHolding(t *testing.T, c isolate.Credential, err error) {
	t.Helper()
	was := hostCredential
	t.Cleanup(func() { hostCredential = was })
	hostCredential = func(context.Context, func(string) (string, bool), isolate.Route) (isolate.Credential, error) {
		return c, err
	}
}

// aSeat is a credential good for the given span from now.
func aSeat(good time.Duration) isolate.Credential {
	return isolate.Credential{
		Fields: map[string]json.RawMessage{
			"fakeOauth.accessToken": json.RawMessage(`"seat-token"`),
			"fakeOauth.scopes":      json.RawMessage(`["user:inference"]`),
		},
		ExpiresAt: time.Now().Add(good).UnixMilli(),
	}
}

// The half-pair hazard arriving through the credential. A token that outlives
// the sense arm and dies during the baseline burns the finished arm: it is paid
// for and can never be paired, because the baseline's budget derives from the
// sense arm it ran with. Presence is not the question; lasting the whole cell is.
func TestACredentialThatDiesPartWayThroughTheCellIsRefusedBeforeSpawning(t *testing.T) {
	w := newProbeWorld(t)
	for _, name := range isolate.Credentials() {
		t.Setenv(name, "")
	}
	// Good for one wall, and the cell needs two: the arms run in sequence.
	hostHolding(t, aSeat(90*time.Second), nil)
	w.declareCredentialRoute(t)

	code, _, stderr := runProbe(t, w.args("-wall", "60s"))
	if code != exitError {
		t.Fatalf("exit %d, want an error", code)
	}
	if !strings.Contains(stderr, "expires") {
		t.Errorf("the refusal names something other than expiry: %q", stderr)
	}
	if strings.Contains(stderr, "no credential") {
		t.Errorf("an expiring credential was reported as an absent one: %q", stderr)
	}
	if _, err := os.Stat(w.out); err == nil {
		t.Error("a cell directory was created for a credential that could not last the cell")
	}
}

func TestACredentialThatOutlivesBothWallsIsAccepted(t *testing.T) {
	w := newProbeWorld(t)
	for _, name := range isolate.Credentials() {
		t.Setenv(name, "")
	}
	hostHolding(t, aSeat(time.Hour), nil)
	w.declareCredentialRoute(t)

	code, stdout, stderr := runProbe(t, w.args("-wall", "60s"))
	if code == exitError {
		t.Fatalf("a credential good for the whole cell was refused:\n%s\n%s", stdout, stderr)
	}
}

// The seat and the key are two doors, and a host with the key needs neither the
// store nor an expiry it cannot read.
func TestAKeyBasedHostRunsWithNoStoreAtAll(t *testing.T) {
	w := newProbeWorld(t)
	hostHolding(t, isolate.Credential{}, errors.New("no store on this machine"))
	t.Setenv("ANTHROPIC_API_KEY", "a-test-key")

	code, stdout, stderr := runProbe(t, w.args())
	if code == exitError {
		t.Fatalf("a key-based host was refused for having no store:\n%s\n%s", stdout, stderr)
	}
}

// Every arm of a seat-based cell is handed the credential, and nothing else from
// the operator's store reaches it. This is the positive-space check cycle 03's
// test set never had: every other isolation test asks what an arm CANNOT reach.
//
// Read off what the SESSION saw rather than off the file, because the file is
// gone by the time the cell returns — release takes the credential and keeps the
// evidence, which is the other half of this pitch.
func TestBothArmsAreProvisionedWithTheCredentialAndNothingElse(t *testing.T) {
	w := newProbeWorld(t)
	for _, name := range isolate.Credentials() {
		t.Setenv(name, "")
	}
	hostHolding(t, aSeat(time.Hour), nil)
	w.declareCredentialRoute(t)

	if code, stdout, stderr := runProbe(t, w.args()); code == exitError {
		t.Fatalf("exit %d:\n%s\n%s", code, stdout, stderr)
	}

	for _, arm := range []string{"sense", "baseline"} {
		said := armTranscript(t, w.out, arm)
		fields := fieldLine(said, "CRED_FIELDS=")
		if fields == "" {
			t.Fatalf("the %s arm read no credential:\n%s", arm, said)
		}
		want := "fakeOauth accessToken scopes"
		if got := strings.Join(strings.Fields(fields), " "); got != want {
			t.Errorf("the %s arm's credential carries %q, want %q", arm, got, want)
		}
	}
}

// The session is pointed at that credential by the tool's own variable, taken
// from the catalog. Without it the file is written where nothing reads it, and
// the arm exits in a second reading as a model with nothing to say — which is
// the shape of the failure this pitch was written for.
func TestTheSessionIsPointedAtItsOwnConfigDirectory(t *testing.T) {
	w := newProbeWorld(t)
	for _, name := range isolate.Credentials() {
		t.Setenv(name, "")
	}
	hostHolding(t, aSeat(time.Hour), nil)
	w.declareCredentialRoute(t)

	if code, stdout, stderr := runProbe(t, w.args()); code == exitError {
		t.Fatalf("exit %d:\n%s\n%s", code, stdout, stderr)
	}

	for _, arm := range []string{"sense", "baseline"} {
		want := filepath.Join(w.out, arm, "config")
		if got := fieldLine(armTranscript(t, w.out, arm), "CONFIG_DIR="); got != want {
			t.Errorf("the %s arm saw config directory %q, want %q", arm, got, want)
		}
	}
}

// Nothing the cell keeps may carry the token. A cell directory is read back for
// months by the scorer, the miner and status, so this walks all of it — the run
// records, the transcripts, the MCP capture and the cell record — rather than
// the one file a test could pick knowing where the token is.
func TestNothingTheCellKeepsCarriesTheToken(t *testing.T) {
	w := newProbeWorld(t)
	for _, name := range isolate.Credentials() {
		t.Setenv(name, "")
	}
	hostHolding(t, aSeat(time.Hour), nil)
	w.declareCredentialRoute(t)

	if code, stdout, stderr := runProbe(t, w.args()); code == exitError {
		t.Fatalf("exit %d:\n%s\n%s", code, stdout, stderr)
	}

	if err := filepath.WalkDir(w.out, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil //nolint:nilerr // an unreadable entry is not a finding
		}
		b, readErr := os.ReadFile(path) // #nosec G304 -- the test's own cell directory
		if readErr == nil && strings.Contains(string(b), "seat-token") {
			rel, _ := filepath.Rel(w.out, path)
			t.Errorf("%s carries the credential, in a directory kept for months", rel)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk the cell: %v", err)
	}
}

// armTranscript is what one arm said, which survives release.
func armTranscript(t *testing.T, cell, arm string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(cell, arm, "session", "raw", "stdout"))
	if err != nil {
		t.Fatalf("read the %s arm's transcript: %v", arm, err)
	}
	return string(b)
}

// fieldLine is the value the stand-in reported for one key.
func fieldLine(said, key string) string {
	for _, line := range strings.Split(said, "\n") {
		if v, ok := strings.CutPrefix(line, key); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// recordingAgentScript is the stand-in agent, plus two lines: it reports the
// config directory it was pointed at and the FIELD NAMES of the credential it
// found there.
//
// The names rather than the values, deliberately. A stand-in that echoed the
// token would put it in a transcript that is kept for months, which is the thing
// this pitch forbids — and it would make the test that checks for that pass by
// accident of its own setup.
const recordingAgentScript = `#!/bin/sh
printf 'CONFIG_DIR=%s\n' "$FAKE_CONFIG_DIR"
printf 'CRED_FIELDS=%s\n' "$(grep -o '"[a-zA-Z]*":' "$FAKE_CONFIG_DIR/.credentials.json" | tr -d '":' | tr '\n' ' ')"
` + fakeAgentScript

// declareCredentialRoute rewrites the world's agent to declare a credential
// route, the way the shipped catalog does: a config variable, a store item and
// the key the credential lives under. Without one, an agent authenticates by
// key and no file is provisioned at all.
func (w probeWorld) declareCredentialRoute(t *testing.T) {
	t.Helper()
	bin := writeScript(t, filepath.Join(t.TempDir(), "recording-agent"), recordingAgentScript)
	at := filepath.Join(w.config, "agents", "fake", "agent.json")
	body := `{"id":"fake","binary":"` + bin + `","setup_tool":"fake-cli",` +
		`"transcript_format":"assistant-events","model_flag":"--model",` +
		`"mcp_registration":{"file":".mcp.json","servers_key":"mcpServers","command_style":"command+args"},` +
		`"config_dirs":[".fake"],"config_dir_var":"FAKE_CONFIG_DIR",` +
		`"keychain_service":"Fake-credentials","credential_file":".credentials.json",` +
		`"credential_fields":["fakeOauth.accessToken","fakeOauth.scopes"],` +
		`"credential_expiry":"ms:fakeOauth.expiresAt",` +
		`"headless_args":["-c"],"judge_args":["-c"],"env":[],"supports_mcp":true,` +
		`"auth_modes":["subscription","api_key"]}`
	if err := os.WriteFile(at, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A model whose provider key is not among the fields the run would be given is
// refused before anything spawns.
//
// The failure it prevents is unreadable rather than merely expensive: the tool
// answers with `UnknownError: Unexpected server error`, which is the same
// message asking for a model that does not exist produces. Measured on two arms
// of a real cell, where the model id was correct and the obvious reading was
// that it was wrong.
func TestAModelWithNoCredentialInTheRunsLoginIsRefusedBeforeSpawning(t *testing.T) {
	agent := catalog.Agent{
		ID: "opencode", CredentialFields: []string{"kimi-for-coding.type", "kimi-for-coding.key"},
	}

	err := carriesThisModel(agent, catalog.Model{ID: "ollama-cloud/glm-5.2", CredentialKey: "ollama-cloud"})

	if err == nil {
		t.Fatal("a cell whose run would carry no key for its model was accepted")
	}
	for _, want := range []string{"ollama-cloud", "credential_fields", "opencode"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not name %q: %v", want, err)
		}
	}
}

func TestAModelWhoseKeyIsCarriedIsAccepted(t *testing.T) {
	agent := catalog.Agent{
		ID: "opencode", CredentialFields: []string{"kimi-for-coding.type", "kimi-for-coding.key",
			"ollama-cloud.type", "ollama-cloud.key"},
	}

	if err := carriesThisModel(agent, catalog.Model{ID: "ollama-cloud/glm-5.2", CredentialKey: "ollama-cloud"}); err != nil {
		t.Fatalf("a cell whose run carries its model's key was refused: %v", err)
	}
}

// A tool whose login is not keyed by provider has nothing to check, and
// inventing a check for it would refuse every cell on that tool.
func TestAModelOnAToolWhoseLoginIsNotPerProviderIsNotChecked(t *testing.T) {
	agent := catalog.Agent{ID: "claude-code", CredentialFields: []string{"claudeAiOauth.accessToken"}}

	if err := carriesThisModel(agent, catalog.Model{ID: "claude-opus-5"}); err != nil {
		t.Fatalf("a model on a tool with no per-provider login was refused: %v", err)
	}
}

// A key that is a prefix of another must not satisfy the check: `ollama` is not
// `ollama-cloud`, and a run given the first would die exactly as before.
func TestAKeyThatMerelySharesAPrefixDoesNotSatisfyTheCheck(t *testing.T) {
	agent := catalog.Agent{ID: "opencode", CredentialFields: []string{"ollama-cloud-eu.key"}}

	if err := carriesThisModel(agent, catalog.Model{ID: "x", CredentialKey: "ollama-cloud"}); err == nil {
		t.Fatal("a different provider whose name starts the same was accepted")
	}
}

// The Sense binary is resolved before it is handed to anything that runs
// somewhere else. `sense setup` runs in the arm's own worktree and the shadow
// PATH symlinks to it from the arm's own bin, so a path relative to where the
// lab was invoked names a file that is not there.
func TestTheSenseBinaryIsResolvedBeforeItIsHandedOn(t *testing.T) {
	for _, tc := range []struct {
		name string
		bin  string
		want func(string) bool
	}{
		{"a path the caller typed", filepath.Join(".", "bin", "sense"), filepath.IsAbs},
		{"a name on PATH", "sense", func(got string) bool { return got == "sense" }},
		{"a path that is already absolute", filepath.Join(string(filepath.Separator), "opt", "sense"),
			func(got string) bool { return got == filepath.Join(string(filepath.Separator), "opt", "sense") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := senseBinary(tc.bin)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.want(got) {
				t.Errorf("senseBinary(%q) = %q", tc.bin, got)
			}
		})
	}
}

// The arms are handed the resolved binary, read off the spec they will run
// with rather than off the flag that was typed.
func TestTheArmsAreHandedTheResolvedSenseBinary(t *testing.T) {
	w := newProbeWorld(t)
	rel, err := filepath.Rel(mustGetwd(t), w.senseBin)
	if err != nil {
		t.Skipf("no relative path to the stand-in binary: %v", err)
	}

	f, err := parseProbeFlags(w.args("-sense", rel)[1:], io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	s, _, err := probeSpec(context.Background(), f)
	if err != nil {
		t.Fatal(err)
	}

	if !filepath.IsAbs(s.SenseBin) {
		t.Errorf("the arms would be handed %q, which nothing running in a worktree can reach", s.SenseBin)
	}
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
