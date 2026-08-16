package judgerun_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/judge"
	"github.com/luuuc/sense/lab/internal/judgerun"
)

// rows is a small reference to grade against.
var rows = []judge.Gold{
	{ID: "d:one", Relation: "app/models/category.rb:1083 the entry point, whose job is dispatching"},
	{ID: "d:two", Relation: "app/lib/extractor.rb:28 parses text entities, borrowing the mention regexp"},
}

// judgeSaying stands in for the agent tool driving the judge: it echoes the
// reply it was built with, and records the payload it was given so a test can
// read what actually reached the model.
func judgeSaying(t *testing.T, reply string) (bin, payloadAt string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	dir := t.TempDir()
	payloadAt = filepath.Join(dir, "payload")
	bin = filepath.Join(dir, "agent")
	script := "#!/bin/sh\ncat > " + payloadAt + "\nprintf '%s' '" + reply + "'\n"
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin, payloadAt
}

const cleanReply = `{"claims":[{"id":"d:one","state":"covered","why":"matches"},{"id":"d:two","state":"contradicted","why":"called it a renderer"}]}`

func spec(t *testing.T, bin string) judgerun.Spec {
	t.Helper()
	return judgerun.Spec{
		Root:      filepath.Join(t.TempDir(), "grading"),
		Rows:      rows,
		Answer:    "Category is dispatched from category.rb and rendered by extractor.rb.",
		Command:   bin,
		Model:     "claude-opus-5",
		ModelFlag: "--model",
		HostPath:  os.Getenv("PATH"),
		Wall:      30 * time.Second,
		Grace:     200 * time.Millisecond,
	}
}

func TestAGradingReturnsTheVerdictAndTheNumbersItMeans(t *testing.T) {
	bin, _ := judgeSaying(t, cleanReply)

	got, err := judgerun.Run(context.Background(), spec(t, bin))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(got.Verdict.Claims) != 2 {
		t.Fatalf("Verdict has %d claims, want 2", len(got.Verdict.Claims))
	}
	if got.Score.Contradicted != 1 {
		t.Errorf("Contradicted = %d, want 1", got.Score.Contradicted)
	}
	if got.Score.GroundedPrecision != 0.5 {
		t.Errorf("GroundedPrecision = %.3f, want 0.5", got.Score.GroundedPrecision)
	}
}

func TestARunRecordsWhichModelGradedIt(t *testing.T) {
	// A board is only comparable across runs that were graded by the same
	// thing, and "which model graded this" is not recoverable afterwards.
	bin, _ := judgeSaying(t, cleanReply)

	got, err := judgerun.Run(context.Background(), spec(t, bin))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if got.Model != "claude-opus-5" {
		t.Errorf("Model = %q, want the pinned model", got.Model)
	}
}

func TestAJudgeWithNoPinnedModelIsRefused(t *testing.T) {
	// An unpinned judge drifts with whatever the arm happened to use, and a
	// judge that moves with the headline arm makes every board incomparable.
	bin, _ := judgeSaying(t, cleanReply)
	s := spec(t, bin)
	s.Model = ""

	_, err := judgerun.Run(context.Background(), s)

	if err == nil {
		t.Fatal("Run graded with no model pinned")
	}
	if !strings.Contains(err.Error(), "pinned") {
		t.Errorf("error = %v, want it to say the model is not pinned", err)
	}
}

func TestThePinnedModelIsWhatTheToolIsAskedFor(t *testing.T) {
	bin, _ := judgeSaying(t, cleanReply)

	got, err := judgerun.Run(context.Background(), spec(t, bin))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	args := readFile(t, filepath.Join(got.Root, "session", "run-meta.json"))
	if !strings.Contains(args, "claude-opus-5") || !strings.Contains(args, "--model") {
		t.Errorf("the recorded invocation does not select the pinned model:\n%s", args)
	}
}

func TestNothingReachingTheJudgeSaysWhichArmItIsGrading(t *testing.T) {
	// Checked on the payload the tool actually received, not on the code that
	// builds it. The code is right until somebody adds a helpful line, and
	// nothing about the resulting grade would look wrong.
	bin, payloadAt := judgeSaying(t, cleanReply)

	if _, err := judgerun.Run(context.Background(), spec(t, bin)); err != nil {
		t.Fatalf("Run: %v", err)
	}

	payload := readFile(t, payloadAt)
	answer := spec(t, bin).Answer
	if leaked := judgerun.Leaks(payload, answer); len(leaked) != 0 {
		t.Errorf("the judge was told %v", leaked)
	}
	for _, label := range []string{"sense", "baseline"} {
		framing := strings.ReplaceAll(strings.ToLower(payload), strings.ToLower(answer), "")
		if strings.Contains(framing, label) {
			t.Errorf("the payload's framing names the arm %q:\n%s", label, payload)
		}
	}
}

