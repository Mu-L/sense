package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/crank"
	"github.com/luuuc/sense/lab/internal/isolate"
	"github.com/luuuc/sense/lab/internal/plans"
	"github.com/luuuc/sense/lab/internal/position"
	"github.com/luuuc/sense/lab/internal/run"
)

// plansDir is where the declared plans live, relative to the config directory.
// The plans are config the way the catalog is: a phase's instructions are a file
// somebody reviews, never a string in this binary.
const plansDir = "plans"

// attemptDir names a run directory for one try. The first is unsuffixed, so the
// ordinary tree reads the way it always has, and a second attempt lands beside
// the first rather than on top of it: a run's environment is created fresh or
// not at all, and overwriting one would destroy the evidence of why there was a
// second attempt.
func attemptDir(name string, try int) string {
	if try <= 1 {
		return name
	}
	return fmt.Sprintf("%s-%d", name, try)
}

// phaseGrace is how long a phase agent gets to stop on its own after its wall.
// It is here rather than in the plan header because it is not a property of any
// phase: it is how long a process takes to die.
const phaseGrace = 30 * time.Second

// turn runs the crank over one repository: one phase, or every phase until it
// stops on its own.
//
// The loop is here rather than inside the crank. `-until` takes no argument and
// means crank until it stops, so nothing in the dispatcher knows which of the
// two it is in and no mode is threaded through it.
func turn(ctx context.Context, f repoFlags, c *catalog.Catalog, id string, stdout, stderr io.Writer) int {
	k, err := crankFor(f, c, id)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab repo: %v\n", err)
		return exitError
	}
	for {
		r, err := k.Advance(ctx, id)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "sense-lab repo: %v\n", err)
			return exitError
		}
		_, _ = fmt.Fprint(stdout, position.Render(r.After))
		if r.Note != "" {
			_, _ = fmt.Fprintf(stdout, "\n%s\n", r.Note)
		}
		if code, stop := stopOn[r.After.Standing]; stop {
			return code
		}
		if !f.until {
			return exitOK
		}
	}
}

// crankFor assembles the wiring once: the declared plans, the repository's
// checkout, and the spawner. Everything a phase needs comes from here or from
// its plan, never from Advance's arguments.
func crankFor(f repoFlags, c *catalog.Catalog, id string) (crank.Crank, error) {
	loaded, err := plans.Load(filepath.Join(f.config, plansDir))
	if err != nil {
		return crank.Crank{}, err
	}
	r, ok := c.Repos[id]
	if !ok {
		return crank.Crank{}, fmt.Errorf("no repository %q in the catalog", id)
	}
	checkout := r.Checkout
	if checkout == "" {
		checkout = filepath.Join(f.checkouts, id)
	}
	spawn, err := phaseSpawner(f, c)
	if err != nil {
		return crank.Crank{}, err
	}
	return crank.Crank{Runs: f.runs, Plans: loaded, Checkout: checkout, Spawn: spawn}, nil
}

// phaseSpawner is the real spawner: the one thing in this cycle that reaches a
// process.
//
// It lives here, at the edge, beside the other things that spawn. The crank
// takes it as a value and never imports one, which is what the depguard rule on
// the deciding packages holds in place.
func phaseSpawner(f repoFlags, c *catalog.Catalog) (crank.Spawner, error) {
	if f.agent == "" || f.model == "" {
		return nil, fmt.Errorf("-agent and -model say what a phase is run by; without them nothing can be dispatched")
	}
	a, m, err := resolveDriver(c, f.agent, f.model)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, j crank.Job) (crank.Ran, error) {
		return spawnPhase(ctx, f, a, m, j)
	}, nil
}

// spawnPhase runs one phase agent under its declared wall, inside a disposable
// environment carrying no more than a run arm's credential.
//
// The agent is spawned in the repository under study, because that is what it
// reads. It writes its artifact and its verdict into the phase directory it is
// told about, which is why that directory is named in what it is handed rather
// than inferred by it.
func spawnPhase(ctx context.Context, f repoFlags, a catalog.Agent, model catalog.Model, j crank.Job) (crank.Ran, error) {
	// The same credential a run arm gets, resolved the same way and reaching no
	// keychain of its own. A phase agent is not a privileged thing.
	cred, err := cellCredential(ctx, a, model, time.Now().Add(j.Wall+phaseGrace))
	if err != nil {
		return crank.Ran{}, err
	}
	env, err := isolate.Prepare(isolate.Spec{
		Root:       filepath.Join(j.Dir, attemptDir("env", j.Try)),
		Arm:        isolate.Sense,
		SenseBin:   f.senseBin,
		HostPath:   os.Getenv("PATH"),
		AgentEnv:   a.Env,
		Credential: cred,
		Route:      credentialRoute(a),
	})
	if err != nil {
		return crank.Ran{}, err
	}
	m, err := run.Session(ctx, filepath.Join(j.Dir, attemptDir("session", j.Try)), run.Spec{
		Dir:   j.Checkout,
		Name:  a.Binary,
		Args:  append(slices.Clone(a.HeadlessArgs), a.ModelFlag, model.ID),
		Stdin: j.Prompt,
		Env:   env.Environ,
		Arm:   string(isolate.Sense),
		Wall:  j.Wall,
		Grace: phaseGrace,
	})
	if err != nil {
		return crank.Ran{}, err
	}
	// The outcome is carried, never interpreted into a verdict. What the phase
	// decided is read from the document it wrote, because an exit code is a
	// claim.
	return crank.Ran{
		Finished: m.Outcome == run.Completed,
		Log:      filepath.Join(j.Dir, attemptDir("session", j.Try), "raw", "stdout"),
	}, nil
}
