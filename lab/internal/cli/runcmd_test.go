package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/catalog"
)

// These tests drive the real `sense-lab run` path end to end against a fake
// agent: a shell script that behaves the way an agent CLI does. Nothing here
// calls a model, touches the network, or spends anything.

const twoStepScenario = `
name: audit the category contract
repo: discourse
description: You are a maintainer about to rework a class.
steps:
  - name: Map the contract
    prompt: Trace the resolution path end to end.
  - name: Audit the dependents
    prompt: Find everywhere that would have to change with it.
`

// testCatalog writes a config directory whose one agent spawns `bin` and whose
// one model that agent can drive. The run tests care about the session, not the
// catalog, so this keeps their flag lists to what they are actually varying.
func testCatalog(t *testing.T, bin string) string { return testCatalogPinned(t, bin, "abc123") }

func testCatalogPinned(t *testing.T, bin, pin string) string {
	t.Helper()
	dir := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "agents", "tool", "agent.json"), `{"id":"tool","binary":"`+bin+`",
	  "model_flag":"--model","headless_args":["-p","--permission-mode","bypassPermissions"],
	  "env":["IS_SANDBOX=1","CLAUDE_CODE_DISABLE_AUTO_MEMORY=1"],
	  "supports_mcp":true,"auth_modes":["api_key"]}`)
	write(filepath.Join(dir, "subjects", "untreated", "subject.json"),
		`{"id":"untreated","kind":"baseline","executor":"isolated-home","agents":["tool"]}`)
	write(filepath.Join(dir, "executors", "isolated-home.json"),
		`{"id":"isolated-home","preserves_auth":["api_key"],"isolates_global_config":true}`)
	write(filepath.Join(dir, "models", "m1.json"),
		`{"id":"m1","provider":"acme","available_under":["api_key"],"agents":["tool"]}`)
	write(filepath.Join(dir, "repos", "r1.json"),
		`{"id":"r1","url":"https://example.test/r1.git","commit":"`+pin+`","languages":["go"]}`)
	return dir
}

// gitCheckout makes a real one-commit repository and returns its path and HEAD,
// so a test can drive a run whose checkout genuinely matches its pin.
func gitCheckout(t *testing.T) (dir, head string) {
	t.Helper()
	dir = t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return dir, strings.TrimSpace(string(out))
}

// runArgs is the flag list for a run against the test catalog, with the caller's
// own flags appended.
func runArgs(t *testing.T, bin, out string, extra ...string) []string {
	t.Helper()
	checkout, head := gitCheckout(t)
	return runArgsIn(t, bin, out, checkout, head, extra...)
}

// runArgsIn is runArgs against a checkout the caller made, so a test that cares
// where the agent ran can supply it. The pin must match that tree: a run
// against a checkout the catalog does not pin is refused.
func runArgsIn(t *testing.T, bin, out, checkout, head string, extra ...string) []string {
	t.Helper()
	return append([]string{"run",
		"-config", testCatalogPinned(t, bin, head),
		"-scenario", scenarioFile(t, twoStepScenario),
		"-repo", "r1", "-checkout", checkout, "-out", out,
		"-agent", "tool", "-model", "m1",
	}, extra...)
}

// fakeAgent writes an executable script that copies its stdin to stdout, so a
// test can assert what the prompt actually looked like on the other side, then
// exits with the code it was built for.
func fakeAgent(t *testing.T, exitCode string, extra string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake-agent")
	body := "#!/bin/sh\ncat\n" + extra + "\nexit " + exitCode + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func scenarioFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scenario.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunDrivesTheAgentAndLeavesARunDirectory(t *testing.T) {
	out := filepath.Join(t.TempDir(), "run-1")

	code, stdout, stderr := dispatch(t, runArgs(t, fakeAgent(t, "0", ""), out)...)

	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "audit the category contract") {
		t.Errorf("stdout does not name the scenario:\n%s", stdout)
	}

	// The prompt the agent actually received, recovered from the capture.
	raw, err := os.ReadFile(filepath.Join(out, "raw", "stdout"))
	if err != nil {
		t.Fatalf("raw/stdout: %v", err)
	}
	for _, want := range []string{
		"You are a maintainer about to rework a class.",
		"Step 1: Map the contract",
		"Step 2: Audit the dependents",
	} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the agent never received %q:\n%s", want, raw)
		}
	}

	b, err := os.ReadFile(filepath.Join(out, "run-meta.json"))
	if err != nil {
		t.Fatalf("run-meta.json: %v", err)
	}
	var m struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode run-meta.json: %v", err)
	}
	if m.Outcome != "completed" {
		t.Errorf("outcome = %q, want completed", m.Outcome)
	}
}

