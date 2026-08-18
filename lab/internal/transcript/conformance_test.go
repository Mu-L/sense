package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// The conformance suite: one set of properties, every normalizer, no branches.
//
// Three tools now produce three formats and all three feed one scorer through
// one canonical type. That type is where a cross-model comparison quietly stops
// meaning anything: if one normalizer counts a tool call differently, or loses
// the last assistant block, or splits text where another joins it, then every
// number produced through that tool is shifted relative to the others and
// nothing errors. The board simply shows one model doing worse, and the natural
// reading is that the model is worse.
//
// The moment this file has a per-tool branch it has stopped testing conformance
// and started documenting divergence.

// conformanceCase is one real capture and what an INDEPENDENT read of it says.
//
// The figures below are not what the normalizer produced. They come from a
// separate scan of the same bytes, written for this purpose and run once over
// every fixture in the tree:
//
//	python3 -c '...'   # decode each line, count tool-call events by their own
//	                   # ids, count text blocks and their characters, sum the
//	                   # usage each event reports
//
// That is the only check here that can catch a shared mistake. Three
// normalizers agreeing on a wrong assumption pass every property below
// perfectly; they cannot all agree with a count taken from outside them.
type conformanceCase struct {
	format  string
	fixture string
	// calls is every tool call the capture itself reports, in order, comma
	// separated. In ORDER because the order is what a miner reads, and as one
	// string because one of these captures made 81 of them.
	calls string
	// texts is how many assistant text blocks it holds, and chars their total
	// length in CHARACTERS, which is what "nothing dropped, nothing duplicated"
	// means when the blocks are large. Characters rather than bytes because
	// that is what the independent scan counted, and half these captures carry
	// text that is not ASCII.
	texts int
	chars int
	// usage is what the capture reports, as fresh input, output, cache reads
	// and cache creation.
	usage    Usage
	reported bool
	// provisional says whether this capture is one the scorer must not read as
	// a clean result.
	provisional bool
}

func conformanceCases() []conformanceCase {
	return []conformanceCase{
		{
			format: "assistant-events", fixture: "claude-normal.jsonl",
			calls: repeated("Bash", 14), texts: 2, chars: 18356,
			usage: Usage{Input: 43, Output: 114, CacheRead: 845014, CacheCreation: 100046}, reported: true,
		},
		{
			format: "assistant-events", fixture: "result-usage-arm.jsonl",
			calls: "Read,Bash,Read,Read,Read,Read,Read,Read,Read,Read,Read,Read,Read,Read,Read,Read,Read," +
				"Read,Grep,Grep,Glob,Grep,Read,Grep,Grep,Read,Grep,Grep,Grep,Grep,Grep,Grep,Grep,Read,Read," +
				"Read,Read,Grep,Grep,Read,Read,Read,Read,Read,Grep,Grep,Read,Read,Read,Read,Grep,Read,Read," +
				"Read,Grep,Read,Read,Read,Read,Read,Read,Read,Glob,Glob,Grep,Grep,Read,Read,Read,Glob,Read," +
				"Read,Read,Read,Read,Read,Glob,Grep,Read,Read,todowrite",
			texts: 27, chars: 6287,
			usage: Usage{Input: 1790899, Output: 5502}, reported: true,
		},
		{
			// The session the tool itself reports as having ended badly. Every
			// other check passes it: it parses, it has text, it closes.
			format: "assistant-events", fixture: "errored-arm.jsonl",
			calls: repeated("Bash", 35), texts: 2, chars: 36526,
			usage: Usage{Input: 104, Output: 499, CacheRead: 2482057, CacheCreation: 113968}, reported: true,
			provisional: true,
		},
		{
			// It reports no usage anywhere AND flags itself as having ended
			// badly, which are two different things and both are carried.
			format: "assistant-events", fixture: "no-usage-arm.jsonl",
			texts: 1, chars: 147, provisional: true,
		},
		{
			// One line, all-zero usage, nothing said.
			format: "assistant-events", fixture: "empty-arm.jsonl",
			provisional: true,
		},
		{
			format: "item-events", fixture: "item-events-tools.jsonl",
			calls: "command_execution", texts: 2, chars: 105,
			usage: Usage{Input: 4322, Output: 140, CacheRead: 24064}, reported: true,
		},
		{
			format: "item-events", fixture: "same-scenario-item-events.jsonl",
			calls: "sense_search,command_execution,command_execution", texts: 2, chars: 755,
			usage: Usage{Input: 13825, Output: 613, CacheRead: 76544}, reported: true,
		},
		{
			format: "item-events", fixture: "item-events-failed.jsonl",
			provisional: true,
		},
		{
			format: "message-parts", fixture: "message-parts-tools.jsonl",
			calls: "bash", texts: 1, chars: 61,
			usage: Usage{Input: 320, Output: 116, CacheRead: 13824}, reported: true,
		},
		{
			format: "message-parts", fixture: "same-scenario-message-parts.jsonl",
			calls: "grep,sense_sense_search,read", texts: 1, chars: 868,
			usage: Usage{Input: 7375, Output: 566, CacheRead: 28928}, reported: true,
		},
		{
			format: "message-parts", fixture: "message-parts-failed.jsonl",
			provisional: true,
		},
	}
}

