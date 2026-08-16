package score

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/scenario"
	"github.com/luuuc/sense/lab/internal/transcript"
)

// The fixture is a real paid run: the two-step probe on discourse at its pinned
// commit, sense arm, captured live by 00-02 ($2.36, 24 turns, 424s of a 600s
// wall). The stream is reduced to the assistant and result events, which is
// exactly what the scorer reads.
//
// What this file preserves is not a number, it is a finding. Every expectation
// below was hand-audited row by row against the transcript, and then re-derived
// independently before it was written down.
const (
	fixtureStream   = "testdata/discourse-probe.stream.jsonl"
	fixtureScenario = "testdata/discourse-probe"

	// The exact shape of the recorded answer, so a fixture that quietly loses
	// half its events cannot pass as the same run.
	//
	// These halved when the scorer moved onto the canonical transcript in
	// 02-01, and the halving is the point: the skeleton's reader also read the
	// final `result` event, which REPEATS the last assistant message. Across
	// all 238 recorded transcripts the result event never carries answer the
	// assistant events lack, so dropping it loses nothing and stops counting
	// the same words twice. The SCORE is unchanged, because citations were
	// deduplicated either way.
	fixtureAnswerChars = 53472
	fixtureCitations   = 254
)

func fixture(t *testing.T) (rows []Row, answer string) {
	t.Helper()
	s, err := scenario.Load("testdata", "discourse-probe")
	if err != nil {
		t.Fatalf("load scenario fixture: %v", err)
	}
	gold, err := s.Gold.Group("dependents")
	if err != nil {
		t.Fatalf("gold group: %v", err)
	}
	tr, err := transcript.ReadClaudeCode(fixtureStream)
	if err != nil {
		t.Fatalf("read transcript fixture: %v", err)
	}
	if tr.Provisional() {
		t.Fatalf("the fixture reads as provisional: %s", tr.Why)
	}
	answer = tr.Answer()
	for _, g := range gold {
		rows = append(rows, Row{ID: g.ID, Cite: g.Cite()})
	}
	return rows, answer
}

// The score, as the strict rule computes it.
func TestTheRecordedRunScoresTwoOfTwelve(t *testing.T) {
	rows, answer := fixture(t)

	got := Group("dependents", rows, text(answer), 0.50)

	if got.Total != 12 {
		t.Fatalf("gold group has %d rows, want the 12 that were audited", got.Total)
	}
	if got.Cited != 2 {
		t.Errorf("cited = %d, want 2 exact path:line hits", got.Cited)
	}
	if got.Verdict != BelowFloor {
		t.Errorf("verdict = %q, want %q at recall %.2f against a 0.50 floor",
			got.Verdict, BelowFloor, got.Recall)
	}

	// Named, not counted. A count that stayed at 2 while the identities changed
	// would mean the matcher had started crediting something else.
	want := map[string]bool{"d:migrations-optimizer": true, "d:data-explorer-parameter": true}
	for _, id := range got.Hits {
		if !want[id] {
			t.Errorf("credited %s, which the audit scored as a miss", id)
		}
		delete(want, id)
	}
	for id := range want {
		t.Errorf("did not credit %s, which the audit scored as a hit", id)
	}
}

// THE FINDING, and the reason 2 of 12 must not be read as "the agent found two
// of the twelve places".
//
// Measured with the matcher itself, not with a weaker substitute: the agent
// reached 11 of the 12 gold FILES and landed on the gold LINE in 2 of them. So
// on this transcript a path-only matcher scores 11/12 and the strict rule
// scores 2/12, from one answer. Nearly the whole metric lives in that gap,
// which makes the line rule most of what the number measures rather than a
// detail of it.
func TestTheRecordedRunReachedElevenFilesAndTwoLines(t *testing.T) {
	rows, answer := fixture(t)
	cites := Scan(answer)

	var reached, wrongLine int
	for _, r := range rows {
		if !ReachedPath(cites, r.Cite) {
			continue
		}
		reached++
		if !anyMatches(cites, r.Cite) {
			wrongLine++
		}
	}

	if reached != 11 {
		t.Errorf("the agent reached %d of 12 gold files, want 11 — the audit found only "+
			"lib/tasks/search.rake never named", reached)
	}
	if wrongLine != 9 {
		t.Errorf("%d rows were the right file at the wrong line, want 9", wrongLine)
	}

	// Asserted on the RENDERED report, because the line's only job is to stop
	// `cited 2 of 12` being read as "found two of the twelve places". Checking
	// the struct fields leaves the report free to print the wrong figure, and
	// checking only the words leaves it free to print `mentioned 2 of 12`.
	//
	// The denominator is part of the claim: "11 of 12 gold files" out of a
	// 182-file answer is far weaker than it reads as, and a longer answer buys
	// mentions for free.
	//
	// 169, where cycle 00 recorded 168. The one extra file is one the answer
	// named only by eliding its path, which the old matcher could not see.
	//
	// The denominator is deliberately files and not citations. An intermediate
	// version of this matcher read 182, because "a dot and a few lowercase
	// letters" also describes `account.id` and `Account.new`; counting those as
	// files inflated the denominator by a tenth. Extensions are a closed set
	// now, and the audit of what that dropped found no real file among them.
	const want = "mentioned  11 of 12 gold files, at any line, out of 169 files named"
	if got := Group("dependents", rows, text(answer), 0.50).String(); !strings.Contains(got, want) {
		t.Errorf("the report does not carry\n\t%s\ngot:\n%s", want, got)
	}
}

