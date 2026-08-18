package transcript

import (
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures are real captures, checked in byte for byte, from codex-cli
// 0.147.0 on 2026-08-18. The failure one was produced by asking for a model the
// account cannot reach, which is the cheapest way to make a tool fail for a
// reason that is genuinely the tool's.
func itemEvents(t *testing.T, name string) Transcript {
	t.Helper()
	tr, err := ReadItemEvents(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return tr
}

func TestASessionThatRanCommandsIsReadWholeFromItsItemEvents(t *testing.T) {
	tr := itemEvents(t, "item-events-tools.jsonl")

	if tr.Provisional() {
		t.Errorf("a session that finished cleanly is marked provisional: %s", tr.Why)
	}
	if tr.SessionID != "01a01433-8955-7af0-a42b-12c2305f9282" {
		t.Errorf("SessionID = %q, want the thread the tool reported", tr.SessionID)
	}
	if len(tr.Text) != 2 {
		t.Errorf("text blocks = %d, want the 2 the agent wrote", len(tr.Text))
	}
	if !strings.Contains(tr.Answer(), "./sub/greeting.txt") {
		t.Errorf("the answer does not carry the citation the agent gave:\n%s", tr.Answer())
	}
	// One command was run, and it arrives as a started item and a completed one.
	// Counting both would double every tool call in every cost comparison.
	if len(tr.Calls) != 1 {
		t.Errorf("calls = %d, want the 1 command the agent ran", len(tr.Calls))
	}
	if got := tr.Calls[0].Name; got != "command_execution" {
		t.Errorf("call name = %q, want the item type the tool reported", got)
	}
	if !strings.Contains(string(tr.Calls[0].Input), "hello world") {
		t.Errorf("the call does not carry what was actually run:\n%s", tr.Calls[0].Input)
	}
}

// Cached input is a SUBSET of the input this format reports, so the fresh count
// means what it means in the other reader. A total that folded them together
// could not be checked against a bill.
func TestTheFreshInputIsSeparatedFromWhatWasCached(t *testing.T) {
	tr := itemEvents(t, "item-events-tools.jsonl")

	if !tr.Usage.Reported {
		t.Fatal("a session that reported usage reads as one that said nothing")
	}
	if tr.Usage.Input != 28386-24064 {
		t.Errorf("fresh input = %d, want %d", tr.Usage.Input, 28386-24064)
	}
	if tr.Usage.CacheRead != 24064 {
		t.Errorf("cache reads = %d, want 24064", tr.Usage.CacheRead)
	}
	if tr.Usage.Output != 140 {
		t.Errorf("output = %d, want 140", tr.Usage.Output)
	}
}

// A stream that reported more cached input than input would otherwise produce a
// negative fresh count, which every consumer would carry as a discount.
func TestAStreamThatCachesMoreThanItReadsIsNotCountedBackwards(t *testing.T) {
	tr, err := parseItemEvents(strings.NewReader(
		`{"type":"thread.started","thread_id":"t"}
{"type":"item.completed","item":{"type":"agent_message","text":"done"}}
{"type":"turn.completed","usage":{"input_tokens":10,"cached_input_tokens":99,"output_tokens":1}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if tr.Usage.Input != 10 {
		t.Errorf("fresh input = %d, want the reported input rather than a negative", tr.Usage.Input)
	}
}

// A failed turn is a session that FINISHED badly, which is a different thing
// from one that was cut off, and both are different from a clean low result.
func TestAFailedTurnIsMarkedAsTheToolReportingAFailure(t *testing.T) {
	tr := itemEvents(t, "item-events-failed.jsonl")

	if !tr.Provisional() {
		t.Fatal("a turn the tool reported as failed reads as a clean result")
	}
	if !strings.Contains(tr.Why, "not supported") {
		t.Errorf("Why = %q, want it to carry what the tool said went wrong", tr.Why)
	}
}

func TestACaptureThatStopsMidStreamIsNotReadAsAFinishedSession(t *testing.T) {
	tr, err := parseItemEvents(strings.NewReader(
		`{"type":"thread.started","thread_id":"t"}
{"type":"turn.started"}
{"type":"item.completed","item":{"type":"agent_message","text":"working on it"}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !tr.Provisional() {
		t.Fatal("a session with no closing turn reads as finished; a killed arm would score as a weak one")
	}
	if !strings.Contains(tr.Why, "did not finish") {
		t.Errorf("Why = %q, want it to say the session did not finish", tr.Why)
	}
}

func TestUnreadableLinesAreCountedRatherThanSkipped(t *testing.T) {
	tr, err := parseItemEvents(strings.NewReader(
		`{"type":"thread.started","thread_id":"t"}
this is not JSON

{"type":"item.completed","item":{"type":"agent_message","text":"done"}}
{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":1}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if tr.Unparseable != 1 {
		t.Errorf("unparseable = %d, want 1", tr.Unparseable)
	}
	if !tr.Provisional() {
		t.Error("a capture with an unreadable line reads as whole")
	}
}

// An error event arrives before the turn fails, and it is what says why. A
// reader that only looked at the closing event would report the failure with no
// reason attached.
func TestAnErrorEventIsWhatSaysWhyWhenTheCloseDoesNot(t *testing.T) {
	tr, err := parseItemEvents(strings.NewReader(
		`{"type":"thread.started","thread_id":"t"}
{"type":"item.completed","item":{"type":"agent_message","text":"trying"}}
{"type":"error","message":"the provider refused"}
{"type":"turn.failed"}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !strings.Contains(tr.Why, "the provider refused") {
		t.Errorf("Why = %q, want the reason the error event gave", tr.Why)
	}
}

func TestAToolThatSaidNothingAtAllIsMarkedRatherThanScored(t *testing.T) {
	tr, err := parseItemEvents(strings.NewReader(
		`{"type":"thread.started","thread_id":"t"}
{"type":"turn.completed","usage":{"input_tokens":1,"output_tokens":0}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !tr.Provisional() {
		t.Fatal("a session that said nothing reads as an answer worth scoring")
	}
}

func TestAnEmptyCaptureIsNotAnAnswer(t *testing.T) {
	tr, err := parseItemEvents(strings.NewReader(""))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !tr.Provisional() {
		t.Fatal("a capture with nothing in it reads as a finished session")
	}
}

func TestAMissingCaptureIsReportedWithItsPath(t *testing.T) {
	_, err := ReadItemEvents(filepath.Join(t.TempDir(), "not-there.jsonl"))

	if err == nil {
		t.Fatal("a capture that is not on disk was read as an empty answer")
	}
}

// A real sense arm, captured on 2026-08-18 during the first end-to-end run of
// this adapter. An MCP call arrives as a started item too, so the started-item
// rule counts it — and the call carries the tool's own name rather than the
// generic kind, because "a tool was used" is not the question anyone asks of a
// transcript.
func TestAnMCPCallIsCountedAndCarriesTheToolItNamed(t *testing.T) {
	tr := itemEvents(t, "same-scenario-item-events.jsonl")

	if tr.Provisional() {
		t.Errorf("a clean sense arm reads as provisional: %s", tr.Why)
	}
	var names []string
	for _, c := range tr.Calls {
		names = append(names, c.Name)
	}
	want := []string{"sense_search", "command_execution", "command_execution"}
	if len(names) != len(want) {
		t.Fatalf("calls = %v, want %v", names, want)
	}
	for i, n := range want {
		if names[i] != n {
			t.Errorf("call %d = %q, want %q", i, names[i], n)
		}
	}
	// What the arm asked Sense is carried whole, undecoded, so the miner reads
	// what the tool actually sent rather than this package's idea of it.
	if !strings.Contains(string(tr.Calls[0].Input), `"server":"sense"`) {
		t.Errorf("the MCP call does not carry the server it reached:\n%s", tr.Calls[0].Input)
	}
}
