// Package crank turns the loop: it reads a repository's position, runs the one
// phase that comes next, guards what came back, and records it.
//
// Nothing in the binary spawned a phase agent before this. `internal/loop` had
// no importer, the eleven plans were opened by a human, and the routing rules
// cycle 05 built and tested were exercised in production by an operator
// remembering them. That operator is the leak: six authoring cycles on one
// repository is roughly thirty hand-spawns, each one a chance to skip the plan,
// read only the latest rejection, or advance on an agent that produced a
// confident summary and no artifact.
//
// # One phase per invocation
//
// [Crank.Advance] takes a context and a repository id, and nothing else, ever.
// The plan, the wall, the checkout and the spawner are resolved from the
// declared graph and the config. That line is the one that has to hold: an
// options struct refused in the design and then handed in as six arguments is
// the same struct, and it is how the retired driver reached 1204 lines, one
// reasonable parameter at a time.
//
// # The binary spawns, and only the binary spawns
//
// A phase agent that spawns its own judge is grading itself. Nothing here hands
// a phase the ability to run another one, and the plans say so as well.
//
// # An exit code is a claim; the artifact is the fact
//
// A phase reports its own verdict, which makes every phase an unreliable
// narrator about itself. So four things are checked before anything advances,
// and the fourth is the one the retired driver learned by hand, at the phase
// where it happened to bite: the artifact the phase owed is on disk, or the
// loop does not move.
package crank

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/plans"
)

// VerdictFile is what a judgment phase writes beside its artifact.
//
// It exists because a verdict used to be prose inside the artifact, which is
// why a parked campaign could be read by a person and by nothing else. The
// document is small on purpose: a phase reports what it decided and why, and
// everything else about where that leads is the graph's.
const VerdictFile = "verdict.json"

// Verdict is that document.
type Verdict struct {
	Phase   phase.Name    `json:"phase"`
	Repo    string        `json:"repo"`
	Cycle   int           `json:"cycle"`
	Verdict phase.Verdict `json:"verdict"`
	// Table is why, and it is what the next attempt has to answer when this one
	// sends work back.
	Table string `json:"table,omitempty"`
	// Anchor is the symbol the attempt is anchored on, where the phase decides
	// one. A phase that does not touch the anchor leaves it empty and the
	// previous one is carried forward.
	Anchor string `json:"anchor,omitempty"`
}

// readVerdict reads the document a phase left in its directory.
func readVerdict(dir string) (Verdict, error) {
	path := filepath.Join(dir, VerdictFile)
	b, err := os.ReadFile(path)
	if err != nil {
		return Verdict{}, fmt.Errorf("read the verdict of this phase: %w. Every phase writes %s; "+
			"one that did not has not finished, whatever it exited with", err, VerdictFile)
	}
	var v Verdict
	if err := json.Unmarshal(b, &v); err != nil {
		return Verdict{}, fmt.Errorf("read %s: %w", path, err)
	}
	return v, nil
}

// guard is every check that stands between a phase's own account of itself and
// the loop moving.
//
// Four things about identity and one about the fact, and it stays one function. If this becomes a package,
// the design took a wrong turn: what it does is compare a document against the
// job that was dispatched and the plan that was declared, and there is nothing
// else it is ever allowed to consider.
//
// The first three are the phase talking about itself. The fourth is the fact.
func guard(v Verdict, j Job, p plans.Plan) error {
	switch {
	case v.Phase != j.Phase:
		return fmt.Errorf("the verdict names phase %q and %q was dispatched; a verdict about another phase "+
			"routes this one", v.Phase, j.Phase)
	case v.Repo != j.Repo:
		return fmt.Errorf("the verdict names repository %q and %q was dispatched; a verdict about another "+
			"repository is a verdict about another campaign", v.Repo, j.Repo)
	case v.Cycle != j.Cycle:
		return fmt.Errorf("the verdict names cycle %d and cycle %d was dispatched; the cycle is what the "+
			"authoring ceiling is counted in, so a verdict about another one is a verdict about another attempt",
			v.Cycle, j.Cycle)
	case !emits(p, v.Verdict):
		return fmt.Errorf("%s emitted %q, which its plan does not declare; it may emit %v",
			j.Phase, v.Verdict, p.Emits)
	}
	return nil
}

// emits reads the plan's declared enum rather than a list held here. A copy of
// that list in Go is a list that disagrees with the file somebody edits.
func emits(p plans.Plan, v phase.Verdict) bool {
	for _, declared := range p.Emits {
		if declared == v {
			return true
		}
	}
	return false
}

// wrote is the fourth check, and the only one that is not the phase talking.
func wrote(dir string, p plans.Plan) bool {
	_, err := os.Stat(filepath.Join(dir, p.Writes))
	return err == nil
}
