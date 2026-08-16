package isolate_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/isolate"
	"github.com/luuuc/sense/lab/internal/run"
)

// runRoot is a path under a fresh temp directory that does not yet exist,
// because Prepare refuses a root that does.
func runRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "run-1")
}

func TestPrepareCreatesTheRunTree(t *testing.T) {
	env, err := isolate.Prepare(isolate.Spec{Root: runRoot(t), Arm: isolate.Sense, Lookup: nothingSet})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	for _, dir := range []string{
		env.Home,
		filepath.Join(env.Home, "config"),
		filepath.Join(env.Home, "cache"),
		filepath.Join(env.Home, "data"),
		filepath.Join(env.Home, "state"),
		env.Logs,
		env.Artifacts,
	} {
		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("%s: %v", dir, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

func TestPrepareLeavesTheWorktreePathForGitToCreate(t *testing.T) {
	// git worktree add refuses a path that already exists, so creating repo/
	// here would break 03-02 in a way only a live git call would show.
	env, err := isolate.Prepare(isolate.Spec{Root: runRoot(t), Lookup: nothingSet})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if _, err := os.Stat(env.Repo); !os.IsNotExist(err) {
		t.Errorf("Stat(%s) = %v, want the path not to exist yet", env.Repo, err)
	}
}

func TestPrepareWithNoLookupReadsTheRealHost(t *testing.T) {
	t.Setenv("TERM", "xterm-from-the-host")

	env, err := isolate.Prepare(isolate.Spec{Root: runRoot(t)})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	if !slices.Contains(env.Environ, "TERM=xterm-from-the-host") {
		t.Errorf("environment %v does not carry the host's TERM; the default lookup is not reading the process environment", env.Environ)
	}
}

func TestPrepareRefusesAnEnvironmentThatAlreadyExists(t *testing.T) {
	root := t.TempDir() // exists, and may hold a recorded run

	_, err := isolate.Prepare(isolate.Spec{Root: root, Lookup: nothingSet})

	if err == nil {
		t.Fatal("Prepare reused an existing directory; a run's environment is created fresh or not at all")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Prepare error = %v, want it to say the root already exists", err)
	}
}

func TestPrepareWithoutARootIsRefused(t *testing.T) {
	if _, err := isolate.Prepare(isolate.Spec{}); err == nil {
		t.Fatal("Prepare accepted an empty root")
	}
}

func TestPrepareFailsWhenTheRunTreeCannotBeCreated(t *testing.T) {
	// A file where the run root should be. MkdirAll then cannot proceed, and
	// the failure must be reported rather than leaving a half-built tree that
	// a session would run inside.
	parent := t.TempDir()
	blocked := filepath.Join(parent, "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := isolate.Prepare(isolate.Spec{Root: filepath.Join(blocked, "run-1"), Lookup: nothingSet})

	if err == nil {
		t.Fatal("Prepare succeeded with an unusable run root")
	}
}

func TestCleanupRemovesTheWholeEnvironment(t *testing.T) {
	env, err := isolate.Prepare(isolate.Spec{Root: runRoot(t), Lookup: nothingSet})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	// State a run would leave behind: a cache the next run must not inherit.
	if err := os.WriteFile(filepath.Join(env.Home, "cache", "warm"), []byte("state"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := isolate.Cleanup(env); err != nil {
		t.Fatalf("Cleanup: %v", err)
	}

	if _, err := os.Stat(env.Root); !os.IsNotExist(err) {
		t.Errorf("Stat(%s) = %v after cleanup, want the tree to be gone", env.Root, err)
	}
}

func TestCleanupReportsAnEnvironmentItCouldNotRemove(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("relies on POSIX directory permissions")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission that makes removal fail")
	}
	env, err := isolate.Prepare(isolate.Spec{Root: runRoot(t), Lookup: nothingSet})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	// A directory whose parent denies write cannot be removed. Cleanup must say
	// so: a run that silently left its home behind contaminates the next one,
	// and the failure would then be read as the model's.
	if err := os.Chmod(env.Home, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(env.Home, 0o700) })
	if err := os.WriteFile(filepath.Join(env.Logs, "kept"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(env.Root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(env.Root, 0o700) })

	if err := isolate.Cleanup(env); err == nil {
		t.Fatal("Cleanup reported success on an environment it could not remove")
	}
}

func TestCleanupWithoutARootIsRefused(t *testing.T) {
	if err := isolate.Cleanup(isolate.Env{}); err == nil {
		t.Fatal("Cleanup accepted an environment with no root; that is one os.RemoveAll away from a very bad day")
	}
}

// The process-level proof. Everything above states what the environment is;
// these run a command inside it and read what the process actually saw.

func TestTheProcessWritesIntoTheDisposableHomeAndXDGDirectories(t *testing.T) {
	env, err := isolate.Prepare(isolate.Spec{
		Root:     runRoot(t),
		Arm:      isolate.Sense,
		HostPath: os.Getenv("PATH"),
		Lookup:   nothingSet,
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out")

	// Written by the process, from the variables the process was given. If HOME
	// or any XDG variable still pointed at the host, the files would land there.
	script := `set -e
	echo home > "$HOME/wrote-home"
	echo config > "$XDG_CONFIG_HOME/wrote-config"
	echo cache > "$XDG_CACHE_HOME/wrote-cache"
	echo data > "$XDG_DATA_HOME/wrote-data"
	echo state > "$XDG_STATE_HOME/wrote-state"`
	m, err := run.Session(context.Background(), out, run.Spec{
		Dir:  env.Root,
		Name: "/bin/sh",
		Args: []string{"-c", script},
		Env:  env.Environ,
		Arm:  string(env.Arm),
		Wall: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("Session: %v", err)
	}
	if m.Outcome != run.Completed {
		stderr, _ := os.ReadFile(filepath.Join(out, "raw", "stderr"))
		t.Fatalf("outcome = %s, want completed; stderr: %s", m.Outcome, stderr)
	}

	for _, path := range []string{
		filepath.Join(env.Home, "wrote-home"),
		filepath.Join(env.Home, "config", "wrote-config"),
		filepath.Join(env.Home, "cache", "wrote-cache"),
		filepath.Join(env.Home, "data", "wrote-data"),
		filepath.Join(env.Home, "state", "wrote-state"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}

func TestTheProcessDoesNotSeeTheHostsHome(t *testing.T) {
	env, err := isolate.Prepare(isolate.Spec{
		Root:     runRoot(t),
		HostPath: os.Getenv("PATH"),
		Lookup:   nothingSet,
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out")

	if _, err := run.Session(context.Background(), out, run.Spec{
		Dir:  env.Root,
		Name: "/bin/sh",
		Args: []string{"-c", `printenv HOME`},
		Env:  env.Environ,
		Wall: 30 * time.Second,
	}); err != nil {
		t.Fatalf("Session: %v", err)
	}

	said := strings.TrimSpace(readFile(t, filepath.Join(out, "raw", "stdout")))
	if said != env.Home {
		t.Errorf("the process saw HOME=%q, want the disposable %q", said, env.Home)
	}
	if host, ok := os.LookupEnv("HOME"); ok && said == host {
		t.Errorf("the process saw the host's HOME %q", host)
	}
}

func TestBothArmsRecordTheirPathAndOnlyOneReachesSense(t *testing.T) {
	senseBin := t.TempDir()
	hostPath := strings.Join([]string{senseBin, "/usr/bin", "/bin"}, string(filepath.ListSeparator))

	paths := map[isolate.Arm]string{}
	for _, arm := range []isolate.Arm{isolate.Sense, isolate.Baseline} {
		env, err := isolate.Prepare(isolate.Spec{
			Root:        filepath.Join(t.TempDir(), "run-"+string(arm)),
			Arm:         arm,
			SenseBinDir: senseBin,
			HostPath:    hostPath,
			Lookup:      nothingSet,
		})
		if err != nil {
			t.Fatalf("Prepare(%s): %v", arm, err)
		}
		out := filepath.Join(t.TempDir(), "out-"+string(arm))
		if _, err := run.Session(context.Background(), out, run.Spec{
			Dir:  env.Root,
			Name: "/bin/sh",
			Args: []string{"-c", "true"},
			Env:  env.Environ,
			Arm:  string(arm),
			Wall: 30 * time.Second,
		}); err != nil {
			t.Fatalf("Session(%s): %v", arm, err)
		}

		var meta run.Meta
		if err := json.Unmarshal([]byte(readFile(t, filepath.Join(out, "run-meta.json"))), &meta); err != nil {
			t.Fatalf("read run-meta for %s: %v", arm, err)
		}
		if meta.Arm != string(arm) {
			t.Errorf("run-meta arm = %q, want %q", meta.Arm, arm)
		}
		if meta.Home != env.Home {
			t.Errorf("run-meta home = %q, want %q", meta.Home, env.Home)
		}
		if meta.Path == "" {
			t.Fatalf("run-meta for %s records no PATH; the arm difference is then invisible on disk", arm)
		}
		paths[arm] = meta.Path
	}

	if !strings.Contains(paths[isolate.Sense], senseBin) {
		t.Errorf("the sense arm's recorded PATH %q does not reach the Sense binary", paths[isolate.Sense])
	}
	if strings.Contains(paths[isolate.Baseline], senseBin) {
		t.Errorf("the baseline arm's recorded PATH %q reaches the Sense binary", paths[isolate.Baseline])
	}
}

// nothingSet is a host that sets no allowlisted variable, so these tests do not
// depend on the environment of the machine running them.
func nothingSet(string) (string, bool) { return "", false }

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestTheSessionCannotSeeTheOperatorsOwnGuidanceFile(t *testing.T) {
	// The seventh channel, and the one an environment variable does not close:
	// the operator's ~/.claude/CLAUDE.md is loaded into every session, in both
	// arms. A personal instruction of the "answer in under six lines" kind
	// suppresses answer length in both, and answer length is what a richness
	// floor and a recall count are measured on.
	//
	// The session is asked rather than told: it looks in its own HOME and says
	// what it found. Setting CLAUDE_CODE_DISABLE_AUTO_MEMORY=1 was measured
	// against a real spawn and does not close this; only a disposable HOME does.
	hostHome := t.TempDir()
	guidance := filepath.Join(hostHome, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(guidance), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(guidance, []byte("answer in under six lines"), 0o644); err != nil {
		t.Fatal(err)
	}

	env, err := isolate.Prepare(isolate.Spec{
		Root:     runRoot(t),
		HostPath: os.Getenv("PATH"),
		Lookup:   nothingSet,
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	out := filepath.Join(t.TempDir(), "out")

	if _, err := run.Session(context.Background(), out, run.Spec{
		Dir:  env.Root,
		Name: "/bin/sh",
		Args: []string{"-c", `cat "$HOME/.claude/CLAUDE.md" 2>/dev/null || echo NOTHING`},
		Env:  env.Environ,
		Wall: 30 * time.Second,
	}); err != nil {
		t.Fatalf("Session: %v", err)
	}

	said := strings.TrimSpace(readFile(t, filepath.Join(out, "raw", "stdout")))
	if said != "NOTHING" {
		t.Errorf("the session read %q from its HOME; the operator's guidance reached it", said)
	}
	// And the guidance is still where the operator left it.
	if _, err := os.Stat(guidance); err != nil {
		t.Errorf("the operator's guidance was disturbed: %v", err)
	}
}
