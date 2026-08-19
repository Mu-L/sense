package crank

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/plans"
	"github.com/luuuc/sense/lab/internal/position"
)

// agent stands in for a phase agent: it writes whatever the test says the agent
// wrote, in the directory it was dispatched to.
//
// Nothing here asserts that it was called. What the crank did next — the record
// it wrote, the position that comes out of it — is the whole subject, and a
// test that asserted a spawn happened would pass against a crank that spawned
// and then ignored the result.
type agent struct {
	verdict  *Verdict
	artifact bool
	stalled  bool
	// jobs is what it was dispatched with, so a test can read what the agent
	// would have been told rather than what the crank meant to tell it.
	jobs []Job
}

func (a *agent) spawn(_ context.Context, j Job) (Ran, error) {
	a.jobs = append(a.jobs, j)
	if err := os.MkdirAll(j.Dir, 0o755); err != nil {
		return Ran{}, err
	}
	log := filepath.Join(j.Dir, "phase.log")
	if err := os.WriteFile(log, []byte("what the agent said\n"), 0o644); err != nil {
		return Ran{}, err
	}
	if a.stalled {
		return Ran{Finished: false, Log: log}, nil
	}
	if a.verdict != nil {
		b, err := json.Marshal(a.verdict)
		if err != nil {
			return Ran{}, err
		}
		if err := os.WriteFile(filepath.Join(j.Dir, VerdictFile), b, 0o644); err != nil {
			return Ran{}, err
		}
	}
	if a.artifact {
		p, _ := phase.Lookup(j.Phase)
		if err := os.WriteFile(filepath.Join(j.Dir, p.Writes), []byte("the artifact\n"), 0o644); err != nil {
			return Ran{}, err
		}
	}
	return Ran{Finished: true, Log: log}, nil
}

// declared is the shipped plan set, loaded from lab/plans. The tests run against
// the real declarations rather than a hand-written copy: a plan set that
// disagreed with the graph would then fail here as well as in its own package.
func declared(t *testing.T) []plans.Plan {
	t.Helper()
	loaded, err := plans.Load(filepath.Join("..", "..", "plans"))
	if err != nil {
		t.Fatal(err)
	}
	return loaded
}

// campaign is a repository admitted and scanned, ready for its first authoring
// phase.
func campaign(t *testing.T) (dir, repo string) {
	t.Helper()
	dir, repo = t.TempDir(), "jellyfin"
	write(t, filepath.Join(dir, repo, "index", "index.json"), `{"repo":"jellyfin","symbols":11410}`)
	return dir, repo
}

func cranked(t *testing.T, dir string, a *agent) Crank {
	t.Helper()
	return Crank{Campaign: dir, Plans: declared(t), Checkout: t.TempDir(), Spawn: a.spawn}
}

func advance(t *testing.T, c Crank, repo string) Result {
	t.Helper()
	r, err := c.Advance(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// The turn: the crank runs the phase the position owes, and the position after
// it is the next phase. Nothing about this asserts that an agent was spawned —
// it asserts that the loop moved, which is the only reason to spawn one.
func TestOneTurnRunsThePhaseThatIsOwedAndMovesOn(t *testing.T) {
	dir, repo := campaign(t)
	a := &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo, Cycle: 1,
		Verdict: phase.Draft, Anchor: "BaseItem"}}

	r := advance(t, cranked(t, dir, a), repo)

	if r.Ran != phase.Author {
		t.Errorf("ran %s, want the phase the position owed", r.Ran)
	}
	if r.After.Awaiting != phase.Minibench {
		t.Errorf("awaiting %s (%s), want the phase the verdict routes to", r.After.Awaiting, r.After.Because)
	}
	if r.After.Standing != position.Ready {
		t.Errorf("standing %s, want more to do", r.After.Standing)
	}
}

// One phase per invocation. `-until` is a loop in main around this call, so a
// turn that ran two phases would be a mode nobody asked for.
func TestOneTurnRunsOnePhase(t *testing.T) {
	dir, repo := campaign(t)
	a := &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo, Cycle: 1, Verdict: phase.Draft}}

	advance(t, cranked(t, dir, a), repo)

	if len(a.jobs) != 1 {
		t.Errorf("dispatched %d phases, want one", len(a.jobs))
	}
	if recorded := attempts(t, dir, repo); len(recorded) != 1 {
		t.Errorf("recorded %d attempts, want the one that ran", len(recorded))
	}
}

