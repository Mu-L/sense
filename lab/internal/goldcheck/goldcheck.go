// Package goldcheck audits gold rows against the rails the laws already state.
//
// Gold is hand-audited, and that is why the numbers mean anything. It is also
// where the expensive mistakes live, and the recorded ones do not look wrong
// when you read the file:
//
//   - A row that can never score. A scenario asked for "the specs/fixtures that
//     must be updated or added ... use file:line throughout", and its two gold
//     rows named files with no line at all. They scored mentioned 2 of 2 in 8 of
//     10 runs and cited 0 of 2 in 10 of 10, both arms, both models, for months.
//     The arms did the work and named the files in the only natural form, since
//     a line number is meaningless for a file that does not exist yet.
//
//   - A row that is free. A gold set whose eighteen rows were each a single
//     `Setting.<key>` read scored 17 of 18 to a baseline in nine tool calls.
//
// Neither is caught by reading the file. Both are caught by a check that knows
// the row's form, its group, and what a covering grep would print.
//
// The validator REPORTS. It never edits gold: a validator that also fixes what
// it finds is a validator nobody can check.
package goldcheck

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/luuuc/sense/lab/internal/scenario"
)

// Reason is why a row was quarantined, or flagged.
//
// Every finding carries one. "Quarantined with a named reason" is the whole
// contract: a row dropped without one is indistinguishable from a row nobody
// looked at.
type Reason string

const (
	// NoLocation is the recorded spec-fixture failure. A row scored at
	// path:line must carry a line a correct answer could actually write.
	NoLocation Reason = "no path:line, so nothing an arm writes could ever match it"
	// Unresolved is a row pointing somewhere that is not there at the pinned
	// commit. A row pointing at a line that moved is not gold, it is history.
	Unresolved Reason = "does not resolve at the pinned commit"
	// DuplicateFile is two rows of one group in the same file, which weights
	// the group toward whichever file has the most matches.
	DuplicateFile Reason = "a second row of this group in the same file"
	// DuplicateID is two rows sharing an id, which makes a hit ambiguous.
	DuplicateID Reason = "another row already has this id"
	// InCoveringGrep is a FLAG and never a quarantine.
	//
	// It says the row's exact location appears in a plain grep for the anchor.
	// On its own that means very little, and the size of the grep is the whole
	// story: on discourse, 19 of 23 rows lie inside a grep for `Category` that
	// prints 4483 lines, and reading 4483 lines to find 23 is not free — it is
	// the work the scenario is asking for. A row is only cheap to the baseline
	// when the covering grep is small enough to read whole.
	//
	// So this reports the membership AND the size, and the human decides. The
	// law is explicit that it ranks and never kills: run as a per-row census it
	// once killed four shapes on a repository that banks +0.53.
	InCoveringGrep Reason = "inside a covering grep for the anchor"
)

// Quarantines reports whether a reason removes the row rather than ranking it.
func (r Reason) Quarantines() bool { return r != InCoveringGrep }

// Finding is one thing wrong, or worth knowing, about one row.
type Finding struct {
	RowID  string
	Group  string
	Cite   string
	Reason Reason
	// Detail carries what the reason alone cannot: the grep's line count for a
	// free row, the other row's id for a duplicate.
	Detail string
}

func (f Finding) String() string {
	s := fmt.Sprintf("%-34s %-12s", f.RowID, f.Group)
	if f.Cite != "" {
		s += " " + f.Cite
	}
	if f.Detail != "" {
		s += " (" + f.Detail + ")"
	}
	return s
}

// Resolver says whether a location exists. It is an interface so this package
// does not need a repository, and a scenario with no checkout is audited for
// everything except resolution rather than not at all.
type Resolver interface {
	// Resolves reports whether path has that line. The second return is false
	// when nothing could be checked.
	Resolves(path string, line int) (ok, checked bool)
}

// Grepper says how many lines a covering grep prints, and whether a location is
// among them. It is what makes "free" measurable rather than a judgement.
type Grepper interface {
	Covering(symbol string) (hits map[string]bool, total int, checked bool)
}

// Report is one scenario's gold, audited.
type Report struct {
	Scenario    string
	Rows        int
	Findings    []Finding
	Quarantined []string // row ids, in gold file order
	// Unchecked names the checks that could not run, so a clean report cannot
	// be read as a complete one.
	Unchecked []string
}

