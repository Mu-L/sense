package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runStatus(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := Run(append([]string{"status"}, args...), &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestStatusReportsACampaignFromItsRunTree(t *testing.T) {
	camp := t.TempDir()
	at := filepath.Join(camp, "mastodon", "1", "author")
	if err := os.MkdirAll(at, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(at, "scenario.draft.yaml"), []byte("name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := runStatus(t, "-campaign", camp)
	if code != exitOK {
		t.Fatalf("exit %d: %s", code, stderr)
	}
	for _, want := range []string{"authoritative", "mastodon", "LOOP POSITION", "SPEND", "RESUME"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the page does not carry %q:\n%s", want, stdout)
		}
	}
}

// A status with no campaign would have to guess which tree it is reporting on,
// and a page about the wrong campaign is worse than no page.
func TestStatusRefusesWithoutACampaign(t *testing.T) {
	code, _, stderr := runStatus(t)
	if code != exitUsage {
		t.Errorf("exit %d, want a usage error", code)
	}
	if !strings.Contains(stderr, "-campaign") {
		t.Errorf("the refusal does not name the missing flag: %q", stderr)
	}
}

func TestStatusReportsAnUnreadableTreeAsAnError(t *testing.T) {
	camp := t.TempDir()
	locked := filepath.Join(camp, "mastodon")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	code, _, stderr := runStatus(t, "-campaign", camp)
	if code != exitError {
		t.Errorf("exit %d, want an error", code)
	}
	if stderr == "" {
		t.Error("the failure was silent")
	}
}

func TestStatusRejectsAnUnknownFlag(t *testing.T) {
	if code, _, _ := runStatus(t, "-campaign", t.TempDir(), "-watch"); code != exitUsage {
		t.Errorf("exit %d, want a usage error: there is no watch mode", code)
	}
}

// The page is written and nothing reads it back. This is the property the type
// split exists for, and it is worth one grep because the failure is silent: a
// display format that becomes a data contract only bites when someone changes a
// heading.
func TestNothingInTheLabParsesTheRenderedPage(t *testing.T) {
	for _, f := range labFiles(t) {
		if strings.HasSuffix(f.path, "_test.go") || f.path == "lab/internal/cli/statuscmd.go" {
			continue
		}
		for _, imp := range f.imports {
			if strings.HasSuffix(imp, "lab/internal/status") {
				t.Errorf("%s imports the status package; only the command that prints the page should",
					f.path)
			}
		}
	}
}
