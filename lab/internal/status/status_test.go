package status_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/position"
	"github.com/luuuc/sense/lab/internal/status"
)

// trees builds the run trees by hand, because the tree IS the contract: a
// position derived from anything else would be a position that can disagree
// with what is on disk.
type trees struct {
	t   *testing.T
	dir string
}

func newTrees(t *testing.T) trees {
	t.Helper()
	return trees{t: t, dir: t.TempDir()}
}

// phaseDone writes a phase's output artifact for one repository and cycle.
func (c trees) phaseDone(repo string, cycle int, name phase.Name) trees {
	c.t.Helper()
	p, ok := phase.Lookup(name)
	if !ok {
		c.t.Fatalf("no phase named %s", name)
	}
	// The index is per repository and sits beside the cycles, which is what a
	// re-entry not rescanning looks like on disk.
	at := filepath.Join(c.dir, repo, itoa(cycle))
	if name == phase.Index {
		at = filepath.Join(c.dir, repo)
	}
	dir := filepath.Join(at, string(name))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.t.Fatal(err)
	}
	write(c.t, filepath.Join(dir, p.Writes), "written")
	return c
}

// cellAt writes a cell record beside its arms.
func (c trees) cellAt(rel, record string) trees {
	c.t.Helper()
	dir := filepath.Join(c.dir, rel)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.t.Fatal(err)
	}
	write(c.t, filepath.Join(dir, "cell-meta.json"), record)
	return c
}

// run writes a run directory. finished says whether it recorded a terminal
// state; an unfinished one is what a reboot leaves behind.
func (c trees) run(rel string, finished bool) trees {
	c.t.Helper()
	dir := filepath.Join(c.dir, rel)
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		c.t.Fatal(err)
	}
	if finished {
		write(c.t, filepath.Join(dir, "run-meta.json"), `{"outcome":"completed"}`)
	}
	return c
}

