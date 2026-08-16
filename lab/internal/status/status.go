// Package status answers "where does this stand" from the run tree alone.
//
// A campaign runs unattended for hours and is picked up later, often by a
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
)

// Repo is one repository's position in the loop.
type Repo struct {
	Name string
	// Cycle is the authoring cycle on disk, which is the highest numbered one.
	Cycle int
	// Indexed says the repository has been scanned. Indexing happens once per
	// repository, before the first authoring cycle, so it does not live under a
	// cycle and is not redone on a re-entry.
	Indexed bool
	// Reached is the furthest phase of that cycle whose artifact exists.
	Reached phase.Name
	// Awaiting is the first phase of that cycle whose artifact does not. It is
	// what to do next, derived rather than decided.
	Awaiting phase.Name
	// Parked says a handoff page was written for this repository. It is waiting
	// for a human and no transition re-enters it.
	Parked bool
	// Banked is every cycle that reached the board.
	Banked []int
}

// ToCeiling is how many authoring cycles are left before this repository parks.
func (r Repo) ToCeiling() int { return max(phase.AuthoringCeiling-r.Cycle, 0) }

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
// owes. There is deliberately no command field: no verb drives the loop yet, and
// a resume line naming one that does not exist is the stale-resume-file failure
// wearing a different shape. With nowhere to put a command, printing a wrong one
// is unrepresentable rather than merely avoided.
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
	Campaign string
	Repos    []Repo
	Cells    []Cell
	// Orphans are run directories that spawned and never recorded a terminal
	// state. They are listed, never folded into a count.
	Orphans []string
	Spend   budget.Spend
	Ceiling int
	Resume  []Resume
}

// runMetaFile and rawDir mark a run directory; recordFile marks a cell.
const (
	runMetaFile = "run-meta.json"
	rawDir      = "raw"
	recordFile  = "cell-meta.json"
)

// Read reports a campaign's position from its run tree.
//
// A campaign directory that does not exist yet reports an empty position rather
// than an error: asking where a campaign stands before it has run is a fair
// question with a short answer.
func Read(campaignDir string, ceiling int) (Position, error) {
	p := Position{Campaign: campaignDir, Ceiling: ceiling}
	spend, err := budget.Read(campaignDir)
	if err != nil {
		return Position{}, err
	}
	p.Spend = spend

	repos, err := subdirs(campaignDir)
	if err != nil {
		return Position{}, err
	}
	for _, name := range repos {
		r, err := readRepo(filepath.Join(campaignDir, name), name)
		if err != nil {
			return Position{}, err
		}
		p.Repos = append(p.Repos, r)
		if !r.Parked && r.Awaiting != "" {
			p.Resume = append(p.Resume, resumeFor(campaignDir, r))
		}
	}
	if err := p.readCells(campaignDir); err != nil {
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

// readRepo derives one repository's position: its latest cycle, how far that
// cycle got, whether it is parked, and every cycle that reached the board.
func readRepo(dir, name string) (Repo, error) {
	r := Repo{Name: name, Indexed: wrote(dir, indexPhase())}
	cycles, err := subdirs(dir)
	if err != nil {
		return Repo{}, err
	}
	for _, c := range cycles {
		n, err := strconv.Atoi(c)
		if err != nil {
			// Not a cycle. The tree holds other things and this walk is not
			// the place to rule on them.
			continue
		}
		at := filepath.Join(dir, c)
		if wrote(at, phaseOf(phase.Board)) {
			r.Banked = append(r.Banked, n)
		}
		if wrote(at, phaseOf(phase.Handoff)) {
			r.Parked = true
		}
		if n > r.Cycle {
			r.Cycle = n
			r.Reached, r.Awaiting = walk(at)
		}
	}
	if !r.Indexed {
		// Nothing under a cycle can be believed before the repository is
		// scanned, and the scan is what the campaign is waiting for.
		r.Awaiting = phase.Index
	}
	return r, nil
}

// indexPhase and phaseOf look a phase up in the graph. They exist so the walks
// below take a declared phase rather than a name, which removes the "no such
// phase" case from every caller: these names are constants in this package.
func indexPhase() phase.Phase { return phaseOf(phase.Index) }

func phaseOf(name phase.Name) phase.Phase {
	p, _ := phase.Lookup(name)
	return p
}

// walk reports the furthest phase of a cycle whose artifact exists, and the
// first whose artifact does not.
//
// It decides nothing. A phase's artifact is on disk or it is not, and which
// phase comes next in the declared order is a fact about the graph.
func walk(dir string) (reached, awaiting phase.Name) {
	for _, p := range phase.Graph {
		if p.Name == phase.Index {
			// The index is per repository, not per cycle. A re-entry does not
			// rescan, so looking for it here would report every cycle after the
			// first as waiting on a scan that already happened.
			continue
		}
		if wrote(dir, p) {
			reached = p.Name
			continue
		}
		if awaiting == "" {
			awaiting = p.Name
		}
	}
	return reached, awaiting
}

// wrote reports whether a phase's output artifact is under dir.
func wrote(dir string, p phase.Phase) bool {
	_, err := os.Stat(filepath.Join(dir, string(p.Name), p.Writes))
	return err == nil
}

// resumeFor builds the next action for a repository. It names the binary's own
// command where one exists and the plan file otherwise, so the line is never a
// verb that does not exist.
func resumeFor(campaignDir string, r Repo) Resume {
	// The index sits beside the cycles rather than inside one, so an
	// unscanned repository is resumed at the repository directory.
	at := filepath.Join(campaignDir, r.Name)
	if r.Indexed {
		at = filepath.Join(at, strconv.Itoa(r.Cycle))
	}
	return Resume{
		Repo:     r.Name,
		Phase:    r.Awaiting,
		Plan:     filepath.Join("lab", "plans", string(r.Awaiting)+".md"),
		Artifact: phaseOf(r.Awaiting).Writes,
		Dir:      filepath.Join(at, string(r.Awaiting)),
	}
}

// readCells walks the tree for cells and for run directories that never
// recorded a terminal state.
func (p *Position) readCells(campaignDir string) error {
	err := filepath.WalkDir(campaignDir, func(path string, d fs.DirEntry, err error) error {
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
		return fmt.Errorf("read cells under %s: %w", campaignDir, err)
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
