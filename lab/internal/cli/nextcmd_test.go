package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/position"
	"github.com/luuuc/sense/lab/internal/stage"
)

// nextWorld is a repository that has been admitted, with a bench declaring what
// its phases are run by, so `next` needs nothing else typed.
func newNextWorld(t *testing.T) crankWorld {
	t.Helper()
	w := newCrankWorld(t)
	declareDriver(t, w, `"driver":{"agent":"phase","model":"m1"},`)
	return w
}

func declareDriver(t *testing.T, w crankWorld, driver string) {
	t.Helper()
	artifact(t, filepath.Join(w.config, "benches", w.repo+".json"),
		`{"repo":"`+w.repo+`","judge":"m1",`+driver+
			`"subjects":["untreated","sense-main"],"arms":[{"role":"headline","model":"m1","runs":2}]}`)
}

func runNext(t *testing.T, w crankWorld, in string, extra ...string) (int, string, string) {
	t.Helper()
	args := append([]string{"-config", w.config, "-runs", w.runs, "-checkouts", w.checkouts,
		"-sense", w.sense}, extra...)
	var stdout, stderr bytes.Buffer
	code := doNext(context.Background(), append(args, w.repo), strings.NewReader(in), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

// The whole flow in one command: it walks the stages it can walk, says what
// each one did, and stops where the money starts — with the command that
// spends it.
func TestNextWalksTheStagesAndStopsWhereTheMoneyStarts(t *testing.T) {
	w := newNextWorld(t)

	code, stdout, stderr := runNext(t, w, "", "-yes")
	if code != exitWaiting {
		t.Fatalf("exit %d, want the waiting-on-a-human code\n%s\n%s", code, stdout, stderr)
	}

	for _, want := range []string{
		"1. Question", "Question written.",
		"2. Trial", "Sense won the trial.",
		"3. Rehearsal", "Grown into a full session.", "Everything can run.",
		"The rehearsal says it is worth paying for.",
		"sense-lab pay " + w.repo,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the page does not say %q:\n%s", want, stdout)
		}
	}
}

// A stage says it has started BEFORE it runs. A forty-minute phase that printed
// one line on completion reads as a hang for forty minutes.
func TestAStageSaysItHasStartedBeforeItRuns(t *testing.T) {
	w := newNextWorld(t)

	_, stdout, _ := runNext(t, w, "", "-yes")

	started := strings.Index(stdout, "─── 1. Question")
	done := strings.Index(stdout, "Question written.")
	switch {
	case started < 0:
		t.Fatalf("no stage line was printed:\n%s", stdout)
	case done < 0:
		t.Fatalf("no outcome line was printed:\n%s", stdout)
	case started > done:
		t.Errorf("the stage announced itself after it had finished:\n%s", stdout)
	}
	if !strings.Contains(stdout, "up to ") {
		t.Errorf("the stage line does not say how long it may take:\n%s", stdout)
	}
}

// The stop has four parts, because a stop that named only the next command
// would leave the reader to work out whether what just happened was good.
func TestTheStopSaysWhatHappenedWhatIsNextWhyAndTheCommand(t *testing.T) {
	w := newNextWorld(t)

	_, stdout, _ := runNext(t, w, "", "-yes")

	for _, want := range []string{
		"The rehearsal says the question is worth paying for.",
		"Next: run the paid cells",
		"Why:  spending is yours, not the instrument's",
		"sense-lab pay " + w.repo,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the stop does not say %q:\n%s", want, stdout)
		}
	}
}

// No phase name and no verdict token reaches the page. They are the right words
// inside the graph and the wrong ones on a terminal.
func TestThePageSpeaksNoMechanism(t *testing.T) {
	w := newNextWorld(t)

	_, stdout, _ := runNext(t, w, "", "-yes")

	for _, name := range []phase.Name{phase.Minibench, phase.Expand, phase.Preflight, phase.Validate} {
		if strings.Contains(stdout, string(name)) {
			t.Errorf("the page names the phase %q:\n%s", name, stdout)
		}
	}
	for _, v := range []phase.Verdict{phase.Draft, phase.Proceed, phase.Pay, phase.Auto} {
		if strings.Contains(stdout, string(v)) {
			t.Errorf("the page prints the verdict %q:\n%s", v, stdout)
		}
	}
}

