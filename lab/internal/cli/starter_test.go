package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/plan"
)

// starterCatalog is a config directory holding two models from two providers,
// one tool that drives both, and the pair of subjects a cell is.
func starterCatalog(t *testing.T, extra ...string) (string, *catalog.Catalog) {
	t.Helper()
	config := filepath.Join(t.TempDir(), "lab")
	declarable(t, config)
	// The repository, as admission records it: the matrix is resolved against a
	// catalog that holds the repository it names.
	artifact(t, filepath.Join(config, "repos", "fake-repo.json"),
		`{"id":"fake-repo","url":"https://example.test/r.git","commit":"`+strings.Repeat("a", 40)+
			`","languages":["go"]}`)
	for i := 0; i+1 < len(extra); i += 2 {
		artifact(t, filepath.Join(config, extra[i]), extra[i+1])
	}
	c, err := catalog.Load(config)
	if err != nil {
		t.Fatal(err)
	}
	return config, c
}

// declarable is the least a lab has to declare before a matrix can be proposed
// from it: a tool, two models from two providers, the pair of subjects a cell
// is, and somewhere to run them.
func declarable(t *testing.T, config string) {
	t.Helper()
	for _, kind := range []string{"subjects", "agents", "models", "repos", "executors"} {
		if err := os.MkdirAll(filepath.Join(config, kind), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	artifact(t, filepath.Join(config, "agents", "tool", "agent.json"),
		`{"id":"tool","binary":"/bin/echo","setup_tool":"tool-cli","transcript_format":"assistant-events",`+
			`"model_flag":"--model","config_dirs":[".tool"],"headless_args":["-c"],"judge_args":["-c"],`+
			`"env":[],"supports_mcp":true,"auth_modes":["api_key"]}`)
	artifact(t, filepath.Join(config, "models", "alpha-1.json"),
		`{"id":"alpha-1","provider":"alpha","aliases":[],"available_under":["api_key"],"agents":["tool"]}`)
	artifact(t, filepath.Join(config, "models", "beta-9.json"),
		`{"id":"beta-9","provider":"beta","aliases":[],"available_under":["api_key"],"agents":["tool"]}`)
	artifact(t, filepath.Join(config, "subjects", "untreated", "subject.json"),
		`{"id":"untreated","kind":"baseline","needs_mcp":false,"needs_isolated_config":false,`+
			`"executor":"isolated-home","agents":["tool"]}`)
	artifact(t, filepath.Join(config, "subjects", "sense-main", "subject.json"),
		`{"id":"sense-main","kind":"sense","needs_mcp":true,"needs_isolated_config":true,`+
			`"executor":"isolated-home","agents":["tool"]}`)
	artifact(t, filepath.Join(config, "executors", "isolated-home.json"),
		`{"id":"isolated-home","preserves_auth":["subscription","api_key"],"isolates_global_config":true}`)
}

// The starter is a matrix this catalog can actually run, written where the
// bench belongs, so the decision is made by editing rather than by authoring.
func TestTheStarterIsAMatrixThisCatalogCanRun(t *testing.T) {
	config, c := starterCatalog(t)

	b, at, written, err := starter(config, "fake-repo", c)
	if err != nil {
		t.Fatal(err)
	}
	if !written {
		t.Error("it reported writing nothing")
	}
	if b.Repo != "fake-repo" || b.Judge == "" || b.Driver.Model == "" {
		t.Errorf("the starter is incomplete: %+v", b)
	}
	if len(b.Subjects) != 2 {
		t.Errorf("subjects = %v, want the pair a cell is", b.Subjects)
	}

	// It is on disk, it parses as a bench, and the resolver clears every job.
	read, err := loadBench(config, "fake-repo")
	if err != nil {
		t.Fatalf("the file it wrote is not a bench: %v", err)
	}
	res, err := plan.Expand(c, read)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rejected) > 0 {
		t.Errorf("the starter it wrote has %d jobs that cannot run: %v", len(res.Rejected), res.Rejected)
	}
	if res.Cells() == 0 {
		t.Error("the starter plans no cells")
	}
	if !strings.HasSuffix(at, filepath.Join("benches", "fake-repo.json")) {
		t.Errorf("it wrote to %s", at)
	}
}

// The headline runs twice. The recorded spread within one cell reaches a
// quarter of the group against a bar of half of it, so a single run is a draw
// rather than a reading.
func TestTheHeadlineArmRunsMoreThanOnce(t *testing.T) {
	config, c := starterCatalog(t)

	b, _, _, err := starter(config, "fake-repo", c)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range b.Arms {
		if a.Role == plan.Headline && a.Runs < 2 {
			t.Errorf("the headline runs %d time(s); one run is a sample of a distribution "+
				"whose spread is half the bar", a.Runs)
		}
	}
}

// The confirmation arm is a model from another provider, because what it is
// asked is whether the result survives a change of model. A second model from
// the same provider would confirm less than it appears to.
func TestTheConfirmationArmIsAnotherProvider(t *testing.T) {
	config, c := starterCatalog(t)

	b, _, _, err := starter(config, "fake-repo", c)
	if err != nil {
		t.Fatal(err)
	}
	var headline, confirmation string
	for _, a := range b.Arms {
		if a.Role == plan.Confirmation {
			confirmation = a.Model
		} else {
			headline = a.Model
		}
	}
	if confirmation == "" {
		t.Fatalf("no confirmation arm was proposed: %+v", b.Arms)
	}
	if c.Models[confirmation].Provider == c.Models[headline].Provider {
		t.Errorf("%s confirms %s and they are the same provider", confirmation, headline)
	}
}

// A catalog with one provider proposes no confirmation arm rather than an arm
// that confirms nothing.
func TestOneProviderProposesNoConfirmationArm(t *testing.T) {
	config, c := starterCatalog(t, filepath.Join("models", "beta-9.json"),
		`{"id":"beta-9","provider":"alpha","aliases":[],"available_under":["api_key"],"agents":["tool"]}`)
	reloaded, err := catalog.Load(config)
	if err != nil {
		t.Fatal(err)
	}
	_ = c

	b, _, _, err := starter(config, "fake-repo", reloaded)
	if err != nil {
		t.Fatal(err)
	}
	if len(b.Arms) != 1 {
		t.Errorf("arms = %+v, want the headline alone", b.Arms)
	}
}

// A bench somebody has already written is the decision this exists to ask for.
// Admission runs on every invocation, so it must never overwrite one.
func TestAnExistingBenchIsNeverOverwritten(t *testing.T) {
	config, c := starterCatalog(t)
	mine := `{"repo":"fake-repo","judge":"beta-9","driver":{"agent":"tool","model":"beta-9"},` +
		`"subjects":["untreated","sense-main"],"arms":[{"role":"headline","model":"beta-9","runs":5}]}`
	artifact(t, filepath.Join(config, "benches", "fake-repo.json"), mine)

	b, at, written, err := starter(config, "fake-repo", c)
	if err != nil {
		t.Fatal(err)
	}
	if written {
		t.Error("it reported writing over a bench somebody wrote")
	}
	if b.Arms[0].Runs != 5 || b.Judge != "beta-9" {
		t.Errorf("it read back %+v, not what was on disk", b)
	}
	if got := readFile(t, at); got != mine {
		t.Errorf("the file changed:\n%s", got)
	}
	if !strings.Contains(starterTable(b, at, false), "already says") {
		t.Error("the page reports an existing bench as one it wrote")
	}
}

// A catalog that cannot imply a matrix says why, and does not write a starter
// that reads as something somebody chose.
func TestACatalogWithTwoSenseSubjectsCannotProposeACell(t *testing.T) {
	config, _ := starterCatalog(t, filepath.Join("subjects", "other-sense", "subject.json"),
		`{"id":"other-sense","kind":"sense","needs_mcp":true,"needs_isolated_config":true,`+
			`"executor":"isolated-home","agents":["tool"]}`)
	c, err := catalog.Load(config)
	if err != nil {
		t.Fatal(err)
	}

	_, at, written, err := starter(config, "fake-repo", c)
	if err == nil {
		t.Fatal("it proposed a cell from three subjects")
	}
	if !strings.Contains(err.Error(), "one subject without Sense against one with it") {
		t.Errorf("the reason does not say what a cell is: %v", err)
	}
	if written {
		t.Error("it wrote a starter it could not stand behind")
	}
	if _, err := os.Stat(at); err == nil {
		t.Error("a bench file was left on disk")
	}
}

// A lab that has not declared a model yet has no arm to propose. It says so
// rather than writing a bench with an empty matrix.
func TestALabWithNoModelsHasNoArmToPropose(t *testing.T) {
	config, _ := starterCatalog(t)
	for _, id := range []string{"alpha-1", "beta-9"} {
		if err := os.Remove(filepath.Join(config, "models", id+".json")); err != nil {
			t.Fatal(err)
		}
	}
	c, err := catalog.Load(config)
	if err != nil {
		t.Fatal(err)
	}

	_, at, written, err := starter(config, "fake-repo", c)
	if err == nil {
		t.Fatal("it proposed an arm with no models")
	}
	if !strings.Contains(err.Error(), "declares no models") {
		t.Errorf("the reason does not say what is missing: %v", err)
	}
	if written {
		t.Error("it wrote a starter with no arms")
	}
	if _, err := os.Stat(at); err == nil {
		t.Error("a bench file was left on disk")
	}
}

// The proposal is readable from the screen. The decision is made by a person
// looking at what it says, not by opening the JSON it wrote.
func TestTheProposalIsReadableFromTheScreen(t *testing.T) {
	config, c := starterCatalog(t)
	b, at, written, err := starter(config, "fake-repo", c)
	if err != nil {
		t.Fatal(err)
	}

	got := starterTable(b, at, written)
	for _, want := range []string{
		"what do we measure?", "the headline model", "which the win is claimed on",
		"a second model", "to confirm the result holds", "the comparison",
		"the judge", "the stages run by", "checked and can run", "These are defaults",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the proposal does not say %q:\n%s", want, got)
		}
	}
}

