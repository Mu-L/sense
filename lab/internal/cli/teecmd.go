package cli

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/luuuc/sense/lab/internal/tee"
)

// `sense-lab tee` is not a verb anybody types. It is the command a run's
// `.mcp.json` points at instead of the Sense server, so that the traffic
// between the agent and Sense passes through something that can record it. It
// is left out of the usage text for that reason: a person invoking it by hand
// has misunderstood what it is for.

// captureFile is written beside the log so a later reader can tell a complete
// capture from a degraded one without inferring it from a file size.
const captureFile = "capture.json"

// teeFlags is the parsed form of `sense-lab tee`: where to log, and the server
// command after a bare --.
type teeFlags struct {
	log    string
	server []string
}

func parseTeeFlags(args []string, stderr io.Writer) (teeFlags, error) {
	var f teeFlags
	// The server command is everything after the first bare --, taken out
	// before flag parsing so the server's own flags are never read as ours.
	for i, arg := range args {
		if arg == "--" {
			f.server = args[i+1:]
			args = args[:i]
			break
		}
	}
	fs := flag.NewFlagSet("tee", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.log, "log", "", "JSONL capture file; empty means no interposition at all")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	if len(f.server) == 0 {
		return f, errors.New("the server command is required after `--`")
	}
	return f, nil
}

// teeServer sits between an MCP client and the server it would have spawned,
// passing every byte through unchanged and recording each frame.
func teeServer(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	f, err := parseTeeFlags(args, stderr)
	if err != nil {
		if err != flag.ErrHelp {
			_, _ = fmt.Fprintf(stderr, "sense-lab tee: %v\n", err)
		}
		return exitUsage
	}

	// No log configured means no interposition at all: the process is replaced
	// by the server, so there is nothing between the client and Sense, not even
	// a copy loop. Fail-open in its strongest form.
	if f.log == "" {
		return execServer(f.server, stderr)
	}

	sink, closeLog := openLog(f.log, stderr)
	defer closeLog()

	return relay(f, sink, stdin, stdout, stderr)
}

// execServer replaces this process with the server. It returns only if that
// could not be done.
func execServer(server []string, stderr io.Writer) int {
	path, err := exec.LookPath(server[0])
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab tee: %v\n", err)
		return exitError
	}
	err = syscall.Exec(path, server, os.Environ())
	_, _ = fmt.Fprintf(stderr, "sense-lab tee: exec %s: %v\n", path, err)
	return exitError
}

// openLog opens the capture file, or reports that capture is off and carries on
// with a sink that records nothing. Capture is telemetry, never a run
// dependency: a session must not fail because its instrument could not.
func openLog(path string, stderr io.Writer) (tee.Sink, func()) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab tee: capture disabled (%v)\n", err)
		return nil, func() {}
	}
	// #nosec G304 -- the path is the runner's own capture file, not user input.
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab tee: capture disabled (%v)\n", err)
		writeCaptureStatus(path, tee.Status{Reason: err.Error()})
		return nil, func() {}
	}

	log := tee.NewLog(f, nil)
	// Written before a frame arrives, so a session killed at its wall still
	// leaves a file saying capture was running rather than nothing at all.
	writeCaptureStatus(path, tee.Status{Capturing: true})
	return log, func() {
		status := log.Close()
		_ = f.Close()
		writeCaptureStatus(path, status)
	}
}

// writeCaptureStatus records what capture managed, beside the log. It is
// best-effort by design: a capture that cannot describe itself must still not
// take the session down with it.
func writeCaptureStatus(logPath string, status tee.Status) {
	b, _ := json.MarshalIndent(status, "", "  ")
	_ = os.WriteFile(filepath.Join(filepath.Dir(logPath), captureFile), append(b, '\n'), 0o644)
}

// relay spawns the server and copies in both directions until it closes its
// output.
func relay(f teeFlags, sink tee.Sink, stdin io.Reader, stdout, stderr io.Writer) int {
	// #nosec G204 -- the command is the run's own MCP registration.
	cmd := exec.Command(f.server[0], f.server[1:]...)
	cmd.Stderr = stderr
	toServer, err := cmd.StdinPipe()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab tee: %v\n", err)
		return exitError
	}
	fromServer, err := cmd.StdoutPipe()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab tee: %v\n", err)
		return exitError
	}
	if err := cmd.Start(); err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab tee: start %s: %v\n", f.server[0], err)
		return exitError
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		// The client's end closing is how a session ends, so the server is told
		// by closing its input rather than by a signal.
		_ = tee.Pump(toServer, stdin, tee.ClientToServer, sink)
		_ = toServer.Close()
	}()

	// The server closing its output is the end of the session, so this one is
	// waited on rather than left to a goroutine.
	_ = tee.Pump(stdout, fromServer, tee.ServerToClient, sink)
	err = cmd.Wait()
	wg.Wait()

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab tee: %v\n", err)
		return exitError
	}
	return exitOK
}
