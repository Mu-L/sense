package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The fixtures are real captured runs, checked in byte for byte. They are the
// three shapes the recorded corpus actually contains, not three shapes someone
// imagined:
//
//   - claude-normal:    a Claude Code session with text, tool calls and usage
//   - result-usage-arm: usage on the CLOSING EVENT and nowhere else, which is
//     how three of the five arms report — 117 of 238 recorded transcripts
//   - errored-arm:      a session the tool reports as having ended badly. It
//     has text, parses cleanly and closes properly, so every check except
//     is_error passes it. 27 of 238, all on the headline arm
//   - no-usage-arm:     a session that genuinely reports no usage at all, which
//     is 16 of 238 rather than the 133 a first sweep claimed
//   - empty-arm:        the mistral arm, whose model id resolved to nothing
const testdata = "testdata"

func read(t *testing.T, name string) Transcript {
	t.Helper()
	tr, err := ReadClaudeCode(filepath.Join(testdata, name+".jsonl"))
	if err != nil {
		t.Fatalf("ReadClaudeCode(%s): %v", name, err)
	}
	return tr
}

// stream writes a synthetic capture that FINISHED: the closing result event is
// appended, because its absence is itself a provisional condition and a test
// about something else should not trip over it. Tests about an unfinished
// session use unfinishedStream.
func stream(t *testing.T, lines ...string) string {
	t.Helper()
	return unfinishedStream(t, append(lines, `{"type":"result","session_id":"s"}`)...)
}