// Re-admitting a repository proposes what it proposed before. An order that
// moved would make two runs of the same command disagree about what this
// repository is measured on.
func TestTheProposalIsTheSameOnASecondRun(t *testing.T) {
	first, c := starterCatalog(t)
	second := filepath.Join(t.TempDir(), "lab")
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatal(err)
	}

	a, _, _, err := starter(first, "fake-repo", c)
	if err != nil {
		t.Fatal(err)
	}
	b, _, _, err := starter(second, "fake-repo", c)
	if err != nil {
		t.Fatal(err)
	}
	if a.Judge != b.Judge || a.Driver != b.Driver || len(a.Arms) != len(b.Arms) {
		t.Errorf("two runs proposed different matrices:\n%+v\n%+v", a, b)
	}
	for i := range a.Arms {
		if a.Arms[i] != b.Arms[i] {
			t.Errorf("arm %d differs: %+v against %+v", i, a.Arms[i], b.Arms[i])
		}
	}
}

// A competitor subject is a third arm, and a cell is two. It is passed over
// rather than counted as one of the pair.
func TestACompetitorSubjectIsNotOneOfThePair(t *testing.T) {
	config, _ := starterCatalog(t, filepath.Join("subjects", "archmcp", "subject.json"),
		`{"id":"archmcp","kind":"competitor","needs_mcp":true,"needs_isolated_config":false,`+
			`"executor":"isolated-home","agents":["tool"]}`)
	c, err := catalog.Load(config)
	if err != nil {
		t.Fatal(err)
	}

	b, _, _, err := starter(config, "fake-repo", c)
	if err != nil {
		t.Fatalf("a competitor in the catalog stopped a pair being proposed: %v", err)
	}
	if len(b.Subjects) != 2 {
		t.Errorf("subjects = %v, want the pair a cell is", b.Subjects)
	}
	for _, id := range b.Subjects {
		if id == "archmcp" {
			t.Errorf("the proposal names a competitor: %v", b.Subjects)
		}
	}
}

