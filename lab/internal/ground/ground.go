// Package ground checks that a citation points at somewhere that exists.
//
// An arm can cite a file and a line that are not there. Unverified, a
// fabricated citation scores exactly like a correct one, so the metric rewards
// confident invention.
//
// This is the ONLY part of scoring that needs a repository on disk, and it is
// its own package for that reason. Recall must not become dependent on the
// state of a disk: a missing checkout downgrades a result to unverified, and
// never fails it.
//
// # What it found on the corpus
//
// Run over the 57 recorded discourse runs, against the checkout at its pinned
// commit: 11,369 located citations, 912 of which do not resolve. By arm that
// reads as sense 10.33% and baseline 2.61%, which invites the conclusion that
// one arm invents four times as much. It does not survive being checked.
//
// Decomposed, outright invention is negligible and SYMMETRIC: a real file cited
// past its end happens 3 times in each arm. 70% of the rest is a path written
// without its directory (`category.rb:1090` for `app/models/category.rb`),
// which is a citation form rather than a falsehood. And the gap is one cell:
// 815 of the 823 sense failures are claude-opus-5, while the same arm on kimi
// is 0.35% and on gpt-5.6 is 0.00%.
//
// So the headline rate measures how often an arm writes a short path, not how
// often it makes something up. Two things follow. The number is reported per
// run and never aggregated across arms without that decomposition, and the 579
// bare basenames are a standing opportunity: a checkout can say whether a
// basename is unique at that commit, which would turn a guess into a fact. That
// is a later pitch, not this one.
package ground

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/luuuc/sense/lab/internal/score"
)

// Checkout is a repository at one commit. A nil *Checkout is the "cannot check"
// case and is valid input everywhere in this package.
type Checkout struct {
	dir    string
	commit string
	git    runner
	lines  map[string]int // -1 means the file is not there at this commit
}

// runner is how this package reaches git, so a test can drive every outcome
// without a repository. The real one is gitCommand.
type runner func(dir string, args ...string) ([]byte, error)

// gitTimeout bounds one git call.
//
// Nothing here is networked, and Stdin is nil so the child cannot read a
// prompt, but git can still reach /dev/tty through an askpass helper and a
// directory on a dead network mount blocks in the kernel with no way out.
// Without a deadline that is a wedged bench rather than a slow one.
const gitTimeout = 30 * time.Second

func gitCommand(dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(errBuf.String()))
	}
	return out, nil
}

// Open prepares a checkout for grounding, verifying that the pinned commit is
// actually there.
//
// A checkout at the wrong commit is worse than none: it would report a citation
// as fabricated because a file moved between revisions, which is a claim about
// the arm made from a fact about the disk.
func Open(dir, commit string) (*Checkout, error) {
	return open(dir, commit, gitCommand)
}

func open(dir, commit string, git runner) (*Checkout, error) {
	if dir == "" || commit == "" {
		return nil, fmt.Errorf("grounding needs a directory and a commit, got %q and %q", dir, commit)
	}
	if _, err := git(dir, "cat-file", "-e", commit+"^{commit}"); err != nil {
		return nil, fmt.Errorf("the pinned commit is not in %s: %w", dir, err)
	}
	return &Checkout{dir: dir, commit: commit, git: git, lines: map[string]int{}}, nil
}

// lineCount returns how many lines path has at the pinned commit, and false
// when the file is not there.
func (c *Checkout) lineCount(path string) (int, bool) {
	if n, ok := c.lines[path]; ok {
		return n, n >= 0
	}
	out, err := c.git(c.dir, "show", c.commit+":"+path)
	if err != nil {
		c.lines[path] = -1
		return 0, false
	}
	n := bytes.Count(out, []byte{'\n'})
	// A last line with no trailing newline is still a line.
	if len(out) > 0 && !bytes.HasSuffix(out, []byte{'\n'}) {
		n++
	}
	c.lines[path] = n
	return n, true
}

