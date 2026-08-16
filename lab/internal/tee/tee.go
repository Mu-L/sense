// Package tee is the bench's transparent capture of the MCP traffic between an
// agent and the Sense server.
//
// It exists because the agent tool's own output says what the agent said, not
// what Sense told it. Without the other half, a losing sense arm is
// unattributable: Sense returned nothing, returned the wrong thing, returned
// the right thing and the agent ignored it, or was never called. Four findings,
// four different fixes, and three of them are product findings.
//
// # Why this is a process
//
// The pitch left it open whether the capture could live inside the binary that
// owns the session. It cannot. The agent CLI spawns the MCP server itself, from
// the `.mcp.json` registration in the repository, so the pipe carrying that
// traffic exists between two of its own processes and the supervisor never
// holds either end. The only place to stand is where the registration points,
// which means a command the registration names. The cost is one process in the
// tree, started once per session and idle between frames.
//
// # The contract
//
// Two properties, inherited deliberately from the shim this replaces:
//
//   - byte-transparent: what reaches the client is exactly what the server
//     wrote. A frame that fails to parse is forwarded verbatim and logged raw.
//     The shim never gates or repairs traffic.
//   - fail-open: capture is telemetry, never a run dependency. A log that
//     cannot be opened or written disables logging and the session continues. A
//     run killed by its own telemetry is a self-inflicted burned cell.
//
// And one the shim did not have: capture never blocks the session. A failed
// write is loud; a blocked write is silent and applies backpressure to a
// session running against a wall, so the instrument alters what it measures.
// When the sink cannot keep up, frames are dropped and counted.
package tee

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"sync/atomic"
	"time"
)

// Direction is which way a frame was travelling.
const (
	// ClientToServer is the agent asking Sense something.
	ClientToServer = "c2s"
	// ServerToClient is Sense answering.
	ServerToClient = "s2c"
)

// entry is one line of the capture log. The shape is the one the corpus already
// holds, so recorded runs and new ones read the same.
type entry struct {
	TS  string          `json:"ts"`
	Dir string          `json:"dir"`
	Msg json.RawMessage `json:"msg"`
}

// Sink takes a copy of every frame. It must never block: Pump calls it on the
// path the session's own traffic travels. A nil Sink means no capture, which is
// what a session with no log configured gets.
type Sink interface {
	Record(dir string, frame []byte)
}

// Pump forwards src to dst frame by frame, handing a copy of each to the sink.
//
// MCP stdio framing is newline-delimited JSON, so a frame is a line. It is read
// with an unbounded reader rather than a scanner: a tool result carrying a large
// answer is exactly the frame whose value shows up months later, and a scanner
// would quietly truncate it at its buffer size.
//
// The bytes written to dst are the bytes read from src, including the newline.
// Nothing is re-encoded, so a frame the shim cannot parse still arrives intact.
func Pump(dst io.Writer, src io.Reader, dir string, sink Sink) error {
	r := bufio.NewReader(src)
	for {
		line, err := r.ReadBytes('\n')
		if len(line) > 0 {
			if _, werr := dst.Write(line); werr != nil {
				return werr
			}
			if sink != nil {
				sink.Record(dir, line)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
	}
}

// Status is what capture managed to do, so a run can say whether its record is
// complete rather than leaving a reader to guess from a file size.
type Status struct {
	// Capturing is false once logging has been given up on.
	Capturing bool `json:"capturing"`
	// Frames is how many were written.
	Frames int `json:"frames"`
	// Dropped is how many were discarded because the sink could not keep up.
	// Non-zero means the capture is incomplete and says so.
	Dropped int `json:"dropped"`
	// Reason is why capture stopped, when it did.
	Reason string `json:"reason,omitempty"`
}

// Complete reports whether the capture is a full record of the session.
func (s Status) Complete() bool { return s.Capturing && s.Dropped == 0 && s.Reason == "" }

// queue is how many frames may be waiting to be written. Deep enough that a
// burst of tool results does not reach the drop path on an ordinary disk,
// shallow enough that a wedged writer is noticed within one session.
const queue = 1024

// Log is a Sink that writes frames to an io.Writer from its own goroutine.
//
// The goroutine is the whole point. The session's traffic is handed to a
// channel and the caller returns immediately, so a slow or wedged writer costs
// dropped frames rather than a stalled session running against a wall.
type Log struct {
	frames  chan entry
	done    chan struct{}
	now     func() time.Time
	written int
	// dropped is touched by every pump goroutine, so it is atomic. written and
	// stopped belong to the writer goroutine and are read only after it has
	// finished.
	dropped atomic.Int64
	stopped string
	// state is read by Status after Close, so it is only ever touched by the
	// writer goroutine before then.
	w io.Writer
}

// NewLog starts a log writing to w. now supplies the timestamp; nil means the
// wall clock. Close stops it and reports what it managed.
func NewLog(w io.Writer, now func() time.Time) *Log {
	if now == nil {
		now = time.Now
	}
	l := &Log{
		frames: make(chan entry, queue),
		done:   make(chan struct{}),
		now:    now,
		w:      w,
	}
	go l.write()
	return l
}

// Record hands a frame to the writer, or drops it. It never blocks, and it
// never returns an error: there is nothing a session could usefully do about a
// capture problem, and the one thing it must not do is stop.
func (l *Log) Record(dir string, frame []byte) {
	e := entry{
		TS:  l.now().UTC().Format(time.RFC3339Nano),
		Dir: dir,
		Msg: message(frame),
	}
	if e.Msg == nil {
		return
	}
	select {
	case l.frames <- e:
	default:
		l.dropped.Add(1)
	}
}

// message turns a raw frame into the JSON the log records: the frame itself
// when it parses, and a marked copy of the text when it does not. A blank line
// is framing, not a frame, and is not recorded.
func message(frame []byte) json.RawMessage {
	text := trimEOL(frame)
	if len(text) == 0 {
		return nil
	}
	if json.Valid(text) {
		return json.RawMessage(text)
	}
	// The shim never repairs traffic. It records what went past, marked as
	// something that was not JSON, and forwards the bytes untouched.
	//
	// Marshalling a string cannot fail: invalid UTF-8 is replaced rather than
	// rejected, so there is no error branch to write here.
	quoted, _ := json.Marshal(string(text))
	return json.RawMessage(`{"_unparsed":` + string(quoted) + `}`)
}

func trimEOL(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}

func (l *Log) write() {
	defer close(l.done)
	enc := json.NewEncoder(l.w)
	for e := range l.frames {
		if l.stopped != "" {
			continue // drain, so Record never blocks after a write failure
		}
		if err := enc.Encode(e); err != nil {
			// Capture is telemetry. Give up on it and let the session run.
			l.stopped = err.Error()
			continue
		}
		l.written++
	}
}

// Close stops the writer and reports what capture managed.
func (l *Log) Close() Status {
	close(l.frames)
	<-l.done
	return Status{
		Capturing: l.stopped == "",
		Frames:    l.written,
		Dropped:   int(l.dropped.Load()),
		Reason:    l.stopped,
	}
}