// The ordinary invocation names a repository and nothing else. What the phases
// are run by is declared once, in the bench, where what this repository is
// measured on already lives.
func TestTheDriverComesFromTheBenchRatherThanTheCommandLine(t *testing.T) {
	w := newNextWorld(t)

	got, err := driver(nextFlags{config: w.config, runs: w.runs}, catalogOf(t, w.config), w.repo)
	if err != nil {
		t.Fatal(err)
	}
	if got.agent != "phase" || got.model != "m1" {
		t.Errorf("driver = %s/%s, want phase/m1", got.agent, got.model)
	}

	// And a flag still overrides it, for a run against something the bench
	// does not declare.
	over, err := driver(nextFlags{config: w.config, runs: w.runs, agent: "other", model: "m2"}, catalogOf(t, w.config), w.repo)
	if err != nil {
		t.Fatal(err)
	}
	if over.agent != "other" || over.model != "m2" {
		t.Errorf("driver = %s/%s, want the override", over.agent, over.model)
	}
}

// A repository with nothing declaring what it is measured on cannot run a
// phase, and the refusal names the file to write.
func TestNoBenchMeansNoPhaseCanBeRun(t *testing.T) {
	w := newCrankWorld(t)

	code, _, stderr := runNext(t, w, "", "-yes")
	if code != exitError {
		t.Errorf("exit %d, want a refusal", code)
	}
	if !strings.Contains(stderr, "Nothing says what "+w.repo+" is measured on") {
		t.Errorf("the refusal does not say what is missing: %s", stderr)
	}
}

// A bench that declares arms and no driver says what to compare and not what to
// run the phases by. The refusal names the field rather than the concept.
func TestABenchWithNoDriverNamesTheFieldToAdd(t *testing.T) {
	w := newCrankWorld(t)
	declareDriver(t, w, "")

	code, _, stderr := runNext(t, w, "", "-yes")
	if code != exitError {
		t.Errorf("exit %d, want a refusal", code)
	}
	if !strings.Contains(stderr, `Add a "driver"`) {
		t.Errorf("the refusal does not name the field: %s", stderr)
	}
}

// Nothing runs until it is confirmed, and a decline is not a failure.
func TestNothingRunsUntilItIsConfirmed(t *testing.T) {
	w := newNextWorld(t)
	restore(t, &isTerminal, func(io.Reader) bool { return true })

	code, stdout, _ := runNext(t, w, "n\n")
	if code != exitRefused {
		t.Errorf("exit %d, want the refusal code", code)
	}
	if !strings.Contains(stdout, "Nothing was run") {
		t.Errorf("it does not say nothing was run:\n%s", stdout)
	}
	if _, err := os.Stat(filepath.Join(w.phaseDir("author"), "scenario.draft.yaml")); err == nil {
		t.Error("a phase ran after the confirmation was declined")
	}
}

// Before it runs anything it says what it is about to do, how far it will go,
// and that stopping costs nothing already done.
func TestItSaysWhatItIsAboutToDoBeforeDoingIt(t *testing.T) {
	w := newNextWorld(t)
	restore(t, &isTerminal, func(io.Reader) bool { return true })

	_, stdout, _ := runNext(t, w, "n\n")

	for _, want := range []string{
		"work through stages 1 to 3", "stop before anything is spent",
		"everything finished is kept", "Continue? [Y/n]",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("it does not say %q before asking:\n%s", want, stdout)
		}
	}
}

