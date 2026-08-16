package isolate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/isolate"
)

// hostBin is a directory on the host PATH holding the given executables.
func hostBin(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\necho "+name+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func hostPath(dirs ...string) string {
	return strings.Join(dirs, string(filepath.ListSeparator))
}

// runs the named command out of a bin directory and returns what it said.
func runs(t *testing.T, bin, name string) (string, bool) {
	t.Helper()
	path := filepath.Join(bin, name)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	out, err := exec.Command(path).CombinedOutput()
	if err != nil {
		t.Fatalf("run %s: %v", path, err)
	}
	return strings.TrimSpace(string(out)), true
}

func TestTheBaselineArmCannotReachASenseTheHostAlreadyHad(t *testing.T) {
	// The failure the channel proof caught. Removing the directory that holds
	// the Sense binary is not enough: on a machine where Sense is installed the
	// way a user installs it, the baseline arm still finds it through a
	// directory the bench never configured.
	installed := hostBin(t, "sense", "git", "rg")
	senseBin := filepath.Join(hostBin(t, "sense"), "sense")

	bin, err := isolate.ShadowBin(filepath.Join(t.TempDir(), "bin"), hostPath(installed), senseBin, isolate.Baseline)
	if err != nil {
		t.Fatalf("ShadowBin: %v", err)
	}

	if _, ok := runs(t, bin, "sense"); ok {
		t.Error("the baseline arm reaches a sense binary")
	}
	// And dropping the directory would have taken these with it.
	for _, name := range []string{"git", "rg"} {
		if _, ok := runs(t, bin, name); !ok {
			t.Errorf("the baseline arm lost %s along with the sense binary", name)
		}
	}
}

func TestTheSenseArmReachesTheBuildUnderTestRatherThanTheHostsInstall(t *testing.T) {
	// A run against whatever happens to be installed measures a binary nobody
	// chose, and nothing in the result would say which one it was.
	installed := hostBin(t, "sense", "git")
	underTest := t.TempDir()
	if err := os.WriteFile(filepath.Join(underTest, "sense"), []byte("#!/bin/sh\necho the build under test\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	bin, err := isolate.ShadowBin(filepath.Join(t.TempDir(), "bin"), hostPath(installed),
		filepath.Join(underTest, "sense"), isolate.Sense)
	if err != nil {
		t.Fatalf("ShadowBin: %v", err)
	}

	said, ok := runs(t, bin, "sense")
	if !ok {
		t.Fatal("the sense arm cannot reach a sense binary at all")
	}
	if said != "the build under test" {
		t.Errorf("the sense arm reached %q, want the build under test", said)
	}
}

func TestBothArmsGetTheSameToolsApartFromSense(t *testing.T) {
	// If the arms differed in the shape of PATH as well as in Sense, they would
	// differ in two things and the measurement would be of neither.
	installed := hostBin(t, "sense", "git", "rg", "jq")
	senseBin := filepath.Join(t.TempDir(), "sense")
	if err := os.WriteFile(senseBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	var got [2][]string
	for i, arm := range []isolate.Arm{isolate.Sense, isolate.Baseline} {
		bin, err := isolate.ShadowBin(filepath.Join(t.TempDir(), "bin"), hostPath(installed), senseBin, arm)
		if err != nil {
			t.Fatalf("ShadowBin(%s): %v", arm, err)
		}
		entries, err := os.ReadDir(bin)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			if e.Name() != "sense" {
				got[i] = append(got[i], e.Name())
			}
		}
	}

	if strings.Join(got[0], ",") != strings.Join(got[1], ",") {
		t.Errorf("the sense arm has %v and the baseline has %v", got[0], got[1])
	}
	if len(got[0]) != 3 {
		t.Errorf("the arms have %v, want git, jq and rg", got[0])
	}
}

func TestTheHostsOwnPrecedenceSurvives(t *testing.T) {
	// A machine with two versions of a tool on PATH resolves the first. An arm
	// that resolved the second would be running something the operator does not
	// run.
	first := hostBin(t)
	second := hostBin(t)
	if err := os.WriteFile(filepath.Join(first, "rg"), []byte("#!/bin/sh\necho the first rg\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(second, "rg"), []byte("#!/bin/sh\necho the second rg\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	bin, err := isolate.ShadowBin(filepath.Join(t.TempDir(), "bin"), hostPath(first, second), "", isolate.Baseline)
	if err != nil {
		t.Fatalf("ShadowBin: %v", err)
	}

	if said, _ := runs(t, bin, "rg"); said != "the first rg" {
		t.Errorf("rg resolved to %q, want the first on the host PATH", said)
	}
}

func TestWhatCannotBeRunIsNotLinked(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "gone"), filepath.Join(dir, "dangling")); err != nil {
		t.Fatal(err)
	}

	bin, err := isolate.ShadowBin(filepath.Join(t.TempDir(), "bin"), hostPath(dir), "", isolate.Baseline)
	if err != nil {
		t.Fatalf("ShadowBin: %v", err)
	}

	entries, err := os.ReadDir(bin)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("linked %v, want nothing runnable", entries)
	}
}

func TestAPathEntryThatIsNotThereIsNotAFailure(t *testing.T) {
	// Every real machine has one. A run must not fail because of it.
	present := hostBin(t, "git")

	bin, err := isolate.ShadowBin(filepath.Join(t.TempDir(), "bin"),
		hostPath("/no/such/directory", present, ""), "", isolate.Baseline)
	if err != nil {
		t.Fatalf("ShadowBin: %v", err)
	}

	if _, ok := runs(t, bin, "git"); !ok {
		t.Error("a missing PATH entry took the rest of the PATH with it")
	}
}

func TestABinDirectoryThatCannotBeCreatedIsReported(t *testing.T) {
	blocked := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blocked, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := isolate.ShadowBin(filepath.Join(blocked, "bin"), "", "", isolate.Baseline); err == nil {
		t.Fatal("ShadowBin succeeded with an unusable directory")
	}
}

func TestALinkThatCannotBeMadeIsReported(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission that makes the link fail")
	}
	into := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(into, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(into, 0o700) })

	if _, err := isolate.ShadowBin(into, hostPath(hostBin(t, "git")), "", isolate.Baseline); err == nil {
		t.Fatal("ShadowBin reported success although it could link nothing")
	}
}

func TestTheSenseLinkIsReportedWhenItCannotBeMade(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the permission that makes the link fail")
	}
	into := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(into, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(into, 0o700) })

	// Nothing on the host PATH, so the only link attempted is the Sense one.
	if _, err := isolate.ShadowBin(into, "", filepath.Join(t.TempDir(), "sense"), isolate.Sense); err == nil {
		t.Fatal("ShadowBin reported success although the sense arm has no sense binary")
	}
}
