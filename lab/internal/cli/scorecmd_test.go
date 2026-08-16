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
`

// The gold is its own file now, which is the whole point of the split: a
// corrected row costs a rescore rather than a re-run.
const scoredGold = `discriminator: dependents
rows:
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

const scoredRubric = `audience: An AI coding agent about to rework this class.
steps:
  - name: Map the contract
    criteria:
      quality:
        weight: 1.0
        question: Is the path traced?
`

// scoredFile writes the scored scenario as a three-file set.
func scoredFile(t *testing.T) string {
	t.Helper()
	return scenarioSet(t, scoredScenario, scoredGold, scoredRubric)
}

// A gold row nothing could ever match: it becomes a permanent miss that looks
// exactly like an arm failing to find the place.
func uncitableFile(t *testing.T) string {
	t.Helper()
	return scenarioSet(t, scoredScenario, `discriminator: dependents
rows:
  - id: d:vague
    group: dependents
    relation: "somewhere in the search code"
`, scoredRubric)
}

// recordedRun writes a run directory the way `sense-lab run` leaves one, with
// the given assistant text as the captured stream.
func recordedRun(t *testing.T, said string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A FINISHED session: the closing result event is there, because its
	// absence is itself a provisional condition and these tests are about
	// scoring rather than about completeness.
	body := `{"type":"assistant","message":{"content":[{"type":"text","text":"` + said + `"}]}}` + "\n" +
		`{"type":"result","session_id":"s"}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "raw", "stdout"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestScoreReportsTheNumberAndPassesAtTheFloor(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083 and lib/tasks/search.rake:32.")

	code, stdout, stderr := dispatch(t, "score",
		"-scenario", scoredFile(t), "-run", run)

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
		"-scenario", scoredFile(t), "-run", run, "-floor", "0.75")

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
		"-scenario", scoredFile(t), "-run", run)

	if !strings.Contains(stdout, "cited      1 of 2") {
		t.Errorf("the dependents group should have 2 rows and 1 hit:\n%s", stdout)
	}
}

// The mark travels. A truncated capture scores low, and reporting that as a
// clean failure would be the exact misreading the mark exists to prevent: it is
// a claim about the agent when the truth is a claim about the capture.
func TestATruncatedCaptureScoresProvisionalNotFailed(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A complete event, then one cut mid-line the way a killed capture is.
	body := `{"type":"assistant","message":{"content":[{"type":"text","text":"app/models/category.rb:1083"}]}}` + "\n" +
		`{"type":"assistant","message":{"content":[{"type":"text","text":"lib/tasks/sea`
	if err := os.WriteFile(filepath.Join(dir, "raw", "stdout"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	// A floor the run is genuinely BELOW, so the provisional mark and the
	// below-floor verdict both apply and the precedence is under test. At the
	// default floor this run passes, and the two orderings are
	// indistinguishable — which is how the ordering went untested.
	code, stdout, _ := dispatch(t, "score",
		"-scenario", scoredFile(t), "-run", dir, "-floor", "0.75")

	// The mark wins. Reported as a clean below-floor failure it is a claim
	// about the agent, when the truth is a claim about the capture — which is
	// the sentence this whole pitch is built on.
	if code != 6 {
		t.Errorf("exit = %d, want 6; a truncated capture must not report as a clean failure", code)
	}
	if !strings.Contains(stdout, "below floor") {
		t.Errorf("the verdict is still shown:\n%s", stdout)
	}
	if !strings.Contains(stdout, "PROVISIONAL") {
		t.Errorf("the report does not carry the mark:\n%s", stdout)
	}
	if !strings.Contains(stdout, "this number is not a result") {
		t.Errorf("the report does not say what the mark means:\n%s", stdout)
	}
	if !strings.Contains(stdout, "1 unreadable lines") {
		t.Errorf("the report does not say what is missing:\n%s", stdout)
	}
	// The number is still shown — a provisional transcript is worth reading,
	// it is just not worth banking.
	if !strings.Contains(stdout, "cited      1 of 2") {
		t.Errorf("the readable half was not scored:\n%s", stdout)
	}
}

// A complete capture carries no mark, so the mark means something.
func TestACompleteCaptureCarriesNoProvisionalMark(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083 and lib/tasks/search.rake:32.")

	code, stdout, _ := dispatch(t, "score",
		"-scenario", scoredFile(t), "-run", run)

	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if strings.Contains(stdout, "PROVISIONAL") {
		t.Errorf("a complete capture was marked provisional:\n%s", stdout)
	}
}

// The strongest signal is not in the capture at all. The runner knows whether
// it killed the session and writes it beside the transcript; a capture can look
// perfect and still belong to a run that hit its wall.
func TestTheRunnersOwnRecordMakesAScoreProvisional(t *testing.T) {
	for _, tc := range []struct {
		name    string
		outcome string
		want    string
		code    int
	}{
		{"a run that hit its wall", "cannot finish at budget",
			"the runner recorded this session as cannot finish at budget", 6},
		{"a run that was interrupted", "interrupted",
			"the runner recorded this session as interrupted", 6},
		{"a run that finished", "completed", "", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// A capture that looks entirely healthy either way.
			run := recordedRun(t, "Found app/models/category.rb:1083 and lib/tasks/search.rake:32.")
			if err := os.WriteFile(filepath.Join(run, "run-meta.json"),
				[]byte(`{"outcome":"`+tc.outcome+`","exit_code":0}`), 0o644); err != nil {
				t.Fatal(err)
			}

			code, stdout, _ := dispatch(t, "score",
				"-scenario", scoredFile(t), "-run", run)

			if code != tc.code {
				t.Errorf("exit = %d, want %d", code, tc.code)
			}
			if tc.want == "" {
				if strings.Contains(stdout, "PROVISIONAL") {
					t.Errorf("a completed run was marked provisional:\n%s", stdout)
				}
				return
			}
			if !strings.Contains(stdout, tc.want) {
				t.Errorf("the report does not carry the runner's account:\n%s", stdout)
			}
		})
	}
}

// A run directory with no record is the cycle-00 shape and is not itself
// evidence of anything, so the transcript decides.
func TestNoRunRecordIsNotEvidenceOfAnything(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083 and lib/tasks/search.rake:32.")

	code, stdout, _ := dispatch(t, "score",
		"-scenario", scoredFile(t), "-run", run)

	if code != 0 || strings.Contains(stdout, "PROVISIONAL") {
		t.Errorf("exit = %d, output:\n%s", code, stdout)
	}
}

// A record that cannot be read is a different answer from no record: something
// wrote it and it is now unreadable, which is worth knowing.
func TestAnUnreadableRunRecordIsItselfProvisional(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083.")
	if err := os.WriteFile(filepath.Join(run, "run-meta.json"), []byte("{oops"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := dispatch(t, "score",
		"-scenario", scoredFile(t), "-run", run)

	if code != 6 {
		t.Errorf("exit = %d, want 6", code)
	}
	if !strings.Contains(stdout, "run record beside this capture could not be read") {
		t.Errorf("the report does not say the record is unreadable:\n%s", stdout)
	}
}

func TestScoreRejectsWhatItCannotScore(t *testing.T) {
	good := scoredFile(t)
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
			args: []string{"-scenario", uncitableFile(t), "-run", run},
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

// The discriminator decides the headline number, so it has to decide which
// group is scored. It was declared, validated and then ignored: -group
// defaulted to a hardcoded "dependents", so a gold file naming a different
// discriminator was silently scored on the wrong rows — the exact failure the
// field was added to end, with a YAML key in front of it.
func TestTheScoredGroupComesFromTheGoldFilesOwnDiscriminator(t *testing.T) {
	// Same rows, but the gold declares CONTRACT as the discriminator. The
	// contract group has one row and the answer cites it, so scoring the
	// declared group gives 1 of 1 and scoring the hardcoded one gives 1 of 2.
	set := scenarioSet(t, scoredScenario,
		strings.Replace(scoredGold, "discriminator: dependents", "discriminator: contract", 1),
		scoredRubric)
	run := recordedRun(t, "Found app/models/category.rb:1083 and app/models/category.rb:1 too.")

	_, stdout, _ := dispatch(t, "score", "-scenario", set, "-run", run)

	if !strings.Contains(stdout, "cited      1 of 1") {
		t.Errorf("the declared discriminator was not the group scored:\n%s", stdout)
	}
	if !strings.Contains(stdout, "contract") {
		t.Errorf("the report does not name the group it scored:\n%s", stdout)
	}
}

// And the flag still wins, because a rescore of one group is how a gold
// correction is checked without touching the file.
func TestNamingAGroupExplicitlyOverridesTheDiscriminator(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1 only.")

	_, stdout, _ := dispatch(t, "score",
		"-scenario", scoredFile(t), "-run", run, "-group", "contract")

	if !strings.Contains(stdout, "cited      1 of 1") {
		t.Errorf("-group did not override the discriminator:\n%s", stdout)
	}
}
