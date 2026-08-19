package position

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/phase"
)

// fixture is the committed campaign: three repositories, one of each shape a
// real one takes.
const fixture = "testdata/campaign"

func read(t *testing.T, campaign, repo string) Position {
	t.Helper()
	p, err := Read(campaign, repo)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// The three shapes, read from the tree the jellyfin campaign's layout was copied
// from. Each is asserted whole rather than field by field across three tests,
// because what matters is that one tree produces one coherent answer.
func TestPositionIsReadFromTheCommittedTree(t *testing.T) {
	for _, tc := range []struct {
		repo     string
		cycle    int
		reached  phase.Name
		awaiting phase.Name
		standing Standing
		last     phase.Verdict
		answers  int
		because  string
	}{
		{"midcycle", 2, phase.Minibench, phase.Author, Ready, phase.Requestion, 2, "routes to author"},
		{"parked", 6, phase.Author, phase.Done, Parked, phase.NoAnchor, 1, "handoff"},
		{"atpay", 1, phase.Validate, phase.Bench, Waiting, phase.Pay, 0, "human"},
		{"banked", 2, phase.Author, phase.Minibench, Ready, phase.Draft, 0, "routes to minibench"},
	} {
		t.Run(tc.repo, func(t *testing.T) {
			p := read(t, fixture, tc.repo)

			if p.Cycle != tc.cycle {
				t.Errorf("cycle = %d, want %d", p.Cycle, tc.cycle)
			}
			if p.Reached != tc.reached {
				t.Errorf("reached = %s, want %s", p.Reached, tc.reached)
			}
			if p.Awaiting != tc.awaiting {
				t.Errorf("awaiting = %s, want %s", p.Awaiting, tc.awaiting)
			}
			if p.Standing != tc.standing {
				t.Errorf("standing = %s (%s), want %s", p.Standing, p.Because, tc.standing)
			}
			if p.Last.Verdict != tc.last {
				t.Errorf("last verdict = %q, want %q: the position is read from the last one recorded in this cycle",
					p.Last.Verdict, tc.last)
			}
			if len(p.Answer) != tc.answers {
				t.Errorf("carried rejections = %d, want %d", len(p.Answer), tc.answers)
			}
			if !strings.Contains(p.Because, tc.because) {
				t.Errorf("because = %q, want it to say %q; a stopped loop is read by whoever finds it", p.Because, tc.because)
			}
			if !p.Indexed {
				t.Error("indexed = false; every repository in the fixture has been scanned")
			}
		})
	}
}

// A repository that banked a cycle and opened another has work to do. Banking
// is per cycle, and a check that asked only whether anything had ever been
// banked would call every re-entry finished.
func TestBankingOneCycleDoesNotFinishTheNextOne(t *testing.T) {
	p := read(t, fixture, "banked")

	if p.Standing != Ready {
		t.Errorf("standing = %s (%s), want a repository with a phase to run", p.Standing, p.Because)
	}
	if len(p.Banked) != 1 || p.Banked[0] != 1 {
		t.Errorf("banked = %v, want the one cycle that reached the board", p.Banked)
	}
}

// The rejection is carried with its table, and the table is what the next
// attempt has to answer. A re-entry that lost it is a fresh guess wearing the
// previous attempt's number.
func TestEveryRejectionIsCarriedWithTheTableThatProducedIt(t *testing.T) {
	p := read(t, fixture, "midcycle")

	if len(p.Answer) != 2 {
		t.Fatalf("carried = %+v, want both re-questions", p.Answer)
	}
	// Oldest first, and both of them: the next attempt reads all of them, not
	// only the last. Six attempts converging is what that makes possible, and
	// six unrelated drafts is what it prevents.
	if a := p.Answer[0]; a.Cycle != 1 || a.Phase != phase.Minibench || !strings.Contains(a.Table, "4 of 4") {
		t.Errorf("first carried = %+v, want cycle 1's mini-bench with its table", a)
	}
	if a := p.Answer[1]; a.Cycle != 2 || !strings.Contains(a.Table, "no more discriminating") {
		t.Errorf("second carried = %+v, want cycle 2's mini-bench with its own table", a)
	}
}

// The cycle is read from the directories, and a record that names a cycle
// nobody opened does not move it. The tree says where a repository is; a record
// says what was judged there.
func TestTheDirectoriesDecideTheCycleAndTheRecordsDoNot(t *testing.T) {
	campaign := t.TempDir()
	repo := filepath.Join(campaign, "r")
	writeArtifact(t, filepath.Join(repo, "index", "index.json"), "{}")
	writeArtifact(t, filepath.Join(repo, "1", "author", "scenario.draft.yaml"), "name: r\n")
	writeArtifact(t, filepath.Join(repo, "2", "author", "scenario.draft.yaml"), "name: r\n")
	record(t, repo, Attempt{Cycle: 2, Phase: phase.Author, Verdict: phase.Draft, Try: 1})
	// A verdict recorded for a cycle whose directory was never opened.
	record(t, repo, Attempt{Cycle: 3, Phase: phase.Author, Verdict: phase.NoAnchor, Try: 1})

	p := read(t, campaign, "r")

	if p.Cycle != 2 {
		t.Errorf("cycle = %d, want the highest directory on disk", p.Cycle)
	}
	if p.Standing != Ready || p.Awaiting != phase.Minibench {
		t.Errorf("standing = %s awaiting %s; the cycle-3 record decided the position", p.Standing, p.Awaiting)
	}
}

// A repository that has opened no cycle is in its first. Reporting cycle 0
// would be a number no rule is written in: the ceiling counts from 1.
func TestAFreshlyIndexedRepositoryIsInItsFirstCycle(t *testing.T) {
	campaign := t.TempDir()
	writeArtifact(t, filepath.Join(campaign, "r", "index", "index.json"), "{}")

	p := read(t, campaign, "r")

	if p.Cycle != 1 || p.Awaiting != phase.Author {
		t.Errorf("cycle %d awaiting %s, want cycle 1 awaiting the author", p.Cycle, p.Awaiting)
	}
	if p.ToCeiling() != phase.AuthoringCeiling-1 {
		t.Errorf("to the ceiling = %d, want %d", p.ToCeiling(), phase.AuthoringCeiling-1)
	}
}

// Nothing under a cycle can be believed before the repository is scanned, so an
// unscanned one waits on its index whatever else is on disk.
func TestAnUnscannedRepositoryWaitsOnItsIndex(t *testing.T) {
	campaign := t.TempDir()
	writeArtifact(t, filepath.Join(campaign, "r", "1", "author", "scenario.draft.yaml"), "name: r\n")

	p := read(t, campaign, "r")

	if p.Indexed || p.Awaiting != phase.Index {
		t.Errorf("indexed %v awaiting %s, want it waiting on the scan", p.Indexed, p.Awaiting)
	}
}

// A repository that is not there at all is a fair question with an answer.
func TestARepositoryThatWasNeverAdmittedIsAPositionRatherThanAnError(t *testing.T) {
	p := read(t, t.TempDir(), "never-heard-of-it")

	if p.Awaiting != phase.Index || p.Standing != Ready {
		t.Errorf("position = %+v, want it awaiting its index", p)
	}
}

// The two refusals are diagnosed differently — an agent that misbehaved against
// an agent that died — so they are separate standings and separate codes.
func TestTheTwoRefusalsAreToldApart(t *testing.T) {
	for _, tc := range []struct {
		name     string
		verdict  phase.Verdict
		artifact bool
		want     Standing
	}{
		{"a verdict the phase cannot emit", phase.Win, true, Unusable},
		{"a usable verdict whose artifact is not there", phase.Draft, false, Missing},
		{"a usable verdict that wrote its artifact", phase.Draft, true, Ready},
	} {
		t.Run(tc.name, func(t *testing.T) {
			campaign := t.TempDir()
			repo := filepath.Join(campaign, "r")
			writeArtifact(t, filepath.Join(repo, "index", "index.json"), "{}")
			if err := os.MkdirAll(filepath.Join(repo, "1"), 0o755); err != nil {
				t.Fatal(err)
			}
			if tc.artifact {
				writeArtifact(t, filepath.Join(repo, "1", "author", "scenario.draft.yaml"), "name: r\n")
			}
			record(t, repo, Attempt{Cycle: 1, Phase: phase.Author, Verdict: tc.verdict, Try: 1})

			p := read(t, campaign, "r")

			if p.Standing != tc.want {
				t.Errorf("standing = %s (%s), want %s", p.Standing, p.Because, tc.want)
			}
		})
	}
}

// A repository on the board is finished, and it says so from the board on disk
// rather than from the last thing anybody recorded about it.
func TestARepositoryOnTheBoardIsFinished(t *testing.T) {
	campaign := t.TempDir()
	repo := filepath.Join(campaign, "r")
	writeArtifact(t, filepath.Join(repo, "index", "index.json"), "{}")
	writeArtifact(t, filepath.Join(repo, "1", "board", "board.md"), "# board\n")

	p := read(t, campaign, "r")

	if p.Standing != Finished || p.Awaiting != phase.Done {
		t.Errorf("standing = %s awaiting %s, want a finished repository", p.Standing, p.Awaiting)
	}
	if len(p.Banked) != 1 || p.Banked[0] != 1 {
		t.Errorf("banked = %v, want cycle 1", p.Banked)
	}
}

// A parked repository is parked whatever a later record says, because no
// transition re-enters one: resuming is a human act.
func TestParkingBeatsAnyVerdictRecordedAfterIt(t *testing.T) {
	campaign := t.TempDir()
	repo := filepath.Join(campaign, "r")
	writeArtifact(t, filepath.Join(repo, "index", "index.json"), "{}")
	writeArtifact(t, filepath.Join(repo, "1", "handoff", "handoff.md"), "# handoff\n")
	writeArtifact(t, filepath.Join(repo, "1", "author", "scenario.draft.yaml"), "name: r\n")
	record(t, repo, Attempt{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1})

	p := read(t, campaign, "r")

	if p.Standing != Parked {
		t.Errorf("standing = %s, want %s", p.Standing, Parked)
	}
}

// An unreadable tree is reported rather than read as an empty position, which
// would be a repository at the start of its first cycle.
func TestAnUnreadableTreeIsAnError(t *testing.T) {
	campaign := t.TempDir()
	blocked := filepath.Join(campaign, "r")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Read(campaign, "r"); err == nil {
		t.Fatal("a file where a repository should be read as a position")
	}
}

// The page shows every rejection rather than the most recent one. A page that
// showed the last would invite the next attempt to answer one thing.
func TestTheRenderedPageCarriesEveryRejection(t *testing.T) {
	page := Render(read(t, fixture, "midcycle"))

	for _, want := range []string{"repo:     midcycle", "cycle:    2 of 6", "reached:  minibench",
		"awaiting: author", "4 of 4", "no more discriminating"} {
		if !strings.Contains(page, want) {
			t.Errorf("page = %q, want it to carry %q", page, want)
		}
	}
}

// Nothing is left blank on the page: an unset phase and a real answer must not
// print alike.
func TestThePageNamesTheAbsenceOfAPhase(t *testing.T) {
	page := Render(Position{Repo: "r", Cycle: 1, Standing: Ready, Because: "nothing yet"})

	if strings.Contains(page, "reached:  \n") || strings.Contains(page, "awaiting: \n") {
		t.Errorf("page = %q, want an absent phase to read as words", page)
	}
}

func TestTheBankedCyclesAreOnThePageWhenThereAreAny(t *testing.T) {
	if page := Render(Position{Repo: "r"}); strings.Contains(page, "banked") {
		t.Errorf("page = %q, want no banked line for a repository with none", page)
	}
	if page := Render(Position{Repo: "r", Banked: []int{1}}); !strings.Contains(page, "banked:   [1]") {
		t.Errorf("page = %q, want the banked cycle", page)
	}
}

func writeArtifact(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func record(t *testing.T, repoDir string, a Attempt) {
	t.Helper()
	if err := Record(repoDir, a); err != nil {
		t.Fatal(err)
	}
}

// The ceiling parks from the verdict as well as from the artifact. The handoff
// is written by the phase that runs next, so between the verdict and that phase
// the position has to know already — otherwise a crank reads "ready" and opens
// a seventh authoring cycle.
func TestTheCeilingParksBeforeTheHandoffIsWritten(t *testing.T) {
	campaign := t.TempDir()
	repo := filepath.Join(campaign, "r")
	writeArtifact(t, filepath.Join(repo, "index", "index.json"), "{}")
	writeArtifact(t, filepath.Join(repo, strconv.Itoa(phase.AuthoringCeiling), "author", "scenario.draft.yaml"), "name: r\n")
	record(t, repo, Attempt{Cycle: phase.AuthoringCeiling, Phase: phase.Author, Verdict: phase.NoAnchor, Try: 1,
		Table: "nothing in this repository carries the question"})

	p := read(t, campaign, "r")

	if p.Standing != Parked || p.Awaiting != phase.Handoff {
		t.Errorf("standing = %s awaiting %s, want it parked at the handoff", p.Standing, p.Awaiting)
	}
	if p.ToCeiling() != 0 {
		t.Errorf("to the ceiling = %d, want none left", p.ToCeiling())
	}
}

// A cycle that wrote everything owes nothing, and the walk says so rather than
// naming a phase that is already done.
func TestACycleThatWroteEverythingAwaitsNothing(t *testing.T) {
	dir := t.TempDir()
	for _, ph := range phase.Graph {
		if ph.Name == phase.Index || ph.Name == phase.Handoff {
			continue
		}
		writeArtifact(t, filepath.Join(dir, string(ph.Name), ph.Writes), "written\n")
	}

	reached, awaiting := walk(dir)

	if reached != phase.Board || awaiting != phase.Done {
		t.Errorf("reached %s awaiting %s, want the board reached and nothing owed", reached, awaiting)
	}
}

// A repository directory holds things that are not cycles, and this walk does
// not rule on them.
func TestWhatIsNotACycleDirectoryIsNotACycle(t *testing.T) {
	campaign := t.TempDir()
	repo := filepath.Join(campaign, "r")
	writeArtifact(t, filepath.Join(repo, "index", "index.json"), "{}")
	// A file named like a cycle, which is what the directory check is for: a
	// name that parses as a number is not a cycle unless it is a directory.
	writeArtifact(t, filepath.Join(repo, "5"), "not a cycle\n")
	writeArtifact(t, filepath.Join(repo, "notes.md"), "# by hand\n")
	writeArtifact(t, filepath.Join(repo, "2", "author", "scenario.draft.yaml"), "name: r\n")

	if p := read(t, campaign, "r"); p.Cycle != 2 {
		t.Errorf("cycle = %d, want the one numbered directory", p.Cycle)
	}
}

// An exit code is a claim; the artifact is the fact. A phase that started and
// died leaves its directory behind, so the check has to be for the artifact the
// phase owed and not for the directory it ran in.
func TestAPhaseDirectoryWithoutItsArtifactIsNotAFinishedPhase(t *testing.T) {
	campaign := t.TempDir()
	repo := filepath.Join(campaign, "r")
	writeArtifact(t, filepath.Join(repo, "index", "index.json"), "{}")
	// What an agent that started and died leaves: the directory, and something
	// in it that is not what it owed.
	writeArtifact(t, filepath.Join(repo, "1", "author", "notes.txt"), "half a thought\n")
	record(t, repo, Attempt{Cycle: 1, Phase: phase.Author, Verdict: phase.Draft, Try: 1})

	p := read(t, campaign, "r")

	if p.Standing != Missing {
		t.Errorf("standing = %s (%s), want %s: the phase wrote no scenario", p.Standing, p.Because, Missing)
	}
	if p.Reached == phase.Author {
		t.Error("reached = author; a directory is not an artifact")
	}
}

// Past the ceiling there is nothing left to spend. A repository can be resumed
// by hand into a cycle beyond it, and the count of what remains does not go
// negative.
func TestNothingIsLeftPastTheCeiling(t *testing.T) {
	p := Position{Cycle: phase.AuthoringCeiling + 1}

	if p.ToCeiling() != 0 {
		t.Errorf("to the ceiling = %d, want none left", p.ToCeiling())
	}
}
