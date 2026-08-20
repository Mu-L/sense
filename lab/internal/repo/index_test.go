package repo

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// realStatus is the shape `sense_status` actually answers in, trimmed to the
// keys this artifact reads. It is quoted from a real answer rather than
// invented, because a record that parses a shape the server does not send is a
// record of nothing.
const realStatus = `{"index":{"path":".sense/index.db","size_bytes":33419264,"files":2137,"symbols":11410,` +
	`"edges":23698,"embeddings":11410,"coverage":1},"languages":{"csharp":{"files":2137,"symbols":11410,` +
	`"tier":"standard"},"go":{"files":12,"symbols":40,"tier":"full"}},"profile":{"tier":"medium"},` +
	`"version":{"binary":"1.14.1-dev+gb5b78be","schema":5},"next_steps":[]}`

// fakeSense stands in for the product, and it is deliberately strict about the
// three things the caller has to get right: it refuses to answer anywhere but
// the checkout, it records the argv it was given, and it answers sense_status
// only after an initialise and only when that is the tool asked for.
//
// A permissive stand-in was worse than none here. An earlier one answered any
// line carrying `tools/call` from any directory, and under it the server could
// be asked in the wrong repository, the scan could lose its flags, and the
// handshake could be deleted outright, with the suite green through all of it.
func fakeSense(t *testing.T, status string) (bin string, argv func() string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	dir := t.TempDir()
	bin = filepath.Join(dir, "sense")
	log := filepath.Join(dir, "argv")
	script := `#!/bin/sh
set -e
echo "$@" >> "` + log + `"
case "$1" in
scan) mkdir -p "$3/.sense"; printf 'index' > "$3/.sense/index.db" ;;
mcp)
  if [ "$PWD" != "$SENSE_EXPECTED_DIR" ]; then
    echo "asked in $PWD, not in $SENSE_EXPECTED_DIR" >&2
    exit 9
  fi
  ready=""
  while IFS= read -r line; do
    case "$line" in
      *'"initialize"'*) ready=1 ;;
      *'"sense_status"'*)
        [ -n "$ready" ] || { echo "asked before the handshake" >&2; exit 9; }
        printf '{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{}}}\n'
        printf '{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":%s}]}}\n' "$STATUS"
        ;;
    esac
  done
  ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	// The status document is carried in the environment rather than baked into
	// the script, so a test can vary the one thing it is varying.
	t.Setenv("STATUS", mustQuote(t, status))
	return bin, func() string {
		b, err := os.ReadFile(log)
		if err != nil {
			t.Fatalf("the stand-in was never run: %v", err)
		}
		return string(b)
	}
}

// mustQuote renders the status document as the JSON string the MCP text field
// carries it in.
func mustQuote(t *testing.T, status string) string {
	t.Helper()
	b, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func scanned(t *testing.T, status string) Index {
	t.Helper()
	i, _ := scannedWithArgv(t, status)
	return i
}

func scannedWithArgv(t *testing.T, status string) (Index, string) {
	t.Helper()
	checkout := t.TempDir()
	bin, argv := fakeSense(t, status)
	// The server answers about the index of wherever it is started, so the
	// stand-in refuses to answer anywhere else and this is what it compares
	// against.
	t.Setenv("SENSE_EXPECTED_DIR", checkout)
	at := time.Date(2026, 8, 19, 10, 30, 0, 0, time.UTC)
	i, err := Scan(context.Background(), bin,
		Plan{ID: "jellyfin", URL: "https://example.test/jellyfin.git", Revision: "abc123", Checkout: checkout}, at)
	if err != nil {
		t.Fatal(err)
	}
	return i, argv()
}

// The counts are quoted from the server, never estimated, and the whole answer
// is kept beside them: a field this record does not name is still readable by
// whoever reads the artifact next.
func TestTheArtifactQuotesWhatTheServerAnswered(t *testing.T) {
	i := scanned(t, realStatus)

	for _, tc := range []struct {
		what      string
		got, want any
	}{
		{"files", i.Files, 2137},
		{"symbols", i.Symbols, 11410},
		{"edges", i.Edges, 23698},
		{"embeddings", i.Embeddings, 11410},
		{"embedding coverage", i.EmbeddingCoverage, float64(1)},
		{"profile tier", i.ProfileTier, "medium"},
		{"sense version", i.SenseVersion, "1.14.1-dev+gb5b78be"},
		{"scanned at", i.ScannedAt, "2026-08-19T10:30:00Z"},
		{"revision", i.Revision, "abc123"},
	} {
		if tc.got != tc.want {
			t.Errorf("%s = %v, want %v", tc.what, tc.got, tc.want)
		}
	}
	if !json.Valid(i.Status) {
		t.Error("the server's own answer was not kept beside the counts")
	}
	if !i.Indexed() {
		t.Error("a repository with 11410 symbols reads as unindexed")
	}
}

// The languages come from the scan rather than from a judgement about the
// repository: a vertical query selects on them, so a guess is a repository that
// silently never matches.
func TestTheLanguagesComeFromTheScanAndAreSorted(t *testing.T) {
	i := scanned(t, realStatus)

	got := i.Names()
	if len(got) != 2 || got[0] != "csharp" || got[1] != "go" {
		t.Errorf("Names() = %v, want the indexed languages in order", got)
	}
	if i.Languages["csharp"].Tier != "standard" {
		t.Errorf("csharp tier = %q, want what the server said", i.Languages["csharp"].Tier)
	}
}

// A scan that indexed nothing says so in a sentence. This is the failure the
// index plan explicitly forbids softening: an index quietly short of its symbols
// is how a repository gets called dark when the tool simply did not run, and the
// authoring phase reads this file to decide exactly that.
func TestAScanThatIndexedNothingSaysSo(t *testing.T) {
	for _, tc := range []struct{ name, index string }{
		{"nothing at all", `{"files":0,"symbols":0}`},
		{"files and no symbols, which is the half-scan", `{"files":900,"symbols":0}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			i := scanned(t, `{"index":`+tc.index+`,"languages":{},"version":{"binary":"1.14.1"}}`)

			if i.Indexed() {
				t.Fatal("an index with no symbols reads as indexed")
			}
			if !strings.Contains(i.Shortfall, "0 symbols") {
				t.Errorf("shortfall = %q, want it to say what was indexed", i.Shortfall)
			}
		})
	}
}

