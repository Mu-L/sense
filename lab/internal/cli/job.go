package cli

import (
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/luuuc/sense/lab/internal/catalog"
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
func resolveJob(c *catalog.Catalog, f target) (job, error) {
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

// target is what resolving one job needs: which repository, driven by which
// tool, on which model.
//
// It was `runFlags`, the parsed form of a command that no longer exists. A flag
// set with no flags is a name that sends the next reader looking for a command
// to match it, and the three fields that outlived that command are the three
// [resolveJob] reads.
type target struct {
	repo  string
	agent string
	model string
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
