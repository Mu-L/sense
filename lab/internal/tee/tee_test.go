package tee_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/tee"
)

// frozen is a clock that never moves, so a recorded frame is compared on what
// it carries rather than on when the test ran.
func frozen() func() time.Time {
	at := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return at }
}

// captured runs the frames through a log and returns what was written.
func captured(t *testing.T, dir string, frames ...string) []map[string]any {
	t.Helper()
	var out bytes.Buffer
	log := tee.NewLog(&out, frozen())
	for _, f := range frames {
		log.Record(dir, []byte(f))
	}
	if status := log.Close(); !status.Complete() {
		t.Fatalf("capture did not complete: %+v", status)
	}
	return decode(t, out.String())
}

func decode(t *testing.T, jsonl string) []map[string]any {
	t.Helper()
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(jsonl), "\n") {
		if line == "" {
			continue
		}
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("capture line %q is not JSON: %v", line, err)
		}
		entries = append(entries, e)
	}
	return entries
}

func TestWhatReachesTheClientIsExactlyWhatTheServerWrote(t *testing.T) {
	// Byte-transparency is the contract. A shim that re-encodes is a shim that
	// can change an answer, and nothing about the run would show it.
	said := "{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"text\":\"a\\u00e9b\"}}\n" +
		"{ \"jsonrpc\" : \"2.0\" , \"id\" : 2 }\n"
	var toClient bytes.Buffer

	if err := tee.Pump(&toClient, strings.NewReader(said), tee.ServerToClient, nil); err != nil {
		t.Fatalf("Pump: %v", err)
	}

	if toClient.String() != said {
		t.Errorf("the client received:\n%q\nthe server wrote:\n%q", toClient.String(), said)
	}
}

func TestAFrameThatIsNotJSONStillReachesTheClientAndIsLoggedRaw(t *testing.T) {
	// The shim never gates or repairs traffic. A server that printed a warning
	// onto its output stream has broken the protocol, and hiding that from the
	// client would turn a diagnosable failure into a mystery.
	broken := "sense: warning, index is stale\n"
	var toClient bytes.Buffer
	var out bytes.Buffer
	log := tee.NewLog(&out, frozen())

	if err := tee.Pump(&toClient, strings.NewReader(broken), tee.ServerToClient, log); err != nil {
		t.Fatalf("Pump: %v", err)
	}
	log.Close()

	if toClient.String() != broken {
		t.Errorf("the client received %q, want %q", toClient.String(), broken)
	}
	entries := decode(t, out.String())
	if len(entries) != 1 {
		t.Fatalf("logged %d entries, want 1", len(entries))
	}
	msg, ok := entries[0]["msg"].(map[string]any)
	if !ok || msg["_unparsed"] != "sense: warning, index is stale" {
		t.Errorf("logged %v, want the unparsed text recorded verbatim", entries[0]["msg"])
	}
}

func TestBothDirectionsAreRecordedAndTold(t *testing.T) {
	// Without the direction, a losing arm cannot be attributed: "Sense returned
	// nothing" and "the agent never asked" are the same file.
	asked := captured(t, tee.ClientToServer, `{"method":"tools/call"}`)
	answered := captured(t, tee.ServerToClient, `{"result":{}}`)

	if asked[0]["dir"] != "c2s" {
		t.Errorf("a request was recorded as %v", asked[0]["dir"])
	}
	if answered[0]["dir"] != "s2c" {
		t.Errorf("a response was recorded as %v", answered[0]["dir"])
	}
}

func TestALargeResponseIsRecordedWholeRatherThanTruncated(t *testing.T) {
	// A scanner's default buffer is 64KB, and a tool result carrying a real
	// answer is exactly the frame whose value shows up months later. The whole
	// point of the file is the question nobody anticipated.
	body := strings.Repeat("x", 512*1024)
	frame := `{"jsonrpc":"2.0","id":1,"result":{"content":"` + body + `"}}`
	var toClient bytes.Buffer
	var out bytes.Buffer
	log := tee.NewLog(&out, frozen())

	if err := tee.Pump(&toClient, strings.NewReader(frame+"\n"), tee.ServerToClient, log); err != nil {
		t.Fatalf("Pump: %v", err)
	}
	log.Close()

	if toClient.Len() != len(frame)+1 {
		t.Errorf("the client received %d bytes, want %d", toClient.Len(), len(frame)+1)
	}
	entries := decode(t, out.String())
	if len(entries) != 1 {
		t.Fatalf("logged %d entries, want 1", len(entries))
	}
	msg, _ := entries[0]["msg"].(map[string]any)
	result, _ := msg["result"].(map[string]any)
	if content, _ := result["content"].(string); len(content) != len(body) {
		t.Errorf("the recorded response holds %d bytes of content, want %d", len(content), len(body))
	}
}

func TestAFrameWithNoTrailingNewlineIsStillForwardedAndRecorded(t *testing.T) {
	// A server killed mid-write leaves a frame with no terminator. Dropping it
	// would lose the last thing Sense said before the session ended, which is
	// the most interesting frame in the file.
	last := `{"jsonrpc":"2.0","id":9}`
	var toClient bytes.Buffer
	var out bytes.Buffer
	log := tee.NewLog(&out, frozen())

	if err := tee.Pump(&toClient, strings.NewReader(last), tee.ServerToClient, log); err != nil {
		t.Fatalf("Pump: %v", err)
	}
	log.Close()

	if toClient.String() != last {
		t.Errorf("the client received %q, want %q", toClient.String(), last)
	}
	if entries := decode(t, out.String()); len(entries) != 1 {
		t.Errorf("logged %d entries, want the final frame recorded", len(entries))
	}
}

