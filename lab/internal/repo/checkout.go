package repo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runner is how this package reaches git, so a test can drive an outcome a real
// repository cannot be made to produce. The real one is gitCommand.
type runner func(ctx context.Context, dir string, args ...string) ([]byte, error)

// gitCommand runs one git call and passes its own message through on failure.
//
// Nothing is added to what git said beyond the arguments it was given. A lab
// error that paraphrases a git error is a second place for the truth to live,
// and the most common first run for anybody but the author is a private or
// unreachable repository, where git's message is the whole answer.
//
// There is no timeout: a clone of a large repository legitimately takes
// minutes, and a wall short enough to catch a wedged one would kill it. The
// context is the caller's, and an interrupt reaches the child through it.
func gitCommand(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errBuf.String()))
	}
	return out, nil
}

// Target is the repository admission is about to act on: what it is called,
// where it comes from, where its checkout is, and what it is already pinned at.
//
// Checkout is empty for a repository the lab clones itself, and that emptiness
// is the ownership record. The lab fixes what it made and reads what it was
// given, so a checkout it can name from the id alone is one it may move, and a
// path somebody handed in is one it may not.
type Target struct {
	ID  string
	URL string
	// Checkout is a handed-in directory. Empty means the lab's own clones root.
	Checkout string
	// Pin is the revision already recorded for this repository, empty for one
	// that has never been admitted.
	Pin string
}

// Action is what admission is about to do to a checkout.
type Action string

const (
	// MakeIt: there is no checkout yet, so the lab clones one.
	MakeIt Action = "clone"
	// Adopt: a checkout is already there at the revision wanted. A clone that
	// is already right is adopted rather than replaced.
	Adopt Action = "adopt"
	// Correct: a lab-owned checkout has drifted off the pin and goes back to
	// it. An unattended crank must not park a repository on a stale tree
	// nobody is watching.
	Correct Action = "correct"
	// Read: a handed-in checkout at the revision wanted, read and never
	// written to.
	Read Action = "read"
	// Refuse: a handed-in checkout at some other revision. It is somebody's
	// working tree and the lab does not perform a checkout in it.
	Refuse Action = "refuse"
)

// Plan is what admission decided, before it has written anything.
//
// It is announced first, every time. This is not a confirmation prompt — the
// run path forbids one — it is the command saying what it decided, so a wrong
// repository is visible in the scrollback rather than three phases later.
type Plan struct {
	ID       string
	URL      string
	Revision string
	Checkout string
	// At is the revision the checkout sits at now, empty when there is none.
	At string
	// Owned says the lab made this checkout.
	Owned bool
	// Admitted says the catalog already holds this repository.
	Admitted bool
}

// Action is what this plan does to the checkout.
func (p Plan) Action() Action {
	switch {
	case p.At == "":
		return MakeIt
	case p.At == p.Revision:
		if p.Owned {
			return Adopt
		}
		return Read
	case p.Owned:
		return Correct
	}
	return Refuse
}

// Prepare puts the checkout at the revision the plan names and reports the
// revision it ended up at.
//
// The order is the point. Everything that decides happens first and touches
// nothing; announce is handed the decision; only then does anything reach a
// disk. A caller whose announce returns an error stops the whole admission
// there, with the directory as it found it.
func Prepare(ctx context.Context, t Target, root string, announce func(Plan) error) (Plan, error) {
	return prepare(ctx, t, root, announce, gitCommand)
}

func prepare(ctx context.Context, t Target, root string, announce func(Plan) error, git runner) (Plan, error) {
	p, err := planFor(ctx, t, root, git)
	if err != nil {
		return Plan{}, err
	}
	if err := announce(p); err != nil {
		return p, err
	}
	if err := apply(ctx, p, git); err != nil {
		return p, err
	}
	// The pin is read back out of the tree rather than carried over from the
	// plan. A revision that was asked for is a request; the one the checkout
	// sits at is the fact, and every later worktree is taken from that tree.
	at, err := head(ctx, p.Checkout, git)
	if err != nil {
		return p, err
	}
	if p.Admitted && at != p.Revision {
		// The last line of defence for the whole package. A repository already
		// pinned may not come back from here at some other revision, however
		// that happened, because the pin is what every worktree after this is
		// taken at.
		return p, fmt.Errorf("%s is pinned at %.12s and its checkout is at %.12s after being prepared; "+
			"a run against that tree would record a commit it did not use", p.ID, p.Revision, at)
	}
	p.At, p.Revision = at, at
	return p, nil
}

