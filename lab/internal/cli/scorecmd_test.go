package cli

import (
	"os"
	"os/exec"
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

// A run below the floor exits non-zero, so a driving script can branch on the
// verdict without parsing the report.
func TestScoreExitsNonZeroBelowTheFloor(t *testing.T) {
	run := recordedRun(t, "Only found app/models/category.rb:1083.")

	// 1 of 2 is exactly 0.50, which passes a 0.50 floor: the floor is
	// inclusive. Raise it so this run is genuinely below.
	code, stdout, _ := dispatch(t, "score",
		"-scenario", scoredFile(t), "-run", run, "-floor", "0.75")

	// Its own code, distinct from "could not run at all": a run that scored and
	// came up short is a result the loop can bank.
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

// Cycle 00 scored one hardcoded group. The corpus carries five — dependents,
// contract, write-path, guards and context — and a margin sitting entirely in
// one group is a different result from one spread across all of them. Reporting
// only the discriminator cannot tell those apart.
func TestScoringEveryGroupReportsEachOfThem(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083 and app/models/category.rb:1 too.")

	// A floor of 0.75 puts the two groups on OPPOSITE sides of it: dependents,
	// the discriminator, is 1 of 2, and contract is 1 of 1. So an exit code
	// taken from the best group, or from all of them together, would differ
	// from one taken from the discriminator.
	code, stdout, _ := dispatch(t, "score",
		"-scenario", scoredFile(t), "-run", run, "-group", "all", "-floor", "0.75")

	for _, want := range []string{"gold group dependents", "gold group contract"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the report is missing %q:\n%s", want, stdout)
		}
	}
	if code != exitBelowFloor {
		t.Errorf("exit code = %d, want the discriminator's below-floor code %d\n%s",
			code, exitBelowFloor, stdout)
	}
}

// And the other direction: a passing discriminator must not be dragged below
// the floor by a weaker group printed beside it.
func TestScoringEveryGroupTakesItsVerdictFromTheDiscriminatorAlone(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083 and lib/tasks/search.rake:32.")

	code, stdout, _ := dispatch(t, "score",
		"-scenario", scoredFile(t), "-run", run, "-group", "all")

	if code != exitOK {
		t.Errorf("exit code = %d, want %d: dependents is 2 of 2 even though contract is 0 of 1\n%s",
			code, exitOK, stdout)
	}
	if !strings.Contains(stdout, "gold group contract") {
		t.Errorf("the weaker group was not reported at all:\n%s", stdout)
	}
}

// A group whose gold cannot be scored stops the whole report rather than
// printing the groups that happened to come first. A partial table with a
// missing row reads as a complete one.
func TestScoringEveryGroupStopsOnGoldItCannotScore(t *testing.T) {
	set := scenarioSet(t, scoredScenario, `discriminator: dependents
rows:
  - id: d:one
    group: dependents
    relation: "app/models/category.rb:1083 the entry point"
  - id: c:vague
    group: contract
    relation: "somewhere in the search code"
`, scoredRubric)
	run := recordedRun(t, "Found app/models/category.rb:1083.")

	code, _, stderr := dispatch(t, "score", "-scenario", set, "-run", run, "-group", "all")

	if code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
	if !strings.Contains(stderr, "c:vague") {
		t.Errorf("the error does not name the unscoreable row:\n%s", stderr)
	}
}

// The provisional mark outranks the floor here exactly as it does for a single
// group: a truncated capture is a claim about the capture, not about the arm.
func TestScoringEveryGroupStaysProvisionalWhenTheCaptureIsTruncated(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "raw"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"assistant","message":{"content":[{"type":"text","text":"app/models/category.rb:1083"}]}}` + "\n" +
		`{"type":"assistant","message":{"content":[{"type":"text","text":"lib/tasks/sea`
	if err := os.WriteFile(filepath.Join(dir, "raw", "stdout"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := dispatch(t, "score",
		"-scenario", scoredFile(t), "-run", dir, "-group", "all")

	if code != exitProvisional {
		t.Errorf("exit code = %d, want %d\n%s", code, exitProvisional, stdout)
	}
}

// Scoring without a checkout is a valid result that says grounding was not
// verified. A scorer that refused to score without one would make the whole
// pure layer depend on the state of a disk — and three of the four benched
// repositories are not cloned on this machine today.
func TestScoringWithoutACheckoutIsValidAndSaysSo(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083 and lib/tasks/search.rake:32.")

	code, stdout, _ := dispatch(t, "score", "-scenario", scoredFile(t), "-run", run)

	if code != exitOK {
		t.Errorf("exit code = %d, want %d: a missing checkout is not a failure", code, exitOK)
	}
	if !strings.Contains(stdout, "NOT VERIFIED") {
		t.Errorf("the report does not say its citations were unverified:\n%s", stdout)
	}
}

// A checkout that cannot be opened is worth a word on stderr, because the
// operator asked for verification and did not get it — but it still scores.
func TestAnUnusableCheckoutSaysSoAndStillScores(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083 and lib/tasks/search.rake:32.")

	code, stdout, stderr := dispatch(t, "score", "-scenario", scoredFile(t), "-run", run,
		"-checkout", t.TempDir(), "-commit", "deadbeef")

	if code != exitOK {
		t.Errorf("exit code = %d, want %d", code, exitOK)
	}
	if !strings.Contains(stderr, "grounding skipped") {
		t.Errorf("nothing on stderr said grounding was skipped:\n%s", stderr)
	}
	if !strings.Contains(stdout, "NOT VERIFIED") {
		t.Errorf("the report claimed more than it checked:\n%s", stdout)
	}
}

// The whole grounding path end to end, against a real checkout.
//
// It asserts the REPORT, not the score, because grounding does not change a
// score: it returns a report and nothing else. An earlier version of this test
// was named for the score and claimed "the number is smaller for it", which was
// never true — its fabricated citation matched no gold row, so pruning it could
// not have moved anything. A test whose name promises more than it checks is
// worse than no test, because it is where you look and stop looking.
func TestAFabricatedCitationIsReportedAgainstARealCheckout(t *testing.T) {
	dir, commit := repoWithGold(t)
	// The answer cites the gold line, and also a line that file does not have.
	run := recordedRun(t, "Found app/models/category.rb:1083 and app/models/category.rb:999999.")

	code, stdout, stderr := dispatch(t, "score", "-scenario", scoredFile(t), "-run", run,
		"-checkout", dir, "-commit", commit)

	if !strings.Contains(stdout, "1 of 2 cited locations do not resolve") {
		t.Errorf("the fabricated citation was not reported:\n%s\n%s", stdout, stderr)
	}
	if strings.Contains(stdout, "NOT VERIFIED") {
		t.Errorf("a run WITH a usable checkout reported as unverified:\n%s\n%s", stdout, stderr)
	}
	if code != exitBelowFloor && code != exitOK {
		t.Errorf("exit code = %d", code)
	}
}

// A checkout that cannot resolve the gold is the WRONG checkout, and that is
// the dangerous case: a commit that exists in another repository passes every
// existence check, resolves nothing, and would report as verified.
func TestACheckoutThatCannotResolveTheGoldIsRefused(t *testing.T) {
	dir, commit := repoMissingGold(t)
	run := recordedRun(t, "Found app/models/category.rb:1083.")

	// The scored scenario's gold also names lib/tasks/search.rake, which this
	// repository does not have.
	_, stdout, stderr := dispatch(t, "score", "-scenario", scoredFile(t), "-run", run,
		"-checkout", dir, "-commit", commit)

	if !strings.Contains(stderr, "wrong checkout or the scenario is stale") {
		t.Errorf("a checkout that cannot resolve the gold was accepted:\n%s", stderr)
	}
	if !strings.Contains(stdout, "NOT VERIFIED") {
		t.Errorf("the report claimed verification it did not have:\n%s", stdout)
	}
}

// -checkout without -commit is a typo, not a deferral. Downgrading it to
// unverified would exit 0 on a stderr line an operator may be piping away.
func TestACheckoutWithoutACommitIsAUsageError(t *testing.T) {
	run := recordedRun(t, "Found app/models/category.rb:1083.")

	code, _, stderr := dispatch(t, "score", "-scenario", scoredFile(t), "-run", run,
		"-checkout", t.TempDir())

	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "-checkout needs -commit") {
		t.Errorf("the error does not say what is missing:\n%s", stderr)
	}
}

// repoWithGold builds a checkout holding every file the scored fixture's gold
// names, long enough to contain each gold line and no longer.
func repoWithGold(t *testing.T) (dir, commit string) {
	t.Helper()
	return buildRepo(t, map[string]string{
		"app/models/category.rb": strings.Repeat("line\n", 1100),
		"lib/tasks/search.rake":  strings.Repeat("line\n", 40),
	})
}

// repoMissingGold holds one of the two gold files, so grounding can tell that
// this is not the repository the scenario was written against.
func repoMissingGold(t *testing.T) (dir, commit string) {
	t.Helper()
	return buildRepo(t, map[string]string{
		"app/models/category.rb": strings.Repeat("line\n", 1100),
	})
}

func buildRepo(t *testing.T, files map[string]string) (dir, commit string) {
	t.Helper()
	dir = t.TempDir()
	for path, body := range files {
		full := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"}, {"config", "user.email", "t@example.com"}, {"config", "user.name", "t"},
		{"add", "-A"}, {"commit", "-qm", "fixture"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Skipf("git is unavailable here: %v: %s", err, out)
		}
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return dir, strings.TrimSpace(string(out))
}