func TestBlankLinesAreFramingRatherThanFrames(t *testing.T) {
	var out bytes.Buffer
	log := tee.NewLog(&out, frozen())
	var toClient bytes.Buffer

	if err := tee.Pump(&toClient, strings.NewReader("\n\n"), tee.ServerToClient, log); err != nil {
		t.Fatalf("Pump: %v", err)
	}
	log.Close()

	if toClient.String() != "\n\n" {
		t.Errorf("the client received %q; framing must pass through too", toClient.String())
	}
	if out.Len() != 0 {
		t.Errorf("logged %q, want nothing", out.String())
	}
}

func TestCaptureNeverBlocksTheSession(t *testing.T) {
	// A failed write is loud; a blocked write is silent and applies backpressure
	// to a session running against a wall, so the instrument alters what it
	// measures. Capture degrades instead.
	log := tee.NewLog(newGated(), frozen())
	frame := []byte(`{"jsonrpc":"2.0","id":1}`)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 20000; i++ {
			log.Record(tee.ServerToClient, frame)
		}
	}()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the session was blocked by its own capture")
	}
}

func TestAWedgedWriterIsReportedAsAnIncompleteCapture(t *testing.T) {
	// Degrading silently would be worse than blocking: a partial file that
	// nothing marks as partial reads as "Sense was never called".
	stuck := newGated()
	log := tee.NewLog(stuck, frozen())
	for i := 0; i < 20000; i++ {
		log.Record(tee.ServerToClient, []byte(`{"jsonrpc":"2.0","id":1}`))
	}
	stuck.release()

	status := log.Close()

	if status.Dropped == 0 {
		t.Fatal("a writer far slower than the session dropped nothing; capture was applying backpressure")
	}
	if status.Complete() {
		t.Errorf("status %+v reports a complete capture although frames were dropped", status)
	}
}

func TestALogThatCannotBeWrittenGivesUpRatherThanFailingTheSession(t *testing.T) {
	// Capture is telemetry, never a run dependency. A run killed by its own
	// telemetry is a self-inflicted burned cell.
	log := tee.NewLog(failing{}, frozen())

	log.Record(tee.ServerToClient, []byte(`{"id":1}`))
	log.Record(tee.ServerToClient, []byte(`{"id":2}`))
	status := log.Close()

	if status.Capturing {
		t.Error("the log still reports itself as capturing after a write failure")
	}
	if status.Reason == "" {
		t.Error("the log gave up without saying why")
	}
	if status.Complete() {
		t.Error("a capture that failed reports itself complete")
	}
}

func TestPumpReportsAClientItCanNoLongerWriteTo(t *testing.T) {
	// The forwarding path is not telemetry. If the client's end is gone, the
	// session is over and saying so is how the shim exits.
	err := tee.Pump(failing{}, strings.NewReader("{\"id\":1}\n"), tee.ServerToClient, nil)

	if err == nil {
		t.Fatal("Pump reported success although the client could not be written to")
	}
}

func TestPumpReportsAServerItCanNoLongerReadFrom(t *testing.T) {
	err := tee.Pump(io.Discard, brokenReader{}, tee.ServerToClient, nil)

	if err == nil {
		t.Fatal("Pump reported success although the stream failed")
	}
}

func TestWithNoSinkTheTrafficStillPassesThrough(t *testing.T) {
	// No log configured is the fail-open case in its strongest form, and
	// forwarding must not depend on capture being on.
	said := "{\"id\":1}\n"
	var toClient bytes.Buffer

	if err := tee.Pump(&toClient, strings.NewReader(said), tee.ServerToClient, nil); err != nil {
		t.Fatalf("Pump: %v", err)
	}
	if toClient.String() != said {
		t.Errorf("the client received %q, want %q", toClient.String(), said)
	}
}

func TestALogWithNoClockUsesTheWallClock(t *testing.T) {
	var out bytes.Buffer
	log := tee.NewLog(&out, nil)

	log.Record(tee.ServerToClient, []byte(`{"id":1}`))
	log.Close()

	entries := decode(t, out.String())
	ts, _ := entries[0]["ts"].(string)
	at, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		t.Fatalf("recorded ts %q is not a timestamp: %v", ts, err)
	}
	if time.Since(at) > time.Minute {
		t.Errorf("recorded ts = %v, want roughly now", at)
	}
}

func TestARecordedFrameCarriesWhenItWentPast(t *testing.T) {
	entries := captured(t, tee.ServerToClient, `{"id":1}`)

	if ts, _ := entries[0]["ts"].(string); !strings.HasPrefix(ts, "2026-08-16T12:00:00") {
		t.Errorf("recorded ts = %v, want the frame's own time", entries[0]["ts"])
	}
}

// gated is a disk that has stopped answering: every write blocks until the test
// lets it go. It stands in for a wedged filesystem without a sleep, so the
// backpressure question is answered by the shape of the code rather than by a
// timing guess.
type gated struct {
	open chan struct{}
	once sync.Once
}

func newGated() *gated { return &gated{open: make(chan struct{})} }

func (g *gated) Write(p []byte) (int, error) {
	<-g.open
	return len(p), nil
}

func (g *gated) release() { g.once.Do(func() { close(g.open) }) }

// failing is a writer whose every write fails.
type failing struct{}

func (failing) Write([]byte) (int, error) { return 0, errors.New("no space left on device") }

// brokenReader is a stream that fails rather than ending.
type brokenReader struct{}

func (brokenReader) Read([]byte) (int, error) { return 0, errors.New("connection reset") }
