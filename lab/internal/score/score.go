// Package score turns a captured session into a number.
//
// It pulls path:line citations out of the agent's answer and matches them
// against the rows of one gold group, strictly. It is pure: it takes bytes and
// returns data, touches nothing, and is the same function applied to a live run
// or a recorded fixture.
//
// This is the walking skeleton's scorer and it is deliberately the simplest
// thing that produces a number. Cycle 02 builds the real matcher against the
// whole recorded corpus.
package score

import (
	"fmt"
	"regexp"
	"strings"
)

// Row is one gold row: what a good answer has to cite, and where.
type Row struct {
	ID string
	// Cite is the row's authoritative path:line, repository-relative.
	Cite string
}

// Result is one gold group scored against one transcript.
type Result struct {
	Group string
	Total int
	Cited int
	// Reached is how many gold rows the answer merely MENTIONED THE FILE of, at
	// any line. It is not part of the score and must never become one: crediting
	// a file mention is the path-only matching this scorer exists to refuse.
	//
	// It is reported because without it a low score cannot be read at all:
	// "found two of the twelve places" and "named eleven of the twelve files and
	// landed on two of the lines" are different results, and on a real run they
	// were the same number.
	//
	// It is reported WITH FilesNamed because it is volume-sensitive in a way the
	// score is not. A long answer mentions more files and buys reach for free; a
	// terse one cannot. Comparing reach across two arms of different lengths,
	// without the denominator, would be comparing verbosity.
	Reached int
	// FilesNamed is how many distinct files the answer cited anywhere. It is the
	// denominator of opportunity for Reached, and nothing else.
	FilesNamed int
	Recall     float64
	Hits       []string // gold row IDs that were cited
	Misses     []string // gold row IDs that were not
	Floor      float64
	Verdict    string
}

// Verdicts. One comparison against one floor: the verdict vocabulary, the
// spread rule and the retry rule are cycle 04.
const (
	AtOrAboveFloor = "at or above floor"
	BelowFloor     = "below floor"
)

// CitationPattern matches a repository-relative path:line as an agent writes
// one. The path must carry an extension, so prose like "step 2:12" is not a
// citation, and a line number is required, because a path-only mention is
// exactly what this scorer refuses to credit.
//
// It is exported because gold rows carry their location in the same shape, and
// the two sides MUST agree: a shape gold accepts that the answer side does not
// would leave rows silently unmatchable.
const CitationPattern = `[A-Za-z0-9._/\-]+\.[A-Za-z][A-Za-z0-9]*:\d+`

var citation = regexp.MustCompile(CitationPattern)

