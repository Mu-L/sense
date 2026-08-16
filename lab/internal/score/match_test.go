package score

import "testing"

// The strict half. A path citation is checked against the gold LINE, because
// the right file at the wrong line is a teammate sent to the wrong place, and
// that is the thing the metric exists to measure.
func TestAPathCitationIsHeldToTheLine(t *testing.T) {
	const goldPath, goldLine = "app/models/category.rb", "1083"

	for _, tc := range []struct {
		name string
		cite Cite
		want bool
	}{
		{"the same file and the same line", Cite{Path: goldPath, Line: 1083}, true},
		{"the same file, one line off", Cite{Path: goldPath, Line: 1084}, false},
		{"the same file, the definition line instead of the use", Cite{Path: goldPath, Line: 1080}, false},
		{"the file with no line at all", Cite{Path: goldPath}, false},
		{"a deeper path ending at the gold one", Cite{Path: "discourse/" + goldPath, Line: 1083}, true},
		// The refused direction. Discourse has many files called category.rb,
		// so a bare basename would credit a row about a different one on a
		// coincidence of line number.
		{"a bare basename", Cite{Path: "category.rb", Line: 1083}, false},
		{"a path that merely ends with the same letters", Cite{Path: "app/models/my_category.rb", Line: 1083}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Matches(tc.cite, goldPath, goldLine); got != tc.want {
				t.Errorf("Matches(%+v) = %v, want %v", tc.cite, got, tc.want)
			}
		})
	}
}

// The loose half, and the reason it is loose. A symbol citation is matched on
// the symbol; its line must be PRESENT but is not compared, because comparing
// it needs the symbol's line range, which needs a repository on disk (02-04).
//
// This is the recorded ruling of 2026-08-04. It is also a known looseness, and
// the test says so, so that nobody reads the asymmetry as an oversight.
func TestASymbolCitationIsMatchedOnTheSymbolAndNotOnItsLine(t *testing.T) {
	const goldPath, goldLine = "app/controllers/admin/action_logs_controller.rb", "9"

	t.Run("the line differs from gold and it still counts", func(t *testing.T) {
		c := Cite{Symbol: "Admin::ActionLogsController#index", Line: 7}
		if !Matches(c, goldPath, goldLine) {
			t.Error("the form the recorded defect missed is still missed")
		}
	})

	t.Run("a symbol with no line at all does not count", func(t *testing.T) {
		c := Cite{Symbol: "Admin::ActionLogsController#index"}
		if Matches(c, goldPath, goldLine) {
			t.Error("a bare symbol with no line was credited; the ruling requires the line")
		}
	})

	t.Run("a different class does not count", func(t *testing.T) {
		c := Cite{Symbol: "Admin::StatusesController#index", Line: 9}
		if Matches(c, goldPath, goldLine) {
			t.Error("an unrelated controller matched")
		}
	})
}

