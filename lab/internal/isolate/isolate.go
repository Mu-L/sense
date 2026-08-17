// Package isolate builds the disposable environment one run happens inside, and
// destroys it afterwards.
//
// It exists because of a specific measurement failure: the walking skeleton ran
// an agent against the host's HOME, so anything the host carried between runs —
// a persisted memory directory keyed off the repository path, a warm cache, an
// agent config — reached both arms of every cell, and nothing in the repository
// or the prompt would show it.
//
// This is isolated-home, written as a concrete type. The Executor interface is
// cycle 08's, extracted once the container exists and both shapes are visible;
// an interface with one implementation is indirection rather than abstraction.
package isolate

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Spec is one environment to prepare.
type Spec struct {
	// Root is the run directory to create. It must not already exist.
	Root string
	// Arm decides the PATH, and nothing else here.
	Arm Arm
	// SenseBin is the Sense binary under test. The sense arm reaches it and the
	// baseline arm reaches no binary of that name at all.
	SenseBin string
	// HostPath is the PATH both arms derive from, usually the host's.
	HostPath string
	// AgentEnv is what the agent tool declares in agent.json, as KEY=VALUE.
	AgentEnv []string
	// Credential is what the run is given to reach a model, read from the host
	// once by the attended parent.
	//
	// An empty one is legal and means a key-based host: the key is an allowlisted
	// environment variable and no file is written. Whether the run has EITHER is
	// not decided here — it is decided in the attended parent before anything
	// spawns, because a session that reaches no model exits in about a second
	// with "Not logged in" and reads in a score exactly like a model that had
	// nothing to say. What Prepare refuses is a half-filled credential, which is
	// a mistake rather than a choice.
	Credential Credential
	// Route is how that credential reaches this agent tool, from the catalog.
	Route Route
	// Lookup reads the host environment. nil means os.LookupEnv.
	Lookup func(string) (string, bool)
}

// Env is a prepared environment: where it is, and what a session run inside it
// sees.
type Env struct {
	Layout
	// Arm is recorded so a run can say which side of the cell it was.
	Arm Arm
	// Environ is the session's complete environment, as KEY=VALUE. It is
	// complete rather than additive: a measured run inherits nothing it was not
	// given.
	Environ []string
}

// Prepare creates the run directory tree and computes the environment a session
// inside it runs with. It spawns nothing.
//
// It refuses a root that already exists. A run's environment is created fresh
// or not at all, because reusing one is the contamination this package exists
// to prevent, arriving through the front door.
func Prepare(s Spec) (Env, error) {
	if s.Root == "" {
		return Env{}, errors.New("prepare: no run root given")
	}
	// Abs only fails when the working directory cannot be resolved, and it
	// returns the path unchanged when it does; the Stat below is the check that
	// matters either way. The absolute form is what gets recorded, so a run
	// directory is readable from anywhere later.
	root, _ := filepath.Abs(s.Root)
	if _, err := os.Stat(root); err == nil {
		return Env{}, fmt.Errorf("%s already exists; a run's environment is created fresh, never reused", root)
	}

	l := LayoutFor(root)
	for _, d := range l.dirs() {
		if err := os.MkdirAll(d.path, d.mode); err != nil {
			return Env{}, fmt.Errorf("prepare run environment: %w", err)
		}
	}

	// The arm's PATH is one directory of symlinks rather than the host's own
	// list, because "the baseline arm cannot reach Sense" has to hold against a
	// machine where Sense is installed the way a user installs it.
	bin, err := ShadowBin(filepath.Join(root, "bin"), s.HostPath, s.SenseBin, s.Arm)
	if err != nil {
		return Env{}, err
	}

	// The credential goes in before anything can be spawned against this
	// environment, and a bad one fails the preparation rather than the session:
	// the alternative is discovering it a wall at a time, twice per cell. An
	// empty one is a key-based host, which needs no file; a half-filled one is a
	// mistake and is refused.
	if !s.Credential.Empty() {
		if err := s.Route.Write(l.Config, s.Credential); err != nil {
			return Env{}, err
		}
	}

	lookup := s.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	return Env{
		Layout:  l,
		Arm:     s.Arm,
		Environ: Environ(l, bin, lookup, s.AgentEnv, s.Route.ConfigDirVar),
	}, nil
}

