package status

import (
	"fmt"
	"strings"

	"github.com/luuuc/sense/lab/internal/phase"
)

// banner is the first thing on the page and it is not decoration. A regenerated
// view cannot be quietly edited into a decision, and saying so is what stops
// someone treating this as the record.
const banner = "Position is authoritative in the run tree. This page is a view of it and decides nothing."

// Render turns a position into the page.
//
// It takes a struct and returns a string, and the only thing that consumes that
// string is a printer. Nothing in this codebase reads it back.
func Render(p Position) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", banner)
	fmt.Fprintf(&b, "run trees under %s\n\n", p.Root)
	renderRepos(&b, p)
	renderCells(&b, p)
	renderResume(&b, p)
	return b.String()
}

func renderRepos(b *strings.Builder, p Position) {
	b.WriteString("LOOP POSITION\n")
	if len(p.Repos) == 0 {
		b.WriteString("  nothing has run\n\n")
		return
	}
	for _, r := range p.Repos {
		state := fmt.Sprintf("cycle %d of %d, %d left before the ceiling", r.Cycle, phase.AuthoringCeiling, r.ToCeiling())
		if r.Parked {
			state = "PARKED at the ceiling, waiting for a human to re-enter it deliberately"
		}
		fmt.Fprintf(b, "  %-20s %s\n", r.Name, state)
		fmt.Fprintf(b, "  %-20s reached %s, awaiting %s\n", "", orNone(r.Reached), orNone(r.Awaiting))
		fmt.Fprintf(b, "  %-20s %s against a ceiling of %d, over this repository's lifetime\n",
			"", r.Spend, p.Ceiling)
		if len(r.Banked) > 0 {
			fmt.Fprintf(b, "  %-20s banked on cycle %v\n", "", r.Banked)
		}
	}
	b.WriteString("\n")
}

// renderCells shows the uncomfortable rows first and never folds them into a
// count. A half-pair, a burned run and an orphaned directory are what a
// resuming session most needs to know.
func renderCells(b *strings.Builder, p Position) {
	b.WriteString("CELLS ON DISK\n")
	incomplete := p.Incomplete()
	if len(p.Cells) == 0 && len(p.Orphans) == 0 {
		b.WriteString("  none\n\n")
		return
	}
	fmt.Fprintf(b, "  %d recorded, %d of them incomplete\n", len(p.Cells), len(incomplete))
	for _, c := range incomplete {
		fmt.Fprintf(b, "  INCOMPLETE %s\n", c.Path)
		if len(c.Burned) > 0 {
			fmt.Fprintf(b, "    burned, can never be paired: %s\n", strings.Join(c.Burned, ", "))
		}
		if len(c.Unusable) > 0 {
			fmt.Fprintf(b, "    no result: %s\n", strings.Join(c.Unusable, ", "))
		}
	}
	for _, o := range p.Orphans {
		fmt.Fprintf(b, "  ORPHAN     %s ran and recorded no terminal state\n", o)
	}
	b.WriteString("\n")
}

// renderResume prints the next action: the phase, the plan that says how to run
// it, and the artifact it owes.
func renderResume(b *strings.Builder, p Position) {
	b.WriteString("RESUME\n")
	if len(p.Resume) == 0 {
		b.WriteString("  nothing to resume\n")
		return
	}
	for _, r := range p.Resume {
		fmt.Fprintf(b, "  %-20s run %s from %s\n", r.Repo, r.Phase, r.Plan)
		fmt.Fprintf(b, "  %-20s it writes %s into %s\n", "", r.Artifact, r.Dir)
	}
}

// orNone names a phase, or says plainly that there is not one. An empty column
// reads as a rendering bug rather than as a fact about the repository.
func orNone(name phase.Name) string {
	if name == "" {
		return "none"
	}
	return string(name)
}
