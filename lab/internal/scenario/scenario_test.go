package scenario

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "scenario.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// The corpus this reads was written for another tool and carries gold, rubric
// weights and pages of campaign history. Refusing to read a file over a field
// this cycle has no use for would be a gate that buys nothing, so the reader
// takes what it needs and ignores the rest.
func TestAScenarioIsReadableAlongsideFieldsTheSkeletonDoesNotUse(t *testing.T) {
	path := write(t, `
name: audit the category contract
repo: discourse
contract_symbol: Category
description: |
  You are a maintainer about to rework a class.
weights:
  correctness: 0.7
steps:
  - name: Map the contract
    prompt: Trace the path end to end.
    checks:
      - type: response_richness
        value: "8"
gold:
  - id: contract-model
    match: [app/models/category.rb]
`)

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if s.Repo != "discourse" {
		t.Errorf("repo = %q, want discourse", s.Repo)
	}
	if len(s.Steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(s.Steps))
	}
	if s.Steps[0].Name != "Map the contract" {
		t.Errorf("step name = %q", s.Steps[0].Name)
	}
}

// A scenario that renders to an empty prompt costs a full run and scores zero,
// which reads on the other side as a failed arm rather than a broken input.
// Both shapes have to be rejected before anything is spent.
func TestAScenarioThatWouldRenderAnEmptyPromptIsRejected(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
	}{
		{
			name: "no steps at all",
			body: "name: empty\ndescription: nothing to do\n",
			want: "no steps",
		},
		{
			name: "a step with a blank prompt",
			body: "name: blank\nsteps:\n  - name: Step one\n    prompt: \"   \"\n",
			want: `step 1 ("Step one") has an empty prompt`,
		},
		{
			name: "a step with no prompt key",
			body: "name: missing\nsteps:\n  - name: Step one\n",
			want: `step 1 ("Step one") has an empty prompt`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(write(t, tc.body))

			if err == nil {
				t.Fatal("Load accepted a scenario that renders to nothing")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

func TestLoadReportsWhatIsWrongWithAFileItCannotRead(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := Load(filepath.Join(t.TempDir(), "absent.yaml"))
		if err == nil || !strings.Contains(err.Error(), "read scenario") {
			t.Errorf("error = %v, want it to name the read", err)
		}
	})

	t.Run("malformed yaml", func(t *testing.T) {
		_, err := Load(write(t, "steps: [oops\n"))
		if err == nil || !strings.Contains(err.Error(), "parse scenario") {
			t.Errorf("error = %v, want it to name the parse", err)
		}
	})
}

// The rendered prompt is the only thing the agent ever sees, so everything the
// scenario means has to survive into it, in order.
func TestThePromptCarriesTheDescriptionAndEveryStepInOrder(t *testing.T) {
	s := Scenario{
		Description: "You are a maintainer.",
		Steps: []Step{
			{Name: "First", Prompt: "Trace the path."},
			{Name: "Second", Prompt: "Audit the dependents."},
		},
	}

	got := s.Prompt()

	for _, want := range []string{
		"You are a maintainer.",
		// Without this sentence a two-step scenario reads as one question, and
		// an agent that answers step one and stops scores as a weak arm rather
		// than a half-run.
		"Work through the following steps in order.",
		"Step 1: First", "Trace the path.",
		"Step 2: Second", "Audit the dependents.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt is missing %q:\n%s", want, got)
		}
	}
	if strings.Index(got, "Step 1") > strings.Index(got, "Step 2") {
		t.Error("the steps are rendered out of order")
	}
}

// Step prompts in the corpus are YAML folded scalars, which arrive with a
// trailing newline and inconsistent leading space. The agent should not be
// asked to read around that.
func TestThePromptDoesNotInheritTheFilesIncidentalWhitespace(t *testing.T) {
	s := Scenario{
		Description: "\n  A description.\n\n",
		Steps:       []Step{{Name: "Only", Prompt: "\n  Do the thing.\n\n"}},
	}

	got := s.Prompt()

	if strings.HasPrefix(got, "\n") || strings.HasPrefix(got, " ") {
		t.Errorf("prompt starts with whitespace: %q", got[:10])
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("prompt has a run of blank lines:\n%s", got)
	}
}