// indexed marks a repository as scanned.
func (c trees) indexed(repo string) trees {
	c.t.Helper()
	return c.phaseDone(repo, 0, phase.Index)
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func (c trees) read(ceiling int) status.Position {
	c.t.Helper()
	p, err := status.Read(c.dir, ceiling)
	if err != nil {
		c.t.Fatal(err)
	}
	return p
}

// The whole point: a session with no memory of a repository reads its position off
// the tree, and nothing hand-maintained can disagree with it.
func TestPositionIsTheFurthestPhaseWithItsArtifactAndTheFirstWithout(t *testing.T) {
	c := newTrees(t).
		indexed("mastodon").
		phaseDone("mastodon", 1, phase.Author).
		phaseDone("mastodon", 1, phase.Minibench)

	p := c.read(40)
	if len(p.Repos) != 1 {
		t.Fatalf("read %d repositories, want 1", len(p.Repos))
	}
	r := p.Repos[0]
	if r.Cycle != 1 {
		t.Errorf("cycle %d, want 1", r.Cycle)
	}
	if r.Reached != phase.Minibench {
		t.Errorf("reached %s, want %s", r.Reached, phase.Minibench)
	}
	if r.Awaiting != phase.Expand {
		t.Errorf("awaiting %s, want %s", r.Awaiting, phase.Expand)
	}
	if r.ToCeiling() != phase.AuthoringCeiling-1 {
		t.Errorf("%d cycles to the ceiling, want %d", r.ToCeiling(), phase.AuthoringCeiling-1)
	}
}

// The latest cycle is the position. Reading an earlier one would report a
// repository as further back than it is, which is the resuming session's worst
// possible input.
func TestTheLatestCycleIsThePosition(t *testing.T) {
	c := newTrees(t).
		indexed("mastodon").
		phaseDone("mastodon", 1, phase.Author).
		phaseDone("mastodon", 1, phase.Minibench).
		phaseDone("mastodon", 3, phase.Author)

	r := c.read(40).Repos[0]
	if r.Cycle != 3 {
		t.Fatalf("cycle %d, want 3", r.Cycle)
	}
	if r.Reached != phase.Author {
		t.Errorf("reached %s on cycle 3, want %s", r.Reached, phase.Author)
	}
}

func TestABankedCycleAndAParkedRepositoryAreBothShown(t *testing.T) {
	p := newTrees(t).
		phaseDone("mastodon", 2, phase.Board).
		phaseDone("chatwoot", 6, phase.Handoff).
		read(40)

	byName := map[string]status.Repo{}
	for _, r := range p.Repos {
		byName[r.Repo] = r
	}
	if got := byName["mastodon"].Banked; len(got) != 1 || got[0] != 2 {
		t.Errorf("mastodon banked %v, want cycle 2", got)
	}
	if byName["mastodon"].Standing == position.Parked {
		t.Error("a repository that banked a win was reported as parked")
	}
	if byName["chatwoot"].Standing != position.Parked {
		t.Errorf("a repository with a handoff page stands as %q, want parked",
			byName["chatwoot"].Standing)
	}
}

// A parked repository is waiting for a deliberate human action, so a resume line
// for it would be an instruction to do the thing the ceiling exists to stop.
func TestAParkedRepositoryGetsNoResumeLine(t *testing.T) {
	p := newTrees(t).
		indexed("chatwoot").indexed("mastodon").
		phaseDone("chatwoot", 6, phase.Handoff).
		phaseDone("mastodon", 1, phase.Author).
		read(40)

	for _, r := range p.Resume {
		if r.Repo == "chatwoot" {
			t.Error("a parked repository was handed a resume line")
		}
	}
	if len(p.Resume) != 1 || p.Resume[0].Repo != "mastodon" {
		t.Errorf("resume lines %+v, want one for mastodon", p.Resume)
	}
}

// The stale-resume-file failure in a new shape: a line naming a verb the binary
// does not have. Every resume line either names a real command or names the
// plan file, and the plan checks guarantee that file exists.
func TestEveryResumeLineIsRunnable(t *testing.T) {
	c := newTrees(t)
	for _, p := range phase.Graph {
		repo := "repo-" + string(p.Name)
		c = c.indexed(repo).phaseDone(repo, 1, p.Name)
	}
	for _, r := range c.read(40).Resume {
		if _, err := os.Stat(filepath.Join("../../..", r.Plan)); err != nil {
			t.Errorf("%s resumes at %s and points at %s, which is not on disk", r.Repo, r.Phase, r.Plan)
		}
		declared, _ := phase.Lookup(r.Phase)
		if r.Artifact != declared.Writes {
			t.Errorf("%s awaits %q; phase %s writes %q", r.Repo, r.Artifact, r.Phase, declared.Writes)
		}
	}
}

// A half-pair, a burned run and an orphan are what a resuming session most needs
// and least expects to ask about. None of them may be folded into a count.
func TestIncompleteCellsBurnedRunsAndOrphansAreAllNamed(t *testing.T) {
	p := newTrees(t).
		cellAt("mastodon/1/bench/cell-0", `{"arms":{"sense":"x"},"complete":false,"burned":["sense"],"unusable":["untreated"]}`).
		cellAt("mastodon/1/bench/cell-1", `{"arms":{"sense":"x","untreated":"y"},"complete":true}`).
		run("mastodon/1/bench/cell-2/sense", false).
		run("mastodon/1/bench/cell-1/sense", true).
		read(40)

	if len(p.Cells) != 2 {
		t.Fatalf("read %d cells, want 2", len(p.Cells))
	}
	incomplete := p.Incomplete()
	if len(incomplete) != 1 {
		t.Fatalf("%d incomplete cells, want 1", len(incomplete))
	}
	if len(incomplete[0].Burned) != 1 || incomplete[0].Burned[0] != "sense" {
		t.Errorf("burned arms %v, want the sense arm named", incomplete[0].Burned)
	}
	if len(incomplete[0].Unusable) != 1 {
		t.Errorf("unusable arms %v, want the untreated arm named", incomplete[0].Unusable)
	}
	if len(p.Orphans) != 1 || !strings.HasSuffix(p.Orphans[0], "cell-2/sense") {
		t.Errorf("orphans %v, want the run that recorded no terminal state", p.Orphans)
	}
}

// Spend is a fact about a repository, not about the fleet: what one repository
// has burned says nothing about the one beside it, and a ceiling is decided
// about the repository it stops.
func TestSpendIsReportedPerRepositoryAgainstTheCeiling(t *testing.T) {
	p := newTrees(t).
		run("mastodon/1/bench/cell-0/sense", true).
		run("mastodon/1/bench/cell-0/untreated", true).
		run("mastodon/1/minibench/cell-0/sense", true).
		run("discourse/1/bench/cell-0/sense", true).
		read(40)

	spent := map[string]int{}
	for _, r := range p.Repos {
		spent[r.Repo] = r.Spend.Runs()
	}
	if spent["mastodon"] != 2 {
		t.Errorf("mastodon spent %d, want 2: a mini-bench is unpaid", spent["mastodon"])
	}
	if spent["discourse"] != 1 {
		t.Errorf("discourse spent %d, want its own 1 and nothing of mastodon's", spent["discourse"])
	}
	if p.Ceiling != 40 {
		t.Errorf("ceiling %d, want the one it was read with", p.Ceiling)
	}
}

// Asking where the repositories stand before any of them has run is a fair
// question with a short answer, not an error.
func TestARootThatHasNotRunReportsAnEmptyPosition(t *testing.T) {
	p, err := status.Read(filepath.Join(t.TempDir(), "never-started"), 40)
	if err != nil {
		t.Fatalf("an unstarted root was an error: %v", err)
	}
	if len(p.Repos) != 0 || len(p.Cells) != 0 {
		t.Errorf("an unstarted root reported %+v", p)
	}
	page := status.Render(p)
	for _, want := range []string{"nothing has run", "none", "nothing to resume"} {
		if !strings.Contains(page, want) {
			t.Errorf("the empty page does not say %q:\n%s", want, page)
		}
	}
}

func TestAnUnreadableCellRecordIsAnErrorRatherThanAMissingRow(t *testing.T) {
	c := newTrees(t).cellAt("mastodon/1/bench/cell-0", "{not json")
	if _, err := status.Read(c.dir, 40); err == nil {
		t.Fatal("a cell whose record could not be read was silently dropped")
	}
}

func TestAnUnreadableTreeIsAnErrorRatherThanAnEmptyPosition(t *testing.T) {
	dir := t.TempDir()
	locked := filepath.Join(dir, "mastodon")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	if _, err := status.Read(dir, 40); err == nil {
		t.Fatal("a tree that could not be read reported a clean empty position")
	}
}

// A directory that is not a cycle is not an error and not a cycle. The tree
// holds other things and this walk is not the place to rule on them.
func TestADirectoryThatIsNotACycleIsIgnored(t *testing.T) {
	c := newTrees(t).indexed("mastodon").phaseDone("mastodon", 1, phase.Author)
	if err := os.MkdirAll(filepath.Join(c.dir, "mastodon", "notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := c.read(40).Repos[0]
	if r.Cycle != 1 {
		t.Errorf("cycle %d, want 1: a directory that is not a number is not a cycle", r.Cycle)
	}
}

// The page and the crank read one position. They used to work it out
// separately, and a repository admitted and scanned but with no cycle directory
// yet was where they disagreed: this page derived the cycle from the numbered
// directories, found none, and reported cycle 0 awaiting nothing with no resume
// line, while the position the crank routes on read cycle 1 awaiting author.
//
// Measured 2026-08-19 on mastodon, freshly admitted.
func TestAnAdmittedRepositoryReadsTheSameHereAsWhereTheCrankRoutes(t *testing.T) {
	c := newTrees(t).indexed("mastodon")

	r := c.read(40).Repos[0]
	want, err := position.Read(c.dir, "mastodon")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(r.Position, want) {
		t.Errorf("the page reads %+v\nthe crank reads %+v", r.Position, want)
	}
	if r.Cycle != 1 || r.Awaiting != phase.Author {
		t.Errorf("cycle %d awaiting %q, want cycle 1 awaiting author", r.Cycle, r.Awaiting)
	}
}

// And it is resumable: a repository with a phase owed is the whole reason the
// page has a RESUME section, so reporting it with nothing to do is the failure
// that section exists to prevent.
func TestAnAdmittedRepositoryIsHandedItsFirstPhaseToRun(t *testing.T) {
	p := newTrees(t).indexed("mastodon").read(40)

	if len(p.Resume) != 1 {
		t.Fatalf("%d resume lines, want the one repository that owes a phase: %+v", len(p.Resume), p.Resume)
	}
	if p.Resume[0].Repo != "mastodon" || p.Resume[0].Phase != phase.Author {
		t.Errorf("resume %+v, want mastodon at author", p.Resume[0])
	}
}
