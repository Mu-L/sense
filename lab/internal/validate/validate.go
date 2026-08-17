// Package validate decides whether a candidate change to Sense earned its way
// in.
//
// "Earned" has a definition and it is three things: the change moves the cells
// its hypothesis named, it does not regress the ones it did not, and the
// movement is not confined to one model.
//
// # The corpus is small and that matters
//
// The banked corpus is four cells. A regression suite of four is thin enough
// that a change can pass it and still be wrong, so every result reports the
// corpus size alongside it: a pass reads as "four of four held" rather than as
// "no regressions". A result that hid the denominator would be the same claim
// with the doubt removed.
//
// # The blind spot is measured, not assumed
//
// The corpus catches a break in something it covers. It says nothing about a
// break in something it does not, which with four cells is the likelier real
// case. So an [Outcome] records whether the surface the candidate touches is
// exercised by the corpus at all. "The corpus does not cover this" is worth more
// than the pass, because it is what says how much a green regression run means.
//
// # Nothing here ships anything
//
// This produces a verdict and its evidence. The numbers decide the verdict; a
// human decides to ship. There is no automatic acceptance, and a rejection is a
// result that is recorded with its numbers rather than discarded, so the next
// person with the same idea finds the measurement instead of repeating it.
package validate

import (
	"fmt"
	"slices"
	"strings"
)

// Direction is what a hypothesis predicted about a cell.
type Direction string

const (
	// Up predicts the margin grows: the change helps this cell.
	Up Direction = "up"
	// Down predicts it shrinks, which is what a candidate that trades one cell
	// for another says out loud rather than discovers.
	Down Direction = "down"
)

// Target is a cell the hypothesis named, and what it predicted.
type Target struct {
	Cell    string
	Model   string
	Before  float64
	After   float64
	Predict Direction
}

// Moved reports whether this cell went the way the hypothesis said.
//
// A cell that did not move at all has not moved in the predicted direction. A
// hypothesis that survives "nothing happened" predicts nothing.
func (t Target) Moved() bool {
	switch t.Predict {
	case Up:
		return t.After > t.Before
	case Down:
		return t.After < t.Before
	}
	return false
}

func (t Target) String() string {
	return fmt.Sprintf("%s on %s: %.3f to %.3f, predicted %s", t.Cell, t.Model, t.Before, t.After, t.Predict)
}

// Banked is one cell the corpus already holds: the margin it was banked at, the
// run-to-run spread it was banked with, and the surfaces its runs exercised.
type Banked struct {
	Cell   string
	Model  string
	Margin float64
	// Spread is the recorded run-to-run variance for this cell. A cell that
	// lands inside its own spread has held: a regression suite that demanded an
	// exact number would fail on the instrument's own noise.
	Spread float64
	// Surfaces are the Sense surfaces this cell's runs exercised. They are what
	// makes the blind spot measurable rather than assumed.
	Surfaces []string
}

// Held reports whether a re-measured margin is still inside this cell's spread.
func (b Banked) Held(after float64) bool { return after >= b.Margin-b.Spread }

// Regression is one banked cell re-measured.
type Regression struct {
	Banked Banked
	After  float64
}

func (r Regression) Held() bool { return r.Banked.Held(r.After) }

func (r Regression) String() string {
	verb := "held"
	if !r.Held() {
		verb = "FELL"
	}
	return fmt.Sprintf("%s on %s %s: banked %.3f ±%.3f, now %.3f",
		r.Banked.Cell, r.Banked.Model, verb, r.Banked.Margin, r.Banked.Spread, r.After)
}

// Candidate is what is being measured: the surface it changes, the cells it
// predicted, and the corpus it is checked against.
type Candidate struct {
	// Surface is the Sense surface the change touches. It is what the blind spot
	// is measured against.
	Surface     string
	Targets     []Target
	Regressions []Regression
}

