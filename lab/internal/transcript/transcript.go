// Package transcript reads an agent tool's own output into one shape.
//
// Scoring, judging, mining and cost accounting all need the same facts out of a
// session. Each one re-deriving them from a vendor's streaming format is four
// places to get it wrong, and the corpus is only usable once something can read
// it all into a stable form.
//
// This type is NOT a faithful model of any tool's event stream. It is the set
// of facts the consumers need. If a consumer needs something new it is added
// deliberately, and the recorded corpus is re-normalized to prove it was always
// there.
//
// The raw stream is kept forever, byte for byte. This form is derived and
// disposable, so a normalizer bug is fixable by re-normalizing rather than by
// re-running, and that property is worth more than any field here.
package transcript

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Transcript is one session, in the shape its consumers read.
//
// Cost and wall time are deliberately absent, and the reason is not that they
// would be expensive to carry — the raw stream is kept forever, so a field
// added later costs a re-normalize rather than a re-run. It is that their SHAPE
// would be guessed: priced tokens, cache tiers and wall attribution are
// decisions cost accounting has to make, and a type invented before those
// decisions would have to be unmade. They join with the code that reads them.
type Transcript struct {
	// SessionID correlates this run with the agent tool's own logs.
	SessionID string
	// Text is what the agent said, in order. Citations live here.
	Text []string
	// Calls are the tool calls, in the order they happened.
	Calls []Call
	Usage Usage

	// Unparseable is how many lines could not be read as events.
	//
	// It is not a diagnostic nobody looks at: any transcript with a count above
	// zero is Provisional, and a score derived from one carries that forward.
	// Counting unreadable lines into a field nobody reads, while a clean-looking
	// number flows onward, is the same failure as not counting them at all.
	Unparseable int
	// Why says what makes this transcript provisional, and empty means it is
	// not. One field rather than a bool beside it: the two would have to agree
	// forever, nothing could enforce that, and the first place that set one
	// without the other would be a silent bug in the mark that exists to stop
	// silent bugs.
	Why string
}

// Provisional means this transcript is known incomplete, so anything derived
// from it is a provisional result rather than a clean low one.
func (t Transcript) Provisional() bool { return t.Why != "" }

// ProvisionalWhy satisfies the scorer's Source: it takes a transcript rather
// than its text precisely so this cannot be dropped on the way in.
func (t Transcript) ProvisionalWhy() string { return t.Why }

// Call is one tool invocation.
type Call struct {
	Name string
	// Input is the argument object exactly as the tool sent it, undecoded.
	// RawMessage rather than a map says "carried, not interpreted" in the type
	// itself: a decoded map invites a type assertion, and whether a call
	// reached Sense and what it asked is the miner's question in cycle 06, not
	// this package's.
	Input json.RawMessage
}

// Usage is what the session consumed. Cache reads and cache creation are
// separate because they are priced differently, and an accounting that folds
// them together cannot be checked against a bill.
type Usage struct {
	Input         int
	Output        int
	CacheRead     int
	CacheCreation int
	// Reported says the tool told us. Measured across the recorded corpus, 16
	// of 238 transcripts carry no usage anywhere.
	//
	// Without this flag a caller reads zero tokens as a cheap session rather
	// than as an unanswered question, which is the same failure as swallowing
	// an unreadable line. The flag was itself wrong once, and expensively: it
	// read only the assistant turns, so it reported "the tool never told us"
	// for the 117 transcripts that carry usage on the closing event instead.
	Reported bool
}

// Answer is everything the agent said, joined. It is what a scorer reads.
func (t Transcript) Answer() string { return strings.Join(t.Text, "\n") }

// String is the one-line summary a human reads. It leads with the provisional
// mark, because the mark changes what every figure after it means.
func (t Transcript) String() string {
	var b strings.Builder
	if t.Provisional() {
		fmt.Fprintf(&b, "PROVISIONAL (%s) ", t.Why)
	}
	fmt.Fprintf(&b, "%d text blocks, %d tool calls", len(t.Text), len(t.Calls))
	if t.Usage.Reported {
		fmt.Fprintf(&b, ", %d in / %d out tokens", t.Usage.Input, t.Usage.Output)
	} else {
		b.WriteString(", usage not reported")
	}
	return b.String()
}
