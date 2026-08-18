package transcript

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// ReadMessageParts normalizes a stream of message parts: one event per part of
// the assistant's message, each carrying the part inside it. It is the shape
// OpenCode writes with `run --format json`.
//
// Named for the FORMAT rather than the vendor, like the readers beside it. The
// shapes were read off real captures on 2026-08-18 against opencode 1.18.18,
// none of them inferred from documentation.
func ReadMessageParts(path string) (Transcript, error) {
	f, err := os.Open(path)
	if err != nil {
		return Transcript{}, fmt.Errorf("read transcript: %w", err)
	}
	defer func() { _ = f.Close() }()
	t, err := parseMessageParts(f)
	if err != nil {
		return Transcript{}, fmt.Errorf("read transcript %s: %w", path, err)
	}
	return t, nil
}

// partEvent is the subset of the stream this reads. Every event wraps one part
// and repeats the session on the outside, which is why the id is read there.
type partEvent struct {
	Type      string `json:"type"`
	SessionID string `json:"sessionID"`
	// Error is a top-level failure event, which is how this format reports a
	// session that could not run at all: measured, a bad model id produces one
	// error event and nothing else — no step, no text, no close.
	Error struct {
		Name string `json:"name"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	} `json:"error"`
	Part struct {
		Type string `json:"type"`
		Text string `json:"text"`
		// Tool is the name of the tool a tool part called.
		Tool string `json:"tool"`
		// CallID distinguishes one call from the same call reported again as it
		// moves through its states.
		CallID string `json:"callID"`
		// Reason says why a step finished, and it is read for one thing only:
		// whether the model stopped talking, which is what closes the session
		// here. It is deliberately NOT read as a verdict — a measured capture
		// ends its intermediate steps with `tool-calls` and its last with
		// `stop`, both of them normal, and a rule that treated every reason but
		// one as a failure marked a clean session as broken. Which other
		// reasons mean trouble is somebody else's taxonomy.
		Reason string `json:"reason"`
		Tokens struct {
			Input     int `json:"input"`
			Output    int `json:"output"`
			Reasoning int `json:"reasoning"`
			Cache     struct {
				Read  int `json:"read"`
				Write int `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
	} `json:"part"`
}

// The events this reader acts on. The rest of the stream is skipped rather than
// modelled, for the same reason the readers beside it skip: a taxonomy is a
// thing somebody has to maintain against another project's release notes.
const (
	partText       = "text"
	partToolUse    = "tool_use"
	partStepFinish = "step_finish"
	partError      = "error"
	// stopReason is the reason a final step gives: the model stopped talking.
	stopReason = "stop"
)

func parseMessageParts(r io.Reader) (Transcript, error) {
	var t Transcript
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)

	var sawAny, sawClose bool
	var failed string
	// A tool part is reported again each time the call changes state, so the
	// calls are counted by their own ids. Counting events would multiply every
	// tool call in every cost comparison by however many states it passed
	// through.
	seen := map[string]bool{}
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e partEvent
		if err := json.Unmarshal(line, &e); err != nil {
			t.Unparseable++
			continue
		}
		sawAny = true
		switch e.Type {
		case partStepFinish:
			// A step closes the SESSION only when the model stopped talking.
			// This format reports a step per round trip, so an intermediate
			// step that ended to run tools is not a close — and reading it as
			// one made a capture cut off mid-session read as a finished one,
			// which is the silent divergence the conformance suite exists to
			// catch. Measured: `tool-calls` on every intermediate step and
			// `stop` on the last, in both recorded captures.
			if e.Part.Reason == stopReason {
				sawClose = true
			}
		case partError:
			// An error event is also a close: the session ended, badly and on
			// purpose, and without this it would be marked as one that was cut
			// off — a different failure with a different diagnosis.
			sawClose = true
			failed = "the tool reported an error"
			if said := firstNonEmpty(e.Error.Data.Message, e.Error.Name); said != "" {
				failed += ": " + said
			}
		}
		t.absorbPart(e, line, seen)
	}
	if err := sc.Err(); err != nil {
		return Transcript{}, err
	}

	mark(&t, sawAny, sawClose, failed)
	return t, nil
}

func (t *Transcript) absorbPart(e partEvent, line []byte, seen map[string]bool) {
	if e.SessionID != "" && t.SessionID == "" {
		t.SessionID = e.SessionID
	}
	switch e.Type {
	case partStepFinish:
		// Reported per step and accumulated across the session, like the
		// assistant turns in the reader beside this one. `input` is the fresh
		// count here: the cache figures are reported separately rather than
		// inside it, which is the opposite of the item-events format and the
		// reason neither reader is shared with the other.
		t.addUsage(usage{
			Input:         e.Part.Tokens.Input,
			Output:        e.Part.Tokens.Output,
			CacheRead:     e.Part.Tokens.Cache.Read,
			CacheCreation: e.Part.Tokens.Cache.Write,
		})
	case partText:
		t.Text = append(t.Text, e.Part.Text)
	case partToolUse:
		if id := e.Part.CallID; id == "" || !seen[id] {
			seen[id] = true
			t.Calls = append(t.Calls, Call{Name: e.Part.Tool, Input: json.RawMessage(clone(line))})
		}
	}
}
