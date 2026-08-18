package isolate

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The rule that picks a cleanup shape, stated on its own. It is "does this
// directory hold evidence", NOT "did this run finish": keying off the run record
// deletes the directory of a run that crashed before writing one, and that
// directory holds the transcript of the crash.
func TestEvidenceRatherThanCompletionDecidesWhatIsKept(t *testing.T) {
	for _, tc := range []struct {
		what  string
		write map[string]string
		want  bool
	}{
		{"a run that produced nothing at all", nil, false},
		{"a run that wrote only its record", map[string]string{"artifacts/run-meta.json": "{}"}, true},
		{"a crashed run with a transcript and no record", map[string]string{"session/raw/stdout": "half an answer"}, true},
		{"a run with only a capture", map[string]string{"logs/session.log": "output"}, true},
		{"a tree of empty directories", map[string]string{"artifacts/": "", "logs/": ""}, false},
	} {
		t.Run(tc.what, func(t *testing.T) {
			root := t.TempDir()
			for rel, body := range tc.write {
				at := filepath.Join(root, rel)
				if strings.HasSuffix(rel, "/") {
					if err := os.MkdirAll(at, 0o755); err != nil {
						t.Fatal(err)
					}
					continue
				}
				if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(at, []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			if got := HoldsEvidence(root); got != tc.want {
				t.Errorf("HoldsEvidence = %v, want %v", got, tc.want)
			}
		})
	}
}

// A run directory outlives its purpose by months, so the token in it must not.
func TestReleaseTakesTheCredentialAndLeavesTheEvidence(t *testing.T) {
	root := filepath.Join(t.TempDir(), "run")
	c := aCredential()
	env, err := Prepare(Spec{Root: root, Arm: Sense, Credential: c, Route: aRoute()})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	transcript := filepath.Join(env.Root, "session", "raw", "stdout")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, []byte("what the arm said"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Release(env); err != nil {
		t.Fatalf("release: %v", err)
	}

	if _, err := os.Stat(aRoute().CredentialPath(env.Config)); err == nil {
		t.Error("the credential survived release, in a directory that is kept for months")
	}
	// The disposable HOME goes too, and for a reason no assertion about our own
	// stand-in could cover: it holds whatever the agent tool decided to write
	// about itself, and a tool that copies its credential into its own state
	// would put a token in a kept directory. Removing it makes the guarantee
	// independent of the tool's behaviour.
	if _, err := os.Stat(env.Home); err == nil {
		t.Error("the disposable HOME survived release; whatever the tool wrote there is kept for months")
	}
	b, err := os.ReadFile(transcript)
	if err != nil {
		t.Fatalf("release destroyed the evidence it exists to keep: %v", err)
	}
	if string(b) != "what the arm said" {
		t.Errorf("transcript = %q, want it untouched", b)
	}
}

func TestReleaseWithNoRunRootIsRefused(t *testing.T) {
	if err := Release(Env{}); err == nil {
		t.Fatal("release reported success with nothing to release")
	}
}

// The proof half, the same as the rest of cleanup: a removal that reported
// nothing is not a removal that happened.
func TestReleaseFailsWhileTheCredentialDirectoryRemains(t *testing.T) {
	root := filepath.Join(t.TempDir(), "run")
	env, err := Prepare(Spec{Root: root, Arm: Sense, Credential: aCredential(), Route: aRoute()})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	// A config directory that cannot be removed, so the removal reports success
	// on nothing and the proof is the only thing standing between the run and a
	// token kept for months.
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("a read-only parent does not stop removal here")
	}
	if err := os.Chmod(filepath.Dir(env.Config), 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(env.Config), 0o700) })

	if err := Release(env); err == nil {
		t.Fatal("release reported success although the credential is still there")
	}
}

// "I could not check" is not "it is gone". Cleaning up against something that is
// not the parent repository must be reported, or a caller pointed at the wrong
// place reads as clean and the real registration is still there.
func TestRemovingAWorktreeFromSomethingThatIsNotARepositoryIsReported(t *testing.T) {
	notARepository := t.TempDir()

	err := RemoveWorktree(context.Background(), notARepository, filepath.Join(notARepository, "repo"))
	if err == nil {
		t.Fatal("a removal against a parent that could not be listed reported success")
	}
}

// The rabbit hole the pitch names, at the one place it can still bite: a
// directory nobody can read must not be mistaken for an empty one.
//
// The costs are not symmetric. Reading "I could not look" as "nothing is there"
// deletes a run that was paid for and can never be recreated; reading it the
// other way keeps a directory somebody removes by hand.
func TestADirectoryThatCannotBeReadCountsAsHoldingEvidence(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("an unreadable directory does not stop a walk here")
	}
	root := t.TempDir()
	unreadable := filepath.Join(root, "session")
	if err := os.MkdirAll(unreadable, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unreadable, "stdout"), []byte("what the arm said"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(unreadable, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(unreadable, 0o700) })

	if !HoldsEvidence(root) {
		t.Fatal("a run directory nobody could read was reported as holding nothing; Finish would delete it")
	}
}
