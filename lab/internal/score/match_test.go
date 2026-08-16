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

	t.Run("a line of zero renders as zero", func(t *testing.T) {
		// Matches returns early on Line == 0, so this is the one caller that
		// could ever hand itoa a zero, and it must not print an empty string.
		if got := itoa(0); got != "0" {
			t.Errorf("itoa(0) = %q", got)
		}
	})
}