func TestAFailedScanCarriesTheToolsOwnOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	bin := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\necho 'no grammar for this tree' >&2\nexit 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Scan(context.Background(), bin, Plan{ID: "x", Checkout: t.TempDir()}, time.Now())

	if err == nil {
		t.Fatal("a scan that exited 3 was recorded as an index")
	}
	if !strings.Contains(err.Error(), "no grammar for this tree") {
		t.Errorf("err = %q, want the tool's own output", err)
	}
}

// The server is asked and it may not answer. A missing answer is a failure
// rather than an artifact full of zeroes, which would read exactly like a
// repository Sense found nothing in.
func TestAServerThatNeverAnswersIsAFailureRatherThanAnEmptyIndex(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	bin := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\ncase \"$1\" in scan) exit 0 ;; mcp) cat > /dev/null ;; esac\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Scan(context.Background(), bin, Plan{ID: "x", Checkout: t.TempDir()}, time.Now())

	if err == nil || !strings.Contains(err.Error(), "never answered") {
		t.Fatalf("err = %v, want a refusal naming the unanswered call", err)
	}
}

func TestASenseBinaryThatCannotBeRunNamesWhatItCouldNotIndex(t *testing.T) {
	checkout := t.TempDir()

	_, err := Scan(context.Background(), filepath.Join(t.TempDir(), "absent"),
		Plan{ID: "x", Checkout: checkout}, time.Now())

	if err == nil {
		t.Fatal("a missing sense binary produced an index")
	}
	if !strings.Contains(err.Error(), checkout) {
		t.Errorf("err = %q, want it to name the checkout it could not index", err)
	}
}