// Every lever cycle 05 declared, driven end to end with no agent, no network
// and no spend. A lever that routes somewhere the graph does not say would show
// up here as a position, which is what the crank is for.
func TestEveryLeverRoutesTheWayTheGraphDeclares(t *testing.T) {
	for _, lever := range phase.Levers {
		if spending[lever.From] {
			// A lever pulled by a phase that spends cannot be driven by the
			// crank this cycle, and is asserted on where it actually stops.
			continue
		}
		t.Run(lever.Name, func(t *testing.T) {
			dir, repo := campaign(t)
			// The lever's cycle is a fact about the tree, so the tree is built
			// with it rather than a counter being set.
			for c := 1; c < lever.Cycle; c++ {
				write(t, filepath.Join(dir, repo, strconv.Itoa(c), "author", "scenario.draft.yaml"), "name: r\n")
			}
			reach(t, dir, repo, lever.Cycle, lever.From)
			a := &agent{artifact: true, verdict: &Verdict{Phase: lever.From, Repo: repo,
				Cycle: lever.Cycle, Verdict: lever.Verdict, Table: "what this attempt has to answer"}}

			r := advance(t, cranked(t, dir, a), repo)

			if r.Ran != lever.From {
				t.Fatalf("ran %s, want %s", r.Ran, lever.From)
			}
			if r.After.Awaiting != lever.To {
				t.Errorf("awaiting %s (%s), want %s", r.After.Awaiting, r.After.Because, lever.To)
			}
		})
	}
}

// The phases that spend are not run, and the list is written down here rather
// than read from the one the crank uses. A test that asked the same map would
// move an entry between its own branches when that map lost one, and pass.
func TestNoPhaseThatSpendsIsEverRun(t *testing.T) {
	for _, spends := range []phase.Name{phase.Bench, phase.Report, phase.Harvest, phase.Board} {
		t.Run(string(spends), func(t *testing.T) {
			dir, repo := campaign(t)
			reach(t, dir, repo, 1, spends)
			a := &agent{}

			r := advance(t, cranked(t, dir, a), repo)

			if len(a.jobs) != 0 {
				t.Errorf("the crank dispatched %s, which spends", spends)
			}
			if r.After.Standing != position.Waiting {
				t.Errorf("standing %s (%s), want it waiting on a human", r.After.Standing, r.After.Because)
			}
			if !strings.Contains(r.Note, "by hand") {
				t.Errorf("note = %q, want it to say what to run", r.Note)
			}
		})
	}
}

// The levers past the pay call are not driven by the crank at all, and it says
// so rather than dispatching a phase that spends. The DoD FAIL lever is the one
// this leaves untested end to end, and it is untested because it is unreachable
// rather than because it was skipped.
func TestALeverPulledByAPhaseThatSpendsIsNotDrivenAtAll(t *testing.T) {
	for _, lever := range phase.Levers {
		if !spending[lever.From] {
			continue
		}
		t.Run(lever.Name, func(t *testing.T) {
			dir, repo := campaign(t)
			reach(t, dir, repo, 1, lever.From)
			a := &agent{}

			r := advance(t, cranked(t, dir, a), repo)

			if len(a.jobs) != 0 {
				t.Errorf("the crank dispatched %s, which spends", lever.From)
			}
			if r.After.Standing != position.Waiting {
				t.Errorf("standing %s (%s), want it waiting on a human", r.After.Standing, r.After.Because)
			}
			if !strings.Contains(r.Note, "by hand") {
				t.Errorf("note = %q, want it to say what to run", r.Note)
			}
		})
	}
}

