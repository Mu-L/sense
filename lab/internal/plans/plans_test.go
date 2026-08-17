package plans_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/plans"
)

// theRealPlans is lab/plans, relative to this package.
const theRealPlans = "../../plans"

// The plans are the method. This is the check that they and the graph still
// describe the same loop, and it runs against the shipped files rather than a
// fixture, because a fixture would only prove the checker works.
func TestEveryPhaseHasAPlanAndEveryPlanMatchesItsPhase(t *testing.T) {
	for _, err := range plans.Check(theRealPlans) {
		t.Error(err)
	}
}

func TestEveryJudgmentPhaseCarriesItsPrecedent(t *testing.T) {
	loaded, err := plans.Load(theRealPlans)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range loaded {
		if len(p.Emits) < 2 {
			continue
		}
		if !bytes.Contains(p.Body, []byte("## Precedent")) {
			t.Errorf("%s decides between %d verdicts and carries no precedent; an instruction with no "+
				"measured case behind it is the one that gets talked out of", p.Path, len(p.Emits))
		}
	}
}

// A plan whose body is composed rather than read is a binary holding opinions.
// The body is the file's own bytes after the header, and nothing is added to it.
func TestAPlansBodyIsTheFileVerbatim(t *testing.T) {
	loaded, err := plans.Load(theRealPlans)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range loaded {
		raw, err := os.ReadFile(p.Path)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasSuffix(raw, p.Body) {
			t.Errorf("%s: the loaded body is not the tail of the file on disk", p.Path)
		}
	}
}

// write puts one plan in a temporary directory. The checks are about
// disagreement with the graph, so every fixture starts from a plan that agrees.
func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// onePlan is a directory holding a single well-formed plan, plus whatever the
// test adds.
func onePlan(t *testing.T, name, body string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, name, body)
	return dir
}

const goodBoard = `---
phase: board
reads: harvest.json
writes: board.md
emits: [AUTO]
---

The board plan.
`

func problems(t *testing.T, dir string) string {
	t.Helper()
	var found []string
	for _, err := range plans.Check(dir) {
		found = append(found, err.Error())
	}
	return strings.Join(found, "\n")
}

func TestAPlanIsHeldToItsPhasesArtifacts(t *testing.T) {
	dir := onePlan(t, "board.md", strings.Replace(goodBoard, "reads: harvest.json", "reads: report.md", 1))
	got := problems(t, dir)
	if !strings.Contains(got, "harvest.json") {
		t.Errorf("a plan reading the wrong artifact was not caught:\n%s", got)
	}
}

func TestAPlanIsHeldToItsPhasesOutput(t *testing.T) {
	dir := onePlan(t, "board.md", strings.Replace(goodBoard, "writes: board.md", "writes: results.md", 1))
	got := problems(t, dir)
	if !strings.Contains(got, "writes") {
		t.Errorf("a plan writing the wrong artifact was not caught:\n%s", got)
	}
}

// A plan naming a verdict the graph does not know is a plan whose routing
// cannot be tested.
func TestAPlanCannotEmitAVerdictItsPhaseDoesNotHave(t *testing.T) {
	dir := onePlan(t, "board.md", strings.Replace(goodBoard, "emits: [AUTO]", "emits: [AUTO, PAY]", 1))
	got := problems(t, dir)
	if !strings.Contains(got, "PAY") {
		t.Errorf("a plan emitting a verdict outside its enum was not caught:\n%s", got)
	}
}

// The quieter half: a case the phase really meets, with no instruction for it.
func TestAPlanMustSayWhatToDoOnEveryVerdictItsPhaseCanEmit(t *testing.T) {
	dir := onePlan(t, "harvest.md", `---
phase: harvest
reads: report.md
writes: harvest.json
emits: [WIN CONFIRMED]
---

## Precedent

The harvest plan.
`)
	got := problems(t, dir)
	if !strings.Contains(got, "DoD FAIL") {
		t.Errorf("a plan silent on a verdict its phase emits was not caught:\n%s", got)
	}
}

func TestAPlanForAPhaseTheGraphDoesNotDeclareIsCaught(t *testing.T) {
	dir := onePlan(t, "triage.md", `---
phase: triage
reads: a.json
writes: b.json
emits: [AUTO]
---

The triage plan.
`)
	got := problems(t, dir)
	if !strings.Contains(got, "triage") {
		t.Errorf("a plan for a phase that does not exist was not caught:\n%s", got)
	}
}

func TestAPhaseWithNoPlanIsCaught(t *testing.T) {
	got := problems(t, onePlan(t, "board.md", goodBoard))
	for _, p := range phase.Graph {
		if p.Name == phase.Board {
			continue
		}
		if !strings.Contains(got, string(p.Name)) {
			t.Errorf("phase %s has no plan and was not reported:\n%s", p.Name, got)
		}
	}
}

func TestTwoPlansCannotClaimTheSamePhase(t *testing.T) {
	dir := onePlan(t, "board.md", goodBoard)
	write(t, dir, "board-2.md", goodBoard)
	got := problems(t, dir)
	if !strings.Contains(got, "both claim") {
		t.Errorf("two plans for one phase were not caught:\n%s", got)
	}
}

func TestAHeaderWithNoPlanUnderItIsCaught(t *testing.T) {
	dir := onePlan(t, "board.md", "---\nphase: board\nreads: harvest.json\nwrites: board.md\nemits: [AUTO]\n---\n\n")
	got := problems(t, dir)
	if !strings.Contains(got, "no plan under it") {
		t.Errorf("an empty plan was not caught:\n%s", got)
	}
}

func TestAMalformedHeaderIsRefusedRatherThanParsedIntoSomethingPlausible(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"no header", "# board\n\nprose\n", "no header"},
		{"unterminated header", "---\nphase: board\n", "unterminated"},
		{"no phase", "---\nreads: harvest.json\n---\n\nprose\n", "names no phase"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := problems(t, onePlan(t, "board.md", tc.body))
			if !strings.Contains(got, tc.want) {
				t.Errorf("%s was not caught:\n%s", tc.name, got)
			}
		})
	}
}

func TestADirectoryWithNoPlansIsAnErrorRatherThanACleanBill(t *testing.T) {
	if got := problems(t, t.TempDir()); !strings.Contains(got, "no plans") {
		t.Errorf("an empty plans directory reported clean:\n%s", got)
	}
	if _, err := plans.Load(filepath.Join(t.TempDir(), "nowhere")); err == nil {
		t.Error("a plans directory that does not exist loaded cleanly")
	}
}

func TestAnUnreadablePlanIsAnError(t *testing.T) {
	dir := onePlan(t, "board.md", goodBoard)
	locked := filepath.Join(dir, "board.md")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o644) })
	if _, err := plans.Load(dir); err == nil {
		t.Error("a plan that could not be read loaded cleanly")
	}
}
