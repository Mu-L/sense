package subject_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/subject"
)

// A subject that is not Sense arrives with an installer and nobody knows what
// it touches in advance. These tests are about the observation, not about any
// particular subject: the stand-ins write files the way an installer does.

func anEnv(t *testing.T) subject.Env {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the stand-in subjects are shell commands")
	}
	root := t.TempDir()
	repo, home := filepath.Join(root, "repo"), filepath.Join(root, "home")
	for _, dir := range []string{repo, home} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return subject.Env{Repo: repo, Home: home, Environ: []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}}
}

// The discovery run. A subject's first run has nothing to check against, so
// what it produces IS the declaration — written from what happened rather than
// from what the subject's documentation claims it does.
func TestAFirstRunReportsEveryPathTheSubjectActuallyWrote(t *testing.T) {
	env := anEnv(t)
	plan := subject.Plan{
		Install: [][]string{{"sh", "-c", "mkdir -p " + env.Home + "/bin && echo binary > " + env.Home + "/bin/tool"}},
		Setup:   [][]string{{"sh", "-c", "echo config > " + env.Home + "/.toolrc; echo snapshot > snapshot.json"}},
		Cleanup: [][]string{{"sh", "-c", "rm -f " + env.Home + "/bin/tool " + env.Home + "/.toolrc snapshot.json"}},
	}

	wrote, err := subject.Prepare(context.Background(), plan, env)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	// Both trees, because the two failures are different: a subject that writes
	// into the repository changes what the next arm sees, and one that writes
	// into HOME changes what every later run on that machine sees.
	for _, want := range []string{"home/bin/tool", "home/.toolrc", "repo/snapshot.json"} {
		if !slices.Contains(wrote, want) {
			t.Errorf("the discovery run missed %s; it reported %v", want, wrote)
		}
	}
}

// Cleanup is proven, not trusted. An uninstaller that leaves a config entry
// behind contaminates every subsequent run on that machine, and the symptom
// looks like drift rather than like a leak.
func TestASubjectThatDoesNotCleanUpCompletelyIsCaught(t *testing.T) {
	env := anEnv(t)
	plan := subject.Plan{
		Install: [][]string{{"sh", "-c", "mkdir -p " + env.Home + "/bin && echo binary > " + env.Home + "/bin/tool"}},
		Setup:   [][]string{{"sh", "-c", "echo config > " + env.Home + "/.toolrc"}},
		// The uninstaller its author wrote: it takes the binary and forgets the
		// config it left beside it.
		Cleanup: [][]string{{"sh", "-c", "rm -f " + env.Home + "/bin/tool"}},
	}

	wrote, err := subject.Prepare(context.Background(), plan, env)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	left, err := subject.Remove(context.Background(), plan, env, wrote)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}

	if !slices.Contains(left, "home/.toolrc") {
		t.Errorf("a config left behind by the uninstaller was not caught; survivors: %v", left)
	}
	if slices.Contains(left, "home/bin/tool") {
		t.Error("a file the cleanup did remove was reported as surviving it")
	}
}

// A clean exit is not a clean machine. This is the case that must not pass by
// accident: the subject removes everything, and the check says so.
func TestASubjectThatCleansUpCompletelyLeavesNothing(t *testing.T) {
	env := anEnv(t)
	plan := subject.Plan{
		Install: [][]string{{"sh", "-c", "mkdir -p " + env.Home + "/bin && echo binary > " + env.Home + "/bin/tool"}},
		Cleanup: [][]string{{"sh", "-c", "rm -f " + env.Home + "/bin/tool"}},
	}

	wrote, err := subject.Prepare(context.Background(), plan, env)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	left, err := subject.Remove(context.Background(), plan, env, wrote)
	if err != nil {
		t.Fatalf("remove: %v", err)
	}

	if len(left) != 0 {
		t.Errorf("a subject that removed everything was reported as leaving %v", left)
	}
}

// A subject that writes outside its declaration is a finding about that
// subject, and it belongs in that subject's README with a date and a version.
func TestWhatASubjectWroteOutsideItsDeclarationIsNamed(t *testing.T) {
	wrote := []string{"home/bin/tool", "home/.config/tool/state.db", "repo/snapshot.json", "home/.hidden-cache"}

	// A declared directory covers what is under it: a subject that says it
	// writes a directory has said what the directory is for.
	got := subject.Undeclared(wrote, []string{"home/bin", "home/.config", "repo/snapshot.json"})

	if len(got) != 1 || got[0] != "home/.hidden-cache" {
		t.Errorf("undeclared = %v, want only the path no declaration covers", got)
	}
}

