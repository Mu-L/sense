package scenario

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/score"
)

// The rendered prompt is the only thing the agent ever sees, so everything the
// scenario means has to survive into it, in order.
func TestThePromptCarriesTheDescriptionAndEveryStepInOrder(t *testing.T) {
	s := Scenario{
		Description: "You are a maintainer.",
		Steps: []Step{
			{Name: "First", Prompt: "Trace the path."},
			{Name: "Second", Prompt: "Audit the dependents."},
		},
	}

	got := s.Prompt()

	for _, want := range []string{
		"You are a maintainer.",
		// Without this sentence a two-step scenario reads as one question, and
		// an agent that answers step one and stops scores as a weak arm rather
		// than a half-run.
		"Work through the following steps in order.",
		"Step 1: First", "Trace the path.",
		"Step 2: Second", "Audit the dependents.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt is missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "Step 1") > strings.Index(got, "Step 2") {
		t.Error("the steps are rendered out of order")
	}
}

// Step prompts in the corpus are YAML folded scalars, which arrive with a
// trailing newline and inconsistent leading space. The agent should not be
// asked to read around that.
func TestThePromptDoesNotInheritTheFilesIncidentalWhitespace(t *testing.T) {
	s := Scenario{
		Description: "\n  A description.\n\n",
		Steps:       []Step{{Name: "Only", Prompt: "\n  Do the thing.\n\n"}},
	}

	got := s.Prompt()

	if strings.HasPrefix(got, "\n") || strings.HasPrefix(got, " ") {
		t.Errorf("prompt starts with whitespace: %q", got[:10])
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("prompt has a run of blank lines:\n%s", got)
	}
}

// The corpus keeps a gold row's authoritative location as the FIRST path:line
// of its Relation prose, with the reason in English after it. The Match field
// carries a path with no line, so scoring against Match is the path-only
// matching the scorer exists to refuse — Cite is what makes strict matching
// possible at all.
func TestCiteTakesTheRowsLocationFromTheFrontOfItsRelation(t *testing.T) {
	for _, tc := range []struct {
		name     string
		relation string
		want     string
	}{
		{
			name:     "a real corpus row",
			relation: "migrations/lib/uploader/tasks/optimizer.rb:43 `@category_id = Category.last.id` inside `#initialize` (def at :40) - the upload optimiser grabs one container up front",
			want:     "migrations/lib/uploader/tasks/optimizer.rb:43",
		},
		{
			// The prose after the location is full of ":40" style references
			// to other lines. Only the leading one is the row's own location.
			name:     "later line references are not the location",
			relation: "app/models/category.rb:1083 `find_by_slug_path` - see also :1103 and :985",
			want:     "app/models/category.rb:1083",
		},
		{
			name:     "leading whitespace from a folded scalar",
			relation: "\n  lib/tasks/search.rake:32 the reindex task\n",
			want:     "lib/tasks/search.rake:32",
		},
		{
			// The anchor is what makes this the ROW'S location rather than the
			// first location mentioned anywhere in its reason. Without it, a
			// row whose prose opens with a few words takes its location from
			// wherever one happens to appear, which could be the row's
			// neighbour.
			name:     "a location buried after prose is not the row's own",
			relation: "the entry point is app/models/category.rb:1083",
			want:     "",
		},
		{
			name:     "prose with no location at all",
			relation: "somewhere in the search code",
			want:     "",
		},
		{
			// Rakefile, Gemfile and Dockerfile all exist in discourse and none
			// of them is a citable path under this rule.
			name:     "an extensionless path is not a location",
			relation: "Rakefile:12 the task list",
			want:     "",
		},
		{
			name:     "a path with no line is not a location",
			relation: "app/models/category.rb is the class under rework",
			want:     "",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := (GoldRow{Relation: tc.relation}).Cite(); got != tc.want {
				t.Errorf("Cite() = %q, want %q", got, tc.want)
			}
		})
	}
}

// The gold side and the answer side must accept the same shapes. They are in
// packages that do not import each other, so nothing but this test stops one
// from drifting: a shape gold takes that a citation cannot match leaves the row
// permanently unmatchable, and it fails as a low score rather than as an error.
func TestCitePatternMatchesTheScorer(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"app/models/category.rb:1083", true},
		{"lib/tasks/search.rake:32", true},
		{"a.js:1", true},
		{"plugins/discourse-ai/lib/ai_helper/semantic_categorizer.rb:25", true},
		{"app/models/category.rb", false}, // no line
		{"Rakefile:12", false},            // no extension
		{"step 2:12", false},              // prose, not a path
	} {
		t.Run(tc.text, func(t *testing.T) {
			gold := (GoldRow{Relation: tc.text}).Cite() != ""
			answer := len(score.Citations(tc.text)) > 0

			if gold != answer {
				t.Errorf("gold accepts=%v but the scorer accepts=%v; the two sides have drifted",
					gold, answer)
			}
			if gold != tc.want {
				t.Errorf("accepted=%v, want %v", gold, tc.want)
			}
		})
	}
}