func TestStatusInReplyReadsTheAnswerOutOfTheStream(t *testing.T) {
	for _, tc := range []struct {
		name  string
		reply string
		want  string
	}{
		{"the answer, past the handshake",
			"{\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{}}\n" +
				`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\"index\":{\"files\":7}}"}]}}` + "\n",
			`{"index":{"files":7}}`},
		{"an earlier reply that carries content of its own is not the answer",
			`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"index\":{\"files\":999}}"}]}}` + "\n" +
				`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\"index\":{\"files\":7}}"}]}}` + "\n",
			`{"index":{"files":7}}`},
		{"an answer with no trailing newline",
			`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\"index\":{}}"}]}}`,
			`{"index":{}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := statusInReply([]byte(tc.reply))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("statusInReply = %s, want %s", got, tc.want)
			}
		})
	}
}

// A reply that carries nothing is not answered with somebody else's content.
// The decoder reuses what it decoded last, so without the reset an earlier
// reply's document is returned as the answer to a call that failed — the status
// of nothing, recorded as the status of this repository.
func TestAFailedCallIsNotAnsweredWithAnEarlierReply(t *testing.T) {
	reply := `{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"index\":{\"files\":999}}"}]}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"error":{"code":-32603,"message":"no index"}}` + "\n"

	_, err := statusInReply([]byte(reply))

	if err == nil {
		t.Fatal("a failed call was answered with an earlier reply's document")
	}
}

// A server that answered with prose rather than a document is refused. Recording
// it would put something no reader can parse where the counts belong.
func TestAnAnswerThatIsNotADocumentIsRefused(t *testing.T) {
	_, err := statusInReply([]byte(`{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"no index here"}]}}` + "\n"))

	if err == nil || !strings.Contains(err.Error(), "not a document") {
		t.Fatalf("err = %v, want a refusal naming what came back", err)
	}
}

// A document the record cannot read is a failure. The alternative is an
// artifact of zeroes that reads as a dark repository.
func TestAStatusThatCannotBeReadIsAFailure(t *testing.T) {
	_, err := record(json.RawMessage(`{"index":[]}`), Plan{}, time.Now())

	if err == nil {
		t.Fatal("a status of the wrong shape was recorded")
	}
}

func TestWriteMakesThePhaseDirectoryAndLeavesReadableJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runs", "jellyfin", "index", "index.json")

	if err := Write(path, scanned(t, realStatus)); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var back Index
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("the artifact does not read back: %v", err)
	}
	if back.Symbols != 11410 {
		t.Errorf("symbols = %d, want what was written", back.Symbols)
	}
	if !strings.HasSuffix(string(b), "\n") {
		t.Error("the artifact does not end in a newline")
	}
	// A clean index says nothing about a shortfall, rather than saying "".
	if strings.Contains(string(b), "shortfall") {
		t.Error("a clean index carries a shortfall field")
	}
}

func TestWriteReportsADirectoryItCannotMake(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocked, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Write(filepath.Join(blocked, "index", "index.json"), Index{}); err == nil {
		t.Fatal("writing under a file succeeded")
	}
}

// Every artifact this package leaves behind goes through one writer, so a
// record it cannot render is refused rather than half-written.
func TestWriteRefusesARecordItCannotRender(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.json")

	if err := Write(path, Index{EmbeddingCoverage: math.NaN()}); err == nil {
		t.Fatal("a record that is not renderable was written")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a file was left behind by a write that failed")
	}
}

// The scan runs over the checkout by flag and blocks on embeddings, and both of
// those are load-bearing rather than stylistic: `sense scan` ignores a
// positional path outright, and without -embed the embedding count recorded here
// is whatever the background pass had reached when the process exited.
func TestTheScanIsRunOverTheCheckoutAndWaitsForItsEmbeddings(t *testing.T) {
	_, argv := scannedWithArgv(t, realStatus)

	if !strings.Contains(argv, "scan -dir ") {
		t.Errorf("argv = %q, want the checkout passed as -dir; a positional path is silently ignored", argv)
	}
	if !strings.Contains(argv, "-embed") {
		t.Errorf("argv = %q, want -embed, or the embedding count is whatever it had reached", argv)
	}
}

// The server answers about the index of wherever it was started, so a status
// taken anywhere else describes another repository entirely — with counts that
// look exactly as plausible as the right ones. The stand-in refuses to answer
// outside the checkout, which is what makes every other test in this file a
// test of the right repository.
func TestTheServerIsAskedInTheCheckout(t *testing.T) {
	bin, _ := fakeSense(t, realStatus)
	t.Setenv("SENSE_EXPECTED_DIR", "/somewhere/else")

	_, err := Scan(context.Background(), bin, Plan{ID: "x", Checkout: t.TempDir()}, time.Now())

	if err == nil {
		t.Fatal("the server was asked somewhere other than the checkout and the answer was recorded")
	}
}

// A server that fails says why, and its own words are what reach the operator.
// Without this the whole failure reads as "exit status 9", which is not
// something anybody can act on.
func TestAServerThatFailsCarriesItsOwnMessage(t *testing.T) {
	bin, _ := fakeSense(t, realStatus)
	t.Setenv("SENSE_EXPECTED_DIR", "/somewhere/else")

	_, err := Scan(context.Background(), bin, Plan{ID: "x", Checkout: t.TempDir()}, time.Now())

	if err == nil || !strings.Contains(err.Error(), "not in /somewhere/else") {
		t.Fatalf("err = %v, want the server's own message", err)
	}
}

// The one pass against the real server. Everything else in this file is written
// against a stand-in, and a stand-in cannot tell whether the exchange this
// package sends is one the product actually answers — which is a whole phase
// that would fail only when it is run for real, against a repository somebody
// waited to clone.
func TestTheRealServerAnswersTheExchangeWeSend(t *testing.T) {
	bin := builtSense(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package app\n\ntype Category struct{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	i, err := Scan(context.Background(), bin, Plan{ID: "x", Checkout: dir}, time.Now())
	if err != nil {
		t.Fatal(err)
	}

	if i.Symbols == 0 || i.SenseVersion == "" {
		t.Errorf("the real server answered %+v, want counts and a version", i)
	}
}

// builtSense is the product binary this repository builds, or a skip. It is not
// built here: `make build` and `make ci` both make it, and building a CGO binary
// inside a test would be minutes per run.
func builtSense(t *testing.T) string {
	t.Helper()
	bin, err := filepath.Abs(filepath.Join("..", "..", "..", "bin", "sense"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skip("bin/sense is not built; run make build")
	}
	return bin
}