// A matrix whose jobs cannot all run is not written. A starter that resolves
// to nothing reads as something somebody chose, and the operator would spend a
// cycle finding out otherwise.
func TestAMatrixWithAnUnrunnableJobIsNotWritten(t *testing.T) {
	// A sense subject only another tool can drive, while the models are driven
	// by this one: every cell loses a side.
	config, _ := starterCatalog(t,
		filepath.Join("agents", "other-tool", "agent.json"),
		`{"id":"other-tool","binary":"/bin/echo","setup_tool":"other-cli",`+
			`"transcript_format":"assistant-events","model_flag":"--model","config_dirs":[".other"],`+
			`"headless_args":["-c"],"judge_args":["-c"],"env":[],"supports_mcp":true,"auth_modes":["api_key"]}`,
		filepath.Join("subjects", "sense-main", "subject.json"),
		`{"id":"sense-main","kind":"sense","needs_mcp":true,"needs_isolated_config":true,`+
			`"executor":"isolated-home","agents":["other-tool"]}`)
	c, err := catalog.Load(config)
	if err != nil {
		t.Fatal(err)
	}

	_, at, written, err := starter(config, "fake-repo", c)
	if err == nil {
		t.Fatal("it wrote a starter whose jobs cannot run")
	}
	if !strings.Contains(err.Error(), "cannot run") {
		t.Errorf("the reason does not say what is wrong: %v", err)
	}
	if written {
		t.Error("it reported writing a starter it refused")
	}
	if _, err := os.Stat(at); err == nil {
		t.Error("a bench file was left on disk")
	}
}

// A matrix for a repository the catalog does not hold cannot be resolved, and
// is reported rather than written. It is the shape of a starter proposed
// against a catalog read before the repository was admitted into it.
func TestAMatrixForAnUnknownRepositoryIsRefused(t *testing.T) {
	config, c := starterCatalog(t)

	_, _, written, err := starter(config, "never-admitted", c)
	if err == nil {
		t.Fatal("it proposed a matrix for a repository nothing declares")
	}
	if !strings.Contains(err.Error(), "does not resolve") {
		t.Errorf("the reason does not say what failed: %v", err)
	}
	if written {
		t.Error("it wrote a matrix it could not resolve")
	}
}

// A bench that cannot be written is reported. A starter the operator is told
// about and which is not on disk is worse than none.
func TestABenchThatCannotBeWrittenIsReported(t *testing.T) {
	config, c := starterCatalog(t)
	// A file where the benches directory belongs.
	artifact(t, filepath.Join(config, "benches"), "not a directory")

	_, _, written, err := starter(config, "fake-repo", c)
	if err == nil {
		t.Fatal("writing into a file reported no error")
	}
	if written {
		t.Error("it reported writing a starter it could not write")
	}
}
