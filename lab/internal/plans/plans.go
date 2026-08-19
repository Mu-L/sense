// Package plans reads the phase plans and checks them against the declared
// graph.
//
// The binary holds no prompt logic by design: how a scenario is crafted, how a
// mini-bench is read, how a pay call is made and how a win is confirmed all live
// as prose in files a human wrote. The moment a phase's instructions are
// assembled in code they stop being reviewable by reading a file, and the binary
// starts holding opinions.
//
// So this package loads. It never composes: a plan's body is the bytes after its
// header, handed on unchanged, and there is nowhere here for a sentence to be
// added to one.
//
// # The header
//
// Four keys, one per line, between two fences, and then the plan:
//
//	---
//	phase: minibench
//	reads: scenario.draft.yaml
//	writes: minibench.md
//	emits: [PROCEED, REQUESTION, NO-ANCHOR]
//	wall: 25m
//	---
//
// It is a fixed set of keys rather than general YAML so that a malformed
// contract fails to parse instead of parsing into something plausible.
//
// The wall is here rather than in Go because it is the number somebody would
// otherwise raise in a hurry to rescue a stalled phase. Declared beside the
// plan, changing it is an edit to a file that is reviewed.
//
// # What the checks can and cannot catch
//
// They catch a plan naming a verdict that does not exist, an artifact the graph
// does not know, a phase with no plan and a plan with no phase. Nothing here
// catches a plan that is subtly wrong. Those defects are only discoverable by
// spending money against a real repository, which is a reason to expect
// revisions back rather than a reason to write the plans differently.
package plans

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/luuuc/sense/lab/internal/phase"
)

// Plan is one phase's plan file: its declared contract, and the prose under it.
type Plan struct {
	Path   string
	Phase  phase.Name
	Reads  string
	Writes string
	Emits  []phase.Verdict
	// Wall is how long this phase's agent gets. It is declared here, beside the
	// plan the agent runs, and never as a constant in Go: config as data is
	// this instrument's rule, and a wall held in a variable is the number that
	// gets raised in a hurry at 3am to rescue a stalled phase — which is what
	// CANNOT-FINISH-AT-BUDGET IS A RESULT forbids.
	//
	// Zero means the phase declares none, and whoever runs it decides what that
	// means. This package loads; it does not supply defaults for what a file
	// left out.
	Wall time.Duration
	// Body is everything after the header, verbatim. Nothing in this package
	// writes to it.
	Body []byte
}

// fence opens and closes the header block.
const fence = "---\n"

// Load reads every plan in dir.
func Load(dir string) ([]Plan, error) {
	// The pattern is fixed, so the only error Glob defines cannot happen. A
	// directory that is missing or empty comes back with no names, and the
	// refusal below is the answer to both.
	names, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	slices.Sort(names)
	var out []Plan
	for _, name := range names {
		b, err := os.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read plan %s: %w", name, err)
		}
		p, err := parse(b)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		p.Path = name
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no plans in %s; a phase graph with no plan files is a graph nobody can run", dir)
	}
	return out, nil
}

// parse splits a plan into its declared header and its prose.
//
// The header is a fixed set of keys rather than general YAML: one field per
// line, so that a plan whose contract is malformed fails here rather than
// parsing into something plausible.
func parse(b []byte) (Plan, error) {
	rest, ok := bytes.CutPrefix(b, []byte(fence))
	if !ok {
		return Plan{}, fmt.Errorf("no header; every plan declares the phase it belongs to")
	}
	head, body, ok := bytes.Cut(rest, []byte(fence))
	if !ok {
		return Plan{}, fmt.Errorf("unterminated header")
	}
	p := Plan{Body: body}
	for _, line := range strings.Split(string(head), "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "phase":
			p.Phase = phase.Name(value)
		case "reads":
			p.Reads = value
		case "writes":
			p.Writes = value
		case "emits":
			p.Emits = verdicts(value)
		case "wall":
			d, err := time.ParseDuration(strings.TrimSpace(value))
			if err != nil {
				return Plan{}, fmt.Errorf("wall %q: %w", value, err)
			}
			p.Wall = d
		}
	}
	if p.Phase == "" {
		return Plan{}, fmt.Errorf("the header names no phase")
	}
	return p, nil
}

func verdicts(value string) []phase.Verdict {
	value = strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(value), "["), "]")
	var out []phase.Verdict
	for _, v := range strings.Split(value, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, phase.Verdict(v))
		}
	}
	return out
}

// Check reports every disagreement between the plans in dir and the graph.
//
// All of them, not the first: an author who fixes one line only to be refused by
// the next has learned nothing about the set.
func Check(dir string) []error {
	loaded, err := Load(dir)
	if err != nil {
		return []error{err}
	}
	var problems []error
	seen := map[phase.Name]string{}
	for _, p := range loaded {
		if at, dup := seen[p.Phase]; dup {
			problems = append(problems, fmt.Errorf("%s and %s both claim phase %s", at, p.Path, p.Phase))
			continue
		}
		seen[p.Phase] = p.Path
		problems = append(problems, p.check()...)
	}
	for _, declared := range phase.Graph {
		if _, ok := seen[declared.Name]; !ok {
			problems = append(problems, fmt.Errorf("phase %s has no plan file; a phase with no plan is a phase nobody can run",
				declared.Name))
		}
	}
	return problems
}

// check compares one plan against its row in the graph.
func (p Plan) check() []error {
	declared, ok := phase.Lookup(p.Phase)
	if !ok {
		return []error{fmt.Errorf("%s names phase %q, which the graph does not declare", p.Path, p.Phase)}
	}
	var problems []error
	if p.Reads != declared.Reads {
		problems = append(problems, fmt.Errorf("%s reads %q; phase %s reads %q",
			p.Path, p.Reads, p.Phase, declared.Reads))
	}
	if p.Writes != declared.Writes {
		problems = append(problems, fmt.Errorf("%s writes %q; phase %s writes %q",
			p.Path, p.Writes, p.Phase, declared.Writes))
	}
	problems = append(problems, p.checkVerdicts(declared)...)
	if len(bytes.TrimSpace(p.Body)) == 0 {
		problems = append(problems, fmt.Errorf("%s has a header and no plan under it", p.Path))
	}
	return problems
}

// checkVerdicts holds the plan's enum to the graph's, in both directions.
//
// A plan naming a verdict the graph does not know is a plan whose routing cannot
// be tested. A plan missing one the graph knows is worse and quieter: it is a
// case the phase really meets and has been given no instruction for, which is
// how a phase writes nothing and stalls a campaign at three in the morning.
func (p Plan) checkVerdicts(declared phase.Phase) []error {
	var problems []error
	for _, v := range p.Emits {
		if !declared.Emits(v) {
			problems = append(problems, fmt.Errorf("%s emits %q; phase %s emits %v",
				p.Path, v, p.Phase, declared.Verdicts))
		}
	}
	for _, v := range declared.Verdicts {
		if !slices.Contains(p.Emits, v) {
			problems = append(problems, fmt.Errorf("%s does not say what to do on %q, which phase %s can emit",
				p.Path, v, p.Phase))
		}
	}
	return problems
}