// A repository that has finished, and one that has given up, each read as what
// they are rather than as a phase name and a standing.
func TestAFinishedAndAParkedRepositoryEachReadAsThemselves(t *testing.T) {
	for _, tc := range []struct {
		what     string
		standing position.Standing
		want     []string
		absent   string
	}{
		{"finished", position.Finished,
			[]string{"Finished.", "reached the board", "Next: nothing"}, "sense-lab"},
		{"parked", position.Parked,
			[]string{"Given up after", "recorded rather than dropped", "Next: nothing here"}, "sense-lab"},
	} {
		at := position.Position{Repo: "fake-repo", Cycle: 6, Standing: tc.standing, Awaiting: phase.Done}
		got := standingBlock(at)
		for _, want := range tc.want {
			if !strings.Contains(got, want) {
				t.Errorf("%s: the stop does not say %q:\n%s", tc.what, want, got)
			}
		}
		if strings.Contains(got, tc.absent) {
			t.Errorf("%s: the stop offers a command for a repository with nothing to do:\n%s", tc.what, got)
		}
	}
}

// A phase that produced no verdict is reported as what the position says it is,
// rather than as a blank line where the result should be.
func TestAPhaseThatSaidNothingIsReportedAsThat(t *testing.T) {
	at := position.Position{Standing: position.Missing,
		Because: "author ran past its wall and recorded no verdict"}

	got := finished(at, phase.Author)
	if !strings.Contains(got, "ran past its wall") {
		t.Errorf("the line does not say what happened:\n%s", got)
	}
	if !strings.Contains(got, "✗") {
		t.Errorf("a phase that recorded nothing reads as a success:\n%s", got)
	}
}

// A verdict the graph does not declare words for still prints something a
// person can act on rather than an empty line.
func TestAVerdictWithNoWordsStillPrints(t *testing.T) {
	at := position.Position{Last: position.Attempt{Verdict: "SOMETHING-NEW"}}

	if got := finished(at, phase.Author); !strings.Contains(got, "SOMETHING-NEW") {
		t.Errorf("the line drops a verdict it has no words for:\n%s", got)
	}
}

// The command dies with the binary, and answers a usage error as one.
func TestNextUnderSignalsReportsAUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := nextSignals(nil, strings.NewReader(""), &stdout, &stderr); code != exitUsage {
		t.Errorf("exit %d, want a usage error", code)
	}
	if !strings.Contains(stderr.String(), "name exactly one repository") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// A repository nothing has admitted is cloned, pinned and indexed — and then it
// STOPS. What to measure is a decision, and a question written before anybody
// made it is work nobody asked for.
func TestAnUnknownRepositoryIsAdmittedAndThenStopsForTheDecision(t *testing.T) {
	a := newAdmission(t, admissionStatus)
	declarable(t, a.config)
	source, _ := sourceRepo(t)

	var stdout, stderr bytes.Buffer
	code := doNext(context.Background(), []string{"-config", a.config, "-runs", a.runs,
		"-checkouts", a.checkouts, "-sense", a.sense, "-yes", source},
		strings.NewReader(""), &stdout, &stderr)

	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, stdout.String(), stderr.String())
	}
	page := stdout.String()
	for _, want := range []string{
		"Five stages", "You are before stage 1. Nothing has been spent.",
		"the checkout", "pinned at", "is admitted.",
		"One decision before we start, and it is yours: what do we measure?",
		"I have written a starter to", "These are defaults",
		filepath.Join(a.config, "benches"),
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not say %q:\n%s", want, page)
		}
	}
	// It stopped: nothing has run a phase against a repository nobody has said
	// how to measure.
	if _, err := os.Stat(filepath.Join(a.runs, filepath.Base(source), "1")); err == nil {
		t.Error("a phase ran before the bench was declared")
	}
}

