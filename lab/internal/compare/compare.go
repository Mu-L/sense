// Package compare renders three or more subjects on one scenario, side by side.
//
// It is pure: it takes scored results and returns a table. The running is
// `run` over a subject list, which the engine already does; what was missing
// was somewhere for the answer to be read.
//
// **The same headline and the same rules as every other output.** Objective
// cited recall, per gold group, with the spread beside it. There is no
// competitor-specific axis here and there is not going to be one: a comparison
// that measured a rival on a scale of its own would be a comparison nobody
// could check against the rest of the corpus.
package compare

import (
	"fmt"
	"sort"
	"strings"
)

// Subject is one treatment's result on one gold group.
type Subject struct {
	// ID is the subject's catalog id.
	ID string
	// Baseline marks the untreated arm, which every delta is measured against.
	// A comparison with no baseline in it has nothing to be a delta from.
	Baseline bool
	// Recall is the run's cited recall for this group, and Runs how many runs
	// it is the mean of. Spread is the gap between the best and worst of them,
	// which is what says whether a delta is bigger than the noise.
	Recall float64
	Runs   int
	Spread float64
	// Executor is where this subject ran, and Containerised says it paid
	// container overhead inside its measured wall. That difference is about
	// isolation and not about code intelligence, so the table states it rather
	// than leaving a reader to infer it.
	Executor      string
	Containerised bool
	// Why says what makes this number provisional, and empty means it is not.
	Why string
}

// Delta is this subject's recall against the baseline's.
func (s Subject) Delta(baseline float64) float64 { return s.Recall - baseline }

// Report is one gold group, every subject on it.
type Report struct {
	Scenario string
	Repo     string
	Group    string
	Model    string
	Wall     string
	Subjects []Subject
}

// Baseline is the untreated arm's recall, and whether there is one at all.
func (r Report) Baseline() (float64, bool) {
	for _, s := range r.Subjects {
		if s.Baseline {
			return s.Recall, true
		}
	}
	return 0, false
}

// Render is the table a human reads.
//
// Subjects are ordered baseline first and then by recall, descending, so the
// row a reader looks for is where they look — and so **a subject that loses is
// in the same table as one that wins**, in its own place. A comparison that
// omitted the places Sense ties or loses would measure nothing, and that rule
// does not soften when the other name is a rival's.
func (r Report) Render() string {
	var b strings.Builder
	fmt.Fprintf(&b, "scenario   %s\n", r.Scenario)
	fmt.Fprintf(&b, "repo       %s\n", r.Repo)
	fmt.Fprintf(&b, "group      %s\n", r.Group)
	fmt.Fprintf(&b, "model      %s\n", r.Model)
	fmt.Fprintf(&b, "budget     %s, the same for every subject\n\n", r.Wall)

	base, hasBase := r.Baseline()
	fmt.Fprintf(&b, "  %-16s %8s %8s %8s %6s  %s\n", "subject", "recall", "delta", "spread", "runs", "where it ran")
	for _, s := range r.ordered() {
		delta := "-"
		if hasBase && !s.Baseline {
			delta = fmt.Sprintf("%+.4f", s.Delta(base))
		}
		fmt.Fprintf(&b, "  %-16s %8.4f %8s %8.4f %6d  %s\n",
			s.ID, s.Recall, delta, s.Spread, s.Runs, s.where())
	}

	if !hasBase {
		b.WriteString("\nNO BASELINE: nothing here is a delta, because there is no untreated arm to be a delta from.\n")
	}
	for _, s := range r.ordered() {
		if s.Why != "" {
			fmt.Fprintf(&b, "\nPROVISIONAL (%s): %s\n", s.ID, s.Why)
		}
	}
	if r.anyContainerised() {
		b.WriteString("\nCONTAINER OVERHEAD: the subjects marked `container` paid their container's setup\n" +
			"inside the measured wall. That difference is about isolation, not about code intelligence.\n")
	}
	b.WriteString("\nNOT PUBLISHED: this comparison describes someone else's product and goes no further\n" +
		"than this repository until that is decided separately.\n")
	return b.String()
}

func (s Subject) where() string {
	if s.Containerised {
		return s.Executor + " (paid container overhead inside the wall)"
	}
	return s.Executor
}

func (r Report) anyContainerised() bool {
	for _, s := range r.Subjects {
		if s.Containerised {
			return true
		}
	}
	return false
}

// ordered is the baseline first, then by recall descending, then by id so two
// subjects that tie do not swap places between renders.
func (r Report) ordered() []Subject {
	out := append([]Subject(nil), r.Subjects...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Baseline != out[j].Baseline {
			return out[i].Baseline
		}
		if out[i].Recall != out[j].Recall {
			return out[i].Recall > out[j].Recall
		}
		return out[i].ID < out[j].ID
	})
	return out
}
