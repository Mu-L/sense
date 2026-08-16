package score

import (
	"regexp"
	"strconv"
	"strings"
)

// Cite is one location an answer pointed at.
//
// An answer does not write locations in one shape. Measured across the 236
// recorded transcript.json captures, inside backticks alone: 25,117 path:line,
// 11,368 bare symbols, 8,161 bare :line continuations, 4,206 paths with no
// line, 67 elided symbols, 41 symbol:line and 35 elided paths. A matcher that
// reads only the first of those is blind to the rest, and the blindness is
// invisible in the result because it can only ever lower a score.
//
// Path and Symbol are MUTUALLY EXCLUSIVE: a cite is one or the other, and which
// one decides how strictly it is judged. Established is separate for that
// reason — it is a file the answer had already named, carried alongside a
// symbol cite, and it can only ever ADD a way to match. Folding it into Path
// would mean that writing the path first turned a hit into a miss.
type Cite struct {
	// Path is repository-relative, and empty when the answer named a symbol.
	Path string
	// Symbol is Class::Nested#method, and empty when the answer named a path.
	Symbol string
	// Established is the file this symbol cite continues from, when the answer
	// had named it and the symbol's own name agrees. Empty otherwise.
	Established string
	// Line is 0 when the answer gave no line. A cite with no line is a mention,
	// never a hit.
	Line int
}

// extensions is the closed set of file suffixes a citation may end in.
//
// A shape test cannot do this job. "A dot and up to four lowercase letters" is
// also the shape of `Account.new`, `account.id` and `Net::HTTP.get`, and
// reading those as files poisons the elision antecedent AND inflates the
// printed count of files an answer named — measured at 16 of 152 on the
// mastodon fixture, about a tenth of that denominator.
//
// The first three are what the checked-in gold actually needs (rb 97, cs 30,
// rake 2). The rest are what answers were measured writing, plus the source
// extensions of the stacks this bench targets, so a new vertical does not
// silently lose citations.
var extensions = []string{
	"rb", "cs", "rake",
	"js", "json", "yml", "yaml", "md", "vue", "tsx", "haml", "eml", "ts",
	"liquid", "txt", "erb", "gjs", "jsx", "html", "css", "scss", "xml",
	"go", "py", "java", "kt", "rs", "php", "sql", "sh", "toml", "proto",
	"c", "h", "cpp", "hpp", "swift", "lock", "cfg", "conf",
}

// The token shapes. Every one is a form an arm demonstrably wrote in a recorded
// transcript; "it might write it this way" is how a matcher becomes satisfiable
// by the route, so a form with no precedent is not here.
//
// The pattern is the ONLY classifier. An earlier version re-sniffed each token
// with a set of predicates, and the two disagreed: the pattern called `Foo.bar`
// a symbol and the predicates called it a file. Which group fired is the answer
// now, and there is no second opinion to conflict with it.
var (
	extRe    = `\.(?:` + strings.Join(extensions, "|") + `)\b`
	pathRe   = `[A-Za-z0-9._/\-]+` + extRe
	classRe  = `[A-Z][A-Za-z0-9_]*(?:::[A-Z][A-Za-z0-9_]*)*`
	methodRe = `[#.][A-Za-z_][A-Za-z0-9_]*[?!]?`
	lineRe   = `(?::\d+)?`
	// The ellipsis an arm writes when it truncates a path or a namespace it
	// has already given in full: `…_measure.rb:20`, `…#allowed_categories_arel`.
	el = `(?:\x{2026}|\.\.\.)`

	token = regexp.MustCompile(
		`(?P<epath>` + el + `[A-Za-z0-9._/\-]*` + extRe + lineRe + `)` +
			`|(?P<esym>` + el + `(?:[A-Za-z0-9_:]*` + methodRe + `|[A-Z][A-Za-z0-9_:]*)` + lineRe + `)` +
			`|(?P<path>` + pathRe + lineRe + `)` +
			`|(?P<sym>` + classRe + methodRe + lineRe + `)` +
			`|(?P<meth>` + methodRe + lineRe + `)` +
			`|(?P<line>:\d+)` +
			// A continuation of the line just given: an arm enumerating several
			// lines in one file writes `X.cs:24,178` or `:143,171,450`. Measured
			// across the corpus these carry 4,333 line numbers that a matcher
			// without them never reads, and they fall HARDER on the baseline arm
			// (21.0 per transcript against 16.6), so ignoring them is a bias in
			// the opposite direction to the symbol rule.
			`|(?P<more>[,\-]\d+)`)

	groups = token.SubexpNames()
)

