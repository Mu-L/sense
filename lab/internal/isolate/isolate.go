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
	// SenseBinDir holds the Sense binary. The Sense arm gets it at the front of
	// PATH and the baseline arm gets it removed.
	SenseBinDir string
	// HostPath is the PATH both arms derive from, usually the host's.
	HostPath string
	// AgentEnv is what the agent tool declares in agent.json, as KEY=VALUE.
	AgentEnv []string
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
	for _, dir := range l.dirs() {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Env{}, fmt.Errorf("prepare run environment: %w", err)
		}
	}

	lookup := s.Lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	return Env{
		Layout:  l,
		Arm:     s.Arm,
		Environ: Environ(l, PathFor(s.Arm, s.HostPath, s.SenseBinDir), lookup, s.AgentEnv),
	}, nil
}

// Cleanup removes the whole environment and proves it is gone.
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
