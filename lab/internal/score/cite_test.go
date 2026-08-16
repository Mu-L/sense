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
		if last.Established != "app/jobs/scheduled/reindex_search.rb" || last.Line != 111 {
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
		if c.Symbol == "Admin::ActionLogsController#index" && c.Established != "" {
			t.Errorf("the controller inherited %q, a file its own name does not agree with", c.Established)
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
	if last.Established != "app/jobs/scheduled/reindex_search.rb" {
		t.Errorf("the corroborated file was dropped: %+v", last)
	}
	// And it lands in Established rather than Path, because a symbol cite that
	// also carries a file must never be judged MORE strictly than the same
	// symbol cite without one.
	if last.Path != "" {
		t.Errorf("the corroborated file was written into Path: %+v", last)
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

// A `:\d+` in ordinary prose is not a citation. An earlier version let a bare
// line attach to the last file named at any distance, so a clock time or a port
// number became a strict location in that file — a fabricated citation, in the
// one form the scorer treats as authoritative, that could land on a gold line
// by coincidence.
func TestANumberInProseIsNotACitation(t *testing.T) {
	for _, tc := range []struct{ name, answer string }{
		{"a clock time", "`app/models/user.rb:5` and the job runs at 10:30 every day"},
		{"a port", "`app/models/user.rb:5`, bound to 1.2.3.4:8080"},
		{"a bare line far from anything", "`app/models/user.rb:5`. Much later prose, then `:99`."},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, c := range Scan(tc.answer) {
				if c.Line != 5 {
					t.Errorf("prose became the citation %+v", c)
				}
			}
		})
	}
}

// An elided symbol that resolves to nothing must not leave its line behind for
// the next thing to pick up. The orphan is worse than the drop: it becomes a
// strict path citation at a line the answer never attached to that file.
func TestADroppedElisionDoesNotLeaveItsLineBehind(t *testing.T) {
	got := Scan("`app/models/user.rb:5` then `…Unrelated::Thing#foo:20`")

	for _, c := range got {
		if c.Path == "app/models/user.rb" && c.Line == 20 {
			t.Errorf("the dropped elision donated its line to a file: %+v", got)
		}
	}
}

// A method call is not a file. Reading `Account.new` or `account.id` as a path
// poisons what later elisions resolve against and inflates the count of files
// an answer named, which is printed as the denominator of reach.
func TestAMethodCallIsNeverReadAsAFile(t *testing.T) {
	for _, tc := range []string{"Struct.new", "account.id", "Net::HTTP.get", "Rails.application"} {
		t.Run(tc, func(t *testing.T) {
			for _, c := range Scan(tc + " is called here") {
				if c.Path != "" {
					t.Errorf("%q was read as the file %q", tc, c.Path)
				}
			}
		})
	}
}

// The full ordered output of one answer carrying five different forms. Written
// out in full because the pairwise tests above each read only the last cite,
// so none of them would notice if Scan returned its cites in another order or
// dropped one from the middle.
func TestOneAnswerCarryingEveryFormAtOnce(t *testing.T) {
	answer := "1. `app/models/category.rb:10` — the anchor\n" +
		"2. `Admin::ActionLogsController#index` `:7` — audits\n" +
		"3. `#destroy` `:9` — and its sibling\n" +
		"4. `app/lib/importer/statuses_index_importer.rb` — named, no line\n" +
		"5. `…_index_importer.rb:74` — truncated"

	want := []Cite{
		{Path: "app/models/category.rb", Line: 10},
		{Symbol: "Admin::ActionLogsController#index", Line: 7},
		{Symbol: "Admin::ActionLogsController#destroy", Line: 9},
		{Path: "app/lib/importer/statuses_index_importer.rb"},
		{Path: "app/lib/importer/statuses_index_importer.rb", Line: 74},
	}
	got := Scan(answer)
	if len(got) != len(want) {
		t.Fatalf("Scan returned %d cites, want %d:\n%+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cite %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// An arm enumerating several lines of one file writes them as a list. Measured
// across the corpus these carry 4,333 line numbers a matcher without them never
// reads, and they fall harder on the BASELINE arm (21.0 per transcript against
// 16.6) — a bias in the opposite direction to the symbol rule, and just as real.
func TestSeveralLinesOfOneFileWrittenAsAList(t *testing.T) {
	t.Run("a comma list after a path", func(t *testing.T) {
		got := Scan("see src/Core/X.cs:24,178 GetAvailablePremiumPlan")
		want := []Cite{
			{Path: "src/Core/X.cs", Line: 24},
			{Path: "src/Core/X.cs", Line: 178},
		}
		assertCites(t, got, want)
	})

	t.Run("a longer list", func(t *testing.T) {
		got := Scan("`app/models/category.rb:143,171,450`")
		if len(got) != 3 || got[2].Line != 450 || got[2].Path != "app/models/category.rb" {
			t.Errorf("got %+v", got)
		}
	})

	// A span's ENDS are what the answer named. Its interior is deliberately not
	// credited: 37 otherwise-missed gold rows sit strictly inside a cited range,
	// and crediting them would be a tolerance window adopted because it raised a
	// number.
	t.Run("a range gives its two ends and not its middle", func(t *testing.T) {
		got := Scan("`app/models/category.rb:59-60`")
		assertCites(t, got, []Cite{
			{Path: "app/models/category.rb", Line: 59},
			{Path: "app/models/category.rb", Line: 60},
		})
	})
}

// A continuation continues something ADJACENT. Without that rule a thousands
// separator in prose extends whatever file was named last into a citation.
func TestAThousandsSeparatorIsNotALineList(t *testing.T) {
	for _, answer := range []string{
		"about 1,000 rows in app/models/category.rb:12",
		"app/models/category.rb:12 and then 1,000 users",
	} {
		got := Scan(answer)
		if len(got) != 1 || got[0].Line != 12 {
			t.Errorf("Scan(%q) = %+v, want just the one citation", answer, got)
		}
	}
}

func assertCites(t *testing.T, got, want []Cite) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d cites, want %d:\n%+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("cite %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The guards on the number parsers are not decorative: the pattern matches any
// run of digits, and a run long enough to overflow an int is a string the
// corpus could contain in a hash or an id.
func TestALineNumberTooLargeToBeALineIsRefused(t *testing.T) {
	for _, answer := range []string{
		"`app/models/category.rb` `:99999999999999999999`",
		"app/models/category.rb:12,99999999999999999999",
		"app/models/category.rb:99999999999999999999",
	} {
		for _, c := range Scan(answer) {
			if c.Line < 0 {
				t.Errorf("Scan(%q) produced a negative line: %+v", answer, c)
			}
		}
	}
}

// A list continuing a citation that has no line of its own has nothing to
// continue from, so it is dropped rather than becoming the file's first line.
func TestAListAfterAFileWithNoLineIsDropped(t *testing.T) {
	got := Scan("app/models/category.rb,5 items in total")
	if len(got) != 1 || got[0].Line != 0 {
		t.Errorf("got %+v, want the bare file mention alone", got)
	}
}

// trimEllipsis removes the ellipsis and nothing else, so a token that never had
// one comes back whole. Trimming a rune SET would eat leading dots.
func TestTrimmingAnEllipsisLeavesEverythingElse(t *testing.T) {
	for _, tc := range [][2]string{
		{"…_measure.rb", "_measure.rb"},
		{"..._measure.rb", "_measure.rb"},
		{"_measure.rb", "_measure.rb"},
		{".hidden.rb", ".hidden.rb"},
	} {
		if got := trimEllipsis(tc[0]); got != tc[1] {
			t.Errorf("trimEllipsis(%q) = %q, want %q", tc[0], got, tc[1])
		}
	}
}
