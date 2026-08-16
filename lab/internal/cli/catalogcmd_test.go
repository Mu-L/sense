package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/catalog"
)

// The claim this pitch makes is that no competitor, agent or model identifier
// is compiled in. These are the two probes that check it, and they are
// deliberately different from each other: a grep over the source is one probe
// for a negative claim, and a negative claim needs a second.

// Probe one: with no config directory, nothing can be planned. If any target
// were still compiled in, something here would still work.
func TestNothingCanBePlannedWithoutAConfigDirectory(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "no-such-config")

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"catalog", []string{"catalog", "-config", absent}},
		{"run", []string{"run", "-config", absent,
			"-scenario", scenarioFile(t, twoStepScenario), "-repo", "discourse",
			"-checkout", t.TempDir(), "-out", filepath.Join(t.TempDir(), "r"),
			"-agent", "claude-code", "-model", "claude-opus-5"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			code, stdout, stderr := dispatch(t, tc.args...)

			if code == 0 {
				t.Error("exit = 0 with no config directory; something is compiled in")
			}
			if !strings.Contains(stderr, "no config directory") {
				t.Errorf("stderr = %q, want it to name the missing config", stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want nothing planned", stdout)
			}
		})
	}
}

// Probe two, and a genuinely different one: read the BUILT BINARY, and derive
// what to look for FROM THE CATALOG rather than from memory.
//
// The deriving is the point. A hand-written list of names is a list someone
// thought of on one afternoon, and it goes stale the moment an arm is added —
// which is exactly how the incident this pitch is named for arrived. A list
// taken from the config cannot go stale, because adding an arm adds to the
// list.
//
// This probe has twice caught what the source read as clean. It found
// `bypassPermissions`, `IS_SANDBOX=1`, `CLAUDE_CODE_DISABLE_AUTO_MEMORY=1` and
// `stream-json` compiled in, which moved them into agent.json. Then, with a
// hand-written list, it MISSED a deliberately planted dispatch on `opencode`
// and a `glm-5.2:cloud` prefix — 19 real catalog identifiers were unchecked.
// The derived list fires on all three.
func TestNoCatalogIdentifierSurvivesIntoTheBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	bin := filepath.Join(t.TempDir(), "sense-lab")
	build := exec.Command("go", "build", "-o", bin, "../../cmd/sense-lab")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	b, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	binary := string(b)

	forbidden := catalogIdentifiers(t)
	if len(forbidden) < 20 {
		t.Fatalf("derived only %d identifiers from the catalog; the derivation is not "+
			"reading what it thinks", len(forbidden))
	}
	for _, id := range forbidden {
		if strings.Contains(binary, id) {
			t.Errorf("%q comes from the catalog and is compiled into the binary", id)
		}
	}
}

// catalogIdentifiers is every string the catalog names that must not be
// compiled in: ids, providers, aliases, binaries, flags, env and config paths.
//
// Strings under five characters are skipped. `-p`, `run` and `exec` appear in
// any Go binary, and they are the only three false positives; every identifier
// that matters is longer.
func catalogIdentifiers(t *testing.T) []string {
	t.Helper()
	c, err := catalog.Load(repoConfigDir(t))
	if err != nil {
		t.Fatalf("load the shipped catalog: %v", err)
	}

	// No exemptions. An earlier version skipped the three subject KINDS,
	// because they are a closed enum the validator legitimately compiles in and
	// the baseline subject's id collided with one. That exemption was keyed on
	// the STRING rather than on the field it came from, so it hid those names
	// wherever they appeared — and a planted dispatch on "baseline" passed.
	// The collision is fixed in the config instead, by giving the subject an id
	// that is not also a kind, which leaves nothing here to justify.
	var out []string
	add := func(vals ...string) {
		for _, v := range vals {
			if len(v) >= 5 {
				out = append(out, v)
			}
		}
	}
	for _, id := range catalog.IDs(c.Subjects) {
		add(id)
	}
	for _, id := range catalog.IDs(c.Agents) {
		a := c.Agents[id]
		add(id, a.Binary, a.ModelFlag)
		add(a.HeadlessArgs...)
		add(a.Env...)
		add(a.AuthModes...)
	}
	for _, id := range catalog.IDs(c.Models) {
		m := c.Models[id]
		add(id, m.Provider)
		add(m.Aliases...)
		add(m.AvailableUnder...)
	}
	for _, id := range catalog.IDs(c.Repos) {
		r := c.Repos[id]
		// The commit especially: checkoutIsAtPin is the site that attracts a
		// "this repository is special" hard-code, and a 40-character hash
		// cannot collide with anything by accident.
		add(id, r.Stack, r.Commit, r.URL)
	}
	return out
}

func TestCatalogHelpIsNotReportedAsAnError(t *testing.T) {
	_, _, stderr := dispatch(t, "catalog", "-help")

	if strings.Contains(stderr, "sense-lab catalog:") {
		t.Errorf("asking for help produced an error message:\n%s", stderr)
	}
}

// What the command PRINTS, against a fixture. Pointing this at the shipped
// catalog would make it fail whenever a model is retired — a legitimate edit
// that says nothing about the printing this test is for. The shipped catalog's
// validity is TestTheShippedCatalogIsValid's job.
func TestCatalogPrintsEveryKindWithWhatIdentifiesIt(t *testing.T) {
	cfg := testCatalog(t, "/bin/true")

	code, stdout, stderr := dispatch(t, "catalog", "-config", cfg)

	if code != 0 {
		t.Fatalf("exit = %d (stderr: %s)", code, stderr)
	}
	// Each line carries the id AND the fact that distinguishes one entry from
	// another of the same kind. An id-only listing would not tell two models
	// apart.
	for _, want := range []string{
		"subject  untreated", "baseline\n", // id and kind
		"agent    tool", "/bin/true\n", // id and binary
		"model    m1", "acme\n", // id and provider
		"repo     r1", "abc123\n", // id and pinned commit
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("output is missing %q:\n%s", want, stdout)
		}
	}
}

// repoConfigDir is this repository's own lab/ config directory, which is the
// catalog the bench actually uses. Testing against it rather than a fixture is
// what makes a malformed real config fail `make test` instead of a run.
func repoConfigDir(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "lab")
}

// The shipped catalog is not a fixture: it is the config every run reads. A
// broken one must fail here, cheaply, rather than forty minutes into a paid
// session.
func TestTheShippedCatalogIsValid(t *testing.T) {
	if code, _, stderr := dispatch(t, "catalog", "-config", repoConfigDir(t)); code != 0 {
		t.Fatalf("the repository's own catalog does not load: %s", stderr)
	}
}

// Every subject and agent carries a README: the human half, with the version
// quirks and side effects no schema can hold. A directory without one is a
// treatment nobody wrote down.
func TestEverySubjectAndAgentHasItsHumanHalf(t *testing.T) {
	for _, kind := range []string{"subjects", "agents"} {
		entries, err := os.ReadDir(filepath.Join(repoConfigDir(t), kind))
		if err != nil {
			t.Fatal(err)
		}
		var seen int
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			seen++
			readme := filepath.Join(repoConfigDir(t), kind, e.Name(), "README.md")
			if _, err := os.Stat(readme); err != nil {
				t.Errorf("%s/%s has no README.md", kind, e.Name())
			}
		}
		if seen == 0 {
			t.Errorf("found no %s at all; the walk is not looking where it thinks", kind)
		}
	}
}
