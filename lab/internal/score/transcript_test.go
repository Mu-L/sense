package score

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func stream(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdout")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A citation the agent read in a grep result is not a citation the agent made.
// Counting tool inputs and results would score the tool rather than the answer,
// and would credit the sense arm for every path Sense handed it whether or not
// the answer ever mentioned it.
func TestOnlyWhatTheAgentSaidIsScored(t *testing.T) {
	path := stream(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"I found app/models/category.rb:1083."}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"lib/secret.rb:99: match"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Grep","input":{"pattern":"lib/other.rb:12"}}]}}`,
	)

	got, err := AnswerText(path)
	if err != nil {
		t.Fatalf("AnswerText: %v", err)
	}

	if !strings.Contains(got, "app/models/category.rb:1083") {
		t.Errorf("the agent's own text was dropped:\n%s", got)
	}
	for _, unwanted := range []string{"lib/secret.rb:99", "lib/other.rb:12"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("tool traffic leaked into the scored text: %q", unwanted)
		}
	}
}

// The stream is a vendor format that gains event kinds without warning. A line
// this reader does not understand must contribute nothing, not fail the score:
// a new event kind should never turn a paid run into an unreadable one.
func TestALineTheReaderDoesNotUnderstandIsIgnored(t *testing.T) {
	path := stream(t,
		`not json at all`,
		``,
		`{"type":"system","subtype":"init","cwd":"/tmp"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"a.rb:1"}]}}`,
	)

	got, err := AnswerText(path)
	if err != nil {
		t.Fatalf("AnswerText: %v", err)
	}
	if !strings.Contains(got, "a.rb:1") {
		t.Errorf("the readable event was lost:\n%s", got)
	}
}

// A session killed on its wall can leave its answer in the final result event
// and nowhere else, so that event is read too.
func TestTheFinalResultEventIsScoredToo(t *testing.T) {
	path := stream(t, `{"type":"result","result":"Summary: app/models/category.rb:1083"}`)

	got, err := AnswerText(path)
	if err != nil {
		t.Fatalf("AnswerText: %v", err)
	}
	if !strings.Contains(got, "app/models/category.rb:1083") {
		t.Errorf("the result event was dropped:\n%s", got)
	}
}

// A single assistant message can be far longer than bufio's default 64KB line
// budget — the live probe run produced a 52,000-character answer in one event,
// and the fixture beside it has a line over 100KB. A silently truncated line is
// a silently lowered score, which is the worst failure this reader can have.
func TestAVeryLongEventIsNotTruncated(t *testing.T) {
	huge := strings.Repeat("filler words to pad the answer out. ", 20000)
	path := stream(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"`+huge+`app/models/category.rb:1083"}]}}`,
	)

	got, err := AnswerText(path)
	if err != nil {
		t.Fatalf("AnswerText: %v", err)
	}
	if len(got) < len(huge) {
		t.Fatalf("text is %d bytes, want at least %d — the line was truncated", len(got), len(huge))
	}
	if !strings.Contains(got, "app/models/category.rb:1083") {
		t.Error("the citation at the end of a long answer was lost")
	}
}

// A line beyond even the raised budget must fail loudly. Returning a partial
// answer would score a truncated transcript as a weak arm.
func TestALineBeyondTheBudgetIsAnErrorNotAPartialScore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stdout")
	// One line, no newline, larger than the 64MB ceiling.
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	chunk := strings.Repeat("x", 1<<20)
	for i := 0; i < 65; i++ {
		if _, err := f.WriteString(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := AnswerText(path); err == nil {
		t.Error("a transcript too large to read was reported as readable")
	}
}

// A transcript that parses cleanly but contains no assistant output at all is a
// broken instrument, not a silent agent: a truncated capture, the wrong file, or
// a stream this reader no longer understands. Returning "" would score 0.00 and
// print "below floor", which makes a broken run indistinguishable from a weak
// arm — the one failure a scorer must never have.
func TestATranscriptWithNoAnswerInItIsAnErrorNotAZero(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines []string
	}{
		{"only tool traffic", []string{
			`{"type":"user","message":{"content":[{"type":"tool_result","content":"a.rb:1"}]}}`,
		}},
		{"only system events", []string{
			`{"type":"system","subtype":"init","cwd":"/tmp"}`,
		}},
		{"nothing parseable at all", []string{`garbage`, ``}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := AnswerText(stream(t, tc.lines...))

			if err == nil {
				t.Fatal("a transcript with no assistant output was read as an empty answer")
			}
			if !strings.Contains(err.Error(), "no assistant output") {
				t.Errorf("error = %q, want it to say the transcript carries no answer", err)
			}
		})
	}
}

// An assistant message can carry several text blocks. Keeping only the first
// would silently drop most of a long answer.
func TestEveryTextBlockOfAMessageIsKept(t *testing.T) {
	path := stream(t,
		`{"type":"assistant","message":{"content":[`+
			`{"type":"text","text":"first a.rb:1"},`+
			`{"type":"tool_use","name":"Grep","input":{}},`+
			`{"type":"text","text":"second b.rb:2"}]}}`,
	)

	got, err := AnswerText(path)
	if err != nil {
		t.Fatalf("AnswerText: %v", err)
	}
	for _, want := range []string{"a.rb:1", "b.rb:2"} {
		if !strings.Contains(got, want) {
			t.Errorf("%q was dropped from a multi-block message:\n%s", want, got)
		}
	}
}
