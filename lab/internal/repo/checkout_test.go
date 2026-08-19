package repo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeGit answers git calls and records them, so a test can drive an outcome a
// real repository cannot be made to produce — a remote whose head moves between
// the question and the clone, most of all.
//
// Two things it deliberately cannot tell you, both covered by the real-git
// tests at the end of this file and in the command's own tests. It keys answers
// by VERB and ignores the directory, so nothing here would notice git being run
// in the wrong place. And an unanswered verb comes back empty rather than
// failing, so a test that forgets to state a revision is testing a repository
// pinned at nothing.
type fakeGit struct {
	// answer is keyed by the git verb.
	answer map[string]string
	// stuck says a checkout does not move the tree, which is how a correction
	// that reported success and silently did not take is expressed.
	stuck bool
	fail  map[string]error
	calls [][]string
}

func (f *fakeGit) run(_ context.Context, dir string, args ...string) ([]byte, error) {
	f.calls = append(f.calls, append([]string{dir}, args...))
	verb := args[0]
	if err, ok := f.fail[verb]; ok {
		return nil, err
	}
	if verb == "checkout" && !f.stuck {
		// A checkout moves the tree, so what the next rev-parse reads back
		// moves with it. Without this the fake would let a correction that
		// went nowhere pass for one that worked.
		f.answer["rev-parse"] = args[len(args)-1]
	}
	return []byte(f.answer[verb] + "\n"), nil
}

// ran reports whether a verb was called at all.
func (f *fakeGit) ran(verb string) bool {
	for _, c := range f.calls {
		if c[1] == verb {
			return true
		}
	}
	return false
}

// wrote reports whether anything that changes a checkout was called. It is the
// question every "nothing was touched" claim in this file rests on.
func (f *fakeGit) wrote() bool {
	return f.ran("clone") || f.ran("checkout") || f.ran("fetch")
}

// The ordering is the whole reason the announcement exists: a caller that stops
// at the announcement stops with the disk as it found it. Without that, printing
// the resolution proves nothing, because a wrong repository would already be
// cloned by the time anybody read the line.
func TestNothingIsWrittenWhenTheAnnouncementIsRefused(t *testing.T) {
	root := t.TempDir()
	git := &fakeGit{answer: map[string]string{"ls-remote": "abc123\tHEAD"}}
	stop := errors.New("the operator stopped here")

	_, err := prepare(context.Background(), Target{ID: "thing", URL: "https://example.test/thing.git"},
		root, func(Plan) error { return stop }, git.run)

	if !errors.Is(err, stop) {
		t.Fatalf("err = %v, want the announcement's own error", err)
	}
	if git.wrote() {
		t.Errorf("git calls = %v, want nothing that changes a checkout", git.calls)
	}
	if _, err := os.Stat(filepath.Join(root, "thing")); !os.IsNotExist(err) {
		t.Error("the checkout was made anyway; the announcement is not a stopping point")
	}
}

// A revision that was asked for is a request; the one the checkout sits at is
// the fact. They can differ — the default branch moves between the question and
// the clone — and recording the request would pin every later worktree to a
// tree this clone is not at.
func TestThePinIsReadOutOfTheCloneRatherThanOutOfTheRequest(t *testing.T) {
	git := &fakeGit{answer: map[string]string{
		"ls-remote": "aaaaaaaaaaaa\tHEAD",
		"rev-parse": "bbbbbbbbbbbb",
	}}
	var announced Plan

	p, err := prepare(context.Background(), Target{ID: "thing", URL: "https://example.test/thing.git"},
		t.TempDir(), func(p Plan) error { announced = p; return nil }, git.run)
	if err != nil {
		t.Fatal(err)
	}

	if announced.Revision != "aaaaaaaaaaaa" {
		t.Errorf("announced revision = %q, want what the remote said when it was asked", announced.Revision)
	}
	if p.Revision != "bbbbbbbbbbbb" {
		t.Errorf("recorded pin = %q, want the revision the clone is at", p.Revision)
	}
}

// The lab fixes what it made. An unattended crank must not park a repository on
// a stale tree nobody is watching, so a clone under the lab's own root goes back
// to its pin.
func TestALabOwnedCloneAtTheWrongRevisionIsCorrected(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "thing"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := &fakeGit{answer: map[string]string{"rev-parse": "drifted", "remote": "https://example.test/thing.git"}}

	p, err := prepare(context.Background(), Target{ID: "thing", URL: "https://example.test/thing.git", Pin: "pinned"},
		root, func(Plan) error { return nil }, git.run)
	if err != nil {
		t.Fatal(err)
	}

	if !git.ran("checkout") {
		t.Fatalf("git calls = %v, want the clone moved back to its pin", git.calls)
	}
	if p.Revision != "pinned" {
		t.Errorf("pin = %q, want the revision the corrected clone reads back as", p.Revision)
	}
	for _, c := range git.calls {
		if c[1] == "checkout" && c[len(c)-1] != "pinned" {
			t.Errorf("checkout %v, want it to name the pin", c)
		}
	}
}

