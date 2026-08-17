package channels_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/channels"
)

// hostHome is a stand-in for the operator's HOME, carrying the three pieces of
// state a run must leave exactly as it found them.
func hostHome(t *testing.T) (home, repo string) {
	t.Helper()
	home = t.TempDir()
	repo = filepath.Join(t.TempDir(), "checkout")
	write(t, filepath.Join(home, ".claude.json"), `{"projects":{}}`)
	write(t, filepath.Join(home, ".claude", "CLAUDE.md"), "answer in under six lines")
	write(t, filepath.Join(home, ".claude", "settings.json"), `{"hooks":{}}`)
	write(t, filepath.Join(channels.MemoryDir(home, ".claude", repo), "MEMORY.md"), "- a fact from a previous run")
	return home, repo
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func snapshot(t *testing.T, home, repo string) channels.Snapshot {
	t.Helper()
	s, err := channels.Take(channels.HostWatch(home, []string{".claude"}, repo))
	if err != nil {
		t.Fatalf("Take: %v", err)
	}
	return s
}

func changed(t *testing.T, s channels.Snapshot) []string {
	t.Helper()
	c, err := s.Changed()
	if err != nil {
		t.Fatalf("Changed: %v", err)
	}
	return c
}

func TestAHostThatWasNotTouchedReportsNothing(t *testing.T) {
	home, repo := hostHome(t)

	if got := changed(t, snapshot(t, home, repo)); len(got) != 0 {
		t.Errorf("an untouched host reports %v", got)
	}
}

func TestAnEditedHostAgentConfigIsReported(t *testing.T) {
	home, repo := hostHome(t)
	before := snapshot(t, home, repo)

	write(t, filepath.Join(home, ".claude.json"), `{"projects":{"/runs/r1/repo":{}}}`)

	got := changed(t, before)
	if len(got) != 1 || !strings.Contains(got[0], "modified") {
		t.Fatalf("Changed = %v, want the host agent config reported as modified", got)
	}
}

func TestAnEditedOperatorGuidanceFileIsReported(t *testing.T) {
	// This is the seventh channel: the operator's own CLAUDE.md reaches both
	// arms and suppresses answer length in both, which is what a richness floor
	// and a recall count are measured on.
	home, repo := hostHome(t)
	before := snapshot(t, home, repo)

	write(t, filepath.Join(home, ".claude", "CLAUDE.md"), "answer in under three lines")

	if got := changed(t, before); len(got) != 1 {
		t.Fatalf("Changed = %v, want the operator's guidance reported", got)
	}
}

func TestAFileTheRunAddedToTheHostMemoryDirectoryIsReported(t *testing.T) {
	// A directory digest must cover additions, not only edits: a run that
	// appended a new memory file has left state the next run would inherit.
	home, repo := hostHome(t)
	before := snapshot(t, home, repo)

	write(t, filepath.Join(channels.MemoryDir(home, ".claude", repo), "learned.md"), "- a fact from this run")

	got := changed(t, before)
	if len(got) != 1 || !strings.Contains(got[0], "modified") {
		t.Fatalf("Changed = %v, want the memory directory reported", got)
	}
}

func TestAHostPathTheRunCreatedIsReported(t *testing.T) {
	// Absent is a recorded state, not a skipped one. A run that CREATES the
	// operator's memory directory has contaminated the host just as surely as
	// one that edited it.
	home := t.TempDir()
	repo := filepath.Join(t.TempDir(), "checkout")
	before := snapshot(t, home, repo)

	write(t, filepath.Join(channels.MemoryDir(home, ".claude", repo), "MEMORY.md"), "- written by the run")

	got := changed(t, before)
	if len(got) != 1 || !strings.Contains(got[0], "created") {
		t.Fatalf("Changed = %v, want the created path reported", got)
	}
}

func TestAHostPathTheRunRemovedIsReported(t *testing.T) {
	home, repo := hostHome(t)
	before := snapshot(t, home, repo)

	if err := os.Remove(filepath.Join(home, ".claude.json")); err != nil {
		t.Fatal(err)
	}

	got := changed(t, before)
	if len(got) != 1 || !strings.Contains(got[0], "removed") {
		t.Fatalf("Changed = %v, want the removed path reported", got)
	}
}

func TestTheWatchListIsTheChannelPathsAndNoMore(t *testing.T) {
	// Scoped on purpose. A general sweep of HOME is flaky the moment any other
	// process on the machine writes something, and a flaky check is a check
	// somebody disables.
	home, repo := hostHome(t)

	got := channels.HostWatch(home, []string{".claude"}, repo)

	want := []string{
		filepath.Join(home, ".claude.json"),
		filepath.Join(home, ".claude", "CLAUDE.md"),
		filepath.Join(home, ".claude", "settings.json"),
		channels.MemoryDir(home, ".claude", repo),
	}
	if len(got) != len(want) {
		t.Fatalf("HostWatch = %v, want %v", got, want)
	}
	for i, path := range want {
		if got[i] != path {
			t.Errorf("HostWatch[%d] = %s, want %s", i, got[i], path)
		}
	}
	// Unrelated host state a run may legitimately touch must not be watched.
	if unrelated := filepath.Join(home, ".cache"); containsPath(got, unrelated) {
		t.Errorf("HostWatch watches %s, which is outside the channels", unrelated)
	}
}

func containsPath(paths []string, want string) bool {
	for _, p := range paths {
		if p == want {
			return true
		}
	}
	return false
}

func TestAPathThatCannotBeReadIsAnErrorRatherThanAPass(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission that makes the read fail")
	}
	home := t.TempDir()
	repo := filepath.Join(t.TempDir(), "checkout")
	secret := filepath.Join(home, ".claude.json")
	write(t, secret, "{}")
	if err := os.Chmod(secret, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(secret, 0o600) })

	// "I could not look" is not "nothing changed": reporting a pass here would
	// clear a run on exactly the machines where something is already wrong.
	if _, err := channels.Take(channels.HostWatch(home, []string{".claude"}, repo)); err == nil {
		t.Fatal("Take reported success on a path it could not read")
	}
}

