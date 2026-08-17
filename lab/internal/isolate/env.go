package isolate

import (
	"path/filepath"
	"strings"
)

// Arm is which side of a cell a run belongs to.
//
// It lives here, in the environment package, because the arms differ in exactly
// one environmental fact: whether the Sense binary is reachable on PATH. Every
// other difference between the two is repository state, and that is 03-02.
type Arm string

const (
	// Sense is the arm that may reach Sense.
	Sense Arm = "sense"
	// Baseline is the arm that may not, by any channel.
	Baseline Arm = "baseline"
)

// Entry is one environment variable a session is allowed to inherit from the
// host, together with the reason it is allowed. The reason is not decoration:
// an allowlist whose entries nobody can justify decays into a denylist one
// convenience at a time.
type Entry struct {
	Name string
	Why  string
}

// allowed is the environment allowlist, and it is an allowlist on purpose. A
// denylist means every variable the host acquires later is a channel the arms
// did not earn and nobody notices.
//
// PATH is deliberately absent. It is not a shared default but one entry with
// two arm-specific values, and treating it as common is how the baseline arm
// silently acquires a CLI fallback. ShadowBin builds it instead.
//
// HOME and the XDG variables are absent for a different reason: they are facts
// about the disposable directory rather than about the host, so inheriting them
// would defeat the whole package. Environ sets them.
var allowed = []Entry{
	{"TERM", "an agent CLI that cannot resolve a terminal writes control codes into the capture, which then reach the scorer as text"},
	{"LANG", "collation and case folding differ without it, and a scenario's gold is matched by string"},
	{"LC_ALL", "same as LANG, and it wins over LANG where both are set"},
	{"ANTHROPIC_API_KEY", "key-based authentication; without it such a run reaches no model and produces an empty arm"},
	{"ANTHROPIC_AUTH_TOKEN", "key-based authentication against a gateway that takes a bearer token rather than a key"},
	{"CLAUDE_CODE_OAUTH_TOKEN", "seat-based authentication; the disposable HOME carries no credential of its own"},
	{"SSL_CERT_FILE", "a host with a custom CA store cannot reach the provider without it, and the failure reads as an empty arm"},
	{"SSL_CERT_DIR", "same as SSL_CERT_FILE, for a store kept as a directory"},
	{"HTTP_PROXY", "a host behind a proxy reaches nothing without it"},
	{"HTTPS_PROXY", "same as HTTP_PROXY, and it is the one the provider is actually reached through"},
	{"NO_PROXY", "a proxy without its exception list sends local traffic through the proxy"},
	{"http_proxy", "the lowercase spelling; tooling is split on which it reads, and honouring only one is a proxy that works for half a session"},
	{"https_proxy", "the lowercase spelling, as above"},
	{"no_proxy", "the lowercase spelling, as above"},
}

// Layout is the directory tree of one run's environment.
type Layout struct {
	// Root is the run directory. Cleanup removes this and everything under it.
	Root string
	// Repo is where the worktree goes. Nothing creates it here: git refuses to
	// add a worktree at a path that already exists, so 03-02 creates it.
	Repo string
	// Home is the disposable HOME, holding config, cache, data and state.
	Home string
	// Logs holds the session's capture.
	Logs string
	// Artifacts holds the transcript, the tool io capture and the run metadata.
	Artifacts string
}

// LayoutFor computes the tree under a run root. It creates nothing.
func LayoutFor(root string) Layout {
	return Layout{
		Root:      root,
		Repo:      filepath.Join(root, "repo"),
		Home:      filepath.Join(root, "home"),
		Logs:      filepath.Join(root, "logs"),
		Artifacts: filepath.Join(root, "artifacts"),
	}
}

// homeSubdirs are the XDG directories inside the disposable HOME. The names are
// the XDG defaults minus the dot, so a tool that ignores the variables and
// hard-codes `~/.config` still lands inside the run directory.
var homeSubdirs = []string{"config", "cache", "data", "state"}

// dirs is everything Prepare creates, parents first.
func (l Layout) dirs() []string {
	d := []string{l.Root, l.Home, l.Logs, l.Artifacts}
	for _, sub := range homeSubdirs {
		d = append(d, filepath.Join(l.Home, sub))
	}
	return d
}

// Environ builds the complete environment a session runs with: the disposable
// directories, the arm's PATH, the allowlist as far as the host actually sets
// it, and whatever the agent tool declares.
//
// lookup reads the host, so the decision is a pure function of its arguments and
// a table test can state the whole of it. Production passes os.LookupEnv.
func Environ(l Layout, path string, lookup func(string) (string, bool), agentEnv []string) []string {
	env := []string{
		"HOME=" + l.Home,
		"PATH=" + path,
	}
	for _, sub := range homeSubdirs {
		env = append(env, "XDG_"+strings.ToUpper(sub)+"_HOME="+filepath.Join(l.Home, sub))
	}
	for _, a := range allowed {
		if v, ok := lookup(a.Name); ok {
			env = append(env, a.Name+"="+v)
		}
	}
	// The agent tool's overrides come last so a tool can correct a default it
	// cannot live with. They are declared in agent.json and reviewed there.
	return append(env, agentEnv...)
}