// And the lab reads what it was given. A clone somebody handed in is their
// working tree: moving it would be the lab performing a checkout in a directory
// it does not own, which is a data-loss shape rather than a correction.
func TestAHandedInCloneAtTheWrongRevisionIsRefusedWithItsReason(t *testing.T) {
	handed := t.TempDir()
	git := &fakeGit{answer: map[string]string{"rev-parse": "aaaaaaaaaaaabbbb", "remote": "https://example.test/thing.git"}}

	_, err := prepare(context.Background(), Target{ID: "thing", Checkout: handed, Pin: "ccccccccccccdddd"},
		t.TempDir(), func(Plan) error { return nil }, git.run)

	if err == nil {
		t.Fatal("a handed-in clone at the wrong revision was accepted")
	}
	for _, want := range []string{handed, "aaaaaaaaaaaa", "cccccccccccc"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to name %q", err, want)
		}
	}
	if git.wrote() {
		t.Errorf("git calls = %v, want nothing written to somebody else's clone", git.calls)
	}
}

// The same clone at the pin is read and left alone. A repository that is
// already right is adopted rather than re-cloned.
func TestACloneAlreadyAtItsRevisionIsAdoptedRatherThanReplaced(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "thing"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := &fakeGit{answer: map[string]string{"rev-parse": "settled"}}

	p, err := prepare(context.Background(), Target{ID: "thing", URL: "u", Pin: "settled"},
		root, func(Plan) error { return nil }, git.run)
	if err != nil {
		t.Fatal(err)
	}

	if git.wrote() {
		t.Errorf("git calls = %v, want a clone that is already right left alone", git.calls)
	}
	if p.Revision != "settled" {
		t.Errorf("pin = %q, want the revision it was already at", p.Revision)
	}
}

// A handed-in clone knows where it came from, and reading it is the only way a
// url gets recorded for one.
func TestTheUrlOfAHandedInCloneIsReadOutOfIt(t *testing.T) {
	handed := t.TempDir()
	git := &fakeGit{answer: map[string]string{"rev-parse": "here", "remote": "git@example.test:team/thing.git"}}

	p, err := prepare(context.Background(), Target{ID: "thing", Checkout: handed},
		t.TempDir(), func(Plan) error { return nil }, git.run)
	if err != nil {
		t.Fatal(err)
	}

	if p.URL != "git@example.test:team/thing.git" {
		t.Errorf("URL = %q, want the clone's own origin", p.URL)
	}
	if p.Owned {
		t.Error("Owned = true for a handed-in clone; the lab would then feel free to move it")
	}
}

// A clone with no origin keeps an empty url rather than an invented one. The
// catalog's own validation is what reports that, in one place.
func TestACloneWithNoOriginKeepsAnEmptyUrl(t *testing.T) {
	handed := t.TempDir()
	git := &fakeGit{
		answer: map[string]string{"rev-parse": "here"},
		fail:   map[string]error{"remote": errors.New("no such remote origin")},
	}

	p, err := prepare(context.Background(), Target{ID: "thing", Checkout: handed},
		t.TempDir(), func(Plan) error { return nil }, git.run)
	if err != nil {
		t.Fatal(err)
	}
	if p.URL != "" {
		t.Errorf("URL = %q, want none", p.URL)
	}
}

// The most common first run for anybody but the author is a repository they
// cannot reach, and git's own message is the whole answer. Nothing is added to
// it but the url that was resolved, because a lab error that paraphrases a git
// error is a second place for the truth to live.
func TestAnUnreachableRepositoryFailsWithGitsOwnMessage(t *testing.T) {
	git := &fakeGit{fail: map[string]error{"ls-remote": errors.New("repository not found")}}
	announced := false

	_, err := prepare(context.Background(), Target{ID: "thing", URL: "https://example.test/thing.git"},
		t.TempDir(), func(Plan) error { announced = true; return nil }, git.run)

	if err == nil {
		t.Fatal("an unreachable repository resolved to a plan")
	}
	for _, want := range []string{"repository not found", "https://example.test/thing.git"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to carry %q", err, want)
		}
	}
	if announced {
		t.Error("a plan was announced for a repository that could not be read")
	}
}

