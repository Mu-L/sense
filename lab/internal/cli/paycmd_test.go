package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/plan"
	"github.com/luuuc/sense/lab/internal/position"
	"github.com/luuuc/sense/lab/internal/probe"
	"github.com/luuuc/sense/lab/internal/say"
)

// payWorld is a repository standing at the paid step, with a bench that
// declares what it is measured on and a scenario to measure it against.
type payWorld struct {
	probeWorld
	repo  string
	dir   string
	cells string
}

func newPayWorld(t *testing.T, bench string) payWorld {
	t.Helper()
	w := newProbeWorld(t)
	const repo = "probe-repo"

	if err := os.MkdirAll(filepath.Join(w.config, "benches"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.config, "benches", repo+".json"), []byte(bench), 0o644); err != nil {
		t.Fatal(err)
	}
	// A clone somebody handed in, named on the repository itself, so the paid
	// step reaches the same tree the probe world pinned.
	writeArtifact(t, filepath.Join(w.config, "repos", repo+".json"),
		`{"id":"`+repo+`","url":"https://example.com/r.git","commit":"`+commitOf(t, w.checkout)+
			`","checkout":"`+w.checkout+`","languages":["go"],"stack":"go"}`)

	// A repository standing where the paid step is owed: the rehearsal said
	// PAY, so the next thing it is waiting for is money.
	cycle := filepath.Join(w.runs, repo, "1")
	// The index lives beside the cycles rather than inside one: a repository is
	// scanned once and a re-entry does not rescan it.
	writeArtifact(t, filepath.Join(w.runs, repo, "index", "index.json"), `{"files":1,"symbols":1}`)
	writeArtifact(t, filepath.Join(cycle, "expand", "scenario.yaml"), readFile(t, w.scenario))
	writeArtifact(t, filepath.Join(cycle, "expand", "scenario.gold.yaml"), readFile(t, goldOf(w.scenario)))
	writeArtifact(t, filepath.Join(cycle, "expand", "scenario.rubric.yaml"),
		readFile(t, strings.TrimSuffix(w.scenario, ".yaml")+".rubric.yaml"))
	// The artifact is the fact: a position reads the phase's own output off
	// disk rather than believing what the record claims about it.
	writeArtifact(t, filepath.Join(cycle, string(phase.Validate), "pay-call.md"), "# Verdict\n\nPAY.\n")
	recordPhase(t, filepath.Join(w.runs, repo), phase.Validate, phase.Pay)

	return payWorld{probeWorld: w, repo: repo,
		dir:   filepath.Join(cycle, string(phase.Bench)),
		cells: filepath.Join(cycle, string(phase.Bench), cellsFile)}
}

func (w payWorld) args(extra ...string) []string {
	return append([]string{"pay",
		"-config", w.config,
		"-runs", w.runs,
		"-checkouts", filepath.Dir(w.checkout),
		"-sense", w.senseBin,
		"-wall", "20s",
		"-yes",
		w.repo,
	}, extra...)
}