// The ceiling parks rather than opening a seventh authoring cycle, and a parked
// repository does not advance again.
func TestTheCeilingParksAndAParkedRepositoryDoesNotAdvance(t *testing.T) {
	dir, repo := campaign(t)
	// Five finished authoring cycles and a sixth open: the position's cycle is a
	// directory name, so the ceiling is reached by the tree rather than set.
	for c := 1; c < phase.AuthoringCeiling; c++ {
		write(t, filepath.Join(dir, repo, strconv.Itoa(c), "author", "scenario.draft.yaml"), "name: r\n")
	}
	if err := os.MkdirAll(filepath.Join(dir, repo, strconv.Itoa(phase.AuthoringCeiling)), 0o755); err != nil {
		t.Fatal(err)
	}
	a := &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo,
		Cycle: phase.AuthoringCeiling, Verdict: phase.NoAnchor, Table: "no symbol carries the question"}}
	c := cranked(t, dir, a)

	if r := advance(t, c, repo); r.After.Standing != position.Parked {
		t.Fatalf("standing %s (%s), want parked at the ceiling", r.After.Standing, r.After.Because)
	}

	spawned := len(a.jobs)
	r := advance(t, c, repo)

	if len(a.jobs) != spawned {
		t.Error("a parked repository advanced; resuming one is a human act, not a transition")
	}
	if r.Ran != "" {
		t.Errorf("ran %s on a parked repository", r.Ran)
	}
}

// The automation stops at the pay call: it prints the command and does not run
// it. Asserted on the position rather than on whether a spawner was called,
// because what matters is that nothing moved.
func TestAPayPrintsTheCommandAndSpendsNothing(t *testing.T) {
	dir, repo := campaign(t)
	reach(t, dir, repo, 1, phase.Validate)
	write(t, filepath.Join(dir, repo, "1", "validate", "pay-call.md"), "PAY\n")
	attempt(t, dir, repo, position.Attempt{Cycle: 1, Phase: phase.Validate, Verdict: phase.Pay, Try: 1})
	a := &agent{}
	c := cranked(t, dir, a)
	before := advance(t, c, repo).Before

	r := advance(t, c, repo)

	if len(a.jobs) != 0 {
		t.Error("the crank dispatched a phase past the pay call")
	}
	if r.After.Standing != position.Waiting {
		t.Errorf("standing %s, want it waiting on a human", r.After.Standing)
	}
	if !strings.Contains(r.Note, "sense-lab probe") {
		t.Errorf("note = %q, want the command to run by hand", r.Note)
	}
	if r.After.Awaiting != before.Awaiting {
		t.Errorf("awaiting moved from %s to %s; nothing should have", before.Awaiting, r.After.Awaiting)
	}
}

// A phase whose agent runs past its wall is recorded as out of clock, and the
// crank moves on rather than holding. Cannot-finish-at-budget is a result.
func TestAPhaseThatRanPastItsWallIsRecordedRatherThanWaitedOn(t *testing.T) {
	dir, repo := campaign(t)
	a := &agent{stalled: true}

	r := advance(t, cranked(t, dir, a), repo)

	if r.After.Standing != position.Missing {
		t.Fatalf("standing %s (%s), want it recorded as an agent that did not finish", r.After.Standing, r.After.Because)
	}
	if !strings.Contains(r.After.Because, "phase.log") {
		t.Errorf("because = %q, want it to name the log", r.After.Because)
	}
	recorded := attempts(t, dir, repo)
	if len(recorded) != 1 || recorded[0].Outcome != position.Stalled {
		t.Errorf("recorded %+v, want one stalled attempt", recorded)
	}
}

// The wall is read from the plan's own header, so changing what a phase gets is
// an edit to a declaration rather than to code.
func TestTheWallComesFromTheDeclarationRatherThanFromCode(t *testing.T) {
	for _, tc := range []struct {
		name phase.Name
		wall time.Duration
	}{
		// Two phases, because one plan is first in the loaded set and a wall
		// read off the wrong plan would look right for that one alone.
		{phase.Author, 4 * time.Minute},
		{phase.Minibench, 11 * time.Minute},
	} {
		t.Run(string(tc.name), func(t *testing.T) {
			dir, repo := campaign(t)
			reach(t, dir, repo, 1, tc.name)
			a := &agent{artifact: true, verdict: &Verdict{Phase: tc.name, Repo: repo, Cycle: 1,
				Verdict: firstEmitted(t, tc.name)}}
			c := cranked(t, dir, a)
			// The same crank, the same code, one line of the declaration
			// different.
			c.Plans = withWall(t, c.Plans, tc.name, tc.wall)

			advance(t, c, repo)

			if a.jobs[0].Wall != tc.wall {
				t.Errorf("wall = %s, want what the plan declares (%s)", a.jobs[0].Wall, tc.wall)
			}
		})
	}
}