// The misses are systematic, not scatter, and that is what makes them cycle
// 02's problem rather than a tolerance to tune: in every row below the agent
// cited a line THE GOLD ROW'S OWN PROSE NAMES, just not the one gold scores.
// The scenario's step-2 prompt asks for the routine's file:line; its gold rows
// store the call site. The two do not ask for the same thing.
//
// Seven of these are the routine's DEFINITION, which gold writes as "def at
// :N". One is not, and it is kept separate because it is a different agent
// behaviour that cycle 02 will need a different answer for: on d:narrative-bot
// the agent cited :4, which gold calls "the state table opens at :4" — the
// file's opening structure, two levels out from the call site at :42 and its
// Proc at :41.
//
// A ninth row, d:reindex-search-job, belongs to this group too and is invisible
// here because its citation elides the path; see TestTheAnswerElidesRepeatedPaths.
//
// Pinned per row with the line the agent actually chose, so a matcher change
// has to confront the specific cases rather than a count.
func TestTheRecordedRunMissesAreLinesTheGoldRowItselfNames(t *testing.T) {
	_, answer := fixture(t)
	cites := Scan(answer)

	for _, tc := range []struct {
		id       string
		gold     string // the call site gold records
		agentHit string // the line the agent chose instead
	}{
		{"d:migrations-importer-categories", "migrations/lib/importer/steps/categories/categories.rb:124", "123"},
		{"d:bulk-import-merger", "script/bulk_import/discourse_merger.rb:230", "226"},
		{"d:import-scripts-base", "script/import_scripts/base.rb:467", "465"},
		{"d:templates-rake", "plugins/discourse-templates/lib/tasks/discourse-templates.rake:113", "100"},
		{"d:slack-migration", "plugins/discourse-chat-integration/app/jobs/onceoff/migrate_from_slack_official.rb:52", "35"},
		{"d:semantic-categorizer", "plugins/discourse-ai/lib/ai_helper/semantic_categorizer.rb:25", "13"},
		{"d:templates-topic-query", "plugins/discourse-templates/lib/discourse_templates/topic_query_extension.rb:9", "4"},
		// Not a def line: gold calls :4 "the state table opens at :4", and
		// names the Proc's def separately at :41.
		{"d:narrative-bot", "plugins/discourse-narrative-bot/lib/discourse_narrative_bot/advanced_user_narrative.rb:42", "4"},
	} {
		t.Run(tc.id, func(t *testing.T) {
			if anyMatches(cites, tc.gold) {
				t.Fatalf("%s now matches the gold call site; the audit recorded it as a miss", tc.id)
			}
			lines := LinesCitedFor(cites, tc.gold)
			if len(lines) == 0 {
				t.Fatalf("%s: the agent never cited this file at all", tc.id)
			}
			var found bool
			for _, l := range lines {
				if l == tc.agentHit {
					found = true
				}
			}
			if !found {
				t.Errorf("%s: the agent cited lines %v, and the audit recorded %s",
					tc.id, lines, tc.agentHit)
			}
		})
	}
}

// ONE row is never found at all: lib/tasks/search.rake is not named in the
// answer under any of its names, at any line. Every other row was reached.
//
// This is a narrower claim than "one row misses under any convention", which is
// what an earlier version of this test said and is not true: d:narrative-bot
// misses under both the call-site and the routine-def conventions. It is also
// narrower than an earlier claim that d:reindex-search-job was a second genuine
// miss, which was false — see TestTheAnswerElidesRepeatedPaths for how.
func TestOnlyOneRowIsNeverFoundAtAll(t *testing.T) {
	rows, answer := fixture(t)
	cites := Scan(answer)

	var cite string
	for _, r := range rows {
		if r.ID == "d:search-rake" {
			cite = r.Cite
		}
	}
	if cite == "" {
		t.Fatal("d:search-rake is not in the gold group")
	}
	if anyMatches(cites, cite) {
		t.Error("d:search-rake is credited, but the file is never named in the answer")
	}
	if ReachedPath(cites, cite) {
		t.Error("d:search-rake reports as reached; the audit found the file never named")
	}
	for _, name := range []string{"search.rake", "search:reindex"} {
		if strings.Contains(answer, name) {
			t.Errorf("the answer does mention %q, so this row is not the miss the audit recorded", name)
		}
	}
}

