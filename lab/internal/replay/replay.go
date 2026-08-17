// Package replay chooses a banked cell to re-run live and rules on what came
// back.
//
// Everything measured so far has been measured against recordings: the scorer
// reproduces recorded scores, the gates reproduce recorded decisions, the miner
// reproduces misses found by hand. All of that proves the pure layer reads the
// past correctly, and none of it proves the instrument can produce a result.
// Between a recorded transcript and a live cell sit isolation, supervision, the
// tee, the session, the judge and the verdict layer, and every one of them is
// new.
//
// # The model has to match, and that is the trap
//
// A banked number is a number for a model. There is a recorded case: a banked
// mastodon cell measured +0.625 on one model and 0.00 on its successor, because
// the newer baseline arm closed the axis the scenario tested. Replayed on the
// wrong model that cell looks like a broken instrument while being a working one
// measuring a changed world.
//
// So [Pick] pins the model. If the banked model is unreachable it picks a
// different CELL, never a different model, and it reports the cell it could not
// replay: a number banked on a retired model is history rather than a live
// result.
//
// # Agreement is a band, not a number
//
// The measured same-cell spreads run from 0.077 to 0.250, so agreement means
// landing inside the cell's own recorded spread rather than reproducing its
// margin exactly.
//
// # What a disagreement means, in order
//
// First suspect the new instrument, then the environment, then the world. Not
// the reverse: the whole point of choosing a banked cell is that its answer is
// known, so a replay that disagrees is a bug in sense-lab until something else
// is demonstrated.
//
// And that ordering needs a stopping point or it is a licence to debug
// indefinitely. When the instrument and the environment both check out, the next
// step is to run the pair again. One live pair is n=1 against a spread that
// reaches 0.250, so a single disagreement is inside ordinary variance, and two
// independent pairs disagreeing the same way is a different claim from one.
package replay

import (
	"fmt"
	"slices"
	"strings"
)

// Bar is the margin a cell must clear to be a win. A replay is judged against
// the banked margin rather than against this, but the choice of cell is not: a
// cell that banked near the bar cannot tell a broken instrument from ordinary
// variance.
const Bar = 0.50

// Cell is one banked result: what it measured, on which model, with what
// run-to-run spread, and the scenario it measured.
type Cell struct {
	Repo  string
	Model string
	// Margin is the banked delta on the discriminator group.
	Margin float64
	// Spread is the widest gap between that cell's own runs. It is the band
	// agreement is judged in.
	Spread float64
	// Scenario pins the version that was measured. A replay of a different
	// scenario is not a replay.
	Scenario string
}

// Unambiguous reports whether this cell's worst run is still clear of the bar.
//
// A cell that banked near the bar cannot distinguish a broken instrument from
// ordinary variance, so it cannot answer the question a replay is asking.
func (c Cell) Unambiguous() bool { return c.Margin-c.Spread > Bar }

func (c Cell) String() string {
	return fmt.Sprintf("%s on %s: banked %.3f, spread %.3f, scenario %s",
		c.Repo, c.Model, c.Margin, c.Spread, c.Scenario)
}

// Skipped is a banked cell that cannot be replayed, and why. It is reported
// rather than dropped: that a cell can no longer be replayed is itself a fact
// about the corpus.
type Skipped struct {
	Cell Cell
	Why  string
}

func (s Skipped) String() string { return s.Cell.String() + " — " + s.Why }

// Pick chooses the cell to replay from the banked corpus, given the models that
// can still be reached.
//
// It picks the cell whose worst run is furthest clear of the bar, because a
// disagreement there is unambiguous. Every cell it did not pick is reported with
// the reason, so a corpus that has quietly become unreplayable says so.
func Pick(banked []Cell, reachable []string) (Cell, []Skipped, error) {
	var best Cell
	var found bool
	var skipped []Skipped
	for _, c := range banked {
		switch {
		case !slices.Contains(reachable, c.Model):
			skipped = append(skipped, Skipped{c, fmt.Sprintf("%s is unreachable, and a banked number is a number for a model", c.Model)})
		case !c.Unambiguous():
			skipped = append(skipped, Skipped{c, fmt.Sprintf("its worst run lands at %.3f against a bar of %.2f, so a disagreement could be ordinary variance",
				c.Margin-c.Spread, Bar)})
		case !found || c.Margin-c.Spread > best.Margin-best.Spread:
			if found {
				skipped = append(skipped, Skipped{best, "another cell is further clear of the bar"})
			}
			best, found = c, true
		default:
			skipped = append(skipped, Skipped{c, "another cell is further clear of the bar"})
		}
	}
	if !found {
		return Cell{}, skipped, fmt.Errorf("no banked cell can be replayed: %d were considered, and picking a different model instead of a different cell is what this refuses",
			len(banked))
	}
	return best, skipped, nil
}

