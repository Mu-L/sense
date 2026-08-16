package ground

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/score"
)

// fakeGit is a repository described as a map, so every outcome below is
// reachable without a checkout on disk. The real runner is one exec.Command.
type fakeGit struct {
	commit string
	files  map[string]string
	grep   []string // "path:line" entries a covering grep would print
	calls  int
}

func (f *fakeGit) run(_ string, args ...string) ([]byte, error) {
	f.calls++
	switch args[0] {
	case "cat-file":
		// EXACT. An earlier version accepted any commit that extended its own,
		// which is more permissive than git and hid the difference.
		if args[2] == f.commit+"^{commit}" {
			return nil, nil
		}
		return nil, errors.New("not a commit")
	case "grep":
		// <commit>:<path>:<line>:<text>, which is what `git grep -n <rev>` emits.
		if len(f.grep) == 0 {
			return nil, errors.New("no matches")
		}
		var b strings.Builder
		for _, hit := range f.grep {
			b.WriteString(f.commit + ":" + hit + ":the line\n")
		}
		return []byte(b.String()), nil
	case "show":
		_, path, _ := strings.Cut(args[1], ":")
		if body, ok := f.files[path]; ok {
			return []byte(body), nil
		}
		return nil, errors.New("path does not exist")
	}
	return nil, errors.New("unexpected git call")
}

func repo(t *testing.T) (*Checkout, *fakeGit) {
	t.Helper()
	g := &fakeGit{commit: "abc123", files: map[string]string{
		"app/models/category.rb": "one\ntwo\nthree\n",
		"app/no_trailing.rb":     "one\ntwo",
	}}
	c, err := open("/repo", "abc123", g.run)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return c, g
}

// The whole point of the package: unverified, a fabricated citation scores
// exactly like a correct one, so the metric rewards confident invention.
func TestAFabricatedCitationIsReportedAndExcluded(t *testing.T) {
	c, _ := repo(t)
	cites := []score.Cite{
		{Path: "app/models/category.rb", Line: 2},
		{Path: "app/models/invented.rb", Line: 1},
	}

	r := Check(cites, c)

	if len(r.Ungrounded) != 1 || r.Ungrounded[0] != "app/models/invented.rb:1" {
		t.Errorf("ungrounded = %v, want the invented file", r.Ungrounded)
	}
	if r.Checked != 2 {
		t.Errorf("checked %d locations, want 2", r.Checked)
	}
}

// A real file and a line past its end. This is the half a file-existence check
// would miss, and it is the commoner shape: the file is real, the line is not.
func TestARealFileAtALineItDoesNotHaveDoesNotResolve(t *testing.T) {
	c, _ := repo(t)

	for _, tc := range []struct {
		name string
		line int
		want bool
	}{
		{"the last line", 3, true},
		{"one past the end", 4, false},
		{"far past the end", 999999, false},
		{"the first line", 1, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Check([]score.Cite{{Path: "app/models/category.rb", Line: tc.line}}, c)
			if got := len(r.Ungrounded) == 0; got != tc.want {
				t.Errorf("resolves = %v, want %v (ungrounded %v)", got, tc.want, r.Ungrounded)
			}
		})
	}
}

// A last line with no trailing newline is still a line, and counting newlines
// alone would report the real last line of such a file as fabricated.
func TestTheLastLineCountsWhenTheFileHasNoTrailingNewline(t *testing.T) {
	c, _ := repo(t)

	r := Check([]score.Cite{{Path: "app/no_trailing.rb", Line: 2}}, c)
	if len(r.Ungrounded) != 0 {
		t.Errorf("the real last line was reported as not resolving: %v", r.Ungrounded)
	}
}

// "cannot check" is a first-class outcome, not an error. A score computed
// without a checkout is valid and says grounding was not verified; what is
// forbidden is silently treating unverified as verified.
func TestWithNoCheckoutNothingIsDroppedAndNothingIsClaimed(t *testing.T) {
	cites := []score.Cite{
		{Path: "app/models/invented.rb", Line: 1},
		{Path: "app/models/category.rb", Line: 2},
	}

	r := Check(cites, nil)

	if r.Checked != 0 {
		t.Errorf("checked %d locations with no checkout", r.Checked)
	}
	if r.Verified() {
		t.Error("a report with no checkout behind it claimed to be verified")
	}
	if !strings.Contains(r.String(), "NOT VERIFIED") {
		t.Errorf("the rendered report does not say it verified nothing:\n%s", r.String())
	}
}