// A blind spot in the matcher, measured on this run and handed forward.
//
// The agent writes a citation with the path ELIDED when it is the same file as
// the item before it:
//
//  43. `Jobs::ReindexSearch#rebuild_categories` — `app/jobs/scheduled/reindex_search.rb:22` — …
//  44. `Jobs::ReindexSearch#load_problem_category_ids` — `:111` — …
//
// The scorer requires a path on every citation, so it never sees item 44. On
// this transcript that hides 65 distinct locations behind 254 visible ones: a
// 26% blind spot, and it is invisible in the score because it can only ever
// lower it.
//
// It changed a conclusion. d:reindex-search-job records call site :112 and
// declares its def at :111, and the agent cited exactly `:111` — so it is a
// routine-def row like the other nine, not the second genuine miss an earlier
// audit called it. The headline is unaffected: 2 of 12 either way.
//
// Resolving elided paths is stateful matching and belongs to cycle 02, not
// here. This pins the size of the problem so the decision is made against a
// measurement.
func TestTheAnswerElidesRepeatedPaths(t *testing.T) {
	_, answer := fixture(t)

	elided := regexp.MustCompile("(?:[^A-Za-z0-9._/\\-]|^)`:(\\d+)`").FindAllString(answer, -1)

	// 80 in the answer itself. The audit counted 160 because the duplicated
	// result event doubled every one of them.
	if len(elided) < 50 {
		t.Errorf("found %d path-elided citations, want the ~80 the audit measured once the "+
			"duplicated result event is excluded; if the shape has changed, the blind spot "+
			"this records may have too", len(elided))
	}
	// The specific one that changed a conclusion.
	if !strings.Contains(answer, "`:111`") {
		t.Error("the answer no longer carries the elided `:111` that locates d:reindex-search-job")
	}
}

// The reduced fixture must still be a real stream-json capture, not something
// hand-written that happens to parse. If the reduction ever loses the answer
// events, every expectation above becomes vacuous.
func TestTheFixtureIsARealCaptureCarryingTheAnswer(t *testing.T) {
	_, answer := fixture(t)

	// Exact, not a threshold. Half this answer is the result event repeating the
	// last assistant message, so a generous lower bound passes with that entire
	// event deleted — and every expectation in this file would then be
	// measuring half a run.
	if len(answer) != fixtureAnswerChars {
		t.Errorf("the fixture answer is %d chars, want exactly %d; the capture has changed",
			len(answer), fixtureAnswerChars)
	}
	if n := len(Citations(answer)); n != fixtureCitations {
		t.Errorf("the fixture carries %d citations, want exactly %d", n, fixtureCitations)
	}
	if !strings.Contains(answer, "Category") {
		t.Error("the fixture does not mention the scenario's anchor; it is not this run")
	}
}

// What resolving elision is actually worth, on a real recorded answer.
//
// This is the headline claim for why the scanner is stateful, so it is measured
// against a checked-in fixture rather than quoted. The figure carried over from
// cycle 00 was 319, arrived at by counting regex matches; the matcher that got
// built yields 322, and an inherited estimate repeated as a measurement is how
// a wrong number survives three documents.
func TestWhatElisionResolutionIsWorthOnARecordedAnswer(t *testing.T) {
	_, answer := fixture(t)

	strict := map[string]bool{}
	for _, c := range Citations(answer) {
		strict[c] = true
	}
	resolved := map[string]bool{}
	for _, c := range Scan(answer) {
		if c.Line == 0 {
			continue
		}
		where := c.Path
		if where == "" {
			where = c.Established
		}
		if where == "" {
			where = c.Symbol
		}
		resolved[fmt.Sprintf("%s:%d", where, c.Line)] = true
	}

	if len(strict) != 254 {
		t.Errorf("the strict path:line count is %d, want the recorded 254", len(strict))
	}
	if len(resolved) != 331 {
		t.Errorf("resolution yields %d locations, want 331", len(resolved))
	}
	// And it must be a superset: a scanner that gained 68 while quietly losing
	// some of the original 254 would show the same total and be worse.
	for c := range strict {
		if !resolved[c] {
			t.Errorf("resolution LOST the plain citation %s", c)
		}
	}
}
