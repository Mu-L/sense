package channels_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/channels"
)

// fakeSense writes a stand-in for the product binary whose `setup` creates the
// given repository-relative files in its working directory.
//
// The mechanism under test is "run setup and read back what it wrote", and a
// stand-in states exactly what was written, so a failure is the derivation's
// rather than the product's. The real binary is exercised separately, below.
func fakeSense(t *testing.T, writes ...string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	script := "#!/bin/sh\nset -e\n"
	for _, rel := range writes {
		script += "mkdir -p \"$(dirname " + rel + ")\"\n"
		script += "echo configured > " + rel + "\n"
	}
	path := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDeriveReportsEveryFileSetupWroteIntoTheProject(t *testing.T) {
	bin := fakeSense(t, ".mcp.json", "CLAUDE.md", ".claude/settings.json", ".claude/skills/sense-explore.md")

	got, err := channels.Derive(context.Background(), bin, filepath.Join(t.TempDir(), "probe"))
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}

	var repo []string
	for _, c := range got {
		if c.Kind == channels.Repository {
			repo = append(repo, c.Rel)
		}
	}
	want := []string{".claude/settings.json", ".claude/skills/sense-explore.md", ".mcp.json", "CLAUDE.md"}
	if !slices.Equal(repo, want) {
		t.Errorf("derived repository channels = %v, want %v", repo, want)
	}
}

func TestDerivePicksUpAChannelNobodyWroteDown(t *testing.T) {
	// The whole reason the list is derived: a channel added to the product must
	// show up without an edit to the bench. A hard-coded list would still be
	// four entries long here and every absence check would still pass.
	bin := fakeSense(t, ".mcp.json", "CLAUDE.md", ".claude/settings.json", ".cursor/mcp.json")

	got, err := channels.Derive(context.Background(), bin, filepath.Join(t.TempDir(), "probe"))
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}

	if !slices.ContainsFunc(got, func(c channels.Channel) bool { return c.Rel == ".cursor/mcp.json" }) {
		t.Errorf("derived channels %v do not include the newly written .cursor/mcp.json", got)
	}
}

func TestDeriveAlsoNamesTheTwoChannelsNoFileCanReveal(t *testing.T) {
	bin := fakeSense(t, ".mcp.json")

	got, err := channels.Derive(context.Background(), bin, filepath.Join(t.TempDir(), "probe"))
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}

	for _, kind := range []channels.Kind{channels.Path, channels.Home} {
		if !slices.ContainsFunc(got, func(c channels.Channel) bool { return c.Kind == kind }) {
			t.Errorf("derived channels do not name the %s channel; nothing setup writes would reveal it", kind)
		}
	}
}

func TestDeriveRefusesASetupThatWroteNothing(t *testing.T) {
	// An empty derived list makes every absence check pass, which is the shape
	// of a leak that reads as a clean bill of health.
	bin := fakeSense(t)

	_, err := channels.Derive(context.Background(), bin, filepath.Join(t.TempDir(), "probe"))

	if err == nil {
		t.Fatal("Derive accepted a setup that wrote nothing")
	}
	if !strings.Contains(err.Error(), "wrote nothing") {
		t.Errorf("Derive error = %v, want it to name the empty result", err)
	}
}

func TestDeriveReportsASetupThatFailed(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "not-a-binary")

	if _, err := channels.Derive(context.Background(), bin, filepath.Join(t.TempDir(), "probe")); err == nil {
		t.Fatal("Derive succeeded with no usable binary")
	}
}

func TestDeriveCannotWriteIntoTheProbeItCannotCreate(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := channels.Derive(context.Background(), fakeSense(t, ".mcp.json"), filepath.Join(blocked, "work")); err == nil {
		t.Fatal("Derive succeeded with an unusable work directory")
	}
}

// The absence proof.

func cleanArm(t *testing.T) channels.Arm {
	t.Helper()
	dir := t.TempDir()
	repo := filepath.Join(dir, "repo")
	home := filepath.Join(dir, "home")
	for _, d := range []string{repo, home} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return channels.Arm{Repo: repo, Home: home, PathValue: "/nonexistent/bin", SenseBinary: "sense"}
}

func TestTheBaselineArmReachesNothing(t *testing.T) {
	cs := []channels.Channel{
		{Name: "the MCP registration", Kind: channels.Repository, Rel: ".mcp.json"},
		{Name: "the routing guidance", Kind: channels.Repository, Rel: "CLAUDE.md"},
		{Name: "the sense binary on PATH", Kind: channels.Path},
		{Name: "the persisted memory directory", Kind: channels.Home},
	}

	if reached := channels.Absent(cs, cleanArm(t)); len(reached) != 0 {
		t.Errorf("a clean arm reaches %v", reached)
	}
}

