package transcript

import (
	"path/filepath"
	"strings"
	"testing"
)

// Real captures from opencode 1.18.18 on 2026-08-18, checked in byte for byte.
// The failure one was produced by asking for a model the provider does not
// serve, which is the cheapest way to make a tool fail for its own reason.
func messageParts(t *testing.T, name string) Transcript {
	t.Helper()
	tr, err := ReadMessageParts(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return tr
}

func TestASessionThatRanToolsIsReadWholeFromItsMessageParts(t *testing.T) {
	tr := messageParts(t, "message-parts-tools.jsonl")

	if tr.Provisional() {
		t.Errorf("a session that finished cleanly is marked provisional: %s", tr.Why)
	}
	if !strings.HasPrefix(tr.SessionID, "ses_") {
		t.Errorf("SessionID = %q, want the session the tool reported", tr.SessionID)
	}
	if !strings.Contains(tr.Answer(), "greeting.txt") {
		t.Errorf("the answer does not carry what the agent found:\n%s", tr.Answer())
	}
	if len(tr.Calls) != 1 {
		t.Fatalf("calls = %d, want the 1 tool the agent used", len(tr.Calls))
	}
	if got := tr.Calls[0].Name; got != "bash" {
		t.Errorf("call name = %q, want the tool the agent named", got)
	}
	if !strings.Contains(string(tr.Calls[0].Input), "hello world") {
		t.Errorf("the call does not carry what was actually run:\n%s", tr.Calls[0].Input)
	}
}

// A tool part is reported again every time the call changes state. Counting the
// events would multiply every tool call in every cost comparison by however
// many states it passed through.
func TestOneCallReportedTwiceIsStillOneCall(t *testing.T) {
	tr, err := parseMessageParts(strings.NewReader(
		`{"type":"tool_use","sessionID":"s","part":{"type":"tool","tool":"bash","callID":"c1"}}
{"type":"tool_use","sessionID":"s","part":{"type":"tool","tool":"bash","callID":"c1"}}
{"type":"tool_use","sessionID":"s","part":{"type":"tool","tool":"read","callID":"c2"}}
{"type":"step_finish","sessionID":"s","part":{"type":"step-finish","reason":"stop","tokens":{"input":1,"output":1}}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(tr.Calls) != 2 {
		t.Errorf("calls = %d, want 2: one call reported twice is one call", len(tr.Calls))
	}
}

// Usage accumulates across steps. A session that took three steps and had only
// its last one counted would look three times cheaper than it was.
func TestUsageAccumulatesAcrossEveryStep(t *testing.T) {
	tr := messageParts(t, "message-parts-tools.jsonl")

	if !tr.Usage.Reported {
		t.Fatal("a session that reported tokens reads as one that said nothing")
	}
	// 102 fresh on the first step and 218 on the second, with each step paying
	// its own cache read of 6,912. Counting one step would report a third of
	// what the session cost.
	if tr.Usage.Input != 102+218 {
		t.Errorf("input = %d, want both steps counted (320)", tr.Usage.Input)
	}
	if tr.Usage.CacheRead != 6912*2 {
		t.Errorf("cache reads = %d, want each step's own read counted", tr.Usage.CacheRead)
	}
}

// A session that could not run at all reports one error event and nothing else:
// no step, no text, no close. Without reading it as a close, that reads as a
// session the wall cut off, which is a different failure with a different
// diagnosis.
func TestASessionThatCouldNotRunIsMarkedByItsError(t *testing.T) {
	tr := messageParts(t, "message-parts-failed.jsonl")

	if !tr.Provisional() {
		t.Fatal("a session that only produced an error reads as a clean result")
	}
	if !strings.Contains(tr.Why, "error") {
		t.Errorf("Why = %q, want it to say the tool reported an error", tr.Why)
	}
	if strings.Contains(tr.Why, "did not finish") {
		t.Errorf("Why = %q, which diagnoses a wall kill for a session that failed outright", tr.Why)
	}
}

// An error with no message at all still has to say something: a mark that names
// nothing is a mark nobody can act on.
func TestAnErrorWithNoMessageIsStillNamed(t *testing.T) {
	tr, err := parseMessageParts(strings.NewReader(
		`{"type":"error","sessionID":"s","error":{"name":"UnknownError"}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !strings.Contains(tr.Why, "UnknownError") {
		t.Errorf("Why = %q, want the name the tool gave", tr.Why)
	}
}

// A step's reason is not a verdict. The measured capture ends its first step
// with `tool-calls` and its last with `stop`, and a rule that read any reason
// but one as a failure marked that clean session as broken.
func TestAStepReasonIsNotReadAsAFailure(t *testing.T) {
	tr, err := parseMessageParts(strings.NewReader(
		`{"type":"tool_use","sessionID":"s","part":{"type":"tool","tool":"bash","callID":"c1"}}
{"type":"step_finish","sessionID":"s","part":{"type":"step-finish","reason":"tool-calls","tokens":{"input":1,"output":1}}}
{"type":"text","sessionID":"s","part":{"type":"text","text":"found it"}}
{"type":"step_finish","sessionID":"s","part":{"type":"step-finish","reason":"stop","tokens":{"input":1,"output":1}}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if tr.Provisional() {
		t.Errorf("a session that stopped to run tools reads as broken: %s", tr.Why)
	}
}

func TestACaptureThatStopsMidStreamIsNotReadAsFinished(t *testing.T) {
	tr, err := parseMessageParts(strings.NewReader(
		`{"type":"step_start","sessionID":"s","part":{"type":"step-start"}}
{"type":"text","sessionID":"s","part":{"type":"text","text":"working"}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if !strings.Contains(tr.Why, "did not finish") {
		t.Errorf("Why = %q, want it to say the session did not finish", tr.Why)
	}
}

func TestUnreadableLinesAreCountedRatherThanSkippedInMessageParts(t *testing.T) {
	tr, err := parseMessageParts(strings.NewReader(
		`{"type":"text","sessionID":"s","part":{"type":"text","text":"done"}}
not json at all
{"type":"step_finish","sessionID":"s","part":{"type":"step-finish","reason":"stop","tokens":{"input":1,"output":1}}}`))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if tr.Unparseable != 1 {
		t.Errorf("unparseable = %d, want 1", tr.Unparseable)
	}
}

func TestAMissingMessagePartsCaptureIsReported(t *testing.T) {
	if _, err := ReadMessageParts(filepath.Join(t.TempDir(), "gone.jsonl")); err == nil {
		t.Fatal("a capture that is not on disk was read as an empty answer")
	}
}

// A real sense arm, captured on 2026-08-18 during the first end-to-end run of
// this adapter. This tool namespaces an MCP call as `<server>_<tool>`, which is
// a third spelling and the reason nothing downstream matches a call name by
// equality.
func TestAnMCPCallCarriesTheToolItNamedInThisToolsSpelling(t *testing.T) {
	tr := messageParts(t, "same-scenario-message-parts.jsonl")

	if tr.Provisional() {
		t.Errorf("a clean sense arm reads as provisional: %s", tr.Why)
	}
	var found string
	for _, c := range tr.Calls {
		if strings.Contains(c.Name, "sense_search") {
			found = c.Name
		}
	}
	if found != "sense_sense_search" {
		t.Errorf("the sense call is named %q, want the server-prefixed form this tool writes", found)
	}
}

// Blank lines happen in a capture that was flushed in pieces, and they are not
// unreadable lines: counting them as unparseable would mark a whole clean
// transcript provisional and carry that into every number derived from it.
func TestBlankLinesAreNotCountedAsUnreadable(t *testing.T) {
	tr, err := parseMessageParts(strings.NewReader(
		"{\"type\":\"text\",\"sessionID\":\"s\",\"part\":{\"type\":\"text\",\"text\":\"done\"}}\n\n\n" +
			"{\"type\":\"step_finish\",\"sessionID\":\"s\",\"part\":{\"type\":\"step-finish\",\"reason\":\"stop\",\"tokens\":{\"input\":1,\"output\":1}}}\n"))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if tr.Unparseable != 0 {
		t.Errorf("unparseable = %d, want 0: a blank line is not an unreadable one", tr.Unparseable)
	}
	if tr.Provisional() {
		t.Errorf("a clean capture with blank lines reads as provisional: %s", tr.Why)
	}
}

// A line longer than the scanner's buffer is a silently truncated answer, which
// is a silently lowered score. It has to arrive as an error rather than as a
// short transcript.
func TestALineTooLongToReadIsAnErrorRatherThanASilentTruncation(t *testing.T) {
	huge := strings.Repeat("x", maxLine+1)

	if _, err := parseMessageParts(strings.NewReader(
		`{"type":"text","sessionID":"s","part":{"type":"text","text":"` + huge + `"}}`)); err == nil {
		t.Fatal("a line past the budget was read as a whole transcript")
	}
}
