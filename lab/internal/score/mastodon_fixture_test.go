package score

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/transcript"
)

// The banked mastodon cell, sense arm, run 2, claude-opus-5. Checked in whole
// rather than trimmed, because a fixture that has been edited to make a point
// cannot be used to check the point.
const mastodonFixture = "testdata/mastodon-symbol-cites.stream.jsonl"

// The rows from the RECORDED DEFECT, with the gold location each one carries.
//
// All of them scored missed_mention AND missed_cite while the answer named them
// plainly, because the scorer matched on the gold row's path and the arm had
// written symbols. The failure mode is the point: a scorer wrong in one
// direction silently discounts the arm that answers in the form it does not
// expect, and nothing about that looks like a bug from the outside. It looks
// like a worse result.
//
// The recorded note says FIVE rows and names three of them. Measured against
// this transcript the form rescues EIGHT, and all eight are listed rather than
// the five, because trimming the list to the number in the note would be
// fitting the evidence to the record. The three the note names are the first,
// second and fifth here.
var mastodonDefectRows = []Row{
	{ID: "d:admin-audit-scope", Cite: "app/controllers/admin/action_logs_controller.rb:9"},
	{ID: "d:favourited-by-accounts", Cite: "app/controllers/api/v1/statuses/favourited_by_accounts_controller.rb:22"},
	{ID: "d:reblogged-by-accounts", Cite: "app/controllers/api/v1/statuses/reblogged_by_accounts_controller.rb:22"},
	{ID: "d:statuses-index-local-votes", Cite: "app/lib/importer/statuses_index_importer.rb:74"},
	{ID: "d:space-usage-avatar-sum", Cite: "app/lib/admin/metrics/dimension/space_usage_dimension.rb:44"},
	{ID: "d:friends-of-friends", Cite: "app/models/account_suggestions/friends_of_friends_source.rb:10"},
	{ID: "d:instance-user-count", Cite: "app/presenters/instance_presenter.rb:54"},
	{ID: "d:serializer-decorator-model-name", Cite: "app/serializers/rest/account_serializer.rb:29"},
}

func mastodonAnswer(t *testing.T) string {
	t.Helper()
	tr, err := transcript.ReadClaudeCode(mastodonFixture)
	if err != nil {
		t.Fatalf("read the recorded mastodon run: %v", err)
	}
	if tr.Provisional() {
		t.Fatalf("the fixture is not a complete capture: %s", tr.ProvisionalWhy())
	}
	return tr.Answer()
}

func TestTheRowsFromTheRecordedDefectAreCited(t *testing.T) {
	cites := Scan(mastodonAnswer(t))

	for _, row := range mastodonDefectRows {
		t.Run(row.ID, func(t *testing.T) {
			if !anyMatches(cites, row.Cite) {
				t.Errorf("%s is still missed; the arm named it and the scorer did not see it", row.ID)
			}
		})
	}
}

// The same rows, through the door they actually come in by. If a later change
// credited them some other way, the test above would stay green while the form
// this pitch exists to read had stopped being read.
func TestTheDefectRowsAreCitedBecauseOfTheSymbolForm(t *testing.T) {
	cites := Scan(mastodonAnswer(t))

	for _, row := range mastodonDefectRows {
		t.Run(row.ID, func(t *testing.T) {
			if got := howMatched(cites, row.Cite); got != matchedSymbol {
				t.Errorf("matched %v, want the symbol form", got)
			}
		})
	}
}

// The composition of the number travels WITH the number.
//
// On this transcript every single hit arrives through the symbol rule, whose
// line is not compared, and no path citation lands on a gold line at all. A
// score built entirely of line-unchecked matches is a different claim from one
// built of strict ones, and printing them identically is how an arm that writes
// symbols quietly out-scores an arm that writes paths to the same places.
func TestTheReportSaysHowManyHitsSkippedTheLineCheck(t *testing.T) {
	r := Group("dependents", mastodonDefectRows, text(mastodonAnswer(t)), 0.50)

	if r.Cited != len(mastodonDefectRows) {
		t.Fatalf("cited %d of %d", r.Cited, r.Total)
	}
	if r.SymbolCited != r.Cited {
		t.Errorf("SymbolCited = %d, want all %d of the hits", r.SymbolCited, r.Cited)
	}
	const want = "strict     0 of 8, dropping the 8 matched on a symbol whose line is unchecked"
	if got := r.String(); !strings.Contains(got, want) {
		t.Errorf("the report hides how the number was earned:\n%s", got)
	}
}

// And the mark is not printed when it would be a lie: a run whose hits are all
// strict must not carry a caveat about looseness it does not have.
func TestTheReportStaysSilentWhenEveryHitWasStrict(t *testing.T) {
	rows := []Row{{ID: "d:one", Cite: "app/models/category.rb:1083"}}
	r := Group("dependents", rows, text("found app/models/category.rb:1083"), 0.50)

	if r.SymbolCited != 0 {
		t.Errorf("SymbolCited = %d on an all-strict run", r.SymbolCited)
	}
	if strings.Contains(r.String(), "whose line is unchecked") {
		t.Errorf("a caveat appeared on a run that did not earn it:\n%s", r.String())
	}
}

// The rows above are written out here so the claim is readable, which means
// they can drift from the gold file they were copied from. A corrected gold row
// would leave this test green while it checked a location nothing scores.
func TestTheDefectRowsStillMatchTheCheckedInGold(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "scenarios", "mastodon", "mastodon.gold.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	gold := string(b)
	for _, row := range mastodonDefectRows {
		if !strings.Contains(gold, row.ID) {
			t.Errorf("gold no longer has a row %q", row.ID)
		}
		if !strings.Contains(gold, row.Cite) {
			t.Errorf("row %q no longer cites %s in the gold file", row.ID, row.Cite)
		}
	}
}
