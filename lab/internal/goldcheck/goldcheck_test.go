package goldcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/luuuc/sense/lab/internal/scenario"
)

// gold builds a Set from rows written inline, so each test reads as the shape
// it is about rather than as a file somewhere else.
func gold(t *testing.T, symbol string, rows ...scenario.GoldRow) scenario.Set {
	t.Helper()
	return scenario.Set{
		Name:     "fixture",
		Scenario: scenario.Scenario{Name: "fixture", ContractSymbol: symbol},
		Gold:     scenario.Gold{Discriminator: "dependents", Rows: rows},
	}
}

func row(id, group, relation string) scenario.GoldRow {
	return scenario.GoldRow{ID: id, Group: group, Relation: relation}
}

func reasons(r Report) map[string]int {
	out := map[string]int{}
	for _, f := range r.Findings {
		out[string(f.Reason)]++
	}
	return out
}

// THE recorded failure, against the gold that produced it.
//
// A scenario asked for specs and fixtures "using file:line throughout", and its
// two gold rows named files with no line. They scored cited 0 of 2 in 10 of 10
// runs for months, both arms, both models — and nothing about the file looks
// wrong when you read it.
func TestTheRecordedSpecFixtureRowsAreQuarantined(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("testdata", "spec-fixture.gold.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var g scenario.Gold
	if err := yaml.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	// Pinned to the recorded scenario, not merely to two ids. Without this the
	// test passes against any two lineless rows carrying those names, and the
	// nineteen lines of history in the fixture are a comment with a
	// test-shaped decoration attached.
	if len(g.Rows) != 3 {
		t.Fatalf("the fixture has %d rows, want 3 including the citeable control", len(g.Rows))
	}
	for _, want := range []struct{ id, group, relation string }{
		{"d:real", "dependents", "app/models/status.rb:12 a dependent with a real location"},
		{"spec:status", "specs", "models/status_spec.rb"},
		{"spec:post-svc", "specs", "services/post_status_service_spec.rb"},
	} {
		var found bool
		for _, r := range g.Rows {
			if r.ID == want.id {
				found = true
				if r.Group != want.group || r.Relation != want.relation {
					t.Errorf("%s = (%q, %q), want (%q, %q)", want.id, r.Group, r.Relation, want.group, want.relation)
				}
			}
		}
		if !found {
			t.Errorf("the fixture no longer has the row %q", want.id)
		}
	}
	set := scenario.Set{Name: "spec-fixture", Gold: g}

	r := Audit(set, nil, nil)

	if r.Rows != 3 {
		t.Errorf("audited %d rows, want 3", r.Rows)
	}

	want := []string{"spec:status", "spec:post-svc"}
	if len(r.Quarantined) != len(want) {
		t.Fatalf("quarantined %v, want exactly %v", r.Quarantined, want)
	}
	for i, id := range want {
		if r.Quarantined[i] != id {
			t.Errorf("quarantined[%d] = %q, want %q", i, r.Quarantined[i], id)
		}
	}
	// And the row that IS citeable survives, or the check is just "reject
	// everything" wearing a reason.
	for _, id := range r.Quarantined {
		if id == "d:real" {
			t.Error("a row with a real location was quarantined")
		}
	}
}

func TestEveryQuarantineCarriesItsReason(t *testing.T) {
	for _, tc := range []struct {
		name string
		set  scenario.Set
		want Reason
	}{
		{
			name: "a row with no location",
			set:  gold(t, "", row("a", "dependents", "somewhere in the search code")),
			want: NoLocation,
		},
		{
			// One item per file, or a group is silently weighted toward
			// whichever file happens to have the most matches in it.
			name: "two rows of one group in the same file",
			set: gold(t, "",
				row("a", "dependents", "app/models/category.rb:12 the first"),
				row("b", "dependents", "app/models/category.rb:99 the second")),
			want: DuplicateFile,
		},
		{
			name: "two rows sharing an id",
			set: gold(t, "",
				row("a", "dependents", "app/models/category.rb:12 the first"),
				row("a", "contract", "app/models/other.rb:1 the second")),
			want: DuplicateID,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Audit(tc.set, nil, nil)
			if len(r.Quarantined) == 0 {
				t.Fatalf("nothing was quarantined:\n%s", r)
			}
			if got := reasons(r)[string(tc.want)]; got == 0 {
				t.Errorf("no finding carried %q; got %v", tc.want, reasons(r))
			}
		})
	}
}

