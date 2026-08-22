package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/plan"
)

// headlineRuns is how many times the headline arm runs per cell.
//
// Two rather than one, and it is not a taste. The recorded spread within one
// cell reaches 0.250 of the group against a bar of 0.50, so a single run is a
// sample of a distribution whose spread is half the bar — a draw rather than a
// reading. The confirmation arm runs once because what is asked of it is
// direction, not margin.
const headlineRuns = 2

// starter writes the bench file a repository does not have yet: a matrix this
// catalog can actually run, for a person to keep or edit.
//
// It writes a DEFAULT and says so, rather than deciding. What a repository is
// measured on is the one judgement in this flow that belongs to a person, and
// the instrument may not make it — but leaving the operator to discover, four
// phases later, that a file they were never told about is missing is not
// leaving the decision to them either. That is what happened on mastodon cycle
// 3: the absence was found at the phase before the money.
//
// Nothing is overwritten. A bench somebody has already written is the decision
// this exists to ask for, and admission is idempotent.
func starter(config, id string, c *catalog.Catalog) (plan.Bench, string, bool, error) {
	at := filepath.Join(config, "benches", id+".json")
	if _, err := os.Stat(at); err == nil {
		b, err := loadBench(config, id)
		return b, at, false, err
	}
	b, err := proposeBench(id, c)
	if err != nil {
		return b, at, false, err
	}
	if err := writeBench(at, b); err != nil {
		return b, at, false, err
	}
	return b, at, true, nil
}

// proposeBench is a matrix built out of what this catalog holds, checked
// against the resolver before it is written.
//
// Checked rather than assumed: a starter that does not resolve is worse than no
// starter, because it reads as something somebody chose.
func proposeBench(id string, c *catalog.Catalog) (plan.Bench, error) {
	subjects, err := starterSubjects(c)
	if err != nil {
		return plan.Bench{}, err
	}
	models := runnableModels(c)
	if len(models) == 0 {
		return plan.Bench{}, errors.New("this catalog declares no models, so there is no arm to propose. " +
			"Add a model, and an agent tool that declares it")
	}

	headline := models[0]
	b := plan.Bench{
		Repo:     id,
		Judge:    headline.ID,
		Driver:   plan.Driver{Model: headline.ID},
		Subjects: subjects,
		Arms:     []plan.Arm{{Role: plan.Headline, Model: headline.ID, Runs: headlineRuns}},
	}
	// A confirmation arm exists to say the result is not an artifact of one
	// model, so it is only worth proposing on a model from another provider. A
	// second model from the same one would confirm less than it appears to.
	if other, ok := otherProvider(models, headline); ok {
		b.Arms = append(b.Arms, plan.Arm{Role: plan.Confirmation, Model: other.ID, Runs: 1})
	}

	res, err := plan.Expand(c, b)
	if err != nil {
		return b, fmt.Errorf("the matrix this catalog implies does not resolve: %w", err)
	}
	if len(res.Rejected) > 0 {
		return b, fmt.Errorf("%d of the %d jobs this catalog implies cannot run, so there is no starter "+
			"worth writing: %s", len(res.Rejected), len(res.Rejected)+len(res.Jobs), res.Rejected[0])
	}
	return b, nil
}

// starterSubjects is the pair a cell is: one arm without Sense against one arm
// with it. A catalog that does not hold exactly one of each cannot be proposed
// from, because which of several a person meant is the decision itself.
func starterSubjects(c *catalog.Catalog) ([]string, error) {
	var baseline, sense []string
	for id, s := range c.Subjects {
		switch s.Kind {
		case catalog.Baseline:
			baseline = append(baseline, id)
		case catalog.Sense:
			sense = append(sense, id)
		case catalog.Competitor:
			// A competitor is a third arm, and a cell is two. Proposing one
			// would propose a comparison the paid step cannot run.
		}
	}
	slices.Sort(baseline)
	slices.Sort(sense)
	if len(baseline) != 1 || len(sense) != 1 {
		return nil, fmt.Errorf("a cell is one subject without Sense against one with it, and this catalog "+
			"holds %v and %v. Name the pair in the bench file yourself", baseline, sense)
	}
	return []string{baseline[0], sense[0]}, nil
}

// runnableModels is every model this catalog holds, in a fixed order.
//
// No filter for whether a tool can drive one. The catalog refuses to load a
// model that names no agent tools, and one that names a tool no agent file
// declares, so every model that reaches here is already drivable — a second
// check would be this file disagreeing with the loader about a rule the loader
// owns.
//
// Sorted by id rather than ranked. There is nothing in a catalog that says one
// model is the better headline, and inventing an order would be the instrument
// making the choice it is supposed to be asking for. The order only has to be
// the same on two runs, so that re-admitting a repository proposes what it
// proposed before.
func runnableModels(c *catalog.Catalog) []catalog.Model {
	out := make([]catalog.Model, 0, len(c.Models))
	for _, m := range c.Models {
		out = append(out, m)
	}
	slices.SortFunc(out, func(a, b catalog.Model) int { return strings.Compare(a.ID, b.ID) })
	return out
}

// otherProvider is the first model from a provider other than the headline's.
func otherProvider(models []catalog.Model, headline catalog.Model) (catalog.Model, bool) {
	for _, m := range models {
		if m.Provider != headline.Provider {
			return m, true
		}
	}
	return catalog.Model{}, false
}

// writeBench writes the starter where the bench for this repository belongs.
func writeBench(at string, b plan.Bench) error {
	if err := os.MkdirAll(filepath.Dir(at), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(b, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(at, append(raw, '\n'), 0o644)
}

// starterTable is the proposal in the words a person reads, so the decision can
// be made from the screen rather than from the JSON.
func starterTable(b plan.Bench, at string, written bool) string {
	out := "  ──────────────────────────────────────────────────────────────────\n\n"
	out += "  One decision before we start, and it is yours: what do we measure?\n\n"
	if written {
		out += fmt.Sprintf("  I have written a starter to %s:\n\n", at)
	} else {
		out += fmt.Sprintf("  %s already says:\n\n", at)
	}
	for _, a := range b.Arms {
		out += fmt.Sprintf("      %-24s%s, run %s%s\n", roleColumn(a.Role), a.Model, times(a.Runs), roleWhy(a.Role))
	}
	// The pair, in the order starterSubjects fixes: the arm without Sense
	// first, because that is the one the win is claimed over.
	out += fmt.Sprintf("      %-24s%s against %s, which is each model with and without Sense\n",
		"the comparison", b.Subjects[0], b.Subjects[1])
	out += fmt.Sprintf("      %-24s%s\n", "the judge", b.Judge)
	out += fmt.Sprintf("      %-24s%s\n", "the stages run by", b.Driver.Model)
	if written {
		out += "\n  Every one of those combinations was checked and can run. These are defaults,\n" +
			"  not choices — keep them, or edit that file first. Either way:\n"
	} else {
		out += "\n  Nothing was changed. To go on:\n"
	}
	return out
}

func roleColumn(r plan.Role) string {
	if r == plan.Confirmation {
		return "a second model"
	}
	return "the headline model"
}

func roleWhy(r plan.Role) string {
	if r == plan.Confirmation {
		return ", to confirm the result holds"
	}
	return ", which the win is claimed on"
}
