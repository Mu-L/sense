// Package stage is the loop in the words an operator uses.
//
// The graph declares eleven phases and every one of them is named for its
// mechanism: minibench, expand, preflight, harvest. Those are the right names
// inside the binary and the wrong ones on a terminal, where the only questions
// are how far in am I, is this going well, and how much is left. Five stages
// answer all three, and the eleven phases sit underneath them unchanged.
//
// The mapping is declared here rather than as a field on [phase.Phase] because
// a stage is how the loop is described and not what it is: nothing routes on a
// stage, no phase behaves differently for being in one, and a field would still
// need the same walk over the graph to mean anything. What holds the two
// together is that walk, in the test — every phase in [phase.Graph] belongs to
// exactly one stage, so a phase added to the graph without a stage fails rather
// than printing as a blank.
package stage

import (
	"fmt"
	"strings"

	"github.com/luuuc/sense/lab/internal/phase"
)

// Stage is one of the five, in the order a repository walks them.
type Stage struct {
	// Number is what it is called on the page: 1 to 5. Admission is 0 and is
	// not on the map.
	Number int
	Name   string
	// What happens here, in one line, in the second person's terms rather than
	// the mechanism's.
	What string
	// Phases is every phase of the graph this stage covers.
	Phases []phase.Name
}

// Admitting is what happens before stage 1, and it is deliberately not one of
// the five.
//
// A repository is cloned, pinned and indexed once in its life, and a map that
// showed it as a sixth stage would show every repository after its first
// command as permanently past a step it will never take again. It is named
// here so the walk over the graph still covers [phase.Index], which is a phase
// like any other and must not be forgotten by being invisible.
var Admitting = Stage{
	Name:   "Admitting",
	What:   "clone the repository and index it with Sense",
	Phases: []phase.Name{phase.Index},
}

// Stages is the five, in order. The text is the whole point of this package, so
// it is data here rather than format strings at the call sites.
var Stages = []Stage{
	{1, "Question", "write a question Sense should answer better than grep",
		[]phase.Name{phase.Author}},
	{2, "Trial", "test it cheaply on two throwaway agents",
		[]phase.Name{phase.Minibench}},
	{3, "Rehearsal", "run the full session unpaid, decide if it's worth paying for",
		[]phase.Name{phase.Expand, phase.Preflight, phase.Validate}},
	{4, "Paid run", "the real measurement, on real money",
		[]phase.Name{phase.Bench}},
	{5, "Verdict", "judge it, check it, record it",
		[]phase.Name{phase.Report, phase.Harvest, phase.Board, phase.Handoff}},
}

// Of reports the stage a phase belongs to.
//
// The second return is false for a name that is not a phase of the graph, which
// includes [phase.Done]: a repository that has finished is not in a stage, and
// answering with stage 5 would mark the map as though the last one were still
// running.
func Of(n phase.Name) (Stage, bool) {
	for _, s := range append([]Stage{Admitting}, Stages...) {
		for _, p := range s.Phases {
			if p == n {
				return s, true
			}
		}
	}
	return Stage{}, false
}

// nameWidth is the longest stage name, so the descriptions line up. It is
// computed rather than counted by hand: a renamed stage that pushed the column
// out would otherwise be found by reading a screenshot.
func nameWidth() int {
	w := 0
	for _, s := range Stages {
		if len(s.Name) > w {
			w = len(s.Name)
		}
	}
	return w
}

// Map is the five stages, one per line, with the one a phase belongs to marked.
//
// It is the entry screen's version: printed when a command starts, where the
// reader may be seeing the arc for the first time and the question is what the
// whole journey is. Everywhere after that is [Line].
func Map(at phase.Name) string {
	here, _ := Of(at)
	var b strings.Builder
	for _, s := range Stages {
		fmt.Fprintf(&b, "%s %d. %-*s  %s\n", marker(s, here), s.Number, nameWidth(), s.Name, s.What)
	}
	return b.String()
}

