package score

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// AnswerText reads a captured stream-json stream and returns everything the
// agent said, concatenated in order.
//
// It parses the agent's own output directly rather than through a normalized
// transcript type, because inventing that type here means inventing it twice:
// it is cycle 02's job, with the whole recorded corpus available to check
// against.
//
// Only assistant text is returned. Tool inputs and results are excluded on
// purpose: a citation the agent merely read in a grep result is not a citation
// the agent made, and counting one would score the tool rather than the answer.
func AnswerText(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("read transcript: %w", err)
	}
	defer func() { _ = f.Close() }()

	var b strings.Builder
	var sawAnswer bool
	sc := bufio.NewScanner(f)
	// A single assistant message can be far longer than the default 64KB line
	// budget; the live probe run produced a 52,000-character final answer in
	// one event, and a silently truncated line is a silently lowered score.
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)

	for sc.Scan() {
		if appendText(&b, sc.Bytes()) {
			sawAnswer = true
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("read transcript %s: %w", path, err)
	}
	// A transcript carrying no assistant event at all is a broken instrument,
	// not a silent agent: a truncated capture, the wrong file, or a stream this
	// reader no longer understands. Returning "" would score 0.00 and print
	// "below floor", making a broken run indistinguishable from a weak arm.
	if !sawAnswer {
		return "", fmt.Errorf("read transcript %s: no assistant output in it at all", path)
	}
	return b.String(), nil
}

// event is the subset of a stream-json line this reads. Anything else in the
// stream is ignored rather than modelled.
type event struct {
	Type    string `json:"type"`
	Result  string `json:"result"`
	Message struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"message"`
}

// appendText adds one event's assistant text to b and reports whether the line
// was an answer-bearing event. A line that is not JSON, or carries nothing this
// reads, contributes nothing: the stream is a vendor format and being strict
// about parts we do not consume would turn a new event kind into a failed
// score. Being strict about finding NO answer at all is a different matter, and
// AnswerText does that.
func appendText(b *strings.Builder, line []byte) bool {
	var e event
	if err := json.Unmarshal(line, &e); err != nil {
		return false
	}
	switch e.Type {
	case "assistant":
		var wrote bool
		for _, c := range e.Message.Content {
			if c.Type == "text" {
				b.WriteString(c.Text)
				b.WriteString("\n")
				wrote = true
			}
		}
		return wrote
	case "result":
		// The final result repeats the last assistant message. It is included
		// because a session killed on its wall can leave the answer here and
		// nowhere else, and Citations already discards duplicates.
		b.WriteString(e.Result)
		b.WriteString("\n")
		return true
	}
	return false
}
