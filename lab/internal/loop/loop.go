// Package loop holds a repository's position in the authoring cycle and the
// rules for re-entering it.
//
// Authoring is a cycle, not a line. Most drafts do not become a benched win on
// the first attempt, and what happens on the second attempt is where the method
// lives. Three verdicts send work back — NO-ANCHOR from a draft, REQUESTION from
// a mini-bench, DO-NOT-PAY from a validation — and each one is an opportunity to
// lose the thing that made the previous attempt worth improving.
//
// # Nothing is deleted on re-entry
//
// The instinct when a draft fails is to clear it and start clean. That instinct
// is what this package exists against, and the cost of yielding to it is
// invisible: six unrelated attempts look exactly like six iterations from the
// outside. So a re-entry appends the finished attempt to the history, carries
// the table that rejected it, keeps the anchor, and lets the author rewrite the
// question in place.
//
// # Re-anchoring is its own decision
//
// A sub-floor cell is a question the anchor could not carry, not necessarily a
// bad anchor. A NO-ANCHOR verdict records that an anchor is needed; it does not
// pick one. Only [State.ReAnchor] changes the anchor, and a re-question never
// touches it.
//
// # The ceiling parks, it does not warn
//
// After the sixth authoring cycle on one repository the loop parks. A parked
// repository is not automatically re-enterable: [State.Resume] is a deliberate
// act, and the fresh cycle count is what a human gets if they choose to resume,
// not an automatic second life. Without that, a loop that believes the next
// attempt will work quietly spends six more cycles.
//
// The other ceiling, spend, is not here: it is read from disk by
// lab/internal/budget, because a ceiling held in a variable is a ceiling that
// resets when the process does, and this binary spends money unattended.
package loop

import (
	"fmt"

	"github.com/luuuc/sense/lab/internal/phase"
)

// Rejection is why an attempt was sent back. The table is the thing the next
// attempt has to respond to: the mini-bench read, the credit table that refused
// to pay, the reason a draft had no anchor.
//
// It is never empty on a re-entry, and that is enforced rather than expected. A
// re-entry without its rejection is a fresh guess wearing the previous
// attempt's number.
type Rejection struct {
	Phase   phase.Name
	Verdict phase.Verdict
	Table   string
}

func (r Rejection) String() string {
	return fmt.Sprintf("%s emitted %s: %s", r.Phase, r.Verdict, r.Table)
}

// Attempt is one finished authoring cycle, kept forever.
type Attempt struct {
	Cycle    int
	Anchor   string
	Draft    string
	Rejected Rejection
}

// State is one repository's position in the authoring loop.
type State struct {
	Repo string
	// Cycle is the authoring cycle being run, 1-based. It is the input the
	// ceiling is expressed in.
	Cycle  int
	Anchor string
	Draft  string
	// Attempts is every cycle that has already been rejected, oldest first.
	// Nothing is ever removed from it.
	Attempts []Attempt
	// Carried is every rejection so far, oldest first. The next attempt reads
	// all of them, not only the last: six attempts converging is what this
	// makes possible and six unrelated drafts is what it prevents.
	Carried []Rejection
	// NeedsAnchor says the last verdict was NO-ANCHOR, so the next attempt owes
	// a re-anchoring decision. The previous anchor is still here: recording that
	// one is needed is not the same as discarding the one that failed.
	NeedsAnchor bool
	// Parked says the repository hit the authoring ceiling and is waiting for a
	// human. Because names which ceiling and on what.
	Parked  bool
	Because string
}

// Start opens a repository's first authoring cycle.
func Start(repo, anchor string) State {
	return State{Repo: repo, Cycle: 1, Anchor: anchor}
}

// Drafted records what the author produced for the current cycle. The question
// is rewritten in place; the previous draft is already in Attempts.
func (s State) Drafted(draft string) State {
	s.Draft = draft
	return s
}