// Ruby REQUIRES the constant-to-file mapping — a class not in its derived file
// does not load — so this is a language rule rather than a guess about layout.
func TestARubyConstantNamesItsOwnFile(t *testing.T) {
	for _, tc := range []struct {
		symbol, goldPath string
		want             bool
	}{
		{"Admin::ActionLogsController#index", "app/controllers/admin/action_logs_controller.rb", true},
		{"Api::V1::Statuses::FavouritedByAccountsController#default_accounts",
			"app/controllers/api/v1/statuses/favourited_by_accounts_controller.rb", true},
		{"Admin::Metrics::Dimension::SpaceUsageDimension#media_size",
			"app/lib/admin/metrics/dimension/space_usage_dimension.rb", true},
		// The suffix must land on a path boundary, exactly as for paths.
		{"Category#find", "app/models/my_category.rb", false},
		{"Admin::ActionLogsController#index", "app/controllers/admin/statuses_controller.rb", false},
	} {
		t.Run(tc.symbol+" -> "+tc.goldPath, func(t *testing.T) {
			if got := symbolNamesPath(tc.symbol, tc.goldPath); got != tc.want {
				t.Errorf("symbolNamesPath = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRubysUnderscoreRule(t *testing.T) {
	for _, tc := range [][2]string{
		{"ActionLogsController", "action_logs_controller"},
		{"V1", "v1"},
		{"SpaceUsageDimension", "space_usage_dimension"},
		{"Category", "category"},
		// A run of capitals breaks before the last one when a lowercase
		// follows: APIController is api_controller, not a_p_i_controller.
		{"APIController", "api_controller"},
	} {
		if got := underscore(tc[0]); got != tc[1] {
			t.Errorf("underscore(%q) = %q, want %q", tc[0], got, tc[1])
		}
	}
}

// The guards, exercised directly rather than left as branches nothing reaches.
// Each one protects against a caller passing something the happy path never
// produces, and an unreached guard is a guard nobody has checked.
func TestTheMatcherRefusesInputThatNamesNoPlace(t *testing.T) {
	t.Run("a symbol with no class part", func(t *testing.T) {
		if symbolNamesPath("#index", "app/controllers/admin/action_logs_controller.rb") {
			t.Error("a bare method with no class matched a file")
		}
	})

	t.Run("a gold row with no location", func(t *testing.T) {
		cites := Scan("app/models/category.rb:1083")
		if howMatched(cites, "") != matchedNone {
			t.Error("a row with no location was matched")
		}
	})

	t.Run("a symbol cite with no established file and no matching name", func(t *testing.T) {
		c := Cite{Symbol: "Unrelated::Thing#go", Line: 3}
		if Matches(c, "app/models/category.rb", "3") {
			t.Error("an unrelated symbol matched on the line alone")
		}
	})
}

// Naming the file first must never cost a hit.
//
// The corroborated file used to be written into Path, which made Matches take
// the strict branch and skip the symbol rule entirely — so an answer that gave
// the path and then the symbol scored WORSE than the same answer with the path
// left out. A scorer that punishes extra information is measuring the wrong
// thing.
func TestNamingTheFileFirstNeverCostsAHit(t *testing.T) {
	const gold = "app/controllers/admin/action_logs_controller.rb"
	const goldLine = "9"

	withPath := Scan("`app/controllers/admin/action_logs_controller.rb:5`. " +
		"Then `Admin::ActionLogsController#index` `:7`.")
	without := Scan("Then `Admin::ActionLogsController#index` `:7`.")

	if !anyMatchesAt(without, gold, goldLine) {
		t.Fatal("the symbol alone did not match; the premise of this test is gone")
	}
	if !anyMatchesAt(withPath, gold, goldLine) {
		t.Error("the same answer scored worse for having named the path first")
	}
}

func anyMatchesAt(cites []Cite, goldPath, goldLine string) bool {
	for _, c := range cites {
		if Matches(c, goldPath, goldLine) {
			return true
		}
	}
	return false
}

// The established file is a STRICTER door, so when it lands on the gold line
// the row counts as a strict hit rather than a loose one — and the report's
// composition has to say so.
func TestAnEstablishedFileOnTheGoldLineCountsAsStrict(t *testing.T) {
	cites := Scan("`app/jobs/scheduled/reindex_search.rb:22` then " +
		"`Jobs::ReindexSearch#load_problem_category_ids` `:111`")

	if got := howMatched(cites, "app/jobs/scheduled/reindex_search.rb:111"); got != matchedPath {
		t.Errorf("matched %v, want the strict path form", got)
	}
}

// Ruby's rule is constant-path-under-an-autoload-ROOT, and a root is one or two
// segments. A constant that skips a directory does not name the file: with
// `admin/` in the path, Ruby requires Admin::ActionLogsController, so the bare
// name must not match — that is the same bare-name credit samePath refuses at
// the front door, arriving through the symbol door.
func TestASymbolMustCarryTheNamespacesInItsPath(t *testing.T) {
	for _, tc := range []struct {
		name, symbol, goldPath string
		want                   bool
	}{
		{"the namespace is present", "Admin::ActionLogsController#index",
			"app/controllers/admin/action_logs_controller.rb", true},
		{"the namespace is missing", "ActionLogsController#index",
			"app/controllers/admin/action_logs_controller.rb", false},
		{"the namespace is missing, deeper", "AccountSerializer#x",
			"app/serializers/rest/account_serializer.rb", false},
		// The right basename under an entirely different namespace, which is
		// the mutation the earlier table could not see: every negative case in
		// it already failed on the basename alone.
		{"the right name under the wrong namespace", "Admin::ActionLogsController#index",
			"app/controllers/api/action_logs_controller.rb", false},
		// A class at the root of an autoload directory has no namespace to
		// carry, and must still match.
		{"a class with no namespace to carry", "InstancePresenter#x",
			"app/presenters/instance_presenter.rb", true},
		{"a one-segment root", "Category#find", "lib/category.rb", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := symbolNamesPath(tc.symbol, tc.goldPath); got != tc.want {
				t.Errorf("symbolNamesPath(%q, %q) = %v, want %v", tc.symbol, tc.goldPath, got, tc.want)
			}
		})
	}
}

// A cite that names nothing at all matches nothing. It is not producible by
// Scan, and the guard is here because the exported entry point takes a struct
// anyone can build.
func TestACiteThatNamesNothingMatchesNothing(t *testing.T) {
	if Matches(Cite{Line: 12}, "app/models/category.rb", "12") {
		t.Error("an empty cite matched on its line alone")
	}
	if NamesFile(Cite{Line: 12}, "app/models/category.rb") {
		t.Error("an empty cite named a file")
	}
}

// A constant at the very root of the tree, with no autoload directory in front
// of it, still names its own file.
func TestAConstantAtTheRootNamesItsFileExactly(t *testing.T) {
	if !symbolNamesPath("Category#find", "category.rb") {
		t.Error("a constant did not name the file of exactly its own name")
	}
	if symbolNamesPath("#nothing", "category.rb") {
		t.Error("a method with no class in front of it named a file")
	}
}

// Reach follows the established file too. A continuation like `:111` under a
// file the answer named earlier has found that file, whatever its line says,
// and counting it as never-found would misread the failure.
func TestReachFollowsTheEstablishedFile(t *testing.T) {
	cites := Scan("`app/jobs/scheduled/reindex_search.rb:22` then " +
		"`Jobs::ReindexSearch#load_problem_category_ids` `:111`")
	last := cites[len(cites)-1]

	if last.Path != "" {
		t.Fatalf("premise gone: the continuation carries a Path %+v", last)
	}
	if !NamesFile(last, "app/jobs/scheduled/reindex_search.rb") {
		t.Error("the continuation did not count as reaching the file it continues")
	}
}
