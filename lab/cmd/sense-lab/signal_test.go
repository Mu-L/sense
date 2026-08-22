package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/position"
)

// Installing a signal handler and attaching the run to it are two different
// things, and getting only the first is silent: the handler fires, the session
// is not attached to it, and an interrupted run leaves the agent running,
// unowned and still spending, with no record on disk. Because the session sits
// in its own process group, a second Ctrl-C cannot reach it either.
//
// This drives the real binary rather than the cli package, because signalling
// the test process would pollute handler state for every other test in it, and
// the production path is the thing worth testing.
//
// It drives `pay`, which is where the hazard costs money. It used to drive
// `run`, a command that spawned a single unisolated session and has been
// deleted; what it proved about that command is true of every command that
// spawns, and the paid cell is the one where an agent left running is spending
// on an arm nobody will ever pair.
func TestSIGTERMRecordsTheRunAndKillsTheAgentTree(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and spawns the binary")
	}

	dir := t.TempDir()
	bin := filepath.Join(dir, "sense-lab")
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build sense-lab: %v\n%s", err, out)
	}

	marker := filepath.Join(dir, "grandchild-still-running")
	agent := filepath.Join(dir, "fake-agent")
	// Reads the prompt, backgrounds a grandchild that would fire well after the
	// signal, then waits. Killing only the direct child leaves the grandchild.
	script := "#!/bin/sh\ncat >/dev/null\necho started\n(sleep 5; touch " + marker + ") &\nwait\n"
	if err := os.WriteFile(agent, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// A scenario is three files.
	scenario := filepath.Join(dir, "scenario.yaml")
	for name, body := range map[string]string{
		"scenario.yaml":        "name: t\nrepo: r1\nsteps:\n  - name: s\n    prompt: go\n",
		"scenario.gold.yaml":   "discriminator: dependents\nrows:\n  - id: d:one\n    group: dependents\n    relation: \"a.rb:1 the thing\"\n",
		"scenario.rubric.yaml": "audience: An agent.\nsteps:\n  - name: s\n    criteria:\n      q:\n        weight: 1.0\n        question: Any good?\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A Sense stand-in that is not the agent. Handing the agent script to
	// `-sense` would have the channel probe run it, and its grandchild would
	// touch the marker before the arm this test interrupts had even started.
	sense := filepath.Join(dir, "fake-sense")
	if err := os.WriteFile(sense, []byte(senseStandInScript), 0o755); err != nil {
		t.Fatal(err)
	}
	head := initRepo(t, dir)
	cfg := writeCatalog(t, dir, agent, head)
	runs := payable(t, dir, cfg, scenario)
	// A key the arms can be given. The stand-in agent never uses it; what it
	// satisfies is the check that refuses a cell whose arms could not
	// authenticate, which would otherwise be what this test exercised.
	t.Setenv("ANTHROPIC_API_KEY", "a-test-key")

	// Where the sense arm of the first cell lands, which is the session this
	// interrupt has to reach.
	out := filepath.Join(runs, "r1", "1", "bench", "cell", "sense", "session")
	lab := exec.Command(bin, "pay", "-config", cfg, "-runs", runs,
		"-checkouts", filepath.Dir(dir), "-sense", sense, "-wall", "5m", "-yes", "r1")
	var labOut strings.Builder
	lab.Stdout, lab.Stderr = &labOut, &labOut
	if err := lab.Start(); err != nil {
		t.Fatalf("start sense-lab: %v", err)
	}

	// Wait until the agent has actually started before interrupting. A fixed
	// sleep races the first exec of a freshly built binary, and signalling too
	// early tests the default handler rather than ours.
	waitFor(t, func() bool {
		b, err := os.ReadFile(filepath.Join(out, "raw", "stdout"))
		return err == nil && strings.Contains(string(b), "started")
	}, "the agent to start")

	// Interrupt the way the crank would.
	if err := lab.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatalf("signal: %v", err)
	}

	done := make(chan error, 1)
	go func() { done <- lab.Wait() }()
	select {
	case <-done:
	case <-time.After(30 * time.Second):
		_ = lab.Process.Kill()
		t.Fatal("sense-lab ignored SIGTERM and ran on toward its wall")
	}

	// 1. The run is recorded. A spent run with no record can never be paired.
	b, err := os.ReadFile(filepath.Join(out, "run-meta.json"))
	if err != nil {
		t.Fatalf("no record left by an interrupted run: %v\nsense-lab said:\n%s", err, labOut.String())
	}
	var m struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("decode run-meta.json: %v", err)
	}

	// 2. It is recorded as an interrupt, not as a budget failure. Conflating
	//    the two puts a row on disk that reads exactly like a stalled arm.
	if m.Outcome != "interrupted" {
		t.Errorf("outcome = %q, want interrupted", m.Outcome)
	}

	// 3. The agent's own children died with it, rather than running on and
	//    spending against a session nobody is waiting for.
	time.Sleep(6 * time.Second)
	if _, err := os.Stat(marker); err == nil {
		t.Error("the agent's grandchild outlived the interrupt and kept running")
	}

	// The prompt is still on disk, so the interrupted run is readable.
	if p, err := os.ReadFile(filepath.Join(out, "prompt.txt")); err != nil || !strings.Contains(string(p), "go") {
		t.Errorf("prompt.txt = %q, %v", p, err)
	}

	// 4. The cell says what it burned. An interrupted pair that left no record
	//    naming the finished arm is how a later pass pairs one.
	if _, err := os.Stat(filepath.Join(runs, "r1", "1", "bench", "cells.json")); err != nil {
		t.Errorf("no record of what the interrupted cell spent: %v", err)
	}
}

