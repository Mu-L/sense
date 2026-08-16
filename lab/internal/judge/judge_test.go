package judge_test

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/luuuc/sense/lab/internal/judge"
)

// mastodonGold is the five-row reference the recorded contradiction was graded
// against, lifted verbatim from the corpus.
func mastodonGold(t *testing.T) []judge.Gold {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "mastodon-dependents.gold.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Rows []struct {
			ID       string `yaml:"id"`
			Relation string `yaml:"relation"`
		} `yaml:"rows"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	rows := make([]judge.Gold, 0, len(doc.Rows))
	for _, r := range doc.Rows {
		rows = append(rows, judge.Gold{ID: r.ID, Relation: r.Relation})
	}
	return rows
}

func verdict(t *testing.T, name string) judge.Verdict {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	v, err := judge.Parse(b)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return v
}

func grade(t *testing.T, name string) judge.Result {
	t.Helper()
	r, err := judge.Grade(mastodonGold(t), verdict(t, name))
	if err != nil {
		t.Fatalf("grade %s: %v", name, err)
	}
	return r
}

func TestAConfidentWrongReasonIsGradedContradicted(t *testing.T) {
	// The recorded case: app/lib/extractor.rb PARSES text entities, and a
	// baseline arm called it a renderer. That is a confident false claim, not a
	// vague one, and a scheme that let it count as merely related would absorb
	// exactly the failure this grading exists to catch.
	got := grade(t, "contradiction.verdict.json")

	if got.Contradicted != 1 {
		t.Errorf("Contradicted = %d, want the renderer claim", got.Contradicted)
	}
	claims := judge.Contradictions(verdict(t, "contradiction.verdict.json"))
	if len(claims) != 1 || claims[0].ID != "d:extractor-mention-re" {
		t.Fatalf("Contradictions = %v, want the extractor row named", claims)
	}
	// A number that fell tells nobody which claim was wrong.
	if !strings.Contains(claims[0].Why, "renderer") {
		t.Errorf("the contradiction is recorded as %q, which does not say what was claimed", claims[0].Why)
	}
}

func TestGroundedPrecisionFallsWhenAClaimContradictsTheReference(t *testing.T) {
	got := grade(t, "contradiction.verdict.json")

	// One contradiction out of four claims.
	if want := 0.75; !near(got.GroundedPrecision, want) {
		t.Errorf("GroundedPrecision = %.3f, want %.3f", got.GroundedPrecision, want)
	}
}

func TestVaguenessCostsRecallAndNotPrecision(t *testing.T) {
	// Silence is omission, not fabrication. An answer that names the right
	// places and says nothing about why has found them; it has not lied.
	vague := grade(t, "vague.verdict.json")
	contradicting := grade(t, "contradiction.verdict.json")

	if !near(vague.GroundedPrecision, 1) {
		t.Errorf("a vague answer scored %.3f on precision, want 1", vague.GroundedPrecision)
	}
	if vague.GroundedPrecision <= contradicting.GroundedPrecision {
		t.Errorf("vagueness (%.3f) is punished as hard as a contradiction (%.3f)",
			vague.GroundedPrecision, contradicting.GroundedPrecision)
	}
	// And it is not free either: nothing was covered, so nothing matched the
	// authored reason.
	if vague.Covered != 0 {
		t.Errorf("Covered = %d for an answer that gave no reasons", vague.Covered)
	}
}

func TestASilentAnswerIsOmissionRatherThanFabrication(t *testing.T) {
	got := grade(t, "silent.verdict.json")

	if !near(got.GroundedPrecision, 1) {
		t.Errorf("GroundedPrecision = %.3f for an answer that claimed nothing, want 1", got.GroundedPrecision)
	}
	if !near(got.RelatedRecall, 0) {
		t.Errorf("RelatedRecall = %.3f for an answer that claimed nothing, want 0", got.RelatedRecall)
	}
}

func TestAContradictedRowIsNotCreditedAsReached(t *testing.T) {
	// Naming the right place for the wrong reason is not reaching it, and
	// crediting it would let a confident error buy recall.
	got := grade(t, "contradiction.verdict.json")

	// Four rows claimed of five, one of them contradicted: three credited.
	if want := 3.0 / 5.0; !near(got.RelatedRecall, want) {
		t.Errorf("RelatedRecall = %.3f, want %.3f", got.RelatedRecall, want)
	}
}

func TestARowClaimedTwiceIsRefusedBecauseTheStatesAreExclusive(t *testing.T) {
	// contradicted is never also related. A verdict that says both has answered
	// a different question, and a number computed from it would look exactly
	// like a number computed from a good one.
	_, err := judge.Grade(mastodonGold(t), judge.Verdict{Claims: []judge.Claim{
		{ID: "d:extractor-mention-re", State: judge.Contradicted},
		{ID: "d:extractor-mention-re", State: judge.Related},
	}})

	if err == nil {
		t.Fatal("Grade accepted a row claimed as both contradicted and related")
	}
	if !strings.Contains(err.Error(), "exclusive") {
		t.Errorf("error = %v, want it to say the states are exclusive", err)
	}
}

func TestAClaimAboutARowThatIsNotInTheGroupIsRefused(t *testing.T) {
	_, err := judge.Grade(mastodonGold(t), judge.Verdict{Claims: []judge.Claim{
		{ID: "d:invented", State: judge.Covered},
	}})

	if err == nil {
		t.Fatal("Grade accepted a claim about a row the gold does not have")
	}
}

func TestAStateThatIsNotOneOfTheThreeIsRefused(t *testing.T) {
	_, err := judge.Grade(mastodonGold(t), judge.Verdict{Claims: []judge.Claim{
		{ID: "d:single-user-mode", State: "excellent"},
	}})

	if err == nil {
		t.Fatal("Grade accepted a state outside the tri-state")
	}
}

func TestAGoldRowWithNoIdIsRefused(t *testing.T) {
	// Without ids the claims cannot be matched at all, and the arithmetic would
	// silently be about nothing.
	if _, err := judge.Grade([]judge.Gold{{Relation: "somewhere"}}, judge.Verdict{}); err == nil {
		t.Fatal("Grade accepted a gold row with no id")
	}
}

func TestAnEmptyGoldGroupScoresNoRecallRatherThanDividingByZero(t *testing.T) {
	got, err := judge.Grade(nil, judge.Verdict{})
	if err != nil {
		t.Fatalf("Grade: %v", err)
	}
	if got.RelatedRecall != 0 || got.Total != 0 {
		t.Errorf("Grade(nil) = %+v", got)
	}
}

// The instruction.

func TestTheInstructionCarriesTheAuthoredReasonForEveryRow(t *testing.T) {
	// The authored relation IS the ground truth for the semantic layer. An
	// instruction that omitted it would be asking for an opinion.
	rows := mastodonGold(t)

	got := judge.Instruction(rows, "the answer")

	for _, row := range rows {
		if !strings.Contains(got, row.ID) {
			t.Errorf("the instruction omits %s", row.ID)
		}
		if !strings.Contains(got, "parsing text entities") && row.ID == "d:extractor-mention-re" {
			t.Errorf("the instruction omits the authored reason for %s", row.ID)
		}
	}
	if !strings.Contains(got, "the answer") {
		t.Error("the instruction omits the answer it is grading")
	}
}

func TestTheJudgeIsNeverAskedWhatIsMissing(t *testing.T) {
	// Completeness is recall against authored gold. A judge that opines on what
	// might be missing is inventing a reference.
	got := strings.ToLower(judge.Instruction(mastodonGold(t), "the answer"))

	for _, forbidden := range []string{"complete", "missing", "what else", "omitted"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("the instruction asks about %q", forbidden)
		}
	}
}

func TestNoBlindQualityAxisIsAskedForOrProduced(t *testing.T) {
	// The retired composite was not a bad idea implemented badly. It measured
	// the wrong thing, and it favoured the arm that wrote well over the arm
	// that found more. Reviving it is not a future pitch.
	got := strings.ToLower(judge.Instruction(mastodonGold(t), "the answer"))

	for _, forbidden := range []string{"quality", "how good", "rate the", "score the", "out of 10"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("the instruction asks for %q", forbidden)
		}
	}
}

// Reading the reply.

func TestProseAroundTheJsonDoesNotThrowAwayAPaidCall(t *testing.T) {
	v, err := judge.Parse([]byte("Sure — here you go.\n\n{\"claims\":[{\"id\":\"d:single-user-mode\",\"state\":\"covered\"}]}\n\nHope that helps."))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(v.Claims) != 1 {
		t.Errorf("Parse found %d claims, want 1", len(v.Claims))
	}
}

func TestABraceInsideAReasonDoesNotEndTheReply(t *testing.T) {
	// The judge quotes code, and code has braces in it.
	v, err := judge.Parse([]byte(`{"claims":[{"id":"a","state":"covered","why":"it calls do { |x| } here"},{"id":"b","state":"related"}]}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(v.Claims) != 2 {
		t.Fatalf("Parse found %d claims, want 2", len(v.Claims))
	}
}