// firstEmitted is a verdict the phase may emit, taken from the graph so a test
// never hand-copies an enum.
func firstEmitted(t *testing.T, name phase.Name) phase.Verdict {
	t.Helper()
	p, ok := phase.Lookup(name)
	if !ok || len(p.Verdicts) == 0 {
		t.Fatalf("no verdicts declared for %s", name)
	}
	return p.Verdicts[0]
}

// The guard: three checks about what the phase says about itself, and one about
// what is on disk. A verdict about another phase or another repository routes
// this one, and a verdict outside the declared enum is not a verdict at all.
func TestTheGuardRefusesAVerdictThatIsNotThisPhasesOwn(t *testing.T) {
	for _, tc := range []struct {
		name    string
		verdict *Verdict
		reason  string
	}{
		{"it names another phase",
			&Verdict{Phase: phase.Minibench, Repo: "jellyfin", Cycle: 1, Verdict: phase.Proceed}, "another phase"},
		{"it names another repository",
			&Verdict{Phase: phase.Author, Repo: "discourse", Cycle: 1, Verdict: phase.Draft}, "another\nrepository"},
		{"it names another cycle",
			&Verdict{Phase: phase.Author, Repo: "jellyfin", Cycle: 4, Verdict: phase.Draft}, "another attempt"},
		{"it emits what the plan does not declare",
			&Verdict{Phase: phase.Author, Repo: "jellyfin", Cycle: 1, Verdict: phase.WinConfirmed}, "does not declare"},
		{"there is no verdict document at all", nil, "has not finished"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, repo := campaign(t)
			a := &agent{artifact: true, verdict: tc.verdict}

			r := advance(t, cranked(t, dir, a), repo)

			if r.After.Standing != position.Unusable {
				t.Fatalf("standing %s (%s), want it refused", r.After.Standing, r.After.Because)
			}
			for _, want := range strings.Split(tc.reason, "\n") {
				if !strings.Contains(r.After.Because, want) {
					t.Errorf("because = %q, want it to say %q", r.After.Because, want)
				}
			}
		})
	}
}

// The declared enum is read off the plan file rather than a list held in Go. A
// copy of that list here is a list that disagrees with the file somebody edits,
// so the check is driven by editing the declaration: a verdict the graph knows
// and the plan no longer declares is refused, and the refusal quotes the plan.
func TestTheDeclaredEnumIsReadFromThePlanFile(t *testing.T) {
	dir, repo := campaign(t)
	a := &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo, Cycle: 1, Verdict: phase.Draft}}
	c := cranked(t, dir, a)
	c.Plans = onlyEmits(t, c.Plans, phase.Author, phase.NoAnchor)

	r := advance(t, c, repo)

	if r.After.Standing != position.Unusable {
		t.Fatalf("standing %s (%s), want it refused against the plan's own list", r.After.Standing, r.After.Because)
	}
	if !strings.Contains(r.After.Because, "NO-ANCHOR") {
		t.Errorf("because = %q, want it to quote what the plan declares", r.After.Because)
	}
}

// An exit code is a claim; the artifact is the fact. A phase that finished, said
// the right thing and wrote nothing does not advance.
func TestAPhaseThatWroteNoArtifactDoesNotAdvance(t *testing.T) {
	dir, repo := campaign(t)
	a := &agent{verdict: &Verdict{Phase: phase.Author, Repo: repo, Cycle: 1, Verdict: phase.Draft}}

	r := advance(t, cranked(t, dir, a), repo)

	if r.After.Standing != position.Missing {
		t.Fatalf("standing %s (%s), want it held at the missing artifact", r.After.Standing, r.After.Because)
	}
	if r.After.Awaiting == phase.Minibench {
		t.Error("the loop moved on from a phase that wrote nothing")
	}
	if recorded := attempts(t, dir, repo); recorded[0].Artifact != "" {
		t.Errorf("recorded artifact %q, want none: nothing was accepted", recorded[0].Artifact)
	}
}

