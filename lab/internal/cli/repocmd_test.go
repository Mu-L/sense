package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// admissionStatus is what the stand-in server answers, trimmed to the keys the
// artifact reads.
const admissionStatus = `{"index":{"files":12,"symbols":40,"edges":55,"embeddings":40,"coverage":1},` +
	`"languages":{"go":{"files":12,"symbols":40,"tier":"full"}},"profile":{"tier":"small"},` +
	`"version":{"binary":"1.14.1-test"}}`

// senseStandIn indexes and answers sense_status, and nothing else. Admission
// runs the product binary the way a user would, so the stand-in is a binary
// too.
//
// It is permissive on purpose: what it stands in for here is a scan that
// happened, so that the wiring around it — which files are written, which are
// not, and in what order — is what these tests are about. Whether the exchange
// the lab sends is one Sense answers, and whether the server is asked in the
// checkout, are pinned in lab/internal/repo against the real binary.
func senseStandIn(t *testing.T, status string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in is a shell script")
	}
	quoted, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "sense")
	script := `#!/bin/sh
set -e
case "$1" in
scan) mkdir -p "$3/.sense" ;;
mcp)
  while IFS= read -r line; do
    case "$line" in
      *'"tools/call"'*) printf '{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":%s}]}}\n' '` + string(quoted) + `' ;;
    esac
  done
  ;;
esac
`
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return bin
}

// sourceRepo is a repository to admit: real git, two commits, so a test can
// move one and see what admission does about it.
func sourceRepo(t *testing.T) (dir, first string) {
	t.Helper()
	dir = t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "app.go"), []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "-c", "user.email=lab@example.test", "-c", "user.name=lab", "commit", "-q", "-m", "first")
	// A real clone knows where it came from, and that origin is what a handed-in
	// repository is recorded with.
	git(t, dir, "remote", "add", "origin", "https://example.test/"+filepath.Base(dir)+".git")
	return dir, git(t, dir, "rev-parse", "HEAD")
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// commit moves a checkout forward, which is how a clone drifts off its pin.
func commit(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "add", ".")
	git(t, dir, "-c", "user.email=lab@example.test", "-c", "user.name=lab", "commit", "-q", "-m", name)
}

// admission is one run of the command, with the paths a test cares about.
type admission struct {
	config, campaign, checkouts, sense string
}

