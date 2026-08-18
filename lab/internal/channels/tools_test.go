package channels_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/channels"
)

// senseServing stands in for `sense mcp`: it answers the handshake with the
// tools it is given.
func senseServing(t *testing.T, tools string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	script := `#!/bin/sh
while IFS= read -r line; do
  case "$line" in
    *'"tools/list"'*) printf '{"jsonrpc":"2.0","id":2,"result":{"tools":[` + tools + `]}}\n' ;;
  esac
done
`
	path := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheToolNamesComeFromTheServerRatherThanFromAList(t *testing.T) {
	// Derived for the same reason the repository routes are: a tool added to
	// the product would leave a written-down list short, the baseline arm's
	// transcript would be searched for the old names, and a run in which the
	// baseline reached Sense through the new one would pass every check.
	bin := senseServing(t, `{"name":"sense_graph"},{"name":"sense_blast"},{"name":"sense_newthing"}`)

	got, err := channels.ToolNames(context.Background(), bin, t.TempDir())
	if err != nil {
		t.Fatalf("ToolNames: %v", err)
	}

	if !slices.Equal(got, []string{"sense_blast", "sense_graph", "sense_newthing"}) {
		t.Errorf("ToolNames = %v", got)
	}
}

func TestAServerThatOffersNoToolsIsRefused(t *testing.T) {
	// An empty list makes every transcript check pass, which is the shape of a
	// leak that reads as a clean bill of health.
	_, err := channels.ToolNames(context.Background(), senseServing(t, ""), t.TempDir())

	if err == nil {
		t.Fatal("ToolNames accepted an empty tool list")
	}
	if !strings.Contains(err.Error(), "no tools") {
		t.Errorf("error = %v, want it to name the empty list", err)
	}
}

func TestAServerThatNeverAnswersIsRefused(t *testing.T) {
	silent := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(silent, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := channels.ToolNames(context.Background(), silent, t.TempDir()); err == nil {
		t.Fatal("ToolNames accepted a server that never answered")
	}
}

func TestAServerThatCannotStartIsReported(t *testing.T) {
	if _, err := channels.ToolNames(context.Background(), filepath.Join(t.TempDir(), "missing"), t.TempDir()); err == nil {
		t.Fatal("ToolNames accepted a server that could not start")
	}
}

func TestNoiseBeforeTheToolListDoesNotHideIt(t *testing.T) {
	// A server that logged a line onto its output stream before answering.
	bin := filepath.Join(t.TempDir(), "sense")
	script := `#!/bin/sh
printf 'starting up\n'
printf '{"jsonrpc":"2.0","id":1,"result":{}}\n'
while IFS= read -r line; do
  case "$line" in
    *'"tools/list"'*) printf '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"sense_graph"}]}}\n' ;;
  esac
done
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := channels.ToolNames(context.Background(), bin, t.TempDir())
	if err != nil {
		t.Fatalf("ToolNames: %v", err)
	}
	if !slices.Equal(got, []string{"sense_graph"}) {
		t.Errorf("ToolNames = %v", got)
	}
}

// What an arm's own calls say it used.

const tools = "sense_graph,sense_blast"

// used is an arm that made these calls and recorded this transcript. The two
// are separate arguments because they answer different questions: the calls say
// what the arm DID, and the raw capture is where a shell invocation of the
// binary is found.
func used(calls []string, transcript string) []string {
	return channels.UsedBy(calls, []byte(transcript), strings.Split(tools, ","), "sense")
}

func TestAnArmThatCalledAnMcpToolIsReported(t *testing.T) {
	got := used([]string{"mcp__sense__sense_graph"}, "")

	if len(got) == 0 {
		t.Fatal("an arm that called an MCP tool from the sense server was reported clean")
	}
}

// The spellings the three agent tools were measured writing. All three are the
// same event, and an arm that made any of these calls used Sense.
func TestEverySpellingOfAnMcpCallIsReported(t *testing.T) {
	for _, call := range []string{"mcp__sense__sense_graph", "sense_graph", "sense_sense_graph"} {
		if got := used([]string{call}, ""); len(got) == 0 {
			t.Errorf("a call named %q was reported clean", call)
		}
	}
}

func TestATranscriptThatRanTheBinaryIsReported(t *testing.T) {
	// The check that catches what configuration checks cannot: an arm that
	// found the binary some other way leaves no configuration trace at all.
	got := used([]string{"Bash"}, `{"name":"Bash","input":{"command":"sense graph -symbol Category"}}`)

	if len(got) != 1 || !strings.Contains(got[0], "ran the") {
		t.Fatalf("UsedBy = %v, want the invocation reported", got)
	}
}

func TestTheBinaryIsCaughtAtAnyCommandPosition(t *testing.T) {
	// `git log | sense search x` runs it just as surely as `sense search x`.
	for _, command := range []string{
		"sense search Category",
		"git log | sense search Category",
		"cd app && sense graph -symbol Category",
		"(sense status)",
		"/usr/local/bin/sense mcp",
	} {
		if got := used(nil, `{"input":{"command":"`+command+`"}}`); len(got) == 0 {
			t.Errorf("%q was reported clean", command)
		}
	}
}

func TestARepositoryThatMerelyMentionsSenseIsNotAnInvocation(t *testing.T) {
	// A bare substring search would report every run against a codebase whose
	// prose uses the word, and a check that cries wolf is a check somebody
	// stops reading.
	for _, command := range []string{
		"grep -rn 'makes sense' .",
		"cat docs/sense-of-scale.md",
		"echo this does not make sense",
	} {
		if got := used(nil, `{"input":{"command":"`+command+`"}}`); len(got) != 0 {
			t.Errorf("%q was reported as an invocation: %v", command, got)
		}
	}
}

func TestACleanBaselineTranscriptReportsNothing(t *testing.T) {
	got := used([]string{"Grep", "Bash"}, `{"type":"assistant","message":{"content":[
		{"type":"tool_use","name":"Grep"},
		{"type":"tool_use","name":"Bash","input":{"command":"grep -rn Category app/"}}]}}
		{"type":"result","result":"Category is referenced in three files"}`)

	if len(got) != 0 {
		t.Errorf("a clean baseline transcript reported %v", got)
	}
}

// The false positive this check was measured producing, and the reason it reads
// calls rather than text.
//
// A baseline arm grepped a repository that happens to contain the tool names —
// Sense's own, in the case that found it — and its own grep OUTPUT quoted them
// back into the transcript. Six calls, all shell, grep and read, and the pair
// was refused as a measurement on the strength of it. What an arm was TOLD is
// not something it did.
func TestAnArmThatOnlyREADAToolNameIsNotReportedAsHavingUsedIt(t *testing.T) {
	got := used([]string{"grep", "read"}, `{"type":"tool_use","part":{"tool":"grep","state":{"output":
		"lab/internal/mcpio/server_test.go:128: t.Run(\"sense_graph\", func(t *testing.T) {"}}}
		{"type":"text","part":{"text":"The tests exercise sense_graph and sense_blast."}}`)

	if len(got) != 0 {
		t.Errorf("an arm that merely read a file naming a sense tool was reported as having used one: %v", got)
	}
}

func TestAToolNameOnItsOwnIsEnough(t *testing.T) {
	// An agent tool that records the bare name rather than the prefixed one is
	// recording the same event.
	if got := used([]string{"sense_blast"}, ""); len(got) == 0 {
		t.Fatal("a transcript naming sense_blast was reported clean")
	}
}

func TestWithNoBinaryNamedNoInvocationIsLookedFor(t *testing.T) {
	got := channels.UsedBy(nil, []byte(`{"input":{"command":"sense graph"}}`), nil, "")

	if len(got) != 0 {
		t.Errorf("UsedBy = %v with no binary named", got)
	}
}

func TestACommandThatIsNotAStringIsSkipped(t *testing.T) {
	// A transcript is another program's output, so it must be read defensively:
	// one malformed record must not stop the check that follows it.
	got := used(nil, `{"input":{"command":"\uDEAD"}}{"input":{"command":"sense graph"}}`)

	if len(got) == 0 {
		t.Fatal("a malformed record hid the invocation that followed it")
	}
}