// Scan returns every location the answer pointed at, in the order it wrote
// them, with elided citations resolved against what came before.
//
// Resolving elision is stateful: the meaning of a citation depends on the one
// before it. When an item is in the same file as the item above, an arm writes
// the line alone:
//
//	`Jobs::ReindexSearch#rebuild_categories` — `app/jobs/…/reindex_search.rb:22`
//	`Jobs::ReindexSearch#load_problem_category_ids` — `:111`
//
// Measured on the checked-in discourse fixture, resolving these takes the
// answer from 254 distinct locations to 331, and loses none of the 254. That
// gain is pinned by a test rather than quoted, because the figure inherited
// from cycle 00 was an estimate from a regex count and it was wrong.
//
// One elided form is deliberately NOT resolved: `…/source.rb:12`, where the
// ellipsis stands for a DIRECTORY prefix rather than the head of a filename.
// After `app/models/account_suggestions.rb` it could mean that file's
// directory or its sibling concern directory, and choosing between them needs
// the checkout — 02-04. It is dropped rather than guessed at. Two citations on
// the mastodon fixture take this form.
//
// A continuation binds to the citation IMMEDIATELY before it and to nothing
// else. An earlier version let a bare `:41` attach to the last file named at any
// distance, which turned "the job runs at 10:30" and "bound to 1.2.3.4:8080"
// into strict citations in whatever file happened to be named last — fabricated
// locations that could land on a gold line by coincidence. Adjacency is the
// bound, and it is the one the form itself justifies.
func Scan(answer string) []Cite {
	var s scanner
	prevEnd := -1
	for _, m := range token.FindAllStringSubmatchIndex(answer, -1) {
		for i := 2; i < len(m); i += 2 {
			if m[i] < 0 {
				continue
			}
			kind, tok := groups[i/2], answer[m[i]:m[i+1]]
			// A continuation only continues something ADJACENT. Without that,
			// the "1,000" in prose would extend whatever file was named last.
			if kind != "more" || m[0] == prevEnd {
				s.token(kind, tok)
			}
			break
		}
		prevEnd = m[1]
	}
	return s.out
}

// scanner carries what later citations resolve against. All of the state lives
// here rather than half in a helper's pointer arguments and half in the caller
// reaching back into the slice it is building.
type scanner struct {
	lastPath   string
	lastSymbol string
	out        []Cite
}

func (s *scanner) token(kind, t string) {
	switch kind {
	case "more":
		s.more(t)
	case "line":
		s.line(t)
	case "path":
		body, line := splitLine(t)
		s.lastPath = body
		s.pinPrecedingSymbol(body, line)
		s.out = append(s.out, Cite{Path: body, Line: line})
	case "sym":
		body, line := splitLine(t)
		s.symbol(body, line)
	case "meth":
		s.method(t)
	case "epath":
		s.elidedPath(t)
	case "esym":
		body, line := splitLine(t)
		s.elidedSymbol(trimEllipsis(body), line)
	}
}

// more continues the line just given: `:24,178` is two locations in one file,
// and `:59-60` is a span whose ENDS the answer named.
//
// The interior of a span is deliberately not credited. 37 otherwise-missed gold
// rows sit strictly inside a cited range, and crediting them would be a
// tolerance window adopted because it raised a number, which is the one way
// this matcher must not move.
func (s *scanner) more(t string) {
	n, err := strconv.Atoi(t[1:])
	if err != nil || len(s.out) == 0 {
		return
	}
	prev := s.out[len(s.out)-1]
	if prev.Line == 0 {
		return
	}
	prev.Line = n
	s.out = append(s.out, prev)
}

// line completes the citation just written when that one is still missing its
// line: `Sym` `:7` is one location in two tokens. It attaches to nothing else.
func (s *scanner) line(t string) {
	n, err := strconv.Atoi(t[1:])
	if err != nil {
		return
	}
	if i := len(s.out); i > 0 && s.out[i-1].Line == 0 {
		s.out[i-1].Line = n
	}
}

// method is a method with its class elided, written when the arm has just named
// the class: `…AccountsController#create` `:17`/`#destroy` `:22`.
func (s *scanner) method(t string) {
	body, line := splitLine(t)
	if cls, _, ok := strings.Cut(s.lastSymbol, "#"); ok && cls != "" {
		s.symbol(cls+"#"+strings.TrimLeft(body, "#."), line)
	}
}

