package plan

import (
	"fmt"
	"slices"
	"strings"

	"github.com/luuuc/sense/lab/internal/catalog"
)

// resolve answers whether one job can run, and returns the reason when it
// cannot. It is six explicit questions with six explicit answers.
//
// It is deliberately not clever. A score for "how well supported" a
// combination is would be a screen wearing a suit: it would let a job through
// on a number rather than on a fact, and every screen this project built was
// measured wrong.
func resolve(c *catalog.Catalog, repo, subjectID string, arm Arm) (Job, string) {
	subject := c.Subjects[subjectID]
	// Through the alias index, so `plan` and `run` agree on what a model name
	// is. Looking in the id map alone reports "no model file declares it" for
	// an alias that `run` accepts, which is a false statement aimed at exactly
	// the person who just wrote a model id in the other form.
	model, _ := c.Model(arm.Model)

	j := Job{
		Repo: repo, Subject: subjectID, Model: model.ID,
		Executor: subject.Executor, Role: arm.Role, Runs: arm.Runs,
	}

	agentID, reason := pickAgent(arm, model)
	if reason != "" {
		return j, reason
	}
	j.Agent = agentID
	agent := c.Agents[agentID]
	executor := c.Executors[subject.Executor]

	if reason := subjectSupportsAgent(subject, agentID); reason != "" {
		return j, reason
	}
	// Questions three and four together, because separating them would leave
	// the auth mode unchosen: a model available two ways passes an executor
	// that preserves only one of them, and if the machine has neither the
	// session dies empty at zero tokens — the very failure this refuses.
	auth, reason := pickAuth(model, agent, executor)
	if reason != "" {
		return j, reason
	}
	j.Auth = auth
	if reason := executorSatisfiesIsolation(executor, subject); reason != "" {
		return j, reason
	}
	if reason := credentialCarriesModel(agent, model); reason != "" {
		return j, reason
	}
	return j, ""
}

// pickAgent chooses the tool to drive the model, and IS resolution question
// two: does this agent tool support this model. Normally a model names exactly
// one and the arm says nothing; when a model names several, the arm has to
// choose, because picking one arbitrarily is how an arm ends up measured on a
// surface nobody intended.
//
// A separate "can this agent drive this model" check after it would be dead
// code: every path out of here already comes from the model's own agent list.
func pickAgent(arm Arm, model catalog.Model) (string, string) {
	if arm.Agent != "" {
		if !slices.Contains(model.Agents, arm.Agent) {
			return "", fmt.Sprintf("the arm names agent %q, but model %s can be driven by %v",
				arm.Agent, model.ID, model.Agents)
		}
		return arm.Agent, ""
	}
	switch len(model.Agents) {
	case 1:
		return model.Agents[0], ""
	case 0:
		return "", fmt.Sprintf("model %s names no agent tool, so nothing can drive it", model.ID)
	default:
		return "", fmt.Sprintf("model %s can be driven by %v and the arm does not say which",
			model.ID, model.Agents)
	}
}

// Question one: does this subject support this agent tool?
func subjectSupportsAgent(s catalog.Subject, agentID string) string {
	if slices.Contains(s.Agents, agentID) {
		return ""
	}
	return fmt.Sprintf("subject %s cannot be driven by %s; it supports %v", s.ID, agentID, s.Agents)
}

// credentialCarriesModel is question six: will the run's own login hold
// anything for this model?
//
// It is asked HERE as well as in the attended parent, and the duplication is
// deliberate. A bench is planned once and run cell by cell, so a cell the
// parent would refuse at spawn is a cell that plans clean and dies four cells
// in — and it dies with a message that reads like a bad model id, because the
// tool answers `UnknownError: Unexpected server error` whether the model does
// not exist or its provider key was not among the fields the run was given.
// Measured 2026-08-18, on two arms.
//
// A model naming no credential key runs on a tool whose login is not keyed by
// provider, and there is nothing here to check.
func credentialCarriesModel(a catalog.Agent, m catalog.Model) string {
	if m.CredentialKey == "" {
		return ""
	}
	for _, field := range a.CredentialFields {
		if strings.HasPrefix(field, m.CredentialKey+".") {
			return ""
		}
	}
	return fmt.Sprintf("model %s needs the %q key and agent %s carries %v; the run would be handed a "+
		"login with nothing for this model, and both arms would die with a server error that reads "+
		"like a bad model id", m.ID, m.CredentialKey, a.ID, a.CredentialFields)
}

// pickAuth answers questions three and four together and CHOOSES the mode,
// rather than merely confirming that some mode could work.
//
// Choosing is the point. A model available under two modes, an agent that
// accepts both and an executor that preserves one of them is runnable — but
// only one way, and nothing downstream would know which. Cycle 03's executor is
// handed the answer instead of guessing, and the rejection message can name
// what was tried.
//
// The order is the model's own, so the choice is deterministic and a plan does
// not change between runs.
func pickAuth(m catalog.Model, a catalog.Agent, e catalog.Executor) (string, string) {
	var reachable []string
	for _, auth := range m.AvailableUnder {
		if slices.Contains(a.AuthModes, auth) {
			reachable = append(reachable, auth)
		}
	}
	// Question three: is this model reachable at all through this tool? A model
	// available only under a mode its tool cannot use resolves to nothing at
	// spawn time, which is how an arm returns empty rather than crashing.
	if len(reachable) == 0 {
		return "", fmt.Sprintf("model %s is available under %v but agent %s authenticates by %v; "+
			"nothing could reach it", m.ID, m.AvailableUnder, a.ID, a.AuthModes)
	}
	// Question four: does the executor preserve one of them? A container that
	// never receives credentials cannot reach a model that needs them.
	for _, auth := range reachable {
		if slices.Contains(e.PreservesAuth, auth) {
			return auth, ""
		}
	}
	return "", fmt.Sprintf("executor %s preserves %v, but reaching model %s through %s needs one of %v",
		e.ID, e.PreservesAuth, m.ID, a.ID, reachable)
}

// Question five: are the subject's isolation requirements met by the executor?
//
// A subject that writes agent configuration must not run where that
// configuration escapes onto the host: the next run would inherit it, and
// nothing in either result would show it.
func executorSatisfiesIsolation(e catalog.Executor, s catalog.Subject) string {
	if !s.NeedsIsolatedConfig || e.IsolatesGlobalConfig {
		return ""
	}
	return fmt.Sprintf("subject %s writes agent configuration but executor %s does not isolate it, "+
		"so it would leak onto the host and into the next run", s.ID, e.ID)
}
