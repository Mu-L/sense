package score

import (
	"strings"
	"testing"
)

func scanOne(t *testing.T, answer string) Cite {
	t.Helper()
	got := Scan(answer)
	if len(got) != 1 {
		t.Fatalf("Scan(%q) returned %d cites, want 1: %+v", answer, len(got), got)
	}
	return got[0]
}

// Every form below is one an arm demonstrably wrote in a recorded transcript,
// with the transcript named. "It might write it this way" is how a matcher
// becomes satisfiable by the route, so a form with no precedent is not here.
func TestEveryFormAnArmActuallyWrote(t *testing.T) {
	for _, tc := range []struct {
		name, answer string
		want         Cite
	}{
		{
			// The overwhelming majority: 25,117 across the corpus.
			name:   "a path with a line",
			answer: "see app/models/category.rb:1083 for the entry point",
			want:   Cite{Path: "app/models/category.rb", Line: 1083},
		},
		{
			// 4,206 across the corpus. A mention, never a hit.
			name:   "a path with no line",
			answer: "it lives in app/models/account.rb somewhere",
			want:   Cite{Path: "app/models/account.rb"},
		},
		{
			// The form the recorded defect missed, banked mastodon cell:
			// the symbol and its line are two separate backtick spans.
			name:   "a symbol and then its line",
			answer: "`Admin::ActionLogsController#index` `:7` — audits",
			want:   Cite{Symbol: "Admin::ActionLogsController#index", Line: 7},
		},
		{
			// 41 across the corpus, the same location written as one token.
			name:   "a symbol carrying its own line",
			answer: "`WhereChain#not:49` builds the negation",
			want:   Cite{Symbol: "WhereChain#not", Line: 49},
		},
		{
			// A path written relative to the tree root. Read as a method call,
			// every citation in the file silently disappears.
			name:   "a path written with a leading dot slash",
			answer: "./app/models/category.rb:1083",
			want:   Cite{Path: "app/models/category.rb", Line: 1083},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := scanOne(t, tc.answer)
			if got.Symbol != tc.want.Symbol || got.Line != tc.want.Line ||
				!strings.HasSuffix(got.Path, tc.want.Path) {
				t.Errorf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

// Elision is stateful: the meaning of a citation depends on the one before it.
// Measured on the checked-in discourse fixture, 8,161 bare :line continuations
// appear across the corpus, so a matcher blind to them is blind to most of what
// answers actually cite.
func TestALineOnItsOwnContinuesFromWhatCameBefore(t *testing.T) {
	t.Run("a list item that drops the repeated path", func(t *testing.T) {
		// The real shape from the discourse fixture, items 43 and 44.
		answer := "43. `Jobs::ReindexSearch#rebuild_categories` — `app/jobs/scheduled/reindex_search.rb:22`\n" +
			"44. `Jobs::ReindexSearch#load_problem_category_ids` — `:111`"

		got := Scan(answer)
		last := got[len(got)-1]
		if last.Path != "app/jobs/scheduled/reindex_search.rb" || last.Line != 111 {
			t.Errorf("the continued citation resolved to %+v, want that file at 111", last)
		}
	})

	t.Run("a second method on the class just named", func(t *testing.T) {
		// `…AccountsController#create` `:17`/`#destroy` `:22`, mastodon.
		answer := "`Api::V1::Lists::AccountsController#create` `:17`/`#destroy` `:22`"

		got := Scan(answer)
		last := got[len(got)-1]
		if last.Symbol != "Api::V1::Lists::AccountsController#destroy" || last.Line != 22 {
			t.Errorf("the continued method resolved to %+v, want #destroy at 22", last)
		}
	})

	t.Run("a namespace replaced by an ellipsis", func(t *testing.T) {
		// The mastodon answer writes the sibling controller this way.
		answer := "`Api::V1::Statuses::FavouritedByAccountsController#default_accounts` `:21`; " +
			"`…RebloggedByAccountsController#default_accounts` `:21`"

		got := Scan(answer)
		last := got[len(got)-1]
		want := "Api::V1::Statuses::RebloggedByAccountsController#default_accounts"
		if last.Symbol != want {
			t.Errorf("the elided namespace resolved to %q, want %q", last.Symbol, want)
		}
	})
}

// An ellipsis that resolves against nothing is DROPPED, not kept as a bare
// suffix. A bare suffix is the shape that lets spec/x.rb pin app/x.rb, which is
// the hazard the path-boundary rule exists to stop, and the recorded note on
// elided paths flags exactly this.
func TestAnEllipsisWithNoAntecedentIsDroppedRatherThanGuessed(t *testing.T) {
	for _, answer := range []string{
		"the fix is in `…source.rb:12`",
		"see `…#matched_scope` for the guard",
	} {
		if got := Scan(answer); len(got) != 0 {
			t.Errorf("Scan(%q) invented %+v from an ellipsis with nothing before it", answer, got)
		}
	}
}

// The other half of the corroboration rule, and the one that makes it safe. An
// inline list of unrelated classes must NOT each inherit whatever file happened
// to be named last, or any of them could land on a gold line by coincidence.
func TestASymbolDoesNotInheritAFileItDoesNotAgreeWith(t *testing.T) {
	answer := "in app/models/category.rb:12 the lookup starts; " +
		"`Admin::ActionLogsController#index` `:7` audits it"

	for _, c := range Scan(answer) {
		if c.Symbol == "Admin::ActionLogsController#index" && c.Path != "" {
			t.Errorf("the controller inherited %q, a file its own name does not agree with", c.Path)
		}
	}
}

// And the case corroboration exists FOR: the symbol agrees with the established
// file, so the continuation keeps the path that the constant rule alone cannot
// derive. Jobs::ReindexSearch derives jobs/reindex_search.rb while the file is
// app/jobs/scheduled/reindex_search.rb, because discourse adds autoload roots.
func TestASymbolKeepsTheFileWhenBothSignalsAgree(t *testing.T) {
	answer := "`app/jobs/scheduled/reindex_search.rb:22` then `Jobs::ReindexSearch#load_problem_category_ids` `:111`"

	got := Scan(answer)
	last := got[len(got)-1]
	if last.Path != "app/jobs/scheduled/reindex_search.rb" {
		t.Errorf("the corroborated file was dropped: %+v", last)
	}
}

// The two elided forms that DO resolve, both with corpus precedent: 35 elided
// paths and 67 elided symbols across the 236 transcripts.
func TestAnEllipsisResolvesAgainstTheThingItTruncated(t *testing.T) {
	t.Run("a truncated path", func(t *testing.T) {
		// `…_measure.rb:20`, mastodon: the arm gives the long path once and
		// then writes only the distinctive tail.
		answer := "`app/lib/admin/metrics/measure/instance_accounts_measure.rb:12`, and again at `…_measure.rb:20`"

		got := Scan(answer)
		last := got[len(got)-1]
		if last.Path != "app/lib/admin/metrics/measure/instance_accounts_measure.rb" || last.Line != 20 {
			t.Errorf("the truncated path resolved to %+v", last)
		}
	})

	t.Run("a truncated symbol naming the same method", func(t *testing.T) {
		answer := "`TopicQuery#matched_scope` `:14` then `…#matched_scope` `:31`"

		got := Scan(answer)
		last := got[len(got)-1]
		if last.Symbol != "TopicQuery#matched_scope" || last.Line != 31 {
			t.Errorf("the truncated symbol resolved to %+v", last)
		}
	})
}

// A line with nothing at all before it is dropped. There is no antecedent to
// continue from, and inventing one is the guess this matcher must not make.
func TestALineWithNothingBeforeItIsDropped(t *testing.T) {
	if got := Scan("the answer opens with `:41` and says no more"); len(got) != 0 {
		t.Errorf("Scan invented %+v from a line with no antecedent", got)
	}
}

// A dot followed by something that is not a lowercase extension is a method
// call, not a file. `Account.find` must not be read as a path.
func TestADotFollowedByNonExtensionTextIsAMethodNotAFile(t *testing.T) {
	got := scanOne(t, "`Account.find_by_username` `:12` resolves it")
	if got.Symbol != "Account.find_by_username" {
		t.Errorf("got %+v, want the method read as a symbol", got)
	}
}

// An ellipsis with nothing attached names nothing, and must not be read as a
// continuation of whatever came before it.
func TestABareEllipsisIsNotACitation(t *testing.T) {
	answer := "`app/models/category.rb:12` … and so on"
	if got := Scan(answer); len(got) != 1 {
		t.Errorf("Scan read %d cites, want just the one real path: %+v", len(got), got)
	}
}

// A dot followed by a capital is a method call in some styles, and it is never
// a file extension.
func TestADotFollowedByACapitalIsNotAFileExtension(t *testing.T) {
	got := scanOne(t, "`Account.Find` `:12`")
	if got.Symbol != "Account.Find" || got.Path != "" {
		t.Errorf("got %+v, want it read as a symbol", got)
	}
}
