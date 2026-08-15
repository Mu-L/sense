// Package scenario reads a scenario file and renders it into a prompt.
//
// It reads only the fields the walking skeleton needs and ignores the rest, so
// it can read the existing corpus as-is. Cycle 02 settles the real scenario,
// gold and rubric types; nothing here is meant to survive it.
package scenario

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scenario is the subset of a scenario file the skeleton uses.
type Scenario struct {
	Name        string    `yaml:"name"`
	Repo        string    `yaml:"repo"`
	Description string    `yaml:"description"`
	Steps       []Step    `yaml:"steps"`
	Gold        []GoldRow `yaml:"gold"`
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

// Load reads a scenario file. Unknown fields are ignored on purpose: the corpus
// carries gold, rubric weights and campaign history this cycle has no use for,
// and refusing to read a file over a field we do not want would be a gate that
// buys nothing.
func Load(path string) (Scenario, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Scenario{}, fmt.Errorf("read scenario: %w", err)
	}
	var s Scenario
	if err := yaml.Unmarshal(b, &s); err != nil {
		return Scenario{}, fmt.Errorf("parse scenario %s: %w", path, err)
	}
	if err := s.validate(); err != nil {
		return Scenario{}, fmt.Errorf("scenario %s: %w", path, err)
	}
	return s, nil
}

// validate rejects a scenario that would produce a prompt with nothing in it.
// A run against an empty prompt costs the same as a real one and scores zero,
// which reads as a failed arm rather than a broken input.
func (s Scenario) validate() error {
	if len(s.Steps) == 0 {
		return fmt.Errorf("no steps")
	}
	for i, step := range s.Steps {
		if strings.TrimSpace(step.Prompt) == "" {
			return fmt.Errorf("step %d (%q) has an empty prompt", i+1, step.Name)
		}
	}
	return nil
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
// rather than let it become a permanent miss — see Scenario.GoldGroup.
func (g GoldRow) Cite() string {
	m := leadingCite.FindStringSubmatch(strings.TrimSpace(g.Relation))
	if m == nil {
		return ""
	}
	return m[1]
}

// GoldGroup returns the rows of one gold group, in file order.
//
// A row with no parseable location is an error rather than a row. Left alone it
// becomes a miss nothing can ever satisfy, which silently deflates the score and
// looks exactly like an arm that failed to find the place — the direction of
// failure that makes a real result look worse than it was.
func (s Scenario) GoldGroup(name string) ([]GoldRow, error) {
	var out []GoldRow
	for _, g := range s.Gold {
		if g.Group != name {
			continue
		}
		if g.Cite() == "" {
			return nil, fmt.Errorf("gold row %q in group %q has no path:line at the front of its relation, "+
				"so nothing could ever match it", g.ID, name)
		}
		out = append(out, g)
	}
	return out, nil
}