// Report is what grounding found, and it is reported separately from recall
// rather than folded into it. "Wrong" and "absent" are different findings about
// an arm, and collapsing them loses the more interesting one: an arm that
// fabricates is failing differently from an arm that stays silent.
type Report struct {
	// Why says why nothing was checked, and empty means it was checked.
	//
	// "cannot check" is a first-class outcome, not an error. A score computed
	// without a checkout is valid and says grounding was not verified. What is
	// forbidden is silently treating unverified as verified.
	Why string
	// Checked is how many distinct locations were looked up.
	Checked int
	// Ungrounded lists the locations that are not there AS WRITTEN, in the
	// order cited.
	//
	// Not all of these are inventions. A path an arm wrote without its
	// directory — `category.rb:1090` for `app/models/category.rb` — does not
	// resolve either, and on the discourse fixture all three ungrounded
	// citations are that rather than fabrication. They are excluded from recall
	// regardless, because a citation that cannot be resolved cannot be checked,
	// and the strict path rule already refuses a bare basename.
	Ungrounded []string
}

// Verified reports whether grounding actually ran.
func (r Report) Verified() bool { return r.Why == "" }

// Check reports which of an answer's citations name somewhere that exists.
//
// It returns a report and NOTHING ELSE. An earlier version also returned the
// surviving citations, for the scorer to use in place of the full set, and that
// was wrong in a way worth writing down.
//
// It bought nothing. A citation is credited under the path rule only by
// carrying a gold row's exact path and line, and gold rows are hand-audited real
// locations, so anything scoring that way names a place that exists. The one
// rule that could credit an invention is the symbol rule, which does not compare
// the line — and that is precisely the citation this package cannot check,
// because it has no path.
//
// And it cost real hits. samePath deliberately credits a citation carrying MORE
// leading directories than gold, which is how an arm writing an absolute path is
// scored; `git show commit:/Users/.../app/x.rb` cannot resolve that, so the
// citation was dropped and the hit with it. There are 127 such citations in the
// recorded corpus, in both arms. That is form-dependent STRICTNESS, the mirror
// of the leniency this cycle already has a law about.
//
// So grounding moves the report and never the number, and recall is
// disk-independent by construction rather than by argument.
func Check(cites []score.Cite, c *Checkout) Report {
	if c == nil {
		return Report{Why: "no checkout available, so no citation was verified to exist"}
	}
	var r Report
	seen := map[string]bool{}

	for _, cite := range cites {
		path := cite.Path
		if path == "" {
			path = cite.Established
		}
		// A symbol with no file behind it names no path to resolve, and a
		// citation with no line names no location. Neither is counted as
		// fabricated: unresolvable is not the same as invented.
		if path == "" || cite.Line == 0 {
			continue
		}
		where := path + ":" + strconv.Itoa(cite.Line)
		if seen[where] {
			continue
		}
		seen[where] = true
		r.Checked++
		if n, ok := c.lineCount(path); !ok || cite.Line > n {
			r.Ungrounded = append(r.Ungrounded, where)
		}
	}
	return r
}

// CheckGold resolves the gold rows themselves, and is how a checkout proves it
// is the RIGHT checkout.
//
// A commit that exists in the wrong repository passes cat-file and then resolves
// nothing, so every citation reads as fabricated while the report says verified:
// a confident, wrong, verified-looking number. Gold rows have known locations at
// the pinned commit, so if they do not resolve, the pair is wrong and grounding
// refuses rather than publishing that.
//
// A gold row that stops resolving on the RIGHT checkout is a stale scenario — a
// row pointing at a line that moved is not gold, it is history — and that is a
// loud failure of the instrument rather than a quiet subtraction from an arm.
func CheckGold(locations []string, c *Checkout) error {
	if c == nil {
		return nil
	}
	var bad []string
	for _, loc := range locations {
		i := strings.LastIndex(loc, ":")
		if i <= 0 {
			continue
		}
		line, err := strconv.Atoi(loc[i+1:])
		if err != nil {
			continue
		}
		if n, ok := c.lineCount(loc[:i]); !ok || line > n {
			bad = append(bad, loc)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf("%d of %d gold locations do not resolve at %s in %s, so this is the wrong "+
		"checkout or the scenario is stale: %s", len(bad), len(locations), c.commit, c.dir, strings.Join(bad, ", "))
}

func (r Report) String() string {
	if !r.Verified() {
		return "grounding   NOT VERIFIED: " + r.Why + "\n"
	}
	if r.Checked == 0 {
		return "grounding   nothing to check: no citation named both a file and a line\n"
	}
	return fmt.Sprintf("grounding   %d of %d cited locations do not resolve at the pinned commit\n",
		len(r.Ungrounded), r.Checked)
}