func repeated(name string, n int) string {
	out := make([]string, n)
	for i := range out {
		out[i] = name
	}
	return strings.Join(out, ",")
}

// wanted is the call list this capture reports, as names.
func (c conformanceCase) wanted() []string {
	if c.calls == "" {
		return nil
	}
	return strings.Split(c.calls, ",")
}

// TestEveryNormalizerAgreesWithItsOwnToolsFigures is the check that catches a
// shared wrong assumption: the expected numbers were read off the captures by
// something that is not the normalizer.
func TestEveryNormalizerAgreesWithItsOwnToolsFigures(t *testing.T) {
	for _, tc := range conformanceCases() {
		t.Run(tc.fixture, func(t *testing.T) {
			tr := mustRead(t, tc.format, tc.fixture)

			// Every tool call the tool reported, exactly once, in order.
			var got []string
			for _, c := range tr.Calls {
				got = append(got, c.Name)
			}
			want := tc.wanted()
			if len(got) != len(want) {
				t.Fatalf("calls = %d %v, want the %d the capture reports %v", len(got), got, len(want), want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("call %d = %q, want %q: the order is what a miner reads", i, got[i], want[i])
				}
			}

			// Assistant text: nothing dropped, nothing duplicated.
			if len(tr.Text) != tc.texts {
				t.Errorf("text blocks = %d, want %d", len(tr.Text), tc.texts)
			}
			if chars := utf8.RuneCountInString(strings.Join(tr.Text, "")); chars != tc.chars {
				t.Errorf("text characters = %d, want %d: something was dropped or repeated", chars, tc.chars)
			}

			// Usage is the tool's own, never derived.
			if tr.Usage.Reported != tc.reported {
				t.Errorf("usage reported = %v, want %v", tr.Usage.Reported, tc.reported)
			}
			if tr.Usage.Input != tc.usage.Input || tr.Usage.Output != tc.usage.Output ||
				tr.Usage.CacheRead != tc.usage.CacheRead || tr.Usage.CacheCreation != tc.usage.CacheCreation {
				t.Errorf("usage = %+v, want %+v", tr.Usage, tc.usage)
			}

			if tr.Provisional() != tc.provisional {
				t.Errorf("provisional = %v (%s), want %v", tr.Provisional(), tr.Why, tc.provisional)
			}
		})
	}
}

// A missing field is ABSENT, not zero. A zero that means "not reported" is
// worse than an absence, because it averages: 16 of the 238 recorded captures
// report no usage anywhere, and folding them in as zeros would quietly lower
// every mean they appear in.
func TestAToolThatReportedNoUsageSaysSoRatherThanReportingZero(t *testing.T) {
	for _, tc := range conformanceCases() {
		if tc.reported {
			continue
		}
		t.Run(tc.fixture, func(t *testing.T) {
			tr := mustRead(t, tc.format, tc.fixture)

			if tr.Usage.Reported {
				t.Fatal("a capture with no usage in it reads as having reported some")
			}
			if !strings.Contains(tr.String(), "usage not reported") {
				t.Errorf("the summary reads %q, which a reader would take as a cheap session", tr.String())
			}
		})
	}
}

// Every normalizer marks an unreadable line, and every one carries the mark
// into the transcript rather than into a field nobody reads.
func TestAnUnreadableLineMakesEveryFormatProvisional(t *testing.T) {
	for _, tc := range conformanceCases() {
		if tc.provisional {
			continue // already provisional for a louder reason
		}
		t.Run(tc.fixture, func(t *testing.T) {
			damaged := withLine(t, tc.fixture, "{ this is not JSON")

			tr, err := Read(tc.format, damaged)
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			if tr.Unparseable != 1 {
				t.Errorf("unparseable = %d, want the 1 line that cannot be read", tr.Unparseable)
			}
			if !tr.Provisional() {
				t.Fatal("a capture with an unreadable line reads as whole")
			}
			if !strings.Contains(tr.ProvisionalWhy(), "unreadable") {
				t.Errorf("Why = %q, want it to name the unreadable lines", tr.ProvisionalWhy())
			}
		})
	}
}