// The map is on the first screen, all five stages, so a reader meeting this for
// the first time sees the whole arc before agreeing to any of it.
func TestTheFirstScreenShowsTheWholeArc(t *testing.T) {
	a := newAdmission(t, admissionStatus)
	source, _ := sourceRepo(t)
	restore(t, &isTerminal, func(io.Reader) bool { return true })

	var stdout, stderr bytes.Buffer
	code := doNext(context.Background(), []string{"-config", a.config, "-runs", a.runs,
		"-checkouts", a.checkouts, "-sense", a.sense, source},
		strings.NewReader("n\n"), &stdout, &stderr)

	if code != exitRefused {
		t.Fatalf("exit %d, want the decline\n%s\n%s", code, stdout.String(), stderr.String())
	}
	page := stdout.String()
	for _, s := range []string{"1. Question", "2. Trial", "3. Rehearsal", "4. Paid run", "5. Verdict"} {
		if !strings.Contains(page, s) {
			t.Errorf("the first screen does not show %q:\n%s", s, page)
		}
	}
	if !strings.Contains(page, "Nothing is spent and nothing is decided") {
		t.Errorf("it does not say what cloning costs:\n%s", page)
	}
	if _, err := os.Stat(filepath.Join(a.checkouts, filepath.Base(source))); err == nil {
		t.Error("it cloned after the confirmation was declined")
	}
}

// A repository already waiting for money says so and offers the command,
// without running a phase. Running the flow again at a stop must not turn the
// crank past the point it stopped at.
func TestARepositoryAlreadyAtAStopSaysSoAndRunsNothing(t *testing.T) {
	w := newNextWorld(t)
	if code, stdout, stderr := runNext(t, w, "", "-yes"); code != exitWaiting {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}
	before := attemptCount(t, filepath.Join(w.runs, w.repo))

	code, stdout, _ := runNext(t, w, "", "-yes")

	if code != exitWaiting {
		t.Errorf("exit %d, want the waiting code again", code)
	}
	if !strings.Contains(stdout, "sense-lab pay "+w.repo) {
		t.Errorf("it does not offer the command that moves it:\n%s", stdout)
	}
	if strings.Contains(stdout, "─── 1. Question") {
		t.Errorf("it ran a phase at a stop:\n%s", stdout)
	}
	if after := attemptCount(t, filepath.Join(w.runs, w.repo)); after != before {
		t.Errorf("attempts went from %d to %d at a stop", before, after)
	}
}

func attemptCount(t *testing.T, tree string) int {
	t.Helper()
	got, err := position.Attempts(tree)
	if err != nil {
		t.Fatal(err)
	}
	return len(got)
}

// A question sent back to be written again reads as the loop working. It is
// marked as having gone the other way, and it says what follows — because the
// trial exists to catch exactly this, cheaply.
func TestAQuestionSentBackReadsAsTheMethodWorking(t *testing.T) {
	at := position.Position{Last: position.Attempt{Verdict: phase.Requestion}}

	got := finished(at, phase.Minibench)

	for _, want := range []string{"✗", "The question does not work yet.", "why the trial is cheap"} {
		if !strings.Contains(got, want) {
			t.Errorf("the line does not say %q:\n%s", want, got)
		}
	}
}

// What is left to do is said in stages, and the paid stage is where an
// unattended run stops.
func TestWhatIsLeftIsSaidInStages(t *testing.T) {
	for _, tc := range []struct {
		at   phase.Name
		want string
	}{
		{phase.Author, "work through stages 1 to 3"},
		{phase.Validate, "work through stages 3 to 3"},
		{phase.Bench, "work through stage 4"},
		{phase.Done, "carry on"},
	} {
		if got := remaining(position.Position{Awaiting: tc.at}); got != tc.want {
			t.Errorf("remaining(%s) = %q, want %q", tc.at, got, tc.want)
		}
	}
}

// A stage whose plan declares no wall still announces itself. A missing
// declaration is a reason to say less, not to say nothing.
func TestAStageWithNoDeclaredWallStillAnnouncesItself(t *testing.T) {
	got := starting(stage.Stages[0], 0)

	if !strings.Contains(got, "1. Question") || !strings.Contains(got, "started") {
		t.Errorf("the line does not announce the stage:\n%s", got)
	}
	if strings.Contains(got, "up to") {
		t.Errorf("the line invents a wall nothing declared:\n%s", got)
	}
	if strings.Contains(starting(stage.Stages[3], 90*time.Minute), "90 minutes") {
		t.Error("a long wall is not said in hours")
	}
}

