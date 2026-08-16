package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// planCatalog is a config directory with two executors, so a test can vary
// where a subject runs and watch resolution answer differently.
func planCatalog(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for path, body := range map[string]string{
		"agents/tool/agent.json": `{"id":"tool","binary":"b","model_flag":"--model",
		  "headless_args":["-p"],"env":[],"supports_mcp":true,"auth_modes":["api_key"]}`,
		"subjects/untreated/subject.json": `{"id":"untreated","kind":"baseline","needs_mcp":false,
		  "needs_isolated_config":false,"executor":"isolated-home","agents":["tool"]}`,
		"subjects/sense/subject.json": `{"id":"sense","kind":"sense","needs_mcp":true,
		  "needs_isolated_config":true,"executor":"isolated-home","agents":["tool"]}`,
		"models/m1.json":    `{"id":"m1","provider":"acme","aliases":[],"available_under":["api_key"],"agents":["tool"]}`,
		"models/judge.json": `{"id":"judge","provider":"acme","aliases":[],"available_under":["api_key"],"agents":["tool"]}`,
		"repos/r1.json":     `{"id":"r1","url":"u","commit":"abc","languages":["go"],"stack":"go"}`,
		"executors/isolated-home.json": `{"id":"isolated-home","preserves_auth":["api_key"],
		  "isolates_global_config":true}`,
		"executors/local.json": `{"id":"local","preserves_auth":["api_key"],"isolates_global_config":false}`,
		"campaigns/c/campaign.json": `{"key":"c","judge":"judge","subjects":["untreated","sense"],
		  "repos":["r1"],"arms":[{"role":"headline","model":"m1","runs":2}]}`,
	} {
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestPlanPrintsWhatWouldRun(t *testing.T) {
	code, stdout, stderr := dispatch(t, "plan", "-config", planCatalog(t), "-campaign", "c")

	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	for _, want := range []string{
		"campaign c",
		"judge    judge (pinned, not an arm)",
		"WILL RUN: 1 cells, 4 sessions", // one repo, one arm, two subjects
		"headline     r1 / untreated / tool / m1 / isolated-home / api_key x2",
		"headline     r1 / sense / tool / m1 / isolated-home / api_key x2",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output is missing %q:\n%s", want, stdout)
		}
	}
}

// A rejection without a reason is a mystery someone routes around by guessing,
// so the reason is printed with the job it belongs to, and the command exits
// non-zero so an unattended caller cannot proceed as though the matrix were
// whole.
func TestPlanPrintsEveryRejectionWithItsReason(t *testing.T) {
	dir := planCatalog(t)
	// The sense subject writes agent config; local does not isolate it.
	if err := os.WriteFile(filepath.Join(dir, "subjects", "sense", "subject.json"),
		[]byte(`{"id":"sense","kind":"sense","needs_mcp":true,"needs_isolated_config":true,
		  "executor":"local","agents":["tool"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := dispatch(t, "plan", "-config", dir, "-campaign", "c")

	// Its own code: a caller must not proceed on a partial matrix, but "your
	// config is broken" and "part of your matrix cannot run" are opposite
	// actions and both used to exit 1.
	if code != 5 {
		t.Errorf("exit = %d, want 5 for a plan with a hole in it", code)
	}
	// BOTH sides go: a cell that lost a subject is a half-pair, and planning
	// the survivor guarantees a burned arm.
	for _, want := range []string{
		"WILL RUN: 0 cells",
		"WILL NOT RUN: 2",
		"r1 / sense / tool / m1 / local",
		"would leak onto the host and into the next run",
		"its cell is incomplete, so running this would burn it",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output is missing %q:\n%s", want, stdout)
		}
	}
}

// A job the catalog says cannot work must never reach a spawn. This is the
// half-pair hazard: a combination that was never going to run burns the arm
// that did.
//
// The break used here is one a VALID catalog permits — a model driven only by a
// tool no subject supports. A model nothing can authenticate to would be
// rejected earlier, when the catalog loads, which is a different and better
// failure: see TestSomeRejectionsCannotBeReachedThroughAValidCatalog.
func TestADeliberatelyBrokenCombinationPlansNothingForThatArm(t *testing.T) {
	dir := planCatalog(t)
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A second tool that exists and is reachable, but that no subject supports.
	write("agents/lonely/agent.json", `{"id":"lonely","binary":"l","model_flag":"-m",
	  "headless_args":["-p"],"env":[],"supports_mcp":true,"auth_modes":["api_key"]}`)
	write("models/m1.json", `{"id":"m1","provider":"acme","aliases":[],
	  "available_under":["api_key"],"agents":["lonely"]}`)

	code, stdout, _ := dispatch(t, "plan", "-config", dir, "-campaign", "c")

	if code == 0 {
		t.Error("exit = 0 for a campaign nothing in which can run")
	}
	if !strings.Contains(stdout, "WILL RUN: 0 cells") {
		t.Errorf("something was planned anyway:\n%s", stdout)
	}
	if !strings.Contains(stdout, "cannot be driven by lonely") {
		t.Errorf("output does not say why:\n%s", stdout)
	}
}

// ONE of the resolution questions cannot fire through a catalog that loads:
// "is this model reachable through this tool", which the catalog answers first
// and refuses the whole config over. That is the better failure — an internally
// inconsistent catalog is a mistake, not a job that happens not to fit.
//
// It is recorded here so nobody later reads that branch as dead code and
// deletes it: without it, the executor question's message degenerates to
// "needs one of []". It is a guard behind an invariant the catalog owns, and it
// is tested directly against an in-memory catalog in the plan package.
//
// The executor question IS reachable — a subject may name the container, which
// preserves nothing — and TestPlanPrintsEveryRejectionWithItsReason exercises
// the isolation question. An earlier version of this comment claimed two
// questions were unreachable and demonstrated one.
func TestOneRejectionCannotBeReachedThroughAValidCatalog(t *testing.T) {
	dir := planCatalog(t)
	if err := os.WriteFile(filepath.Join(dir, "models", "m1.json"),
		[]byte(`{"id":"m1","provider":"acme","aliases":[],"available_under":["subscription"],"agents":["tool"]}`),
		0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := dispatch(t, "plan", "-config", dir, "-campaign", "c")

	if code == 0 {
		t.Error("a model nothing can authenticate to was accepted")
	}
	if !strings.Contains(stderr, "nothing could reach it") {
		t.Errorf("stderr = %q, want the catalog to refuse it at load", stderr)
	}
	if stdout != "" {
		t.Errorf("a plan was printed for a config that does not load:\n%s", stdout)
	}
}

// Adding an arm is one line in the campaign and one model file. Nothing else:
// no dispatch to edit, no prefix matching to extend. That is the whole point of
// the catalog, and it is what failed when a runner rewrote an arm's id into
// something that did not exist.
func TestAddingAnArmIsOneFileAndOneLine(t *testing.T) {
	dir := planCatalog(t)
	before := planned(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "models", "m2.json"),
		[]byte(`{"id":"m2","provider":"acme","aliases":[],"available_under":["api_key"],"agents":["tool"]}`),
		0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "campaigns", "c", "campaign.json"),
		[]byte(`{"key":"c","judge":"judge","subjects":["untreated","sense"],"repos":["r1"],
		  "arms":[{"role":"headline","model":"m1","runs":2},{"role":"confirmation","model":"m2","runs":2}]}`),
		0o644); err != nil {
		t.Fatal(err)
	}

	after := planned(t, dir)

	if !strings.Contains(after, "m2") {
		t.Errorf("the new arm does not appear in the plan:\n%s", after)
	}
	if strings.Contains(before, "m2") {
		t.Error("the arm was in the plan before it was added")
	}
	if !strings.Contains(after, "WILL RUN: 2 cells, 8 sessions") {
		t.Errorf("the new arm did not add a cell:\n%s", after)
	}
}

// The "one file" claim is narrower than it reads, and the narrow part is worth
// pinning: an arm on a tool no subject lists needs an edit to every subject
// too. That is correct behaviour — a subject decides what can drive it — but
// somebody reading "adding an arm is one new file" should not be surprised by
// it.
func TestAnArmOnANewToolAlsoNeedsEverySubjectToAllowThatTool(t *testing.T) {
	dir := planCatalog(t)
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("agents/newtool/agent.json", `{"id":"newtool","binary":"n","model_flag":"-m",
	  "headless_args":["-p"],"env":[],"supports_mcp":true,"auth_modes":["api_key"]}`)
	write("models/m2.json", `{"id":"m2","provider":"acme","aliases":[],
	  "available_under":["api_key"],"agents":["newtool"]}`)
	write("campaigns/c/campaign.json", `{"key":"c","judge":"judge","subjects":["untreated","sense"],
	  "repos":["r1"],"arms":[{"role":"headline","model":"m1","runs":2},
	  {"role":"confirmation","model":"m2","runs":2}]}`)

	code, stdout, _ := dispatch(t, "plan", "-config", dir, "-campaign", "c")

	if code != 5 {
		t.Errorf("exit = %d, want 5", code)
	}
	if !strings.Contains(stdout, "cannot be driven by newtool; it supports [tool]") {
		t.Errorf("the plan does not say what else needs editing:\n%s", stdout)
	}
	// And the existing arm is untouched.
	if !strings.Contains(stdout, "WILL RUN: 1 cells") {
		t.Errorf("the new arm took the existing one with it:\n%s", stdout)
	}
}

func planned(t *testing.T, dir string) string {
	t.Helper()
	code, stdout, stderr := dispatch(t, "plan", "-config", dir, "-campaign", "c")
	if code != 0 {
		t.Fatalf("plan exit = %d (stderr: %s)", code, stderr)
	}
	return stdout
}

func TestPlanRejectsWhatItCannotRead(t *testing.T) {
	dir := planCatalog(t)

	for _, tc := range []struct {
		name     string
		args     []string
		want     string
		wantCode int
	}{
		{"no campaign named", []string{"-config", dir}, "-campaign is required", 2},
		{"an unknown flag", []string{"-config", dir, "-campaign", "c", "-arm", "x"},
			"flag provided but not defined", 2},
		{"a campaign that does not exist", []string{"-config", dir, "-campaign", "nope"},
			"read campaign", 1},
		{"no config directory", []string{"-config", filepath.Join(t.TempDir(), "gone"), "-campaign", "c"},
			"no config directory", 1},
		{"a campaign that is malformed rather than unsatisfiable", []string{
			"-config", malformedCampaign(t), "-campaign", "c"},
			"has 2 headline arms", 1},
		{"a campaign naming a subject twice", []string{
			"-config", duplicateSubject(t), "-campaign", "c"},
			`names subject "untreated" twice; a duplicated arm is not a pair`, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, _, stderr := dispatch(t, append([]string{"plan"}, tc.args...)...)

			if code != tc.wantCode {
				t.Errorf("exit = %d, want %d", code, tc.wantCode)
			}
			if !strings.Contains(stderr, tc.want) {
				t.Errorf("stderr = %q, want %q", stderr, tc.want)
			}
		})
	}
}

// A campaign file whose key disagrees with its directory is ambiguous about
// which campaign it is, and every later artifact is keyed on one of the two.
// malformedCampaign is a config directory whose campaign cannot be read as a
// campaign at all. That is a different answer from a campaign whose jobs cannot
// run: one sends you to fix the campaign file, the other to fix the catalog.
func malformedCampaign(t *testing.T) string {
	t.Helper()
	dir := planCatalog(t)
	if err := os.WriteFile(filepath.Join(dir, "campaigns", "c", "campaign.json"),
		[]byte(`{"key":"c","judge":"judge","subjects":["untreated"],"repos":["r1"],
		  "arms":[{"role":"headline","model":"m1","runs":2},{"role":"headline","model":"judge","runs":2}]}`),
		0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// A list naming the same subject twice is the hazard in disguise: two untreated
// arms and no sense arm looks complete to anything counting jobs.
func duplicateSubject(t *testing.T) string {
	t.Helper()
	dir := planCatalog(t)
	if err := os.WriteFile(filepath.Join(dir, "campaigns", "c", "campaign.json"),
		[]byte(`{"key":"c","judge":"judge","subjects":["untreated","untreated"],"repos":["r1"],
		  "arms":[{"role":"headline","model":"m1","runs":2}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestACampaignKeyMustMatchItsDirectory(t *testing.T) {
	dir := planCatalog(t)
	if err := os.WriteFile(filepath.Join(dir, "campaigns", "c", "campaign.json"),
		[]byte(`{"key":"other","judge":"judge","subjects":["untreated"],"repos":["r1"],
		  "arms":[{"role":"headline","model":"m1","runs":2}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := dispatch(t, "plan", "-config", dir, "-campaign", "c")

	if code == 0 {
		t.Error("a mismatched campaign key was accepted")
	}
	if !strings.Contains(stderr, `declares key "other" but lives under "c"`) {
		t.Errorf("stderr = %q", stderr)
	}
}

// A typo'd key in a campaign file is a setting that looks applied and is not,
// for the same reason it is in the catalog.
func TestAnUnknownFieldInACampaignIsRefused(t *testing.T) {
	dir := planCatalog(t)
	if err := os.WriteFile(filepath.Join(dir, "campaigns", "c", "campaign.json"),
		[]byte(`{"key":"c","judge":"judge","subjects":["untreated"],"repos":["r1"],"wall":"8m",
		  "arms":[{"role":"headline","model":"m1","runs":2}]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := dispatch(t, "plan", "-config", dir, "-campaign", "c")

	if code == 0 || !strings.Contains(stderr, "unknown field") {
		t.Errorf("exit = %d, stderr = %q", code, stderr)
	}
}

// The shipped campaign is the reference this pitch is checked against: the
// csharp-aspnet arm set as of a named commit, planning to the cells that
// campaign had. A broken one must fail here rather than in a session.
func TestTheShippedCampaignPlansTheCellsItHad(t *testing.T) {
	code, stdout, stderr := dispatch(t, "plan", "-config", repoConfigDir(t), "-campaign", "csharp-aspnet")

	if code != 0 {
		t.Fatalf("the shipped campaign does not plan: %s", stderr)
	}
	// One repository and five arms is five CELLS; two subjects makes each one a
	// pair, so twenty sessions. A cell is the unit that is paired, budgeted and
	// banked, and counting jobs as cells doubles every board.
	if !strings.Contains(stdout, "WILL RUN: 5 cells, 20 sessions") {
		t.Errorf("plan is not the frozen arm set:\n%s", stdout)
	}
	// The headline cell is the one that banked: both arms, on the headline
	// model, against the repository the campaign had reached.
	for _, want := range []string{
		"headline     bitwarden-server / untreated / claude-code / claude-opus-5 / isolated-home / subscription x2",
		"headline     bitwarden-server / sense-main / claude-code / claude-opus-5 / isolated-home / subscription x2",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("the banked cell is not planned:\n%s", stdout)
		}
	}
	// The arm whose id cost a campaign to debug, in the form that works.
	if !strings.Contains(stdout, "ollama-cloud/mistral-large-3:675b") {
		t.Errorf("the mistral arm is missing:\n%s", stdout)
	}
	if strings.Contains(stdout, "WILL NOT RUN") {
		t.Errorf("the frozen arm set rejects something:\n%s", stdout)
	}
}

func TestPlanHelpIsNotReportedAsAnError(t *testing.T) {
	_, _, stderr := dispatch(t, "plan", "-help")

	if strings.Contains(stderr, "sense-lab plan:") {
		t.Errorf("asking for help produced an error message:\n%s", stderr)
	}
}

// `plan` and `run` must agree on what a model NAME is. `run` resolves through
// the alias index; if `plan` looked only at the id map it would report "no
// model file declares it" for a name that a model file does declare — a false
// statement aimed at exactly the person who just wrote the id in the other
// form, which is the confusion that cost an arm in the first place.
func TestACampaignMayNameAModelByItsAlias(t *testing.T) {
	dir := planCatalog(t)
	write := func(path, body string) {
		t.Helper()
		full := filepath.Join(dir, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("models/long.json", `{"id":"provider/long-model:675b","provider":"acme",
	  "aliases":["long-model:675b"],"available_under":["api_key"],"agents":["tool"]}`)
	// The campaign names the SHORT form.
	write("campaigns/c/campaign.json", `{"key":"c","judge":"judge","subjects":["untreated","sense"],
	  "repos":["r1"],"arms":[{"role":"headline","model":"long-model:675b","runs":2}]}`)

	code, stdout, stderr := dispatch(t, "plan", "-config", dir, "-campaign", "c")

	if code != 0 {
		t.Fatalf("a campaign naming a model by its alias did not plan: %s", stderr)
	}
	// And the plan shows the canonical id, so one plan names one form.
	if !strings.Contains(stdout, "provider/long-model:675b") {
		t.Errorf("the plan does not name the canonical id:\n%s", stdout)
	}
	if strings.Contains(stdout, "/ long-model:675b /") {
		t.Errorf("the plan names the alias as though it were the model:\n%s", stdout)
	}
}