func newAdmission(t *testing.T, status string) admission {
	t.Helper()
	root := t.TempDir()
	config := filepath.Join(root, "lab")
	// The catalog reads all five kinds, so an admission runs against a config
	// directory that is empty rather than absent — which is what a first
	// admission actually meets.
	for _, kind := range []string{"subjects", "agents", "models", "repos", "executors"} {
		if err := os.MkdirAll(filepath.Join(config, kind), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return admission{
		config:    config,
		campaign:  filepath.Join(root, "campaign"),
		checkouts: filepath.Join(root, "checkouts"),
		sense:     senseStandIn(t, status),
	}
}

func (a admission) run(t *testing.T, name string) (int, string, string) {
	t.Helper()
	return dispatch(t, "repo", "-config", a.config, "-campaign", a.campaign,
		"-checkouts", a.checkouts, "-sense", a.sense, name)
}

func (a admission) repoFile(id string) string { return filepath.Join(a.config, "repos", id+".json") }
func (a admission) artifact(id string) string {
	return filepath.Join(a.campaign, id, "index", "index.json")
}

// The whole point of the command: a repository nobody declared goes from a name
// to a pinned clone, an index artifact and a repository file, in one call.
func TestAUrlIsClonedPinnedIndexedAndRecorded(t *testing.T) {
	source, head := sourceRepo(t)
	a := newAdmission(t, admissionStatus)

	code, stdout, stderr := a.run(t, "file://"+source)

	if code != 0 {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "the lab will clone it") {
		t.Errorf("stdout = %q, want it to say what it was about to do", stdout)
	}

	var r struct {
		ID, URL, Commit, Checkout string
		Languages                 []string
	}
	readJSON(t, a.repoFile(filepath.Base(source)), &r)
	if r.Commit != head {
		t.Errorf("commit = %q, want the revision the clone is at (%q)", r.Commit, head)
	}
	if r.Checkout != "" {
		t.Errorf("checkout = %q, want none: the lab made this clone and owns it", r.Checkout)
	}
	if len(r.Languages) != 1 || r.Languages[0] != "go" {
		t.Errorf("languages = %v, want what the scan found", r.Languages)
	}

	var i struct {
		Revision, Checkout string
		Symbols            int
	}
	readJSON(t, a.artifact(filepath.Base(source)), &i)
	if i.Symbols != 40 || i.Revision != head {
		t.Errorf("artifact = %+v, want the counts and the pin", i)
	}
	if _, err := os.Stat(filepath.Join(a.checkouts, filepath.Base(source))); err != nil {
		t.Errorf("the clone is not under the lab's own root: %v", err)
	}
}

// The pin is read out of the clone. A repository file whose sha is not the sha
// the clone sits at gives every later worktree the wrong tree, and nothing
// about a result says so.
func TestAHandedInCloneIsReadAndRecordedAtItsOwnHead(t *testing.T) {
	source, head := sourceRepo(t)
	a := newAdmission(t, admissionStatus)

	code, stdout, stderr := a.run(t, source)

	if code != 0 {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "read and never written to") {
		t.Errorf("stdout = %q, want it to say the clone is not the lab's", stdout)
	}
	var r struct{ Commit, Checkout, URL string }
	readJSON(t, a.repoFile(filepath.Base(source)), &r)
	if r.Commit != head {
		t.Errorf("commit = %q, want %q", r.Commit, head)
	}
	if r.Checkout != source {
		t.Errorf("checkout = %q, want the handed-in path recorded, which is what says the lab does not own it", r.Checkout)
	}
	if r.URL != "https://example.test/"+filepath.Base(source)+".git" {
		t.Errorf("url = %q, want the clone's own origin", r.URL)
	}
}

// Admission is idempotent because the crank calls it on every invocation. A
// second call admits nothing and rewrites nothing.
func TestRe_runningOnAnAdmittedRepositoryWritesNothing(t *testing.T) {
	source, _ := sourceRepo(t)
	a := newAdmission(t, admissionStatus)
	id := filepath.Base(source)
	if code, _, stderr := a.run(t, source); code != 0 {
		t.Fatalf("first admission: %s", stderr)
	}
	before := read(t, a.repoFile(id))
	artifactBefore := read(t, a.artifact(id))

	code, stdout, stderr := a.run(t, id)

	if code != 0 {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "indexed:  yes") || !strings.Contains(stdout, "awaiting: author") {
		t.Errorf("stdout = %q, want where the repository stands", stdout)
	}
	if read(t, a.repoFile(id)) != before {
		t.Error("the repository file was rewritten by a second admission")
	}
	if read(t, a.artifact(id)) != artifactBefore {
		t.Error("the repository was re-indexed by a second admission")
	}
}

// The lab fixes what it made: an unattended crank must not park a repository on
// a stale tree nobody is watching.
func TestALabOwnedCloneThatDriftedIsPutBack(t *testing.T) {
	source, head := sourceRepo(t)
	a := newAdmission(t, admissionStatus)
	id := filepath.Base(source)
	if code, _, stderr := a.run(t, "file://"+source); code != 0 {
		t.Fatalf("first admission: %s", stderr)
	}
	clone := filepath.Join(a.checkouts, id)
	commit(t, clone, "drift.go")

	code, stdout, stderr := a.run(t, id)

	if code != 0 {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "moving back to its pin") {
		t.Errorf("stdout = %q, want it to say the clone is being corrected", stdout)
	}
	if at := git(t, clone, "rev-parse", "HEAD"); at != head {
		t.Errorf("the clone is at %q, want its pin %q", at, head)
	}
}

// And it reads what it was given: a clone somebody handed in is their working
// tree, and the lab does not perform a checkout in it.
func TestAHandedInCloneThatDriftedIsRefusedRatherThanMoved(t *testing.T) {
	source, head := sourceRepo(t)
	a := newAdmission(t, admissionStatus)
	if code, _, stderr := a.run(t, source); code != 0 {
		t.Fatalf("first admission: %s", stderr)
	}
	commit(t, source, "theirs.go")
	moved := git(t, source, "rev-parse", "HEAD")

	code, _, stderr := a.run(t, filepath.Base(source))

	if code == 0 {
		t.Fatal("a handed-in clone at the wrong revision was accepted")
	}
	if !strings.Contains(stderr, "read and never moved") {
		t.Errorf("stderr = %q, want the refusal to say why", stderr)
	}
	if at := git(t, source, "rev-parse", "HEAD"); at != moved {
		t.Errorf("the handed-in clone is at %q, want it left where its owner put it (%q)", at, moved)
	}
	if at := head; at == moved {
		t.Fatal("the test did not move the clone")
	}
}

// A scan that indexed nothing writes an artifact that says so and does not
// advance the repository. Recording it would put a repository every later phase
// reads as dark into the catalog.
func TestAScanThatIndexedNothingWritesTheFindingAndAdmitsNothing(t *testing.T) {
	source, _ := sourceRepo(t)
	a := newAdmission(t, `{"index":{"files":0,"symbols":0},"languages":{},"version":{"binary":"1.14.1-test"}}`)

	code, stdout, _ := a.run(t, source)

	if code == 0 {
		t.Fatal("an unindexed repository was admitted")
	}
	var i struct{ Shortfall string }
	readJSON(t, a.artifact(filepath.Base(source)), &i)
	if !strings.Contains(i.Shortfall, "0 symbols") {
		t.Errorf("artifact shortfall = %q, want it to say what was indexed", i.Shortfall)
	}
	if !strings.Contains(stdout, "0 symbols") {
		t.Errorf("stdout = %q, want the finding printed as well as recorded", stdout)
	}
	if _, err := os.Stat(a.repoFile(filepath.Base(source))); !os.IsNotExist(err) {
		t.Error("a repository file was written for a repository with no index")
	}
}

// An unreachable repository is the most common first run for anybody but the
// author. Git's message is passed through, and nothing is left behind.
func TestAnUnreachableRepositoryLeavesNoRepositoryFile(t *testing.T) {
	a := newAdmission(t, admissionStatus)

	code, stdout, stderr := a.run(t, "file:///nonexistent/nowhere.git")

	if code == 0 {
		t.Fatal("an unreachable repository was admitted")
	}
	if !strings.Contains(stderr, "/nonexistent/nowhere.git") {
		t.Errorf("stderr = %q, want the resolved url beside git's message", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing announced for a repository that could not be read", stdout)
	}
	if _, err := os.Stat(a.repoFile("nowhere")); !os.IsNotExist(err) {
		t.Error("a repository file was written for a repository that was never cloned")
	}
}

func TestAdmissionRefusesWhatItCannotResolve(t *testing.T) {
	a := newAdmission(t, admissionStatus)

	code, _, stderr := a.run(t, "jellyfin")

	if code == 0 {
		t.Fatal("a bare word was admitted as a repository")
	}
	if !strings.Contains(stderr, "not an admitted id") {
		t.Errorf("stderr = %q, want it to say what it tried", stderr)
	}
}

func TestAdmissionNeedsOneNameAndACampaign(t *testing.T) {
	a := newAdmission(t, admissionStatus)

	for _, tc := range []struct {
		name string
		args []string
	}{
		{"no repository named", []string{"repo", "-config", a.config, "-campaign", a.campaign}},
		{"two repositories named", []string{"repo", "-config", a.config, "-campaign", a.campaign, "a", "b"}},
		{"no campaign", []string{"repo", "-config", a.config, "owner/name"}},
		{"an unknown flag", []string{"repo", "-nope"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if code, _, _ := dispatch(t, tc.args...); code != 2 {
				t.Errorf("exit = %d, want a usage error", code)
			}
		})
	}
}

func TestAdmissionCannotRunWithoutAConfigDirectory(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "no-such-config")

	code, _, stderr := dispatch(t, "repo", "-config", absent, "-campaign", t.TempDir(), "owner/name")

	if code == 0 || !strings.Contains(stderr, "no config directory") {
		t.Errorf("exit = %d, stderr = %q; want it to name the missing config", code, stderr)
	}
}

func readJSON(t *testing.T, path string, into any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

// A clone with no origin is refused before a scan is spent on it. Recording it
// would leave a repository nothing can clone again, in a config directory that
// then fails to load until somebody edits it by hand.
func TestACloneWithNoOriginIsRefusedRatherThanRecorded(t *testing.T) {
	source := t.TempDir()
	git(t, source, "init", "-q")
	git(t, source, "-c", "user.email=lab@example.test", "-c", "user.name=lab", "commit", "-q", "--allow-empty", "-m", "first")
	a := newAdmission(t, admissionStatus)

	code, _, stderr := a.run(t, source)

	if code == 0 {
		t.Fatal("a clone with no origin was admitted")
	}
	if !strings.Contains(stderr, "no origin") {
		t.Errorf("stderr = %q, want it to say what is missing", stderr)
	}
	if _, err := os.Stat(a.artifact(filepath.Base(source))); !os.IsNotExist(err) {
		t.Error("the repository was scanned anyway")
	}
}

// A repository admitted before it was indexed reads as admitted and not
// indexed. The two facts are separate on disk, so the position says both.
func TestPositionReportsARepositoryThatWasNeverIndexed(t *testing.T) {
	source, head := sourceRepo(t)
	a := newAdmission(t, admissionStatus)
	if err := os.MkdirAll(filepath.Dir(a.repoFile("thing")), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.repoFile("thing"),
		[]byte(`{"id":"thing","url":"file://`+source+`","commit":"`+head+`","checkout":"`+source+`","languages":["go"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, stderr := a.run(t, "thing")

	if code != 0 {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "indexed:  no") || !strings.Contains(stdout, "awaiting: index") {
		t.Errorf("stdout = %q, want it to say the repository is waiting on its scan", stdout)
	}
}

// A clone that is already at its pin is adopted rather than replaced, and the
// line says so: re-cloning a repository the lab already has is minutes of
// network for no change.
func TestALabOwnedCloneAtItsPinIsAdopted(t *testing.T) {
	source, _ := sourceRepo(t)
	a := newAdmission(t, admissionStatus)
	if code, _, stderr := a.run(t, "file://"+source); code != 0 {
		t.Fatalf("first admission: %s", stderr)
	}

	before := read(t, a.artifact(filepath.Base(source)))

	code, stdout, stderr := a.run(t, filepath.Base(source))

	if code != 0 {
		t.Fatalf("exit = %d: %s", code, stderr)
	}
	if !strings.Contains(stdout, "already at this revision") {
		t.Errorf("stdout = %q, want it to say the clone was adopted", stdout)
	}
	if read(t, a.artifact(filepath.Base(source))) != before {
		t.Error("the repository was re-cloned and re-indexed; adoption is what stops minutes of network for no change")
	}
}

// A scan that could not run at all is a failure of this phase and is recorded
// as one: nothing is written, and the repository is not admitted.
func TestAScanThatCouldNotRunAdmitsNothing(t *testing.T) {
	source, _ := sourceRepo(t)
	a := newAdmission(t, admissionStatus)
	a.sense = filepath.Join(t.TempDir(), "no-such-sense")

	code, _, stderr := a.run(t, source)

	if code == 0 {
		t.Fatal("a repository nothing could index was admitted")
	}
	if !strings.Contains(stderr, "index") {
		t.Errorf("stderr = %q, want it to name the phase that failed", stderr)
	}
	if _, err := os.Stat(a.repoFile(filepath.Base(source))); !os.IsNotExist(err) {
		t.Error("a repository file was written for a repository that was never indexed")
	}
}

// An artifact that cannot be written is a failure rather than a silent
// admission: the index phase owes an artifact, and a repository admitted
// without one is a repository whose index nobody can read.
func TestAnArtifactThatCannotBeWrittenAdmitsNothing(t *testing.T) {
	source, _ := sourceRepo(t)
	a := newAdmission(t, admissionStatus)
	blocked(t, a.artifact(filepath.Base(source)))

	code, _, stderr := a.run(t, source)

	if code == 0 {
		t.Fatal("a repository whose artifact could not be written was admitted")
	}
	if stderr == "" {
		t.Error("nothing was reported")
	}
	if _, err := os.Stat(a.repoFile(filepath.Base(source))); !os.IsNotExist(err) {
		t.Error("a repository file was written without its index artifact")
	}
}

// And the same the other way: a repository file that cannot be written is
// reported rather than swallowed, because the next command reads the catalog.
func TestARepositoryFileThatCannotBeWrittenIsReported(t *testing.T) {
	source, _ := sourceRepo(t)
	a := newAdmission(t, admissionStatus)
	repos := filepath.Join(a.config, "repos")
	if err := os.Chmod(repos, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(repos, 0o755) })

	code, _, stderr := a.run(t, source)

	if code == 0 {
		t.Fatal("a repository nothing recorded was reported as admitted")
	}
	if !strings.Contains(stderr, "repos") {
		t.Errorf("stderr = %q, want it to name what could not be written", stderr)
	}
	// The index artifact stands, so whoever fixes the directory re-runs and the
	// scan is not spent twice.
	if _, err := os.Stat(a.artifact(filepath.Base(source))); err != nil {
		t.Errorf("the index artifact was lost with the repository file: %v", err)
	}
}

// blocked puts a file where a directory has to go, which is the simplest thing
// a write cannot get past.
func blocked(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Dir(path)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Dir(path), []byte("in the way"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The exit codes are this command's API, because it ends up in a shell loop:
//
//	while sense-lab repo <id>; do :; done
//
// Every code is asserted here, and every one that must stop that loop is
// asserted to be non-zero in the same table — which is the property the loop
// actually rests on.
func TestTheExitCodeCarriesTheStanding(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(t *testing.T, repoDir string)
		show  bool
		want  int
	}{
		{"a repository with a phase to run", func(*testing.T, string) {}, false, 0},
		{"on the board", func(t *testing.T, dir string) {
			artifact(t, filepath.Join(dir, "1", "board", "board.md"), "# board\n")
		}, false, 3},
		{"parked at the ceiling", func(t *testing.T, dir string) {
			artifact(t, filepath.Join(dir, "6", "handoff", "handoff.md"), "# handoff\n")
		}, false, 4},
		{"waiting at a PAY", func(t *testing.T, dir string) {
			artifact(t, filepath.Join(dir, "1", "validate", "pay-call.md"), "PAY\n")
			attempt(t, dir, `{"cycle":1,"phase":"validate","verdict":"PAY","try":1}`)
		}, false, 5},
		{"refused: a verdict the phase cannot emit", func(t *testing.T, dir string) {
			artifact(t, filepath.Join(dir, "1", "author", "scenario.draft.yaml"), "name: r\n")
			attempt(t, dir, `{"cycle":1,"phase":"author","verdict":"WIN","try":1}`)
		}, false, 6},
		{"refused: the artifact the verdict claims is not there", func(t *testing.T, dir string) {
			artifact(t, filepath.Join(dir, "1", "minibench", "minibench.md"), "# read\n")
			attempt(t, dir, `{"cycle":1,"phase":"author","verdict":"DRAFT","try":1}`)
		}, false, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, id := admitted(t)
			tc.build(t, filepath.Join(a.campaign, id))

			args := []string{"repo", "-config", a.config, "-campaign", a.campaign,
				"-checkouts", a.checkouts, "-sense", a.sense}
			if tc.show {
				args = append(args, "-show")
			}
			code, stdout, stderr := dispatch(t, append(args, id)...)

			if code != tc.want {
				t.Errorf("exit = %d, want %d\n%s%s", code, tc.want, stdout, stderr)
			}
			if !strings.Contains(stdout, "standing:") {
				t.Errorf("stdout = %q, want the position printed", stdout)
			}
		})
	}
}

// -show is a preview of the admission, so it stops before the first thing an
// admission would write. Without that it is a second reading of the inputs
// rather than a look at what is about to happen.
func TestShowWritesNothing(t *testing.T) {
	source, _ := sourceRepo(t)
	a := newAdmission(t, admissionStatus)

	code, stdout, stderr := dispatch(t, "repo", "-config", a.config, "-campaign", a.campaign,
		"-checkouts", a.checkouts, "-sense", a.sense, "-show", source)

	if code != 2 {
		t.Fatalf("exit = %d, want 2: %s", code, stderr)
	}
	if !strings.Contains(stdout, "id:       "+filepath.Base(source)) {
		t.Errorf("stdout = %q, want the resolution it was about to act on", stdout)
	}
	if _, err := os.Stat(a.repoFile(filepath.Base(source))); !os.IsNotExist(err) {
		t.Error("a repository was admitted by a command asked only to show")
	}
	if _, err := os.Stat(a.artifact(filepath.Base(source))); !os.IsNotExist(err) {
		t.Error("a repository was indexed by a command asked only to show")
	}
}

// admitted is a repository already in the catalog, with a real checkout at its
// pin, so a test can put a run tree under it and ask where it stands.
func admitted(t *testing.T) (admission, string) {
	t.Helper()
	source, head := sourceRepo(t)
	a := newAdmission(t, admissionStatus)
	id := filepath.Base(source)
	artifact(t, a.repoFile(id), `{"id":"`+id+`","url":"https://example.test/`+id+`.git","commit":"`+head+
		`","checkout":"`+source+`","languages":["go"]}`)
	artifact(t, a.artifact(id), `{"repo":"`+id+`","symbols":40}`)
	return a, id
}

func artifact(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func attempt(t *testing.T, repoDir, body string) {
	t.Helper()
	var a struct {
		Cycle int    `json:"cycle"`
		Phase string `json:"phase"`
		Try   int    `json:"try"`
	}
	if err := json.Unmarshal([]byte(body), &a); err != nil {
		t.Fatal(err)
	}
	artifact(t, filepath.Join(repoDir, "attempts", fmt.Sprintf("%d-%s-%d.json", a.Cycle, a.Phase, a.Try)), body)
}

// A position that cannot be read is reported rather than answered with a
// default one, which would be a repository at the start of its first cycle.
func TestAPositionThatCannotBeReadIsReported(t *testing.T) {
	a, id := admitted(t)
	if err := os.RemoveAll(a.campaign); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(a.campaign, []byte("in the way"), 0o644); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := dispatch(t, "repo", "-config", a.config, "-campaign", a.campaign,
		"-checkouts", a.checkouts, "-sense", a.sense, id)

	if code != 1 {
		t.Errorf("exit = %d, want the error code", code)
	}
	if !strings.Contains(stdout, "unreadable") {
		t.Errorf("stdout = %q, want it to say the position could not be read", stdout)
	}
}