// Outcome is the verdict and everything behind it.
type Outcome struct {
	Decision string
	// Reasons is every reason the decision was reached, not the first. An author
	// who fixes one thing only to be refused by the next has learned nothing
	// about the change.
	Reasons []string
	// CorpusSize travels with every result. Four of four held and forty of forty
	// held are different claims.
	CorpusSize int
	// Models is every model the movement was seen on, and CrossModel says there
	// was more than one. A candidate validated on a single model is recorded as
	// single-model, never as confirmed.
	Models     []string
	CrossModel bool
	// BlindSpot says the corpus exercises none of the surface this change
	// touches, so a pass says nothing about it. Recorded as a fact rather than
	// left implicit.
	BlindSpot bool
	Targets   []Target
	Corpus    []Regression
}

// The verdicts. There is no third: a validation that neither accepts nor rejects
// has not finished.
const (
	Accepted = "accepted"
	Rejected = "rejected"
)

// Reason is the whole verdict in one line, and it always carries the
// denominator.
func (o Outcome) Reason() string {
	head := fmt.Sprintf("%s: %d of %d banked cells held", o.Decision, o.heldCount(), o.CorpusSize)
	if !o.CrossModel {
		head += ", single-model"
	}
	if o.BlindSpot {
		head += ", and the corpus exercises none of the changed surface"
	}
	if len(o.Reasons) == 0 {
		return head
	}
	return head + "; " + strings.Join(o.Reasons, "; ")
}

func (o Outcome) heldCount() int {
	var n int
	for _, r := range o.Corpus {
		if r.Held() {
			n++
		}
	}
	return n
}

// Run measures a candidate and returns the verdict with its evidence.
//
// It refuses a candidate whose corpus is nothing but the cells its own
// hypothesis named: a change measured only against its own intent has been
// measured against nothing.
func Run(c Candidate) (Outcome, error) {
	if len(c.Targets) == 0 {
		return Outcome{}, fmt.Errorf("a candidate that predicts no cell; there would be nothing for it to be right or wrong about")
	}
	if err := independent(c); err != nil {
		return Outcome{}, err
	}

	o := Outcome{
		Decision:   Accepted,
		CorpusSize: len(c.Regressions),
		Targets:    c.Targets,
		Corpus:     c.Regressions,
		Models:     modelsOf(c),
	}
	o.CrossModel = len(o.Models) > 1
	o.BlindSpot = blindTo(c)

	for _, t := range c.Targets {
		if !t.Moved() {
			o.reject(fmt.Sprintf("%s did not move as predicted", t))
		}
	}
	for _, r := range c.Regressions {
		if !r.Held() {
			o.reject(r.String())
		}
	}
	return o, nil
}

func (o *Outcome) reject(why string) {
	o.Decision = Rejected
	o.Reasons = append(o.Reasons, why)
}

// independent refuses a corpus that is only the candidate's own targets.
//
// A change validated on the cells its hypothesis named, with nothing else behind
// it, is a change measured against its own intent. The corpus has to contain at
// least one cell the candidate did not ask about.
func independent(c Candidate) error {
	if len(c.Regressions) == 0 {
		return fmt.Errorf("a candidate checked against no banked cell is measured against its own intent")
	}
	named := map[string]bool{}
	for _, t := range c.Targets {
		named[t.Cell+"/"+t.Model] = true
	}
	for _, r := range c.Regressions {
		if !named[r.Banked.Cell+"/"+r.Banked.Model] {
			return nil
		}
	}
	return fmt.Errorf("every banked cell in this corpus is one the hypothesis named; a change measured only against the cells it targeted is measured against its own intent")
}

// modelsOf is every model a target actually moved on. It is the movement that
// has to be cross-model, not the measurement: running an unchanged cell on five
// models confirms nothing.
func modelsOf(c Candidate) []string {
	var out []string
	for _, t := range c.Targets {
		if t.Moved() && !slices.Contains(out, t.Model) {
			out = append(out, t.Model)
		}
	}
	slices.Sort(out)
	return out
}

// blindTo reports whether the corpus exercises none of the surface the candidate
// changes.
//
// A candidate that names no surface is blind by definition: nothing can be said
// about coverage of a surface nobody stated.
func blindTo(c Candidate) bool {
	if c.Surface == "" {
		return true
	}
	for _, r := range c.Regressions {
		if slices.Contains(r.Banked.Surfaces, c.Surface) {
			return false
		}
	}
	return true
}
