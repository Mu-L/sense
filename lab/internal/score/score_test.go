package score

import (
	"strings"
	"testing"
)

// incomplete is a Source standing in for a capture that is missing part of
// itself.
type incomplete string

func (i incomplete) Answer() string         { return "a.rb:1" }
func (i incomplete) ProvisionalWhy() string { return string(i) }

func rows(cites ...string) []Row {
	var out []Row
	for i, c := range cites {
		out = append(out, Row{ID: string(rune('a' + i)), Cite: c})
	}
	return out
}

// The whole point of this metric is that it credits a location, not a file. The
// right file at the wrong line sends a teammate to the wrong place, and a file
// named with no line sends them nowhere at all — so neither may count. These
// are the cases the pitch names, and the ones the instrument being replaced
// gets wrong.
func TestOnlyAnExactPathAndLineCounts(t *testing.T) {
	gold := rows("app/models/category.rb:1083")

	for _, tc := range []struct {
		name string
		text string
		want bool
	}{
		{
			name: "the same file and the same line",
			text: "See app/models/category.rb:1083 for the entry point.",
			want: true,
		},
		{
			name: "the same file, one line off",
			text: "See app/models/category.rb:1084 for the entry point.",
			want: false,
		},
		{
			name: "the file with no line at all",
			text: "It is all in app/models/category.rb somewhere.",
			want: false,
		},
		{
			name: "a different file at the same line",
			text: "See app/models/topic.rb:1083 for the entry point.",
			want: false,
		},
		{
			name: "a file gold does not mention",
			text: "See lib/tasks/search.rake:32 instead.",
			want: false,
		},
		{
			name: "the line number appearing as bare prose",
			text: "app/models/category.rb, around line 1083.",
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Group("g", gold, text(tc.text), 0.5)

			if cited := got.Cited == 1; cited != tc.want {
				t.Errorf("cited = %v, want %v (recall %.2f)", cited, tc.want, got.Recall)
			}
		})
	}
}

// An agent writes a citation inside backticks, in a bullet, in a table cell and
// in the middle of a sentence. All of those are the same citation, and a
// scorer that only recognises one shape scores the formatting rather than the
// answer.
func TestACitationIsFoundHoweverTheAgentDressesIt(t *testing.T) {
	gold := rows("app/models/category.rb:1083")

	for _, tc := range []string{
		"`app/models/category.rb:1083`",
		"- **app/models/category.rb:1083** — the entry point",
		"| app/models/category.rb:1083 | the entry point |",
		"defined at app/models/category.rb:1083, and called below",
		"(app/models/category.rb:1083)",
		"./app/models/category.rb:1083",
	} {
		t.Run(tc, func(t *testing.T) {
			if got := Group("g", gold, text(tc), 0.5); got.Cited != 1 {
				t.Errorf("not cited in %q", tc)
			}
		})
	}
}

// Gold is repository-relative, and an agent may write MORE leading directories
// than gold has (a path from the machine's checkout, say). Matching on the path
// suffix handles that, but it must stop at a directory boundary: gold "base.rb"
// must not be credited by a citation of "my_base.rb", which ends the same way
// and is a different file.
//
// Gold is deliberately the SHORT side here. Written the other way round, every
// negative case fails on strings.HasSuffix before the boundary check is
// reached, and deleting the boundary check changes nothing.
func TestPathMatchingStopsAtADirectoryBoundary(t *testing.T) {
	gold := rows("base.rb:467")

	for _, tc := range []struct {
		name string
		text string
		want bool
	}{
		{"exactly the gold path", "base.rb:467", true},
		{"one directory above, at a boundary", "import_scripts/base.rb:467", true},
		{"several directories above", "script/import_scripts/base.rb:467", true},
		{"a longer name ending the same way", "my_base.rb:467", false},
		{"a longer name ending the same way, in a directory", "script/my_base.rb:467", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if cited := Group("g", gold, text(tc.text), 0.5).Cited == 1; cited != tc.want {
				t.Errorf("cited = %v, want %v", cited, tc.want)
			}
		})
	}
}

// A citation SHORTER than the gold path is a bare basename, and crediting it is
// the path-only matching this package exists to refuse. Discourse has many
// files called category.rb, so a bare "category.rb:1083" would credit a row
// about a different one on a coincidence of line number.
func TestACitationShorterThanGoldIsNeverCredited(t *testing.T) {
	gold := rows("app/models/category.rb:1083")

	for _, tc := range []string{
		"it is in category.rb:1083",
		"see models/category.rb:1083",
	} {
		t.Run(tc, func(t *testing.T) {
			if got := Group("g", gold, text(tc), 0.5); got.Cited != 0 {
				t.Error("a citation with less path than gold was credited; " +
					"discourse has many files called category.rb, so that is a coincidence of line number")
			}
		})
	}
}

// A row with no location cannot be scored strictly, and crediting it would be
// exactly the path-only matching this scorer refuses.
func TestAGoldRowWithNoLocationIsAMissNotAFreeHit(t *testing.T) {
	got := Group("g", []Row{{ID: "no-location", Cite: ""}}, text("anything at all.rb:1"), 0.5)

	if got.Cited != 0 {
		t.Errorf("cited = %d, want 0", got.Cited)
	}
	if len(got.Misses) != 1 || got.Misses[0] != "no-location" {
		t.Errorf("misses = %v, want the unlocatable row reported", got.Misses)
	}
}