// A checkout at the wrong commit is worse than none: it reports a citation as
// fabricated because a file moved between revisions, which is a claim about the
// arm made from a fact about the disk.
func TestACheckoutWithoutThePinnedCommitIsRefused(t *testing.T) {
	g := &fakeGit{commit: "abc123"}

	if _, err := open("/repo", "deadbeef", g.run); err == nil {
		t.Error("a checkout that does not have the pinned commit was accepted")
	}
	// A prefix of the pinned commit is not the pinned commit here: the fake is
	// exact so that it cannot be laxer than git and hide a bug.
	if _, err := open("/repo", "abc", g.run); err == nil {
		t.Error("a prefix of the pinned commit was accepted by the fake")
	}
	for _, tc := range [][2]string{{"", "abc123"}, {"/repo", ""}} {
		if _, err := open(tc[0], tc[1], g.run); err == nil {
			t.Errorf("open(%q, %q) was accepted", tc[0], tc[1])
		}
	}
}

// A citation with no line, or a bare symbol with no file behind it, names no
// location to resolve. It is carried through rather than counted as fabricated,
// because this package cannot tell where a bare constant lives.
func TestWhatCannotBeResolvedIsCarriedRatherThanCondemned(t *testing.T) {
	c, _ := repo(t)
	cites := []score.Cite{
		{Path: "app/models/category.rb"},                                // named, no line
		{Symbol: "Admin::ActionLogsController#index", Line: 7},          // no file behind it
		{Established: "app/models/category.rb", Symbol: "X#y", Line: 2}, // a file it continues
	}

	r := Check(cites, c)

	if len(r.Ungrounded) != 0 {
		t.Errorf("unresolvable was reported as fabricated: %v", r.Ungrounded)
	}
	if r.Checked != 1 {
		t.Errorf("checked %d locations, want only the one that names a file and a line", r.Checked)
	}
}

// The same location cited twenty times is one lookup. Grounding shells out per
// distinct file, and a long answer names the same file constantly.
func TestTheSameFileIsLookedUpOnce(t *testing.T) {
	c, g := repo(t)
	before := g.calls

	var cites []score.Cite
	for i := 1; i <= 3; i++ {
		for j := 0; j < 5; j++ {
			cites = append(cites, score.Cite{Path: "app/models/category.rb", Line: i})
		}
	}
	if r := Check(cites, c); r.Checked != 3 {
		t.Errorf("checked %d distinct locations, want 3", r.Checked)
	}
	if got := g.calls - before; got != 1 {
		t.Errorf("%d git calls for one file, want 1", got)
	}
}

// The real git seam, against a real repository.
//
// Every test above drives a fake runner, which proves the logic and proves
// nothing about the one line that actually shells out. A repository built here
// is small, hermetic and disposable, and it is the only thing that can catch a
// wrong argument to git.
func newRepo(t *testing.T) (dir, commit string) {
	t.Helper()
	dir = t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "app", "models"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "app", "models", "category.rb"),
		[]byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
		{"add", "-A"},
		{"commit", "-qm", "fixture"},
	} {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skipf("git is unavailable here: %v: %s", err, out)
		}
	}
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	return dir, strings.TrimSpace(string(out))
}

func TestAgainstARealRepository(t *testing.T) {
	dir, commit := newRepo(t)

	c, err := Open(dir, commit)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	r := Check([]score.Cite{
		{Path: "app/models/category.rb", Line: 3},
		{Path: "app/models/category.rb", Line: 4},
		{Path: "app/models/nowhere.rb", Line: 1},
	}, c)

	if len(r.Ungrounded) != 2 {
		t.Errorf("ungrounded = %v, want the past-the-end line and the absent file", r.Ungrounded)
	}
	if !r.Verified() {
		t.Error("a report from a real checkout says it verified nothing")
	}
	if got := r.String(); !strings.Contains(got, "2 of 3 cited locations do not resolve") {
		t.Errorf("rendered as %q", got)
	}
}

func TestOpeningSomethingThatIsNotARepository(t *testing.T) {
	if _, err := Open(t.TempDir(), "deadbeef"); err == nil {
		t.Error("an empty directory was accepted as a checkout")
	}
}