// A declaration that names a prefix of a path must not cover it: `home/bin`
// covers `home/bin/tool`, and it does not cover `home/binary-cache`.
func TestADeclarationCoversADirectoryAndNotANameThatMerelyStartsTheSame(t *testing.T) {
	got := subject.Undeclared([]string{"home/binary-cache"}, []string{"home/bin"})

	if len(got) != 1 {
		t.Errorf("undeclared = %v, want the path the declaration does not actually cover", got)
	}
}

// A command that cannot run has to stop the subject rather than be skipped: a
// half-installed competitor that then gets benched is a measurement of nothing.
func TestAStageThatFailsStopsTheSubject(t *testing.T) {
	env := anEnv(t)
	plan := subject.Plan{Install: [][]string{{"sh", "-c", "exit 3"}}}

	if _, err := subject.Prepare(context.Background(), plan, env); err == nil {
		t.Fatal("a subject whose install failed was reported as prepared")
	} else if !strings.Contains(err.Error(), "install step 1") {
		t.Errorf("error = %q, want it to name the stage and the step", err)
	}
}

func TestAnEmptyCommandIsRefusedRatherThanRun(t *testing.T) {
	env := anEnv(t)
	plan := subject.Plan{Setup: [][]string{{}}}

	if _, err := subject.Prepare(context.Background(), plan, env); err == nil {
		t.Fatal("an empty command was accepted")
	}
}

// The credential rule, made structural. A competitor gets its repository and
// whatever its own setup needs; it does not get the host's tokens or the host's
// agent configuration, and there is no field here through which they could
// arrive.
func TestASubjectRunsWithTheArmsEnvironmentAndNothingElse(t *testing.T) {
	env := anEnv(t)
	t.Setenv("A_HOST_SECRET", "sk-do-not-leak")
	plan := subject.Plan{
		Setup:   [][]string{{"sh", "-c", "env > " + env.Home + "/seen-env"}},
		Cleanup: [][]string{{"sh", "-c", "rm -f " + env.Home + "/seen-env"}},
	}

	if _, err := subject.Prepare(context.Background(), plan, env); err != nil {
		t.Fatalf("prepare: %v", err)
	}

	b, err := os.ReadFile(filepath.Join(env.Home, "seen-env"))
	if err != nil {
		t.Fatalf("read what the subject saw: %v", err)
	}
	if strings.Contains(string(b), "sk-do-not-leak") {
		t.Errorf("a host secret reached the subject:\n%s", b)
	}
}

// A tree that cannot be read is a check that cannot be made, and an unmade
// check must not read as a clean one: a subject reported as touching nothing
// because nothing could be looked at is the worst possible pass.
func TestATreeThatCannotBeReadFailsRatherThanReportingNothing(t *testing.T) {
	env := anEnv(t)
	gone := subject.Env{Repo: env.Repo, Home: filepath.Join(env.Home, "not-there"), Environ: env.Environ}

	if _, err := subject.Prepare(context.Background(), subject.Plan{}, gone); err == nil {
		t.Fatal("a subject whose HOME could not be read was reported as touching nothing")
	}
}

// The same on the way out. An over-eager uninstaller that removes the directory
// the check reads leaves nothing to check, and that must fail rather than pass.
func TestACleanupThatRemovesWhatTheCheckReadsFailsTheCheck(t *testing.T) {
	env := anEnv(t)
	plan := subject.Plan{
		Install: [][]string{{"sh", "-c", "echo binary > " + env.Home + "/tool"}},
		Cleanup: [][]string{{"sh", "-c", "rm -rf " + env.Home}},
	}
	wrote, err := subject.Prepare(context.Background(), plan, env)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if _, err := subject.Remove(context.Background(), plan, env, wrote); err == nil {
		t.Fatal("a cleanup that removed the directory under inspection was reported as clean")
	}
}

// A subject with no HOME of its own is checked on its repository alone, rather
// than being checked against the whole of whatever HOME happens to be.
func TestASubjectWithNoHomeIsCheckedOnItsRepositoryAlone(t *testing.T) {
	env := anEnv(t)
	only := subject.Env{Repo: env.Repo, Environ: env.Environ}
	plan := subject.Plan{Setup: [][]string{{"sh", "-c", "echo snapshot > snapshot.json"}}}

	wrote, err := subject.Prepare(context.Background(), plan, only)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}

	if len(wrote) != 1 || wrote[0] != "repo/snapshot.json" {
		t.Errorf("wrote = %v, want only what appeared in the repository", wrote)
	}
}
