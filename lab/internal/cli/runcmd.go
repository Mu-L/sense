package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/run"
	"github.com/luuuc/sense/lab/internal/scenario"
)

// The wall is the one remaining constant, and it is a default rather than a
// fact about any particular job. Everything else — which repository, which
// agent tool, which model — comes from the catalog, so adding one is a new JSON
// file and nothing else.
const defaultWall = 8 * time.Minute

// job is a resolved run target: which repository, driven by which agent tool,
// on which model. All three come from the catalog.
type job struct {
	repo  catalog.Repo
	agent catalog.Agent
	model catalog.Model
}

// resolveJob turns three ids into the config they name, and refuses a
// combination the catalog says cannot work. This runs BEFORE anything is
// spawned, because an unsupported combination discovered forty minutes into a
// paid run is a run wasted.
func resolveJob(c *catalog.Catalog, f runFlags) (job, error) {
	var j job
	var ok bool
	if j.repo, ok = c.Repos[f.repo]; !ok {
		return j, fmt.Errorf("no repo %q in the catalog; have %v", f.repo, catalog.IDs(c.Repos))
	}
	agent, model, err := resolveDriver(c, f.agent, f.model)
	if err != nil {
		return j, err
	}
	j.agent, j.model = agent, model
	return j, nil
}

// resolveDriver turns an agent id and a model id into the pair that can run,
// and refuses a pair the catalog says cannot. It is separate from the repository
// lookup because a phase agent is driven without one: it runs against a
// repository rather than benching it.
func resolveDriver(c *catalog.Catalog, agentID, modelID string) (catalog.Agent, catalog.Model, error) {
	a, ok := c.Agents[agentID]
	if !ok {
		return a, catalog.Model{}, fmt.Errorf("no agent %q in the catalog; have %v", agentID, catalog.IDs(c.Agents))
	}
	m, ok := c.Model(modelID)
	if !ok {
		return a, m, fmt.Errorf("no model %q in the catalog; have %v", modelID, catalog.IDs(c.Models))
	}
	if !slices.Contains(m.Agents, a.ID) {
		return a, m, fmt.Errorf("model %s cannot be driven by %s; it names %v", m.ID, a.ID, m.Agents)
	}
	return a, m, nil
}

// A note that used to live beside the compiled-in agent flags, kept because the
// flags moved and the measurement did not:
//
// The prompt goes to the agent on STDIN, never as the value of a flag. A prompt
// beginning with a dash is read as an option, which killed a spawn before it ran
// a single step and was reported as a failing check rather than a crash.
//
// An agent's `env` marks a session as unattended. It does NOT isolate it:
// measured against a real spawn, the operator's own ~/.claude/CLAUDE.md and the
// subject repository's CLAUDE.md are both still loaded into every benched
// session, so instructions neither arm was meant to have reach both arms. Cycle
// 03 owns isolation, and 03-02 records what has to be closed.

// runFlags is the parsed form of `sense-lab run`: three catalog ids, two paths
// and a wall.
type runFlags struct {
	config   string
	scenario string
	repo     string
	checkout string
	out      string
	agent    string
	model    string
	wall     time.Duration
}

func parseRunFlags(args []string, stderr io.Writer) (runFlags, error) {
	var f runFlags
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configFlag(fs, &f.config)
	fs.StringVar(&f.scenario, "scenario", "", "path to the scenario file (required)")
	fs.StringVar(&f.repo, "repo", "", "catalog repo id to run against (required)")
	fs.StringVar(&f.checkout, "checkout", "", "path to the repository checkout (required)")
	fs.StringVar(&f.out, "out", "", "directory to write the run into (required)")
	fs.StringVar(&f.agent, "agent", "", "catalog agent id to drive (required)")
	fs.StringVar(&f.model, "model", "", "catalog model id (required)")
	fs.DurationVar(&f.wall, "wall", defaultWall, "how long the session may take")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	for _, required := range []struct {
		name, value string
	}{
		{"-scenario", f.scenario},
		{"-repo", f.repo},
		{"-checkout", f.checkout},
		{"-out", f.out},
		{"-agent", f.agent},
		{"-model", f.model},
	} {
		if required.value == "" {
			return f, fmt.Errorf("%s is required", required.name)
		}
	}
	return f, nil
}

// checkoutIsAtPin refuses a checkout that is not at the repo's pinned commit.
//
// Without it the pinned commit is a label rather than a fact: the run reports a
// commit, records it, and may have been taken against a different branch or
// last month's clone. Every number is then unreproducible in a way nothing
// about the result shows — the same class of failure as an arm whose model
// resolved to nothing.
func checkoutIsAtPin(checkout, pin string) error {
	out, err := exec.Command("git", "-C", checkout, "rev-parse", "HEAD").Output()
	if err != nil {
		return fmt.Errorf("checkout %s: cannot read its commit: %w", checkout, err)
	}
	at := strings.TrimSpace(string(out))
	if at != pin {
		return fmt.Errorf("checkout %s is at %.12s but the catalog pins %.12s; "+
			"a run against the wrong tree records a commit it did not use", checkout, at, pin)
	}
	return nil
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

	c, err := catalog.Load(f.config)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab run: %v\n", err)
		return exitError
	}
	j, err := resolveJob(c, f)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab run: %v\n", err)
		return exitError
	}

	set, err := scenario.LoadPath(f.scenario)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab run: %v\n", err)
		return exitError
	}

	// The absolute path is what gets recorded, so a run is readable from
	// anywhere later. Abs only fails when the working directory cannot be
	// resolved, and it returns the path unchanged when it does, so the Stat
	// below is the check that matters either way.
	checkout, _ := filepath.Abs(f.checkout)
	if _, err := os.Stat(checkout); err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab run: checkout: %v\n", err)
		return exitError
	}
	if err := checkoutIsAtPin(checkout, j.repo.Commit); err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab run: %v\n", err)
		return exitError
	}

	_, _ = fmt.Fprintf(stdout, "scenario: %s\nrepo:     %s @ %.8s\nagent:    %s\nmodel:    %s\nwall:     %s\n\n",
		set.Scenario.Name, j.repo.ID, j.repo.Commit, j.agent.ID, j.model.ID, f.wall)

	m, err := run.Session(ctx, f.out, run.Spec{
		Dir:   checkout,
		Name:  j.agent.Binary,
		Args:  append(slices.Clone(j.agent.HeadlessArgs), j.agent.ModelFlag, j.model.ID),
		Stdin: set.Scenario.Prompt(),
		// The host's environment plus the agent's own, composed here rather
		// than inside run.Session so the inheritance is a visible decision at
		// the edge. It is still the wrong environment for a measured run, and
		// 03-04 replaces this call site with a prepared isolate.Env.
		Env:  append(os.Environ(), j.agent.Env...),
		Wall: f.wall,
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