// The property that makes grounding safe, stated as strongly as it can be.
//
// Grounding does not return citations at all, so it CANNOT change a score. An
// earlier version returned a filtered set for the scorer to use, which looked
// stricter and was not: the only rule that can credit an invention is the symbol
// rule, which has no path for this package to check, while samePath deliberately
// credits a citation carrying more leading directories than gold — an absolute
// path, of which the corpus holds 127 — and `git show commit:/Users/...` cannot
// resolve one. So the filter removed true positives and no fabrications.
//
// This test is the guard on that: the matches a gold row has before grounding
// are the matches it has after, whatever grounding thinks of them.
func TestGroundingCannotChangeWhatMatches(t *testing.T) {
	c, _ := repo(t)
	const goldPath, goldLine = "app/models/category.rb", "2"

	cites := []score.Cite{
		{Path: goldPath, Line: 2},
		{Path: "/Users/someone/checkouts/" + goldPath, Line: 2}, // absolute; samePath credits it, git cannot resolve it
		{Path: goldPath, Line: 4000},
		{Path: "app/models/invented.rb", Line: 2},
	}

	before := countMatches(cites, goldPath, goldLine)
	r := Check(cites, c)
	after := countMatches(cites, goldPath, goldLine)

	if len(r.Ungrounded) < 2 {
		t.Fatalf("premise gone: only %v failed to resolve", r.Ungrounded)
	}
	if before != after || before != 2 {
		t.Errorf("matches went from %d to %d, want 2 both times", before, after)
	}
}

func countMatches(cites []score.Cite, goldPath, goldLine string) int {
	n := 0
	for _, c := range cites {
		if score.Matches(c, goldPath, goldLine) {
			n++
		}
	}
	return n
}

// The wrong checkout is the dangerous case: a commit that exists in another
// repository passes every existence check, resolves nothing, and the report
// says verified. The gold rows are the detector.
func TestAValidCommitInTheWrongRepositoryIsRefused(t *testing.T) {
	c, _ := repo(t)

	if err := CheckGold([]string{"app/models/category.rb:2"}, c); err != nil {
		t.Fatalf("the right checkout was refused: %v", err)
	}

	err := CheckGold([]string{"app/models/category.rb:2", "src/Core/Billing/Plan.cs:40"}, c)
	if err == nil {
		t.Fatal("a checkout that cannot resolve the gold was accepted")
	}
	if !strings.Contains(err.Error(), "Plan.cs:40") {
		t.Errorf("the error does not name what failed: %v", err)
	}
	// And a stale gold row on the RIGHT checkout fails the same way, loudly,
	// rather than quietly subtracting from an arm.
	if err := CheckGold([]string{"app/models/category.rb:9999"}, c); err == nil {
		t.Error("a gold row pointing past the end of a real file was accepted")
	}
	if err := CheckGold([]string{"app/models/category.rb:2"}, nil); err != nil {
		t.Errorf("with no checkout there is nothing to refuse: %v", err)
	}
	// A gold row carrying no location is 02-05's problem, not grounding's: it
	// is skipped rather than reported as a checkout that cannot resolve it,
	// because condemning the checkout for it would point at the wrong thing.
	for _, unlocatable := range []string{
		"the ActiveRecord::Relation object itself", // colons, but no line after one
		"app/models/category.rb:not-a-number",      // a colon, and not a number
		"a gold row that is pure prose",            // no colon at all
	} {
		if err := CheckGold([]string{unlocatable}, c); err != nil {
			t.Errorf("CheckGold(%q) blamed the checkout for a row that names no location: %v",
				unlocatable, err)
		}
	}
}

// Nothing checkable is not a clean bill of health.
func TestAReportWithNothingToCheckSaysSo(t *testing.T) {
	c, _ := repo(t)
	r := Check([]score.Cite{{Symbol: "Category#find", Line: 3}}, c)

	if got := r.String(); !strings.Contains(got, "nothing to check") {
		t.Errorf("rendered as %q, which reads as a pass", got)
	}
}

// The peel, which only a real repository can express. `commit^{commit}` is what
// makes a tree or a tag resolve to the commit it belongs to, or fail. Without
// it a tree SHA passes as a commit and every path afterwards resolves against
// something that is not a revision.
func TestATreeIsNotACommit(t *testing.T) {
	dir, commit := newRepo(t)
	out, err := exec.Command("git", "-C", dir, "rev-parse", commit+"^{tree}").Output()
	if err != nil {
		t.Fatal(err)
	}
	tree := strings.TrimSpace(string(out))

	if tree == commit {
		t.Fatal("premise gone: the tree and the commit are the same object")
	}
	if _, err := Open(dir, tree); err == nil {
		t.Error("a tree SHA was accepted as the pinned commit")
	}
}