// A standing with no words of its own falls back to the position's own
// sentence rather than to silence, and offers the command that reads it.
func TestAStandingWithNoWordsFallsBackToThePositionsOwn(t *testing.T) {
	at := position.Position{Standing: position.Missing, Repo: "fake-repo",
		Because: "author emitted DRAFT and did not write scenario.draft.yaml"}

	got := standingBlock(at)

	if !strings.Contains(got, "did not write scenario.draft.yaml") {
		t.Errorf("the stop does not carry the reason:\n%s", got)
	}
	if !strings.Contains(got, "sense-lab why fake-repo") {
		t.Errorf("the stop does not offer the command that reads it:\n%s", got)
	}
}

// A confirmation that could not be put is reported as itself, not as a
// decline: an unattended caller with no terminal has not said no.
func TestAConfirmationThatCouldNotBePutIsNotADecline(t *testing.T) {
	w := newNextWorld(t)

	code, stdout, stderr := runNext(t, w, "")
	if code != exitError {
		t.Errorf("exit %d, want the refusal to be reported as an error", code)
	}
	if strings.Contains(stdout, "Cancelled") {
		t.Errorf("it reads as though somebody declined:\n%s", stdout)
	}
	if !strings.Contains(stderr, "-yes") {
		t.Errorf("the error does not name the flag that fixes it: %s", stderr)
	}
}

// A scan that indexed nothing does not admit the repository, and says what
// Sense found rather than reporting a broken command.
func TestAScanThatIndexedNothingIsSaidPlainly(t *testing.T) {
	a := newAdmission(t, `{"index":{"files":0,"symbols":0,"edges":0,"embeddings":0,"coverage":0},`+
		`"languages":{},"profile":{"tier":"small"},"version":{"binary":"1.14.1-test"}}`)
	source, _ := sourceRepo(t)

	var stdout, stderr bytes.Buffer
	code := doNext(context.Background(), []string{"-config", a.config, "-runs", a.runs,
		"-checkouts", a.checkouts, "-sense", a.sense, "-yes", source},
		strings.NewReader(""), &stdout, &stderr)

	if code != exitError {
		t.Fatalf("exit %d, want a refusal\n%s\n%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "Sense found nothing to index") {
		t.Errorf("it does not say what happened:\n%s", stdout.String())
	}
	if _, err := os.Stat(filepath.Join(a.config, "repos", filepath.Base(source)+".json")); err == nil {
		t.Error("a repository Sense has nothing to say about was admitted anyway")
	}
}

// Every way this command cannot start is reported in its own name and stops
// before anything is cloned, indexed or spawned.
func TestTheWaysItCannotStartAreEachReported(t *testing.T) {
	w := newNextWorld(t)

	for _, tc := range []struct {
		what string
		args []string
		want string
	}{
		{"a flag it does not have", []string{"-nonesuch", w.repo}, "flag provided but not defined"},
		{"no repository at all", []string{"-config", w.config}, "name exactly one repository"},
		{"two repositories", []string{"-config", w.config, w.repo, "another"}, "name exactly one repository"},
		{"a config directory that is not one",
			[]string{"-config", filepath.Join(w.config, "repos", w.repo+".json"), w.repo}, ""},
		{"a name that is nothing on disk and nothing in the catalog",
			[]string{"-config", w.config, "-runs", w.runs, "../not/a/repository"}, ""},
	} {
		var stdout, stderr bytes.Buffer
		code := doNext(context.Background(), tc.args, strings.NewReader(""), &stdout, &stderr)
		if code == exitOK {
			t.Errorf("%s: exit 0, want a refusal\n%s", tc.what, stdout.String())
		}
		if stderr.Len() == 0 {
			t.Errorf("%s: it refused and said nothing", tc.what)
		}
		if tc.want != "" && !strings.Contains(stderr.String(), tc.want) {
			t.Errorf("%s: stderr = %q, want it to say %q", tc.what, stderr.String(), tc.want)
		}
	}
}

// A clone the lab cannot prepare stops the flow before a scan is spent on it.
func TestACheckoutThatCannotBePreparedStopsBeforeTheScan(t *testing.T) {
	a := newAdmission(t, admissionStatus)
	source, head := sourceRepo(t)
	// A repository the catalog pins at a revision its checkout is not at, and
	// which is not the lab's own to move back.
	artifact(t, filepath.Join(a.config, "repos", filepath.Base(source)+".json"),
		`{"id":"`+filepath.Base(source)+`","url":"https://example.test/r.git","commit":"`+
			strings.Repeat("0", len(head))+`","checkout":"`+source+`","languages":["go"]}`)
	artifact(t, filepath.Join(a.config, "benches", filepath.Base(source)+".json"),
		`{"repo":"`+filepath.Base(source)+`","judge":"m1","driver":{"agent":"phase","model":"m1"},`+
			`"subjects":["untreated","sense-main"],"arms":[{"role":"headline","model":"m1","runs":1}]}`)

	var stdout, stderr bytes.Buffer
	code := doNext(context.Background(), []string{"-config", a.config, "-runs", a.runs,
		"-checkouts", a.checkouts, "-sense", a.sense, "-yes", filepath.Base(source)},
		strings.NewReader(""), &stdout, &stderr)

	if code != exitError {
		t.Fatalf("exit %d, want a refusal\n%s\n%s", code, stdout.String(), stderr.String())
	}
	if stderr.Len() == 0 {
		t.Error("it refused and said nothing")
	}
}

// A phase reporting that the loop cannot go on stops it, and the page says what
// to do about it: nothing is wrong with the question, and nothing is re-run.
func TestABlockedPhaseStopsTheFlowAndSaysWhatToFix(t *testing.T) {
	at := position.Position{Repo: "fake-repo", Cycle: 1, Standing: position.Blocked,
		Awaiting: phase.Preflight,
		Because:  "nothing declares what fake-repo is measured on: write lab/benches/fake-repo.json"}

	got := standingBlock(at)

	for _, want := range []string{
		"something outside the loop has to change first",
		"write lab/benches/fake-repo.json",
		"Next: fix what it named above",
		"nothing here is wrong with the question",
		"sense-lab next fake-repo",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the stop does not say %q:\n%s", want, got)
		}
	}
}

