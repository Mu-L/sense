package isolate

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// ShadowBin builds the one directory an arm's PATH points at: a symlink to
// every executable the host PATH offers, minus any binary named like Sense,
// plus the Sense binary itself for the sense arm.
//
// # Why a farm rather than a filtered PATH
//
// Removing the directory that holds the Sense binary is not enough, and the
// channel proof caught it: on a machine where Sense is installed the way a user
// installs it, the baseline arm still found `sense` on the host's own PATH,
// through a directory the bench never configured. Dropping that directory
// instead would take every other tool in it with it.
//
// So both arms get a farm, and they differ in exactly one link. That is also
// what makes the arms comparable: a baseline with a farm and a sense arm with
// the raw host PATH would differ in the shape of PATH as well as in Sense.
//
// First entry wins, so the host's own precedence survives.
func ShadowBin(dir, hostPath, senseBin string, arm Arm) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("prepare the arm's bin directory: %w", err)
	}

	deny := ""
	if senseBin != "" {
		deny = filepath.Base(senseBin)
	}
	linked := map[string]bool{}
	for _, from := range filepath.SplitList(hostPath) {
		if from == "" {
			continue
		}
		if err := linkExecutables(dir, from, deny, linked); err != nil {
			return "", err
		}
	}

	// The Sense binary goes in last and deliberately, so the sense arm reaches
	// the build under test rather than whatever the host happens to have
	// installed.
	if arm == Sense && senseBin != "" {
		target := filepath.Join(dir, deny)
		_ = os.Remove(target)
		if err := os.Symlink(senseBin, target); err != nil {
			return "", fmt.Errorf("link the sense binary: %w", err)
		}
	}
	return dir, nil
}

// linkExecutables links everything runnable in one host directory, skipping
// names already linked and the denied one.
func linkExecutables(into, from, deny string, linked map[string]bool) error {
	entries, err := os.ReadDir(from)
	if err != nil {
		// A PATH entry that is not there is normal on any real machine, and a
		// run must not fail because of one.
		return nil //nolint:nilerr // an unreadable PATH entry is not a run failure
	}
	for _, e := range entries {
		name := e.Name()
		if linked[name] || name == deny || !runnable(from, e) {
			continue
		}
		if err := os.Symlink(filepath.Join(from, name), filepath.Join(into, name)); err != nil {
			return fmt.Errorf("link %s: %w", name, err)
		}
		linked[name] = true
	}
	return nil
}

// runnable reports whether an entry can be executed. A directory cannot, and a
// file with no execute bit is data.
func runnable(dir string, e fs.DirEntry) bool {
	info, err := os.Stat(filepath.Join(dir, e.Name()))
	if err != nil {
		// A dangling symlink on the host PATH. Skipping it is what the shell
		// does too.
		return false
	}
	return !info.IsDir() && info.Mode()&0o111 != 0
}