// Each advance records what it READ as well as what it decided, so a park
// opened six months later shows the chain that produced it rather than a phase
// name and a date.
func TestAnAdvanceRecordsThePlanTheVerdictAndTheArtifact(t *testing.T) {
	dir, repo := campaign(t)
	a := &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo, Cycle: 1, Verdict: phase.Draft}}

	advance(t, cranked(t, dir, a), repo)

	recorded := attempts(t, dir, repo)[0]
	for _, tc := range []struct{ what, got string }{
		{"plan", recorded.Plan},
		{"verdict document", recorded.VerdictDoc},
		{"artifact", recorded.Artifact},
		{"log", recorded.Log},
	} {
		if tc.got == "" {
			t.Errorf("no %s recorded", tc.what)
			continue
		}
		if _, err := os.Stat(tc.got); err != nil {
			t.Errorf("recorded %s %q, which is not there: %v", tc.what, tc.got, err)
		}
	}
}

// A re-entry keeps the anchor and carries the rejection, and the next attempt
// reads both — every rejection, oldest first, because reading only the latest is
// how six attempts oscillate between two failures without landing in between.
func TestAReEntryKeepsTheAnchorAndCarriesEveryRejection(t *testing.T) {
	dir, repo := campaign(t)
	c := cranked(t, dir, &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo,
		Cycle: 1, Verdict: phase.Draft, Anchor: "MediaBrowser.Controller.Entities.BaseItem"}})
	advance(t, c, repo)

	// The mini-bench sends it back, and says why.
	c.Spawn = (&agent{artifact: true, verdict: &Verdict{Phase: phase.Minibench, Repo: repo, Cycle: 1,
		Verdict: phase.Requestion, Table: "the baseline cited 4 of 4 rows"}}).spawn
	advance(t, c, repo)

	// The next author attempt is what has to read them.
	next := &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo, Cycle: 1, Verdict: phase.Draft}}
	c.Spawn = next.spawn
	advance(t, c, repo)

	if len(next.jobs) != 1 {
		t.Fatalf("dispatched %d, want the re-entered author", len(next.jobs))
	}
	prompt := next.jobs[0].Prompt
	if !strings.Contains(prompt, "anchor: MediaBrowser.Controller.Entities.BaseItem") {
		t.Errorf("the re-entry lost the anchor:\n%s", prompt)
	}
	if !strings.Contains(prompt, "the baseline cited 4 of 4 rows") {
		t.Errorf("the re-entry lost the rejection it has to answer:\n%s", prompt)
	}
}

// And the loop moves ON from the re-entered phase.
//
// This is the failure that made `-until` spin: read back in graph order, the
// mini-bench that sent work back sorts after the author attempt that answered
// it, so the position routes from the rejection every turn and the author runs
// forever, at whatever an agent costs. The turn after a re-entry is the one
// nothing was checking.
func TestTheLoopMovesOnFromAReEnteredPhase(t *testing.T) {
	dir, repo := campaign(t)
	c := cranked(t, dir, &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo,
		Cycle: 1, Verdict: phase.Draft, Anchor: "BaseItem"}})
	advance(t, c, repo)
	c.Spawn = (&agent{artifact: true, verdict: &Verdict{Phase: phase.Minibench, Repo: repo, Cycle: 1,
		Verdict: phase.Requestion, Table: "the baseline cited 4 of 4 rows"}}).spawn
	advance(t, c, repo)

	again := &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo, Cycle: 1, Verdict: phase.Draft}}
	c.Spawn = again.spawn
	r := advance(t, c, repo)

	if r.After.Awaiting != phase.Minibench {
		t.Fatalf("awaiting %s (%s), want the phase after the re-entered author", r.After.Awaiting, r.After.Because)
	}
	// And one more turn goes to the mini-bench rather than back to the author.
	last := &agent{artifact: true, verdict: &Verdict{Phase: phase.Minibench, Repo: repo, Cycle: 1,
		Verdict: phase.Proceed}}
	c.Spawn = last.spawn
	if r := advance(t, c, repo); r.Ran != phase.Minibench {
		t.Errorf("ran %s, want the loop to have moved on", r.Ran)
	}
}

