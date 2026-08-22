package cli

// The fixtures more than one command's tests are built from.
//
// They lived in the tests of `run`, which was the first command to need a
// scenario, a catalog and a way to read a file back. That command is gone — it
// spawned a single unisolated session and took no subject, so a tree built with
// it carried arm names and no arms — and these outlived it because what they
// describe is the lab, not the verb.
//
// Only what is still used. The rest went with the command: a fixture nothing
// builds on is a description of a world that no longer exists, and it reads as
// though somebody meant to use it.

import (
	"os"
	"path/filepath"
	"testing"
)

// read is a file's contents, or a failed test. Every caller here is asserting
// on what was written, so an unreadable file is never the thing under test.
func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

// scenarioSet writes a scenario AND the gold and rubric beside it, returning
// the scenario's path. A scenario is three files, and a test that wrote one
// would be building a set that cannot load.
func scenarioSet(t *testing.T, scenario, gold, rubric string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{
		"scenario.yaml":        scenario,
		"scenario.gold.yaml":   gold,
		"scenario.rubric.yaml": rubric,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "scenario.yaml")
}

// testCatalog writes a config directory holding one agent tool that spawns bin,
// one model it can drive, one subject and one repository — the least a command
// can be pointed at that loads.
func testCatalog(t *testing.T, bin string) string {
	t.Helper()
	dir := t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(dir, "agents", "tool", "agent.json"), `{"id":"tool","binary":"`+bin+`","setup_tool":"tool-cli","transcript_format":"assistant-events",
	  "model_flag":"--model","config_dirs":[".tool"],"headless_args":["-p","--permission-mode","bypassPermissions"],
	  "env":["IS_SANDBOX=1","CLAUDE_CODE_DISABLE_AUTO_MEMORY=1"],
	  "supports_mcp":true,"auth_modes":["api_key"]}`)
	write(filepath.Join(dir, "subjects", "untreated", "subject.json"),
		`{"id":"untreated","kind":"baseline","executor":"isolated-home","agents":["tool"]}`)
	write(filepath.Join(dir, "executors", "isolated-home.json"),
		`{"id":"isolated-home","preserves_auth":["api_key"],"isolates_global_config":true}`)
	write(filepath.Join(dir, "models", "m1.json"),
		`{"id":"m1","provider":"acme","available_under":["api_key"],"agents":["tool"]}`)
	write(filepath.Join(dir, "repos", "r1.json"),
		`{"id":"r1","url":"https://example.test/r1.git","commit":"abc123","languages":["go"]}`)
	return dir
}
