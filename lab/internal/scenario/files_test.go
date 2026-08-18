package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	okScenario = `name: audit the category contract
repo: discourse
steps:
  - name: Map the contract
    prompt: Trace the resolution path.
  - name: Audit the dependents
    prompt: Find everywhere that would change with it.
`
	okGold = `discriminator: dependents
rows:
  - id: c:anchor
    group: contract
    relation: "app/models/category.rb:1 the class itself"
  - id: d:one
    group: dependents
    relation: "lib/tasks/search.rake:32 the reindex task"
`
	okRubric = `audience: An AI coding agent about to rework this class.
steps:
  - name: Map the contract
    criteria:
      map_quality:
        weight: 1.0
        question: Is the path traced through the routines that make it up?
  - name: Audit the dependents
    criteria:
      completeness:
        weight: 1.0
        question: Is every dependent pinned to a file:line?
`
)

// set writes the three files and returns the directory. Tests vary one file at
// a time, so each failure has exactly one cause.
func set(t *testing.T, scenario, gold, rubric string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"s.yaml":        scenario,
		"s.gold.yaml":   gold,
		"s.rubric.yaml": rubric,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func loadErr(t *testing.T, dir string) string {
	t.Helper()
	_, err := Load(dir, "s")
	if err == nil {
		t.Fatal("Load accepted a set it should have refused")
	}
	return err.Error()
}

func TestAScenarioLoadsWithItsGoldAndRubric(t *testing.T) {
	got, err := Load(set(t, okScenario, okGold, okRubric), "s")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got.Scenario.Repo != "discourse" || len(got.Scenario.Steps) != 2 {
		t.Errorf("scenario = %+v", got.Scenario)
	}
	if got.Gold.Discriminator != "dependents" || len(got.Gold.Rows) != 2 {
		t.Errorf("gold = %+v", got.Gold)
	}
	if len(got.Rubric.Steps) != 2 || got.Rubric.Audience == "" {
		t.Errorf("rubric = %+v", got.Rubric)
	}
}

// THE property the split exists for. Gold correction is most of the work in
// this bench, and while gold lived inside the scenario a corrected row looked
// like the arms had seen something different — so the honest response was a
// re-run. Three hashes means a gold fix costs a rescore and the transcripts
// stay valid.
func TestEditingGoldChangesTheGoldHashAndLeavesTheScenarioAlone(t *testing.T) {
	before, err := Load(set(t, okScenario, okGold, okRubric), "s")
	if err != nil {
		t.Fatal(err)
	}

	corrected := strings.Replace(okGold, "search.rake:32", "search.rake:41", 1)
	after, err := Load(set(t, okScenario, corrected, okRubric), "s")
	if err != nil {
		t.Fatal(err)
	}

	if after.GoldID == before.GoldID {
		t.Error("correcting a gold row did not change the gold hash")
	}
	if after.ScenarioID != before.ScenarioID {
		t.Error("correcting a gold row changed the SCENARIO hash, so it would read as a re-run")
	}
	if after.RubricID != before.RubricID {
		t.Error("correcting a gold row changed the rubric hash")
	}
}

// And the same in the other two directions, so each hash tracks its own file.
func TestEachFileHasItsOwnHash(t *testing.T) {
	base, err := Load(set(t, okScenario, okGold, okRubric), "s")
	if err != nil {
		t.Fatal(err)
	}

	t.Run("editing the scenario means a re-run and nothing else", func(t *testing.T) {
		edited := strings.Replace(okScenario, "Trace the resolution path.", "Trace it end to end.", 1)
		got, err := Load(set(t, edited, okGold, okRubric), "s")
		if err != nil {
			t.Fatal(err)
		}
		if got.ScenarioID == base.ScenarioID {
			t.Error("editing a prompt did not change the scenario hash")
		}
		if got.GoldID != base.GoldID || got.RubricID != base.RubricID {
			t.Error("editing the scenario changed a hash belonging to another file")
		}
	})

	t.Run("editing the rubric means a rejudge and nothing else", func(t *testing.T) {
		edited := strings.Replace(okRubric, "weight: 1.0", "weight: 0.5", 1)
		got, err := Load(set(t, okScenario, okGold, edited), "s")
		if err != nil {
			t.Fatal(err)
		}
		if got.RubricID == base.RubricID {
			t.Error("editing a criterion did not change the rubric hash")
		}
		if got.ScenarioID != base.ScenarioID || got.GoldID != base.GoldID {
			t.Error("editing the rubric changed a hash belonging to another file")
		}
	})
}

// A malformed file must fail the test suite, not a paid run forty minutes in.
func TestAMalformedSetIsRefusedAtLoadTime(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		scenario, gold, rubric string
		want                   string
	}{
		{
			name:     "a scenario with no repo",
			scenario: strings.Replace(okScenario, "repo: discourse\n", "", 1),
			gold:     okGold, rubric: okRubric,
			want: "names no repo",
		},
		{
			name:     "a step with an empty prompt",
			scenario: strings.Replace(okScenario, "prompt: Trace the resolution path.", `prompt: "  "`, 1),
			gold:     okGold, rubric: okRubric,
			want: `step 1 ("Map the contract") has an empty prompt`,
		},
		{
			// The headline number would otherwise be decided by a convention
			// two readers could read two ways, with nothing to say so.
			name:     "gold naming no discriminator",
			scenario: okScenario,
			gold:     strings.Replace(okGold, "discriminator: dependents\n", "", 1),
			rubric:   okRubric,
			want:     "no discriminator group named",
		},
		{
			name:     "a discriminator with no rows in it",
			scenario: okScenario,
			gold:     strings.Replace(okGold, "discriminator: dependents", "discriminator: retention", 1),
			rubric:   okRubric,
			want:     `the discriminator group "retention" has no rows`,
		},
		{
			name:     "a gold row with no id",
			scenario: okScenario,
			gold:     strings.Replace(okGold, "  - id: d:one\n", "  - \n", 1),
			rubric:   okRubric,
			want:     "a gold row has no id",
		},
		{
			name:     "one gold row twice",
			scenario: okScenario,
			gold:     okGold + "  - id: d:one\n    group: dependents\n    relation: \"a.rb:1 again\"\n",
			rubric:   okRubric,
			want:     `gold row "d:one" appears twice`,
		},
		{
			name:     "a gold row in no group",
			scenario: okScenario,
			gold:     strings.Replace(okGold, "    group: contract\n", "", 1),
			rubric:   okRubric,
			want:     `gold row "c:anchor" is in no group`,
		},
		{
			// A rubric grading a different number of steps is grading something
			// else, and the mismatch is invisible in a score.
			name:     "a rubric that grades the wrong number of steps",
			scenario: okScenario, gold: okGold,
			rubric: strings.Replace(okRubric, `  - name: Audit the dependents
    criteria:
      completeness:
        weight: 1.0
        question: Is every dependent pinned to a file:line?
`, "", 1),
			want: "the rubric grades 1 steps but the scenario asks 2",
		},
		{
			name:     "a rubric with no audience",
			scenario: okScenario, gold: okGold,
			rubric: strings.Replace(okRubric, "audience: An AI coding agent about to rework this class.\n", "", 1),
			want:   "names no audience",
		},
		{
			name:     "a rubric step with no criteria",
			scenario: okScenario, gold: okGold,
			rubric: strings.Replace(okRubric, `    criteria:
      completeness:
        weight: 1.0
        question: Is every dependent pinned to a file:line?
`, "", 1),
			want: "has no criteria",
		},
		{
			name:     "gold that is not YAML",
			scenario: okScenario, gold: "rows: [oops\n", rubric: okRubric,
			want: "parse",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := set(t, tc.scenario, tc.gold, tc.rubric)

			if got := loadErr(t, dir); !strings.Contains(got, tc.want) {
				t.Errorf("error = %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// Each of the three has to be there. A scenario with no gold cannot be scored,
// and a missing file is a different problem from a malformed one.
func TestEveryFileOfTheSetIsRequired(t *testing.T) {
	for _, missing := range []string{"s.yaml", "s.gold.yaml", "s.rubric.yaml"} {
		t.Run(missing, func(t *testing.T) {
			dir := set(t, okScenario, okGold, okRubric)
			if err := os.Remove(filepath.Join(dir, missing)); err != nil {
				t.Fatal(err)
			}

			if got := loadErr(t, dir); !strings.Contains(got, "read scenario file") {
				t.Errorf("error = %q, want it to name the read", got)
			}
		})
	}
}

// A caller with a path to the scenario gets its companions without having to
// know the naming rule.
func TestLoadPathFindsTheCompanionsFromTheScenarioPath(t *testing.T) {
	dir := set(t, okScenario, okGold, okRubric)

	got, err := LoadPath(filepath.Join(dir, "s.yaml"))
	if err != nil {
		t.Fatalf("LoadPath: %v", err)
	}
	if len(got.Gold.Rows) != 2 || len(got.Rubric.Steps) != 2 {
		t.Errorf("LoadPath did not find the companions: %+v", got)
	}
}

// Gold groups come back in file order, and a row nothing could ever match is an
// error rather than a permanent miss.
func TestAGoldGroupReturnsItsOwnRowsAndRefusesAnUnmatchableOne(t *testing.T) {
	s, err := Load(set(t, okScenario, okGold, okRubric), "s")
	if err != nil {
		t.Fatal(err)
	}

	rows, err := s.Gold.Group("dependents")
	if err != nil {
		t.Fatalf("Group: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != "d:one" {
		t.Errorf("group = %+v, want only the dependents row", rows)
	}

	vague := strings.Replace(okGold, `"lib/tasks/search.rake:32 the reindex task"`,
		`"somewhere in the search code"`, 1)
	s2, err := Load(set(t, okScenario, vague, okRubric), "s")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s2.Gold.Group("dependents"); err == nil {
		t.Error("a row nothing could match was returned as a row")
	}
}

// Well-formed YAML that is not this KIND of file fails differently from
// malformed YAML, and the messages have to say which: one sends you to a
// linter, the other to the schema.
func TestWellFormedYAMLOfTheWrongShapeIsRefused(t *testing.T) {
	dir := set(t, okScenario, "discriminator: dependents\nrows: a string, not rows\n", okRubric)

	got := loadErr(t, dir)

	if !strings.Contains(got, "read") || !strings.Contains(got, "s.gold.yaml") {
		t.Errorf("error = %q, want it to name the file it could not read as gold", got)
	}
}

// The remaining schema checks, each the smallest thing that would make a file
// unusable to its consumer.
func TestTheRestOfTheSchemaIsChecked(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		scenario, gold, rubric string
		want                   string
	}{
		{
			name:     "a scenario with no name",
			scenario: strings.Replace(okScenario, "name: audit the category contract\n", "", 1),
			gold:     okGold, rubric: okRubric,
			want: "the scenario has no name",
		},
		{
			name:     "a scenario with no steps",
			scenario: "name: audit\nrepo: discourse\n",
			gold:     okGold, rubric: okRubric,
			want: "no steps",
		},
		{
			name:     "a gold file with no rows",
			scenario: okScenario, gold: "discriminator: dependents\n", rubric: okRubric,
			want: "the gold file has no rows",
		},
		{
			name:     "a rubric with no steps",
			scenario: okScenario, gold: okGold,
			rubric: "audience: An agent.\n",
			want:   "the rubric has no steps",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := loadErr(t, set(t, tc.scenario, tc.gold, tc.rubric)); !strings.Contains(got, tc.want) {
				t.Errorf("error = %q, want %q", got, tc.want)
			}
		})
	}
}

// Cycle 00 scored one hardcoded group. The corpus carries five, and a scorer
// that can only see one of them cannot say whether a margin is broad or sits
// entirely in one place.
func TestTheGoldGroupsAreListedInTheOrderTheRowsNameThem(t *testing.T) {
	g := Gold{Rows: []GoldRow{
		{ID: "d:1", Group: "dependents"},
		{ID: "c:1", Group: "contract"},
		{ID: "d:2", Group: "dependents"}, // a repeat is not a second group
		{ID: "x:1", Group: ""},           // and a row with no group names none
		{ID: "w:1", Group: "write-path"},
	}}

	got := Gold.Groups(g)
	want := []string{"dependents", "contract", "write-path"}
	if len(got) != len(want) {
		t.Fatalf("Groups() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Groups()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