// elidedPath resolves a truncated path against the last full one. The tail must
// be a tail of it at a boundary: a bare suffix is the shape that lets
// spec/x.rb pin app/x.rb, which is what the boundary rule exists to stop.
func (s *scanner) elidedPath(t string) {
	body, line := splitLine(t)
	if hasTail(s.lastPath, trimEllipsis(body)) {
		s.out = append(s.out, Cite{Path: s.lastPath, Line: line})
	}
}

// pinPrecedingSymbol gives a line to the symbol this path belongs to.
//
// The winning question in this bench asks the agent to "name the routine the
// dependency is inside, give its file:line", and the answer to it is written as
// one item in two tokens:
//
//	`SeedData::Categories#create_category`, `lib/seed_data/categories.rb:116`
//
// Read as two citations the routine is lost: the symbol carries no line, so it
// can never match, and the path is held to the gold line. Measured against the
// recorded corpus that cost 18 rows on one run alone, every one of them a case
// where the arm named the right routine and pinned its definition line while
// gold cites a use inside it.
//
// The pairing is CORROBORATED, exactly as an inherited file is: the symbol has
// to name this path under Ruby's own rule. An unrelated constant next to an
// unrelated path stays two citations, so this cannot merge things that merely
// sit beside each other.
func (s *scanner) pinPrecedingSymbol(path string, line int) {
	i := len(s.out) - 1
	if line == 0 || i < 0 {
		return
	}
	prev := &s.out[i]
	if prev.Symbol == "" || prev.Line != 0 || !symbolNamesPath(prev.Symbol, path) {
		return
	}
	prev.Line = line
	prev.Established = path
}

func (s *scanner) symbol(sym string, line int) {
	s.lastSymbol = sym
	s.out = append(s.out, Cite{Symbol: sym, Established: corroborated(sym, s.lastPath), Line: line})
}

// elidedSymbol resolves a truncated constant against the last full one.
func (s *scanner) elidedSymbol(tail string, line int) {
	if tail == "" || s.lastSymbol == "" {
		return
	}
	if strings.HasSuffix(s.lastSymbol, tail) {
		s.out = append(s.out, Cite{Symbol: s.lastSymbol, Established: corroborated(s.lastSymbol, s.lastPath), Line: line})
		return
	}
	// The namespace was elided but the rest differs from the last one:
	// `Api::V1::Statuses::FavouritedByAccountsController#default_accounts`
	// then `…RebloggedByAccountsController#default_accounts`. Rebuild it from
	// the last symbol's namespace.
	if ns := namespaceOf(s.lastSymbol); ns != "" {
		s.symbol(ns+"::"+tail, line)
	}
}

// namespaceOf returns everything before the last :: of a symbol's class part.
func namespaceOf(sym string) string {
	cls, _, _ := strings.Cut(sym, "#")
	i := strings.LastIndex(cls, "::")
	if i < 0 {
		return ""
	}
	return cls[:i]
}

// trimEllipsis removes the leading … or ... and nothing else. Trimming a rune
// SET would also eat the leading dots of whatever follows it.
func trimEllipsis(s string) string {
	for _, p := range []string{"…", "..."} {
		if rest, ok := strings.CutPrefix(s, p); ok {
			return rest
		}
	}
	return s
}

// hasTail reports whether long ends with tail at a boundary, so a truncation
// lands on a whole path segment or word rather than mid-identifier.
func hasTail(long, tail string) bool {
	if tail == "" || long == "" || !strings.HasSuffix(long, tail) || len(long) <= len(tail) {
		return false
	}
	// The truncation has to land on a separator, either just before the tail
	// (`…/source.rb` of `app/models/source.rb`) or at its own front
	// (`…_measure.rb` of `instance_accounts_measure.rb`, which is the form
	// mastodon actually wrote). What is refused is a cut mid-identifier, which
	// is how `…ategory.rb` would resolve against anything ending in those
	// letters.
	return isSep(long[len(long)-len(tail)-1]) || isSep(tail[0])
}

func isSep(c byte) bool { return c == '/' || c == '_' || c == '-' || c == '.' }

func splitLine(t string) (body string, line int) {
	i := strings.LastIndex(t, ":")
	if i <= 0 || i == len(t)-1 || t[i-1] == ':' {
		return t, 0
	}
	n, err := strconv.Atoi(t[i+1:])
	if err != nil {
		return t, 0
	}
	return t[:i], n
}