// Cleanup removes the whole environment and proves it is gone.
//
// It is the right shape for a throwaway directory — the channel probe, a refused
// cell — and the wrong one for a measured arm, which is what Release is for.
// HoldsEvidence decides which, and it decides from the directory rather than
// from a flag a caller passes: a boolean at the call site is a boolean somebody
// eventually gets backwards, silently, on the side that deletes the evidence.
//
// The proof is the point. RemoveAll returning nil says nothing was reported,
// not that nothing remains, and a run that leaves state behind contaminates the
// next one — which is the failure that would then be attributed to the model.
func Cleanup(e Env) error {
	if e.Root == "" {
		return errors.New("cleanup: no run root")
	}
	if err := os.RemoveAll(e.Root); err != nil {
		return fmt.Errorf("remove run environment %s: %w", e.Root, err)
	}
	return gone(e.Root)
}

// gone is the proof half of Cleanup: it reports an error unless the path is
// really absent. It is separate from the removal so the check can be stated on
// its own, which is the only thing that distinguishes it from trusting
// RemoveAll's return value.
func gone(path string) error {
	_, err := os.Stat(path)
	switch {
	case err == nil:
		return fmt.Errorf("run environment %s still exists after cleanup", path)
	case errors.Is(err, fs.ErrNotExist):
		return nil
	default:
		return fmt.Errorf("verify cleanup of %s: %w", path, err)
	}
}

// evidence are the places a later reader looks: the capture, the session's raw
// transcript, and the run's own record.
//
// A run directory is kept for months so a result can be re-read, and what makes
// it worth keeping is that one of these holds something.
var evidence = []string{"artifacts", "logs", "session"}

// HoldsEvidence reports whether a run directory holds anything a later reader
// needs, which is what decides how it is cleaned up.
//
// The property is "does this directory hold evidence", NOT "did this run
// finish", and the distinction is load-bearing. The obvious rule gets it
// backwards: keying off the run record deletes the directory of a run that
// crashed before writing one, and that directory holds the transcript of the
// crash — the most valuable thing on the disk at that moment.
func HoldsEvidence(root string) bool {
	for _, dir := range evidence {
		if holdsAFile(filepath.Join(root, dir)) {
			return true
		}
	}
	return false
}

// holdsAFile reports whether anything at all was written under a directory.
//
// A directory that cannot be read counts as holding something, and that is the
// load-bearing half. "I could not look" is not "there is nothing there", and the
// two answers have very different costs here: the false negative deletes a run
// that was paid for and can never be recreated, and the false positive keeps a
// directory somebody removes by hand. An earlier version returned false on an
// unreadable directory while its comment claimed the opposite, which is the
// rabbit hole this pitch names — do not delete a directory because it looks
// unfinished.
func holdsAFile(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// Never created. That is a real absence, not a failure to look.
			return nil
		case err != nil:
			found = true
			return fs.SkipAll
		case !d.IsDir():
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}

// Release gives back what a finished run does not need and keeps what a later
// reader does: the config directory and the disposable HOME go, the evidence
// stays.
//
// It is the second cleanup shape, and it exists because the first one is wrong
// for a measured arm. Cleanup removes the run root, which holds the transcript,
// the capture and the record — the evidence the scorer, the miner and status all
// read, for a run that is paid for and can never be recreated.
//
// The config directory goes because it holds the credential: a run directory
// outlives its purpose by months, and a plaintext bearer token in one outlives
// its purpose with it.
//
// The disposable HOME goes for the same reason and one more. It is not evidence
// — the contamination checks read it, and they run before this — and it is
// whatever the agent tool decided to write about itself, which is exactly the
// thing nobody can enumerate in advance. A tool that copies its credential into
// its own HOME state would put a token in a kept directory, and no assertion
// about what our stand-in writes would catch it. Dropping the directory makes
// the guarantee independent of the tool's behaviour rather than a claim about it.
//
// The arm's bin farm stays: it is symlinks and it carries nothing. Note what
// that does NOT buy — run-meta.json records the arm's HOME as well as its PATH,
// and the HOME path no longer resolves after a release. Both are kept as a
// record of what the arm ran with, not as directories a later reader can walk.
//
// Both removals are proven rather than assumed, the way the rest of this package
// proves its own: RemoveAll returning nil says nothing was reported.
func Release(e Env) error {
	if e.Root == "" {
		return errors.New("release: no run root")
	}
	var errs []error
	// A slice rather than a map: removal order and the joined error text are then
	// the same on every run, and nondeterministic output ordering has cost this
	// repository a diff before.
	for _, part := range []struct{ what, at string }{
		{"credential", e.Config},
		{"disposable HOME", e.Home},
	} {
		if err := os.RemoveAll(part.at); err != nil {
			errs = append(errs, fmt.Errorf("remove the run's %s at %s: %w", part.what, part.at, err))
			continue
		}
		errs = append(errs, gone(part.at))
	}
	return errors.Join(errs...)
}