func TestRecallAndVerdictAgainstTheFloor(t *testing.T) {
	gold := rows("a.rb:1", "b.rb:2", "c.rb:3", "d.rb:4")

	for _, tc := range []struct {
		name        string
		text        string
		wantRecall  float64
		wantVerdict string
	}{
		{"nothing cited", "no citations here", 0.0, BelowFloor},
		{"one of four, under the floor", "a.rb:1", 0.25, BelowFloor},
		{"two of four, exactly at the floor", "a.rb:1 b.rb:2", 0.5, AtOrAboveFloor},
		{"all four", "a.rb:1 b.rb:2 c.rb:3 d.rb:4", 1.0, AtOrAboveFloor},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Group("g", gold, text(tc.text), 0.5)

			if got.Recall != tc.wantRecall {
				t.Errorf("recall = %.2f, want %.2f", got.Recall, tc.wantRecall)
			}
			// The floor is inclusive. A run that lands exactly on it passes,
			// and getting that boundary wrong silently reclassifies every
			// borderline cell in the corpus.
			if got.Verdict != tc.wantVerdict {
				t.Errorf("verdict = %q, want %q", got.Verdict, tc.wantVerdict)
			}
		})
	}
}

// Citations reports each location once, in the order it first appears. An
// answer that repeats itself in a summary section describes the same places,
// and anything counting citations rather than gold rows would otherwise read a
// repetitive answer as a richer one.
func TestCitationsAreReportedOnceInFirstAppearanceOrder(t *testing.T) {
	got := Citations("first b.rb:2, then a.rb:1, then b.rb:2 again, and a.rb:1 once more")

	want := []string{"b.rb:2", "a.rb:1"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

func TestAnEmptyGoldGroupScoresZeroWithoutDividingByZero(t *testing.T) {
	got := Group("g", nil, text("a.rb:1"), 0.5)

	if got.Total != 0 || got.Cited != 0 || got.Recall != 0 {
		t.Errorf("got %+v, want an empty result", got)
	}
}

func TestTheResultNamesWhatWasMissed(t *testing.T) {
	gold := []Row{{ID: "found", Cite: "a.rb:1"}, {ID: "lost", Cite: "b.rb:2"}}

	// A 0.75 floor, so 1 of 2 lands below it and the verdict line is the
	// interesting one to read.
	out := Group("dependents", gold, text("a.rb:1"), 0.75).String()

	// "not a score" is asserted, not decoration: the mention count is
	// volume-sensitive and the report must never present it as a second metric.
	for _, want := range []string{"dependents", "1 of 2", "0.50", "below floor", "lost",
		"mentioned", "not a score"} {
		if !strings.Contains(out, want) {
			t.Errorf("output is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "found") {
		t.Errorf("output lists a row that was cited:\n%s", out)
	}
}

// ReachedPath and LinesCitedFor separate "never found the place" from "found
// the file and pointed at the wrong line". They are not part of the score, and
// they are what makes a miss readable rather than merely counted.
func TestReachedPathAndLinesCitedForReadAMiss(t *testing.T) {
	cites := Scan("look at app/models/category.rb:12, and again at app/models/category.rb:99")

	t.Run("the file was reached at the wrong lines", func(t *testing.T) {
		gold := "app/models/category.rb:1083"
		if !ReachedPath(cites, gold) {
			t.Error("the file was cited twice but reports as unreached")
		}
		if anyMatches(cites, gold) {
			t.Error("a wrong-line citation was scored as a hit")
		}
		got := LinesCitedFor(cites, gold)
		if len(got) != 2 || got[0] != "12" || got[1] != "99" {
			t.Errorf("lines = %v, want [12 99] in the order they appear", got)
		}
	})

	t.Run("a file never named", func(t *testing.T) {
		gold := "lib/tasks/search.rake:32"
		if ReachedPath(cites, gold) {
			t.Error("a file the answer never names reports as reached")
		}
		if got := LinesCitedFor(cites, gold); got != nil {
			t.Errorf("lines = %v, want none", got)
		}
	})

	// A row with no location is unreachable rather than reached-at-no-line, so
	// it cannot quietly count towards the reach figure either.
	t.Run("a gold row with no location", func(t *testing.T) {
		if ReachedPath(cites, "") {
			t.Error("a row with no location reports as reached")
		}
		if got := LinesCitedFor(cites, ""); got != nil {
			t.Errorf("lines = %v, want none", got)
		}
	})
}

// The provisional mark leads the report, because it changes what every line
// under it means: the number is not a low result, it is a number taken from a
// capture that is missing part of itself.
func TestAProvisionalResultSaysSoBeforeItSaysAnythingElse(t *testing.T) {
	// Scored from a source that says it is incomplete. The mark cannot be set
	// after the fact any more: Group takes the source, so nothing has to
	// remember to copy it.
	r := Group("dependents", rows("a.rb:1", "b.rb:2"), incomplete("3 unreadable lines"), 0.5)

	out := r.String()

	if !strings.HasPrefix(out, "PROVISIONAL  3 unreadable lines") {
		t.Errorf("report = %q, want the mark and its reason first", out)
	}
	if !strings.Contains(out, "this number is not a result") {
		t.Errorf("report does not say what the mark means:\n%s", out)
	}
	// The number is still there: a provisional result is worth reading, it is
	// just not worth banking.
	if !strings.Contains(out, "cited      1 of 2") {
		t.Errorf("report dropped the number:\n%s", out)
	}
}