func TestAnEscapedQuoteInsideAReasonDoesNotEndTheReply(t *testing.T) {
	v, err := judge.Parse([]byte(`{"claims":[{"id":"a","state":"covered","why":"it says \"renderer\" here"}]}`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(v.Claims) != 1 {
		t.Errorf("Parse found %d claims, want 1", len(v.Claims))
	}
}

func TestAReplyWithNoJsonIsRefusedRatherThanReadAsSilence(t *testing.T) {
	// An empty verdict and a reply the judge never gave are opposite results:
	// one says the answer claimed nothing, the other says nothing was graded.
	if _, err := judge.Parse([]byte("I could not grade this.")); err == nil {
		t.Fatal("Parse accepted a reply with no verdict in it")
	}
}

func TestAnUnbalancedReplyIsRefused(t *testing.T) {
	if _, err := judge.Parse([]byte(`{"claims":[{"id":"a"`)); err == nil {
		t.Fatal("Parse accepted a truncated reply")
	}
}

func TestAReplyThatIsNotTheExpectedShapeIsRefused(t *testing.T) {
	if _, err := judge.Parse([]byte(`{"claims":"none"}`)); err == nil {
		t.Fatal("Parse accepted a reply of the wrong shape")
	}
}

func TestContradictionsAreListedInAStableOrder(t *testing.T) {
	// A diagnosis reads this list, and a list that reorders between runs makes
	// two identical verdicts look like two different ones.
	v := judge.Verdict{Claims: []judge.Claim{
		{ID: "d:single-user-mode", State: judge.Contradicted, Why: "wrong reason"},
		{ID: "d:admin-audit-scope", State: judge.Covered},
		{ID: "d:extractor-mention-re", State: judge.Contradicted, Why: "called a renderer"},
	}}

	got := judge.Contradictions(v)

	if len(got) != 2 {
		t.Fatalf("Contradictions = %v, want both", got)
	}
	if got[0].ID != "d:extractor-mention-re" || got[1].ID != "d:single-user-mode" {
		t.Errorf("Contradictions = %v, want them in id order", got)
	}
}

func TestAVerdictWithNoContradictionsListsNone(t *testing.T) {
	if got := judge.Contradictions(verdict(t, "vague.verdict.json")); len(got) != 0 {
		t.Errorf("Contradictions = %v for a vague verdict", got)
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
