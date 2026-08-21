package stage

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/luuuc/sense/lab/internal/phase"
)

// The walk that holds the two declarations together. A phase added to the graph
// with no stage would print as a blank position on every screen, and a stage
// naming a phase the graph no longer has would mark a stage nothing can reach.
func TestEveryPhaseOfTheGraphBelongsToExactlyOneStage(t *testing.T) {
	covered := map[phase.Name]int{}
	for _, s := range append([]Stage{Admitting}, Stages...) {
		for _, p := range s.Phases {
			covered[p]++
		}
	}

	for _, g := range phase.Graph {
		switch covered[g.Name] {
		case 1:
		case 0:
			t.Errorf("phase %s is in the graph and in no stage; it would print as a blank position", g.Name)
		default:
			t.Errorf("phase %s is in %d stages; the map would mark two places at once", g.Name, covered[g.Name])
		}
		delete(covered, g.Name)
	}
	for name := range covered {
		t.Errorf("stages name %s, which is not a phase of the graph", name)
	}
}

func TestTheStagesAreNumberedOneToFiveInOrder(t *testing.T) {
	for i, s := range Stages {
		if s.Number != i+1 {
			t.Errorf("stage %q is numbered %d and sits at position %d", s.Name, s.Number, i+1)
		}
	}
}

// Every stage says what happens in it, in the operator's terms. A stage with no
// description is a row of the map that teaches nothing.
func TestEveryStageSaysWhatHappensInIt(t *testing.T) {
	for _, s := range append([]Stage{Admitting}, Stages...) {
		if s.Name == "" || s.What == "" {
			t.Errorf("stage %+v is missing its name or its line", s)
		}
		if strings.Contains(s.What, string(phase.Minibench)) || strings.Contains(s.What, string(phase.Preflight)) {
			t.Errorf("stage %q describes itself with a phase name: %q", s.Name, s.What)
		}
	}
}

func TestAPhaseIsFoundInItsStage(t *testing.T) {
	for _, tc := range []struct {
		phase phase.Name
		want  string
	}{
		{phase.Index, "Admitting"},
		{phase.Author, "Question"},
		{phase.Minibench, "Trial"},
		{phase.Expand, "Rehearsal"},
		{phase.Preflight, "Rehearsal"},
		{phase.Validate, "Rehearsal"},
		{phase.Bench, "Paid run"},
		{phase.Report, "Verdict"},
		{phase.Handoff, "Verdict"},
	} {
		got, ok := Of(tc.phase)
		if !ok || got.Name != tc.want {
			t.Errorf("Of(%s) = %q (%v), want %q", tc.phase, got.Name, ok, tc.want)
		}
	}
}

// A repository that reached the board is not standing in stage 5. Answering
// with the last stage would mark the map as though it were still running.
func TestAFinishedRepositoryIsInNoStage(t *testing.T) {
	if s, ok := Of(phase.Done); ok {
		t.Errorf("Of(done) = %q, want no stage", s.Name)
	}
}

func TestTheMapMarksTheStageThePhaseBelongsTo(t *testing.T) {
	page := Map(phase.Minibench)

	lines := strings.Split(strings.TrimRight(page, "\n"), "\n")
	if len(lines) != len(Stages) {
		t.Fatalf("the map is %d lines, want one per stage:\n%s", len(lines), page)
	}
	if !strings.HasPrefix(lines[1], "  ▸ 2. Trial") {
		t.Errorf("the trial line is %q, want it marked", lines[1])
	}
	if n := strings.Count(page, "▸"); n != 1 {
		t.Errorf("the map carries %d markers, want exactly one:\n%s", n, page)
	}
}

// Admission is not a sixth row. A repository past its first command would
// otherwise show a step it can never take again, marked as done, forever.
func TestAdmissionIsNotOnTheMap(t *testing.T) {
	page := Map(phase.Index)

	if strings.Contains(page, Admitting.Name) {
		t.Errorf("the map carries admission:\n%s", page)
	}
	if strings.Contains(page, "▸") {
		t.Errorf("the map marks a stage while the repository is still being admitted:\n%s", page)
	}
}

