package channels

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
)

// HostWatch is the set of host paths a run must leave untouched: the host agent
// config, the operator's own routing guidance, and the memory directory for the
// repository under study.
//
// It is exactly the paths the channels name, and no more. A general sweep of
// HOME is flaky the moment any other process on the machine writes something,
// and a flaky check is a check somebody disables.
func HostWatch(home string, configDirs []string, repo string) []string {
	var out []string
	for _, dir := range configDirs {
		out = append(out,
			filepath.Join(home, dir+".json"),
			filepath.Join(home, dir, "CLAUDE.md"),
			filepath.Join(home, dir, "settings.json"),
			MemoryDir(home, dir, repo))
	}
	return out
}

// entryDigest is what identifies one entry: its contents for a regular file,
// and what it IS for anything else.
//
// A symlink is recorded by its target rather than followed. Following one fails
// outright on a broken link — mastodon ships `public/500.html` pointing at an
// asset that only exists after a build, and it stopped subject preparation
// before either arm spawned — and on a link inside the tree it would hash the
// same bytes twice. The target is kept rather than skipped, because a run that
// rewires a link has changed the tree and a skipped entry would hide it.
//
// Anything else irregular is recorded by its mode and NOT opened. Reading a
// fifo blocks until somebody writes to it, which would hang the walk with no
// output and no wall to stop it: this walk runs before the session, so nothing
// is counting.
func entryDigest(path string, d fs.DirEntry) (string, error) {
	switch mode := d.Type(); {
	case mode&fs.ModeSymlink != 0:
		target, err := os.Readlink(path)
		if err != nil {
			return "", err
		}
		return "symlink:" + target, nil
	case !mode.IsRegular():
		return "irregular:" + mode.String(), nil
	}
	return fileDigest(path)
}

// absent is the digest of a path that is not there. It is a value rather than a
// missing key, so a path that appears during a run is a change rather than a
// silently ignored one.
const absent = "absent"

// Snapshot is the state of a set of paths at a moment, as path to digest.
type Snapshot map[string]string

// Take records the given paths. A path that does not exist is recorded as
// absent rather than skipped: a run that CREATES the operator's memory
// directory has contaminated the host just as surely as one that edits it.
func Take(paths []string) (Snapshot, error) {
	s := make(Snapshot, len(paths))
	for _, path := range paths {
		d, err := digest(path)
		if err != nil {
			return nil, fmt.Errorf("snapshot %s: %w", path, err)
		}
		s[path] = d
	}
	return s, nil
}

// Changed names every watched path whose state differs from the snapshot.
//
// A path that changed is a failed run and a bug. It is never something to tidy
// up and continue past: the state that leaked out is also the state the next
// run would have inherited.
func (s Snapshot) Changed() ([]string, error) {
	var changed []string
	for path, was := range s {
		now, err := digest(path)
		if err != nil {
			return nil, fmt.Errorf("re-read %s: %w", path, err)
		}
		if now == was {
			continue
		}
		switch {
		case was == absent:
			changed = append(changed, fmt.Sprintf("%s was created by the run", path))
		case now == absent:
			changed = append(changed, fmt.Sprintf("%s was removed by the run", path))
		default:
			changed = append(changed, fmt.Sprintf("%s was modified by the run", path))
		}
	}
	sort.Strings(changed)
	return changed, nil
}

// digest is a content hash for a file, a content hash over the whole tree for a
// directory, and the absent marker for a path that is not there.
func digest(path string) (string, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return absent, nil
	case err != nil:
		return "", err
	case info.IsDir():
		return treeDigest(path)
	default:
		return fileDigest(path)
	}
}

func fileDigest(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

// treeDigest covers additions and removals as well as edits, because a run that
// appended one memory file has left state the next run would inherit.
func treeDigest(root string) (string, error) {
	tree, err := Tree(root)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, rel := range Sorted(tree) {
		_, _ = io.WriteString(h, rel+"\x00"+tree[rel]+"\n")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Tree hashes every file below root, keyed by its path relative to root.
// Directories with any of the given names are not descended into.
//
// It is the one walker in the lab: the channel derivation, the host
// contamination digest and the record of what setup wrote are all "what files
// are here and what is in them", and three spellings of that is three places
// for a symlink or a permission to be handled differently.
func Tree(root string, skip ...string) (map[string]string, error) {
	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && slices.Contains(skip, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		digest, err := entryDigest(path, d)
		if err != nil {
			return err
		}
		tree[rel] = digest
		return nil
	})
	if err != nil {
		return nil, err
	}
	return tree, nil
}

// Sorted lists a tree's paths in a stable order, so anything derived from one
// reads the same between runs.
func Sorted(tree map[string]string) []string {
	paths := make([]string, 0, len(tree))
	for path := range tree {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