// The blocked standing has an exit code of its own, because the act it asks for
// is its own: not read a transcript, not re-run anything, but edit a file.
func TestBlockedIsItsOwnExitCode(t *testing.T) {
	seen := map[int]position.Standing{}
	for standing, code := range stopOn {
		if was, clash := seen[code]; clash {
			t.Errorf("%q and %q share exit code %d", was, standing, code)
		}
		seen[code] = standing
	}
	if stopOn[position.Blocked] == 0 {
		t.Error("a blocked repository exits 0, which reads as success")
	}
}

// A lab that has declared nothing to measure with cannot be given a starting
// point, so it is told what is missing and stops. It is a stop rather than an
// error: nothing is broken, something has not been declared yet.
func TestALabThatCannotProposeAMatrixSaysWhatIsMissing(t *testing.T) {
	a := newAdmission(t, admissionStatus)
	source, _ := sourceRepo(t)
	// A lab with nothing to measure with: the models go, and with them any arm
	// that could be proposed.
	for _, id := range []string{"alpha-1", "beta-9"} {
		if err := os.Remove(filepath.Join(a.config, "models", id+".json")); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	code := doNext(context.Background(), []string{"-config", a.config, "-runs", a.runs,
		"-checkouts", a.checkouts, "-sense", a.sense, "-yes", source},
		strings.NewReader(""), &stdout, &stderr)

	if code != exitBlocked {
		t.Fatalf("exit %d, want the blocked code\n%s\n%s", code, stdout.String(), stderr.String())
	}
	page := stdout.String()
	for _, want := range []string{"is admitted.", "could not propose a starting point", "Write "} {
		if !strings.Contains(page, want) {
			t.Errorf("the page does not say %q:\n%s", want, page)
		}
	}
	// Admitted all the same: the clone and the index are done and keeping them
	// is what makes the next run pick up where this one stopped.
	if _, err := os.Stat(filepath.Join(a.config, "repos", filepath.Base(source)+".json")); err != nil {
		t.Errorf("the repository was not admitted: %v", err)
	}
}

// Every command this instrument prints is one it has.
//
// It printed three it did not: `sense-lab report`, `sense-lab harvest` and
// `sense-lab board`, one for each phase past the paid cell that stopped the
// loop. An operator who finished the expensive step was told to run a command
// that answers `unknown command`.
func TestEveryCommandThePagesPrintIsOneTheBinaryHas(t *testing.T) {
	w := newNextWorld(t)
	pages := []string{}
	_, stdout, _ := runNext(t, w, "", "-yes")
	pages = append(pages, stdout)
	for _, at := range []position.Position{
		{Repo: w.repo, Standing: position.Waiting, Awaiting: phase.Bench},
		{Repo: w.repo, Standing: position.Blocked, Awaiting: phase.Preflight, Because: "no bench"},
		{Repo: w.repo, Standing: position.Missing, Because: "no artifact"},
		{Repo: w.repo, Standing: position.Finished, Cycle: 1},
		{Repo: w.repo, Standing: position.Parked, Cycle: 6},
	} {
		pages = append(pages, standingBlock(at))
	}

	named := regexp.MustCompile(`sense-lab ([a-z-]+)`)
	for _, page := range pages {
		for _, m := range named.FindAllStringSubmatch(page, -1) {
			var out, errs bytes.Buffer
			if code := Run([]string{m[1], "-h"}, &out, &errs); code == exitUsage &&
				strings.Contains(errs.String(), "sense-lab — the bench instrument") {
				t.Errorf("a page says to run %q, which this binary does not have", m[0])
			}
		}
	}
}

// The archive a person asks for on purpose: every verdict, oldest first, which
// is what the position page used to print at every stop whether or not anybody
// wanted it.
func TestWhyPrintsTheWholeRecord(t *testing.T) {
	w := newNextWorld(t)
	if code, stdout, stderr := runNext(t, w, "", "-yes"); code != exitWaiting {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}

	var stdout, stderr bytes.Buffer
	if code := whyRepo([]string{"-runs", w.runs, w.repo}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	page := stdout.String()
	for _, want := range []string{"repo:     " + w.repo, "cycle:", "reached:", "awaiting:", "standing:"} {
		if !strings.Contains(page, want) {
			t.Errorf("the record does not carry %q:\n%s", want, page)
		}
	}
}

func TestWhyNeedsExactlyOneRepository(t *testing.T) {
	for _, args := range [][]string{{}, {"one", "two"}, {"-nonesuch", "one"}} {
		var stdout, stderr bytes.Buffer
		if code := whyRepo(args, &stdout, &stderr); code != exitUsage {
			t.Errorf("why %v = %d, want a usage error", args, code)
		}
	}
}

// A repository with no run tree has a position too, and it is the useful one:
// nothing has been scanned yet. Asking why about a repository that has done
// nothing is not an error.
func TestWhyAboutARepositoryThatHasDoneNothingSaysSo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := whyRepo([]string{"-runs", filepath.Join(t.TempDir(), "runs"), "nothing"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "has not been scanned") {
		t.Errorf("the record does not say what has happened:\n%s", stdout.String())
	}
}

// A run tree that cannot be read is reported rather than printed as an empty
// record. A repository that has done nothing and a tree nothing can read are
// different answers to the same question.
func TestWhyReportsATreeItCannotRead(t *testing.T) {
	// A file where the run tree belongs: reading it is not "nothing here yet".
	notATree := filepath.Join(t.TempDir(), "runs")
	writeArtifact(t, notATree, "not a directory")

	var stdout, stderr bytes.Buffer
	if code := whyRepo([]string{"-runs", notATree, "fake-repo"}, &stdout, &stderr); code != exitError {
		t.Fatalf("exit %d, want the unreadable tree reported\n%s", code, stdout.String())
	}
	if !strings.Contains(stderr.String(), "sense-lab why:") {
		t.Errorf("the error is not reported in the command's own name: %s", stderr.String())
	}
}

func catalogOf(t *testing.T, config string) *catalog.Catalog {
	t.Helper()
	c, err := catalog.Load(config)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
