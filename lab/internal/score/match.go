package score

import (
	"strconv"
	"strings"
)

// Matches reports whether a cite names the gold row's location.
//
// The two forms are held to DIFFERENT standards, and the difference is the most
// important thing in this file.
//
// A PATH citation is strict: same file, same line. The right file at the wrong
// line is a teammate sent to the wrong place, which is the thing the metric
// exists to measure.
//
// A SYMBOL citation is matched on the symbol, and its line is required to be
// present but is NOT compared. That is the recorded ruling of 2026-08-04
// ("count class name plus line number"), and it is what the five rows on the
// banked mastodon cell turn on: the arm wrote `Admin::ActionLogsController#index`
// `:7` where gold cites line 9, and scoring that as a miss silently discounted
// the arm for answering in a form the scorer did not expect.
//
// This asymmetry is deliberate and it is a KNOWN, MEASURED LOOSENESS.
// Comparing a symbol citation's line against gold needs the symbol's line
// range, which needs a repository on disk — that is 02-04, and recall must not
// depend on the state of a disk.
//
// The corpus DOES exploit it, and the size is on Result.SymbolCited: 144 of the
// 195 rows credited this way are in the sense arm, and the rule inflates the
// measured margin by 73%. That is why Result prints the strict number beside
// the loose one and never a single figure.
func Matches(c Cite, goldPath, goldLine string) bool {
	if c.Line == 0 {
		return false
	}
	if c.Path != "" {
		return strconv.Itoa(c.Line) == goldLine && samePath(c.Path, goldPath)
	}
	if c.Symbol == "" {
		return false
	}
	// Either door. The established file is a FALLBACK, never a replacement: an
	// answer that named the path earlier and then wrote the symbol must not
	// score worse than one that wrote the symbol alone, and it would if the
	// stricter test replaced the looser one instead of joining it.
	return matchesEstablished(c, goldPath, goldLine) || symbolNamesPath(c.Symbol, goldPath)
}

// matchesEstablished is the STRICT way a symbol cite can match: the file the
// answer had already named, held to the gold line exactly as a path cite is.
func matchesEstablished(c Cite, goldPath, goldLine string) bool {
	return c.Established != "" && strconv.Itoa(c.Line) == goldLine && samePath(c.Established, goldPath)
}

// NamesFile reports whether a cite names the gold row's file at any line, which
// is reach rather than score.
func NamesFile(c Cite, goldPath string) bool {
	if c.Path != "" {
		return samePath(c.Path, goldPath)
	}
	if c.Established != "" && samePath(c.Established, goldPath) {
		return true
	}
	return c.Symbol != "" && symbolNamesPath(c.Symbol, goldPath)
}

// symbolNamesPath reports whether a Ruby constant names the gold file.
//
// This is not a guess about where a class probably lives: Ruby's autoloader
// REQUIRES the mapping, so `Admin::Metrics::Dimension::SpaceUsageDimension` is
// defined in `admin/metrics/dimension/space_usage_dimension.rb` or it does not
// load at all. The gold path may carry leading directories the constant does
// not (`app/lib/`), so the derived path must be a suffix of gold at a path
// boundary — the same rule, and the same reason, as samePath.
//
// It is a RUBY rule and it is named like one. A C# or Go symbol does not
// resolve through it and simply fails to match, which is honest: the general
// answer is the grounded range check in 02-04, not a second convention guessed
// at here.
func symbolNamesPath(symbol, goldPath string) bool {
	cls, _, _ := strings.Cut(symbol, "#")
	if cls == "" {
		return false
	}
	var b strings.Builder
	for i, part := range strings.Split(cls, "::") {
		if i > 0 {
			b.WriteByte('/')
		}
		b.WriteString(underscore(part))
	}
	derived := b.String() + ".rb"
	if goldPath == derived {
		return true
	}
	if !hasPathSuffix(goldPath, derived) {
		return false
	}
	// What is left over once the constant's own path is removed must be an
	// AUTOLOAD ROOT, and a root is one or two segments: `lib`, `app/models`,
	// `app/lib`. Any deeper and a directory that should have been part of the
	// constant was skipped — `ActionLogsController` does not name
	// `app/controllers/admin/action_logs_controller.rb`, because with `admin/`
	// in the path Ruby requires `Admin::ActionLogsController`.
	//
	// Without this the symbol door credits what samePath refuses at the front
	// door: a name matching the tail of a file it does not actually name.
	root := goldPath[:len(goldPath)-len(derived)-1]
	return strings.Count(root, "/") < 2
}

// underscore is Ruby's constant-to-filename rule: ActionLogsController becomes
// action_logs_controller, V1 becomes v1, and a run of capitals breaks before
// the last one when a lowercase letter follows it.
func underscore(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			if i > 0 && (!isUpper(s[i-1]) || (i+1 < len(s) && !isUpper(s[i+1]))) {
				b.WriteByte('_')
			}
			c += 'a' - 'A'
		}
		b.WriteByte(c)
	}
	return b.String()
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }

// corroborated returns the file an answer has already established, but only
// when the symbol independently agrees that it is the right file.
//
// An arm writing a list stops repeating the path once it has given it:
//
//  43. `Jobs::ReindexSearch#rebuild_categories` — `app/jobs/…/reindex_search.rb:22`
//  44. `Jobs::ReindexSearch#load_problem_category_ids` — `:111`
//
// Item 44 is a location in that same file, and reading it as a symbol alone
// loses it: `Jobs::ReindexSearch` derives `jobs/reindex_search.rb`, while the
// file is `app/jobs/scheduled/reindex_search.rb`. Discourse and mastodon both
// add autoload roots, so the constant rule alone does not reach these.
//
// Inheriting the path unconditionally is the dangerous version, and it is the
// one to refuse: an inline list of unrelated controllers would each pick up
// whatever file happened to be named last, and any of them could then land on a
// gold line by coincidence. So the two signals have to agree — the symbol's own
// derived basename must be the basename of the established file. Neither signal
// is trusted alone.
func corroborated(symbol, lastPath string) string {
	if lastPath == "" {
		return ""
	}
	cls, _, _ := strings.Cut(symbol, "#")
	if i := strings.LastIndex(cls, "::"); i >= 0 {
		cls = cls[i+2:]
	}
	if underscore(cls)+".rb" == basename(lastPath) {
		return lastPath
	}
	return ""
}

func basename(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