// The package's central promise: it reads the PINNED commit, not whatever is
// checked out. A one-commit fixture cannot tell the two apart, so this one has
// two and the file grows between them.
func TestItReadsThePinnedCommitAndNotTheCurrentOne(t *testing.T) {
	dir, first := newRepo(t)

	if err := os.WriteFile(filepath.Join(dir, "app", "models", "category.rb"),
		[]byte("one\ntwo\nthree\nfour\nfive\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-qm", "grew"}} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", err, out)
		}
	}

	// Line 5 exists at HEAD and does not exist at the pinned commit.
	c, err := Open(dir, first)
	if err != nil {
		t.Fatal(err)
	}
	r := Check([]score.Cite{{Path: "app/models/category.rb", Line: 5}}, c)
	if len(r.Ungrounded) != 1 {
		t.Error("a line that only exists at HEAD resolved against the pinned commit")
	}

}

// Resolves is the gold validator's view of the same question grounding already
// answers: is this location there at the pinned commit?
func TestResolvesAnswersForTheValidator(t *testing.T) {
	c, _ := repo(t)

	for _, tc := range []struct {
		name       string
		path       string
		line       int
		ok, checkd bool
	}{
		{"a real line", "app/models/category.rb", 3, true, true},
		{"past the end", "app/models/category.rb", 4, false, true},
		{"line zero is not a line", "app/models/category.rb", 0, false, true},
		{"a file that is not there", "app/models/nope.rb", 1, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, checked := c.Resolves(tc.path, tc.line)
			if ok != tc.ok || checked != tc.checkd {
				t.Errorf("Resolves = (%v, %v), want (%v, %v)", ok, checked, tc.ok, tc.checkd)
			}
		})
	}

	var absent *Checkout
	if _, checked := absent.Resolves("app/models/category.rb", 1); checked {
		t.Error("a nil checkout claimed to have checked something")
	}
}

// Covering is what the baseline gets for free, and the SIZE is the whole story:
// a row inside a 4483-line grep is not free to anybody.
func TestCoveringReportsTheGrepAndItsSize(t *testing.T) {
	g := &fakeGit{
		commit: "abc123",
		grep:   []string{"app/models/category.rb:12", "app/models/other.rb:4"},
	}
	c, err := open("/repo", "abc123", g.run)
	if err != nil {
		t.Fatal(err)
	}

	hits, total, checked := c.Covering("Category")
	if !checked {
		t.Fatal("Covering reported it could not check")
	}
	if total != 2 {
		t.Errorf("total = %d, want 2", total)
	}
	if !hits["app/models/category.rb:12"] || hits["app/models/nowhere.rb:1"] {
		t.Errorf("hits = %v", hits)
	}

	t.Run("an anchor that appears nowhere is a real answer", func(t *testing.T) {
		empty := &fakeGit{commit: "abc123"}
		c, _ := open("/repo", "abc123", empty.run)
		hits, total, checked := c.Covering("Nowhere")
		if !checked || total != 0 || len(hits) != 0 {
			t.Errorf("got (%v, %d, %v), want an empty but checked answer", hits, total, checked)
		}
	})

	t.Run("nothing to grep for", func(t *testing.T) {
		if _, _, checked := c.Covering(""); checked {
			t.Error("Covering claimed to check an empty anchor")
		}
		var absent *Checkout
		if _, _, checked := absent.Covering("Category"); checked {
			t.Error("a nil checkout claimed to have grepped")
		}
	})
}

// git grep output that is not the shape this expects is skipped rather than
// half-parsed into a location nobody cited.
func TestCoveringIgnoresLinesThatAreNotHits(t *testing.T) {
	g := &fakeGit{commit: "abc123"}
	c, _ := open("/repo", "abc123", func(dir string, args ...string) ([]byte, error) {
		if args[0] == "grep" {
			return []byte("abc123:app/models/category.rb:12:real\n" +
				"some binary file matches\n" +
				"abc123:app/models/x.rb:notanumber:nope\n" +
				"abc123:justapath\n"), nil
		}
		return g.run(dir, args...)
	})

	hits, total, _ := c.Covering("Category")
	if total != 1 || !hits["app/models/category.rb:12"] {
		t.Errorf("got %v (total %d), want only the well-formed hit", hits, total)
	}
}