// A run that could not finish inside its budget is a result. Reporting success
// would let a stalled arm pass unnoticed, which is exactly how a wall gets
// raised to rescue one.
func TestARunThatOverrunsItsWallExitsNonZero(t *testing.T) {
	out := filepath.Join(t.TempDir(), "run-1")

	code, stdout, _ := dispatch(t, runArgs(t, fakeAgent(t, "0", "sleep 60"), out, "-wall", "300ms")...)

	// Its own code, distinct from a failed agent and from a broken binary: a
	// run that hit its wall left a record on disk and is bankable as a result.
	if code != 3 {
		t.Errorf("exit = %d, want 3 for a run that hit its wall", code)
	}
	if !strings.Contains(stdout, "cannot finish at budget") {
		t.Errorf("stdout does not report the wall:\n%s", stdout)
	}
}

func TestAnAgentThatFailsExitsNonZero(t *testing.T) {
	code, stdout, _ := dispatch(t, runArgs(t, fakeAgent(t, "7", ""), filepath.Join(t.TempDir(), "run-1"))...)

	if code != 1 {
		t.Errorf("exit = %d, want 1 for an agent that exited 7", code)
	}
	if !strings.Contains(stdout, "failed") {
		t.Errorf("stdout does not report the failure:\n%s", stdout)
	}
}

// Everything below fails before anything is spawned. A run that is going to be
// rejected must be rejected before the agent starts, not after it has spent
// eight minutes.
func TestRunRejectsBadInputBeforeSpawningAnything(t *testing.T) {
	agent := fakeAgent(t, "0", "")
	cfg := testCatalog(t, agent)
	good := scenarioFile(t, twoStepScenario)

	// The code matters as much as the rejection: 2 says you typed it wrong, 1
	// says the run could not proceed. A caller script branches on that.
	full := func(over map[string]string) []string {
		args := map[string]string{
			"-config": cfg, "-scenario": good, "-repo": "r1",
			"-checkout": t.TempDir(), "-out": filepath.Join(t.TempDir(), "run-1"),
			"-agent": "tool", "-model": "m1",
		}
		var out []string
		for k, v := range over {
			if v == "" {
				delete(args, k)
				continue
			}
			args[k] = v
		}
		for _, k := range []string{"-config", "-scenario", "-repo", "-checkout", "-out", "-agent", "-model"} {
			if v, ok := args[k]; ok {
				out = append(out, k, v)
			}
		}
		return append([]string{"run"}, out...)
	}

	for _, tc := range []struct {
		name     string
		args     []string
		want     string
		wantCode int
	}{
		{"no scenario", full(map[string]string{"-scenario": ""}), "-scenario is required", 2},
		{"no repo", full(map[string]string{"-repo": ""}), "-repo is required", 2},
		{"no checkout", full(map[string]string{"-checkout": ""}), "-checkout is required", 2},
		{"no out", full(map[string]string{"-out": ""}), "-out is required", 2},
		{"no agent", full(map[string]string{"-agent": ""}), "-agent is required", 2},
		{"no model", full(map[string]string{"-model": ""}), "-model is required", 2},
		{"an unknown flag", append(full(nil), "-arm", "sense"), "flag provided but not defined", 2},
		{"a scenario that does not exist", full(map[string]string{"-scenario": "/no/such.yaml"}), "read scenario", 1},
		{"a checkout that does not exist", full(map[string]string{"-checkout": "/no/such/repo"}), "checkout", 1},

		// The three that only the catalog can catch. Each names what IS
		// available, because the next thing anyone does after a typo is look
		// for the right spelling.
		{"a repo the catalog does not have", full(map[string]string{"-repo": "discourse"}),
			`no repo "discourse" in the catalog; have [r1]`, 1},
		{"an agent the catalog does not have", full(map[string]string{"-agent": "codex"}),
			`no agent "codex" in the catalog; have [tool]`, 1},
		{"a model the catalog does not have", full(map[string]string{"-model": "claude-opus-5"}),
			`no model "claude-opus-5" in the catalog; have [m1]`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := dispatch(t, tc.args...)

			if code != tc.wantCode {
				t.Errorf("exit = %d, want %d (stderr: %s)", code, tc.wantCode, stderr)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want it to name %q", stderr, tc.want)
			}
		})
	}
}

