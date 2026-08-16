package rescore

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/score"
)

// The classifier's whole job is to refuse the comfortable label.
//
// `scorer version` is the comfortable one: it says the old number was right for
// its time and nothing is wrong now. The symbol-oracle fix only ever ADDED
// credit, so it can only ever explain a rescore that went UP, and a pre-fix run
// that rescores DOWN is a new defect wearing the wrong label.
func TestAPreFixRunThatScoresLowerIsNotAScorerVersionDifference(t *testing.T) {
	for _, tc := range []struct {
		name string
		row  Row
		want Cause
	}{
		{
			name: "pre-fix and higher, which the fix explains",
			row:  Row{Recorded: 5, Now: 7, PreFix: true},
			want: ScorerVersion,
		},
		{
			name: "pre-fix and LOWER, which it cannot",
			row:  Row{Recorded: 7, Now: 5, PreFix: true, FileLevel: 2},
			want: WrongLine,
		},
		{
			name: "higher, but not pre-fix, so nothing explains it",
			row:  Row{Recorded: 5, Now: 7},
			want: Unexplained,
		},
		{
			name: "identical",
			row:  Row{Recorded: 5, Now: 5},
			want: "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.row); got != tc.want {
				t.Errorf("Classify = %q, want %q", got, tc.want)
			}
		})
	}
}

// The two old-defect classes are different claims about the old instrument, and
// collapsing them would lose the starker one.
func TestTheTwoOldDefectsAreToldApart(t *testing.T) {
	fileLevel := Row{Recorded: 6, Now: 2, FileLevel: 4, Uncited: 0}
	uncited := Row{Recorded: 6, Now: 2, FileLevel: 1, Uncited: 3}

	if got := Classify(fileLevel); got != WrongLine {
		t.Errorf("Classify = %q, want %q", got, WrongLine)
	}
	if got := Classify(uncited); got != NeverLocated {
		t.Errorf("Classify = %q, want %q", got, NeverLocated)
	}
}

// A quarantined group answers a different question now, and that is a named
// cause rather than a difference to explain away. It outranks the rest, because
// a group whose rows were removed cannot be compared on its numbers at all.
func TestAQuarantinedGroupIsAGoldChange(t *testing.T) {
	if got := Classify(Row{Recorded: 9, Now: 0, Quarantined: true, FileLevel: 9}); got != GoldChange {
		t.Errorf("Classify = %q, want %q", got, GoldChange)
	}
}

// A difference nothing accounts for is UNEXPLAINED, which is not a cause. It is
// the state that keeps the cycle open, so it must be reachable — a classifier
// that always finds a label has stopped being a check.
func TestADifferenceWithNoAccountIsUnexplained(t *testing.T) {
	r := Report{Rows: []Row{{Recorded: 4, Now: 1}}} // lower, but nothing credited extra

	if got := Classify(r.Rows[0]); got != Unexplained {
		t.Fatalf("Classify = %q, want %q", got, Unexplained)
	}
	if len(r.Unexplained()) != 1 {
		t.Error("the report does not surface it")
	}
	if !strings.Contains(r.String(), "UNEXPLAINED. The cycle stays open") {
		t.Errorf("the report does not say the cycle stays open:\n%s", r.String())
	}
}

// The checkpoint is scheduled in advance rather than reached at the moment of
// fatigue, which is the whole point of having one.
func TestTheCheckpointFiresPastAThird(t *testing.T) {
	same := Row{Recorded: 1, Now: 1}
	diff := Row{Recorded: 2, Now: 1, FileLevel: 1}

	for _, tc := range []struct {
		name string
		rows []Row
		want bool
	}{
		{"nothing compared", nil, false},
		{"none differ", []Row{same, same, same}, false},
		{"exactly a third", []Row{diff, same, same}, false},
		{"past a third", []Row{diff, diff, same}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rp := Report{Rows: tc.rows}
			if got := rp.CheckpointFired(); got != tc.want {
				t.Errorf("CheckpointFired = %v, want %v", got, tc.want)
			}
		})
	}
}

// The distribution is read BEFORE the table: four differing rows is an
// afternoon of reading and forty is a structural problem, and the shape says
// which before anyone starts going row by row.
func TestTheDistributionIsReportedBeforeTheCauses(t *testing.T) {
	r := Report{Rows: []Row{
		{Recorded: 5, Now: 5},
		{Recorded: 5, Now: 4, FileLevel: 1},
		{Recorded: 5, Now: 4, FileLevel: 1},
	}}

	d := r.Distribution()
	if d[0] != 1 || d[-1] != 2 {
		t.Errorf("Distribution = %v, want one identical and two at -1", d)
	}
	out := r.String()
	if i, j := strings.Index(out, "difference, in rows cited"), strings.Index(out, "cause of every difference"); i < 0 || j < 0 || i > j {
		t.Errorf("the causes are printed before the distribution:\n%s", out)
	}
}