func TestABenchAuthoredArmLabelStopsTheGradingBeforeItIsSent(t *testing.T) {
	// The gold relations are bench-authored and reach the judge, so a row that
	// names an arm is a leak the bench put there. Refusing before the call is
	// what stops it costing a paid grading AND a wrong number.
	bin, _ := judgeSaying(t, cleanReply)
	s := spec(t, bin)
	s.Rows = append([]judge.Gold{{ID: "d:leak", Relation: "found by the sense arm"}}, rows...)

	_, err := judgerun.Run(context.Background(), s)

	if err == nil {
		t.Fatal("Run sent a grading that names the arm")
	}
	if !strings.Contains(err.Error(), "which arm") {
		t.Errorf("error = %v, want it to say the arm would be revealed", err)
	}
}

func TestAnAnswerThatNamesItsOwnToolsIsNotTreatedAsABenchLeak(t *testing.T) {
	// The recorded limit. An answer that says which tools it used identifies its
	// own arm, and no scrubbing removes that without corrupting the text being
	// graded. What the bench controls is the framing, and that is what is
	// checked; refusing here instead would make every sense-arm grading fail.
	payload := judge.Instruction(rows, "I used sense_graph to find the dependents.")

	if leaked := judgerun.Leaks(payload, "I used sense_graph to find the dependents."); len(leaked) != 0 {
		t.Errorf("Leaks = %v; the answer's own words are not the bench's leak", leaked)
	}
}

func TestTheJudgeIsSpawnedSomewhereWithNothingToRead(t *testing.T) {
	// Tool-lessness does not rest on a flag alone. The grading runs in an empty
	// directory inside a disposable HOME, so there is no repository, no MCP
	// registration and no routing guidance to reach even if a tool were enabled.
	bin := filepath.Join(t.TempDir(), "agent")
	// An agent that tries to read its way to an answer, and reports what it
	// found. If a repository were reachable, this would print its files.
	script := `#!/bin/sh
cat > /dev/null
printf '{"claims":[{"id":"d:one","state":"related","why":"saw %s"}]}' "$(ls -A | tr '\n' ' ')"
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := judgerun.Run(context.Background(), spec(t, bin))
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	why := got.Verdict.Claims[0].Why
	if strings.TrimSpace(why) != "saw" {
		t.Errorf("the judge found %q in its working directory, want nothing", why)
	}
}

func TestAJudgeThatSaidNothingUsableIsRefusedRatherThanScoredAsSilence(t *testing.T) {
	bin, _ := judgeSaying(t, "I decline to grade this.")

	if _, err := judgerun.Run(context.Background(), spec(t, bin)); err == nil {
		t.Fatal("Run accepted a reply with no verdict in it")
	}
}

func TestAJudgeThatClaimedARowOutsideTheReferenceIsRefused(t *testing.T) {
	bin, _ := judgeSaying(t, `{"claims":[{"id":"d:invented","state":"covered"}]}`)

	if _, err := judgerun.Run(context.Background(), spec(t, bin)); err == nil {
		t.Fatal("Run accepted a verdict about a row the reference does not have")
	}
}

func TestAGradingThatCouldNotFinishIsNotAScore(t *testing.T) {
	// A judge that hit its wall produced a partial answer, and grading from one
	// would be a number nobody could reconstruct.
	bin := filepath.Join(t.TempDir(), "agent")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ncat > /dev/null\nsleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	s := spec(t, bin)
	s.Wall = 300 * time.Millisecond

	_, err := judgerun.Run(context.Background(), s)

	if err == nil {
		t.Fatal("Run returned a score from a grading that hit its wall")
	}
	if !strings.Contains(err.Error(), "budget") {
		t.Errorf("error = %v, want it to say the grading could not finish", err)
	}
}

func TestAnAgentThatCannotBeSpawnedIsReported(t *testing.T) {
	s := spec(t, filepath.Join(t.TempDir(), "no-such-agent"))

	if _, err := judgerun.Run(context.Background(), s); err == nil {
		t.Fatal("Run reported success although the judge could not be spawned")
	}
}

func TestAGradingDirectoryThatAlreadyExistsIsRefused(t *testing.T) {
	bin, _ := judgeSaying(t, cleanReply)
	s := spec(t, bin)
	s.Root = t.TempDir()

	if _, err := judgerun.Run(context.Background(), s); err == nil {
		t.Fatal("Run reused an existing grading directory")
	}
}

func TestAToolWithNoModelFlagIsStillGivenItsModel(t *testing.T) {
	// A tool that takes the model as a bare argument is an ecosystem fact like
	// any other, and it must not end up passing an empty flag.
	bin, payloadAt := judgeSaying(t, cleanReply)
	s := spec(t, bin)
	s.ModelFlag = ""

	got, err := judgerun.Run(context.Background(), s)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	_ = payloadAt
	meta := readFile(t, filepath.Join(got.Root, "session", "run-meta.json"))
	if strings.Contains(meta, `""`) {
		t.Errorf("an empty flag was passed to the tool:\n%s", meta)
	}
	if !strings.Contains(meta, "claude-opus-5") {
		t.Errorf("the model was not passed at all:\n%s", meta)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