// A remote that answers with no head is not a repository anything can be pinned
// against, and an empty pin would be recorded as one.
func TestARemoteThatNamesNoHeadIsRefused(t *testing.T) {
	git := &fakeGit{answer: map[string]string{"ls-remote": ""}}

	_, err := prepare(context.Background(), Target{ID: "thing", URL: "u"},
		t.TempDir(), func(Plan) error { return nil }, git.run)

	if err == nil || !strings.Contains(err.Error(), "nothing to pin") {
		t.Fatalf("err = %v, want a refusal naming what is missing", err)
	}
}

func TestAFailedCloneCarriesTheUrlItFailedOn(t *testing.T) {
	git := &fakeGit{
		answer: map[string]string{"ls-remote": "abc\tHEAD"},
		fail:   map[string]error{"clone": errors.New("could not read Username")},
	}

	_, err := prepare(context.Background(), Target{ID: "thing", URL: "https://example.test/thing.git"},
		t.TempDir(), func(Plan) error { return nil }, git.run)

	if err == nil {
		t.Fatal("a failed clone was accepted")
	}
	for _, want := range []string{"could not read Username", "https://example.test/thing.git"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to carry %q", err, want)
		}
	}
}

// A correction asks for the one revision first and falls back to fetching the
// whole remote, because a host that refuses to serve a sha by name is common and
// the checkout is what decides either way.
func TestACorrectionFallsBackToAFullFetchWhenTheShaCannotBeAskedFor(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "thing"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := &fakeGit{
		answer: map[string]string{"rev-parse": "drifted"},
		fail:   map[string]error{"fetch": errors.New("server does not allow request for unadvertised object")},
	}

	if _, err := prepare(context.Background(), Target{ID: "thing", URL: "u", Pin: "pinned"},
		root, func(Plan) error { return nil }, git.run); err != nil {
		t.Fatal(err)
	}

	var fetches int
	for _, c := range git.calls {
		if c[1] == "fetch" {
			fetches++
		}
	}
	if fetches != 2 {
		t.Errorf("fetch calls = %d, want the sha asked for and then the remote", fetches)
	}
}

// A correction that cannot reach the pin fails rather than leaving the clone
// wherever it drifted to.
func TestACorrectionThatCannotReachThePinFails(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "thing"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := &fakeGit{
		answer: map[string]string{"rev-parse": "drifted"},
		fail:   map[string]error{"checkout": errors.New("pathspec did not match")},
	}

	_, err := prepare(context.Background(), Target{ID: "thing", URL: "u", Pin: "pinnedaaaaaa"},
		root, func(Plan) error { return nil }, git.run)

	if err == nil || !strings.Contains(err.Error(), "pinnedaaaaaa") {
		t.Fatalf("err = %v, want a failure naming the pin it could not reach", err)
	}
}

// A checkout whose revision cannot be read is not a checkout, and guessing one
// would pin the repository at nothing.
func TestACheckoutWhoseRevisionCannotBeReadIsRefused(t *testing.T) {
	handed := t.TempDir()
	git := &fakeGit{fail: map[string]error{"rev-parse": errors.New("not a git repository")}}

	_, err := prepare(context.Background(), Target{ID: "thing", Checkout: handed},
		t.TempDir(), func(Plan) error { return nil }, git.run)

	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("err = %v, want git's own message", err)
	}
}

// The revision read back after a successful clone is the pin, so a clone that
// lands and then cannot be read is a failure rather than an empty pin.
func TestAPinThatCannotBeReadBackIsAFailure(t *testing.T) {
	git := &fakeGit{
		answer: map[string]string{"ls-remote": "abc\tHEAD"},
		fail:   map[string]error{"rev-parse": errors.New("unknown revision")},
	}

	if _, err := prepare(context.Background(), Target{ID: "thing", URL: "u"},
		t.TempDir(), func(Plan) error { return nil }, git.run); err == nil {
		t.Fatal("a clone whose revision could not be read was recorded anyway")
	}
}

// The real runner, exercised once: git's own message has to survive the wrapper,
// because every refusal above is only as good as what reaches it.
func TestTheRealRunnerPassesGitsMessageThrough(t *testing.T) {
	_, err := gitCommand(context.Background(), t.TempDir(), "rev-parse", "HEAD")

	if err == nil {
		t.Fatal("rev-parse in an empty directory succeeded")
	}
	if !strings.Contains(err.Error(), "rev-parse") || !strings.Contains(strings.ToLower(err.Error()), "git repository") {
		t.Errorf("err = %q, want git's own words", err)
	}
}

