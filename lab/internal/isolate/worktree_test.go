package isolate_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/isolate"
)

// parentRepo is a two-commit repository, so a test can pin an older commit and
// tell a pinned checkout from a checkout of the tip.
func parentRepo(t *testing.T) (dir, first, second string) {
	t.Helper()
	dir = t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=bench", "GIT_AUTHOR_EMAIL=bench@example.com",
			"GIT_COMMITTER_NAME=bench", "GIT_COMMITTER_EMAIL=bench@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-q", "-m", "first")
	first = git("rev-parse", "HEAD")

	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package app\n\nfunc New() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("commit", "-q", "-am", "second")
	second = git("rev-parse", "HEAD")
	return dir, first, second
}

func TestAWorktreeIsCheckedOutAtThePinnedCommitRatherThanTheTip(t *testing.T) {
	parent, pinned, tip := parentRepo(t)
	dest := filepath.Join(t.TempDir(), "repo")

	if err := isolate.AddWorktree(context.Background(), parent, pinned, dest); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	at := strings.TrimSpace(gitOut(t, dest, "rev-parse", "HEAD"))
	if at != pinned {
		t.Errorf("worktree is at %.12s, want the pinned %.12s", at, pinned)
	}
	if at == tip {
		t.Error("the worktree drifted to the tip; the commit in run-meta.json would not be the commit that was read")
	}
	content := readFile(t, filepath.Join(dest, "app.go"))
	if strings.Contains(content, "func New") {
		t.Error("the worktree holds the later commit's content")
	}
}

func TestAWorktreeIsDetachedSoNoBranchCanMoveUnderIt(t *testing.T) {
	parent, pinned, _ := parentRepo(t)
	dest := filepath.Join(t.TempDir(), "repo")
	if err := isolate.AddWorktree(context.Background(), parent, pinned, dest); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	// symbolic-ref fails on a detached HEAD, which is the state we want.
	cmd := exec.Command("git", "-C", dest, "symbolic-ref", "-q", "HEAD")
	if out, err := cmd.CombinedOutput(); err == nil {
		t.Errorf("the worktree is on branch %s; a branch is a second thing that could move", strings.TrimSpace(string(out)))
	}
}

func TestAWorktreeIsRemovedCompletely(t *testing.T) {
	parent, pinned, _ := parentRepo(t)
	dest := filepath.Join(t.TempDir(), "repo")
	if err := isolate.AddWorktree(context.Background(), parent, pinned, dest); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}

	if err := isolate.RemoveWorktree(context.Background(), parent, dest); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}

	// Checked after cleanup, not assumed from a clean exit.
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("Stat(%s) = %v after removal, want the worktree gone", dest, err)
	}
	if list := gitOut(t, parent, "worktree", "list"); strings.Contains(list, dest) {
		t.Errorf("the parent still lists %s:\n%s", dest, list)
	}
}

func TestAWorktreeTheSessionDirtiedIsStillRemoved(t *testing.T) {
	// A session is expected to have written to the repository. A run that
	// modified the tree must still clean up, or the next run inherits it.
	parent, pinned, _ := parentRepo(t)
	dest := filepath.Join(t.TempDir(), "repo")
	if err := isolate.AddWorktree(context.Background(), parent, pinned, dest); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dest, "app.go"), []byte("package app // edited by the session\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "scratch.txt"), []byte("notes"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := isolate.RemoveWorktree(context.Background(), parent, dest); err != nil {
		t.Fatalf("RemoveWorktree: %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Errorf("Stat(%s) = %v, want a dirty worktree removed anyway", dest, err)
	}
}

func TestAWorktreeAtACommitTheParentDoesNotHaveIsRefused(t *testing.T) {
	parent, _, _ := parentRepo(t)
	dest := filepath.Join(t.TempDir(), "repo")

	err := isolate.AddWorktree(context.Background(), parent, "0000000000000000000000000000000000000000", dest)

	if err == nil {
		t.Fatal("AddWorktree succeeded at a commit the parent does not have")
	}
	if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
		t.Errorf("a failed AddWorktree left %s behind", dest)
	}
}

func TestRemovingAWorktreeThatIsNotThereIsReported(t *testing.T) {
	parent, _, _ := parentRepo(t)

	if err := isolate.RemoveWorktree(context.Background(), parent, filepath.Join(t.TempDir(), "never-added")); err == nil {
		t.Fatal("RemoveWorktree reported success on a worktree that was never added")
	}
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}