// payable puts r1 where the paid step is owed: indexed, its scenario written by
// the stage that writes one, its rehearsal recorded as a PAY, and a bench
// declaring what it is measured on.
func payable(t *testing.T, dir, cfg, scenario string) string {
	t.Helper()
	runs := filepath.Join(dir, "runs")
	write := func(at, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(at, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// The repository names its own checkout: the arms are taken from the tree
	// this test made, not from a clone the lab would have to fetch.
	write(filepath.Join(cfg, "repos", "r1.json"),
		`{"id":"r1","url":"https://example.test/r1.git","commit":"`+headOf(t, dir)+
			`","checkout":"`+dir+`","languages":["go"]}`)
	write(filepath.Join(cfg, "subjects", "sense-main", "subject.json"),
		`{"id":"sense-main","kind":"sense","needs_mcp":false,"executor":"isolated-home","agents":["tool"]}`)
	write(filepath.Join(cfg, "benches", "r1.json"),
		`{"repo":"r1","judge":"m1","driver":{"agent":"tool","model":"m1"},`+
			`"subjects":["untreated","sense-main"],"arms":[{"role":"headline","model":"m1","runs":1}]}`)
	write(filepath.Join(runs, "r1", "index", "index.json"), `{"files":1,"symbols":1}`)
	for _, name := range []string{"scenario.yaml", "scenario.gold.yaml", "scenario.rubric.yaml"} {
		b, err := os.ReadFile(filepath.Join(filepath.Dir(scenario), name))
		if err != nil {
			t.Fatal(err)
		}
		write(filepath.Join(runs, "r1", "1", "expand", name), string(b))
	}
	write(filepath.Join(runs, "r1", "1", "validate", "pay-call.md"), "# Verdict\n\nPAY.\n")
	if err := position.Record(filepath.Join(runs, "r1"), position.Attempt{
		Cycle: 1, Phase: phase.Validate, Try: 1, Verdict: phase.Pay,
		Table: "the rehearsal cleared the bar", Artifact: "written", VerdictDoc: "written",
	}); err != nil {
		t.Fatal(err)
	}
	return runs
}

// writeCatalog writes the smallest config directory that can drive bin.
// initRepo makes dir a one-commit git repository and returns its HEAD, so the
// run's checkout matches the commit the catalog pins.
func initRepo(t *testing.T, dir string) string {
	t.Helper()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=t@t", "-c", "user.name=t", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func writeCatalog(t *testing.T, dir, bin, pin string) string {
	t.Helper()
	cfg := filepath.Join(dir, "config")
	for path, body := range map[string]string{
		"agents/tool/agent.json": `{"id":"tool","binary":"` + bin + `","setup_tool":"tool-cli",
		  "transcript_format":"assistant-events","model_flag":"--model",
		  "config_dirs":[".tool"],"headless_args":["-p"],"env":[],"supports_mcp":false,"auth_modes":["api_key"]}`,
		"subjects/untreated/subject.json": `{"id":"untreated","kind":"baseline","executor":"isolated-home","agents":["tool"]}`,
		"executors/isolated-home.json":    `{"id":"isolated-home","preserves_auth":["api_key"],"isolates_global_config":true}`,
		"models/m1.json":                  `{"id":"m1","provider":"acme","available_under":["api_key"],"agents":["tool"]}`,
		"repos/r1.json":                   `{"id":"r1","url":"https://example.test/r1.git","commit":"` + pin + `","languages":["go"]}`,
	} {
		full := filepath.Join(cfg, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return cfg
}

// senseStandInScript answers what the paid step asks of the Sense binary and
// spawns nothing of its own.
const senseStandInScript = `#!/bin/sh
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

// headOf is the commit the checkout sits at.
func headOf(t *testing.T, dir string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// waitFor polls until cond holds, so a test does not race a process it just
// started. It fails rather than hanging if the condition never arrives.
func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}