// unfinishedStream writes exactly the lines given, with no closing event.
func unfinishedStream(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestARecordedSessionNormalizesToWhatItsConsumersRead(t *testing.T) {
	got := read(t, "claude-normal")

	if got.Provisional() {
		t.Errorf("a complete capture was marked provisional: %s", got.Why)
	}
	if len(got.Text) != 2 {
		t.Errorf("got %d text blocks, want 2", len(got.Text))
	}
	if len(got.Calls) != 14 {
		t.Errorf("got %d tool calls, want 14", len(got.Calls))
	}
	if got.SessionID == "" {
		t.Error("no session id, so this run cannot be correlated with the tool's own logs")
	}
	// Cache reads dwarf fresh input on a real session, and they are priced
	// differently, so folding them together would produce an accounting that
	// cannot be checked against a bill.
	if got.Usage.CacheRead != 845014 || got.Usage.Input != 43 {
		t.Errorf("usage = %+v, want cache reads counted separately from input", got.Usage)
	}
	if !got.Usage.Reported {
		t.Error("usage was present and reports as unreported")
	}
}

// A tool call's input is carried but not interpreted. Whether a call reached
// Sense, and what it asked, is the miner's question in a later cycle; this
// package answering it would mean two places deciding what a call means.
func TestToolCallsKeepTheirNameOrderAndInputUnread(t *testing.T) {
	got := read(t, "claude-normal")

	// Order with repeats: a caller wanting the sequence cannot recover it from
	// a set, and a tool called twice is two calls.
	if got.Calls[0].Name == "" {
		t.Error("a tool call has no name")
	}
	var withInput int
	for _, c := range got.Calls {
		if len(c.Input) > 0 {
			withInput++
		}
	}
	if withInput == 0 {
		t.Error("no tool call carried its input; the miner would have nothing to read")
	}
}

// Only what the agent SAID is text. A tool result is what it was TOLD, and
// counting it would score the tool rather than the answer. Thinking is neither.
func TestOnlyAssistantTextIsKept(t *testing.T) {
	path := stream(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"I say this"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"I think this"}]}}`,
		`{"type":"user","message":{"content":[{"type":"tool_result","content":"a grep told me this"}]}}`,
	)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}

	if answer := got.Answer(); answer != "I say this" {
		t.Errorf("answer = %q, want only what the agent said", answer)
	}
}

// THE failure this package's provisional mark exists for, and it is a real
// capture: the mistral arm, whose model id resolved to nothing. The whole
// transcript is one result event with zeros. Scored without the mark it reads
// as an arm that found nothing, which is a claim about the agent rather than
// about the run.
func TestTheEmptyArmIsProvisionalRatherThanAWeakResult(t *testing.T) {
	got := read(t, "empty-arm")

	if !got.Provisional() {
		t.Fatal("a capture with no agent output at all was not marked provisional")
	}
	if !strings.Contains(got.Why, "said nothing") {
		t.Errorf("why = %q, want it to say what is missing", got.Why)
	}
	if len(got.Text) != 0 || len(got.Calls) != 0 {
		t.Errorf("got %d text and %d calls from an empty capture", len(got.Text), len(got.Calls))
	}
	// The mark leads the summary, so it cannot be read past.
	if !strings.HasPrefix(got.String(), "PROVISIONAL") {
		t.Errorf("summary = %q, want the mark first", got.String())
	}
}

// Zero tokens and "the tool did not say" are different facts. Measured across
// the recorded corpus, 133 of 238 transcripts carry no usage at all — only the
// Claude arm reports it in this shape. A caller reading zero as a cheap session
// is the same failure as swallowing an unreadable line.
// Usage lives on the CLOSING EVENT for three of the five arms, under the same
// field names. Reading only the assistant turns said "the tool never told us"
// about 117 of 238 recorded transcripts — the flag doing precisely the thing it
// exists to prevent, on half the corpus.
func TestUsageIsReadFromTheClosingEventWhenTheTurnsDoNotCarryIt(t *testing.T) {
	got := read(t, "result-usage-arm")

	if got.Provisional() {
		t.Errorf("a complete capture was marked provisional: %s", got.Why)
	}
	if !got.Usage.Reported {
		t.Fatal("a session reporting usage on its closing event reads as unreported")
	}
	if got.Usage.Input != 1790899 || got.Usage.Output != 5502 {
		t.Errorf("usage = %+v, want the totals the closing event carries", got.Usage)
	}
	if !strings.Contains(got.String(), "1790899 in / 5502 out") {
		t.Errorf("summary = %q", got.String())
	}
}

// The closing event carries a session TOTAL, not a turn, so taking it as well
// as the turns would double every figure on the arm that reports both.
func TestTheClosingEventDoesNotDoubleCountUsageTheTurnsAlreadyGave(t *testing.T) {
	got := read(t, "claude-normal")

	if got.Usage.Input != 43 || got.Usage.Output != 114 {
		t.Errorf("usage = %+v, want the turn totals unchanged by the closing event", got.Usage)
	}
}

// And a session that genuinely reports nothing still says so. That is 16 of
// 238, not the 133 a first sweep claimed before the closing event was read.
func TestAToolThatReportsNoUsageAtAllStillSaysSo(t *testing.T) {
	got := read(t, "no-usage-arm")

	if got.Usage.Reported {
		t.Error("usage reports as reported when the tool never sent any")
	}
	if !strings.Contains(got.String(), "usage not reported") {
		t.Errorf("summary = %q, want it to say usage is unanswered rather than zero", got.String())
	}
}

// A real capture of the shape every other check passes: text, no unreadable
// lines, a proper close — and the tool saying it ended badly.
func TestARecordedErroredSessionIsProvisional(t *testing.T) {
	got := read(t, "errored-arm")

	if got.Unparseable != 0 || len(got.Text) == 0 || len(got.Calls) == 0 {
		t.Fatalf("the fixture is supposed to look complete: %d unreadable, %d text, %d calls",
			got.Unparseable, len(got.Text), len(got.Calls))
	}
	if !got.Provisional() {
		t.Fatal("a recorded session the tool reported as failed scored as a clean result")
	}
	if !strings.Contains(got.Why, "error_during_execution") {
		t.Errorf("why = %q, want the tool's own reason", got.Why)
	}
}

// A line that cannot be read makes the transcript provisional. Counting
// unreadable lines into a field nobody looks at, while a clean-looking number
// flows onward, is the same failure as not counting them at all.
func TestAnUnreadableLineMakesTheWholeTranscriptProvisional(t *testing.T) {
	path := stream(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"a.rb:1"}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"trunc`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"b.rb:2"}]}}`,
	)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}

	if got.Unparseable != 1 {
		t.Errorf("counted %d unreadable lines, want 1", got.Unparseable)
	}
	if !got.Provisional() {
		t.Fatal("a transcript with an unreadable line was not marked provisional")
	}
	// The readable events still come through: a provisional transcript is worth
	// reading, it is just not worth trusting whole.
	if len(got.Text) != 2 {
		t.Errorf("got %d text blocks, want the 2 that were readable", len(got.Text))
	}
	if !strings.Contains(got.String(), "1 unreadable lines") {
		t.Errorf("summary = %q, want the count visible", got.String())
	}
}

// An event kind this reader does not model contributes nothing and is not an
// error. Every streaming format has a dozen kinds and this needs four; the rest
// are skipped rather than represented, because a taxonomy has to be maintained
// against somebody else's release notes forever.
func TestAnUnmodelledEventKindIsSkippedNotFailed(t *testing.T) {
	path := stream(t,
		`{"type":"system","subtype":"hook_started","hook_name":"SessionStart"}`,
		`{"type":"rate_limit_event","limit":"x"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"a.rb:1"}]}}`,
	)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}

	if got.Unparseable != 0 {
		t.Errorf("an unmodelled event counted as unreadable: %d", got.Unparseable)
	}
	if got.Provisional() {
		t.Errorf("an unmodelled event made the transcript provisional: %s", got.Why)
	}
	if len(got.Text) != 1 {
		t.Errorf("got %d text blocks, want 1", len(got.Text))
	}
}

// Usage accumulates across turns: it is reported per assistant message, and a
// reader that took only the last one would report a fraction of the session.
func TestUsageAccumulatesAcrossTurns(t *testing.T) {
	path := stream(t,
		`{"type":"assistant","message":{"content":[],"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":100,"cache_creation_input_tokens":7}}}`,
		`{"type":"assistant","message":{"content":[],"usage":{"input_tokens":3,"output_tokens":2,"cache_read_input_tokens":50,"cache_creation_input_tokens":1}}}`,
	)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}

	want := Usage{Input: 13, Output: 7, CacheRead: 150, CacheCreation: 8, Reported: true}
	if got.Usage != want {
		t.Errorf("usage = %+v, want %+v", got.Usage, want)
	}
}

// A single assistant message can be enormous — a recorded run produced a
// 52,000-character answer in one event — and a silently truncated line is a
// silently lowered score.
func TestAVeryLongEventIsNotTruncated(t *testing.T) {
	huge := strings.Repeat("padding words to make this long. ", 30000)
	path := stream(t, `{"type":"assistant","message":{"content":[{"type":"text","text":"`+huge+`the-end"}]}}`)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}

	if !strings.HasSuffix(got.Answer(), "the-end") {
		t.Errorf("the answer was truncated at %d bytes", len(got.Answer()))
	}
	if got.Provisional() {
		t.Errorf("a long but complete transcript was marked provisional: %s", got.Why)
	}
}

func TestATranscriptThatCannotBeOpenedIsAnError(t *testing.T) {
	if _, err := ReadClaudeCode(filepath.Join(t.TempDir(), "absent.jsonl")); err == nil {
		t.Error("reading a transcript that does not exist succeeded")
	}
}

// A file with nothing in it is not an empty session, it is a capture that
// failed. Both are provisional, and this one says which.
func TestAnEmptyFileIsProvisional(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}
	if !got.Provisional() || !strings.Contains(got.Why, "no readable events") {
		t.Errorf("provisional=%v why=%q", got.Provisional(), got.Why)
	}
}

// A line beyond even the raised budget fails loudly rather than returning a
// partial transcript, which would be scored as a weak arm.
func TestALineBeyondTheBudgetIsAnErrorNotAPartialTranscript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "transcript.jsonl")
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

	if _, err := ReadClaudeCode(path); err == nil {
		t.Error("a transcript too large to read was reported as readable")
	}
}

// The session id is taken from the first event that carries one, and later
// events do not overwrite it: a stream that changed session mid-way would
// otherwise be correlated with the wrong log.
func TestTheSessionIDIsTakenOnceFromTheFirstEventThatHasOne(t *testing.T) {
	path := stream(t,
		`{"type":"system","subtype":"init"}`,
		`{"type":"assistant","session_id":"first","message":{"content":[]}}`,
		`{"type":"assistant","session_id":"second","message":{"content":[]}}`,
	)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}
	if got.SessionID != "first" {
		t.Errorf("session id = %q, want the first one seen", got.SessionID)
	}
}

// Blank lines between events are formatting, not content. A capture written by
// a shell redirect can acquire them, and counting one as unreadable would mark
// a perfectly good transcript provisional.
func TestBlankLinesAreNotUnreadableLines(t *testing.T) {
	path := stream(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"a.rb:1"}]}}`,
		``,
		``,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"b.rb:2"}]}}`,
	)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}

	if got.Unparseable != 0 {
		t.Errorf("counted %d unreadable lines for blank ones", got.Unparseable)
	}
	if got.Provisional() {
		t.Errorf("blank lines made the transcript provisional: %s", got.Why)
	}
	if len(got.Text) != 2 {
		t.Errorf("got %d text blocks, want 2", len(got.Text))
	}
}

// The one-line summary is what a human reads, so both halves of the usage
// question have to be legible in it.
func TestTheSummaryReportsUsageWhenThereIsSome(t *testing.T) {
	got := read(t, "claude-normal")

	if !strings.Contains(got.String(), "43 in / 114 out tokens") {
		t.Errorf("summary = %q, want the token counts", got.String())
	}
	if strings.Contains(got.String(), "PROVISIONAL") {
		t.Errorf("summary = %q, want no mark on a complete capture", got.String())
	}
}

// The condition a killed run actually meets, and the one the first three
// checks missed. A session cut off mid-stream ends on a line boundary as often
// as not: every line parses, there is plenty of text, and it simply stops.
// Scored without the mark that is a clean low result — a claim about the agent
// when the truth is a claim about the capture.
//
// The fixture is a REAL complete run with its closing event removed, which is
// exactly what a killed capture looks like.
func TestASessionCutOffMidStreamIsProvisionalEvenThoughItParses(t *testing.T) {
	full, err := os.ReadFile(filepath.Join(testdata, "claude-normal.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimRight(string(full), "\n"), "\n")
	cut := filepath.Join(t.TempDir(), "killed.jsonl")
	if err := os.WriteFile(cut, []byte(strings.Join(lines[:len(lines)-1], "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ReadClaudeCode(cut)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}

	if got.Unparseable != 0 {
		t.Fatalf("the cut capture has %d unreadable lines; it is supposed to parse cleanly", got.Unparseable)
	}
	if len(got.Text) == 0 {
		t.Fatal("the cut capture has no text; it is supposed to look complete")
	}
	if !got.Provisional() {
		t.Fatal("a session cut off mid-stream was not marked provisional")
	}
	if !strings.Contains(got.Why, "did not finish") {
		t.Errorf("why = %q, want it to say the session did not finish", got.Why)
	}
}

// And the complete capture it was cut from is NOT provisional, so the check
// means something.
func TestTheSameCaptureUncutIsNotProvisional(t *testing.T) {
	if got := read(t, "claude-normal"); got.Provisional() {
		t.Errorf("a complete capture was marked provisional: %s", got.Why)
	}
}

// A Transcript is a scorer Source, which is what stops the provisional mark
// being something every caller has to remember to copy across.
func TestATranscriptIsAScoreSourceCarryingItsOwnMark(t *testing.T) {
	var src interface {
		Answer() string
		ProvisionalWhy() string
	} = read(t, "empty-arm")

	if src.ProvisionalWhy() == "" {
		t.Error("an incomplete capture reports no reason as a score source")
	}
	if src.Answer() != "" {
		t.Errorf("answer = %q, want nothing from an empty capture", src.Answer())
	}

	whole := read(t, "claude-normal")
	if whole.ProvisionalWhy() != "" {
		t.Errorf("a complete capture reports %q as a score source", whole.ProvisionalWhy())
	}
}

// The condition every other check passes: a session the tool itself reports as
// having ended badly. It has text, it parses with no unreadable lines, and it
// closes properly, so unreadable-lines, no-events, no-close and no-text all say
// it is fine.
//
// Measured on the recorded corpus, 27 of 238 transcripts are this shape, and
// every one is on the HEADLINE arm — the arm a win is claimed on. They were
// scoring as clean results. Where the run's own record carries an exit code,
// 104 of 104 agree with this flag and none disagree.
func TestASessionTheToolReportsAsFailedIsProvisional(t *testing.T) {
	path := unfinishedStream(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"app/models/category.rb:1083"}]}}`,
		`{"type":"result","is_error":true,"subtype":"error_during_execution","session_id":"s"}`,
	)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}

	if got.Unparseable != 0 || len(got.Text) == 0 {
		t.Fatalf("the fixture is supposed to look complete: %d unreadable, %d text blocks",
			got.Unparseable, len(got.Text))
	}
	if !got.Provisional() {
		t.Fatal("a session the tool reported as failed was scored as a clean result")
	}
	if !strings.Contains(got.Why, "error_during_execution") {
		t.Errorf("why = %q, want the tool's own reason", got.Why)
	}
}