// Audit runs every check that can run, and names the ones that cannot.
//
// resolve and grep may both be nil: a scenario with no checkout is audited for
// form and shape, and the report says what it could not look at. A validator
// that refused to run without a repository would be a validator nobody runs.
func Audit(set scenario.Set, resolve Resolver, grep Grepper) Report {
	r := Report{Scenario: set.Name, Rows: len(set.Gold.Rows)}
	seenID := map[string]string{}
	seenFile := map[string]string{}
	quarantined := map[string]bool{}

	add := func(f Finding) {
		r.Findings = append(r.Findings, f)
		if f.Reason.Quarantines() && !quarantined[f.RowID] {
			quarantined[f.RowID] = true
			r.Quarantined = append(r.Quarantined, f.RowID)
		}
	}

	for _, row := range set.Gold.Rows {
		cite := row.Cite()
		f := Finding{RowID: row.ID, Group: row.Group, Cite: cite}

		if prev, ok := seenID[row.ID]; ok {
			f.Reason, f.Detail = DuplicateID, "first seen in group "+prev
			add(f)
		}
		seenID[row.ID] = row.Group

		if cite == "" {
			f.Reason = NoLocation
			add(f)
			continue
		}
		path, line := splitCite(cite)
		key := row.Group + "\x00" + path
		if prev, ok := seenFile[key]; ok {
			f.Reason, f.Detail = DuplicateFile, "the first is "+prev
			add(f)
		} else {
			seenFile[key] = row.ID
		}

		if resolve != nil {
			if ok, checked := resolve.Resolves(path, line); checked && !ok {
				f.Reason, f.Detail = Unresolved, ""
				add(f)
			}
		}
	}

	if resolve == nil {
		r.Unchecked = append(r.Unchecked, string(Unresolved))
	}
	r.Unchecked = append(r.Unchecked, freeRows(set, grep, add)...)
	return r
}

// freeRows flags the rows whose location a covering grep for the anchor prints.
//
// It FLAGS, and the grep's size travels with every flag, because membership
// alone is close to meaningless: a row inside a 4483-line grep is not free to
// anybody. The law is explicit that this ranks and never kills.
func freeRows(set scenario.Set, grep Grepper, add func(Finding)) []string {
	symbol := set.Scenario.ContractSymbol
	if grep == nil || symbol == "" {
		return []string{string(InCoveringGrep)}
	}
	hits, total, checked := grep.Covering(symbol)
	if !checked {
		return []string{string(InCoveringGrep)}
	}
	for _, row := range set.Gold.Rows {
		cite := row.Cite()
		if cite == "" || !hits[cite] {
			continue
		}
		add(Finding{
			RowID: row.ID, Group: row.Group, Cite: cite, Reason: InCoveringGrep,
			Detail: fmt.Sprintf("grep %q prints %d lines in all", symbol, total),
		})
	}
	return nil
}

func splitCite(cite string) (path string, line int) {
	i := strings.LastIndex(cite, ":")
	if i <= 0 {
		return cite, 0
	}
	n, err := strconv.Atoi(cite[i+1:])
	if err != nil {
		return cite, 0
	}
	return cite[:i], n
}

// String renders the report for a human, quarantines first.
func (r Report) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d gold rows, %d quarantined\n", r.Scenario, r.Rows, len(r.Quarantined))
	byReason := map[Reason][]Finding{}
	for _, f := range r.Findings {
		byReason[f.Reason] = append(byReason[f.Reason], f)
	}
	reasons := make([]string, 0, len(byReason))
	for reason := range byReason {
		reasons = append(reasons, string(reason))
	}
	sort.Strings(reasons)
	for _, reason := range reasons {
		fs := byReason[Reason(reason)]
		if !Reason(reason).Quarantines() {
			// One line, not one per row: the detail is identical for every row
			// and the size of the grep is the only part that carries meaning.
			fmt.Fprintf(&b, "\n  flagged: %d of %d rows %s\n            %s\n",
				len(fs), r.Rows, reason, fs[0].Detail)
			continue
		}
		fmt.Fprintf(&b, "\n  QUARANTINED: %s (%d)\n", reason, len(fs))
		for _, f := range fs {
			fmt.Fprintf(&b, "    %s\n", f)
		}
	}
	for _, u := range r.Unchecked {
		fmt.Fprintf(&b, "\n  NOT CHECKED: %s\n", u)
	}
	return b.String()
}