// The rows do not shift as the loop advances. A map that moved sideways with
// the marker would read as a different map at every stop.
func TestTheMapKeepsItsColumnsWhereverTheLoopIs(t *testing.T) {
	first := Map(phase.Author)
	last := Map(phase.Board)

	strip := func(page string) string { return strings.ReplaceAll(page, "▸", " ") }
	if strip(first) != strip(last) {
		t.Errorf("the map changes shape between stages:\n%s\n%s", first, last)
	}
	for _, line := range strings.Split(strings.TrimRight(first, "\n"), "\n") {
		if !strings.HasPrefix(line, "  ▸ ") && !strings.HasPrefix(line, "    ") {
			t.Errorf("line %q does not start in the marker column", line)
		}
	}
}

// The descriptions line up under each other whatever the longest name is.
func TestTheDescriptionsShareAColumn(t *testing.T) {
	page := Map(phase.Author)

	column := -1
	for i, line := range strings.Split(strings.TrimRight(page, "\n"), "\n") {
		at := strings.Index(line, Stages[i].What)
		if at == -1 {
			t.Fatalf("line %q does not carry its description", line)
		}
		// In runes, which is what the terminal lines up. The marker is three
		// bytes and one column, so a byte offset would report the marked row as
		// two columns further along than it prints.
		what := utf8.RuneCountInString(line[:at])
		if column == -1 {
			column = what
			continue
		}
		if what != column {
			t.Errorf("line %q starts its description at column %d, want %d", line, what, column)
		}
	}
}

func TestTheLineNamesEveryStageAndMarksTheCurrentOne(t *testing.T) {
	got := Line(phase.Bench)

	for _, s := range Stages {
		if !strings.Contains(got, s.Name) {
			t.Errorf("the line %q does not name %q", got, s.Name)
		}
	}
	if !strings.Contains(got, "▸ 4. Paid run") {
		t.Errorf("the line %q does not mark the paid run", got)
	}
	if n := strings.Count(got, "▸"); n != 1 {
		t.Errorf("the line %q carries %d markers, want exactly one", got, n)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("the line %q is more than one line", got)
	}
}

func TestTheLineMarksNothingWhenThereIsNoStage(t *testing.T) {
	if got := Line(phase.Done); strings.Contains(got, "▸") {
		t.Errorf("the line %q marks a stage for a finished repository", got)
	}
}

// Every verdict the graph can emit has words a person can read. A phase gaining
// a verdict would otherwise print nothing at the moment somebody is watching to
// see what happened.
func TestEveryVerdictOfEveryPhaseHasWords(t *testing.T) {
	for _, g := range phase.Graph {
		for _, v := range g.Verdicts {
			o, ok := Says(g.Name, v)
			if !ok {
				t.Errorf("%s can emit %s and there are no words for it", g.Name, v)
				continue
			}
			if o.Line == "" {
				t.Errorf("%s/%s has an empty line", g.Name, v)
			}
			if strings.Contains(o.Line, strings.ToUpper(string(v))) {
				t.Errorf("%s/%s says %q, which is the verdict rather than what it means", g.Name, v, o.Line)
			}
		}
	}
}

// A verdict that goes the other way says what it means and what follows. The
// mini-bench sending a question back is the loop working, and a line that read
// like a failure would be a rebuke for the thing the trial exists to catch.
func TestAVerdictThatGoesTheOtherWaySaysWhatFollows(t *testing.T) {
	for _, g := range phase.Graph {
		for _, v := range g.Verdicts {
			o, _ := Says(g.Name, v)
			if !o.Good && o.More == "" {
				t.Errorf("%s/%s reads as a setback and does not say what follows", g.Name, v)
			}
			if o.Good && o.More != "" {
				t.Errorf("%s/%s went the way it should and explains itself anyway: %q", g.Name, v, o.More)
			}
		}
	}
}

func TestAPairTheGraphDoesNotDeclareHasNoWords(t *testing.T) {
	if _, ok := Says(phase.Board, phase.Requestion); ok {
		t.Error("the board is said to emit a re-question")
	}
}
