package isolate

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// AddWorktree checks the parent repository out at a commit, at dest.
//
// A worktree rather than a clone: it is cheap, it is pinned by construction,
// and it cannot drift from the commit recorded in run-meta.json. Detached,
// because a branch name would be a second thing that could move.
//
// Worktrees share the parent's object store. That is fine at one run at a time,
// which the law requires anyway, but it means parallel runs contend and a
// corrupted parent takes every worktree with it. Whoever adds concurrency meets
// that constraint deliberately.
func AddWorktree(ctx context.Context, parent, commit, dest string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", parent, "worktree", "add", "--detach", dest, commit)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("add worktree at %.12s: %w: %s", commit, err, out)
	}
	return nil
}

// RemoveWorktree removes the worktree and proves both it and its registration
// are gone.
//
// The proof is the point, as it is for the rest of cleanup: git reporting
// success says nothing was reported, and a worktree left behind is state the
// next run inherits. --force because a session is expected to have dirtied the
// tree, and a run that modified the repository must still clean up.
//
// The registration is checked separately from the directory because the two come
// apart: a directory removed by something other than git leaves an entry behind,
// and git then refuses to add a worktree at that path again. Three such entries
// accumulated in one afternoon and had to be pruned by hand before a retry would
// run, so `prune` runs first and the list is read back afterwards.
func RemoveWorktree(ctx context.Context, parent, dest string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", parent, "worktree", "remove", "--force", dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// git refuses when the directory is already gone, and that is exactly the
		// case that leaves a stale entry behind. Prune it — but only when the
		// parent still names this path. Otherwise there was nothing here to
		// remove, and reporting that is what catches a caller cleaning up
		// somewhere it never ran.
		if deregistered(ctx, parent, dest) == nil {
			return fmt.Errorf("remove worktree %s: %w: %s", dest, err, out)
		}
		prune(ctx, parent)
	}
	// Both proofs run and both are reported. The directory and the registration
	// come apart, so one of them passing says nothing about the other.
	return errors.Join(gone(dest), deregistered(ctx, parent, dest))
}

// prune clears registrations whose worktrees are no longer on disk.
//
// Its result is deliberately not reported. Whether git was happy is not the
// question; whether the entry is still listed is, and RemoveWorktree asks that
// directly afterwards. A prune that failed and mattered shows up there.
func prune(ctx context.Context, parent string) {
	_ = exec.CommandContext(ctx, "git", "-C", parent, "worktree", "prune").Run()
}

// deregistered is the proof half: the parent repository no longer names dest.
func deregistered(ctx context.Context, parent, dest string) error {
	out, err := exec.CommandContext(ctx, "git", "-C", parent, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return fmt.Errorf("list the worktrees of %s: %w", parent, err)
	}
	abs, _ := filepath.Abs(dest)
	for _, line := range strings.Split(string(out), "\n") {
		at, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		if registered, _ := filepath.Abs(strings.TrimSpace(at)); registered == abs {
			return fmt.Errorf("%s is still registered as a worktree of %s; the next attempt at that path "+
				"would be refused", dest, parent)
		}
	}
	return nil
}