// Every normalizer marks a capture that stops mid-stream. A session cut off by
// the wall ends on a line boundary as often as not, so every line parses and
// there is plenty of text — it just stops, and without this it scores as a
// clean low result.
func TestATruncatedCaptureIsMarkedInEveryFormat(t *testing.T) {
	for _, tc := range conformanceCases() {
		if tc.provisional || tc.calls == "" {
			continue // nothing to cut that leaves a session worth reading
		}
		t.Run(tc.fixture, func(t *testing.T) {
			cut := withoutLastLine(t, tc.fixture)

			tr, err := Read(tc.format, cut)
			if err != nil {
				t.Fatalf("read: %v", err)
			}

			if tr.Unparseable != 0 {
				t.Errorf("unparseable = %d: cutting at a line boundary leaves every line readable", tr.Unparseable)
			}
			if !tr.Provisional() {
				t.Fatal("a session that never closed reads as one that finished")
			}
			if !strings.Contains(tr.Why, "did not finish") {
				t.Errorf("Why = %q, want it to say the session did not finish", tr.Why)
			}
		})
	}
}

// Every normalizer carries the tool's own session id, which is the only thing
// that correlates a run with the tool's own logs.
func TestEveryFormatCarriesTheToolsOwnSessionID(t *testing.T) {
	for _, tc := range conformanceCases() {
		t.Run(tc.fixture, func(t *testing.T) {
			if tr := mustRead(t, tc.format, tc.fixture); tr.SessionID == "" {
				t.Error("no session id, so this run cannot be tied back to the tool's own logs")
			}
		})
	}
}

// Every normalizer carries a call's arguments as the tool sent them, undecoded.
// A call whose input was dropped is a call the miner can count and not read.
func TestEveryFormatCarriesWhatACallActuallyAsked(t *testing.T) {
	for _, tc := range conformanceCases() {
		if tc.calls == "" {
			continue
		}
		t.Run(tc.fixture, func(t *testing.T) {
			for i, c := range mustRead(t, tc.format, tc.fixture).Calls {
				if len(c.Input) == 0 {
					t.Errorf("call %d (%s) carries no input at all", i, c.Name)
				}
			}
		})
	}
}

// The differential check: the SAME scenario, the same repository, the same day,
// run on all three tools.
//
// The answers differ because the models differ, and the call counts differ for
// the same reason — asserting they match would be asserting three models made
// the same choices. What must not differ is the STRUCTURE: every arm says what
// it did, says what it cost, and can be tied back to its own logs.
func TestTheSameScenarioOnThreeToolsProducesTheSameShapeOfTranscript(t *testing.T) {
	for _, tc := range []struct{ format, fixture string }{
		{"assistant-events", "same-scenario-assistant-events.jsonl"},
		{"item-events", "same-scenario-item-events.jsonl"},
		{"message-parts", "same-scenario-message-parts.jsonl"},
	} {
		t.Run(tc.format, func(t *testing.T) {
			tr := mustRead(t, tc.format, tc.fixture)

			if tr.Provisional() {
				t.Errorf("a clean run reads as provisional: %s", tr.Why)
			}
			if len(tr.Calls) == 0 {
				t.Error("no tool calls at all, from an arm that used tools")
			}
			if len(tr.Text) == 0 {
				t.Error("the arm said nothing, so there is nothing to score")
			}
			if !tr.Usage.Reported {
				t.Error("no usage, so this arm cannot be priced against the others")
			}
			if tr.SessionID == "" {
				t.Error("no session id")
			}
			// The answer the scorer reads is exactly the blocks, joined, in
			// order. A normalizer that reordered or re-joined them would move
			// every citation in that answer.
			if got, want := tr.Answer(), strings.Join(tr.Text, "\n"); got != want {
				t.Error("the answer is not the text blocks joined in order")
			}
		})
	}
}

func mustRead(t *testing.T, format, fixture string) Transcript {
	t.Helper()
	tr, err := Read(format, filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read %s as %s: %v", fixture, format, err)
	}
	return tr
}

// withLine copies a real capture with one line added, which is how a damaged
// capture is made out of a real one rather than out of an author's idea of the
// format.
func withLine(t *testing.T, fixture, line string) string {
	t.Helper()
	return writeCopy(t, fixture, func(body string) string {
		return strings.TrimRight(body, "\n") + "\n" + line + "\n"
	})
}

// withoutLastLine is the same capture cut at a line boundary, which is what a
// session killed by the wall usually looks like.
func withoutLastLine(t *testing.T, fixture string) string {
	t.Helper()
	return writeCopy(t, fixture, func(body string) string {
		lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
		return strings.Join(lines[:len(lines)-1], "\n") + "\n"
	})
}

func writeCopy(t *testing.T, fixture string, change func(string) string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", fixture))
	if err != nil {
		t.Fatalf("read %s: %v", fixture, err)
	}
	path := filepath.Join(t.TempDir(), fixture)
	if err := os.WriteFile(path, []byte(change(string(b))), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
