package status_test

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/position"
	"github.com/luuuc/sense/lab/internal/status"
)

// A regenerated view that does not say it is a view is one somebody edits into a
// decision. The banner is load-bearing, not decoration.
func TestThePageSaysThePositionIsAuthoritativeOnDiskAndItDecidesNothing(t *testing.T) {
	page := status.Render(newTrees(t).indexed("mastodon").phaseDone("mastodon", 1, phase.Author).read(40), verb)
	first := strings.SplitN(page, "\n", 2)[0]
	for _, want := range []string{"authoritative", "run tree", "decides nothing"} {
		if !strings.Contains(first, want) {
			t.Errorf("the first line does not say %q: %q", want, first)
		}
	}
}

// A status that shows only progress is a status nobody trusts twice.
func TestTheUncomfortableRowsAreOnThePage(t *testing.T) {
	page := status.Render(newTrees(t).
		indexed("chatwoot").
		phaseDone("chatwoot", 6, phase.Handoff).
		cellAt("mastodon/1/bench/cell-0", `{"arms":{"sense":"x"},"complete":false,"burned":["sense"],"unusable":["untreated"]}`).
		run("mastodon/1/bench/cell-1/sense", false).
		read(40), verb)

	for _, want := range []string{
		"PARKED",
		"INCOMPLETE",
		"burned, can never be paired: sense",
		"no result: untreated",
		"ORPHAN",
		"recorded no terminal state",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page hides %q:\n%s", want, page)
		}
	}
}

// The ceiling's scope and window are named on the page rather than left to be
// inferred: per repository, over its lifetime.
func TestSpendIsShownAgainstTheCeilingWithItsScope(t *testing.T) {
	page := status.Render(newTrees(t).
		indexed("mastodon").
		run("mastodon/1/bench/cell-0/sense", true).
		read(7), verb)

	for _, want := range []string{"1 paid runs", "ceiling of 7", "over this repository's lifetime"} {
		if !strings.Contains(page, want) {
			t.Errorf("the spend section does not say %q:\n%s", want, page)
		}
	}
}

func TestTheLoopPositionShowsTheCycleAndTheDistanceToTheCeiling(t *testing.T) {
	page := status.Render(newTrees(t).
		indexed("mastodon").
		phaseDone("mastodon", 4, phase.Author).
		phaseDone("mastodon", 2, phase.Board).
		read(40), verb)

	for _, want := range []string{"cycle 4 of 6", "2 left before the ceiling", "reached author", "awaiting minibench", "banked on cycle [2]"} {
		if !strings.Contains(page, want) {
			t.Errorf("the loop position does not say %q:\n%s", want, page)
		}
	}
}

// The resume line names the plan and the artifact it awaits, so a session with
// no memory of the repository can act on it without asking.
func TestTheResumeLineNamesThePlanAndTheArtifact(t *testing.T) {
	page := status.Render(newTrees(t).
		indexed("mastodon").
		phaseDone("mastodon", 1, phase.Author).
		phaseDone("mastodon", 1, phase.Minibench).
		read(40), verb)
	for _, want := range []string{"RESUME", "mastodon", "expand", "lab/plans/expand.md", "scenario.yaml"} {
		if !strings.Contains(page, want) {
			t.Errorf("the resume line does not say %q:\n%s", want, page)
		}
	}
}

// A repository whose cells are all complete still reports them, because "two
// recorded, none incomplete" is the answer to a question a resuming session has.
func TestCompleteCellsAreCountedAndNotListedAsProblems(t *testing.T) {
	page := status.Render(newTrees(t).
		cellAt("mastodon/1/bench/cell-0", `{"arms":{"sense":"x","untreated":"y"},"complete":true}`).
		read(40), verb)

	if !strings.Contains(page, "1 recorded, 0 of them incomplete") {
		t.Errorf("complete cells are not reported:\n%s", page)
	}
	if strings.Contains(page, "INCOMPLETE") {
		t.Errorf("a complete cell was reported as a problem:\n%s", page)
	}
}

// A phase with no artifact at all is a repository at the start, and the page
// says so plainly instead of leaving a blank column that reads as a bug.
func TestAPhaseThatIsNotThereIsNamedRatherThanLeftBlank(t *testing.T) {
	page := status.Render(status.Position{Root: "x", Repos: []status.Repo{
		{Position: position.Position{Repo: "mastodon", Cycle: 1}}}}, verb)
	if !strings.Contains(page, "reached none") {
		t.Errorf("an absent phase was rendered as a blank:\n%s", page)
	}
}

// Every row something can be done about carries the command that does it. A
// page that says where every repository stands and not what to do about any of
// them makes the reader work out the verb they opened it with.
func TestEveryRowThatCanMoveCarriesTheCommandThatMovesIt(t *testing.T) {
	page := status.Render(newTrees(t).
		indexed("waiting").phaseDone("waiting", 1, phase.Validate).
		indexed("chatwoot").phaseDone("chatwoot", 6, phase.Handoff).
		read(40), verb)

	for _, want := range []string{"sense-lab next waiting"} {
		if !strings.Contains(page, want) {
			t.Errorf("no row carries a command:\n%s", page)
		}
	}
	// A parked repository has nothing to run, and a column filled for its own
	// sake is a page suggesting work that does not exist.
	for _, line := range strings.Split(page, "\n") {
		if strings.Contains(line, "chatwoot") && strings.Contains(line, "sense-lab") {
			t.Errorf("a parked repository is offered a command: %q", line)
		}
	}
}

// verb is what a command layer would render an act into. The page does not know
// the binary's verbs, so a test supplies the same shape the binary does.
func verb(at position.Position) string {
	switch at.Next() {
	case position.Spend:
		return "sense-lab pay " + at.Repo
	case position.Advance:
		return "sense-lab next " + at.Repo
	case position.Diagnose:
		return "sense-lab why " + at.Repo
	default:
		return ""
	}
}