func runPay(t *testing.T, args []string, in string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := payCells(context.Background(), args[1:], strings.NewReader(in), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeArtifact(t *testing.T, at, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(at, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// commitOf is the revision the handed-in clone sits at.
func commitOf(t *testing.T, checkout string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", checkout, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func readFile(t *testing.T, at string) string {
	t.Helper()
	b, err := os.ReadFile(at)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// goldOf is the gold file beside a scenario, which scenario.LoadPath expects to
// find by name.
func goldOf(scenarioPath string) string {
	return strings.TrimSuffix(scenarioPath, ".yaml") + ".gold.yaml"
}

func recordPhase(t *testing.T, tree string, at phase.Name, v phase.Verdict) {
	t.Helper()
	if err := position.Record(tree, position.Attempt{
		Cycle: 1, Phase: at, Try: 1, Verdict: v, Table: "the rehearsal cleared the bar",
		Artifact: "written", VerdictDoc: "written",
	}); err != nil {
		t.Fatal(err)
	}
}

const oneArm = `{"repo":"probe-repo","judge":"fake-model","subjects":["untreated","sense-main"],
  "arms":[{"role":"headline","model":"fake-model","runs":1}]}`

const twoRuns = `{"repo":"probe-repo","judge":"fake-model","subjects":["untreated","sense-main"],
  "arms":[{"role":"headline","model":"fake-model","runs":2}]}`

// The whole point of the command: one invocation runs the declared cells,
// scores both arms of each, and leaves the record behind — with no path, no
// arm and no commit typed by the operator.
func TestPayRunsTheDeclaredCellsAndRecordsThem(t *testing.T) {
	w := newPayWorld(t, oneArm)

	code, stdout, stderr := runPay(t, w.args(), "")
	if code != exitOK {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	var rec paidCells
	if err := json.Unmarshal([]byte(readFile(t, w.cells)), &rec); err != nil {
		t.Fatalf("%s: %v", w.cells, err)
	}
	if len(rec.Cells) != 1 {
		t.Fatalf("recorded %d cells, want 1: %+v", len(rec.Cells), rec.Cells)
	}
	c := rec.Cells[0]
	if !c.Sound || !c.Complete {
		t.Errorf("the cell is not a usable measurement: %+v", c)
	}
	for _, arm := range []string{"sense", "baseline"} {
		if c.Arms[arm] == "" {
			t.Errorf("the record does not name the %s arm: %+v", arm, c)
		}
		if _, err := os.Stat(filepath.Join(c.Dir, arm, "session")); err != nil {
			t.Errorf("the %s arm has no session on disk: %v", arm, err)
		}
	}
	if rec.Repo != w.repo || rec.Cycle != 1 || rec.Group == "" {
		t.Errorf("the record does not say what was measured: %+v", rec)
	}
}

// Every replicate the bench declares is run, and each lands beside the last
// rather than on top of it.
func TestEveryDeclaredRunIsRunAndLandsBesideTheOthers(t *testing.T) {
	w := newPayWorld(t, twoRuns)

	code, stdout, stderr := runPay(t, w.args(), "")
	if code != exitOK {
		t.Fatalf("exit %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	var rec paidCells
	if err := json.Unmarshal([]byte(readFile(t, w.cells)), &rec); err != nil {
		t.Fatal(err)
	}
	if len(rec.Cells) != 2 {
		t.Fatalf("recorded %d cells, want 2", len(rec.Cells))
	}
	if rec.Cells[0].Dir == rec.Cells[1].Dir {
		t.Errorf("both runs landed in %s", rec.Cells[0].Dir)
	}
	if !strings.Contains(stdout, "run 1 of 2") || !strings.Contains(stdout, "run 2 of 2") {
		t.Errorf("the runs are not reported as they happen:\n%s", stdout)
	}
	if !strings.Contains(stdout, "averaged over 2 runs") {
		t.Errorf("the summary does not say what the number stands on:\n%s", stdout)
	}
}

// The operator does no arithmetic. Whatever the arms scored, the page states
// what it means rather than printing two recalls.
func TestTheGapIsStatedRatherThanLeftToTheReader(t *testing.T) {
	w := newPayWorld(t, oneArm)

	_, stdout, _ := runPay(t, w.args(), "")

	if !strings.Contains(stdout, "With Sense") && !strings.Contains(stdout, "no advantage") {
		t.Errorf("the page does not say what the numbers mean:\n%s", stdout)
	}
	if decimal.MatchString(stdout) {
		t.Errorf("the page prints a recall in the scorer's form:\n%s", stdout)
	}
	if strings.Contains(stdout, "delta") {
		t.Errorf("the page says %q, which is the mechanism's word:\n%s", "delta", stdout)
	}
}

// The warning states consequence, not action: what it costs, what it uses up,
// and what an interruption does.
func TestTheWarningStatesWhatSpendingCosts(t *testing.T) {
	w := newPayWorld(t, twoRuns)

	_, stdout, _ := runPay(t, w.args(), "")

	for _, want := range []string{
		"spends money", "4 real agent sessions", "cannot be undone", "about 1 minute",
		"2 of the 40 paid runs", "stops there and spends nothing further",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the warning does not say %q:\n%s", want, stdout)
		}
	}
}

// Nothing is spent until the repository's own name is typed back.
func TestNothingIsSpentUntilTheNameIsTyped(t *testing.T) {
	w := newPayWorld(t, oneArm)
	args := w.args()
	args = withoutYes(args)

	restore(t, &isTerminal, func(io.Reader) bool { return true })
	spent := false
	restore(t, &runPair, func(_ context.Context, _ probe.Spec) (probe.Report, error) {
		spent = true
		return probe.Report{}, nil
	})

	code, stdout, _ := runPay(t, args, "not-the-name\n")
	if code != exitRefused {
		t.Errorf("exit %d, want the refusal code", code)
	}
	if spent {
		t.Error("it spent after the name was typed wrong")
	}
	if !strings.Contains(stdout, "Nothing was spent") {
		t.Errorf("it does not say nothing was spent:\n%s", stdout)
	}
	if _, err := os.Stat(w.cells); err == nil {
		t.Error("it wrote a record for a matrix it never ran")
	}
}

// A pair that is not a measurement stops the matrix where it is, and what it
// ran is on disk before the command returns.
func TestAnUnusablePairStopsTheMatrixAndIsRecorded(t *testing.T) {
	w := newPayWorld(t, twoRuns)
	calls := 0
	restore(t, &runPair, func(_ context.Context, s probe.Spec) (probe.Report, error) {
		calls++
		// Ran, cost what it cost, and cannot be compared: the baseline reached
		// Sense, which is the check the whole corpus rests on. Both arms are on
		// disk, because they ran — an unusable pair is not an unfinished one.
		fakeCell(t, s.Root)
		return probe.Report{BaselineReached: []string{"the baseline could reach the sense server"}}, nil
	})

	code, stdout, _ := runPay(t, w.args(), "")
	if code != exitRefused {
		t.Errorf("exit %d, want the refusal code", code)
	}
	if calls != 1 {
		t.Errorf("it ran %d cells, want it to stop after the first", calls)
	}
	if !strings.Contains(stdout, "Not a measurement") {
		t.Errorf("it does not say why it stopped:\n%s", stdout)
	}

	var rec paidCells
	if err := json.Unmarshal([]byte(readFile(t, w.cells)), &rec); err != nil {
		t.Fatalf("the cell it ran is not recorded: %v", err)
	}
	if len(rec.Cells) != 1 || rec.Cells[0].Sound {
		t.Errorf("the record does not name the unusable cell: %+v", rec.Cells)
	}
}

// The record is written after every cell, not at the end. A matrix interrupted
// between two cells used to leave nothing on disk naming the arms that ran, and
// a later pass would pair one of them.
func TestTheRecordIsOnDiskBeforeTheNextCellRuns(t *testing.T) {
	w := newPayWorld(t, twoRuns)
	var atSecond []paidCell
	first := true
	restore(t, &runPair, func(_ context.Context, s probe.Spec) (probe.Report, error) {
		if !first {
			var rec paidCells
			_ = json.Unmarshal([]byte(readFile(t, w.cells)), &rec)
			atSecond = rec.Cells
		}
		first = false
		return soundReport(t, s)
	})

	if code, stdout, stderr := runPay(t, w.args(), ""); code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}
	if len(atSecond) != 1 {
		t.Errorf("the first cell was not recorded before the second ran: %+v", atSecond)
	}
}

// A repository that is not standing at the paid step is refused, and told what
// to run instead. The command that spends may not be the one that guesses.
func TestARepositoryNotAtThePaidStepIsRefused(t *testing.T) {
	w := newPayWorld(t, oneArm)
	// Back to the authoring cycle: the rehearsal is no longer what happened
	// last, so nothing here is owed money.
	recordPhase(t, filepath.Join(w.runs, w.repo), phase.Minibench, phase.Requestion)

	code, _, stderr := runPay(t, w.args(), "")
	if code != exitError {
		t.Errorf("exit %d, want a refusal", code)
	}
	if !strings.Contains(stderr, "not at the paid step") || !strings.Contains(stderr, "sense-lab next") {
		t.Errorf("the refusal does not say what to run instead: %s", stderr)
	}
}

// No bench file is a refusal before anything spawns. Money spent against a
// matrix no file declares is spend nobody can defend afterwards, and it is
// what happened on mastodon cycle 3.
func TestNoBenchFileIsRefusedBeforeAnythingSpawns(t *testing.T) {
	w := newPayWorld(t, oneArm)
	if err := os.Remove(filepath.Join(w.config, "benches", w.repo+".json")); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runPay(t, w.args(), "")
	if code != exitError {
		t.Errorf("exit %d, want a refusal", code)
	}
	if !strings.Contains(stderr, "matrix no file states") {
		t.Errorf("the refusal does not say what is missing: %s", stderr)
	}
}

// A cell is one arm without Sense against one arm with it. A bench naming a
// third subject is naming a comparison this command cannot run, and it is
// refused before the first model's runs rather than discovered after them.
func TestABenchThatIsNotAPairIsRefused(t *testing.T) {
	for _, subjects := range []string{
		`["untreated","sense-main","archmcp"]`,
		`["untreated"]`,
		`["sense-main","sense-main"]`,
	} {
		w := newPayWorld(t, `{"repo":"probe-repo","judge":"fake-model","subjects":`+subjects+`,
			"arms":[{"role":"headline","model":"fake-model","runs":1}]}`)

		code, _, stderr := runPay(t, w.args(), "")
		if code != exitError {
			t.Errorf("subjects %s: exit %d, want a refusal", subjects, code)
		}
		// Two of these three are refused by the planner, in its own words, and
		// the third by the pair check here. What matters is that none of them
		// reaches a session.
		if !strings.Contains(stderr, "one arm without Sense against one arm with it") &&
			!strings.Contains(stderr, "no subject file declares") &&
			!strings.Contains(stderr, "a duplicated arm is not a pair") {
			t.Errorf("subjects %s: %s", subjects, stderr)
		}
	}
}

// The scenario is derived, not typed. The command this replaces printed a path
// that does not exist, because the scenario is written into the run tree and
// the run tree is not committed.
func TestTheScenarioComesFromWhereThePhaseThatWroteItPutIt(t *testing.T) {
	w := newPayWorld(t, oneArm)

	p, err := resolvePay(payFlags{config: w.config, runs: w.runs, checkouts: filepath.Dir(w.checkout),
		senseBin: w.senseBin, name: w.repo})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(w.runs, w.repo, "1", string(phase.Expand), "scenario.yaml")
	if p.scenario != want {
		t.Errorf("scenario %s, want %s", p.scenario, want)
	}

	if err := os.Remove(want); err != nil {
		t.Fatal(err)
	}
	if _, err := resolvePay(payFlags{config: w.config, runs: w.runs, senseBin: w.senseBin, name: w.repo}); err == nil {
		t.Error("it would have run against a scenario that is not there")
	}
}

// restore swaps a package variable for one test and puts it back.
func restore[T any](t *testing.T, at *T, with T) {
	t.Helper()
	was := *at
	*at = with
	t.Cleanup(func() { *at = was })
}

// decimal is a recall in the form the scorer produces it, which is the form no
// page of this command may print.
var decimal = regexp.MustCompile(`\d\.\d`)

func withoutYes(args []string) []string {
	return slices.DeleteFunc(slices.Clone(args), func(a string) bool { return a == "-yes" })
}

// soundReport is a cell that ran and can be compared: two arms on disk, the
// sense one having cited what was hidden and the baseline one not.
func soundReport(t *testing.T, s probe.Spec) (probe.Report, error) {
	t.Helper()
	fakeCell(t, s.Root)
	return probe.Report{SenseUsed: []string{"it called an MCP tool from the sense server"}, Frames: 3}, nil
}

// fakeCell is a completed pair on disk, as the prober would leave one.
func fakeCell(t *testing.T, root string) {
	t.Helper()
	fakeArm(t, filepath.Join(root, "sense"), "Category is used at app.go:3")
	fakeArm(t, filepath.Join(root, "baseline"), "Category is referenced in three files")
	writeArtifact(t, filepath.Join(root, "cell-meta.json"),
		`{"arms":{"sense":"`+filepath.Join(root, "sense")+`","baseline":"`+
			filepath.Join(root, "baseline")+`"},"complete":true}`)
}

func fakeArm(t *testing.T, dir, answer string) {
	t.Helper()
	at := filepath.Join(dir, "session")
	// The answer is what the agent SAID, which is the text content of an
	// assistant turn. A result event carries the session total and no answer,
	// and a capture with only one of those scores as silence.
	writeArtifact(t, filepath.Join(at, "raw", "stdout"),
		`{"type":"assistant","message":{"content":[{"type":"text","text":"`+answer+`"}]}}`+"\n"+
			`{"type":"result","result":"`+answer+`"}`+"\n")
	writeArtifact(t, filepath.Join(at, "run-meta.json"),
		`{"outcome":"completed","transcript_format":"assistant-events","stdout_bytes":128}`)
}

// The three things a cell can amount to, read as a person reads them. An
// interruption names what it burned, because a count would let a later pass
// pair the very run it is warning about.
func TestACellReadsAsWhatItAmountedTo(t *testing.T) {
	restore(t, &clock, func() string { return "09:41" })

	for _, tc := range []struct {
		what string
		cell paidCell
		want []string
	}{
		{"a measurement", paidCell{Complete: true, Sound: true, Sense: 0.87, Baseline: 0.20},
			[]string{"only difference between the two was Sense", "87%", "20%", "We need a 50-point gap"}},
		{"not comparable", paidCell{Complete: true, Note: "the baseline reached Sense"},
			[]string{"Not a measurement", "the baseline reached Sense"}},
		{"interrupted after one arm", paidCell{Burned: []string{"sense"}, Unusable: []string{"baseline"}},
			[]string{"Stopped part way", "sense", "can never be paired", "baseline"}},
		{"interrupted before any arm", paidCell{Unusable: []string{"sense", "baseline"}},
			[]string{"Stopped part way", "Nothing finished"}},
		{"interrupted with only a burned arm", paidCell{Burned: []string{"sense"}},
			[]string{"can never be paired"}},
	} {
		got := tc.cell.lines(0.50)
		for _, want := range tc.want {
			if !strings.Contains(got, want) {
				t.Errorf("%s: the line does not say %q:\n%s", tc.what, want, got)
			}
		}
	}
}

const twoModels = `{"repo":"probe-repo","judge":"fake-model","subjects":["untreated","sense-main"],
  "arms":[{"role":"headline","model":"fake-model","runs":1},
          {"role":"confirmation","model":"other-model","runs":1}]}`

// A confirmation arm is not held to the headline bar. What is asked of it is
// whether the result moves the same way on a second model, and the summary says
// which of those two questions each number answers.
func TestEachArmsNumberSaysWhatItIsFor(t *testing.T) {
	w := newPayWorld(t, twoModels)
	writeArtifact(t, filepath.Join(w.config, "models", "other-model.json"),
		`{"id":"other-model","provider":"fake","aliases":[],"available_under":["api_key"],"agents":["fake"]}`)
	restore(t, &runPair, func(_ context.Context, s probe.Spec) (probe.Report, error) {
		return soundReport(t, s)
	})

	code, stdout, stderr := runPay(t, w.args(), "")
	if code != exitOK {
		t.Fatalf("exit %d\n%s\n%s", code, stdout, stderr)
	}
	for _, want := range []string{"the headline number", "the confirmation model, which moved the same way"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the summary does not say %q:\n%s", want, stdout)
		}
	}
}

// A confirmation arm that did not move the same way is reported as that, not as
// a second headline that failed.
func TestAConfirmationArmThatDidNotMoveIsSaidSo(t *testing.T) {
	if got := roleNote(string(plan.Confirmation), say.Pair{Sense: 0.20, Baseline: 0.60}); !strings.Contains(got, "did not move") {
		t.Errorf("roleNote = %q, want it to say the confirmation did not move the same way", got)
	}
}

// A cell whose record cannot be read is not reported as a complete one. The
// record is the fact; its absence is not evidence that both arms finished.
func TestACellWithNoRecordIsNotReadAsComplete(t *testing.T) {
	c := paidCell{Dir: t.TempDir()}
	c.readRecord()
	if c.Complete || c.Arms != nil {
		t.Errorf("a cell with no record reads as %+v", c)
	}

	writeArtifact(t, filepath.Join(c.Dir, "cell-meta.json"), "{not json")
	c.readRecord()
	if c.Complete {
		t.Errorf("a cell with an unreadable record reads as complete: %+v", c)
	}
}

// The artifact is written where the phase's own directory is, and a tree it
// cannot write into is reported rather than passed over: a cell that ran and
// was not recorded is the burn this whole path exists to prevent.
func TestAnUnwritableRecordIsReported(t *testing.T) {
	at := filepath.Join(t.TempDir(), "bench")
	if err := os.WriteFile(at, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeCells(at, paidCells{Repo: "probe-repo"}); err == nil {
		t.Error("writing the record into a file reported no error")
	}
}

// A repository the catalog does not hold is refused with what to run instead.
func TestAnUnknownRepositoryIsRefused(t *testing.T) {
	w := newPayWorld(t, oneArm)

	code, _, stderr := runPay(t, []string{"pay", "-config", w.config, "-runs", w.runs, "no-such-repo"}, "")
	if code != exitError {
		t.Errorf("exit %d, want a refusal", code)
	}
	if !strings.Contains(stderr, "sense-lab next no-such-repo") {
		t.Errorf("the refusal does not say how to admit it: %s", stderr)
	}
}

// The command dies with the binary, and it answers a usage error as one.
func TestPayUnderSignalsReportsAUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := paySignals(nil, strings.NewReader(""), &stdout, &stderr); code != exitUsage {
		t.Errorf("exit %d, want a usage error", code)
	}
	if !strings.Contains(stderr.String(), "name exactly one repository") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// A scenario that cannot be scored is refused before anything spawns, rather
// than after two sessions. Scoring both arms against nothing returns zero for
// each, and zero against zero reads as a measurement in which Sense gave no
// advantage.
func TestAScenarioWithNothingToScoreAgainstIsRefused(t *testing.T) {
	w := newPayWorld(t, oneArm)
	// Gold with rows, none of them in the group the win is decided on.
	writeArtifact(t, filepath.Join(w.runs, w.repo, "1", string(phase.Expand), "scenario.gold.yaml"),
		"discriminator: dependents\nrows:\n"+
			`  - {id: "c:app", group: contract, match: ["app.go"], relation: "app.go:1 the Category type"}`+"\n")

	code, _, stderr := runPay(t, w.args(), "")
	if code != exitError {
		t.Errorf("exit %d, want a refusal", code)
	}
	if !strings.Contains(stderr, `the discriminator group "dependents" has no rows`) {
		t.Errorf("the refusal does not say what is missing: %s", stderr)
	}
}

// A repository that has spent its lifetime's paid runs is refused, and the
// refusal is the instrument working rather than a breakage.
func TestARepositoryAtItsCeilingIsRefused(t *testing.T) {
	w := newPayWorld(t, oneArm)
	for n := 1; n <= defaultCeiling; n++ {
		at := filepath.Join(w.runs, w.repo, "1", string(phase.Bench), fmt.Sprintf("cell-%d", n), "sense")
		writeArtifact(t, filepath.Join(at, "raw", "stdout"), "{}\n")
		writeArtifact(t, filepath.Join(at, "run-meta.json"), `{"outcome":"completed"}`)
	}

	code, _, stderr := runPay(t, w.args(), "")
	if code != exitError {
		t.Errorf("exit %d, want a refusal", code)
	}
	if !strings.Contains(stderr, "ceiling") {
		t.Errorf("the refusal does not name the ceiling: %s", stderr)
	}
}

// A bench with an arm that cannot run is refused whole. A partial matrix is not
// a measurement, and finding the unrunnable arm after the others have been paid
// for is the burn this check exists to prevent.
func TestABenchWithAnUnrunnableArmIsRefused(t *testing.T) {
	w := newPayWorld(t, oneArm)
	// A second tool the catalog holds, and a sense subject that can only be run
	// through it — while the arm's model is driven by the first. The cell loses
	// one side, and a cell that lost a subject rejects its survivor too.
	writeArtifact(t, filepath.Join(w.config, "agents", "other-tool", "agent.json"),
		`{"id":"other-tool","binary":"/bin/echo","setup_tool":"other-cli",`+
			`"transcript_format":"assistant-events","model_flag":"--model",`+
			`"mcp_registration":{"file":".mcp.json","servers_key":"mcpServers","command_style":"command+args"},`+
			`"config_dirs":[".other"],"headless_args":["-c"],"judge_args":["-c"],"env":[],"supports_mcp":true,`+
			`"auth_modes":["api_key"]}`)
	writeArtifact(t, filepath.Join(w.config, "subjects", "sense-main", "subject.json"),
		`{"id":"sense-main","kind":"sense","needs_mcp":true,"needs_isolated_config":true,`+
			`"executor":"isolated-home","agents":["other-tool"]}`)

	code, _, stderr := runPay(t, w.args(), "")
	if code != exitError {
		t.Errorf("exit %d, want a refusal", code)
	}
	if !strings.Contains(stderr, "A partial matrix is not a measurement") {
		t.Errorf("the refusal does not say why the whole bench is refused: %s", stderr)
	}
}

// An arm with no capture to read is reported rather than scored as silence. A
// missing transcript and an agent that said nothing are different results.
func TestAnArmWithNoCaptureIsReported(t *testing.T) {
	w := newPayWorld(t, oneArm)
	restore(t, &runPair, func(_ context.Context, _ probe.Spec) (probe.Report, error) {
		// A pair the checks pass, with nothing on disk to score.
		return probe.Report{SenseUsed: []string{"it called an MCP tool"}, Frames: 1}, nil
	})

	code, _, stderr := runPay(t, w.args(), "")
	if code != exitError {
		t.Errorf("exit %d, want the read to be reported", code)
	}
	if !strings.Contains(stderr, "score the sense arm") {
		t.Errorf("the error does not say which arm could not be read: %s", stderr)
	}
}

// A repository the lab cloned itself names no checkout, and the paid step
// reaches it where the lab keeps its own clones.
func TestTheLabsOwnCloneIsUsedWhenTheRepositoryNamesNone(t *testing.T) {
	w := newPayWorld(t, oneArm)
	writeArtifact(t, filepath.Join(w.config, "repos", w.repo+".json"),
		`{"id":"`+w.repo+`","url":"https://example.com/r.git","commit":"`+commitOf(t, w.checkout)+
			`","languages":["go"],"stack":"go"}`)

	p, err := resolvePay(payFlags{config: w.config, runs: w.runs, checkouts: "clones",
		senseBin: w.senseBin, name: w.repo})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("clones", w.repo); p.checkout != want {
		t.Errorf("checkout %s, want %s", p.checkout, want)
	}
}

// How long something will take is said in units a person plans around, and the
// nouns agree with their numbers.
func TestHowLongThingsTakeIsSaidInPlainUnits(t *testing.T) {
	for _, tc := range []struct {
		cells int
		each  time.Duration
		want  string
	}{
		{1, 30 * time.Second, "under a minute"},
		{2, 4 * time.Minute, "about 8 minutes"},
		{1, 90 * time.Minute, "about 1.5 hours"},
		{6, 30 * time.Minute, "about 3.0 hours"},
	} {
		if got := roughly(tc.cells, tc.each); got != tc.want {
			t.Errorf("roughly(%d, %s) = %q, want %q", tc.cells, tc.each, got, tc.want)
		}
	}
	if got := count(3, "hidden answer"); got != "3 hidden answers" {
		t.Errorf("count = %q", got)
	}
}

// The confirmation is only put to somebody who is there. A pipe is not.
func TestAPipeIsNotSomebodyToAsk(t *testing.T) {
	f, err := os.Open(writeTemp(t, "piped input"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if isTerminal(f) {
		t.Error("a file reads as a terminal")
	}
	if isTerminal(strings.NewReader("")) {
		t.Error("a reader that is not a file at all reads as a terminal")
	}
}

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	at := filepath.Join(t.TempDir(), "stdin")
	writeArtifact(t, at, body)
	return at
}

// Every screen of the flow opens the same way: which repository, which attempt
// of six, and where in the five stages this is. A reader who has walked away
// and come back reads those three lines and knows where they stand.
func TestEveryScreenOpensBySayingWhereYouAre(t *testing.T) {
	got := header("fake-repo", 3, phase.Bench)

	for _, want := range []string{"fake-repo", "attempt 3 of 6", "▸ 4. Paid run", "1. Question"} {
		if !strings.Contains(got, want) {
			t.Errorf("the header does not say %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, string(phase.Bench)) {
		t.Errorf("the header names the phase rather than the stage:\n%s", got)
	}
}

// A second run into a bench directory that already holds a record is refused.
//
// The record is written after every cell so an interruption leaves the burned
// arm named on disk. Starting a fresh record in the same place would write over
// the only thing that names it — the failure the incremental write exists to
// prevent, arriving through the front door.
func TestASecondRunWillNotWriteOverWhatWasAlreadyPaidFor(t *testing.T) {
	for _, tc := range []struct {
		what   string
		record string
		want   string
	}{
		{"a cell that cannot be paired",
			`{"repo":"probe-repo","cells":[{"model":"fake-model","dir":"cell","complete":false,` +
				`"burned":["sense"],"unusable":["baseline"]}]}`,
			"cannot be paired"},
		{"cells already paid for",
			`{"repo":"probe-repo","cells":[{"model":"fake-model","dir":"cell","complete":true,"sound":true}]}`,
			"already records"},
		{"a record nothing can read",
			`{not json`,
			"cannot be read"},
	} {
		w := newPayWorld(t, oneArm)
		writeArtifact(t, w.cells, tc.record)
		spent := false
		restore(t, &runPair, func(_ context.Context, _ probe.Spec) (probe.Report, error) {
			spent = true
			return probe.Report{}, nil
		})

		code, _, stderr := runPay(t, w.args(), "")

		if code != exitError {
			t.Errorf("%s: exit %d, want a refusal", tc.what, code)
		}
		if !strings.Contains(stderr, tc.want) {
			t.Errorf("%s: the refusal does not say %q: %s", tc.what, tc.want, stderr)
		}
		if spent {
			t.Errorf("%s: it spent over a record of what was already paid for", tc.what)
		}
		if got := readFile(t, w.cells); got != tc.record {
			t.Errorf("%s: the record changed:\n%s", tc.what, got)
		}
	}
}