// Verdict is what a live pair said about a banked cell.
type Verdict struct {
	Cell     Cell
	Measured float64
	Agrees   bool
	// StillAWin says the live pair clears the bar. It is separate from Agrees:
	// a cell can land outside its recorded spread and still be a win, and those
	// are different facts.
	StillAWin bool
}

func (v Verdict) String() string {
	verb := "DISAGREES with"
	if v.Agrees {
		verb = "agrees with"
	}
	return fmt.Sprintf("%.3f %s the banked %.3f ±%.3f on %s",
		v.Measured, verb, v.Cell.Margin, v.Cell.Spread, v.Cell.Repo)
}

// Compare rules on a live measurement against its banked cell.
//
// Agreement is landing inside the cell's own recorded spread, in either
// direction. Demanding the banked number exactly would fail on the instrument's
// own noise; allowing anything above it would let a broken measurement pass by
// being generous.
func Compare(c Cell, measured float64) Verdict {
	return Verdict{
		Cell:      c,
		Measured:  measured,
		Agrees:    measured >= c.Margin-c.Spread && measured <= c.Margin+c.Spread,
		StillAWin: measured >= Bar,
	}
}

// Step is what to do next about a disagreement.
type Step string

const (
	// Settled: the replay agreed and there is nothing to investigate.
	Settled Step = "the replay agreed; nothing to investigate"
	// SuspectTheInstrument is always first. The cell's answer is known, so a
	// disagreement is a bug in sense-lab until something else is demonstrated.
	SuspectTheInstrument Step = "suspect sense-lab: the cell's answer is known, so a disagreement is a bug here until something else is shown"
	// SuspectTheEnvironment is second: the checkout, the index, the subject, the
	// channels the arm actually had.
	SuspectTheEnvironment Step = "suspect the environment: the checkout, the index, the subject and the channels each arm reached"
	// RunASecondPair is the stopping point. One pair is n=1 against a spread
	// that reaches 0.250.
	RunASecondPair Step = "run the pair once more: one live pair is n=1, and the measured same-cell spread reaches 0.250"
	// TheWorldMoved is reached only after all three. It is a real answer and it
	// is the last one, not the first.
	TheWorldMoved Step = "the world moved: two independent pairs disagree the same way, and the instrument and environment both check out"
)

// Checks is what has been ruled out so far, and how many live pairs have been
// run.
type Checks struct {
	InstrumentClean  bool
	EnvironmentClean bool
	// Pairs is how many independent live pairs have disagreed the same way.
	Pairs int
}

// Investigate reports the next step, in the one order that does not let a
// conclusion about the world be reached first.
func Investigate(v Verdict, done Checks) Step {
	switch {
	case v.Agrees:
		return Settled
	case !done.InstrumentClean:
		return SuspectTheInstrument
	case !done.EnvironmentClean:
		return SuspectTheEnvironment
	case done.Pairs < 2:
		return RunASecondPair
	}
	return TheWorldMoved
}

// Inputs are what a replay pins. Every one of them is a thing that, changed,
// makes the comparison meaningless.
type Inputs struct {
	Model    string
	Scenario string
	Wall     string
	Prompt   string
	Gold     string
	Subject  string
}

// Drift reports every pinned input that differs between the banked run and the
// replay.
//
// It exists because "nothing was adjusted to make it land" is a claim, and a
// replay tuned until it agrees has proven that tuning works. This is how the
// claim is checked rather than asserted.
func Drift(banked, live Inputs) []string {
	var out []string
	for _, f := range []struct {
		name          string
		before, after string
	}{
		{"model", banked.Model, live.Model},
		{"scenario", banked.Scenario, live.Scenario},
		{"wall", banked.Wall, live.Wall},
		{"prompt", banked.Prompt, live.Prompt},
		{"gold", banked.Gold, live.Gold},
		{"subject", banked.Subject, live.Subject},
	} {
		if f.before != f.after {
			out = append(out, fmt.Sprintf("%s: banked %q, replayed %q", f.name, f.before, f.after))
		}
	}
	return out
}

// Pinned reports whether nothing moved, and says what did.
func Pinned(banked, live Inputs) (bool, string) {
	drift := Drift(banked, live)
	if len(drift) == 0 {
		return true, "every pinned input matches the banked run"
	}
	return false, "this is not a replay: " + strings.Join(drift, "; ")
}
