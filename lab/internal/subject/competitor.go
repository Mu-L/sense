package subject

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/luuuc/sense/lab/internal/channels"
)

// A subject that is not Sense arrives with an installer, may write to the home
// directory, may register itself with the agent tool, may start a background
// process, and may leave state behind that changes the next run. Every one of
// those is a contamination channel, and unlike Sense's, nobody here knows what
// they are in advance.
//
// So the shape is: declare what it touches, run it, and CHECK the declaration
// against what happened. A subject's own documentation describes what it
// intends to do; only a run says what it does.

// Plan is what a subject does to a machine, as the catalog declares it.
//
// Command LISTS rather than a script, so what a subject does is readable
// without running it. A script would be one opaque string in a config file, and
// the whole point of this declaration is that a person can read it before a
// competitor's installer runs on their machine.
type Plan struct {
	// Touches are the paths this subject may write, relative to the arm's HOME
	// or to its repository. It is a DECLARATION, checked against what the
	// subject actually wrote, and it starts out empty for a subject nobody has
	// run yet.
	Touches []string
	// Install, Setup and Cleanup are run in that order around a session, each
	// as argv.
	Install [][]string
	Setup   [][]string
	Cleanup [][]string
}

// Env is where a subject's commands run: its own repository, its own disposable
// HOME, and the environment the arm was given.
//
// It carries the arm's environment rather than the host's, which is the
// credential rule made structural: a subject is handed the repository and
// whatever its own setup needs, and never the host's tokens or the host's agent
// configuration. There is nothing here to pass them through.
type Env struct {
	Repo    string
	Home    string
	Environ []string
}

// Prepare installs and sets up a subject, and reports every path that appeared
// under its HOME or its repository while it ran.
//
// The observation is the point. On a subject's FIRST run there is nothing to
// check against, so this is a discovery run: what it returns is what the
// declaration should say, written from what happened rather than from what the
// subject's documentation claims.
func Prepare(ctx context.Context, p Plan, env Env) ([]string, error) {
	before, err := state(env)
	if err != nil {
		return nil, err
	}
	for _, stage := range []struct {
		what string
		cmds [][]string
	}{{"install", p.Install}, {"setup", p.Setup}} {
		if err := runAll(ctx, stage.what, stage.cmds, env); err != nil {
			return nil, err
		}
	}
	after, err := state(env)
	if err != nil {
		return nil, err
	}
	return appeared(before, after), nil
}

// Remove runs the subject's cleanup and reports what survived it.
//
// An empty result is the only acceptable outcome, and it is CHECKED rather than
// inferred from a clean exit. An uninstaller that leaves a config entry behind
// contaminates every subsequent run on that machine, and the symptom looks like
// drift rather than like a leak.
func Remove(ctx context.Context, p Plan, env Env, installed []string) ([]string, error) {
	if err := runAll(ctx, "cleanup", p.Cleanup, env); err != nil {
		return nil, err
	}
	now, err := state(env)
	if err != nil {
		return nil, err
	}
	var left []string
	for _, path := range installed {
		if _, still := now[path]; still {
			left = append(left, path)
		}
	}
	return left, nil
}

// Undeclared names the paths a subject wrote that its declaration does not
// cover.
//
// A declared path covers itself and everything under it, because a subject that
// says it writes a directory has said what the directory is for. Anything else
// is a finding about the subject, and it belongs in that subject's README with
// a date and a version — measured, not read out of its documentation.
func Undeclared(wrote []string, declared []string) []string {
	var out []string
	for _, path := range wrote {
		if !covered(path, declared) {
			out = append(out, path)
		}
	}
	return out
}

func covered(path string, declared []string) bool {
	for _, d := range declared {
		if path == d || strings.HasPrefix(path, strings.TrimSuffix(d, "/")+"/") {
			return true
		}
	}
	return false
}

// state is every file under the arm's HOME and repository, by path.
//
// Both, because the two failure modes are different: a subject that writes into
// the repository has changed what the next arm sees, and one that writes into
// HOME has changed what every later run on that machine sees.
func state(env Env) (map[string]string, error) {
	all := map[string]string{}
	for _, root := range []struct{ prefix, dir string }{{"home", env.Home}, {"repo", env.Repo}} {
		if root.dir == "" {
			continue
		}
		// .git and .sense are machinery rather than anything a subject did, and
		// both are large enough to make this the slowest step in a run.
		tree, err := channels.Tree(root.dir, ".git", ".sense")
		if err != nil {
			return nil, fmt.Errorf("read the %s tree: %w", root.prefix, err)
		}
		for rel, hash := range tree {
			all[path.Join(root.prefix, rel)] = hash
		}
	}
	return all, nil
}

// appeared is what the second state holds that the first did not, or holds
// differently. Sorted, because a set of paths a human reads in a different
// order every run is a set nobody can diff.
func appeared(before, after map[string]string) []string {
	var out []string
	for p, hash := range after {
		if before[p] != hash {
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out
}

// runAll runs one stage's commands in order, stopping at the first failure.
func runAll(ctx context.Context, what string, cmds [][]string, env Env) error {
	for i, argv := range cmds {
		if len(argv) == 0 {
			return fmt.Errorf("%s step %d is an empty command", what, i+1)
		}
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...) // #nosec G204 -- the argv a subject declares
		cmd.Dir = env.Repo
		cmd.Env = env.Environ
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s step %d (%s): %w: %s", what, i+1, argv[0], err, out)
		}
	}
	return nil
}
