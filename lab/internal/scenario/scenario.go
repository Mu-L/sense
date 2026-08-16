// Package scenario reads a scenario, its gold and its rubric, and renders the
// scenario into a prompt.
//
// The three live in three files with three hashes, because they invalidate
// three different things: the scenario invalidates the run, the gold only the
// score, the rubric only the judgment. That split is what makes a gold
// correction cost a rescore instead of a re-run, and gold correction is most of
// the work in this bench.
package scenario

import (
	"fmt"
	"regexp"
	"strings"
)

// Scenario is the subset of a scenario file the skeleton uses.
type Scenario struct {
	Name        string `yaml:"name"`
	Repo        string `yaml:"repo"`
	Description string `yaml:"description"`
	Steps       []Step `yaml:"steps"`
}

// GoldRow is one thing a good answer has to find. The corpus keeps the row's
// authoritative location as the first path:line of its Relation prose, with the
// reason in English after it.
type GoldRow struct {
	ID       string `yaml:"id"`
	Group    string `yaml:"group"`
	Relation string `yaml:"relation"`
}

// Step is one question in the session.
type Step struct {
	Name   string `yaml:"name"`
	Prompt string `yaml:"prompt"`
}

// Prompt renders the scenario into the text handed to the agent. It concatenates
// the description and the numbered steps and does nothing else: a template
// engine here would be guessing at cycle 05's shape from one example.
func (s Scenario) Prompt() string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(s.Description))
	b.WriteString("\n\nWork through the following steps in order. ")
	b.WriteString("Answer each one fully before moving to the next.\n")
	for i, step := range s.Steps {
		fmt.Fprintf(&b, "\n## Step %d: %s\n\n%s\n", i+1, step.Name, strings.TrimSpace(step.Prompt))
	}
	return b.String()
}

// leadingCite is the row's authoritative location: the first path:line in its
// Relation prose, anchored so a location buried in the reason text cannot be
// mistaken for the row's own.
//
// This MUST accept exactly what the scorer's citation pattern accepts on the
// answer side. A shape gold takes that a citation cannot match would leave the
// row permanently unmatchable, and the packages do not import each other, so
// CitePatternMatchesTheScorer in the tests is what holds them together.
var leadingCite = regexp.MustCompile(`^([A-Za-z0-9._/\-]+\.[A-Za-z][A-Za-z0-9]*:\d+)`)

// Cite returns the row's authoritative path:line, or "" if it has none. A row
// with no location cannot be scored strictly at all, so callers must refuse it
// rather than let it become a permanent miss — see Gold.Group.
func (g GoldRow) Cite() string {
	m := leadingCite.FindStringSubmatch(strings.TrimSpace(g.Relation))
	if m == nil {
		return ""
	}
	return m[1]
}
