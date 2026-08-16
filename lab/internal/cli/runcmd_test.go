package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

	code, stdout, stderr := dispatch(t, "run",
		"-scenario", scenarioFile(t, twoStepScenario),
		"-repo", t.TempDir(),
		"-out", out,
		"-agent", fakeAgent(t, "0", ""),
	)

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

	code, stdout, _ := dispatch(t, "run",
		"-scenario", scenarioFile(t, twoStepScenario),
		"-repo", t.TempDir(),
		"-out", out,
		"-agent", fakeAgent(t, "0", "sleep 60"),
		"-wall", "300ms",
	)

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
	code, stdout, _ := dispatch(t, "run",
		"-scenario", scenarioFile(t, twoStepScenario),
		"-repo", t.TempDir(),
		"-out", filepath.Join(t.TempDir(), "run-1"),
		"-agent", fakeAgent(t, "7", ""),
	)

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
	good := scenarioFile(t, twoStepScenario)
	agent := fakeAgent(t, "0", "")

	// The code matters as much as the rejection: 2 says you typed it wrong, 1
	// says the run could not proceed. A caller script branches on that.
	for _, tc := range []struct {
		name     string
		args     []string
		want     string
		wantCode int
	}{
		{
			name: "no scenario", wantCode: 2,
			args: []string{"-repo", t.TempDir(), "-out", t.TempDir()},
			want: "-scenario is required",
		},
		{
			name: "no repo", wantCode: 2,
			args: []string{"-scenario", good, "-out", t.TempDir()},
			want: "-repo is required",
		},
		{
			name: "no out", wantCode: 2,
			args: []string{"-scenario", good, "-repo", t.TempDir()},
			want: "-out is required",
		},
		{
			name: "an unknown flag", wantCode: 2,
			args: []string{"-scenario", good, "-repo", t.TempDir(), "-out", t.TempDir(), "-model", "x"},
			want: "flag provided but not defined",
		},
		{
			name: "a scenario that does not exist", wantCode: 1,
			args: []string{"-scenario", "/no/such/scenario.yaml", "-repo", t.TempDir(), "-out", t.TempDir()},
			want: "read scenario",
		},
		{
			name: "a repository that does not exist", wantCode: 1,
			args: []string{"-scenario", good, "-repo", "/no/such/repo", "-out", t.TempDir()},
			want: "repository",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := dispatch(t, append([]string{"run", "-agent", agent}, tc.args...)...)

			if code != tc.wantCode {
				t.Errorf("exit = %d, want %d", code, tc.wantCode)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want it to name %q", stderr, tc.want)
			}
		})
	}
}

// The session must run inside the repository under study, not wherever the
// binary happened to be invoked: an agent pointed at the wrong tree produces a
// plausible answer about the wrong code.
func TestTheAgentRunsInsideTheRepository(t *testing.T) {
	repo := t.TempDir()
	out := filepath.Join(t.TempDir(), "run-1")
	agent := filepath.Join(t.TempDir(), "fake-agent")
	if err := os.WriteFile(agent, []byte("#!/bin/sh\ncat >/dev/null\npwd\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if code, _, stderr := dispatch(t, "run",
		"-scenario", scenarioFile(t, twoStepScenario),
		"-repo", repo, "-out", out, "-agent", agent,
	); code != 0 {
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

	code, _, stderr := dispatch(t, "run",
		"-scenario", scenarioFile(t, twoStepScenario),
		"-repo", t.TempDir(),
		"-out", filepath.Join(blocked, "run-1"),
		"-agent", fakeAgent(t, "0", ""),
	)

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
	if code, _, stderr := dispatch(t, "run",
		"-scenario", scenarioFile(t, twoStepScenario),
		"-repo", t.TempDir(), "-out", out, "-agent", agent,
	); code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}

	gotArgs := read(t, argsFile)
	for _, want := range []string{"-p", "--permission-mode", "bypassPermissions"} {
		if !strings.Contains(gotArgs, want) {
			t.Errorf("the agent was not given %q:\n%s", want, gotArgs)
		}
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
