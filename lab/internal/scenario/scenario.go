// Package scenario reads a scenario file and renders it into a prompt.
//
// It reads only the fields the walking skeleton needs and ignores the rest, so
// it can read the existing corpus as-is. Cycle 02 settles the real scenario,
// gold and rubric types; nothing here is meant to survive it.
package scenario

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Scenario is the subset of a scenario file the skeleton uses.
type Scenario struct {
	Name        string `yaml:"name"`
	Repo        string `yaml:"repo"`
	Description string `yaml:"description"`
	Steps       []Step `yaml:"steps"`
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
