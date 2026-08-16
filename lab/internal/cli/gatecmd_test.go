package cli

import (
	"bytes"
	"strings"
	"testing"
)

// runGate is the command as a caller sees it: a decision in, an exit code out.
func runGate(t *testing.T, decision string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code = gateCmd(nil, strings.NewReader(decision), &out, &errOut)
	return code, out.String(), errOut.String()
}

// clean is a decision no gate refuses.
const clean = `{
  "baseline_reached": 4, "group_total": 12,
  "baseline_recall": 0.21, "baseline_recorded": true,
  "mini_bench_ran": true,
  "Validation": {"Ran": true, "Scored": false, "Wall": 480000000000, "RealWall": 480000000000},
  "sense_runs": 2, "baseline_runs": 2,
  "retries": 0, "scored_a_parked_run": false
}`

func TestACleanCellExitsZero(t *testing.T) {
	code, stdout, stderr := runGate(t, clean)

	if code != exitOK {
		t.Fatalf("exit = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "no gate refuses") {
		t.Errorf("stdout = %q", stdout)
	}
}

func TestARefusedCellExitsNonZero(t *testing.T) {
	// This is the whole pitch. A gate that only builds a report string
	// constrains nothing, and the old tree has the receipt: a rule quoted as
	// law lived inside a function that renders a sentence.
	refused := strings.Replace(clean, `"mini_bench_ran": true`, `"mini_bench_ran": false`, 1)

	code, _, stderr := runGate(t, refused)

	if code == exitOK {
		t.Fatal("a cell with no mini-bench exited zero")
	}
	if code != exitRefused {
		t.Errorf("exit = %d, want the refusal code %d", code, exitRefused)
	}
	if !strings.Contains(stderr, "mini-bench") {
		t.Errorf("stderr = %q, which does not name the gate that fired", stderr)
	}
}

func TestARefusalCodeIsNotTheBrokenBinaryCode(t *testing.T) {
	// "A gate says no" and "the binary broke" are opposite situations for
	// whoever is reading. One of them is the instrument working.
	refused := strings.Replace(clean, `"mini_bench_ran": true`, `"mini_bench_ran": false`, 1)
	gateCode, _, _ := runGate(t, refused)
	brokenCode, _, _ := runGate(t, "not a decision")

	if gateCode == brokenCode {
		t.Errorf("a refusal and a broken input both exit %d", gateCode)
	}
	if brokenCode != exitUsage {
		t.Errorf("an unreadable decision exits %d, want a usage error", brokenCode)
	}
}

func TestEveryGateThatFiredIsPrinted(t *testing.T) {
	// An operator who fixes one thing only to be refused by the next has
	// learned nothing about the cell.
	code, _, stderr := runGate(t, `{
	  "baseline_reached": 12, "group_total": 12,
	  "baseline_recall": 0.9, "baseline_recorded": true,
	  "mini_bench_ran": false,
	  "sense_runs": 1, "baseline_runs": 1,
	  "retries": 3, "scored_a_parked_run": true
	}`)

	if code != exitRefused {
		t.Fatalf("exit = %d", code)
	}
	if !strings.Contains(stderr, "6 gate(s)") {
		t.Errorf("stderr = %q, want all six reported", stderr)
	}
	for _, gate := range []string{"assembles the set", "caps at 1.00", "mini-bench", "validation", "scored runs", "never a loop"} {
		if !strings.Contains(stderr, gate) {
			t.Errorf("stderr does not report %q:\n%s", gate, stderr)
		}
	}
}

func TestADecisionDescribingSomethingElseIsRefusedAsUnreadable(t *testing.T) {
	// A field these gates do not check, accepted silently, would report a pass
	// on a question nobody asked.
	code, _, _ := runGate(t, `{"mini_bench_ran": true, "force": true}`)

	if code != exitUsage {
		t.Errorf("exit = %d for a decision carrying an unknown field, want a usage error", code)
	}
}

func TestThereIsNoBypassFlag(t *testing.T) {
	// Not --force, not an environment variable, not a config field. A gate with
	// an override is a suggestion, and the moment of frustration is exactly
	// when it would be used.
	refused := strings.Replace(clean, `"mini_bench_ran": true`, `"mini_bench_ran": false`, 1)

	for _, bypass := range []string{"-force", "--force", "-yes", "-override", "-skip-gates"} {
		var out, errOut bytes.Buffer
		if code := gateCmd([]string{bypass}, strings.NewReader(refused), &out, &errOut); code == exitOK {
			t.Errorf("%s got a refused cell past the gates", bypass)
		}
	}
}