func TestAPathThatBecomesUnreadableDuringTheRunIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission that makes the read fail")
	}
	home, repo := hostHome(t)
	before := snapshot(t, home, repo)

	memory := channels.MemoryDir(home, ".claude", repo)
	if err := os.Chmod(memory, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(memory, 0o700) })

	if _, err := before.Changed(); err == nil {
		t.Fatal("Changed reported success on a path it could not re-read")
	}
}

func TestAHomeThatIsNotADirectoryIsAnErrorRatherThanAnAllClear(t *testing.T) {
	// Lstat fails with something other than "not there". Treating that as
	// absent would report a clean host on a machine whose HOME is misconfigured,
	// which is the one case where a clean bill of health is least deserved.
	notADir := filepath.Join(t.TempDir(), "home")
	write(t, notADir, "this is a file")

	if _, err := channels.Take(channels.HostWatch(notADir, []string{".claude"}, "/runs/r1/repo")); err == nil {
		t.Fatal("Take reported success against a HOME that is not a directory")
	}
}

func TestATreeSkipsTheDirectoriesItIsToldTo(t *testing.T) {
	// .git and .sense are machinery rather than channels, and both are large
	// enough to make hashing them the slowest step in a run.
	root := t.TempDir()
	write(t, filepath.Join(root, "app.go"), "package app")
	write(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main")
	write(t, filepath.Join(root, ".sense", "index.db"), "binary")

	tree, err := channels.Tree(root, ".git", ".sense")
	if err != nil {
		t.Fatalf("Tree: %v", err)
	}

	if got := channels.Sorted(tree); len(got) != 1 || got[0] != "app.go" {
		t.Errorf("Tree = %v, want only app.go", got)
	}
}

// mastodon ships public/500.html pointing at an asset that only exists after a
// build. Following it stopped subject preparation before either arm spawned,
// which is a repository the instrument simply could not measure.
func TestABrokenSymlinkDoesNotStopTheWalk(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "app.rb"), "class Account; end")
	if err := os.Symlink("assets/500.html", filepath.Join(root, "500.html")); err != nil {
		t.Fatal(err)
	}

	tree, err := channels.Tree(root)
	if err != nil {
		t.Fatalf("a broken symlink stopped the walk: %v", err)
	}
	if _, ok := tree["app.rb"]; !ok {
		t.Error("the real file was lost")
	}
	if got := tree["500.html"]; got != "symlink:assets/500.html" {
		t.Errorf("the link is recorded as %q, want its target", got)
	}
}

// Recorded rather than skipped: a run that rewires a link has changed the tree,
// and a skipped entry would hide it.
func TestARewiredSymlinkIsAChange(t *testing.T) {
	root := t.TempDir()
	link := filepath.Join(root, "config.yml")
	if err := os.Symlink("config/production.yml", link); err != nil {
		t.Fatal(err)
	}
	before, err := channels.Tree(root)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("config/staging.yml", link); err != nil {
		t.Fatal(err)
	}
	after, err := channels.Tree(root)
	if err != nil {
		t.Fatal(err)
	}
	if before["config.yml"] == after["config.yml"] {
		t.Errorf("a link pointing somewhere else reads as unchanged: %q", after["config.yml"])
	}
}

// Reading a fifo blocks until somebody writes to it. This walk runs before the
// session, so nothing is counting and it would hang with no output.
func TestAnIrregularFileIsRecordedWithoutBeingOpened(t *testing.T) {
	root := t.TempDir()
	fifo := filepath.Join(root, "pipe")
	if out, err := exec.Command("mkfifo", fifo).CombinedOutput(); err != nil {
		t.Skipf("mkfifo unavailable: %v: %s", err, out)
	}
	write(t, filepath.Join(root, "app.rb"), "class Account; end")

	done := make(chan struct{})
	var tree map[string]string
	var err error
	go func() {
		tree, err = channels.Tree(root)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("the walk hung on a fifo")
	}
	if err != nil {
		t.Fatalf("an irregular file stopped the walk: %v", err)
	}
	if !strings.HasPrefix(tree["pipe"], "irregular:") {
		t.Errorf("the fifo is recorded as %q", tree["pipe"])
	}
}