// Split is what makes the accounting an accounting rather than a subtraction of
// two totals: it says WHICH rows the old scorer credited that the new one does
// not, and why each one fails now.
func TestSplitSaysWhyEachExtraCreditFailsNow(t *testing.T) {
	gold := []score.Row{
		{ID: "a", Cite: "app/models/category.rb:100"}, // cited at the wrong line
		{ID: "b", Cite: "app/models/other.rb:12"},     // never located at all
		{ID: "c", Cite: "app/models/third.rb:5"},      // still a hit
		{ID: "d", Cite: "app/models/fourth.rb:9"},     // the old scorer did not credit it either
	}
	cites := score.Scan("app/models/category.rb:40 and app/models/third.rb:5")
	oldCited := map[string]bool{"a": true, "b": true, "c": true}

	fileLevel, uncited := Split(cites, gold, oldCited)

	if fileLevel != 1 {
		t.Errorf("fileLevel = %d, want 1 (the row cited at the wrong line)", fileLevel)
	}
	if uncited != 1 {
		t.Errorf("uncited = %d, want 1 (the row never located)", uncited)
	}
}

// A file named only by its bare basename was still NAMED. The matcher refuses
// to score it, on purpose, but calling it "never located" in the accounting
// would overstate how badly the old instrument behaved.
func TestABareBasenameCountsAsLocatedForTheAccounting(t *testing.T) {
	gold := []score.Row{{ID: "a", Cite: "src/Core/Services/OrganizationService.cs:835"}}
	cites := score.Scan("see OrganizationService.cs:36 for the check")

	fileLevel, uncited := Split(cites, gold, map[string]bool{"a": true})

	if fileLevel != 1 || uncited != 0 {
		t.Errorf("fileLevel=%d uncited=%d, want the basename counted as located", fileLevel, uncited)
	}
}

// A symbol citation carries the file it continues in Established rather than in
// Path, so the accounting has to look at both or it reads a located file as
// never located.
func TestTheAccountingSeesAFileASymbolCiteContinues(t *testing.T) {
	gold := []score.Row{{ID: "a", Cite: "app/jobs/scheduled/reindex_search.rb:999"}}
	// Built rather than scanned, because a scan that produces this cite also
	// produces the path cite that established it, and the path would answer
	// first — leaving the branch that reads Established untested.
	// The established file is a DIFFERENT path with the same basename, so
	// NamesFile refuses it (as it should — many files share a name) and the
	// basename check is the only thing left to see it.
	cites := []score.Cite{{
		Symbol:      "Jobs::ReindexSearch#load_problem_category_ids",
		Established: "app/jobs/regular/reindex_search.rb",
		Line:        111,
	}}

	fileLevel, uncited := Split(cites, gold, map[string]bool{"a": true})

	if fileLevel != 1 || uncited != 0 {
		t.Errorf("fileLevel=%d uncited=%d, want the continued file counted as located", fileLevel, uncited)
	}
}

// A gold row with no location is 02-05's problem, and must not be counted as
// something the old scorer wrongly credited.
func TestARowWithNoLocationIsNotCountedAsAnExtraCredit(t *testing.T) {
	gold := []score.Row{{ID: "a", Cite: ""}, {ID: "b", Cite: "no-colon-here"}}

	fileLevel, uncited := Split(nil, gold, map[string]bool{"a": true, "b": true})

	if fileLevel != 0 || uncited != 0 {
		t.Errorf("fileLevel=%d uncited=%d, want both zero", fileLevel, uncited)
	}
}

// The per-repo breakdown is what makes `gold change` checkable against 02-05's
// handover list, which is a list of runs in one repository.
func TestTheReportBreaksTheCausesDownByRepo(t *testing.T) {
	r := Report{Rows: []Row{
		{Repo: "rails", Recorded: 9, Now: 0, Quarantined: true},
		{Repo: "discourse", Recorded: 5, Now: 3, FileLevel: 2},
	}}

	out := r.String()
	for _, want := range []string{"cause by repo", "rails", "gold change", "discourse"} {
		if !strings.Contains(out, want) {
			t.Errorf("%q missing from:\n%s", want, out)
		}
	}
}
