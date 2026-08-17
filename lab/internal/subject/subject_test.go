package subject_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/channels"
	"github.com/luuuc/sense/lab/internal/subject"
)

// fakeSense stands in for the product binary, reproducing the one behaviour
// this pitch turns on: `scan` configures the repository as a side effect
// whenever the index directory is absent, exactly as the real first-run branch
// does, and `setup` configures it deliberately.
//
// The stand-in is what makes the trap testable. Against the real binary a
// missing pre-created index directory shows up as an extra .mcp.json nobody
// looks at; here it shows up as a failing assertion.
func fakeSense(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	const script = `#!/bin/sh
set -e
configure() {
  printf '{"mcpServers":{"sense":{}}}' > "$1/.mcp.json"
  mkdir -p "$1/.claude/skills"
  printf '{"hooks":{}}' > "$1/.claude/settings.json"
  printf '# guidance\n' > "$1/CLAUDE.md"
  printf '# explore\n' > "$1/.claude/skills/sense-explore.md"
}
case "$1" in
scan)
  repo="$3"
  if [ ! -d "$repo/.sense" ]; then
    # The first-run branch, and the trap: an unindexed repository is configured
    # by the scan, writing EXACTLY what setup would have written.
    configure "$repo"
  fi
  mkdir -p "$repo/.sense"
  printf 'index' > "$repo/.sense/index.db"
  printf '.sense/\n' >> "$repo/.gitignore"
  ;;
setup)
  configure .
  ;;
esac
`
	path := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// repoWithSource is a project with something to index and nothing else.
func repoWithSource(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "app.go"), []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func prepare(t *testing.T) (repo string, wrote map[string]string) {
	t.Helper()
	repo = repoWithSource(t)
	wrote, err := subject.Sense(context.Background(), fakeSense(t), "claude-code", repo)
	if err != nil {
		t.Fatalf("Sense: %v", err)
	}
	return repo, wrote
}

func TestIndexingTheSubjectDoesNotSilentlyConfigureIt(t *testing.T) {
	// The trap: `sense scan` runs setup itself on a repository with no index,
	// so the naive preparation configures the repository as a side effect and
	// the record of what setup wrote comes back empty. Creating the index
	// directory first is what makes configuring it an explicit act.
	repo, wrote := prepare(t)

	if _, ok := wrote[".mcp.json"]; !ok {
		t.Errorf("the recorded setup %v does not include .mcp.json; the scan configured the repository before setup ran, and the record is now blind to it",
			channels.Sorted(wrote))
	}
	if _, err := os.Stat(filepath.Join(repo, ".mcp.json")); err != nil {
		t.Errorf(".mcp.json is not in the repository: %v", err)
	}
}

func TestTheRecordNamesEveryFileSetupWroteWithItsHash(t *testing.T) {
	// Recorded per run because a change to `sense setup` changes what the sense
	// arm sees, and nothing about that is visible in a result months later.
	_, wrote := prepare(t)

	for _, rel := range []string{".mcp.json", "CLAUDE.md", ".claude/settings.json", ".claude/skills/sense-explore.md"} {
		hash, ok := wrote[rel]
		if !ok {
			t.Errorf("the recorded setup %v does not include %s", channels.Sorted(wrote), rel)
			continue
		}
		if len(hash) != 64 {
			t.Errorf("%s is recorded as %q, which is not a content hash", rel, hash)
		}
	}
}

func TestTwoSetupsThatWroteDifferentContentAreDistinguishable(t *testing.T) {
	// The whole point of hashing rather than listing: setup drift is a change in
	// what the arm saw, and two runs far apart must be comparable on it.
	_, first := prepare(t)
	_, second := prepare(t)
	if first[".mcp.json"] != second[".mcp.json"] {
		t.Fatal("the same setup produced two different hashes")
	}

	repo := repoWithSource(t)
	drifted := fakeSenseWriting(t, `printf '{"mcpServers":{"sense":{"args":["mcp","--new"]}}}' > .mcp.json`)
	changed, err := subject.Sense(context.Background(), drifted, "claude-code", repo)
	if err != nil {
		t.Fatalf("Sense: %v", err)
	}

	if changed[".mcp.json"] == first[".mcp.json"] {
		t.Error("a setup that wrote different content recorded the same hash")
	}
}

func TestWhatTheScanLeftBehindIsNotReportedAsSetup(t *testing.T) {
	// The scan appends a .gitignore line and builds an index. Neither is a
	// channel, and reporting them as setup's work would make the record noise
	// that nobody reads.
	_, wrote := prepare(t)

	if slices.ContainsFunc(channels.Sorted(wrote), func(rel string) bool {
		return rel == ".gitignore" || strings.HasPrefix(rel, ".sense/")
	}) {
		t.Errorf("the recorded setup %v includes what the scan left behind", channels.Sorted(wrote))
	}
}

func TestAFailedScanStopsBeforeAnythingIsConfigured(t *testing.T) {
	repo := repoWithSource(t)
	broken := fakeSenseFailing(t, "scan")

	_, err := subject.Sense(context.Background(), broken, "claude-code", repo)

	if err == nil {
		t.Fatal("Sense succeeded with a scan that failed")
	}
	if !strings.Contains(err.Error(), "index the subject") {
		t.Errorf("error = %v, want it to name the indexing step", err)
	}
}

func TestAFailedSetupIsReportedRatherThanRecordedAsEmpty(t *testing.T) {
	// An empty record and a failed setup look identical to whoever reads the run
	// later, and one of them means the sense arm ran without Sense configured.
	repo := repoWithSource(t)
	broken := fakeSenseFailing(t, "setup")

	_, err := subject.Sense(context.Background(), broken, "claude-code", repo)

	if err == nil {
		t.Fatal("Sense succeeded with a setup that failed")
	}
	if !strings.Contains(err.Error(), "configure the subject") {
		t.Errorf("error = %v, want it to name the configuring step", err)
	}
}

func TestAnUnusableIndexDirectoryIsRefused(t *testing.T) {
	notARepo := filepath.Join(t.TempDir(), "repo")
	if err := os.WriteFile(notARepo, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := subject.Sense(context.Background(), fakeSense(t), "claude-code", notARepo); err == nil {
		t.Fatal("Sense succeeded where the index directory cannot be created")
	}
}

func TestTheIndexLandsWhereTheMcpServerLooksForIt(t *testing.T) {
	// A sense arm whose index is elsewhere is configured, reports no error, and
	// reaches an empty index: the worst possible failure, because it looks like
	// a measurement.
	repo, _ := prepare(t)

	if _, err := os.Stat(filepath.Join(repo, ".sense")); err != nil {
		t.Errorf("no index at <repo>/.sense: %v", err)
	}
}

func TestAnUnreadableSubjectTreeIsReported(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission that makes the read fail")
	}
	repo := repoWithSource(t)
	locked := filepath.Join(repo, "vendor")
	if err := os.MkdirAll(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	if _, err := subject.Sense(context.Background(), fakeSense(t), "claude-code", repo); err == nil {
		t.Fatal("Sense succeeded on a tree it could not read")
	}
}

// The baseline arm gets none of this, and that is checked rather than assumed.

func TestTheBaselineArmsRepositoryReachesNoChannelAfterPreparation(t *testing.T) {
	derived, err := channels.Derive(context.Background(), fakeSenseWriting(t,
		`printf '{}' > .mcp.json; mkdir -p .claude; printf '{}' > .claude/settings.json; printf '#\n' > CLAUDE.md`),
		"claude-code", filepath.Join(t.TempDir(), "probe"))
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}

	// Preparing the baseline arm is doing nothing to it, and this is what
	// proves the nothing.
	baseline := repoWithSource(t)
	arm := channels.Arm{Repo: baseline, Home: t.TempDir(), PathValue: "/nonexistent/bin", SenseBinary: "sense", ConfigDirs: []string{".claude"}}

	if reached := channels.Absent(derived, arm); len(reached) != 0 {
		t.Errorf("the baseline arm reaches %v", reached)
	}
}

// fakeSenseWriting is a stand-in whose `setup` runs the given shell and whose
// `scan` does nothing.
func fakeSenseWriting(t *testing.T, setup string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	script := "#!/bin/sh\nset -e\ncase \"$1\" in\nsetup)\n" + setup + "\n;;\nesac\n"
	path := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakeSenseFailing is a stand-in that fails on one subcommand.
func fakeSenseFailing(t *testing.T, failOn string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	script := "#!/bin/sh\nif [ \"$1\" = \"" + failOn + "\" ]; then echo boom >&2; exit 1; fi\nexit 0\n"
	path := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
