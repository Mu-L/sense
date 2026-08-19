// Package plan expands one repository's bench into the jobs it implies and
// answers, for each one, whether it can run at all.
//
// It exists so that an impossible combination is found before anything is
// spent, rather than forty minutes into a session, or worse at the end of a
// cell whose other arm already completed and can now never be paired. Two
// recorded failures have this shape: an arm whose model id resolved to nothing
// and returned empty at zero tokens, and a half-pair whose finished arm was
// burned because its partner was never going to run.
//
// It is pure: config in, a list of jobs and a list of named rejections out. No
// network, no spawn, nothing on disk beyond the config it was handed.
package plan

import (
	"fmt"
	"strings"

	"github.com/luuuc/sense/lab/internal/catalog"
)

// Role is what an arm is for.
type Role string

const (
	// Headline is the arm a win is claimed on. Exactly one per repository.
	Headline Role = "headline"
	// Confirmation is a cross-model check that the win is not an artifact of
	// one model.
	Confirmation Role = "confirmation"
)

// Arm is one model the repository is benched on, and how many times per cell.
type Arm struct {
	Role  Role   `json:"role"`
	Model string `json:"model"`
	Runs  int    `json:"runs"`
	// Agent names the tool to drive the model with. It may be empty when the
	// model names exactly one, which is the normal case.
	Agent string `json:"agent"`
}

// Bench is how one repository is measured: which arms, with which subjects,
// graded by which judge.
//
// The scope is the repository because everything else already is. A repository
// carries its own run tree, its own authoring cycles and its own spend ceiling,
// and it advances to a verdict on its own — no second repository is authored
// while the first is mid-diagnosis. A container above it held no fact that was
// not either a decision about one repository or a directory name.
type Bench struct {
	Repo string `json:"repo"`
	// Judge grades the rubric axes. It is NOT an arm: it produces no cell and
	// it is a service the scoring layer calls. It lives here as a pinned field
	// because a judge that moves with the arm makes every board incomparable,
	// and because leaving it in the arm list would mean every consumer skipping
	// it by convention, forever.
	Judge    string   `json:"judge"`
	Subjects []string `json:"subjects"`
	Arms     []Arm    `json:"arms"`
}

// Job is one cell's worth of work: one scenario's repository, one subject, one
// agent tool, one model, one executor, run N times.
type Job struct {
	// cell groups the jobs that must run together: one repository and one arm,
	// across every subject. It is keyed on the arm's POSITION rather than on
	// resolved fields, because a job rejected before its agent was chosen would
	// otherwise key differently from its own partner.
	cell     string
	Repo     string
	Subject  string
	Agent    string
	Model    string
	Executor string
	// Auth is the mode chosen to reach this model, not merely one that could
	// work. Cycle 03's executor provisions it rather than guessing.
	Auth string
	Role Role
	Runs int
}

func (j Job) String() string {
	return fmt.Sprintf("%s / %s / %s / %s / %s / %s x%d",
		j.Repo, j.Subject, j.Agent, j.Model, j.Executor, j.Auth, j.Runs)
}

// Rejection is a job that cannot run, and why. The reason is always one of the
// resolution questions: a rejection without a reason is a mystery someone will
// route around by guessing.
type Rejection struct {
	Job    Job
	Reason string
}

func (r Rejection) String() string { return r.Job.String() + "\n    " + r.Reason }

// Result is what a bench plans to.
type Result struct {
	Jobs     []Job
	Rejected []Rejection
}

// Cells is how many cells the plan runs: a cell is one arm across every
// subject, which is the unit that is paired, budgeted and banked. It is NOT the
// number of jobs — a two-subject bench runs two jobs per cell.
func (r Result) Cells() int {
	seen := map[string]bool{}
	for _, j := range r.Jobs {
		seen[j.cell] = true
	}
	return len(seen)
}
func (r Result) Runs() int {
	var n int
	for _, j := range r.Jobs {
		n += j.Runs
	}
	return n
}

// Expand turns a repository's bench into every job it implies and resolves each
// one.
//
// Order is subject, then arm, so the output reads the way the bench is run:
// both arms of a cell adjacent.
func Expand(c *catalog.Catalog, b Bench) (Result, error) {
	if err := b.validate(c); err != nil {
		return Result{}, err
	}

	// One ordered pass, decided afterwards, so the output reads in the order the
	// bench is run rather than grouped by outcome.
	var walked []Rejection
	for _, subject := range b.Subjects {
		for i, arm := range b.Arms {
			j, reason := resolve(c, b.Repo, subject, arm)
			j.cell = fmt.Sprintf("%s#%d", b.Repo, i)
			walked = append(walked, Rejection{Job: j, Reason: reason})
		}
	}
	return decide(walked, b.Subjects), nil
}