// Line is the same five on one line, for a screen that has already shown the
// map. A reader who has seen the arc needs the position, not the arc again.
func Line(at phase.Name) string {
	here, _ := Of(at)
	parts := make([]string, 0, len(Stages))
	for _, s := range Stages {
		name := fmt.Sprintf("%d. %s", s.Number, s.Name)
		if s.Number == here.Number {
			name = "▸ " + name
		}
		parts = append(parts, name)
	}
	return strings.Join(parts, " ─ ")
}

// marker is the arrow, or the space that keeps every other line in the same
// column. A map whose rows shifted by two characters as the loop advanced would
// read as a different map each time.
func marker(s, here Stage) string {
	if s.Number == here.Number {
		return "  ▸"
	}
	return "   "
}

// Outcome is what a phase's verdict amounts to, for somebody watching.
//
// The verdict itself is the right word inside the graph and the wrong one on a
// terminal: REQUESTION names the transition exactly and tells a reader nothing
// about whether to worry. What a reader needs is whether this was good, what
// happened, and — when it was not good — whether it is a normal part of the
// method or the end of the road.
type Outcome struct {
	// Good is whether this is the direction the loop wants to go. It decides
	// the mark on the line and nothing else.
	Good bool
	// Line is what happened, in one sentence.
	Line string
	// More is the sentence a reader needs only when something went the other
	// way: what it means and what follows. Empty for the ordinary case, because
	// a screen that explains every step is a screen nobody finishes reading.
	More string
}

// outcomes is every verdict the graph can emit, in the operator's words.
//
// Declared against the graph's own pairs rather than free text at the call
// site, and a test walks [phase.Graph] to fail on a verdict that has no entry:
// a phase gaining a verdict would otherwise print a blank line at the exact
// moment somebody is watching to see what happened.
var outcomes = map[step]Outcome{
	{phase.Index, phase.Auto}:   {true, "Indexed.", ""},
	{phase.Author, phase.Draft}: {true, "Question written.", ""},
	{phase.Author, phase.NoAnchor}: {false, "No question can be anchored in this repository.",
		"Nothing here separates what Sense knows from what a text search finds."},
	{phase.Minibench, phase.Proceed}: {true, "Sense won the trial.", ""},
	{phase.Minibench, phase.Requestion}: {false, "The question does not work yet.",
		"This is normal and it is why the trial is cheap: the next attempt rewrites it."},
	{phase.Minibench, phase.NoAnchor}: {false, "The trial could not be run against this question.",
		"The question goes back to be written again."},
	{phase.Expand, phase.Auto}: {true, "Grown into a full session.", ""},
	{phase.Expand, phase.Requestion}: {false, "The full session would not hold the question.",
		"It goes back to be written again."},
	{phase.Preflight, phase.Auto}: {true, "Everything can run.", ""},
	{phase.Preflight, phase.Blocked}: {false, "It cannot run yet.",
		"Nothing here is wrong with the question: something the loop does not own has to " +
			"be declared first, and the check says which."},
	{phase.Validate, phase.Pay}: {true, "The rehearsal says it is worth paying for.", ""},
	{phase.Validate, phase.DoNotPay}: {false, "The rehearsal says it is not worth paying for.",
		"No money was spent, which is what the rehearsal is for."},
	{phase.Bench, phase.Auto}: {true, "The paid cells ran.", ""},
	{phase.Report, phase.Win}: {true, "A win.", ""},
	{phase.Report, phase.Diagnosis}: {false, "Not a win.",
		"The write-up says what happened, and the result is recorded rather than dropped."},
	{phase.Harvest, phase.WinConfirmed}: {true, "All five checks pass.", ""},
	{phase.Harvest, phase.DoDFail}: {false, "The win did not survive its checks.",
		"It goes back to be read again rather than onto the board."},
	{phase.Board, phase.Auto}:   {true, "On the board.", ""},
	{phase.Handoff, phase.Auto}: {true, "Written up and handed on.", ""},
}

// step is one phase and one verdict it emitted.
type step struct {
	Phase   phase.Name
	Verdict phase.Verdict
}

// Says reports what a verdict amounts to. The second return is false for a
// pair the graph does not declare, which a caller prints as the raw verdict
// rather than as silence.
func Says(p phase.Name, v phase.Verdict) (Outcome, bool) {
	o, ok := outcomes[step{p, v}]
	return o, ok
}
