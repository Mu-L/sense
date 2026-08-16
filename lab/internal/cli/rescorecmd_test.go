package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recordedTree builds a results tree the way the old bench left one: a run
// directory holding the capture it produced and the score it was given.
func recordedTree(t *testing.T, scored, answer string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "results", "opus", "hash", "sense", "discourse", "run-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"assistant","message":{"content":[{"type":"text","text":` + quote(answer) + `}]}}` + "\n" +
		`{"type":"result","session_id":"s"}` + "\n"
	for name, content := range map[string]string{
		"scored.json":     scored,
		"transcript.json": body,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

// scenarioDir writes one scenario set into a directory laid out the way the
// rescore command expects to find them.
func scenarioDir(t *testing.T, name, scenario, gold, rubric string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for suffix, content := range map[string]string{
		".yaml":        scenario,
		".gold.yaml":   gold,
		".rubric.yaml": rubric,
	} {
		if err := os.WriteFile(filepath.Join(dir, name+suffix), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

const discourseGold = `discriminator: dependents
rows:
  - id: d:one
    group: dependents
    relation: "app/models/category.rb:1083 the entry point"
  - id: d:two
    group: dependents
    relation: "lib/tasks/search.rake:32 the reindex task"
`

// The old scorer credited both rows. The answer names the first file at the
// wrong line and the second at the right one, so the new scorer credits one.
const recordedBoth = `{"gold_recall":{"groups":{"dependents":{"total":2,"cited":2}},
"details":[{"id":"d:one","cited":true},{"id":"d:two","cited":true}]}}`

func TestRescoreNamesTheCauseOfEveryDifference(t *testing.T) {
	results := recordedTree(t, recordedBoth,
		"Found app/models/category.rb:40 and lib/tasks/search.rake:32.")
	scenarios := scenarioDir(t, "discourse", scoredScenario, discourseGold, scoredRubric)

	code, stdout, stderr := dispatch(t, "rescore", "-results", results, "-scenarios", scenarios)

	if !strings.Contains(stdout, "the right file at a line gold does not name") {
		t.Errorf("the cause was not named:\n%s\n%s", stdout, stderr)
	}
	if strings.Contains(stdout, "UNEXPLAINED") {
		t.Errorf("a difference went unexplained:\n%s", stdout)
	}
	if code != exitOK {
		t.Errorf("exit code = %d, want %d when everything is accounted for", code, exitOK)
	}
}

// Zero unexplained is what closes the pitch, so a difference nothing accounts
// for has to fail the command rather than print quietly.
func TestRescoreFailsWhileAnythingIsUnexplained(t *testing.T) {
	// The old scorer credited NEITHER row, and the answer cites one exactly, so
	// the new score is HIGHER with no pre-fix list to explain it.
	recorded := `{"gold_recall":{"groups":{"dependents":{"total":2,"cited":0}},"details":[]}}`
	results := recordedTree(t, recorded, "Found app/models/category.rb:1083.")
	scenarios := scenarioDir(t, "discourse", scoredScenario, discourseGold, scoredRubric)

	code, stdout, _ := dispatch(t, "rescore", "-results", results, "-scenarios", scenarios)

	if !strings.Contains(stdout, "UNEXPLAINED. The cycle stays open") {
		t.Errorf("an unexplained difference was not surfaced:\n%s", stdout)
	}
	if code == exitOK {
		t.Errorf("exit code = %d; an unexplained difference must not pass", code)
	}
}

// And the same difference, with the pre-fix list saying this run predates the
// symbol-oracle fix. Now it has a cause, and the command passes.
func TestRescoreReadsThePreFixListWrittenBeforeTheComparison(t *testing.T) {
	recorded := `{"gold_recall":{"groups":{"dependents":{"total":2,"cited":0}},"details":[]}}`
	results := recordedTree(t, recorded, "Found app/models/category.rb:1083.")
	scenarios := scenarioDir(t, "discourse", scoredScenario, discourseGold, scoredRubric)

	list := filepath.Join(t.TempDir(), "pre-fix.md")
	if err := os.WriteFile(list, []byte(
		"| when | repo | model | arm | run | dated by |\n"+
			"|---|---|---|---|---|---|\n"+
			"| 2026-08-01 | discourse | opus | sense | run-1 | run_meta |\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := dispatch(t, "rescore",
		"-results", results, "-scenarios", scenarios, "-pre-fix", list)

	if !strings.Contains(stdout, "scorer version") {
		t.Errorf("the pre-fix list was not read:\n%s", stdout)
	}
	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
}

// A group whose gold this cycle quarantined answers a different question now.
// It is a named cause, not a row quietly dropped from the comparison.
func TestRescoreCallsAQuarantinedGroupAGoldChange(t *testing.T) {
	unciteable := `discriminator: dependents
rows:
  - id: d:vague
    group: dependents
    relation: "somewhere in the search code"
`
	results := recordedTree(t, recordedBoth, "Found app/models/category.rb:1083.")
	scenarios := scenarioDir(t, "discourse", scoredScenario, unciteable, scoredRubric)

	_, stdout, _ := dispatch(t, "rescore", "-results", results, "-scenarios", scenarios)

	if !strings.Contains(stdout, "gold change") {
		t.Errorf("a quarantined group was not called a gold change:\n%s", stdout)
	}
}

// A recorded run with no readable capture cannot be compared, and saying so is
// the difference between "they all agreed" and "I could not look".
func TestRescoreSaysHowManyRunsItCouldNotCompare(t *testing.T) {
	results := recordedTree(t, recordedBoth, "Found app/models/category.rb:1083.")
	scenarios := scenarioDir(t, "discourse", scoredScenario, discourseGold, scoredRubric)
	// A second run whose capture is empty.
	dir := filepath.Join(results, "results", "opus", "hash", "baseline", "discourse", "run-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{"scored.json": recordedBoth, "transcript.json": ""} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	_, stdout, _ := dispatch(t, "rescore", "-results", results, "-scenarios", scenarios)

	if !strings.Contains(stdout, "1 recorded runs were not comparable") {
		t.Errorf("the uncomparable run was not reported:\n%s", stdout)
	}
}

func TestRescoreRefusesWithoutTheThingsItNeeds(t *testing.T) {
	scenarios := scenarioDir(t, "discourse", scoredScenario, discourseGold, scoredRubric)

	t.Run("no results tree", func(t *testing.T) {
		code, _, stderr := dispatch(t, "rescore")
		if code != exitUsage || !strings.Contains(stderr, "-results is required") {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("a flag it does not have", func(t *testing.T) {
		if code, _, _ := dispatch(t, "rescore", "-nope"); code != exitUsage {
			t.Errorf("code = %d, want %d", code, exitUsage)
		}
	})
	t.Run("no scenarios to score against", func(t *testing.T) {
		code, _, stderr := dispatch(t, "rescore", "-results", t.TempDir(), "-scenarios", t.TempDir())
		if code != exitError || !strings.Contains(stderr, "no scenarios") {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("nothing recorded to compare", func(t *testing.T) {
		code, _, stderr := dispatch(t, "rescore", "-results", t.TempDir(), "-scenarios", scenarios)
		if code != exitError || !strings.Contains(stderr, "nothing to compare") {
			t.Errorf("code = %d, stderr = %q", code, stderr)
		}
	})
	t.Run("a pre-fix list that is not there", func(t *testing.T) {
		results := recordedTree(t, recordedBoth, "Found app/models/category.rb:40.")
		// A missing list is not an error: it means nothing is known to predate
		// the fix, which is the safe reading, and the run still gets accounted.
		if code, _, _ := dispatch(t, "rescore", "-results", results,
			"-scenarios", scenarios, "-pre-fix", filepath.Join(t.TempDir(), "nope.md")); code != exitOK {
			t.Errorf("code = %d, want %d", code, exitOK)
		}
	})
}

// The paths and files this walks are produced by another program, so every
// shape it can hand over has to be handled rather than assumed away.
func TestRescoreSkipsWhatIsNotARecordedRun(t *testing.T) {
	results := recordedTree(t, recordedBoth, "Found app/models/category.rb:40.")
	scenarios := scenarioDir(t, "discourse", scoredScenario, discourseGold, scoredRubric)

	base := filepath.Join(results, "results", "opus", "hash")
	for _, tc := range []struct{ dir, scored, transcript string }{
		// Not an arm we know, so not a run.
		{filepath.Join(base, "sideways", "discourse", "run-1"), recordedBoth, "{}"},
		// An arm and a repo we do not ship a scenario for.
		{filepath.Join(base, "sense", "unknown-repo", "run-1"), recordedBoth, "{}"},
		// A parked run, which the leading underscore marks.
		{filepath.Join(base, "sense", "_pre-harness-fix-discourse", "run-1"), recordedBoth, "{}"},
		// A score file that is not JSON.
		{filepath.Join(base, "sense", "discourse", "run-2"), "not json at all", "{}"},
		// A score file with no gold recall in it.
		{filepath.Join(base, "sense", "discourse", "run-3"), `{"efficiency":0.5}`, "{}"},
	} {
		if err := os.MkdirAll(tc.dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, body := range map[string]string{"scored.json": tc.scored, "transcript.json": tc.transcript} {
			if err := os.WriteFile(filepath.Join(tc.dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	// A scored.json somewhere with no results segment above it at all.
	stray := filepath.Join(results, "stray")
	if err := os.MkdirAll(stray, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stray, "scored.json"), []byte(recordedBoth), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := dispatch(t, "rescore", "-results", results, "-scenarios", scenarios)

	// The one real run is still compared, and nothing above turned into a row.
	if !strings.Contains(stdout, "group-scores compared 1") {
		t.Errorf("the walk picked up something that is not a recorded run:\n%s", stdout)
	}
	if code != exitOK {
		t.Errorf("exit code = %d", code)
	}
}

// A scenario directory holding something that is not a scenario is skipped
// rather than failing the whole rescore.
func TestRescoreSkipsWhatIsNotAScenario(t *testing.T) {
	scenarios := scenarioDir(t, "discourse", scoredScenario, discourseGold, scoredRubric)
	if err := os.WriteFile(filepath.Join(scenarios, "loose-file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(scenarios, "not-a-scenario"), 0o755); err != nil {
		t.Fatal(err)
	}
	results := recordedTree(t, recordedBoth, "Found app/models/category.rb:40.")

	if code, _, stderr := dispatch(t, "rescore",
		"-results", results, "-scenarios", scenarios); code != exitOK {
		t.Errorf("exit code = %d: %s", code, stderr)
	}
}

// A malformed pre-fix list must not be read as naming runs. A row it half-parses
// would attach `scorer version` to a run nobody wrote down, which is exactly the
// story-fitting the written list exists to prevent.
func TestRescoreIgnoresRowsThatAreNotRunsInThePreFixList(t *testing.T) {
	results := recordedTree(t, recordedBoth, "Found app/models/category.rb:40.")
	scenarios := scenarioDir(t, "discourse", scoredScenario, discourseGold, scoredRubric)

	list := filepath.Join(t.TempDir(), "pre-fix.md")
	if err := os.WriteFile(list, []byte(
		"Prose above the table, which is not a row.\n"+
			"| when | repo | model | arm | run | dated by |\n"+
			"|---|---|---|---|---|---|\n"+
			"| short | row |\n"+
			"|  |  |  |  |  |  |\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, stdout, _ := dispatch(t, "rescore",
		"-results", results, "-scenarios", scenarios, "-pre-fix", list)

	if strings.Contains(stdout, "scorer version") {
		t.Errorf("a half-parsed row named a pre-fix run:\n%s", stdout)
	}
}

// A scenarios path that is not a directory at all leaves nothing to score
// against, and that is a refusal rather than an empty comparison reading clean.
func TestRescoreRefusesAScenariosPathThatIsNotADirectory(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	results := recordedTree(t, recordedBoth, "Found app/models/category.rb:40.")

	code, _, stderr := dispatch(t, "rescore", "-results", results, "-scenarios", notADir)

	if code != exitError || !strings.Contains(stderr, "no scenarios") {
		t.Errorf("code = %d, stderr = %q", code, stderr)
	}
}

// A recorded score that cannot be read is not comparable, and must be counted
// as such rather than crashing the walk or passing as agreement.
func TestRescoreCountsAScoreItCannotRead(t *testing.T) {
	results := recordedTree(t, recordedBoth, "Found app/models/category.rb:40.")
	scenarios := scenarioDir(t, "discourse", scoredScenario, discourseGold, scoredRubric)

	dir := filepath.Join(results, "results", "opus", "hash", "baseline", "discourse", "run-1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	unreadable := filepath.Join(dir, "scored.json")
	if err := os.WriteFile(unreadable, []byte(recordedBoth), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := os.ReadFile(unreadable); err == nil {
		t.Skip("this user can read a mode-000 file, so the case is unreachable here")
	}

	code, stdout, _ := dispatch(t, "rescore", "-results", results, "-scenarios", scenarios)

	if !strings.Contains(stdout, "1 recorded runs were not comparable") {
		t.Errorf("the unreadable score was not counted:\n%s", stdout)
	}
	if code != exitOK {
		t.Errorf("exit code = %d", code)
	}
}