// decide splits a walked matrix into what runs and what does not, rejecting the
// survivors of any cell that lost a subject.
//
// That last part is the hazard the whole pitch is named for, and without it the
// planner CREATES it rather than preventing it: if one subject of a cell
// resolves and the other does not, planning only the survivor guarantees a
// burned arm. The finished side can never be paired, and the baseline's budget
// derives from its partner's wall, so there is nothing to derive it from.
//
// Rejecting the survivor is the right direction. A run that never happens costs
// nothing; a run that happens and cannot be paired costs its whole spend and
// yields no result.
func decide(walked []Rejection, subjects []string) Result {
	// The SET of subjects present per cell, not how many jobs there are. A
	// count is a proxy that a duplicated subject satisfies without a partner
	// ever existing: two untreated arms and no sense arm counts as two.
	planned := map[string]map[string]bool{}
	// Why each incomplete cell is incomplete, so a survivor's rejection names
	// the reason its partner failed rather than merely reporting that one did.
	lost := map[string]string{}
	for _, w := range walked {
		if w.Reason != "" {
			if _, ok := lost[w.Job.cell]; !ok {
				lost[w.Job.cell] = w.Job.Subject + ": " + w.Reason
			}
			continue
		}
		if planned[w.Job.cell] == nil {
			planned[w.Job.cell] = map[string]bool{}
		}
		planned[w.Job.cell][w.Job.Subject] = true
	}

	var res Result
	for _, w := range walked {
		switch {
		case w.Reason != "":
			res.Rejected = append(res.Rejected, w)
		case len(subjects) > 1 && !complete(planned[w.Job.cell], subjects):
			why, ok := lost[w.Job.cell]
			if !ok {
				// Nothing in this cell was rejected, so it is short a subject
				// for a reason the bench chose: one listed twice, or one
				// missing.
				why = "the bench does not name every subject for this cell"
			}
			res.Rejected = append(res.Rejected, Rejection{Job: w.Job,
				Reason: fmt.Sprintf("its cell is incomplete, so running this would burn it: %s", why)})
		default:
			res.Jobs = append(res.Jobs, w.Job)
		}
	}
	return res
}

func complete(present map[string]bool, subjects []string) bool {
	for _, s := range subjects {
		if !present[s] {
			return false
		}
	}
	return true
}

// validate refuses a bench that is malformed rather than merely unsatisfiable.
// A bench naming a subject the catalog does not have is a typo; a bench whose
// model no tool can drive is a rejection with a reason, and those are different
// answers.
func (b Bench) validate(c *catalog.Catalog) error {
	if err := b.validateArms(); err != nil {
		return err
	}
	if err := b.validateNothingIsNamedTwice(); err != nil {
		return err
	}
	return b.validateReferences(c)
}

// validateArms checks the arm set can be read as one: exactly one arm to claim
// a win on, and no arm that never runs.
//
// EVERY ARM GETS 2 RUNS PER CELL is a spending law and belongs to the gates,
// not here. What this refuses is a bench nobody could interpret.
func (b Bench) validateArms() error {
	var heads int
	for _, a := range b.Arms {
		switch a.Role {
		case Headline:
			heads++
		case Confirmation:
		default:
			return fmt.Errorf("arm %s: role %q is not headline or confirmation", a.Model, a.Role)
		}
		if a.Runs < 1 {
			return fmt.Errorf("arm %s: %d runs; an arm that never runs is not an arm", a.Model, a.Runs)
		}
	}
	if heads != 1 {
		return fmt.Errorf("the bench for %s has %d headline arms; exactly one arm is the one a win is claimed on",
			b.Repo, heads)
	}
	return nil
}

// validateReferences checks every id the bench names exists.
//
// A bench naming a subject the catalog does not have is a TYPO; a bench whose
// model no tool can drive is a rejection with a reason. Confusing the two sends
// someone to fix the wrong file.
// validateNothingIsNamedTwice refuses a list that repeats itself.
//
// This is the half-pair hazard in disguise, and it is why the check exists:
// two untreated arms and no sense arm looks like a complete cell to anything
// counting jobs, and the pair guard would pass it.
func (b Bench) validateNothingIsNamedTwice() error {
	if len(b.Subjects) == 0 || len(b.Arms) == 0 {
		return fmt.Errorf("the bench for %s plans nothing: %d subjects, %d arms",
			b.Repo, len(b.Subjects), len(b.Arms))
	}
	if id := firstRepeat(b.Subjects); id != "" {
		return fmt.Errorf("the bench for %s names subject %q twice; a duplicated arm is not a pair",
			b.Repo, id)
	}
	var armKeys []string
	for _, a := range b.Arms {
		armKeys = append(armKeys, a.Model+"/"+a.Agent)
	}
	if key := firstRepeat(armKeys); key != "" {
		return fmt.Errorf("the bench for %s names model %q twice; running one arm twice is not two arms",
			b.Repo, strings.SplitN(key, "/", 2)[0])
	}
	return nil
}

func firstRepeat(ids []string) string {
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			return id
		}
		seen[id] = true
	}
	return ""
}

func (b Bench) validateReferences(c *catalog.Catalog) error {
	if !has(c.Repos, b.Repo) {
		return fmt.Errorf("the bench names repo %q, which no repo file declares", b.Repo)
	}
	if b.Judge == "" {
		return fmt.Errorf("the bench for %s pins no judge; a judge that moves with the arm makes every board incomparable",
			b.Repo)
	}
	if _, ok := c.Model(b.Judge); !ok {
		return fmt.Errorf("the bench for %s pins judge %q, which no model file declares", b.Repo, b.Judge)
	}
	for _, s := range b.Subjects {
		if !has(c.Subjects, s) {
			return fmt.Errorf("the bench for %s names subject %q, which no subject file declares", b.Repo, s)
		}
	}
	for _, a := range b.Arms {
		if _, ok := c.Model(a.Model); !ok {
			return fmt.Errorf("the bench for %s names model %q, which no model file declares", b.Repo, a.Model)
		}
	}
	return nil
}

func has[T any](m map[string]T, id string) bool {
	_, ok := m[id]
	return ok
}
