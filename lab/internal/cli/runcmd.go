package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/luuuc/sense/lab/internal/run"
	"github.com/luuuc/sense/lab/internal/scenario"
)

// The walking skeleton's target, hard-coded. Cycle 01 replaces every constant
// here with config as data; until then the point is to have one path work end
// to end, not to have it be configurable.
//
// The scenario is the two-step probe rather than the seven-step session so a
// live run stays short, and discourse is banked on the headline model, so the
// number this produces has something recorded to be checked against.
const (
	defaultWall  = 8 * time.Minute
	defaultAgent = "claude"
)

// defaultAgentArgs drives the agent CLI headlessly with the prompt on stdin.
// The prompt goes on stdin rather than as the value of -p because a prompt
// beginning with a dash is read as an option: that killed a spawn before it ran
// a single step, and the driver reported the crash as a failing check.
//
// Permission prompts are bypassed because there is nobody at the terminal; an
// unattended session that stops to ask a question just burns its wall.
var defaultAgentArgs = []string{
	"-p",
	"--output-format", "stream-json",
	"--verbose",
	"--permission-mode", "bypassPermissions",
}

// defaultAgentEnv marks the session as unattended. It does NOT isolate it:
// measured against a real spawn, the operator's own ~/.claude/CLAUDE.md and the
// subject repository's CLAUDE.md are both still loaded into every benched
// session, so instructions neither arm was meant to have are in both arms.
// There is no isolation in this cycle and this env does not provide any; cycle
// 03 owns it, and 03-02 records what has to be closed.
var defaultAgentEnv = []string{
	"IS_SANDBOX=1",
	"CLAUDE_CODE_DISABLE_AUTO_MEMORY=1",
}

// runFlags is the parsed form of `sense-lab run`. Every field is a path or a
// duration the skeleton needs and cycle 01 will take over.
type runFlags struct {
	scenario string
	repo     string
	out      string
	agent    string
	wall     time.Duration
}

func parseRunFlags(args []string, stderr io.Writer) (runFlags, error) {
	var f runFlags
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.scenario, "scenario", "", "path to the scenario file (required)")
	fs.StringVar(&f.repo, "repo", "", "path to the repository to run against (required)")
	fs.StringVar(&f.out, "out", "", "directory to write the run into (required)")
	fs.StringVar(&f.agent, "agent", defaultAgent, "agent CLI to drive")
	fs.DurationVar(&f.wall, "wall", defaultWall, "how long the session may take")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	for _, required := range []struct {
		name, value string
	}{
		{"-scenario", f.scenario},
		{"-repo", f.repo},
		{"-out", f.out},
	} {
		if required.value == "" {
			return f, fmt.Errorf("%s is required", required.name)
		}
	}
	return f, nil
}

// runSession is the whole skeleton: read the scenario, render it, spawn the
// agent against the repository under a wall, and leave a run directory behind.
func runSession(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	f, err := parseRunFlags(args, stderr)
	if err != nil {
		if err != flag.ErrHelp {
			_, _ = fmt.Fprintf(stderr, "sense-lab run: %v\n", err)
		}
		return exitUsage
	}

	s, err := scenario.Load(f.scenario)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab run: %v\n", err)
		return exitError
	}

	// The absolute path is what gets recorded, so a run is readable from
	// anywhere later. Abs only fails when the working directory cannot be
	// resolved, and it returns the path unchanged when it does, so the Stat
	// below is the check that matters either way.
	repo, _ := filepath.Abs(f.repo)
	if _, err := os.Stat(repo); err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab run: repository: %v\n", err)
		return exitError
	}

	_, _ = fmt.Fprintf(stdout, "scenario: %s\nrepo:     %s\nwall:     %s\n\n", s.Name, repo, f.wall)

	m, err := run.Session(ctx, f.out, run.Spec{
		Dir:   repo,
		Name:  f.agent,
		Args:  defaultAgentArgs,
		Stdin: s.Prompt(),
		Env:   defaultAgentEnv,
		Wall:  f.wall,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab run: %v\n", err)
		return exitError
	}

	_, _ = fmt.Fprintf(stdout, "outcome:  %s\nexit:     %d\ntook:     %.1fs\nrun:      %s\n",
		m.Outcome, m.ExitCode, m.TookSeconds, f.out)

	// A run that could not finish inside its budget is a result, and the exit
	// code says so rather than pretending the session succeeded. The wall is
	// never raised to rescue it.
	switch m.Outcome {
	case run.Completed:
		return exitOK
	case run.CannotFinishAtBudget:
		return exitCannotFinish
	default:
		return exitError
	}
}