// The same file in two DIFFERENT groups is fine: the rule is one item per file
// per group, because that is where the weighting happens.
func TestTheSameFileInTwoGroupsIsNotADuplicate(t *testing.T) {
	set := gold(t, "",
		row("c:anchor", "contract", "app/models/category.rb:1 the class itself"),
		row("d:one", "dependents", "app/models/category.rb:1083 a dependent"))

	if r := Audit(set, nil, nil); len(r.Quarantined) != 0 {
		t.Errorf("quarantined %v across two groups:\n%s", r.Quarantined, r)
	}
}

// stubResolver answers for exactly the locations it is given.
type stubResolver map[string]bool

func (s stubResolver) Resolves(path string, _ int) (ok, checked bool) {
	return s[path], true
}

// A row pointing at a line that moved is not gold, it is history.
func TestARowThatDoesNotResolveIsQuarantined(t *testing.T) {
	set := gold(t, "",
		row("a", "dependents", "app/models/category.rb:12 still there"),
		row("b", "dependents", "app/models/moved.rb:40 not any more"))

	r := Audit(set, stubResolver{"app/models/category.rb": true}, nil)

	if len(r.Quarantined) != 1 || r.Quarantined[0] != "b" {
		t.Errorf("quarantined %v, want just the row that moved", r.Quarantined)
	}
	if reasons(r)[string(Unresolved)] != 1 {
		t.Errorf("the reason was not %q: %v", Unresolved, reasons(r))
	}
}

// A validator that refused to run without a repository would be a validator
// nobody runs. It audits what it can and NAMES what it could not, so a clean
// report cannot be read as a complete one.
func TestWithoutACheckoutItSaysWhatItCouldNotCheck(t *testing.T) {
	set := gold(t, "Category", row("a", "dependents", "app/models/category.rb:12 fine"))

	r := Audit(set, nil, nil)

	if len(r.Quarantined) != 0 {
		t.Errorf("quarantined %v with nothing to resolve against", r.Quarantined)
	}
	if len(r.Unchecked) != 2 {
		t.Errorf("Unchecked = %v, want both resolution and the covering grep", r.Unchecked)
	}
	if got := r.String(); !strings.Contains(got, "NOT CHECKED") {
		t.Errorf("a partial audit read as a complete one:\n%s", got)
	}
}

type stubGrep struct {
	hits  map[string]bool
	total int
}

func (g stubGrep) Covering(string) (map[string]bool, int, bool) { return g.hits, g.total, true }

// A free row RANKS and never kills. Run as a per-row census this check once
// killed four shapes on a repository that banks +0.53, so it flags and the
// human decides.
func TestARowInsideTheCoveringGrepIsFlaggedAndNeverQuarantined(t *testing.T) {
	set := gold(t, "Setting",
		row("a", "dependents", "config/settings.rb:12 one Setting read"),
		row("b", "dependents", "app/models/other.rb:4 not in the grep"))

	// A distinctive count, so a validator that fabricates the number rather
	// than reporting the grepper's is not green.
	r := Audit(set, nil, stubGrep{hits: map[string]bool{"config/settings.rb:12": true}, total: 4483})

	if len(r.Quarantined) != 0 {
		t.Errorf("a free row was quarantined: %v", r.Quarantined)
	}
	if reasons(r)[string(InCoveringGrep)] != 1 {
		t.Errorf("the free row was not flagged: %v", reasons(r))
	}
	// The size of the grep is the whole story: a row inside a 4483-line grep is
	// not free to anybody, so the count travels with the flag.
	if got := r.String(); !strings.Contains(got, "prints 4483 lines") {
		t.Errorf("the flag does not carry the grep's own size:\n%s", got)
	}
	// And the size never turns the flag into a quarantine, however large. A row
	// inside a 4483-line grep is not free to anybody, but that is a judgement
	// for a human and never a rejection here.
	if len(r.Quarantined) != 0 {
		t.Errorf("a large covering grep quarantined %v", r.Quarantined)
	}
}

// A row with no location has exactly ONE thing wrong with it.
//
// Checking it for duplicate files and for resolution invents reasons that are
// false: splitCite("") gives an empty path, every such row collides with every
// other in seenFile, and the empty path then goes to a real checkout. The
// report would tell a human that two rows in two different files are the same
// file and that both moved at the pinned commit. Neither is true, and every
// other test in this file agrees with it, because they all assert that a reason
// is PRESENT and none asserts that a reason is absent.
func TestARowWithNoLocationCollectsNoOtherReason(t *testing.T) {
	set := gold(t, "",
		row("spec:a", "specs", "models/status_spec.rb"),
		row("spec:b", "specs", "services/post_status_service_spec.rb"))

	r := Audit(set, stubResolver{}, nil)

	got := reasons(r)
	if len(got) != 1 || got[string(NoLocation)] != 2 {
		t.Errorf("reasons = %v, want only NoLocation twice", got)
	}
}

