package isolate

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// noHost is a host that sets nothing, so a test states its own inheritance
// rather than inheriting whatever the machine running it happens to carry.
func noHost(string) (string, bool) { return "", false }

// hostWith is a host that sets exactly the given variables.
func hostWith(kv map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := kv[name]
		return v, ok
	}
}

// envMap turns a KEY=VALUE slice into a map, honouring the last-wins rule exec
// applies to duplicates.
func envMap(t *testing.T, env []string) map[string]string {
	t.Helper()
	m := make(map[string]string, len(env))
	for _, kv := range env {
		name, value, ok := strings.Cut(kv, "=")
		if !ok {
			t.Fatalf("environment entry %q is not KEY=VALUE", kv)
		}
		m[name] = value
	}
	return m
}

func TestSessionSeesTheDisposableHomeAndXDGDirectories(t *testing.T) {
	l := LayoutFor("/runs/r1")

	got := envMap(t, Environ(l, "/usr/bin", noHost, nil, ""))

	want := map[string]string{
		"HOME":            "/runs/r1/home",
		"XDG_CONFIG_HOME": "/runs/r1/home/config",
		"XDG_CACHE_HOME":  "/runs/r1/home/cache",
		"XDG_DATA_HOME":   "/runs/r1/home/data",
		"XDG_STATE_HOME":  "/runs/r1/home/state",
	}
	for name, value := range want {
		if got[name] != value {
			t.Errorf("%s = %q, want %q", name, got[name], value)
		}
	}
}

func TestHostVariablesOutsideTheAllowlistDoNotReachTheSession(t *testing.T) {
	// A denylist would let every one of these through, and the memory directory
	// is the one that has already cost a measurement.
	host := hostWith(map[string]string{
		"TERM": "xterm",
		"CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": "1",
		"AWS_SECRET_ACCESS_KEY":                    "leaked",
		"GITHUB_TOKEN":                             "leaked",
		"EDITOR":                                   "vim",
	})

	got := envMap(t, Environ(LayoutFor("/runs/r1"), "/usr/bin", host, nil, ""))

	if got["TERM"] != "xterm" {
		t.Errorf("TERM = %q, want the allowlisted host value", got["TERM"])
	}
	for _, name := range []string{"AWS_SECRET_ACCESS_KEY", "GITHUB_TOKEN", "EDITOR", "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s reached the session; it is not on the allowlist", name)
		}
	}
}

func TestAnAllowlistedVariableTheHostDoesNotSetIsNotInvented(t *testing.T) {
	got := envMap(t, Environ(LayoutFor("/runs/r1"), "/usr/bin", noHost, nil, ""))

	// An empty ANTHROPIC_API_KEY is not the same as an absent one: a tool that
	// checks for presence would take the empty string as api-key auth and fail
	// to reach a model at all.
	if _, ok := got["ANTHROPIC_API_KEY"]; ok {
		t.Error("ANTHROPIC_API_KEY was set although the host does not set it")
	}
}

func TestEveryAllowlistEntryRecordsWhyItIsThere(t *testing.T) {
	for _, e := range allowed {
		if e.Name == "" {
			t.Error("an allowlist entry has no name")
		}
		if e.Why == "" {
			t.Errorf("allowlist entry %s records no reason", e.Name)
		}
	}
}

func TestPathIsNotInheritedFromTheHostAllowlist(t *testing.T) {
	// PATH is an arm-specific value, so it must not be reachable as an ordinary
	// allowlist entry: an entry would give both arms the host's PATH and the
	// baseline arm a CLI fallback with it.
	if slices.ContainsFunc(allowed, func(e Entry) bool { return e.Name == "PATH" }) {
		t.Fatal("PATH is on the environment allowlist; it must be built per arm by ShadowBin")
	}

	host := hostWith(map[string]string{"PATH": "/host/bin"})
	got := envMap(t, Environ(LayoutFor("/runs/r1"), "/arm/bin", host, nil, ""))
	if got["PATH"] != "/arm/bin" {
		t.Errorf("PATH = %q, want the arm's value /arm/bin", got["PATH"])
	}
}