// Citations returns every distinct path:line found in text, in the order they
// first appear. Markdown emphasis and backticks around a citation are stripped
// by the character class, so a cite inside `code` is found the same as a bare
// one.
func Citations(text string) []string {
	var out []string
	seen := map[string]bool{}
	for _, c := range citation.FindAllString(text, -1) {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	return out
}

// Group scores one gold group against the citations in text.
//
// Matching is strict: a citation counts for a row only when it names the same
// line of the same file. There is no fuzzy matching, no path-only fallback and
// no partial credit — the right file at the wrong line is a teammate sent to
// the wrong place, which is the thing the metric exists to measure.
//
// A gold path is repository-relative and an agent may cite it with a leading
// directory the repository does not have, so a citation matches when it ends at
// a path boundary of the gold cite. That is a suffix rule on the PATH only; the
// line still has to be exactly right.
func Group(name string, rows []Row, text string, floor float64) Result {
	cites := Citations(text)
	r := Result{Group: name, Total: len(rows), Floor: floor}

	for _, row := range rows {
		if ReachedPath(cites, row.Cite) {
			r.Reached++
		}
		if anyMatches(cites, row.Cite) {
			r.Hits = append(r.Hits, row.ID)
		} else {
			r.Misses = append(r.Misses, row.ID)
		}
	}

	r.FilesNamed = distinctFiles(cites)
	r.Cited = len(r.Hits)
	if r.Total > 0 {
		r.Recall = float64(r.Cited) / float64(r.Total)
	}
	r.Verdict = BelowFloor
	if r.Recall >= floor {
		r.Verdict = AtOrAboveFloor
	}
	// Hits and misses stay in gold file order rather than sorted by id. The
	// order the rows were written in is the order a human reads them back in.
	return r
}

// distinctFiles counts the separate files a set of citations names.
func distinctFiles(cites []string) int {
	seen := map[string]bool{}
	for _, c := range cites {
		if p, _, ok := split(c); ok {
			seen[p] = true
		}
	}
	return len(seen)
}

// ReachedPath reports whether any citation names the gold row's FILE, at any
// line. It is not part of the score — strict matching is — but it separates
// "never found the place" from "found the file and pointed at the wrong line",
// and those are different failures needing different fixes.
func ReachedPath(cites []string, gold string) bool {
	goldPath, _, ok := split(gold)
	if !ok {
		return false
	}
	for _, c := range cites {
		if p, _, ok := split(c); ok && samePath(p, goldPath) {
			return true
		}
	}
	return false
}

// LinesCitedFor returns every line the answer attached to the gold row's file,
// in the order they appear. It exists so a miss can be read rather than merely
// counted.
func LinesCitedFor(cites []string, gold string) []string {
	goldPath, _, ok := split(gold)
	if !ok {
		return nil
	}
	var out []string
	for _, c := range cites {
		if p, l, ok := split(c); ok && samePath(p, goldPath) {
			out = append(out, l)
		}
	}
	return out
}

// anyMatches reports whether any citation names the gold row's location.
func anyMatches(cites []string, gold string) bool {
	goldPath, goldLine, ok := split(gold)
	if !ok {
		return false
	}
	for _, c := range cites {
		p, l, ok := split(c)
		if ok && l == goldLine && samePath(p, goldPath) {
			return true
		}
	}
	return false
}

// split separates a path:line into its two halves.
//
// The guard is defensive rather than covered. Every citation reaching it comes
// from CitationPattern and always has both halves; what genuinely reaches it is
// a gold row whose Cite is empty, and the callers must not treat that as a
// location. A malformed ":12" or "a.rb:" is not producible from either side
// today, and the guard is kept because a caller passing an unvalidated string
// would otherwise index out of a path that is not there.
func split(cite string) (path, line string, ok bool) {
	i := strings.LastIndex(cite, ":")
	if i <= 0 || i == len(cite)-1 {
		return "", "", false
	}
	return cite[:i], cite[i+1:], true
}

// samePath reports whether a cited path names the gold path.
//
// Gold is repository-relative and an agent may write MORE leading directories
// than gold has, so a citation counts when gold is a suffix of it at a path
// boundary: "discourse/script/import_scripts/base.rb" names
// "script/import_scripts/base.rb", and "my_base.rb" never names "base.rb".
//
// The other direction is deliberately refused. A citation SHORTER than gold is
// a bare basename, and crediting it is the path-only matching this package
// exists to reject: discourse has many files called category.rb, so a bare
// "category.rb:1083" would credit a row about a different one on a coincidence
// of line number.
func samePath(cited, gold string) bool {
	return cited == gold || hasPathSuffix(cited, gold)
}

func hasPathSuffix(long, short string) bool {
	return len(long) > len(short) &&
		strings.HasSuffix(long, short) &&
		long[len(long)-len(short)-1] == '/'
}

// String renders the result the way the skeleton prints it.
func (r Result) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "gold group %s\n", r.Group)
	fmt.Fprintf(&b, "cited      %d of %d\n", r.Cited, r.Total)
	fmt.Fprintf(&b, "mentioned  %d of %d gold files, at any line, out of %d files named (not a score)\n",
		r.Reached, r.Total, r.FilesNamed)
	fmt.Fprintf(&b, "recall     %.2f\n", r.Recall)
	fmt.Fprintf(&b, "floor      %.2f\n", r.Floor)
	fmt.Fprintf(&b, "verdict    %s\n", r.Verdict)
	if len(r.Misses) > 0 {
		fmt.Fprintf(&b, "missed     %s\n", strings.Join(r.Misses, ", "))
	}
	return b.String()
}