// The phase is dispatched against the repository under study. An agent pointed
// at nothing reads nothing, and its draft would be about a repository it never
// opened.
func TestThePhaseIsDispatchedAgainstTheCheckout(t *testing.T) {
	dir, repo := campaign(t)
	a := &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo, Cycle: 1, Verdict: phase.Draft}}
	c := cranked(t, dir, a)

	advance(t, c, repo)

	if a.jobs[0].Checkout != c.Checkout {
		t.Errorf("checkout = %q, want the repository under study (%q)", a.jobs[0].Checkout, c.Checkout)
	}
}

// The plan reaches the agent verbatim, with the facts of the attempt in front
// of it. Nothing here composes a method: a sentence added in code is a method
// nobody can review by reading a file.
func TestThePlanReachesTheAgentVerbatim(t *testing.T) {
	dir, repo := campaign(t)
	a := &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo, Cycle: 1, Verdict: phase.Draft}}
	c := cranked(t, dir, a)
	author, _ := planFor(c.Plans, phase.Author)

	advance(t, c, repo)

	if !strings.HasSuffix(a.jobs[0].Prompt, string(author.Body)) {
		t.Error("the plan did not reach the agent unchanged")
	}
	if !strings.Contains(a.jobs[0].Prompt, "verdict: ") {
		t.Error("the agent was not told where its verdict belongs")
	}
}

// A repository with nothing owed is not dispatched against.
func TestARepositoryThatIsNotReadyIsNotDispatchedAgainst(t *testing.T) {
	dir, repo := campaign(t)
	write(t, filepath.Join(dir, repo, "1", "board", "board.md"), "# board\n")
	a := &agent{}

	r := advance(t, cranked(t, dir, a), repo)

	if len(a.jobs) != 0 {
		t.Error("a finished repository was dispatched against")
	}
	if !strings.Contains(r.Note, "finished") {
		t.Errorf("note = %q, want it to say why nothing ran", r.Note)
	}
}

// A phase with no plan is a phase nobody can run, and that is a refusal rather
// than a dispatch with an empty prompt.
func TestAPhaseWithNoPlanIsRefused(t *testing.T) {
	dir, repo := campaign(t)
	c := cranked(t, dir, &agent{})
	c.Plans = without(c.Plans, phase.Author)

	if _, err := c.Advance(context.Background(), repo); err == nil {
		t.Fatal("a phase with no plan was dispatched")
	}
}

// A spawner that fails is the crank's failure, not a verdict.
func TestASpawnerThatFailsIsNotAVerdict(t *testing.T) {
	dir, repo := campaign(t)
	c := cranked(t, dir, &agent{})
	c.Spawn = func(context.Context, Job) (Ran, error) { return Ran{}, os.ErrPermission }

	if _, err := c.Advance(context.Background(), repo); err == nil {
		t.Fatal("a spawner that failed was read as a phase that decided something")
	}
	if recorded := attempts(t, dir, repo); len(recorded) != 0 {
		t.Errorf("recorded %+v for a phase that never ran", recorded)
	}
}

// An unreadable position is an error rather than a repository at the start of
// its first cycle.
func TestAnUnreadablePositionIsAnError(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "jellyfin"), "not a directory")

	if _, err := cranked(t, dir, &agent{}).Advance(context.Background(), "jellyfin"); err == nil {
		t.Fatal("a file where a repository should be was cranked")
	}
}

