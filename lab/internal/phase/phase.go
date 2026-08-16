// Package phase declares the order in which a repository becomes a benched
// result, as data rather than as control flow.
//
// It exists because that order is the most valuable thing the old tree knows
// and the least tested thing in it. There the order is a 1204-line bash driver:
// a chain of functions, each calling the next, with the state in a JSON file
// beside it. Every lever in the loop — a draft with no anchor, a mini-bench that
// says re-question, a refusal to pay, an authoring ceiling — can only be
// exercised by running the thing that produces those verdicts, which means agent
// sessions, wall time and money. So the routing is the least-tested part of the
// most consequential system.
//
// Declared as data, the whole routing layer is pure: feed it a phase, a verdict
// and a cycle count, assert the transition. No agent, no network, no spend.
//
// # Two invariants this encodes rather than respects
//
// A phase never chooses the next phase. It emits a verdict; [Next] decides. The
// phase that measures a scenario is never the phase that wrote it, and that is
// structural here rather than conventional: no verdict in any enum names a
// phase, so there is no value a phase could emit that selects its own successor.
//
// A phase never spawns. Only the binary spawns, because a phase agent that
// spawns its own judge is grading itself. Nothing in this package spawns
// anything, and nothing it returns tells a caller to.
//
// # An exit code is a claim; the artifact is the fact
//
// [Advance] refuses to move on from a phase whose output artifact is not there,
// whatever the phase reported about itself. The old driver learned this and
// added the check in one place, by hand, at the phase where the defect happened
// to bite. It bites once per phase otherwise, so here it is in the one function
// every phase passes through.
package phase

import "fmt"

// Name is one phase of the loop.
type Name string

// The eleven phases. Handoff is not in the chain: it is reached only at the
// authoring ceiling, or when a benched cell is handed up as a swap.
const (
	Index     Name = "index"
	Author    Name = "author"
	Minibench Name = "minibench"
	Expand    Name = "expand"
	Preflight Name = "preflight"
	Validate  Name = "validate"
	Bench     Name = "bench"
	Report    Name = "report"
	Harvest   Name = "harvest"
	Board     Name = "board"
	Handoff   Name = "handoff"
)

// Done is not a phase. It is what the loop reaches when a win is on the board,
// and it is a name rather than an empty string so that a caller reading a
// transition cannot mistake "finished" for "unset".
const Done Name = "done"

// Verdict is what a phase emits. It is an enum per phase and never free text: a
// phase that can return anything is a phase whose routing cannot be tested.
type Verdict string

// The verdicts. Auto belongs to the phases that make no judgement — they either
// produce their artifact or they do not, and the artifact check is what holds
// them.
const (
	Auto         Verdict = "AUTO"
	Draft        Verdict = "DRAFT"
	NoAnchor     Verdict = "NO-ANCHOR"
	Proceed      Verdict = "PROCEED"
	Requestion   Verdict = "REQUESTION"
	Pay          Verdict = "PAY"
	DoNotPay     Verdict = "DO-NOT-PAY"
	Win          Verdict = "WIN"
	Diagnosis    Verdict = "DIAGNOSIS"
	WinConfirmed Verdict = "WIN CONFIRMED"
	DoDFail      Verdict = "DoD FAIL"
)

// AuthoringCeiling is how many authoring cycles a repository gets before it
// parks for a human. The sixth re-entry goes to [Handoff] rather than back to
// [Author], which is why [Next] cannot be written without the count.
const AuthoringCeiling = 6

// Phase is one phase declared: what it reads, what it writes, and everything it
// is allowed to say.
//
// Reads and Writes are the artifacts, named relative to the phase's directory in
// the run tree. They are here rather than in the runner because the plan files
// are written against them: a plan whose stated input does not match its phase's
// row is a broken plan, and that is only checkable if the row exists.
type Phase struct {
	Name     Name
	Reads    string
	Writes   string
	Verdicts []Verdict
}

// Emits reports whether v is in this phase's enum.
func (p Phase) Emits(v Verdict) bool {
	for _, known := range p.Verdicts {
		if known == v {
			return true
		}
	}
	return false
}

// Graph is every phase, in the order a campaign walks them. Handoff is last
// because it is not walked to: it is arrived at.
var Graph = []Phase{
	{Index, "repo.json", "index.json", []Verdict{Auto}},
	{Author, "slate.md", "scenario.draft.yaml", []Verdict{Draft, NoAnchor}},
	{Minibench, "scenario.draft.yaml", "minibench.md", []Verdict{Proceed, Requestion, NoAnchor}},
	{Expand, "minibench.md", "scenario.yaml", []Verdict{Auto, Requestion}},
	{Preflight, "scenario.yaml", "preflight.json", []Verdict{Auto}},
	{Validate, "scenario.yaml", "pay-call.md", []Verdict{Pay, DoNotPay}},
	{Bench, "scenario.yaml", "cells.json", []Verdict{Auto}},
	{Report, "cells.json", "report.md", []Verdict{Win, Diagnosis}},
	{Harvest, "report.md", "harvest.json", []Verdict{WinConfirmed, DoDFail}},
	{Board, "harvest.json", "board.md", []Verdict{Auto}},
	{Handoff, "campaign.json", "handoff.md", []Verdict{Auto}},
}

// Lookup reports the declared phase named n.
func Lookup(n Name) (Phase, bool) {
	for _, p := range Graph {
		if p.Name == n {
			return p, true
		}
	}
	return Phase{}, false
}

