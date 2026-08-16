package isolate

import (
	"context"
	"fmt"
	"os/exec"
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

// RemoveWorktree removes the worktree and proves it is gone.
//
// The proof is the point, as it is for the rest of cleanup: git reporting
// success says nothing was reported, and a worktree left behind is state the
// next run inherits. --force because a session is expected to have dirtied the
// tree, and a run that modified the repository must still clean up.
func RemoveWorktree(ctx context.Context, parent, dest string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", parent, "worktree", "remove", "--force", dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove worktree %s: %w: %s", dest, err, out)
	}
	return gone(dest)
}