// reach writes every artifact up to and including the phase before `to`, which
// is what makes that phase the one owed.
func reach(t *testing.T, campaign, repo string, cycle int, to phase.Name) {
	t.Helper()
	// The cycle is a directory name, so an open cycle with nothing written in
	// it is a directory that exists and is empty.
	if err := os.MkdirAll(filepath.Join(campaign, repo, strconv.Itoa(cycle)), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range phase.Graph {
		if p.Name == to {
			return
		}
		if p.Name == phase.Index || p.Name == phase.Handoff {
			continue
		}
		write(t, filepath.Join(campaign, repo, strconv.Itoa(cycle), string(p.Name), p.Writes), "written\n")
	}
	t.Fatalf("%s is not a phase the graph walks to", to)
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func attempt(t *testing.T, campaign, repo string, a position.Attempt) {
	t.Helper()
	if err := position.Record(filepath.Join(campaign, repo), a); err != nil {
		t.Fatal(err)
	}
}

func attempts(t *testing.T, campaign, repo string) []position.Attempt {
	t.Helper()
	all, err := position.Attempts(filepath.Join(campaign, repo))
	if err != nil {
		t.Fatal(err)
	}
	return all
}

// withWall and withEmits change one line of one declaration, which is what a
// person editing a plan file does.
func withWall(t *testing.T, loaded []plans.Plan, name phase.Name, wall time.Duration) []plans.Plan {
	t.Helper()
	out := append([]plans.Plan(nil), loaded...)
	for i := range out {
		if out[i].Phase == name {
			out[i].Wall = wall
			return out
		}
	}
	t.Fatalf("no plan for %s", name)
	return nil
}

func onlyEmits(t *testing.T, loaded []plans.Plan, name phase.Name, v ...phase.Verdict) []plans.Plan {
	t.Helper()
	out := append([]plans.Plan(nil), loaded...)
	for i := range out {
		if out[i].Phase == name {
			out[i].Emits = v
			return out
		}
	}
	t.Fatalf("no plan for %s", name)
	return nil
}

func without(loaded []plans.Plan, name phase.Name) []plans.Plan {
	var out []plans.Plan
	for _, p := range loaded {
		if p.Phase != name {
			out = append(out, p)
		}
	}
	return out
}

// A verdict document that is not a document is refused like any other thing
// that is not a verdict, and the loop stops rather than routing on it.
func TestAVerdictThatIsNotADocumentIsRefused(t *testing.T) {
	dir, repo := campaign(t)
	c := cranked(t, dir, &agent{artifact: true})
	// The agent writes an artifact and something unreadable where its verdict
	// belongs, which is what a half-finished write looks like.
	c.Spawn = func(_ context.Context, j Job) (Ran, error) {
		if err := os.MkdirAll(j.Dir, 0o755); err != nil {
			return Ran{}, err
		}
		write(t, filepath.Join(j.Dir, VerdictFile), "{not a document")
		p, _ := phase.Lookup(j.Phase)
		write(t, filepath.Join(j.Dir, p.Writes), "the artifact\n")
		return Ran{Finished: true, Log: filepath.Join(j.Dir, "phase.log")}, nil
	}

	r := advance(t, c, repo)

	if r.After.Standing != position.Unusable {
		t.Fatalf("standing %s (%s), want it refused", r.After.Standing, r.After.Because)
	}
}

// A second attempt at one phase is told which attempt it is, so a spawner can
// keep the first's evidence rather than writing over it.
func TestASecondAttemptAtOnePhaseIsToldWhichItIs(t *testing.T) {
	dir, repo := campaign(t)
	attempt(t, dir, repo, position.Attempt{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1})
	// The artifact is missing, so the position holds at the author. A human
	// clearing that record is not this test's subject: what matters is that the
	// next dispatch knows it is the second.
	a := &agent{artifact: true, verdict: &Verdict{Phase: phase.Author, Repo: repo, Cycle: 1, Verdict: phase.Draft}}
	c := cranked(t, dir, a)
	c.Campaign = dir

	// The position is Missing, so nothing dispatches. The try is still what the
	// records say it is, and that is what the spawner would be handed.
	if got := c.try(repo, 1, phase.Author); got != 2 {
		t.Errorf("try = %d, want the second", got)
	}
}

// An unreadable record set means nothing is known about earlier tries, and the
// dispatch is the first one rather than a failure inside a counter.
func TestAnUnreadableRecordSetCountsAsNoEarlierTries(t *testing.T) {
	dir, repo := campaign(t)
	write(t, filepath.Join(dir, repo, "attempts", "1-author-1.json"), "{not a document")

	if got := cranked(t, dir, &agent{}).try(repo, 1, phase.Author); got != 1 {
		t.Errorf("try = %d, want the first", got)
	}
}