// Advance records a phase's verdict and reports where the loop goes next.
//
// wrote is the artifact check the graph refuses to advance without; it is passed
// through so there is one way to leave a phase rather than two, and no path that
// skips the check.
func (s State) Advance(from phase.Name, v phase.Verdict, r Rejection, wrote func(string) bool) (State, phase.Name, error) {
	if s.Parked {
		return s, "", fmt.Errorf("%s is parked (%s); resuming it is a deliberate human action, not a transition",
			s.Repo, s.Because)
	}
	next, err := phase.Advance(phase.Outcome{Phase: from, Verdict: v, Cycle: s.Cycle}, wrote)
	if err != nil {
		return s, "", err
	}
	if !s.isReEntry(from, v) {
		return s, next, nil
	}
	if r.Table == "" {
		return s, "", fmt.Errorf("%s emitted %s and carries no table; a re-entry without the thing that rejected it is a fresh guess",
			from, v)
	}
	r.Phase, r.Verdict = from, v
	if next == phase.Handoff {
		return s.park(r), next, nil
	}
	return s.reEnter(r, v), next, nil
}

// isReEntry reports whether this verdict sends work back to authoring.
//
// It asks the graph at the first cycle rather than at the current one, which is
// what separates a re-entry that the ceiling converted into a park from a
// diagnosis that was always going to be handed up. The two land on the same
// phase and mean opposite things.
func (s State) isReEntry(from phase.Name, v phase.Verdict) bool {
	at, err := phase.Next(from, v, 1)
	return err == nil && at == phase.Author
}

// reEnter files the finished attempt and opens the next one. Nothing is
// removed: the anchor stays, the prior draft is kept in the history, and the
// rejection joins everything the next attempt has to answer.
func (s State) reEnter(r Rejection, v phase.Verdict) State {
	s.Attempts = append(append([]Attempt{}, s.Attempts...),
		Attempt{Cycle: s.Cycle, Anchor: s.Anchor, Draft: s.Draft, Rejected: r})
	s.Carried = append(append([]Rejection{}, s.Carried...), r)
	s.Cycle++
	s.NeedsAnchor = v == phase.NoAnchor
	return s
}

// park files the attempt that hit the ceiling and stops the loop.
func (s State) park(r Rejection) State {
	s = s.reEnter(r, r.Verdict)
	s.Parked = true
	s.Because = fmt.Sprintf("%d authoring cycles on %s, which is the ceiling", phase.AuthoringCeiling, s.Repo)
	return s
}

// ReAnchor is the only way the anchor changes.
//
// It refuses unless a verdict asked for one, so re-anchoring can never happen as
// a side effect of re-questioning. The previous anchor is already in the
// attempt history and is not touched here.
func (s State) ReAnchor(anchor string) (State, error) {
	if !s.NeedsAnchor {
		return s, fmt.Errorf("%s was not asked to re-anchor; re-anchoring is its own decision and never a side effect of a re-question",
			s.Repo)
	}
	if anchor == "" {
		return s, fmt.Errorf("%s: re-anchoring to nothing is not a decision", s.Repo)
	}
	s.Anchor = anchor
	s.NeedsAnchor = false
	return s, nil
}

// Resume re-enters a parked repository, which is the deliberate human action the
// park is waiting for.
//
// The next run gets a fresh cycle count. It keeps everything else: the anchor,
// every attempt and every rejection, because the six cycles already spent are
// what the seventh has to learn from.
func (s State) Resume() (State, error) {
	if !s.Parked {
		return s, fmt.Errorf("%s is not parked; there is nothing to resume", s.Repo)
	}
	s.Parked = false
	s.Because = ""
	s.Cycle = 1
	return s, nil
}

// Manual stops the loop before a named phase and says which artifact it was
// going to produce, so a person can produce it by hand.
//
// It is the one escape hatch and there is deliberately no second. A
// --skip-phase or a --resume-from would be a bypass with a friendlier name, and
// the moment of frustration is exactly when it would be used.
type Manual struct {
	StopAt phase.Name
}

// NewManual refuses a phase the graph does not declare, so a typo stops the
// campaign at the flag rather than never stopping it at all.
func NewManual(at string) (Manual, error) {
	if _, ok := phase.Lookup(phase.Name(at)); !ok {
		return Manual{}, fmt.Errorf("no phase named %q to stop at", at)
	}
	return Manual{StopAt: phase.Name(at)}, nil
}

// Halt reports whether the loop stops before next, and what to hand it.
func (m Manual) Halt(next phase.Name) (string, bool) {
	if m.StopAt == "" || m.StopAt != next {
		return "", false
	}
	p, _ := phase.Lookup(next)
	return fmt.Sprintf("stopped before %s; it awaits %s and writes %s", next, p.Reads, p.Writes), true
}
