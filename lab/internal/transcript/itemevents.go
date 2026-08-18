package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ReadItemEvents normalizes a stream of thread, turn and item events.
//
// Named for the FORMAT rather than the vendor, like the reader beside it, and
// registered under that name so nothing picks a reader by knowing a tool. The
// names inside are the stream's own words, which also keeps a tool id out of
// the binary's symbol and file tables — the identifier probe reads those.
//
// Every shape below was read off a real capture on 2026-08-18 against
// codex-cli 0.147.0, including the failure shapes, which were produced by
// asking it for a model a ChatGPT account cannot reach. None of it is inferred
// from documentation.
func ReadItemEvents(path string) (Transcript, error) {
	f, err := os.Open(path)
	if err != nil {
		return Transcript{}, fmt.Errorf("read transcript: %w", err)
	}
	defer func() { _ = f.Close() }()
	t, err := parseItemEvents(f)
	if err != nil {
		return Transcript{}, fmt.Errorf("read transcript %s: %w", path, err)
	}
	return t, nil
}

// itemEvent is the subset of the stream this reads.
type itemEvent struct {
	Type string `json:"type"`
	// ThreadID identifies the session, and is Codex's name for it.
	ThreadID string `json:"thread_id"`
	// Message carries the text of a top-level error event.
	Message string `json:"message"`
	Error   struct {
		Message string `json:"message"`
	} `json:"error"`
	Item  streamItem `json:"item"`
	Usage turnUsage  `json:"usage"`
}

// streamItem is one thing the agent did. Its own JSON is carried whole, so the
// consumers that read a call's arguments read what the tool actually sent.
type streamItem struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Text string `json:"text"`
	// Tool is what an item that called a tool names it, measured on a real MCP
	// call: `{"type":"mcp_tool_call","server":"sense","tool":"sense_search"}`.
	// Recorded as the call's name when it is there, because "mcp_tool_call"
	// says a tool was used and not which one — and which one is the question
	// every consumer of a call actually asks.
	Tool string `json:"tool"`
}

// turnUsage is what a finished turn reports.
//
// `cached_input_tokens` is read as a SUBSET of `input_tokens` and subtracted,
// so the fresh count here means the same thing it means in the reader beside
// this one; the guard below is what keeps a stream that means something else
// from producing a negative. `reasoning_output_tokens` is likewise read as a
// subset of `output_tokens` and is not added to it, which is the shape the
// measured capture is consistent with — 14,096 input against 11,008 cached, and
// 5 output against 0 reasoning for a one-word answer.
type turnUsage struct {
	Input      int `json:"input_tokens"`
	Cached     int `json:"cached_input_tokens"`
	CacheWrite int `json:"cache_write_input_tokens"`
	Output     int `json:"output_tokens"`
}

// The events this reader acts on. The rest of the stream is skipped rather than
// modelled, for the same reason the reader beside it skips: a taxonomy is a
// thing somebody has to maintain against another project's release notes.
const (
	threadStarted = "thread.started"
	itemStarted   = "item.started"
	itemCompleted = "item.completed"
	turnCompleted = "turn.completed"
	turnFailed    = "turn.failed"
	errorEvent    = "error"
	agentMessage  = "agent_message"
)

func parseItemEvents(r io.Reader) (Transcript, error) {
	var t Transcript
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)

	var sawAny, sawClose bool
	var failed string
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e itemEvent
		if err := json.Unmarshal(line, &e); err != nil {
			t.Unparseable++
			continue
		}
		sawAny = true
		// A turn closes by completing or by failing, and both are the session
		// having finished. Only the absence of either says it was cut off.
		switch e.Type {
		case turnCompleted:
			sawClose = true
		case turnFailed:
			sawClose = true
			// The closing event first, then whatever an earlier error event
			// said: a close that carries no reason must not erase the one
			// thing in the stream that gave one.
			failed = firstNonEmpty(e.Error.Message, e.Message, failed, "the tool reported the turn failed")
		case errorEvent:
			if failed == "" {
				failed = firstNonEmpty(e.Message, "the tool reported an error")
			}
		}
		t.absorbItemEvent(e, line)
	}
	if err := sc.Err(); err != nil {
		return Transcript{}, err
	}

	mark(&t, sawAny, sawClose, failed)
	return t, nil
}

func (t *Transcript) absorbItemEvent(e itemEvent, line []byte) {
	if e.Type == threadStarted && t.SessionID == "" {
		t.SessionID = e.ThreadID
	}
	if e.Type == turnCompleted {
		t.addUsage(usage{
			Input:         freshInput(e.Usage),
			Output:        e.Usage.Output,
			CacheRead:     e.Usage.Cached,
			CacheCreation: e.Usage.CacheWrite,
		})
	}
	// A CALL is an item that was STARTED, and that is a rule about the stream
	// rather than a list of tool names. Measured: what the agent says and what
	// it reports as an error arrive completed and never started, and the work it
	// runs arrives started and then completed. Counting starts therefore counts
	// each piece of work once, and still counts one the wall cut off before it
	// could complete — which is the case a completed-only rule would drop, and
	// exactly the case worth seeing.
	//
	// A list of tool types here would be this package's own taxonomy of somebody
	// else's product, wrong the first time they ship a new kind of item, and
	// wrong by undercounting.
	if e.Type == itemStarted {
		t.Calls = append(t.Calls, Call{Name: callName(e.Item), Input: json.RawMessage(clone(line))})
		return
	}
	if e.Type == itemCompleted && e.Item.Type == agentMessage {
		t.Text = append(t.Text, e.Item.Text)
	}
}

// callName is what to call this piece of work: the tool it named, or the kind
// of item it is when it named none. A shell command names no tool and its kind
// is the most specific thing there is to say about it.
func callName(it streamItem) string {
	if it.Tool != "" {
		return it.Tool
	}
	return it.Type
}

// freshInput is the input the turn paid for, with the cached part taken out.
// The guard is what happens if a future stream reports the two separately: a
// negative fresh count would be worse than a large one.
func freshInput(u turnUsage) int {
	if u.Cached > u.Input {
		return u.Input
	}
	return u.Input - u.Cached
}

// clone copies a scanner's line, which is only valid until the next Scan.
func clone(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

// firstNonEmpty is the first of these that says something.
func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}