// planFor reads what is there and decides what admission will do. It writes
// nothing, which is what makes announcing its result before acting worth
// anything.
func planFor(ctx context.Context, t Target, root string, git runner) (Plan, error) {
	p := Plan{ID: t.ID, URL: t.URL, Admitted: t.Pin != "", Checkout: t.Checkout, Owned: t.Checkout == ""}
	if p.Owned {
		p.Checkout = filepath.Join(root, t.ID)
	}
	if OnDisk(p.Checkout) {
		at, err := head(ctx, p.Checkout, git)
		if err != nil {
			return Plan{}, debris(p, err)
		}
		p.At = at
	}
	if p.URL == "" {
		// A handed-in clone knows where it came from, and reading it is the
		// only way to record a url for one. A clone with no origin keeps an
		// empty url, and the catalog's own validation is what reports that.
		if out, err := git(ctx, p.Checkout, "remote", "get-url", "origin"); err == nil {
			p.URL = strings.TrimSpace(string(out))
		}
	}
	rev, err := wanted(ctx, t, p, git)
	if err != nil {
		return Plan{}, err
	}
	p.Revision = rev
	return p, nil
}

// wanted is the revision this admission is aiming at: the recorded pin if there
// is one, the checkout's own head if it is already there, and otherwise
// whatever the remote's default branch points at right now.
func wanted(ctx context.Context, t Target, p Plan, git runner) (string, error) {
	switch {
	case t.Pin != "":
		return t.Pin, nil
	case p.At != "":
		return p.At, nil
	}
	out, err := git(ctx, ".", "ls-remote", "--", p.URL, "HEAD")
	if err != nil {
		return "", fmt.Errorf("read the head of %s: %w", p.URL, err)
	}
	sha, _, ok := strings.Cut(strings.TrimSpace(string(out)), "\t")
	if !ok || sha == "" {
		return "", fmt.Errorf("%s answered no head revision, so there is nothing to pin", p.URL)
	}
	return sha, nil
}

// apply carries the plan out.
func apply(ctx context.Context, p Plan, git runner) error {
	switch p.Action() {
	case MakeIt:
		if err := os.MkdirAll(filepath.Dir(p.Checkout), 0o755); err != nil {
			return fmt.Errorf("prepare the clones root: %w", err)
		}
		// `--` before the url, because everything after it is an operand
		// however it is spelled. The resolver already refuses anything that is
		// not a handle, a url or a directory, so a url that reads as an option
		// cannot get here — and this is the second lock on a door that opens
		// onto running whatever an argument names.
		if _, err := git(ctx, filepath.Dir(p.Checkout), "clone", "--", p.URL, filepath.Base(p.Checkout)); err != nil {
			return fmt.Errorf("clone %s: %w", p.URL, err)
		}
		if !p.Admitted {
			// A first admission is pinned at whatever it cloned, and that is
			// the revision this clone lands on by default.
			return nil
		}
		// A repository that was already admitted and whose clone is gone is
		// re-cloned at ITS pin, not at whatever the default branch points at
		// today. Without this the tree would move under a repository file that
		// did not, which is precisely the disagreement admission exists to
		// prevent — and nothing about a later result would show it.
		if _, err := git(ctx, p.Checkout, "checkout", "--detach", "--quiet", p.Revision); err != nil {
			return fmt.Errorf("put the new clone of %s at its pin %.12s: %w", p.ID, p.Revision, err)
		}
		return nil
	case Correct:
		if _, err := git(ctx, p.Checkout, "fetch", "--quiet", "origin", p.Revision); err != nil {
			// A revision already in the clone needs no fetch, and a remote that
			// refuses to serve one by sha is common. Either way the checkout
			// below is what decides, and it reports its own failure.
			_, _ = git(ctx, p.Checkout, "fetch", "--quiet", "origin")
		}
		if _, err := git(ctx, p.Checkout, "checkout", "--detach", "--quiet", p.Revision); err != nil {
			return fmt.Errorf("move %s back to its pin %.12s: %w", p.Checkout, p.Revision, err)
		}
		return nil
	case Refuse:
		return fmt.Errorf("%s is at %.12s and this repository is pinned at %.12s, and the lab did not "+
			"make that checkout: it is yours, so it is read and never moved. Check it out at the pin "+
			"yourself, or hand in a different clone", p.Checkout, p.At, p.Revision)
	default:
		return nil
	}
}

// debris says what to do about a directory that is in the checkout's place and
// is not a repository.
//
// The case is ordinary rather than exotic: a clone is minutes of network, and
// an interrupt part way through leaves a half-made directory under the lab's
// own root. Every admission after that finds a directory, fails to read a
// revision out of it, and reports "not a git repository" about a repository
// that is not in the catalog and so has no pin to recover from. The way out is
// one `rm -rf`, and it is the message's job to say so — the lab does not delete
// a directory on its own, even one it made, because the same code path would
// delete a clone somebody handed in if the ownership rule ever moved.
func debris(p Plan, err error) error {
	if !p.Owned {
		return err
	}
	return fmt.Errorf("%w. %s is the lab's own clone directory and it is not a repository, "+
		"which is what an interrupted clone leaves behind; remove it and run this again", err, p.Checkout)
}

// head is the revision a checkout sits at.
func head(ctx context.Context, dir string, git runner) (string, error) {
	out, err := git(ctx, dir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("read the revision of %s: %w", dir, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// OnDisk reports whether a path is a directory. It is what [Resolve] is handed
// as its on-disk test, so a path resolves and a checkout is found here by one
// rule rather than by two that could drift apart.
func OnDisk(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && fi.IsDir()
}