// Prepare is what the command calls, and it reaches the real git. One pass
// through it proves the seam below is the same code path.
func TestPrepareReachesRealGit(t *testing.T) {
	dir, head := oneCommitRepo(t)

	p, err := Prepare(context.Background(), Target{ID: "thing", Checkout: dir}, t.TempDir(), func(Plan) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	if p.Revision != head {
		t.Errorf("pin = %q, want the checkout's head %q", p.Revision, head)
	}
	if p.Action() != Read {
		t.Errorf("Action = %s, want %s for a handed-in clone", p.Action(), Read)
	}
}

// oneCommitRepo is a real repository with one commit, which is all the pin
// needs to be read out of something.
func oneCommitRepo(t *testing.T) (dir, head string) {
	t.Helper()
	dir = t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"-c", "user.email=lab@example.test", "-c", "user.name=lab", "commit", "-q", "--allow-empty", "-m", "first"},
	} {
		if _, err := gitCommand(context.Background(), dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	out, err := gitCommand(context.Background(), dir, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return dir, strings.TrimSpace(string(out))
}

// A clone the lab made and somebody deleted is re-made AT ITS PIN, not at
// whatever the default branch points at today. The tree would otherwise move
// under a repository file that did not, which is the disagreement this package
// exists to prevent.
func TestAnAdmittedRepositoryWhoseCloneIsGoneIsRemadeAtItsPin(t *testing.T) {
	git := &fakeGit{answer: map[string]string{"rev-parse": "whatever-the-branch-is-at"}}

	p, err := prepare(context.Background(), Target{ID: "thing", URL: "u", Pin: "pinnedaaaaaa"},
		t.TempDir(), func(Plan) error { return nil }, git.run)
	if err != nil {
		t.Fatal(err)
	}

	if !git.ran("clone") || !git.ran("checkout") {
		t.Fatalf("git calls = %v, want it cloned and put at its pin", git.calls)
	}
	if p.Revision != "pinnedaaaaaa" {
		t.Errorf("pin = %q, want the repository's own pin", p.Revision)
	}
}

// And if it cannot be put there, that is a failure rather than a repository
// quietly re-pinned at a tree nobody chose.
func TestARemadeCloneThatCannotReachItsPinIsAFailure(t *testing.T) {
	git := &fakeGit{
		answer: map[string]string{"rev-parse": "somewhere-else"},
		fail:   map[string]error{"checkout": errors.New("reference is not a tree")},
	}

	_, err := prepare(context.Background(), Target{ID: "thing", URL: "u", Pin: "pinnedaaaaaa"},
		t.TempDir(), func(Plan) error { return nil }, git.run)

	if err == nil || !strings.Contains(err.Error(), "pinnedaaaaaa") {
		t.Fatalf("err = %v, want a failure naming the pin", err)
	}
}

// The last line of defence: a repository already pinned may not come back from
// admission at some other revision, however that happened.
func TestAnAdmittedRepositoryNeverComesBackAtADifferentRevision(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "thing"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A correction that reports success and leaves the tree where it was.
	git := &fakeGit{answer: map[string]string{"rev-parse": "movedbbbbbbb"}, stuck: true}

	_, err := prepare(context.Background(), Target{ID: "thing", URL: "u", Pin: "pinnedaaaaaa"},
		root, func(Plan) error { return nil }, git.run)

	if err == nil || !strings.Contains(err.Error(), "did not use") {
		t.Fatalf("err = %v, want a refusal rather than a silent re-pin", err)
	}
}

// A clone is minutes of network and an interrupt part way through leaves a
// half-made directory behind. Every admission after that finds a directory,
// cannot read a revision out of it, and — for a repository that is not in the
// catalog and so has no pin to recover from — says only "not a git repository".
// The way out is one removal, and the message is what says so.
func TestAHalfMadeCloneSaysHowToGetOutOfIt(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "thing"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := &fakeGit{fail: map[string]error{"rev-parse": errors.New("not a git repository")}}

	_, err := prepare(context.Background(), Target{ID: "thing", URL: "u"},
		root, func(Plan) error { return nil }, git.run)

	if err == nil {
		t.Fatal("a half-made clone was admitted")
	}
	for _, want := range []string{"not a git repository", filepath.Join(root, "thing"), "remove it"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to carry %q", err, want)
		}
	}
}

// The same message would be wrong about a directory the lab did not make, so it
// is not said there: a handed-in path that is not a repository is the operator's
// to explain, and telling them to delete it is telling them to delete their own
// directory.
func TestTheRemovalIsOnlySuggestedForTheLabsOwnDirectory(t *testing.T) {
	handed := t.TempDir()
	git := &fakeGit{fail: map[string]error{"rev-parse": errors.New("not a git repository")}}

	_, err := prepare(context.Background(), Target{ID: "thing", Checkout: handed},
		t.TempDir(), func(Plan) error { return nil }, git.run)

	if err == nil {
		t.Fatal("a directory that is not a repository was admitted")
	}
	if strings.Contains(err.Error(), "remove it") {
		t.Errorf("err = %q, want no advice to delete a directory the lab does not own", err)
	}
}