// A finding reads as one line naming the row, where it points, and what is
// wrong with it. It is the unit a human acts on, so it carries the location
// rather than making them go and look it up.
func TestAFindingReadsAsOneLine(t *testing.T) {
	f := Finding{RowID: "d:one", Group: "dependents", Cite: "app/models/category.rb:12",
		Reason: Unresolved, Detail: "moved in 2024"}

	got := f.String()
	for _, want := range []string{"d:one", "dependents", "app/models/category.rb:12", "moved in 2024"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing from %q", want, got)
		}
	}
	// A row with no location has none to print, and must not render an empty
	// column that reads as a location of "".
	bare := Finding{RowID: "a", Group: "specs", Reason: NoLocation}
	if strings.Contains(bare.String(), "()") {
		t.Errorf("an empty detail rendered as parentheses: %q", bare.String())
	}
}

// Free rows RANK, everything else REMOVES. The distinction is the law, so it is
// asserted on the type rather than left to each call site.
func TestOnlyTheCoveringGrepFlagLeavesARowInPlace(t *testing.T) {
	for _, r := range []Reason{NoLocation, Unresolved, DuplicateFile, DuplicateID} {
		if !r.Quarantines() {
			t.Errorf("%q does not quarantine", r)
		}
	}
	if InCoveringGrep.Quarantines() {
		t.Error("a free row was quarantined; the law says this ranks and never kills")
	}
}

// A scenario naming no anchor has nothing to grep for, and the report says the
// check did not run rather than reporting no free rows.
func TestWithNoAnchorTheGrepCheckIsNamedAsSkipped(t *testing.T) {
	set := gold(t, "", row("a", "dependents", "app/models/category.rb:12 fine"))

	r := Audit(set, stubResolver{"app/models/category.rb": true}, stubGrep{})

	if len(r.Unchecked) != 1 || r.Unchecked[0] != string(InCoveringGrep) {
		t.Errorf("Unchecked = %v, want the covering grep named", r.Unchecked)
	}
}

// A grepper that cannot answer is the same case as no grepper at all.
type blindGrep struct{}

func (blindGrep) Covering(string) (map[string]bool, int, bool) { return nil, 0, false }

func TestAGrepperThatCannotAnswerIsNamedAsSkipped(t *testing.T) {
	set := gold(t, "Category", row("a", "dependents", "app/models/category.rb:12 fine"))

	r := Audit(set, nil, blindGrep{})

	found := false
	for _, u := range r.Unchecked {
		if u == string(InCoveringGrep) {
			found = true
		}
	}
	if !found {
		t.Errorf("Unchecked = %v, want the covering grep named", r.Unchecked)
	}
}

// A row with no location cannot be in a covering grep, and must not be counted
// as one on the strength of an empty string matching an empty key.
func TestARowWithNoLocationIsNotCountedAsFree(t *testing.T) {
	set := gold(t, "Category", row("a", "specs", "models/status_spec.rb"))

	r := Audit(set, nil, stubGrep{hits: map[string]bool{"": true}, total: 3})

	if reasons(r)[string(InCoveringGrep)] != 0 {
		t.Errorf("a row with no location was flagged as free: %v", reasons(r))
	}
}

// A resolver that cannot answer for a row leaves it alone rather than
// quarantining it for a question nobody could ask.
type mutedResolver struct{}

func (mutedResolver) Resolves(string, int) (bool, bool) { return false, false }

func TestAResolverThatCannotAnswerQuarantinesNothing(t *testing.T) {
	set := gold(t, "", row("a", "dependents", "app/models/category.rb:12 fine"))

	if r := Audit(set, mutedResolver{}, nil); len(r.Quarantined) != 0 {
		t.Errorf("quarantined %v on an answer nobody gave", r.Quarantined)
	}
}

// The report a human reads. Quarantines are listed row by row because each one
// is a decision someone has to take with the scenario in front of them, and a
// count alone does not tell them which rows to open.
func TestTheReportListsEveryQuarantinedRow(t *testing.T) {
	set := gold(t, "",
		row("spec:status", "specs", "models/status_spec.rb"),
		row("spec:post-svc", "specs", "services/post_status_service_spec.rb"),
		row("d:fine", "dependents", "app/models/status.rb:12 a real one"))

	got := Audit(set, nil, nil).String()

	for _, want := range []string{
		"3 gold rows, 2 quarantined",
		"QUARANTINED: no path:line",
		"spec:status",
		"spec:post-svc",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("%q is missing from the report:\n%s", want, got)
		}
	}
	if strings.Contains(got, "d:fine") {
		t.Errorf("a sound row appears in the report:\n%s", got)
	}
}
