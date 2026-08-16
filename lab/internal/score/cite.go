package score

import (
	"regexp"
	"strconv"
	"strings"
)

// Cite is one location an answer pointed at.
//
// An answer does not write locations in one shape. Measured across the 236
// recorded transcripts, inside backticks alone: 25,117 path:line, 11,368 bare
// symbols, 8,161 bare :line continuations, 4,206 paths with no line, 67 elided
// symbols, 41 symbol:line and 35 elided paths. A matcher that reads only the
// first of those is blind to the rest, and the blindness is invisible in the
// result because it can only ever lower a score.
type Cite struct {
	// Path is repository-relative, and empty when the answer named only a
	// symbol.
	Path string
	// Symbol is Class::Nested#method, and empty when the answer named only a
	// path.
	Symbol string
	// Line is 0 when the answer gave no line. A cite with no line is a mention,
	// never a hit.
	Line int
}

// The token shapes, in the order they are tried. Order matters: path:line has
// to win over path, and symbol:line over symbol, or the longer form is read as
// the shorter one with trailing text.
//
// Every one of these is a form an arm demonstrably wrote in a recorded
// transcript. None is speculative — "it might write it this way" is how a
// matcher becomes satisfiable by the route.
const (
	pathRe   = `[A-Za-z0-9._/\-]+\.[A-Za-z][A-Za-z0-9]*`
	symbolRe = `[A-Z][A-Za-z0-9_]*(?:::[A-Z][A-Za-z0-9_]*)*[#.][A-Za-z_][A-Za-z0-9_]*[?!]?`
	// The ellipsis an arm writes when it truncates a path or a namespace it has
	// already given in full: `…_measure.rb:20`, `…#allowed_categories_arel`.
	elision = `(?:\x{2026}|\.\.\.)`
)

var token = regexp.MustCompile(
	`(?:` + pathRe + `:\d+` + // 1. path with a line
		`|` + symbolRe + `:\d+` + // 2. symbol with a line
		`|` + elision + `[A-Za-z0-9._/\-]*\.[A-Za-z][A-Za-z0-9]*(?::\d+)?` + // 3. elided path
		`|` + elision + `[A-Za-z0-9_:]*[#.][A-Za-z_][A-Za-z0-9_]*[?!]?` + // 4. elided symbol
		`|` + symbolRe + // 5. bare symbol
		`|` + pathRe + // 6. bare path
		`|[#.][A-Za-z_][A-Za-z0-9_]*[?!]?` + // 7. method with the class elided
		`|:\d+` + // 8. a line on its own
		`)`)

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
// answer from 254 distinct locations to 319. A matcher that ignores them is
// blind to a quarter of what the answer actually cited, and it already changed
// a conclusion once: a row recorded as a genuine miss was cited at :111 by
// exactly this form.
//
// The antecedent is always the nearest preceding citation, with no distance
// bound, because that is what the elision means: the arm is continuing from the
// last place it named. A bound in characters would be a number chosen to make a
// corpus agree, which is the tuning this matcher must not do.
func Scan(answer string) []Cite {
	var out []Cite
	var lastPath, lastSymbol string

	for _, t := range token.FindAllString(answer, -1) {
		c, ok := resolve(t, &lastPath, &lastSymbol)
		if !ok {
			continue
		}
		// A line on its own attaches to the citation just written when that one
		// is still missing its line: `Sym` `:7` is one location written in two
		// tokens, not a symbol and then a stray number.
		if c.Path == "" && c.Symbol == "" {
			if n := len(out); n > 0 && out[n-1].Line == 0 {
				out[n-1].Line = c.Line
				continue
			}
			c.Path, c.Symbol = lastPath, lastSymbol
			if c.Path == "" && c.Symbol == "" {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// resolve turns one token into a Cite, updating what later elisions resolve
// against.
func resolve(t string, lastPath, lastSymbol *string) (Cite, bool) {
	body, line := splitLine(t)

	switch {
	case body == "": // a line on its own; the caller attaches it
		return Cite{Line: line}, line > 0

	case strings.HasPrefix(body, "…"), strings.HasPrefix(body, "..."):
		return elided(strings.TrimLeft(body, "…."), line, lastPath, lastSymbol)

	case isMethodOnly(body):
		// A method with its class elided, written when the arm has just named
		// the class: `…AccountsController#create` `:17`/`#destroy` `:22`.
		cls, _, ok := strings.Cut(*lastSymbol, "#")
		if !ok || cls == "" {
			return Cite{}, false
		}
		return Cite{Symbol: cls + "#" + strings.TrimLeft(body, "#."), Line: line}, true

	case isSymbol(body):
		*lastSymbol = body
		return Cite{Symbol: body, Path: corroborated(body, *lastPath), Line: line}, true

	default:
		*lastPath = body
		return Cite{Path: body, Line: line}, true
	}
}

// elided resolves a truncated path or symbol against the last full one.
//
// The tail must actually be a tail of what came before. An ellipsis that
// resolves against nothing is dropped rather than kept as a bare suffix,
// because a bare suffix is exactly the shape that lets `spec/x.rb` pin
// `app/x.rb` — the hazard the path-boundary rule exists to stop.
func elided(tail string, line int, lastPath, lastSymbol *string) (Cite, bool) {
	if tail == "" {
		return Cite{}, false
	}
	if strings.ContainsAny(tail, "#") || isSymbolTail(tail) {
		if strings.HasSuffix(*lastSymbol, tail) {
			return Cite{Symbol: *lastSymbol, Line: line}, true
		}
		// The namespace was elided but the method differs from the last one:
		// `Api::V1::Statuses::FavouritedByAccountsController#default_accounts`
		// then `…RebloggedByAccountsController#default_accounts`. Rebuild it
		// from the last symbol's namespace.
		if ns := namespaceOf(*lastSymbol); ns != "" {
			return Cite{Symbol: ns + "::" + tail, Line: line}, true
		}
		return Cite{}, false
	}
	if strings.HasSuffix(*lastPath, tail) {
		return Cite{Path: *lastPath, Line: line}, true
	}
	return Cite{}, false
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

func isSymbol(s string) bool {
	i := strings.IndexAny(s, "#.")
	if i <= 0 {
		return false
	}
	// A path always carries a dot before an extension, and a symbol's dot is a
	// method call, so the two are told apart by what follows the separator and
	// by the capital that opens a constant.
	return s[0] >= 'A' && s[0] <= 'Z' && !strings.Contains(s, "/") &&
		(s[i] == '#' || !isExtension(s[i+1:]))
}

// isMethodOnly reports whether a token is a bare method name. A path written
// with a leading "./" also starts with a dot, and reading `./app/models/x.rb`
// as a method call is how a whole file's citations silently disappear.
func isMethodOnly(s string) bool {
	if len(s) < 2 || (s[0] != '#' && s[0] != '.') {
		return false
	}
	return !strings.ContainsAny(s[1:], "/.")
}

func isSymbolTail(s string) bool {
	return s != "" && s[0] >= 'A' && s[0] <= 'Z' && !strings.Contains(s, "/")
}

// isExtension reports whether what follows a dot looks like a file extension
// rather than a method name.
func isExtension(s string) bool {
	if s == "" || len(s) > 4 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return true
}

func splitLine(t string) (body string, line int) {
	i := strings.LastIndex(t, ":")
	if i < 0 || i == len(t)-1 {
		return t, 0
	}
	n, err := strconv.Atoi(t[i+1:])
	if err != nil {
		return t, 0
	}
	return t[:i], n
}
