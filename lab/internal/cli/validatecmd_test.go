package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The shipped rails scenario, which cannot be scored: not one of its 25 gold
// rows carries a path:line. It is the reason this command exists, and it must
// be visible from the command line rather than discovered by a paid run.
func TestValidateQuarantinesTheShippedRailsGold(t *testing.T) {
	code, stdout, _ := dispatch(t, "validate",
		"-scenario", filepath.Join(repoRoot(t), "lab", "scenarios", "rails", "rails.yaml"))

	if code != exitBelowFloor {
		t.Errorf("exit code = %d, want %d for a scenario with quarantined rows", code, exitBelowFloor)
	}
	if !strings.Contains(stdout, "25 quarantined") {
		t.Errorf("the rails gold was not quarantined:\n%s", stdout)
	}
	if !strings.Contains(stdout, "no path:line") {
		t.Errorf("the quarantine carries no named reason:\n%s", stdout)
	}
}

// A scenario whose gold is sound exits clean, or the command is just "always
// complain" and nobody would read its output twice.
func TestValidateAcceptsGoldThatIsSound(t *testing.T) {
	code, stdout, stderr := dispatch(t, "validate",
		"-scenario", filepath.Join(repoRoot(t), "lab", "scenarios", "discourse", "discourse.yaml"))

	if code != exitOK {
		t.Errorf("exit code = %d, want %d\n%s\n%s", code, exitOK, stdout, stderr)
	}
	if !strings.Contains(stdout, "0 quarantined") {
		t.Errorf("sound gold was quarantined:\n%s", stdout)
	}
}

// Quarantining a row moves every recorded score taken against it, so this pitch
// names which runs rather than leaving the rescore proof to discover it.
func TestValidateListsTheRunsAQuarantineMoves(t *testing.T) {
	results := t.TempDir()
	for _, run := range []string{
		"opus/hash/sense/rails/run-1",
		"opus/hash/baseline/rails/run-1",
		"opus/hash/sense/discourse/run-1", // a different repo, and not affected
	} {
		dir := filepath.Join(results, filepath.FromSlash(run))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "scored.json"), []byte(`{"recall":0.5}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	_, stdout, _ := dispatch(t, "validate",
		"-scenario", filepath.Join(repoRoot(t), "lab", "scenarios", "rails", "rails.yaml"),
		"-results", results)

	if !strings.Contains(stdout, "moves the recorded score of 2 runs") {
		t.Errorf("the handover list is wrong:\n%s", stdout)
	}
	for _, want := range []string{"sense/rails/run-1", "baseline/rails/run-1"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the handover list is missing %s:\n%s", want, stdout)
		}
	}
	if strings.Contains(stdout, "discourse") {
		t.Errorf("a run of another repo is in the handover list:\n%s", stdout)
	}
}

// -checkout without -commit resolves gold against whatever happens to be
// checked out, which is how a scenario gets quarantined for being stale when it
// is not.
func TestValidateRefusesACheckoutWithoutACommit(t *testing.T) {
	code, _, stderr := dispatch(t, "validate",
		"-scenario", filepath.Join(repoRoot(t), "lab", "scenarios", "rails", "rails.yaml"),
		"-checkout", t.TempDir())

	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "-checkout needs -commit") {
		t.Errorf("the error does not say what is missing:\n%s", stderr)
	}
}

// An unusable checkout downgrades the audit rather than failing it, and the
// report says which checks did not run.
func TestValidateWithAnUnusableCheckoutSaysWhatItSkipped(t *testing.T) {
	_, stdout, stderr := dispatch(t, "validate",
		"-scenario", filepath.Join(repoRoot(t), "lab", "scenarios", "discourse", "discourse.yaml"),
		"-checkout", t.TempDir(), "-commit", "deadbeef")

	if !strings.Contains(stderr, "resolving skipped") {
		t.Errorf("nothing said resolving was skipped:\n%s", stderr)
	}
	if !strings.Contains(stdout, "NOT CHECKED") {
		t.Errorf("a partial audit read as a complete one:\n%s", stdout)
	}
}

func TestValidateNeedsAScenario(t *testing.T) {
	code, _, stderr := dispatch(t, "validate")

	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
	if !strings.Contains(stderr, "-scenario is required") {
		t.Errorf("unhelpful error:\n%s", stderr)
	}
}

func TestValidateReportsAScenarioItCannotLoad(t *testing.T) {
	code, _, stderr := dispatch(t, "validate", "-scenario", filepath.Join(t.TempDir(), "nope.yaml"))

	if code != exitError {
		t.Errorf("exit code = %d, want %d", code, exitError)
	}
	if stderr == "" {
		t.Error("nothing was reported")
	}
}

// The whole audit against a real checkout, which is the only way the resolver
// and the grepper are actually reached.
func TestValidateAgainstARealCheckout(t *testing.T) {
	dir, commit := repoWithGold(t)
	// An anchor, so the covering-grep check runs too and nothing is skipped.
	withAnchor := scoredScenario + "contract_symbol: Category\n"
	set := scenarioSet(t, withAnchor, `discriminator: dependents
rows:
  - id: d:real
    group: dependents
    relation: "app/models/category.rb:1083 a line the checkout has"
  - id: d:moved
    group: dependents
    relation: "lib/tasks/search.rake:4000 a line that moved"
`, scoredRubric)

	code, stdout, stderr := dispatch(t, "validate", "-scenario", set, "-checkout", dir, "-commit", commit)

	if code != exitBelowFloor {
		t.Errorf("exit code = %d, want %d\n%s\n%s", code, exitBelowFloor, stdout, stderr)
	}
	if !strings.Contains(stdout, "does not resolve at the pinned commit") {
		t.Errorf("the moved row was not caught:\n%s", stdout)
	}
	if !strings.Contains(stdout, "d:moved") {
		t.Errorf("the report does not name the moved row:\n%s", stdout)
	}
	if strings.Contains(stdout, "d:real") {
		t.Errorf("a row the checkout resolves was quarantined:\n%s", stdout)
	}
	// Both checks ran, so nothing should be named as skipped.
	if strings.Contains(stdout, "NOT CHECKED") {
		t.Errorf("a full audit reported checks it had not skipped:\n%s", stdout)
	}
}

func TestValidateRejectsAFlagItDoesNotHave(t *testing.T) {
	code, _, _ := dispatch(t, "validate", "-nope")

	if code != exitUsage {
		t.Errorf("exit code = %d, want %d", code, exitUsage)
	}
}

// A results tree that is not there is worth a word, and must not be silently
// read as "no runs are affected".
func TestValidateSaysWhenItCannotListTheAffectedRuns(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "not-created")

	_, stdout, stderr := dispatch(t, "validate",
		"-scenario", filepath.Join(repoRoot(t), "lab", "scenarios", "rails", "rails.yaml"),
		"-results", missing)

	if !strings.Contains(stderr, "could not list recorded runs") {
		t.Errorf("a missing results tree passed quietly:\n%s", stderr)
	}
	if strings.Contains(stdout, "moves the recorded score of 0 runs") {
		t.Errorf("a missing tree was reported as no affected runs:\n%s", stdout)
	}
}
