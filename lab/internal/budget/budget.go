// Package budget reports what a repository has spent, by reading its run tree.
//
// It never holds a counter. A ceiling kept in a variable is a ceiling that
// resets when the process does, and this binary spends money unattended: a
// loop restarted after a crash would start again from zero and spend its whole
// budget a second time without anything looking wrong.
//
// So spend is recomputed, every time, from the directories that exist. That is
// also why it cannot be double-counted across a restart: the total is the size
// of a SET of run directories, not a running sum something adds to. Reading it
// twice gives the same answer because reading it does not change it.
//
// # The unit is paid runs
//
// Not dollars. Nothing on disk records a price — the model files carry an id, a
// provider and the tools that can drive them, and no rate — so a dollar figure
// here would be a number this package invented. A paid run is the thing the
// lab actually decides to spend, and it is a fact in the tree.
//
// # The scope is the repository, over its lifetime
//
// A ceiling is decided about a repository: one that burns its runs without
// reaching the board is the one to stop, and nothing about the repository
// beside it changed. Not per calendar period, because a month boundary is not a
// fact about the work. [Read] is handed a repository's run tree and walks all
// of it.
package budget

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/luuuc/sense/lab/internal/phase"
)

// runMetaFile is what a run writes when it reaches a terminal state, and
// rawDir is the capture directory a run creates before it spawns anything.
// A directory holding rawDir is a run; whether it holds runMetaFile is whether
// that run finished.
const (
	runMetaFile = "run-meta.json"
	rawDir      = "raw"
)

// Spend is what a repository has spent, as the runs that produced it.
//
// Orphaned is not folded into Recorded and is not dropped. A bench run that
// started and never wrote a terminal record still spent its money, and a ceiling
// that ignores it undercounts by exactly the runs nobody can account for — which
// are the ones a resuming session most needs to see.
type Spend struct {
	Recorded []string
	Orphaned []string
}

// Runs is the repository's spend: every paid run in the tree, accounted or not.
func (s Spend) Runs() int { return len(s.Recorded) + len(s.Orphaned) }

func (s Spend) String() string {
	if len(s.Orphaned) == 0 {
		return fmt.Sprintf("%d paid runs", s.Runs())
	}
	return fmt.Sprintf("%d paid runs, %d of them orphaned with no terminal record", s.Runs(), len(s.Orphaned))
}

// Read reports the spend recorded under a repository's run tree.
//
// A paid run is one under the bench phase. Mini-bench and validation runs are
// unscored and unpaid by law, and counting them against the ceiling would refuse
// a repository for the runs it does to avoid spending.
//
// A tree that does not exist yet has spent nothing, which is a fact rather than
// an error: the ceiling is checked before the first run as well as after the
// hundredth.
func Read(tree string) (Spend, error) {
	var s Spend
	err := filepath.WalkDir(tree, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case !d.IsDir() || !paid(tree, path):
			return nil
		}
		if !exists(filepath.Join(path, rawDir)) {
			return nil
		}
		if exists(filepath.Join(path, runMetaFile)) {
			s.Recorded = append(s.Recorded, path)
		} else {
			s.Orphaned = append(s.Orphaned, path)
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return Spend{}, nil
	}
	if err != nil {
		return Spend{}, fmt.Errorf("read spend under %s: %w", tree, err)
	}
	slices.Sort(s.Recorded)
	slices.Sort(s.Orphaned)
	return s, nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// paid reports whether a path sits under the bench phase. It reads the phase
// name from the graph rather than repeating the string, so a renamed phase
// cannot silently make every paid run invisible to the ceiling.
func paid(root, path string) bool {
	// The walk only ever yields paths under root, so a relative path always
	// exists. An empty one contains no bench segment, which is the same answer.
	rel, _ := filepath.Rel(root, path)
	return slices.Contains(strings.Split(filepath.ToSlash(rel), "/"), string(phase.Bench))
}

// ErrCeiling is what a refusal is. It is a sentinel because a caller has to
// tell "the ceiling says no" from "the tree could not be read": one is the
// instrument working and the answer to the question that was asked, the other
// is a broken invocation.
var ErrCeiling = errors.New("spend ceiling reached")

// Ceiling refuses a repository that has reached its spend ceiling.
//
// It refuses rather than warns. A ceiling that warns is a ceiling that gets
// passed at the moment someone believes the next run will work, and that belief
// is what the ceiling exists to bound.
func Ceiling(s Spend, ceiling int) error {
	if s.Runs() >= ceiling {
		return fmt.Errorf("%w: this repository has spent %s against a ceiling of %d over its lifetime",
			ErrCeiling, s, ceiling)
	}
	return nil
}
