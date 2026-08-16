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
	// Why says what makes this number provisional, and empty means it is not.
	// It travels from the transcript because a score that does not carry the
	// mark is exactly the failure the mark exists to prevent: a truncated
	// capture read as a weak arm.
	Why   string
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
	// SymbolCited is how many of the Hits were credited ONLY by the symbol
	// rule, whose line is required but not compared.
	//
	// It is printed beside the score because on a real transcript it was ALL of
	// it: the banked mastodon cell scores 8 of 20, and all 8 arrive this way,
	// with no path citation landing on a gold line at all. A number composed
	// entirely of line-unchecked matches is a different claim from one composed
	// of strict ones, and reading them as the same is how an arm that writes
	// symbols quietly out-scores an arm that writes paths to the same places.
	//
	// The range check that closes this needs a repository on disk and is 02-04.
	// Until then the composition travels with the number.
	SymbolCited int
	Recall      float64
	Hits        []string // gold row IDs that were cited
	Misses      []string // gold row IDs that were not
	Floor       float64
	Verdict     string
}

// Source is what a score is taken from. It is an interface so this package does
// not depend on the transcript package, and so a test can score a string
// without pretending to be a capture.
type Source interface {
	// Answer is everything the agent said.
	Answer() string
	// ProvisionalWhy says why the capture is incomplete, or "" when it is not.
	ProvisionalWhy() string
}

// text is a Source for prose with no capture behind it.
//
// It is UNEXPORTED on purpose. It reports not-provisional unconditionally, so
// handing it a real capture's answer launders the mark: `text(tr.Answer())`
// would compile, score clean, and read as deliberate. Group takes a Source
// precisely so the transcript itself goes in, and an exported always-clean
// Source is a door in that wall. Tests inside this package use it; nothing
// outside can.
type text string

func (t text) Answer() string         { return string(t) }
func (t text) ProvisionalWhy() string { return "" }

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

// Provisional means this number came from a transcript known to be incomplete.
func (r Result) Provisional() bool { return r.Why != "" }

// Group scores one gold group against a transcript.
//
// It takes the transcript rather than its text so the provisional mark cannot
// be dropped on the way in. A mark that every caller has to remember to copy
// across is an annotation, and an annotation is what this refuses to be.
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
func Group(name string, rows []Row, tr Source, floor float64) Result {
	cites := Scan(tr.Answer())
	r := Result{Group: name, Total: len(rows), Floor: floor, Why: tr.ProvisionalWhy()}

	for _, row := range rows {
		p, _, ok := split(row.Cite)
		if !ok {
			r.Misses = append(r.Misses, row.ID)
			continue
		}
		if anyNames(cites, p) {
			r.Reached++
		}
		switch how := howMatched(cites, row.Cite); how {
		case matchedNone:
			r.Misses = append(r.Misses, row.ID)
		default:
			r.Hits = append(r.Hits, row.ID)
			if how == matchedSymbol {
				r.SymbolCited++
			}
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
func distinctFiles(cites []Cite) int {
	seen := map[string]bool{}
	for _, c := range cites {
		if c.Path != "" {
			seen[c.Path] = true
		}
	}
	return len(seen)
}

// anyNames reports whether any citation names the gold row's FILE, at any line.
// It is not part of the score — strict matching is — but it separates "never
// found the place" from "found the file and pointed at the wrong line", and
// those are different failures needing different fixes.
func anyNames(cites []Cite, goldPath string) bool {
	for _, c := range cites {
		if NamesFile(c, goldPath) {
			return true
		}
	}
	return false
}

// anyMatches reports whether any citation names the gold row's location.
func anyMatches(cites []Cite, gold string) bool {
	return howMatched(cites, gold) != matchedNone
}

// How a row was matched. The distinction is not cosmetic: a path match is
// checked against the gold LINE and a symbol match is not.
const (
	matchedNone = iota
	matchedPath
	matchedSymbol
)

// howMatched reports the STRONGEST way any citation matches the row, so that a
// row cited both ways is not counted as a loose one.
func howMatched(cites []Cite, gold string) int {
	goldPath, goldLine, ok := split(gold)
	if !ok {
		return matchedNone
	}
	best := matchedNone
	for _, c := range cites {
		if !Matches(c, goldPath, goldLine) {
			continue
		}
		if c.Path != "" && itoa(c.Line) == goldLine {
			return matchedPath
		}
		best = matchedSymbol
	}
	return best
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
	if r.Provisional() {
		fmt.Fprintf(&b, "PROVISIONAL  %s\n", r.Why)
		b.WriteString("             this number is not a result; the capture is incomplete\n")
	}
	fmt.Fprintf(&b, "gold group %s\n", r.Group)
	fmt.Fprintf(&b, "cited      %d of %d\n", r.Cited, r.Total)
	if r.SymbolCited > 0 {
		fmt.Fprintf(&b, "           %d of those on a symbol whose line is not checked (see 02-04)\n", r.SymbolCited)
	}
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

// ReachedPath reports whether any citation names the gold row's file, at any
// line.
func ReachedPath(cites []Cite, gold string) bool {
	p, _, ok := split(gold)
	return ok && anyNames(cites, p)
}

// LinesCitedFor returns every line the answer attached to the gold row's file,
// in the order they appear. It exists so a miss can be read rather than merely
// counted: "found the file and pointed at 465 where gold says 467" is a
// different failure from "never named the file".
func LinesCitedFor(cites []Cite, gold string) []string {
	goldPath, _, ok := split(gold)
	if !ok {
		return nil
	}
	var out []string
	for _, c := range cites {
		if c.Line != 0 && NamesFile(c, goldPath) {
			out = append(out, itoa(c.Line))
		}
	}
	return out
}
