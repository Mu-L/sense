// Package status answers "where does this stand" from the run trees alone.
//
// A repository runs unattended for hours and is picked up later, often by a
// session with no memory of it. The retired instrument answered this with a page
// assembled from a results tree, two state files, a banked log and a ledger.
// That page exists at all only because a resuming session was once pointed at
// the plans and the laws and had to discover its position by asking.
//
// Two of its properties are kept and one is dropped.
//
// Kept: it is write-only and it decides nothing. Position is authoritative on
// disk and the page says so at the top. Nobody edits it into a decision because
// it is regenerated.
//
// Dropped: the hand-maintained resume file. It goes stale by construction, and a
// stale resume line is worse than none because it looks current. What [Read]
// reports is derived from the tree every time.
//
// # Nothing parses the rendered page
//
// [Read] returns a [Position] and [Render] turns one into a string. Later
// callers use [Read]; the string exists to be printed. Parsing your own report
// is how a display format silently becomes a data contract, and the split here
// is in the types rather than in a sentence asking people not to.
//
// # Nothing uncomfortable is summarised away
//
// Incomplete cells, burned runs, orphaned run directories and parked
// repositories are the lines a resuming session most needs and least expects to
// ask about. A status that shows only progress is a status nobody trusts twice.
package status

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/luuuc/sense/lab/internal/budget"
	"github.com/luuuc/sense/lab/internal/cell"
	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/position"
)

// Repo is one repository's position in the loop, and what it has cost.
//
// The position is READ rather than derived here. It is the position package's
// answer, unchanged, which is the same answer the crank routes on: two readers
// working the same tree out separately is two answers, and the one on a page is
// the one nobody checks against a verdict. That is not hypothetical — this page
// used to derive the cycle from the numbered directories itself, so a
// repository admitted and scanned but with no cycle directory yet read as cycle
// 0 awaiting nothing, while the crank read cycle 1 awaiting author.
type Repo struct {
	position.Position
	// Spend is what this repository has cost, read from its own tree. The
	// scope is the repository because that is what a ceiling is decided
	// about: a repository that burns its runs is a repository to stop, and
	// nothing about the one beside it changed.
	Spend budget.Spend
}

// Cell is one recorded cell, complete or not.
type Cell struct {
	Path     string
	Complete bool
	Burned   []string
	Unusable []string
}

// Resume is the exact next action, and it cannot go stale because it is derived.
//
// It names the phase, the plan file that says how to run it and the artifact it
// owes, and it still carries no command field. It used to have no command
// because no verb drove the loop; now there is one, and it is not held here:
// what moves a repository is a property of where it STANDS, so it is derived
// from the standing by [position.Position.Next] and printed on the row. A
// second copy on this struct would be the one that goes stale.
//
// The plan file is guaranteed to be there: every phase in the graph has one, and
// that is checked mechanically rather than believed.
type Resume struct {
	Repo     string
	Phase    phase.Name
	Plan     string
	Artifact string
	Dir      string
}

// Position is everything the page reports.
type Position struct {
	// Root is the directory the run trees live under, one per repository.
	Root  string
	Repos []Repo
	Cells []Cell
	// Orphans are run directories that spawned and never recorded a terminal
	// state. They are listed, never folded into a count.
	Orphans []string
	// Ceiling is how many paid runs each repository gets over its lifetime.
	Ceiling int
	Resume  []Resume
}

// runMetaFile and rawDir mark a run directory; recordFile marks a cell.
const (
	runMetaFile = "run-meta.json"
	rawDir      = "raw"
	recordFile  = "cell-meta.json"
)

// Read reports every repository's position from the run trees under root.
//
// A root that does not exist yet reports an empty position rather than an
// error: asking where the repositories stand before any of them has run is a
// fair question with a short answer.
func Read(root string, ceiling int) (Position, error) {
	p := Position{Root: root, Ceiling: ceiling}
	repos, err := subdirs(root)
	if err != nil {
		return Position{}, err
	}
	for _, name := range repos {
		r, err := readRepo(root, name)
		if err != nil {
			return Position{}, err
		}
		p.Repos = append(p.Repos, r)
		if r.Standing == position.Ready {
			// Ready is the only standing with a phase to run. Every other one
			// stops the loop and wants a human, and the standing line already
			// says which and why.
			p.Resume = append(p.Resume, resumeFor(root, r))
		}
	}
	if err := p.readCells(root); err != nil {
		return Position{}, err
	}
	return p, nil
}

// subdirs lists a directory's immediate subdirectories, sorted. A missing
// directory has none, which is the answer rather than a failure.
func subdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out, nil
}

// readRepo reads one repository's position and its spend.
func readRepo(root, name string) (Repo, error) {
	at, err := position.Read(root, name)
	if err != nil {
		return Repo{}, err
	}
	spend, err := budget.Read(filepath.Join(root, name))
	if err != nil {
		return Repo{}, err
	}
	return Repo{Position: at, Spend: spend}, nil
}

// phaseOf looks a phase up in the graph. It exists so a caller takes a declared
// phase rather than a name, which removes the "no such phase" case: these names
// are constants in this package.
func phaseOf(name phase.Name) phase.Phase {
	p, _ := phase.Lookup(name)
	return p
}

// resumeFor builds the next action for a repository. It names the binary's own
// command where one exists and the plan file otherwise, so the line is never a
// verb that does not exist.
func resumeFor(root string, r Repo) Resume {
	// The index sits beside the cycles rather than inside one, so an
	// unscanned repository is resumed at the repository directory.
	at := filepath.Join(root, r.Repo)
	if r.Indexed {
		at = filepath.Join(at, strconv.Itoa(r.Cycle))
	}
	return Resume{
		Repo:     r.Repo,
		Phase:    r.Awaiting,
		Plan:     filepath.Join("lab", "plans", string(r.Awaiting)+".md"),
		Artifact: phaseOf(r.Awaiting).Writes,
		Dir:      filepath.Join(at, string(r.Awaiting)),
	}
}

// readCells walks the tree for cells and for run directories that never
// recorded a terminal state.
func (p *Position) readCells(root string) error {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case !d.IsDir():
			return nil
		case exists(filepath.Join(path, recordFile)):
			return p.addCell(path)
		case exists(filepath.Join(path, rawDir)) && !exists(filepath.Join(path, runMetaFile)):
			p.Orphans = append(p.Orphans, path)
		}
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read cells under %s: %w", root, err)
	}
	slices.Sort(p.Orphans)
	slices.SortFunc(p.Cells, func(a, b Cell) int { return strings.Compare(a.Path, b.Path) })
	return nil
}

func (p *Position) addCell(path string) error {
	rec, err := cell.ReadRecord(path)
	if err != nil {
		return err
	}
	p.Cells = append(p.Cells, Cell{
		Path: path, Complete: rec.Complete, Burned: rec.Burned, Unusable: rec.Unusable,
	})
	return nil
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Incomplete reports the cells that can never be paired.
func (p Position) Incomplete() []Cell {
	var out []Cell
	for _, c := range p.Cells {
		if !c.Complete {
			out = append(out, c)
		}
	}
	return out
}
