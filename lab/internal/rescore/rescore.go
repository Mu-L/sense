// Package rescore recomputes every recorded score and accounts for the
// differences.
//
// It answers the question the whole rewrite rests on: does the method survive
// re-expression in Go? Not by reproducing the old numbers — the corpus was not
// scored under one scorer, and reproducing every recorded number exactly would
// mean reproducing a defect that was already fixed. Parity is a diagnostic, and
// the output that closes the work is not "zero differences" but "zero
// unexplained differences".
package rescore

import (
	"fmt"
	"sort"
	"strings"

	"github.com/luuuc/sense/lab/internal/score"
)

// Cause is why one recomputed score differs from the one on disk.
//
// The set is CLOSED and was decided before the table was read, so nobody gets
// to invent a category that makes a difference comfortable.
type Cause string

const (
	// ScorerVersion means the recorded score predates a fix that is now
	// standard. The symbol-oracle fix only ever ADDED credit, so a run under
	// this cause must rescore HIGHER; one that rescores lower is a new defect
	// wearing the wrong label, and Classify refuses to call it this.
	ScorerVersion Cause = "scorer version"
	// WrongLine means the old scorer gave a row to a citation naming the right
	// file at a line gold does not name. It is an OLD DEFECT and a finding: the
	// right file at the wrong line is a teammate sent to the wrong place, which
	// is the thing this metric exists to measure.
	WrongLine Cause = "old defect: the right file at a line gold does not name"
	// NeverLocated means the old scorer gave a row to an answer that never
	// located the file in any form. Also an old defect, and a starker one.
	NeverLocated Cause = "old defect: a row the answer never located"
	// GoldChange means rows were quarantined or corrected, so a different
	// question is being answered. 02-05 hands over the affected runs in
	// advance, so this cause is checked against a prediction.
	GoldChange Cause = "gold change"
	// NewDefect means the new scorer is wrong here. It BLOCKS the cycle.
	NewDefect Cause = "new defect"
	// Unexplained is not a cause. Anything here means the work continues.
	Unexplained Cause = "unexplained"
)

// Row is one recomputed group-score set beside the one on disk.
type Row struct {
	Repo, Model, Arm, Run, Group string
	Recorded, Now, Total         int
	// PreFix says the recorded score predates the symbol-oracle fix, taken from
	// the list written down before this comparison ran.
	PreFix bool
	// Quarantined says 02-05 removed rows from this group.
	Quarantined bool
	// FileLevel and Uncited split the rows the old scorer credited and the new
	// one does not, by whether the answer located the file at all.
	FileLevel, Uncited int
}

// Diff is the difference, positive when the new score is higher.
func (r Row) Diff() int { return r.Now - r.Recorded }

// Classify names the cause of one row's difference.
//
// The direction test comes FIRST, and it is what stops the comfortable label
// being reached for: a pre-fix run that rescores LOWER cannot be a scorer
// version difference, because that fix only ever added credit.
func Classify(r Row) Cause {
	switch {
	case r.Diff() == 0:
		return ""
	case r.Quarantined:
		return GoldChange
	case r.Diff() > 0:
		if r.PreFix {
			return ScorerVersion
		}
		return Unexplained
	case r.Uncited > r.FileLevel:
		return NeverLocated
	case r.FileLevel > 0:
		return WrongLine
	default:
		return Unexplained
	}
}

// Report is the whole comparison.
type Report struct {
	Rows []Row
}

// Distribution is how many group-scores differ, and by how much.
//
// It is read BEFORE the table: four differing rows is an afternoon of reading,
// forty is a structural problem, and the shape says which before anyone starts
// going row by row.
func (rp Report) Distribution() map[int]int {
	d := map[int]int{}
	for _, r := range rp.Rows {
		d[r.Diff()]++
	}
	return d
}

// Causes counts the rows under each cause.
func (rp Report) Causes() map[Cause]int {
	c := map[Cause]int{}
	for _, r := range rp.Rows {
		if cause := Classify(r); cause != "" {
			c[cause]++
		}
	}
	return c
}

// Unexplained is what keeps the cycle open. Zero of these closes the pitch;
// anything else means the work continues.
func (rp Report) Unexplained() []Row {
	var out []Row
	for _, r := range rp.Rows {
		if Classify(r) == Unexplained {
			out = append(out, r)
		}
	}
	return out
}

// Differing counts the rows whose score moved.
func (rp Report) Differing() int {
	n := 0
	for _, r := range rp.Rows {
		if r.Diff() != 0 {
			n++
		}
	}
	return n
}