// The same shape without the error flag is a clean result, so the flag means
// something.
func TestASessionThatClosesCleanlyIsNotProvisional(t *testing.T) {
	path := unfinishedStream(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"app/models/category.rb:1083"}]}}`,
		`{"type":"result","is_error":false,"session_id":"s"}`,
	)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}
	if got.Provisional() {
		t.Errorf("a clean session was marked provisional: %s", got.Why)
	}
}

// An error with no subtype still says something rather than trailing off.
func TestAnErrorWithNoSubtypeStillGivesAReason(t *testing.T) {
	path := unfinishedStream(t,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"a.rb:1"}]}}`,
		`{"type":"result","is_error":true,"session_id":"s"}`,
	)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}
	if !got.Provisional() || !strings.Contains(got.Why, "reported an error") {
		t.Errorf("why = %q", got.Why)
	}
}

// A turn reporting only cache reads is still a turn that reported usage.
// Otherwise the flag inverts: cache reads accumulate while usage reads as
// unanswered, which is the confusion it exists to prevent.
func TestUsageCountsAsReportedWhenOnlyCacheIsCharged(t *testing.T) {
	path := stream(t,
		`{"type":"assistant","message":{"content":[],"usage":{"cache_read_input_tokens":900}}}`,
	)

	got, err := ReadClaudeCode(path)
	if err != nil {
		t.Fatalf("ReadClaudeCode: %v", err)
	}
	if !got.Usage.Reported {
		t.Error("a turn charging only cache reads reports as no usage at all")
	}
	if got.Usage.CacheRead != 900 {
		t.Errorf("cache reads = %d, want 900", got.Usage.CacheRead)
	}
}
