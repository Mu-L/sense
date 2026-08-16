package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// dispatch is a test helper: it calls Run with fresh buffers and returns the code
// plus what landed on each stream.
func dispatch(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestVersionPrintsTheBuildStampToStdout(t *testing.T) {
	old := Version
	t.Cleanup(func() { Version = old })
	Version = "1.2.3-dev+gdeadbee"

	code, stdout, stderr := dispatch(t, "version")

	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "1.2.3-dev+gdeadbee" {
		t.Errorf("stdout = %q, want the version stamp", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

// A usage error must be distinguishable from success by exit code alone, and
// must not pollute stdout: a caller piping `sense-lab version` into another
// command would otherwise consume the usage text as data.
func TestUnknownCommandIsAUsageErrorOnStderr(t *testing.T) {
	code, stdout, stderr := dispatch(t, "measure")

	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "unknown command: measure") {
		t.Errorf("stderr = %q, want it to name the rejected command", stderr)
	}
	if !strings.Contains(stderr, "Usage: sense-lab") {
		t.Errorf("stderr = %q, want the usage text", stderr)
	}
}

func TestNoArgsIsAUsageErrorOnStderr(t *testing.T) {
	code, stdout, stderr := dispatch(t)

	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Usage: sense-lab") {
		t.Errorf("stderr = %q, want the usage text", stderr)
	}
}

// Asking for help is not an error: it succeeds and prints to stdout, so
// `sense-lab --help | less` works.
func TestHelpSucceedsOnStdout(t *testing.T) {
	for _, arg := range []string{"help", "-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			code, stdout, stderr := dispatch(t, arg)

			if code != 0 {
				t.Errorf("exit = %d, want 0", code)
			}
			if !strings.Contains(stdout, "Usage: sense-lab") {
				t.Errorf("stdout = %q, want the usage text", stdout)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want empty", stderr)
			}
		})
	}
}

// Arguments after the command are the subcommand's business, not the
// dispatcher's: an extra argument must not turn a known command into a usage
// error.
func TestTrailingArgumentsDoNotBreakDispatch(t *testing.T) {
	code, _, stderr := dispatch(t, "version", "--json")

	if code != 0 {
		t.Errorf("exit = %d, want 0 (stderr: %q)", code, stderr)
	}
}

// The build stamps Version through a symbol path written out in full in the
// Makefile, and nothing connects that string to this package. Rename or move
// the package and every test here still passes while the shipped binary
// silently reports 0.0.0-dev, because -X naming a symbol that does not exist is
// ignored rather than an error. This fences that path to this file.
func TestTheMakefileStampsTheVersionSymbolThisPackageDeclares(t *testing.T) {
	makefile, err := os.ReadFile(filepath.Join(repoRoot(t), "Makefile"))
	if err != nil {
		t.Fatalf("read Makefile: %v", err)
	}

	m := regexp.MustCompile(`LAB_LDFLAGS[^\n]*-X '([^=]+)=`).FindSubmatch(makefile)
	if m == nil {
		t.Fatal("LAB_LDFLAGS in the Makefile no longer stamps a -X symbol; the binary reports no version")
	}

	want := thisPackagePath(t) + ".Version"
	if got := string(m[1]); got != want {
		t.Errorf("Makefile stamps %q, but this package is %q", got, want)
	}
}

// thisPackagePath derives the import path of the package under test from where
// its own source sits, so moving or renaming the package moves the expectation
// with it instead of leaving a stale string literal.
//
// It reads a real path out of runtime.Caller, which holds because no test
// target passes -trimpath. If one ever does, this fails loudly and the failure
// will read as a version bug when it is a build-flag change.
func thisPackagePath(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not locate this test file")
	}
	root := repoRoot(t)

	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	mod := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(gomod)
	if mod == nil {
		t.Fatal("go.mod has no module line")
	}

	rel, err := filepath.Rel(root, filepath.Dir(self))
	if err != nil {
		t.Fatalf("locate package relative to %s: %v", root, err)
	}
	return string(mod[1]) + "/" + filepath.ToSlash(rel)
}
