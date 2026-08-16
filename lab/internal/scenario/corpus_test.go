package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The shipped scenarios are not fixtures: they are what every run reads. A
// malformed one must fail here, cheaply, rather than forty minutes into a paid
// session.
//
// The counts are the split's own receipt. Five scenarios carried 154 gold rows
// before the split and carry 154 after; the rows moved and nothing about them
// changed.
func TestEveryShippedScenarioLoadsWithItsGoldAndRubric(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "lab", "scenarios")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]int{
		"bitwarden-server": 30,
		"chatwoot":         38,
		"discourse":        23,
		"mastodon":         38,
		"rails":            25,
	}
	var total, seen int

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		seen++
		t.Run(e.Name(), func(t *testing.T) {
			s, err := Load(filepath.Join(dir, e.Name()), e.Name())
			if err != nil {
				t.Fatalf("the shipped scenario does not load: %v", err)
			}
			if got := len(s.Gold.Rows); got != want[e.Name()] {
				t.Errorf("%d gold rows, want the %d it had before the split", got, want[e.Name()])
			}
			// Every id unique within its own file, so no row was duplicated on
			// the way across.
			ids := map[string]bool{}
			for _, r := range s.Gold.Rows {
				if ids[r.ID] {
					t.Errorf("gold row %q appears twice", r.ID)
				}
				ids[r.ID] = true
			}
			// Three distinct hashes: a set whose files hashed alike would mean
			// the split had not actually separated them.
			if s.ScenarioID == s.GoldID || s.GoldID == s.RubricID || s.ScenarioID == s.RubricID {
				t.Error("two of the three files hash alike")
			}
		})
		if n, ok := want[e.Name()]; ok {
			total += n
		}
	}

	if seen != 5 {
		t.Errorf("found %d shipped scenarios, want 5", seen)
	}
	if total != 154 {
		t.Errorf("the shipped scenarios account for %d gold rows, want the 154 that were split", total)
	}
}

// repoRoot walks up from the test's working directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("no go.mod found above the working directory")
		}
		dir = parent
	}
}

// The hash deliberately ignores comments so that the seventy-line headers
// explaining why an attempt exists are free to write. That property guards
// nothing if the headers are not there — and they were not: the first split
// round-tripped the YAML through an emitter and deleted 1,222 comment lines
// across the five scenarios without a single test noticing.
func TestEveryShippedScenarioKeepsItsProvenanceHeader(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "lab", "scenarios")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		t.Run(name, func(t *testing.T) {
			b, err := os.ReadFile(filepath.Join(dir, name, name+".yaml"))
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			var comments int
			for _, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "#") {
					comments++
				}
			}
			// Every scenario in the corpus carries a header saying what the last
			// attempt measured and which risk was taken knowingly. The smallest
			// is rails at 28 lines.
			if comments < 20 {
				t.Errorf("%s carries %d comment lines; the provenance header is gone", name, comments)
			}
		})
	}
}
