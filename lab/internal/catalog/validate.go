package catalog

import (
	"fmt"
	"slices"
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
	p = append(p, checkSubjectCommands(s)...)
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

// checkSubjectCommands refuses a command nothing could run, and a subject that
// installs something without saying how to remove it.
//
// The second is the one that matters. A subject with an install and no cleanup
// leaves whatever it wrote on the machine, every later run on that machine
// reads it, and the symptom looks like drift rather than like a leak.
func checkSubjectCommands(s Subject) []string {
	var p []string
	for _, stage := range []struct {
		what string
		cmds [][]string
	}{{"install", s.Install}, {"setup", s.Setup}, {"cleanup", s.Cleanup}} {
		for i, argv := range stage.cmds {
			if len(argv) == 0 {
				p = append(p, fmt.Sprintf("subject %s: %s step %d is an empty command", s.ID, stage.what, i+1))
			}
		}
	}
	if len(s.Install) > 0 && len(s.Cleanup) == 0 {
		p = append(p, fmt.Sprintf("subject %s: installs something and declares no cleanup, so whatever it "+
			"writes stays on the machine and every later run reads it", s.ID))
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
	if a.SetupTool == "" {
		// The channel derivation asks `sense setup` to configure this tool and
		// reads back what it wrote. Asking for a tool the product does not know
		// writes nothing, and an empty derived list makes every absence check
		// pass — a contaminated baseline arm reading as a clean one.
		p = append(p, fmt.Sprintf("agent %s: no setup tool, so the channel list would be derived by "+
			"configuring a tool the product does not know, and an empty list passes every check", a.ID))
	}
	p = append(p, checkCapture(a)...)
	p = append(p, checkWallNote(a)...)
	if a.TranscriptFormat == "" {
		p = append(p, fmt.Sprintf("agent %s: no transcript format, so nothing could read what it says", a.ID))
	}
	if len(a.ConfigDirs) == 0 {
		// Not cosmetic. The contamination checks join this onto the disposable
		// HOME, and an empty one joins to HOME itself — which exists for every
		// arm, so every arm reads as contaminated. A check that cannot run must
		// not be allowed to run wrong.
		p = append(p, fmt.Sprintf("agent %s: no config dirs, so there is nowhere to check for "+
			"leaked state and the check would read the whole disposable HOME", a.ID))
	}
	return append(p, checkCredentialRoute(a)...)
}

// checkCapture refuses a registration shape the capture shim cannot rewrite. A
// tool that declares none runs uncaptured on purpose; a tool that declares half
// a shape runs uncaptured by accident.
func checkCapture(a Agent) []string {
	m := a.MCPRegistration
	if m.File == "" {
		return nil
	}
	var p []string
	if m.ServersKey == "" {
		p = append(p, fmt.Sprintf("agent %s: names an MCP registration file but no key the servers live "+
			"under, so the capture would rewrite nothing and report zero frames", a.ID))
	}
	if m.CommandStyle != CommandArgv && m.CommandStyle != CommandAndArgs {
		p = append(p, fmt.Sprintf("agent %s: states its MCP command style as %q, which is neither %q nor %q",
			a.ID, m.CommandStyle, CommandArgv, CommandAndArgs))
	}
	return p
}

// checkWallNote refuses a note that would be written and never reach an arm.
func checkWallNote(a Agent) []string {
	var p []string
	if a.WallNote != "" && a.WallNoteDelivery != WallNoteFlagged && a.WallNoteDelivery != WallNotePrompted {
		p = append(p, fmt.Sprintf("agent %s: declares a wall note but delivers it by %q, which is neither "+
			"%q nor %q, so the note would be written and never reach the arm",
			a.ID, a.WallNoteDelivery, WallNoteFlagged, WallNotePrompted))
	}
	if a.WallNoteDelivery == WallNoteFlagged && a.WallNoteFlag == "" {
		p = append(p, fmt.Sprintf("agent %s: delivers its wall note by flag but names no flag", a.ID))
	}
	return p
}

// checkCredentialRoute refuses a HALF-declared credential route.
//
// The four fields are all-or-nothing: the variable points a run at its config
// directory, the file names the document inside it, the field list is what may
// be copied into it, and the expiry is how a planner knows the credential
// outlives the cell.
//
// `keychain_service` is deliberately NOT among them. It is a second source for
// the same document that only some tools have, and requiring it would refuse
// the route of every tool that keeps its login in a file and nowhere else. Declaring some of them
// produces a route that cannot carry a credential, and the run then falls back
// to key authentication and reads as a model with nothing to say — which is the
// failure this whole route exists to end, re-entering through the catalog.
//
// None of the three is the same as some of them: a tool that takes no config
// directory legitimately declares nothing here and authenticates by key.
func checkCredentialRoute(a Agent) []string {
	// A tool takes its credential either as a file in a config directory it is
	// pointed at, or as a document in a variable. The two halves are checked
	// as two routes rather than one long list, because a tool that takes the
	// environment route legitimately has neither a config variable nor a file.
	if a.CredentialEnv != "" {
		return checkEnvCredentialRoute(a)
	}
	declared := map[string]string{
		"config_dir_var":    a.ConfigDirVar,
		"credential_file":   a.CredentialFile,
		"credential_expiry": a.CredentialExpiry,
	}
	if len(a.CredentialFields) > 0 {
		declared["credential_fields"] = "declared"
	} else {
		declared["credential_fields"] = ""
	}
	var missing []string
	for name, value := range declared {
		if value == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) == 0 || len(missing) == len(declared) {
		return nil
	}
	slices.Sort(missing)
	return []string{fmt.Sprintf("agent %s: a half-declared credential route, missing %v. All four or "+
		"none: a partial route cannot carry a credential, and the run falls back to key authentication "+
		"and produces an arm that reads as a model with nothing to say", a.ID, missing)}
}

// checkEnvCredentialRoute refuses a half-declared environment route.
//
// The document still has to be READ from the operator's own machine before it
// can be handed to a run, so the file and its fields are as necessary here as
// they are for a tool that takes a file. What is not necessary is a variable
// pointing the run at a config directory, because the run is given no
// directory to point at.
func checkEnvCredentialRoute(a Agent) []string {
	var missing []string
	if a.CredentialFile == "" {
		missing = append(missing, "credential_file")
	}
	if len(a.CredentialFields) == 0 {
		missing = append(missing, "credential_fields")
	}
	if a.CredentialExpiry == "" {
		missing = append(missing, "credential_expiry")
	}
	if len(missing) == 0 {
		return nil
	}
	return []string{fmt.Sprintf("agent %s: takes its credential through %s but declares no %v, so nothing "+
		"could read the operator's own login to hand it over", a.ID, a.CredentialEnv, missing)}
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
