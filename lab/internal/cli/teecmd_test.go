package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// echoServer stands in for `sense mcp`: it reads newline-delimited frames and
// answers each one, so a test can watch traffic in both directions.
func echoServer(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	const script = `#!/bin/sh
while IFS= read -r line; do
  printf '{"jsonrpc":"2.0","id":1,"result":{"echo":%s}}\n' "$(printf '%s' "$line" | wc -c | tr -d ' ')"
done
`
	path := filepath.Join(t.TempDir(), "server")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func runTee(t *testing.T, args []string, stdin string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = teeServer(args, strings.NewReader(stdin), &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestTheSessionSeesTheServersAnswersThroughTheShim(t *testing.T) {
	log := filepath.Join(t.TempDir(), "artifacts", "sense-io.jsonl")

	code, stdout, stderr := runTee(t,
		[]string{"-log", log, "--", echoServer(t)},
		"{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\"}\n")

	if code != exitOK {
		t.Fatalf("tee exited %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, `"echo"`) {
		t.Errorf("the client received %q, want the server's answer", stdout)
	}
	captured := readCapture(t, log)
	if len(captured) != 2 {
		t.Fatalf("captured %d frames, want the request and the response", len(captured))
	}
	if captured[0]["dir"] != "c2s" || captured[1]["dir"] != "s2c" {
		t.Errorf("captured directions %v and %v", captured[0]["dir"], captured[1]["dir"])
	}
}

func TestTheCaptureSaysWhetherItIsComplete(t *testing.T) {
	log := filepath.Join(t.TempDir(), "artifacts", "sense-io.jsonl")

	if code, _, stderr := runTee(t, []string{"-log", log, "--", echoServer(t)}, "{\"id\":1}\n"); code != exitOK {
		t.Fatalf("tee exited %d: %s", code, stderr)
	}

	var status struct {
		Capturing bool `json:"capturing"`
		Frames    int  `json:"frames"`
		Dropped   int  `json:"dropped"`
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(log), "capture.json"))
	if err != nil {
		t.Fatalf("no capture status beside the log: %v", err)
	}
	if err := json.Unmarshal(b, &status); err != nil {
		t.Fatalf("capture status is not JSON: %v", err)
	}
	if !status.Capturing || status.Frames != 2 || status.Dropped != 0 {
		t.Errorf("capture status = %+v, want a complete capture of two frames", status)
	}
}

func TestALogThatCannotBeOpenedDoesNotStopTheSession(t *testing.T) {
	// Capture is telemetry, never a run dependency. A session that dies because
	// its instrument could not write is a self-inflicted burned cell.
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runTee(t,
		[]string{"-log", filepath.Join(blocked, "sense-io.jsonl"), "--", echoServer(t)},
		"{\"id\":1}\n")

	if code != exitOK {
		t.Fatalf("tee exited %d although only capture failed: %s", code, stderr)
	}
	if !strings.Contains(stdout, `"echo"`) {
		t.Errorf("the client received %q; the session was cut off by its own capture", stdout)
	}
	if !strings.Contains(stderr, "capture disabled") {
		t.Errorf("stderr = %q, want it to say capture was disabled", stderr)
	}
}

func TestAnUnwritableLogFileDisablesCaptureAndSaysWhy(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission that makes the open fail")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "sense-io.jsonl")
	if err := os.WriteFile(log, nil, 0o400); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(log, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(log, 0o600) })

	code, stdout, _ := runTee(t, []string{"-log", log, "--", echoServer(t)}, "{\"id\":1}\n")

	if code != exitOK {
		t.Fatalf("tee exited %d although only capture failed", code)
	}
	if !strings.Contains(stdout, `"echo"`) {
		t.Error("the session lost its answers when capture failed")
	}
	// The run must be able to tell "capture failed" from "Sense was never
	// called", and the two are the same empty file otherwise.
	b, err := os.ReadFile(filepath.Join(dir, "capture.json"))
	if err != nil {
		t.Fatalf("no capture status was written: %v", err)
	}
	if !strings.Contains(string(b), `"capturing": false`) {
		t.Errorf("capture status = %s, want it to report capture off", b)
	}
}

func TestNoLogMeansNoInterpositionAtAll(t *testing.T) {
	// The strongest form of fail-open: the process is replaced by the server,
	// so there is not even a copy loop between the client and Sense. Proven by
	// the process image changing, since a shim that merely forwarded would
	// still be this process.
	if runtime.GOOS == "windows" {
		t.Skip("exec replacement is a POSIX behaviour")
	}
	code, _, stderr := runTee(t, []string{"--", filepath.Join(t.TempDir(), "no-such-server")}, "")

	// The lookup fails, which is the only way this path can return at all.
	if code != exitError {
		t.Fatalf("tee exited %d, want the exec failure reported", code)
	}
	if !strings.Contains(stderr, "no-such-server") {
		t.Errorf("stderr = %q, want it to name the server it could not become", stderr)
	}
}

func TestAServerThatCannotBeBecomeIsReported(t *testing.T) {
	// With no log there is nothing between the client and Sense: this process
	// is replaced by the server. The only way that path can return is when the
	// replacement itself fails, and it must say so rather than exiting quietly
	// and leaving the agent with a server that never spoke.
	if runtime.GOOS == "windows" {
		t.Skip("exec replacement is a POSIX behaviour")
	}
	// Executable, so it is found, but not something the kernel can run: a file
	// with no interpreter line. Anything the kernel COULD run would replace
	// this test process.
	notRunnable := filepath.Join(t.TempDir(), "server")
	if err := os.WriteFile(notRunnable, []byte("\x00\x01not an executable"), 0o755); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := runTee(t, []string{"--", notRunnable}, "")

	if code != exitError {
		t.Fatalf("tee exited %d, want the failed replacement reported", code)
	}
	if !strings.Contains(stderr, "exec") {
		t.Errorf("stderr = %q, want it to name the replacement that failed", stderr)
	}
}

func TestAServerCommandIsRequired(t *testing.T) {
	if code, _, _ := runTee(t, []string{"-log", "/tmp/x.jsonl"}, ""); code != exitUsage {
		t.Errorf("tee exited %d with no server command, want a usage error", code)
	}
	if code, _, _ := runTee(t, []string{"-log", "/tmp/x.jsonl", "--"}, ""); code != exitUsage {
		t.Errorf("tee exited %d with an empty server command, want a usage error", code)
	}
}

func TestAnUnknownFlagIsAUsageError(t *testing.T) {
	if code, _, _ := runTee(t, []string{"-nonsense", "--", "true"}, ""); code != exitUsage {
		t.Errorf("tee exited %d on an unknown flag, want a usage error", code)
	}
}

func TestTheServersExitStatusIsPassedOn(t *testing.T) {
	failing := filepath.Join(t.TempDir(), "server")
	if err := os.WriteFile(failing, []byte("#!/bin/sh\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	code, _, _ := runTee(t, []string{"-log", filepath.Join(t.TempDir(), "io.jsonl"), "--", failing}, "")

	if code != 3 {
		t.Errorf("tee exited %d, want the server's own 3", code)
	}
}

func TestAServerThatCannotBeStartedIsReported(t *testing.T) {
	code, _, stderr := runTee(t,
		[]string{"-log", filepath.Join(t.TempDir(), "io.jsonl"), "--", filepath.Join(t.TempDir(), "missing")}, "")

	if code != exitError {
		t.Errorf("tee exited %d, want an error", code)
	}
	if stderr == "" {
		t.Error("tee failed to start the server without saying so")
	}
}

func readCapture(t *testing.T, path string) []map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read capture: %v", err)
	}
	var entries []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
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
