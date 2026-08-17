package isolate

import (
	"os"
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
	// Credential says this entry carries authentication.
	//
	// It is a field rather than a reading of Why, and that is a correction. This
	// used to be decided by looking for the word "authentication" inside the
	// human-readable reason, which was tolerable only while a credential WAS an
	// environment variable. The primary credential is now a file, so a function
	// that string-matches comments to answer an authentication question is both
	// wrong and the kind of wrong that reads as working.
	Credential bool
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
//
// CLAUDE_CODE_OAUTH_TOKEN was here and is deliberately gone. When it is set the
// agent tool writes a plaintext credential, and its fallback combiner then
// deletes the operator's keychain entry on exit — so a bench run on a machine
// where that variable is set can destroy the operator's own login, and nothing
// in a bench result would show it (anthropics/claude-code#37512, closed as not
// planned).
//
// The mechanism is recorded rather than the headline, because the mechanism is
// what a later reader has to re-check. The delete fires only when the keychain
// read returned non-null, the keychain write then failed, and the plaintext
// write succeeded; a disposable HOME makes that first read null, so the branch
// cannot fire today. The variable still goes: a bench that only fails to destroy
// the operator's login by accident of another design decision is one refactor
// away from destroying it. Seats are reached through the config directory now.
var allowed = []Entry{
	{Name: "TERM", Why: "an agent CLI that cannot resolve a terminal writes control codes into the capture, which then reach the scorer as text"},
	{Name: "LANG", Why: "collation and case folding differ without it, and a scenario's gold is matched by string"},
	{Name: "LC_ALL", Why: "same as LANG, and it wins over LANG where both are set"},
	{Name: "ANTHROPIC_API_KEY", Why: "key-based authentication; without it such a run reaches no model and produces an empty arm", Credential: true},
	{Name: "ANTHROPIC_AUTH_TOKEN", Why: "key-based authentication against a gateway that takes a bearer token rather than a key", Credential: true},
	{Name: "SSL_CERT_FILE", Why: "a host with a custom CA store cannot reach the provider without it, and the failure reads as an empty arm"},
	{Name: "SSL_CERT_DIR", Why: "same as SSL_CERT_FILE, for a store kept as a directory"},
	{Name: "HTTP_PROXY", Why: "a host behind a proxy reaches nothing without it"},
	{Name: "HTTPS_PROXY", Why: "same as HTTP_PROXY, and it is the one the provider is actually reached through"},
	{Name: "NO_PROXY", Why: "a proxy without its exception list sends local traffic through the proxy"},
	{Name: "http_proxy", Why: "the lowercase spelling; tooling is split on which it reads, and honouring only one is a proxy that works for half a session"},
	{Name: "https_proxy", Why: "the lowercase spelling, as above"},
	{Name: "no_proxy", Why: "the lowercase spelling, as above"},
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
	// Config is the run's agent config directory, and it is the door
	// authentication comes through. CLAUDE_CONFIG_DIR points at it directly,
	// which is what lets a credential be provisioned into a run without the run
	// being able to reach a host keychain.
	//
	// It sits beside the disposable HOME rather than inside it, and deliberately:
	// the contamination proof reads persisted memory as "a tool state directory
	// exists under HOME", and a provisioned credential inside one would make
	// every arm read as contaminated. Outside, the two questions stay separate —
	// which is also why the memory check is told where this is.
	Config string
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
		Config:    filepath.Join(root, "config"),
		Logs:      filepath.Join(root, "logs"),
		Artifacts: filepath.Join(root, "artifacts"),
	}
}

// homeSubdirs are the XDG directories inside the disposable HOME. The names are
// the XDG defaults minus the dot, so a tool that ignores the variables and
// hard-codes `~/.config` still lands inside the run directory.
var homeSubdirs = []string{"config", "cache", "data", "state"}

// dir is one directory to create, and the mode it needs.
type dir struct {
	path string
	mode os.FileMode
}

// dirs is everything Prepare creates, parents first.
//
// The config directory is 0700 and the rest are 0755, because that one holds a
// plaintext bearer token for the length of the run. The file is 0600, but a
// world-listable directory around it still tells everyone on the machine that
// the credential is there and what it is called — and the mode has to be set
// HERE, at creation: MkdirAll does not chmod a directory that already exists, so
// provisioning into one made at 0755 would leave it at 0755.
func (l Layout) dirs() []dir {
	d := []dir{
		{l.Root, 0o755},
		{l.Home, 0o755},
		{l.Config, 0o700},
		{l.Logs, 0o755},
		{l.Artifacts, 0o755},
	}
	for _, sub := range homeSubdirs {
		d = append(d, dir{filepath.Join(l.Home, sub), 0o755})
	}
	return d
}

// Credentials are the allowlisted variables that carry authentication.
//
// They are the alternative door, not the main one: a seat reaches a run through
// the provisioned config directory, and these are how a key-based host reaches
// one instead. A session that has neither cannot reach a model, and the arm it
// produces is empty rather than failed.
//
// It is derived from the allowlist rather than written out again — a credential
// added there and forgotten here would be a variable the session carries and
// nothing counts as authentication — but derived from the flag on each entry
// rather than from the wording of its reason.
func Credentials() []string {
	var out []string
	for _, e := range allowed {
		if e.Credential {
			out = append(out, e.Name)
		}
	}
	return out
}

// Environ builds the complete environment a session runs with: the disposable
// directories, the arm's PATH, the allowlist as far as the host actually sets
// it, and whatever the agent tool declares.
//
// lookup reads the host, so the decision is a pure function of its arguments and
// a table test can state the whole of it. Production passes os.LookupEnv.
// configDirVar is the agent tool's own name for the variable that points it at a
// config directory, and it comes from the catalog: a name compiled in here would
// be right for one tool and silently wrong for every other, which is the same
// rule the contamination checks follow. A tool that declares none is given none,
// and it authenticates by key or not at all.
func Environ(l Layout, path string, lookup func(string) (string, bool), agentEnv []string, configDirVar string) []string {
	env := []string{
		"HOME=" + l.Home,
		"PATH=" + path,
	}
	if configDirVar != "" {
		env = append(env, configDirVar+"="+l.Config)
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
