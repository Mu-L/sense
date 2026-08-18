package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A capture read by the wrong reader parses cleanly, finds none of its events
// and produces an empty transcript — which reads exactly like an arm that said
// nothing. Refusing an unknown name is what keeps that from being silent.
func TestAnUnknownFormatIsRefusedRatherThanGuessed(t *testing.T) {
	_, err := Read("some-other-tool", filepath.Join("testdata", "claude-normal.jsonl"))

	if err == nil {
		t.Fatal("a capture in an unknown format was read by whichever reader came first")
	}
	if !strings.Contains(err.Error(), "assistant-events") {
		t.Errorf("error = %q, want it to name the formats that do exist", err)
	}
}

func TestEachFormatIsReadByItsOwnReader(t *testing.T) {
	for _, tc := range []struct{ format, fixture, wantText string }{
		{"assistant-events", "claude-normal.jsonl", ""},
		{"item-events", "item-events-tools.jsonl", "./sub/greeting.txt"},
	} {
		t.Run(tc.format, func(t *testing.T) {
			tr, err := Read(tc.format, filepath.Join("testdata", tc.fixture))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if len(tr.Text) == 0 {
				t.Fatal("the reader found no text at all, which is what reading with the wrong one looks like")
			}
			if tc.wantText != "" && !strings.Contains(tr.Answer(), tc.wantText) {
				t.Errorf("the answer does not carry %q:\n%s", tc.wantText, tr.Answer())
			}
		})
	}
}

// Every run recorded before the format was written down came from one agent
// tool, so a capture that names none is that tool's. Refusing them would make
// the whole recorded corpus unreadable to prove a point.
func TestARunThatNamesNoFormatIsReadAsTheOneTheCorpusIsIn(t *testing.T) {
	if got := FormatOfRun(""); got != LegacyFormat {
		t.Errorf("FormatOfRun(\"\") = %q, want %q", got, LegacyFormat)
	}
	if got := FormatOfRun("item-events"); got != "item-events" {
		t.Errorf("a run that names its format was read as %q", got)
	}
	if _, ok := Formats[LegacyFormat]; !ok {
		t.Errorf("%q has no reader, so the recorded corpus cannot be read", LegacyFormat)
	}
}

// The format names must not be a tool's own spelling of its output format: the
// identifier probe reads the built binary for catalog values, and `stream-json`
// is one of them.
func TestNoFormatNameIsAToolsOwnArgument(t *testing.T) {
	agents := filepath.Join("..", "..", "agents")
	entries, err := os.ReadDir(agents)
	if err != nil {
		t.Fatalf("read the agent configs: %v", err)
	}
	for _, e := range entries {
		b, err := os.ReadFile(filepath.Join(agents, e.Name(), "agent.json"))
		if err != nil {
			continue
		}
		for name := range Formats {
			// The format appears in the config once, as the value of
			// transcript_format. Twice means it is also an argument or a
			// binary, and then the probe would flag it in the binary.
			if strings.Count(string(b), name) > 1 {
				t.Errorf("%s names %q more than once; a format name that is also an argument "+
					"is a catalog identifier compiled into the reader", e.Name(), name)
			}
		}
	}
}