// An unsupported combination must be refused at planning time, not forty
// minutes into a paid run. The catalog knows which agent tools can drive which
// models, so it can say so before anything spawns.
func TestAModelTheAgentCannotDriveIsRefusedBeforeSpawning(t *testing.T) {
	agent := fakeAgent(t, "0", "")
	cfg := testCatalog(t, agent)
	// A second model, declared as drivable only by a tool that is not ours.
	writeFile(t, filepath.Join(cfg, "agents", "other", "agent.json"),
		`{"id":"other","binary":"othertool","model_flag":"-m","headless_args":["-p"],
		  "env":[],"supports_mcp":false,"auth_modes":["api_key"]}`)
	writeFile(t, filepath.Join(cfg, "models", "m2.json"),
		`{"id":"m2","provider":"acme","available_under":["api_key"],"agents":["other"]}`)
	out := filepath.Join(t.TempDir(), "run-1")

	code, _, stderr := dispatch(t, "run", "-config", cfg,
		"-scenario", scenarioFile(t, twoStepScenario), "-repo", "r1",
		"-checkout", t.TempDir(), "-out", out, "-agent", "tool", "-model", "m2")

	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(stderr, "model m2 cannot be driven by tool") {
		t.Errorf("stderr = %q, want it to name the mismatch", stderr)
	}
	if _, err := os.Stat(out); err == nil {
		t.Error("a run directory was created for a job that could never run")
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The session must run inside the repository under study, not wherever the
// binary happened to be invoked: an agent pointed at the wrong tree produces a
// plausible answer about the wrong code.
func TestTheAgentRunsInsideTheRepository(t *testing.T) {
	repo, head := gitCheckout(t)
	out := filepath.Join(t.TempDir(), "run-1")
	agent := filepath.Join(t.TempDir(), "fake-agent")
	if err := os.WriteFile(agent, []byte("#!/bin/sh\ncat >/dev/null\npwd\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if code, _, stderr := dispatch(t, runArgsIn(t, agent, out, repo, head)...); code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}

	raw, err := os.ReadFile(filepath.Join(out, "raw", "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatal(err)
	}
	got, err := filepath.EvalSymlinks(strings.TrimSpace(string(raw)))
	if err != nil {
		t.Fatalf("resolve %q: %v", raw, err)
	}
	if got != want {
		t.Errorf("the agent ran in %q, want the repository %q", got, want)
	}
}

// A run that cannot be recorded must not be reported as a run that happened.
func TestARunThatCannotBeRecordedExitsNonZero(t *testing.T) {
	// -out points inside an existing file, so the run directory cannot be made.
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocked, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := dispatch(t, runArgs(t, fakeAgent(t, "0", ""), filepath.Join(blocked, "run-1"))...)

	if code == 0 {
		t.Error("exit = 0 for a run that could not be written")
	}
	if !strings.Contains(stderr, "prepare run directory") {
		t.Errorf("stderr = %q, want it to name the failure", stderr)
	}
}

// The two literals in runcmd.go ARE the hard-coded run, and both carry a
// contract. --permission-mode bypassPermissions is what stops an unattended
// session stalling on a question until its wall expires, and the env marks the
// session as a sandbox. Nothing else in the suite notices if either is dropped,
// and a flag that silently stops being passed costs a whole run to discover.
func TestTheAgentIsInvokedHeadlessAndUnattended(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "args")
	envFile := filepath.Join(dir, "env")
	agent := filepath.Join(dir, "recording-agent")
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" >" + argsFile + "\nenv >" + envFile + "\ncat\n"
	if err := os.WriteFile(agent, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "run-1")
	if code, _, stderr := dispatch(t, runArgs(t, agent, out)...); code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}

	gotArgs := read(t, argsFile)
	for _, want := range []string{"-p", "--permission-mode", "bypassPermissions"} {
		if !strings.Contains(gotArgs, want) {
			t.Errorf("the agent was not given %q:\n%s", want, gotArgs)
		}
	}

	// THE line this pitch exists for. The measured failure was a model id
	// transformed on its way to the spawn: it resolved to nothing, and every
	// run came back empty with exit 1 rather than crashing. The flag and the id
	// are asserted adjacent and verbatim, so neither dropping the selection nor
	// sending the wrong string can pass.
	if !strings.Contains(gotArgs, "--model\nm1\n") {
		t.Errorf("the model id did not reach the agent verbatim, immediately after its flag:\n%s", gotArgs)
	}

	gotEnv := read(t, envFile)
	for _, want := range []string{"IS_SANDBOX=1", "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1"} {
		if !strings.Contains(gotEnv, want) {
			t.Errorf("the agent did not receive %q", want)
		}
	}
	// The env adds to the host's rather than replacing it: an agent CLI needs
	// PATH and HOME to run at all.
	if !strings.Contains(gotEnv, "PATH=") {
		t.Error("the agent lost the host PATH")
	}

	// Whatever the agent was invoked with must be in the record, or a run
	// cannot be told apart from one made with different flags.
	rec := read(t, filepath.Join(out, "run-meta.json"))
	if !strings.Contains(rec, "bypassPermissions") {
		t.Errorf("run-meta.json does not record the arguments:\n%s", rec)
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// -help on a subcommand is a request, not a mistake: it must not be reported as
// an error on stderr on top of the usage flag package already printed.
func TestRunHelpIsNotReportedAsAnError(t *testing.T) {
	_, _, stderr := dispatch(t, "run", "-help")

	if strings.Contains(stderr, "sense-lab run:") {
		t.Errorf("asking for help produced an error message:\n%s", stderr)
	}
}

// A run against a tree that is not at the pinned commit records a commit it did
// not use. Nothing about the result would show it, so every number taken that
// way is quietly unreproducible — the same class of failure as an arm whose
// model resolved to nothing.
func TestACheckoutThatIsNotAtThePinIsRefused(t *testing.T) {
	agent := fakeAgent(t, "0", "")
	checkout, _ := gitCheckout(t)
	out := filepath.Join(t.TempDir(), "run-1")

	t.Run("a different commit", func(t *testing.T) {
		// The catalog pins something this tree is not at.
		code, _, stderr := dispatch(t, runArgsIn(t, agent, out, checkout,
			"0000000000000000000000000000000000000000")...)

		if code != 1 {
			t.Errorf("exit = %d, want 1", code)
		}
		if !strings.Contains(stderr, "records a commit it did not use") {
			t.Errorf("stderr = %q, want it to say why this matters", stderr)
		}
		if _, err := os.Stat(out); err == nil {
			t.Error("a run directory was created for a checkout that was refused")
		}
	})

	t.Run("a directory that is not a repository at all", func(t *testing.T) {
		code, _, stderr := dispatch(t, runArgsIn(t, agent, filepath.Join(t.TempDir(), "r"),
			t.TempDir(), "abc123")...)

		if code != 1 {
			t.Errorf("exit = %d, want 1", code)
		}
		if !strings.Contains(stderr, "cannot read its commit") {
			t.Errorf("stderr = %q, want it to say the commit is unreadable", stderr)
		}
	})
}

// The shipped claude-code config must carry the headless contract. The rest of
// the suite drives a fixture agent, so a flag deleted from the real file would
// otherwise pass every test and cost a run to discover — which is exactly what
// this test's fixture-based sibling stopped catching when the flags moved into
// config.
func TestTheShippedClaudeAgentCarriesTheHeadlessContract(t *testing.T) {
	c, err := catalog.Load(repoConfigDir(t))
	if err != nil {
		t.Fatalf("load the shipped catalog: %v", err)
	}
	a, ok := c.Agents["claude-code"]
	if !ok {
		t.Fatal("the shipped catalog has no claude-code agent")
	}

	args := strings.Join(a.HeadlessArgs, " ")
	for _, want := range []string{
		"-p",                                  // the prompt comes on stdin
		"--permission-mode bypassPermissions", // nobody is at the terminal to answer
	} {
		if !strings.Contains(args, want) {
			t.Errorf("headless_args = %q, missing %q", args, want)
		}
	}
	env := strings.Join(a.Env, " ")
	for _, want := range []string{"IS_SANDBOX=1", "CLAUDE_CODE_DISABLE_AUTO_MEMORY=1"} {
		if !strings.Contains(env, want) {
			t.Errorf("env = %q, missing %q", env, want)
		}
	}
	if a.ModelFlag == "" {
		t.Error("the shipped agent has no model flag, so no arm could select a model")
	}
}
