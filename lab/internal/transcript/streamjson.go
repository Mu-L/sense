package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// maxLine is the per-line budget. A single assistant message can be enormous —
// a recorded run produced a 52,000-character answer in one event — and a
// silently truncated line is a silently lowered score, which is the worst
// failure this package can have.
const maxLine = 64 << 20

// ReadClaudeCode normalizes Claude Code's stream-json output.
//
// The file is named for the FORMAT rather than the vendor, because that is what
// varies: cycle 08 adds normalizers for two more tools, each reading its own
// format, one file each. It also keeps the vendor's name out of the binary's
// file table, which the catalog-identifier check in 01-02 flags — though the
// exported symbol carries it either way, so that is a side effect and not the
// reason.
//
// It reads four event shapes and skips the rest. Every streaming format has a
// dozen event kinds and this needs about four of them; the others are not
// modelled, because a taxonomy is a thing that has to be maintained against
// somebody else's release notes forever.
func ReadClaudeCode(path string) (Transcript, error) {
	f, err := os.Open(path)
	if err != nil {
		return Transcript{}, fmt.Errorf("read transcript: %w", err)
	}
	defer func() { _ = f.Close() }()
	t, err := parseClaudeCode(f)
	if err != nil {
		return Transcript{}, fmt.Errorf("read transcript %s: %w", path, err)
	}
	return t, nil
}

func parseClaudeCode(r io.Reader) (Transcript, error) {
	var t Transcript
	sc := bufio.NewScanner(r)
	// A small start buffer, grown as needed, so maxLine is what actually bounds
	// a line. A large start buffer would silently become the floor and the
	// constant would never be the thing under test.
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)

	var sawAny, sawResult bool
	var failed string
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e event
		if err := json.Unmarshal(line, &e); err != nil {
			t.Unparseable++
			continue
		}
		sawAny = true
		if e.Type == "result" {
			sawResult = true
			if e.IsError {
				failed = e.Subtype
				if failed == "" {
					failed = "the tool reported an error"
				}
			}
		}
		t.absorb(e)
	}
	if err := sc.Err(); err != nil {
		return Transcript{}, err
	}

	mark(&t, sawAny, sawResult, failed)
	return t, nil
}

// event is the subset of the stream this reads. Fields the consumers do not
// need are absent rather than carried.
type event struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	// IsError is set on the closing result event when the session ended badly.
	// It is the condition a wall-killed session that REACHED its close meets,
	// which is the common shape and which every other check here misses: such a
	// capture has text, parses cleanly and ends properly.
	IsError bool   `json:"is_error"`
	Subtype string `json:"subtype"`
	// Usage on the RESULT event, which is where the tools behind three of the
	// arms report it — 117 of 238 recorded transcripts carry usage here and
	// nowhere else. Reading only the assistant turns says "the tool never told
	// us" about half the corpus, which is exactly the confusion Usage.Reported
	// exists to prevent.
	Usage   usage `json:"usage"`
	Message struct {
		Usage   usage `json:"usage"`
		Content []struct {
			Type  string          `json:"type"`
			Text  string          `json:"text"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
}

// usage is the shape both the assistant turns and the closing result event use.
type usage struct {
	Input         int `json:"input_tokens"`
	Output        int `json:"output_tokens"`
	CacheRead     int `json:"cache_read_input_tokens"`
	CacheCreation int `json:"cache_creation_input_tokens"`
}

func (t *Transcript) absorb(e event) {
	if e.SessionID != "" && t.SessionID == "" {
		t.SessionID = e.SessionID
	}
	// The result event carries a session TOTAL rather than a turn, so taking it
	// as well would double-count anything already summed from the turns. It is
	// read only when the turns said nothing, which is how three of the arms
	// report.
	if e.Type == "result" {
		if !t.Usage.Reported {
			t.addUsage(e.Usage)
		}
		return
	}
	if e.Type != "assistant" {
		return
	}
	// Reported per assistant turn, and accumulates across the session. Any of
	// the four counts as an answer: a turn reporting only cache reads would
	// otherwise accumulate them while reading as unreported, which inverts the
	// confusion the flag exists to prevent.
	t.addUsage(e.Message.Usage)

	for _, c := range e.Message.Content {
		switch c.Type {
		case "text":
			t.Text = append(t.Text, c.Text)
		case "tool_use":
			t.Calls = append(t.Calls, Call{Name: c.Name, Input: c.Input})
		}
		// thinking and tool_result are deliberately skipped. A tool result is
		// what the agent was TOLD, not what it said, and crediting it would
		// score the tool rather than the answer.
	}
}

// mark decides whether the transcript can be trusted whole.
//
// A provisional transcript is not a diagnostic tucked into a field: it is a
// property that travels with every number derived from it. The failure this
// prevents is a truncated capture scoring low and being read as a weak arm.
func (t *Transcript) addUsage(u usage) {
	if u.Input == 0 && u.Output == 0 && u.CacheRead == 0 && u.CacheCreation == 0 {
		return
	}
	t.Usage.Reported = true
	t.Usage.Input += u.Input
	t.Usage.Output += u.Output
	t.Usage.CacheRead += u.CacheRead
	t.Usage.CacheCreation += u.CacheCreation
}

// mark is a function rather than a method, because the answer depends on state
// the transcript does not hold: whether anything was read at all, and whether
// the stream closed. A method that can only be called correctly from one loop
// is a method pretending to be one.
func mark(t *Transcript, sawAny, sawResult bool, failed string) {
	switch {
	case !sawAny:
		t.Why = "no readable events at all"
	case t.Unparseable > 0:
		t.Why = fmt.Sprintf("%d unreadable lines", t.Unparseable)
	case !sawResult:
		// The condition a killed run actually meets. A session cut off
		// mid-stream ends on a line boundary as often as not, so every line
		// parses and there is plenty of text — it just stops. All 238 recorded
		// transcripts carry a closing result event, so its absence means the
		// session did not finish, and without this check such a capture scores
		// as a clean low result.
		t.Why = "the session did not finish; there is no closing result event"
	case failed != "":
		// Measured across the recorded corpus: 27 of 238 transcripts carry
		// is_error, have text, parse with no unreadable lines AND close
		// properly, so every other check here passes them. Where the run
		// record also exists, agreement is exact — 25 of 25 non-zero exits
		// carry it and 79 of 79 clean exits do not.
		t.Why = "the tool reported the session ended badly: " + failed
	case len(t.Text) == 0:
		// A session that called tools and never said anything cannot be scored
		// on what it said, and reporting that as recall 0.00 would be a claim
		// about the agent rather than about the capture.
		t.Why = "the agent said nothing; the capture may be truncated"
	}
}