// CheckpointFired reports whether more than a third of the rows differ, which
// is the structural signal to stop and look at the shape rather than work the
// queue. It is scheduled in advance rather than reached at the moment of
// fatigue, which is the whole point of having it.
func (rp Report) CheckpointFired() bool {
	return len(rp.Rows) > 0 && float64(rp.Differing()) > float64(len(rp.Rows))/3
}

// Split counts how the old scorer's extra credits break down for one group.
//
// The split IS the finding: a credit for the right file at the wrong line is a
// different claim about the old instrument from a credit for a row the answer
// never located.
func Split(cites []score.Cite, gold []score.Row, oldCited map[string]bool) (fileLevel, uncited int) {
	for _, g := range gold {
		if !oldCited[g.ID] || g.Cite == "" {
			continue
		}
		i := strings.LastIndex(g.Cite, ":")
		if i <= 0 {
			continue
		}
		path, line := g.Cite[:i], g.Cite[i+1:]
		if anyMatches(cites, path, line) {
			continue
		}
		if anyNames(cites, path) || mentionsBasename(cites, path) {
			fileLevel++
			continue
		}
		uncited++
	}
	return fileLevel, uncited
}

func anyMatches(cites []score.Cite, path, line string) bool {
	for _, c := range cites {
		if score.Matches(c, path, line) {
			return true
		}
	}
	return false
}

func anyNames(cites []score.Cite, path string) bool {
	for _, c := range cites {
		if score.NamesFile(c, path) {
			return true
		}
	}
	return false
}

// mentionsBasename catches a file the answer named by its bare basename.
//
// The matcher refuses a bare basename on purpose, because many files share a
// name and crediting one would land on a gold line by coincidence. For
// ACCOUNTING the question is different: did the old scorer credit a file the
// answer had named in some form? It nearly always had — `OrganizationService.cs:36`
// against gold at :835 — and filing that under "never located" would overstate
// how badly the old instrument behaved.
func mentionsBasename(cites []score.Cite, path string) bool {
	base := basename(path)
	for _, c := range cites {
		where := c.Path
		if where == "" {
			where = c.Established
		}
		if where != "" && basename(where) == base {
			return true
		}
	}
	return false
}

func basename(p string) string { return p[strings.LastIndex(p, "/")+1:] }

func sortedCauses(m map[Cause]int) []Cause {
	out := make([]string, 0, len(m))
	for c := range m {
		out = append(out, string(c))
	}
	sort.Strings(out)
	cs := make([]Cause, len(out))
	for i, s := range out {
		cs[i] = Cause(s)
	}
	return cs
}

// String renders the distribution, then the causes, then what is unexplained.
func (rp Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "group-scores compared %d\n  identical %d\n  different %d\n",
		len(rp.Rows), len(rp.Rows)-rp.Differing(), rp.Differing())
	if rp.CheckpointFired() {
		b.WriteString("\nCHECKPOINT: more than a third differ. Hold it before reading row by row.\n")
	}

	b.WriteString("\ndifference, in rows cited (new minus recorded):\n")
	dist := rp.Distribution()
	keys := make([]int, 0, len(dist))
	for k := range dist {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %+4d  %4d\n", k, dist[k])
	}

	b.WriteString("\ncause of every difference:\n")
	causes := rp.Causes()
	names := make([]string, 0, len(causes))
	for c := range causes {
		names = append(names, string(c))
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(&b, "  %4d  %s\n", causes[Cause(n)], n)
	}
	// Per repo as well as in total, because a cause that is concentrated in one
	// repository is a different finding from one spread across all of them —
	// and because `gold change` has to be checked against 02-05's handover
	// list, which is a list of runs in one repository.
	b.WriteString("\ncause by repo:\n")
	byRepo := map[string]map[Cause]int{}
	for _, r := range rp.Rows {
		cause := Classify(r)
		if cause == "" {
			continue
		}
		if byRepo[r.Repo] == nil {
			byRepo[r.Repo] = map[Cause]int{}
		}
		byRepo[r.Repo][cause]++
	}
	repos := make([]string, 0, len(byRepo))
	for k := range byRepo {
		repos = append(repos, k)
	}
	sort.Strings(repos)
	for _, repo := range repos {
		for _, n := range sortedCauses(byRepo[repo]) {
			fmt.Fprintf(&b, "  %-18s %4d  %s\n", repo, byRepo[repo][n], n)
		}
	}

	if n := len(rp.Unexplained()); n > 0 {
		fmt.Fprintf(&b, "\n%d UNEXPLAINED. The cycle stays open.\n", n)
	}
	return b.String()
}
