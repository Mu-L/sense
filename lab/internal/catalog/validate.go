package catalog

import (
	"fmt"
	"strings"
)

// validate checks what a consumer reads and no more.
//
// The depth follows the consumers that exist: identifiers resolve, referenced
// ids exist, required fields are present. A rule for a field nothing reads is a
// rule written from imagination, and it will be wrong in a way nobody notices
// until it blocks a real config.
//
// Every problem is reported, not just the first. A config directory is edited
// by hand and fixing one error at a time is how a five-minute change becomes an
// afternoon.
func (c *Catalog) validate() error {
	var probs []string

	for _, id := range IDs(c.Subjects) {
		probs = append(probs, c.checkSubject(c.Subjects[id])...)
	}
	for _, id := range IDs(c.Agents) {
		probs = append(probs, checkAgent(c.Agents[id])...)
	}
	for _, id := range IDs(c.Models) {
		probs = append(probs, c.checkModel(c.Models[id])...)
	}
	for _, id := range IDs(c.Repos) {
		probs = append(probs, checkRepo(c.Repos[id])...)
	}

	if len(probs) == 0 {
		return nil
	}
	return fmt.Errorf("catalog:\n  %s", strings.Join(probs, "\n  "))
}

func (c *Catalog) checkSubject(s Subject) []string {
	var p []string
	switch s.Kind {
	case Baseline, Sense, Competitor:
	case "":
		p = append(p, fmt.Sprintf("subject %s: no kind", s.ID))
	default:
		p = append(p, fmt.Sprintf("subject %s: kind %q is not baseline, sense or competitor", s.ID, s.Kind))
	}
	if len(s.Agents) == 0 {
		p = append(p, fmt.Sprintf("subject %s: names no agent tools, so nothing can drive it", s.ID))
	}
	if s.Executor == "" {
		p = append(p, fmt.Sprintf("subject %s: names no executor, so there is nowhere to run it", s.ID))
	} else if _, ok := c.Executors[s.Executor]; !ok {
		p = append(p, fmt.Sprintf("subject %s: names executor %q, which no executor file declares", s.ID, s.Executor))
	}
	for _, a := range s.Agents {
		if _, ok := c.Agents[a]; !ok {
			p = append(p, fmt.Sprintf("subject %s: names agent %q, which no agent file declares", s.ID, a))
		}
	}
	// A subject that registers an MCP server can only run through an agent
	// tool that speaks MCP. Catching it here costs nothing; catching it at run
	// time costs the run.
	if s.NeedsMCP {
		for _, a := range s.Agents {
			if agent, ok := c.Agents[a]; ok && !agent.SupportsMCP {
				p = append(p, fmt.Sprintf("subject %s needs MCP but agent %s does not support it", s.ID, a))
			}
		}
	}
	return p
}

func checkAgent(a Agent) []string {
	var p []string
	if a.Binary == "" {
		p = append(p, fmt.Sprintf("agent %s: no binary to spawn", a.ID))
	}
	if a.ModelFlag == "" {
		p = append(p, fmt.Sprintf("agent %s: no model flag, so no model could be selected", a.ID))
	}
	if len(a.HeadlessArgs) == 0 {
		p = append(p, fmt.Sprintf("agent %s: no headless args, so a session would wait for a terminal "+
			"that is not there and burn its wall", a.ID))
	}
	if len(a.AuthModes) == 0 {
		p = append(p, fmt.Sprintf("agent %s: no auth modes, so no model is reachable through it", a.ID))
	}
	if len(a.ConfigDirs) == 0 {
		// Not cosmetic. The contamination checks join this onto the disposable
		// HOME, and an empty one joins to HOME itself — which exists for every
		// arm, so every arm reads as contaminated. A check that cannot run must
		// not be allowed to run wrong.
		p = append(p, fmt.Sprintf("agent %s: no config dirs, so there is nowhere to check for "+
			"leaked state and the check would read the whole disposable HOME", a.ID))
	}
	return p
}

func (c *Catalog) checkModel(m Model) []string {
	var p []string
	if m.Provider == "" {
		p = append(p, fmt.Sprintf("model %s: no provider", m.ID))
	}
	if len(m.Agents) == 0 {
		p = append(p, fmt.Sprintf("model %s: names no agent tools, so nothing can drive it", m.ID))
	}
	for _, a := range m.Agents {
		agent, ok := c.Agents[a]
		if !ok {
			p = append(p, fmt.Sprintf("model %s: names agent %q, which no agent file declares", m.ID, a))
			continue
		}
		// A model reachable only under an auth mode its agent tool cannot use
		// is a model that resolves to nothing at spawn time. That has already
		// cost a whole arm: an id written in the wrong form mapped to a
		// provider id that did not exist, and every run came back empty with
		// zero tokens and exit 1.
		if !overlaps(m.AvailableUnder, agent.AuthModes) {
			p = append(p, fmt.Sprintf("model %s is available under %v but agent %s authenticates by %v; "+
				"nothing could reach it", m.ID, m.AvailableUnder, a, agent.AuthModes))
		}
	}
	return p
}

func checkRepo(r Repo) []string {
	var p []string
	if r.URL == "" {
		p = append(p, fmt.Sprintf("repo %s: no url", r.ID))
	}
	// A repo with no pinned commit is a repo that means something different
	// next week, so every number taken against it is unreproducible.
	if r.Commit == "" {
		p = append(p, fmt.Sprintf("repo %s: no pinned commit", r.ID))
	}
	if len(r.Languages) == 0 {
		p = append(p, fmt.Sprintf("repo %s: no languages, so no vertical query can select it", r.ID))
	}
	return p
}

func overlaps(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}