func TestTheAgentToolsOwnEnvironmentWinsOverADefault(t *testing.T) {
	agentEnv := []string{"IS_SANDBOX=1", "TERM=dumb"}
	host := hostWith(map[string]string{"TERM": "xterm"})

	got := envMap(t, Environ(LayoutFor("/runs/r1"), "/usr/bin", host, agentEnv, ""))

	if got["IS_SANDBOX"] != "1" {
		t.Errorf("IS_SANDBOX = %q, want the agent tool's 1", got["IS_SANDBOX"])
	}
	if got["TERM"] != "dumb" {
		t.Errorf("TERM = %q, want the agent tool's dumb to win over the host's xterm", got["TERM"])
	}
}

func TestTheCleanupProofFailsWhileAnythingRemains(t *testing.T) {
	// The proof exists because os.RemoveAll returning nil says nothing was
	// reported, not that nothing remains.
	dir := t.TempDir()

	if err := gone(dir); err == nil {
		t.Fatal("the cleanup proof passed on a directory that still exists")
	}
	if err := gone(filepath.Join(dir, "never-created")); err != nil {
		t.Errorf("the cleanup proof failed on an absent path: %v", err)
	}
}

func TestTheCleanupProofFailsWhenItCannotTell(t *testing.T) {
	// A path that cannot be statted at all. "I could not look" is not "it is
	// gone", and reporting success here would let a contaminated environment
	// through on exactly the machines where something is already wrong.
	long := strings.Repeat("x", 1<<12)

	if err := gone(filepath.Join(long, long, "root")); err == nil {
		t.Fatal("the cleanup proof passed on a path it could not check")
	}
}

func TestTheLayoutNamesTheWholeRunTree(t *testing.T) {
	l := LayoutFor("/runs/r1")

	for name, got := range map[string]string{
		"Repo":      l.Repo,
		"Home":      l.Home,
		"Logs":      l.Logs,
		"Artifacts": l.Artifacts,
	} {
		if !strings.HasPrefix(got, "/runs/r1/") {
			t.Errorf("%s = %q, which is outside the run root", name, got)
		}
	}
	if l.Root != "/runs/r1" {
		t.Errorf("Root = %q, want /runs/r1", l.Root)
	}
}

// The correction this pitch made, stated so it cannot regress silently.
//
// Credentials() used to decide what a credential is by looking for the word
// "authentication" inside each entry's human-readable reason. Both flagged
// entries happen to carry that word today, so swapping the flag back for the
// string match passes every other test in the tree — which is exactly the shape
// of wrong that reads as working. The fixture breaks the coincidence: one entry
// says "authentication" and is not a credential, one is a credential and never
// says it.
func TestWhatCountsAsACredentialIsDeclaredRatherThanReadOutOfAComment(t *testing.T) {
	was := allowed
	t.Cleanup(func() { allowed = was })
	allowed = []Entry{
		{Name: "TALKS_ABOUT_AUTH", Why: "not a credential, but its reason mentions authentication in passing"},
		{Name: "THE_REAL_ONE", Why: "reaches a model, and says so without using the word", Credential: true},
	}

	got := Credentials()

	if !slices.Equal(got, []string{"THE_REAL_ONE"}) {
		t.Errorf("Credentials() = %v, want only the entry declared as one", got)
	}
}

// The variable that leaves the allowlist, named so re-adding it is a test
// failure rather than a comment nobody reads.
//
// The hazard is measured: when it is set the agent tool writes a plaintext
// credential and its fallback combiner then deletes the operator's keychain
// entry on exit. A disposable HOME means the branch cannot fire today, which is
// a reason to record the mechanism and not a reason to keep the variable.
func TestTheKeychainDestroyingVariableIsNotOnTheAllowlist(t *testing.T) {
	const destroys = "CLAUDE_CODE_OAUTH_TOKEN"

	if slices.ContainsFunc(allowed, func(e Entry) bool { return e.Name == destroys }) {
		t.Fatalf("%s is on the environment allowlist; a run on a host that sets it can delete "+
			"the operator's own login, and nothing in a bench result would show it", destroys)
	}

	// And it does not reach a session even when the host sets it.
	host := hostWith(map[string]string{destroys: "a-token"})
	if _, ok := envMap(t, Environ(LayoutFor("/runs/r1"), "/usr/bin", host, nil, ""))[destroys]; ok {
		t.Errorf("%s reached the session", destroys)
	}
}