// step keys a transition on the pair that decides it.
type step struct {
	from    Name
	verdict Verdict
}

// reAuthor is every verdict that sends work back to authoring. These are the
// transitions the ceiling bounds, and they are a separate table from the
// forward ones for exactly that reason: a re-entry is the thing being counted.
var reAuthor = map[step]bool{
	{Author, NoAnchor}:      true,
	{Minibench, Requestion}: true,
	// Expansion carries the discriminator step over verbatim. When it cannot —
	// a gold row that does not hold at its line, or a step that cannot be
	// written without naming a gold file — the mini-bench number has stopped
	// describing the scenario, and that is a re-question rather than a phase
	// that quietly writes nothing and stalls the loop.
	{Expand, Requestion}:  true,
	{Minibench, NoAnchor}: true,
	{Validate, DoNotPay}:  true,
}

// forward is every transition that is not a re-entry.
//
// Report's diagnosis goes to handoff rather than back to authoring, and that is
// the shape of the rule that the loop cannot record a loss: it wins, it parks,
// or it hands up a swap with the numbers attached. Harvest's DoD failure goes
// back to report for diagnosis and never to the board, because a cell that
// failed its confirmation checks is not a result to publish.
var forward = map[step]Name{
	{Index, Auto}:           Author,
	{Author, Draft}:         Minibench,
	{Minibench, Proceed}:    Expand,
	{Expand, Auto}:          Preflight,
	{Preflight, Auto}:       Validate,
	{Validate, Pay}:         Bench,
	{Bench, Auto}:           Report,
	{Report, Win}:           Harvest,
	{Report, Diagnosis}:     Handoff,
	{Harvest, WinConfirmed}: Board,
	{Harvest, DoDFail}:      Report,
	{Board, Auto}:           Done,
	{Handoff, Auto}:         Done,
}

// Lever is one routed re-entry, named. The table exists because a done-means of
// "every lever a real campaign has used is exercised" is only checkable against
// a fixed list, and a list held in whoever remembers it is not one.
type Lever struct {
	Name    string
	From    Name
	Verdict Verdict
	Cycle   int
	To      Name
}

// Levers is that list. Any lever a recorded campaign used that is missing from
// here is a hole in the test set, not merely a gap in the documentation.
var Levers = []Lever{
	{"NO-ANCHOR from a draft", Author, NoAnchor, 1, Author},
	{"REQUESTION from a mini-bench", Minibench, Requestion, 1, Author},
	{"NO-ANCHOR from a mini-bench", Minibench, NoAnchor, 1, Author},
	{"REQUESTION from an expansion", Expand, Requestion, 1, Author},
	{"DO-NOT-PAY from a validation", Validate, DoNotPay, 1, Author},
	{"the authoring ceiling", Author, NoAnchor, AuthoringCeiling, Handoff},
	{"DoD FAIL from a harvest", Harvest, DoDFail, 1, Report},
}

// Next reports the phase that follows a verdict, and it is a pure function of
// (phase, verdict, cycle count).
//
// The count is passed in and never held. The sixth NO-ANCHOR parks the
// repository instead of re-entering authoring, so the ceiling cannot be
// expressed without it — and a count this package held would be a count that
// resets when the process does, in a binary that runs unattended.
func Next(from Name, v Verdict, cycle int) (Name, error) {
	p, ok := Lookup(from)
	if !ok {
		return "", fmt.Errorf("no phase named %q", from)
	}
	if !p.Emits(v) {
		return "", fmt.Errorf("phase %s cannot emit %q; it emits %v", from, v, p.Verdicts)
	}
	if cycle < 1 {
		return "", fmt.Errorf("phase %s: authoring cycle %d; the count is 1-based and the first cycle is 1", from, cycle)
	}
	if reAuthor[step{from, v}] {
		if cycle >= AuthoringCeiling {
			return Handoff, nil
		}
		return Author, nil
	}
	// Every verdict of every phase is in one of the two tables, and a test walks
	// the graph to keep it that way. There is no third case to handle here: a
	// verdict added without a transition fails that test rather than returning a
	// runtime error nobody would see until a campaign stalled.
	return forward[step{from, v}], nil
}

// Outcome is everything a phase reports. There is no field here naming a
// successor, and that absence is the invariant: a phase says what happened, and
// the graph says what happens next.
type Outcome struct {
	Phase   Name
	Verdict Verdict
	// Cycle is which authoring cycle this repository is on, 1-based.
	Cycle int
}

// Advance validates an outcome against the graph and reports the next phase.
//
// wrote reports whether the phase's output artifact is on disk. It is injected
// rather than read here so this package stays pure, and it is consulted for
// every phase rather than for the ones where a missing artifact has already
// caused an incident.
//
// The phase's exit code is deliberately absent. A phase that reported success
// and wrote nothing has not finished, and a phase that reported failure and
// wrote its artifact has: the artifact is the fact.
func Advance(o Outcome, wrote func(artifact string) bool) (Name, error) {
	p, ok := Lookup(o.Phase)
	if !ok {
		return "", fmt.Errorf("no phase named %q", o.Phase)
	}
	if !wrote(p.Writes) {
		return "", fmt.Errorf("phase %s did not write %s; an exit code is a claim and the artifact is the fact",
			p.Name, p.Writes)
	}
	return Next(o.Phase, o.Verdict, o.Cycle)
}
