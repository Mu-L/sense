package executor_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/executor"
	"github.com/luuuc/sense/lab/internal/isolate"
)

func aSpec(t *testing.T) isolate.Spec {
	t.Helper()
	return isolate.Spec{Root: filepath.Join(t.TempDir(), "run"), Arm: isolate.Baseline, HostPath: "/usr/bin"}
}

// Both implementations satisfy the interface without special cases. If they had
// disagreed about what a method means, the interface would be the wrong shape
// and the disagreement would be the design input — this is where that is
// checked rather than assumed.
func TestBothExecutorsPrepareAndReleaseTheSameWay(t *testing.T) {
	for _, e := range []executor.Executor{
		executor.IsolatedHome{},
		executor.Container{Runtime: "docker", Image: "alpine:3"},
	} {
		t.Run(e.ID(), func(t *testing.T) {
			env, err := e.Prepare(context.Background(), aSpec(t))
			if err != nil {
				t.Fatalf("prepare: %v", err)
			}
			if env.Repo == "" || env.Home == "" {
				t.Fatal("a prepared environment has nowhere to run")
			}
			if err := e.Release(context.Background(), env); err != nil {
				t.Fatalf("release: %v", err)
			}
		})
	}
}

// The whole of the difference between the two, and the reason the interface has
// a third method at all.
func TestTheDefaultExecutorRunsTheArgvAsItIs(t *testing.T) {
	argv := []string{"claude", "-p", "--model", "opus"}

	got := executor.IsolatedHome{}.Command(isolate.Env{}, argv)

	if !slices.Equal(got, argv) {
		t.Errorf("command = %v, want the argv unchanged", got)
	}
}

// One mount, and it is the repository. Not HOME, where a credential would be;
// not the config directory, where an agent's login would be. A container that
// mounted either would be a container that received host credentials, which is
// the one thing this executor exists to prevent.
func TestTheContainerMountsTheRepositoryAndNothingElse(t *testing.T) {
	env := isolate.Env{Layout: isolate.Layout{
		Repo: "/runs/cell/sense/repo", Home: "/runs/cell/sense/home", Config: "/runs/cell/sense/config",
	}}
	c := executor.Container{Runtime: "docker", Image: "alpine:3", WorkDir: "/repo"}

	got := c.Command(env, []string{"rival", "--scan"})

	line := strings.Join(got, " ")
	if !strings.Contains(line, "-v /runs/cell/sense/repo:/repo") {
		t.Errorf("the repository is not mounted: %s", line)
	}
	for _, forbidden := range []string{env.Home, env.Config} {
		if strings.Contains(line, forbidden) {
			t.Errorf("%s was mounted into the container: %s", forbidden, line)
		}
	}
	if !strings.HasSuffix(line, "alpine:3 rival --scan") {
		t.Errorf("the arm's own command is not what runs inside: %s", line)
	}
	// No network, because a subject that can reach the internet from inside the
	// container is a subject whose run is not reproducible.
	if !strings.Contains(line, "--network none") {
		t.Errorf("the container can reach the network: %s", line)
	}
}

func TestAContainerWithNoWorkDirStillHasOne(t *testing.T) {
	got := executor.Container{Runtime: "docker", Image: "alpine:3"}.Command(
		isolate.Env{Layout: isolate.Layout{Repo: "/r"}}, []string{"x"})

	if !slices.Contains(got, "-w") {
		t.Errorf("no working directory inside the container: %v", got)
	}
}

// An executor a subject names and nothing implements is a catalog problem, and
// it reads differently from a run that failed.
func TestAnExecutorNothingImplementsIsRefusedByName(t *testing.T) {
	if _, err := executor.Of("kubernetes", executor.Container{}); err == nil {
		t.Fatal("an executor nothing implements resolved")
	} else if !strings.Contains(err.Error(), "kubernetes") {
		t.Errorf("error = %q, want it to name what was asked for", err)
	}
}

