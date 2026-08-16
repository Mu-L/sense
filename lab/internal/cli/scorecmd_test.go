package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const scoredScenario = `
name: audit the category contract
repo: discourse
steps:
  - name: Map the contract
    prompt: Trace the resolution path.
gold:
  - id: d:one
    group: dependents
    relation: "app/models/category.rb:1083 the entry point every lookup bottoms out in"
  - id: d:two
    group: dependents
    relation: "lib/tasks/search.rake:32 the reindex task"
  - id: c:anchor
    group: contract
    relation: "app/models/category.rb:1 the class itself"
`

const uncitableScenario = `
name: vague
repo: discourse
steps:
  - name: Only
    prompt: Find it.
gold:
  - id: d:vague
    group: dependents
    relation: "somewhere in the search code"
`

// recordedRun writes a run directory the way `sense-lab run` leaves one, with
// the given assistant text as the captured stream.
func recordedRun(t *testing.T, said string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"assistant","message":{"content":[{"type":"text","text":"` + said + `"}]}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "raw", "stdout"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestScoreReportsTheNumberAndPassesAtTheFloor(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083 and lib/tasks/search.rake:32.")

	code, stdout, stderr := dispatch(t, "score",
		"-scenario", scenarioFile(t, scoredScenario), "-run", run)

	if code != 0 {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{"discourse", "cited      2 of 2", "recall     1.00", "at or above floor"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output is missing %q:\n%s", want, stdout)
		}
	}
}

// A run below the floor exits non-zero, so a campaign script can branch on the
// verdict without parsing the report.
func TestScoreExitsNonZeroBelowTheFloor(t *testing.T) {
	run := recordedRun(t, "Only found app/models/category.rb:1083.")

	// 1 of 2 is exactly 0.50, which passes a 0.50 floor: the floor is
	// inclusive. Raise it so this run is genuinely below.
	code, stdout, _ := dispatch(t, "score",
		"-scenario", scenarioFile(t, scoredScenario), "-run", run, "-floor", "0.75")

	// Its own code, distinct from "could not run at all": a run that scored and
	// came up short is a result a campaign can bank.
	if code != 4 {
		t.Errorf("exit = %d, want 4 for a run below the floor", code)
	}
	if !strings.Contains(stdout, "below floor") {
		t.Errorf("output does not report the verdict:\n%s", stdout)
	}
	if !strings.Contains(stdout, "d:two") {
		t.Errorf("output does not name what was missed:\n%s", stdout)
	}
}

// Only the named group is scored. Counting the contract rows into the
// dependents number would inflate every score by the anchors both arms reach.
func TestScoreCountsOnlyTheNamedGroup(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083 and app/models/category.rb:1 too.")

	_, stdout, _ := dispatch(t, "score",
		"-scenario", scenarioFile(t, scoredScenario), "-run", run)

	if !strings.Contains(stdout, "cited      1 of 2") {
		t.Errorf("the dependents group should have 2 rows and 1 hit:\n%s", stdout)
	}
}

func TestScoreRejectsWhatItCannotScore(t *testing.T) {
	good := scenarioFile(t, scoredScenario)
	run := recordedRun(t, "nothing")

	for _, tc := range []struct {
		name     string
		args     []string
		want     string
		wantCode int
	}{
		{
			name: "no scenario", wantCode: 2,
			args: []string{"-run", run}, want: "-scenario is required",
		},
		{
			name: "no run", wantCode: 2,
			args: []string{"-scenario", good}, want: "-run is required",
		},
		{
			name: "an unknown flag", wantCode: 2,
			args: []string{"-scenario", good, "-run", run, "-arm", "sense"},
			want: "flag provided but not defined",
		},
		{
			name: "a scenario that does not exist", wantCode: 1,
			args: []string{"-scenario", "/no/such.yaml", "-run", run}, want: "read scenario",
		},
		{
			// A typo'd group name would otherwise score 0 of 0 and read as a
			// legitimately empty result rather than a mistake.
			name: "a gold group the scenario does not have", wantCode: 1,
			args: []string{"-scenario", good, "-run", run, "-group", "dependants"},
			want: `no gold group "dependants"`,
		},
		{
			name: "a run directory with no transcript", wantCode: 1,
			args: []string{"-scenario", good, "-run", t.TempDir()}, want: "read transcript",
		},
		{
			// A gold row nothing could ever match would otherwise be a
			// permanent miss, deflating the score in the direction that makes
			// a real arm look weak.
			name: "a gold row with no location", wantCode: 1,
			args: []string{"-scenario", scenarioFile(t, uncitableScenario), "-run", run},
			want: "nothing could ever match it",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := dispatch(t, append([]string{"score"}, tc.args...)...)

			if code != tc.wantCode {
				t.Errorf("exit = %d, want %d", code, tc.wantCode)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want it to name %q", stderr, tc.want)
			}
		})
	}
}

func TestScoreHelpIsNotReportedAsAnError(t *testing.T) {
	_, _, stderr := dispatch(t, "score", "-help")

	if strings.Contains(stderr, "sense-lab score:") {
		t.Errorf("asking for help produced an error message:\n%s", stderr)
	}
}