func TestARepositoryChannelIsReportedByName(t *testing.T) {
	arm := cleanArm(t)
	if err := os.WriteFile(filepath.Join(arm.Repo, ".mcp.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	cs := []channels.Channel{
		{Name: "the MCP registration", Kind: channels.Repository, Rel: ".mcp.json"},
		{Name: "the routing guidance", Kind: channels.Repository, Rel: "CLAUDE.md"},
	}

	reached := channels.Absent(cs, arm)

	if len(reached) != 1 {
		t.Fatalf("Absent = %v, want exactly the MCP registration", reached)
	}
	// "contaminated" does not say which route leaked, and the route is the
	// whole diagnosis.
	if !strings.Contains(reached[0], "the MCP registration") {
		t.Errorf("Absent reported %q, which does not name the channel", reached[0])
	}
}

func TestTheSenseBinaryOnPathIsReported(t *testing.T) {
	arm := cleanArm(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "sense"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	arm.PathValue = strings.Join([]string{"/nonexistent/bin", bin}, string(filepath.ListSeparator))

	reached := channels.Absent([]channels.Channel{{Name: "the sense binary on PATH", Kind: channels.Path}}, arm)

	if len(reached) != 1 {
		t.Fatalf("Absent = %v, want the CLI fallback reported", reached)
	}
}

func TestANonExecutableFileNamedSenseIsNotACliFallback(t *testing.T) {
	arm := cleanArm(t)
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "sense"), []byte("notes about sense"), 0o644); err != nil {
		t.Fatal(err)
	}
	arm.PathValue = bin

	if reached := channels.Absent([]channels.Channel{{Kind: channels.Path, Name: "path"}}, arm); len(reached) != 0 {
		t.Errorf("Absent = %v; a file that cannot be executed is not a fallback", reached)
	}
}

func TestAnEmptyPathElementIsNotSearched(t *testing.T) {
	arm := cleanArm(t)
	arm.PathValue = string(filepath.ListSeparator)

	if reached := channels.Absent([]channels.Channel{{Kind: channels.Path, Name: "path"}}, arm); len(reached) != 0 {
		t.Errorf("Absent = %v for an empty PATH", reached)
	}
}

func TestNoBinaryNameMeansNoPathChannelToCheck(t *testing.T) {
	arm := cleanArm(t)
	arm.SenseBinary = ""

	if reached := channels.Absent([]channels.Channel{{Kind: channels.Path, Name: "path"}}, arm); len(reached) != 0 {
		t.Errorf("Absent = %v with no binary named", reached)
	}
}

// The memory directory gets its own test because it is the one channel outside
// the repository, and the one a reasonable implementation misses: nothing in
// the worktree or the prompt would show it.

func TestThePersistedMemoryDirectoryIsReportedEvenThoughItIsOutsideTheRepository(t *testing.T) {
	arm := cleanArm(t)
	memory := channels.MemoryDir(arm.Home, arm.Repo)
	if err := os.MkdirAll(memory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memory, "MEMORY.md"), []byte("- use sense_graph"), 0o644); err != nil {
		t.Fatal(err)
	}

	reached := channels.Absent([]channels.Channel{{Name: "the persisted memory directory", Kind: channels.Home}}, arm)

	if len(reached) != 1 {
		t.Fatalf("Absent = %v, want the memory directory reported", reached)
	}
	if !strings.Contains(reached[0], memory) {
		t.Errorf("Absent reported %q, want it to name %s", reached[0], memory)
	}
}

func TestAgentStateInTheDisposableHomeIsReportedEvenUnderAnotherKey(t *testing.T) {
	// The directory name depends on how the agent tool flattens a repository
	// path, which is observed rather than documented. If that ever changes, the
	// exact key stops matching and the whole tree is what catches it.
	arm := cleanArm(t)
	if err := os.MkdirAll(filepath.Join(arm.Home, ".claude", "projects", "some-other-key"), 0o755); err != nil {
		t.Fatal(err)
	}

	reached := channels.Absent([]channels.Channel{{Name: "the persisted memory directory", Kind: channels.Home}}, arm)

	if len(reached) != 1 {
		t.Fatalf("Absent = %v, want agent state in the disposable HOME reported", reached)
	}
}

func TestTheMemoryDirectoryIsKeyedOffTheRepositoryPath(t *testing.T) {
	// Two arms on different worktrees must not share a memory directory: that
	// is the shared mutable host state the per-run worktree exists to separate.
	sense := channels.MemoryDir("/home/bench", "/runs/r1/repo")
	baseline := channels.MemoryDir("/home/bench", "/runs/r2/repo")

	if sense == baseline {
		t.Fatalf("both arms resolve to %s", sense)
	}
	if strings.ContainsAny(filepath.Base(filepath.Dir(sense)), "/._") {
		t.Errorf("the directory key %q is not flattened", filepath.Base(filepath.Dir(sense)))
	}
}

// The real product binary, when it is around. The stand-in above proves the
// derivation; this proves the derivation is pointed at the right thing.

func TestTheRealBinaryWritesTheRepositoryChannelsWeExpect(t *testing.T) {
	bin, err := filepath.Abs(filepath.Join("..", "..", "..", "bin", "sense"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(bin); err != nil {
		t.Skip("no built sense binary; make build")
	}

	got, err := channels.Derive(context.Background(), bin, filepath.Join(t.TempDir(), "probe"))
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}

	// Not the whole list, which is the product's to change. These four are the
	// routes the cycle names, and losing one silently would mean the baseline
	// arm stopped being checked for it.
	for _, rel := range []string{".mcp.json", "CLAUDE.md", ".claude/settings.json"} {
		if !slices.ContainsFunc(got, func(c channels.Channel) bool { return c.Rel == rel }) {
			t.Errorf("sense setup no longer writes %s; the derived channel list has changed", rel)
		}
	}
	if !slices.ContainsFunc(got, func(c channels.Channel) bool { return strings.HasPrefix(c.Rel, ".claude/skills/") }) {
		t.Error("sense setup no longer writes any skill file")
	}
}