func TestEachDeclaredExecutorResolvesToItsImplementation(t *testing.T) {
	for _, id := range []string{executor.IsolatedHomeID, executor.ContainerID} {
		e, err := executor.Of(id, executor.Container{Runtime: "docker", Image: "alpine:3"})
		if err != nil {
			t.Fatalf("resolve %s: %v", id, err)
		}
		if e.ID() != id {
			t.Errorf("resolved %s to an executor calling itself %s", id, e.ID())
		}
	}
}

// A runtime that is installed but not running fails at spawn, which costs the
// arm and reads in a score exactly like a model with nothing to say. It is
// asked before a campaign instead.
func TestAContainerRuntimeThatCannotStartAnythingIsRefusedBeforeARunIsSpent(t *testing.T) {
	for _, tc := range []struct{ what, runtime string }{
		{"no runtime declared at all", ""},
		{"a runtime that is not on this machine", "no-such-container-runtime"},
	} {
		t.Run(tc.what, func(t *testing.T) {
			if err := (executor.Container{Runtime: tc.runtime}).Available(context.Background()); err == nil {
				t.Fatal("a runtime that cannot start a container was reported as ready")
			}
		})
	}
}

// And the positive case, on a machine that has one. Skipped rather than faked
// where there is none: a stand-in here would be a test of the stand-in.
func TestAWorkingContainerRuntimeIsReportedAsReady(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("no container runtime on this machine")
	}
	c := executor.Container{Runtime: "docker", Image: "alpine:3"}

	if err := c.Available(context.Background()); err != nil {
		t.Skipf("a container runtime is installed but not running: %v", err)
	}
}

// The argv the container executor builds has to actually run something, or
// this is a test of string formatting. Skipped where there is no runtime,
// because a stand-in container would be a test of the stand-in.
func TestTheContainerActuallyRunsTheArmsCommandAgainstTheRepository(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("no container runtime on this machine")
	}
	c := executor.Container{Runtime: "docker", Image: "alpine:3", WorkDir: "/repo"}
	if err := c.Available(context.Background()); err != nil {
		t.Skipf("a container runtime is installed but not running: %v", err)
	}
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "marker.txt"), []byte("the repository"), 0o600); err != nil {
		t.Fatal(err)
	}

	argv := c.Command(isolate.Env{Layout: isolate.Layout{Repo: repo, Home: "/host/home"}},
		[]string{"cat", "marker.txt"})
	out, err := exec.CommandContext(context.Background(), argv[0], argv[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("run inside the container: %v\n%s", err, out)
	}

	if !strings.Contains(string(out), "the repository") {
		t.Errorf("the command did not see the repository it was given:\n%s", out)
	}
}

// And the host's home is not in there. Asserted on what the container can
// actually see rather than on the argv, because the argv is what we wrote and
// the mount table is what happened.
func TestTheContainerCannotSeeTheHostsHome(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("no container runtime on this machine")
	}
	c := executor.Container{Runtime: "docker", Image: "alpine:3", WorkDir: "/repo"}
	if err := c.Available(context.Background()); err != nil {
		t.Skipf("a container runtime is installed but not running: %v", err)
	}
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".credentials.json"), []byte("a token"), 0o600); err != nil {
		t.Fatal(err)
	}

	argv := c.Command(isolate.Env{Layout: isolate.Layout{Repo: t.TempDir(), Home: home, Config: home}},
		[]string{"sh", "-c", "ls " + home + " 2>&1 || true"})
	out, err := exec.CommandContext(context.Background(), argv[0], argv[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("run inside the container: %v\n%s", err, out)
	}

	if strings.Contains(string(out), "a token") || strings.Contains(string(out), ".credentials.json") {
		t.Errorf("the host's home reached the container:\n%s", out)
	}
}

// A runtime that answers with nothing is the shape that matters: the command
// exists and exits zero, and there is still no daemon to start a container. An
// availability check that read that as ready would pass on exactly the machine
// where the arm is about to be burned.
func TestARuntimeThatAnswersWithNothingIsNotReady(t *testing.T) {
	c := executor.Container{Runtime: "true", Image: "alpine:3"}

	if err := c.Available(context.Background()); err == nil {
		t.Fatal("a runtime that reported no server version was read as ready")
	} else if !strings.Contains(err.Error(), "no server version") {
		t.Errorf("error = %q, want it to say nothing is running", err)
	}
}
