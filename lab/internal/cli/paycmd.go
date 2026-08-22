package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/luuuc/sense/lab/internal/ask"
	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/plan"
	"github.com/luuuc/sense/lab/internal/position"
	"github.com/luuuc/sense/lab/internal/probe"
	"github.com/luuuc/sense/lab/internal/say"
	"github.com/luuuc/sense/lab/internal/scenario"
	"github.com/luuuc/sense/lab/internal/score"
	"github.com/luuuc/sense/lab/internal/stage"
	"github.com/luuuc/sense/lab/internal/transcript"
)

// cellsFile is the paid step's artifact: every cell it ran, and what came of
// each. It is written after EVERY cell rather than at the end, because an
// interrupted matrix that left nothing on disk is exactly how a burned arm gets
// paired by a later pass.
const cellsFile = "cells.json"

// payFlags is the paid step. It takes a repository and nothing else that
// matters: which arms, which scenario, which cell directory and which commit
// are all facts already recorded, and typing them again is how a cell gets run
// against a matrix no file declares.
type payFlags struct {
	config    string
	runs      string
	checkouts string
	senseBin  string
	wall      time.Duration
	yes       bool
	name      string
}

func parsePayFlags(args []string, stderr io.Writer) (payFlags, error) {
	var f payFlags
	fs := flag.NewFlagSet("pay", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configFlag(fs, &f.config)
	fs.StringVar(&f.runs, "runs", defaultRuns, "the root the repositories' run trees live under")
	fs.StringVar(&f.checkouts, "checkouts", defaultCheckouts, "the lab's own clones root")
	fs.StringVar(&f.senseBin, "sense", "sense", "the Sense binary under test")
	fs.DurationVar(&f.wall, "wall", defaultWall, "the sense arm's budget; the baseline's derives from it")
	fs.BoolVar(&f.yes, "yes", false, "spend without being asked, for an unattended run")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	if fs.NArg() != 1 {
		return f, errors.New("name exactly one repository to spend on")
	}
	f.name = fs.Arg(0)
	return f, nil
}

// payCells runs the paid cells a repository's bench declares, scores both arms
// of each, and records what it spent.
func payCells(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	f, err := parsePayFlags(args, stderr)
	if err != nil {
		if err != flag.ErrHelp {
			_, _ = fmt.Fprintf(stderr, "sense-lab pay: %v\n", err)
		}
		return exitUsage
	}

	p, err := resolvePay(f)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab pay: %v\n", err)
		return exitError
	}

	_, _ = fmt.Fprint(stdout, header(p.repo, p.cycle, phase.Bench))
	_, _ = fmt.Fprint(stdout, p.warning())

	asker := ask.Asker{In: stdin, Out: stdout, Assumed: f.yes, Terminal: isTerminal(stdin)}
	if err := asker.Name(p.repo); err != nil {
		if errors.Is(err, ask.ErrDeclined) {
			_, _ = fmt.Fprint(stdout, "\n  Cancelled. Nothing was spent.\n")
			return exitRefused
		}
		_, _ = fmt.Fprintf(stderr, "sense-lab pay: %v\n", err)
		return exitError
	}

	return p.runCells(ctx, f, stdout, stderr)
}

// paid is one repository's paid step, resolved: what will run, against what,
// and where it lands.
type paid struct {
	repo     string
	cycle    int
	dir      string
	scenario string
	set      scenario.Set
	group    string
	gold     []score.Row
	floor    float64
	arms     []payArm
	wall     time.Duration
	checkout string
}

// payArm is one model the bench declares, with the tool that drives it.
type payArm struct {
	Model string
	Agent string
	Role  string
	Runs  int
}

// resolvePay reads everything the paid step needs and refuses every way it
// cannot run, before a session is spawned.
func resolvePay(f payFlags) (*paid, error) {
	c, err := catalog.Load(f.config)
	if err != nil {
		return nil, err
	}
	r, ok := c.Repos[f.name]
	if !ok {
		return nil, fmt.Errorf("no repository %q in the catalog; admit it first with `sense-lab next %s`", f.name, f.name)
	}
	b, err := loadBench(f.config, r.ID)
	if err != nil {
		return nil, fmt.Errorf("%w. Nothing declares which arms %s is measured on, "+
			"and a cell run without one is spend against a matrix no file states", err, r.ID)
	}
	arms, err := payArms(c, b)
	if err != nil {
		return nil, err
	}

	at, err := position.Read(f.runs, r.ID)
	if err != nil {
		return nil, err
	}
	if at.Awaiting != phase.Bench {
		return nil, fmt.Errorf("%s is not at the paid step: it is %s, %s. Run `sense-lab next %s`",
			r.ID, at.Standing, at.Because, r.ID)
	}
	dir := filepath.Join(f.runs, r.ID, fmt.Sprint(at.Cycle), string(phase.Bench))
	if err := nothingBurned(dir); err != nil {
		return nil, err
	}
	if err := underCeiling(filepath.Join(f.runs, r.ID)); err != nil {
		return nil, err
	}

	path, err := scenarioFor(f.runs, r.ID, at.Cycle)
	if err != nil {
		return nil, err
	}
	set, err := scenario.LoadPath(path)
	if err != nil {
		return nil, err
	}
	// The rows are resolved here rather than at the point they are scored
	// against, so a scenario that cannot be scored is refused before a session
	// is spawned rather than after one. Scoring both arms against nothing
	// returns zero for each, and zero against zero reads as a measurement in
	// which Sense gave no advantage.
	gold, err := goldRows(set, set.Gold.Discriminator)
	if err != nil {
		return nil, err
	}
	checkout := r.Checkout
	if checkout == "" {
		checkout = filepath.Join(f.checkouts, r.ID)
	}

	return &paid{
		repo: r.ID, cycle: at.Cycle, dir: dir, scenario: path, set: set,
		group: set.Gold.Discriminator, gold: gold, floor: defaultFloor, arms: arms,
		wall: f.wall, checkout: checkout,
	}, nil
}

// payArms is the arms the bench declares, refusing anything the paid step
// cannot actually run.
//
// The subject list is checked here rather than discovered mid-matrix. A cell is
// two arms — one without Sense and one with it — and that is what the prober
// produces; a bench naming a third subject is naming a comparison this command
// has no way to run, and finding that out after the first model's runs would
// have spent for a matrix that was never going to complete.
func payArms(c *catalog.Catalog, b plan.Bench) ([]payArm, error) {
	res, err := plan.Expand(c, b)
	if err != nil {
		return nil, err
	}
	if len(res.Rejected) > 0 {
		return nil, fmt.Errorf("%d of this bench's arms cannot run; `sense-lab plan %s` names each one "+
			"and why. A partial matrix is not a measurement", len(res.Rejected), b.Repo)
	}
	if err := twoSubjects(c, b); err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var arms []payArm
	for _, j := range res.Jobs {
		if seen[j.Model] {
			continue
		}
		seen[j.Model] = true
		arms = append(arms, payArm{Model: j.Model, Agent: j.Agent, Role: string(j.Role), Runs: j.Runs})
	}
	if len(arms) == 0 {
		return nil, fmt.Errorf("%s declares no arms to run", b.Repo)
	}
	return arms, nil
}

// twoSubjects refuses a bench whose subjects are not the pair a cell is.
func twoSubjects(c *catalog.Catalog, b plan.Bench) error {
	kinds := map[string]int{}
	for _, id := range b.Subjects {
		// A subject the catalog does not hold is already refused by the
		// planner, in its own words, before this is reached. What is left here
		// is a bench naming subjects that are real and are not a pair.
		kinds[string(c.Subjects[id].Kind)]++
	}
	if len(b.Subjects) != 2 || kinds[string(catalog.Baseline)] != 1 || kinds[string(catalog.Sense)] != 1 {
		return fmt.Errorf("a cell is one arm without Sense against one arm with it, and this bench names "+
			"%v. Declare exactly one baseline subject and one sense subject", b.Subjects)
	}
	return nil
}

// scenarioFor is the scenario the paid step runs, where the graph says the
// phase that writes one put it.
//
// Derived rather than typed. The command that used to print this path named
// `lab/scenarios/<repo>/<repo>.yaml`, which does not exist, because the file is
// written into the run tree and the run tree is not committed.
func scenarioFor(runs, repo string, cycle int) (string, error) {
	wrote, ok := phase.Wrote("scenario.yaml")
	if !ok {
		return "", errors.New("no phase of the graph writes scenario.yaml")
	}
	at := filepath.Join(runs, repo, fmt.Sprint(cycle), string(wrote), "scenario.yaml")
	if _, err := os.Stat(at); err != nil {
		return "", fmt.Errorf("the scenario this would be run against is not there: %w", err)
	}
	return at, nil
}

// nothingBurned refuses a second run into a bench directory whose record names
// something that can never be paired.
//
// The record is written after every cell so that an interruption leaves the
// burned arm named on disk. Running again would start a fresh record in the
// same place and write over the only thing that names it — the exact failure
// the incremental write exists to prevent, arriving through the front door
// rather than through an interrupt.
//
// The fix is a person's: read what is there, move it aside or take it into
// account, and run again. The instrument will not decide which.
func nothingBurned(dir string) error {
	b, err := os.ReadFile(filepath.Join(dir, cellsFile))
	if err != nil {
		// No record is the ordinary case: nothing has been paid for here yet.
		return nil
	}
	var rec paidCells
	if json.Unmarshal(b, &rec) != nil {
		return fmt.Errorf("%s is there and cannot be read. It records what has already been paid for, "+
			"and running again would write over it", filepath.Join(dir, cellsFile))
	}
	var stuck []string
	for _, c := range rec.Cells {
		if !c.Complete || !c.Sound {
			stuck = append(stuck, c.Dir)
		}
	}
	if len(stuck) == 0 {
		return fmt.Errorf("%s already records %s paid for in this attempt. Running again would write over "+
			"that record; read it, and move it aside if this is meant to be a second matrix",
			filepath.Join(dir, cellsFile), count(len(rec.Cells), "cell"))
	}
	return fmt.Errorf("%s records %s that cannot be paired: %v. Running again would write over the only "+
		"thing that names them, and a later pass would pair one. Read it and move it aside first",
		filepath.Join(dir, cellsFile), count(len(stuck), "cell"), stuck)
}

// warning is what the operator reads before the only irreversible act this
// instrument performs. It states consequence rather than action: what it costs,
// how long it takes, what it uses up, and what an interruption does.
func (p *paid) warning() string {
	cells, sessions := 0, 0
	for _, a := range p.arms {
		cells += a.Runs
		sessions += a.Runs * 2
	}
	out := "  ⚠  This is the step that spends money.\n\n"
	out += fmt.Sprintf("     %d real agent sessions, %s.\n", sessions, roughly(cells, p.wallOf()))
	for _, a := range p.arms {
		out += fmt.Sprintf("     %-16s %s, run %s, with and without Sense\n", a.Model, a.Role, times(a.Runs))
	}
	out += fmt.Sprintf("     %-16s %s, %s\n", "measuring", p.set.Scenario.Name, count(len(p.gold), "hidden answer"))
	out += fmt.Sprintf("\n     This uses %d of the %d paid runs %s gets in its lifetime.\n",
		cells, defaultCeiling, p.repo)
	out += "\n     It cannot be undone, and a session stopped halfway wastes the half that finished.\n"
	out += "     If a cell comes back unusable it stops there and spends nothing further.\n\n"
	return out
}

// wallOf is how long one cell takes at worst: both arms, back to back.
func (p *paid) wallOf() time.Duration { return p.wall + probe.BaselineWall(p.wall) }

// runCells runs every cell, scores both arms of each, and records as it goes.
func (p *paid) runCells(ctx context.Context, f payFlags, stdout, stderr io.Writer) int {
	rec := paidCells{Repo: p.repo, Cycle: p.cycle, Scenario: p.scenario, Group: p.group, Floor: p.floor}
	for _, arm := range p.arms {
		for n := 1; n <= arm.Runs; n++ {
			_, _ = fmt.Fprintf(stdout, "\n  ─── %s, run %d of %d ───────────────── %s\n",
				arm.Model, n, arm.Runs, clock())

			cell, err := p.cell(ctx, f, arm, n)
			rec.Cells = append(rec.Cells, cell)
			if writeErr := writeCells(p.dir, rec); writeErr != nil {
				_, _ = fmt.Fprintf(stderr, "sense-lab pay: %v\n", writeErr)
				return exitError
			}
			if err != nil {
				_, _ = fmt.Fprintf(stderr, "sense-lab pay: %v\n", err)
				return exitError
			}
			_, _ = fmt.Fprint(stdout, cell.lines(p.floor))
			if !cell.Sound || !cell.Complete {
				_, _ = fmt.Fprintf(stdout, "\n  Stopped after %s. Nothing further was spent, and %s records\n  what may never be paired.\n",
					count(len(rec.Cells), "cell"), filepath.Join(p.dir, cellsFile))
				return exitRefused
			}
		}
	}
	_, _ = fmt.Fprint(stdout, p.summary(rec))
	return exitOK
}

// cell runs one pair and scores both its arms.
func (p *paid) cell(ctx context.Context, f payFlags, arm payArm, n int) (paidCell, error) {
	dir, err := freeCell(p.dir)
	if err != nil {
		return paidCell{}, err
	}
	c := paidCell{Model: arm.Model, Role: arm.Role, Run: n, Of: arm.Runs, Dir: dir}

	s, _, err := probeSpec(ctx, probeFlags{
		config: f.config, scenario: p.scenario, repo: p.repo, checkout: p.checkout,
		out: dir, runs: f.runs, agent: arm.Agent, model: arm.Model, senseBin: f.senseBin, wall: f.wall,
	})
	if err != nil {
		return c, err
	}
	report, err := runPair(ctx, s)
	if err != nil {
		return c, err
	}
	c.readRecord()
	if !report.Sound() {
		c.Note = notSound(report)
		return c, nil
	}
	c.Sound = true
	if c.Sense, err = p.recall(dir, "sense"); err != nil {
		return c, err
	}
	c.Baseline, err = p.recall(dir, "baseline")
	return c, err
}

// runPair is the two-arm cell, as a variable so a test can state what came back
// without spawning two agents to say it.
//
// One seam for both callers: the paid step runs a cell, and so does the crank
// for the phases that read one. A second call site reaching probe.Run directly
// would be a path no test could speak for.
var runPair = probe.Run

// recall scores one arm of a cell against the discriminator group.
func (p *paid) recall(dir, arm string) (float64, error) {
	at := filepath.Join(dir, arm, "session")
	tr, err := transcript.Read(runFormat(at), filepath.Join(at, "raw", "stdout"))
	if err != nil {
		return 0, fmt.Errorf("score the %s arm: %w", arm, err)
	}
	src := scoredRun{tr: tr, why: runWasNotCompleted(at)}
	result := score.GroupCites(p.group, p.gold, score.Scan(src.Answer()), src.ProvisionalWhy(), p.floor)
	return result.Recall, nil
}

// summary is what the whole matrix amounted to, per model, and what happens
// next. The mean is what may be reported: the recorded spread within one cell
// reaches a quarter of the group against a bar of half of it, so a single run
// is a draw rather than a reading.
func (p *paid) summary(rec paidCells) string {
	out := "\n  ─────────────────────────────────────────────────────────────────────\n\n"
	for _, arm := range p.arms {
		runs := rec.of(arm.Model)
		if len(runs) == 0 {
			continue
		}
		mean := say.Mean(runs)
		out += fmt.Sprintf("      %-16s %s\n", arm.Model, under(23, mean.Sentence()))
		out += fmt.Sprintf("      %-16s %s, %s\n\n", "", say.Runs(len(runs)), roleNote(arm.Role, mean))
	}
	out += "  Money is done. Nothing after this costs anything.\n\n"
	out += fmt.Sprintf("  Next: a judge reads how each arm reached its answer, and a second pass checks\n"+
		"  the result against five rules before it can go on the board.\n\n      sense-lab next %s\n", p.repo)
	return out
}

// roleNote says what this arm's number is for. A confirmation arm below the
// headline bar has not failed: what it is asked is whether the result moves the
// same way on another model.
func roleNote(role string, mean say.Pair) string {
	if plan.Role(role) != plan.Confirmation {
		return "the headline number"
	}
	if mean.Gap() > 0 {
		return "the confirmation model, which moved the same way"
	}
	return "the confirmation model, which did not move the same way"
}

// paidCell is one comparison as the record keeps it.
type paidCell struct {
	Model    string            `json:"model"`
	Role     string            `json:"role"`
	Run      int               `json:"run"`
	Of       int               `json:"of"`
	Dir      string            `json:"dir"`
	Arms     map[string]string `json:"arms,omitempty"`
	Complete bool              `json:"complete"`
	Sound    bool              `json:"sound"`
	Note     string            `json:"note,omitempty"`
	Sense    float64           `json:"sense_recall"`
	Baseline float64           `json:"baseline_recall"`
	// Burned names arms that finished and can never be paired; Unusable names
	// arms with no result at all. Both are read back from the cell's own record
	// rather than inferred here.
	Burned   []string `json:"burned,omitempty"`
	Unusable []string `json:"unusable,omitempty"`
}

// readRecord takes the arms, and what may never be paired, from the record the
// cell wrote about itself.
func (c *paidCell) readRecord() {
	b, err := os.ReadFile(filepath.Join(c.Dir, "cell-meta.json"))
	if err != nil {
		return
	}
	var rec struct {
		Arms     map[string]string `json:"arms"`
		Complete bool              `json:"complete"`
		Burned   []string          `json:"burned"`
		Unusable []string          `json:"unusable"`
	}
	if json.Unmarshal(b, &rec) != nil {
		return
	}
	c.Arms, c.Complete, c.Burned, c.Unusable = rec.Arms, rec.Complete, rec.Burned, rec.Unusable
}

// lines is the cell as the operator reads it: whether it was a measurement,
// and what it measured. The floor travels in because a gap stated without the
// bar it is measured against is a number the reader has to take on trust.
func (c paidCell) lines(floor float64) string {
	if !c.Complete {
		return fmt.Sprintf("  ✗ %s   Stopped part way. %s\n", clock(), burnedNote(c))
	}
	if !c.Sound {
		return fmt.Sprintf("  ✗ %s   Not a measurement, so it may not be scored.\n            %s\n",
			clock(), under(12, c.Note))
	}
	p := say.Pair{Sense: c.Sense, Baseline: c.Baseline, Floor: floor}
	return fmt.Sprintf("  ✓ %s   Checked: the only difference between the two was Sense.\n            %s\n",
		clock(), under(12, p.Sentence()))
}

// under keeps a sentence that runs to more than one line inside its own
// column. A second line starting at the left margin reads as a different
// thought rather than the rest of this one.
func under(at int, s string) string {
	return strings.ReplaceAll(s, "\n", "\n"+strings.Repeat(" ", at))
}

// burnedNote names what the interruption cost, because a count would let a
// later pass pair the run it names.
func burnedNote(c paidCell) string {
	switch {
	case len(c.Burned) > 0 && len(c.Unusable) > 0:
		return fmt.Sprintf("%v finished and can never be paired; %v has no result.", c.Burned, c.Unusable)
	case len(c.Burned) > 0:
		return fmt.Sprintf("%v finished and can never be paired.", c.Burned)
	default:
		return "Nothing finished, so nothing was burned."
	}
}

// paidCells is the artifact: every cell this repository has paid for in this
// cycle, and what came of each.
type paidCells struct {
	Repo     string     `json:"repo"`
	Cycle    int        `json:"cycle"`
	Scenario string     `json:"scenario"`
	Group    string     `json:"group"`
	Floor    float64    `json:"floor"`
	Cells    []paidCell `json:"cells"`
}

// of is the sound, complete runs of one model, as pairs to be read.
func (r paidCells) of(model string) []say.Pair {
	var out []say.Pair
	for _, c := range r.Cells {
		if c.Model == model && c.Sound && c.Complete {
			out = append(out, say.Pair{Sense: c.Sense, Baseline: c.Baseline, Floor: r.Floor})
		}
	}
	return out
}

// writeCells writes the artifact after every cell.
//
// After every cell rather than at the end. A matrix interrupted between two
// cells used to leave nothing on disk naming the arms that had run, and a later
// pass would pair one of them: `lab/plans/bench.md` states the rule as an
// interruption is RECORDED, not raised, and this is where it is kept.
func writeCells(dir string, rec paidCells) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, cellsFile), append(b, '\n'), 0o644)
}

// header is the two lines every screen of the flow opens with: which
// repository, which attempt, and where in the five stages this is.
func header(repo string, cycle int, at phase.Name) string {
	return fmt.Sprintf("\n  %s · attempt %d of %d\n\n      %s\n\n",
		repo, cycle, phase.AuthoringCeiling, stage.Line(at))
}

// times is a count of runs in words. A page asking to be trusted with money
// does not say "run 1 times".
func times(n int) string {
	switch n {
	case 1:
		return "once"
	case 2:
		return "twice"
	default:
		return fmt.Sprintf("%d times", n)
	}
}

// roughly is how long a set of cells takes at worst, rounded to something a
// person plans around rather than a duration to the second.
func roughly(cells int, each time.Duration) string {
	total := time.Duration(cells) * each
	if total < time.Minute {
		return "under a minute"
	}
	if total < time.Hour {
		return "about " + count(int(total.Minutes()), "minute")
	}
	return fmt.Sprintf("about %.1f hours", total.Hours())
}

// count is a number and its noun, agreeing. A page that says "1 hidden
// answers" reads as generated rather than written, and this one is asking to
// be trusted with money.
func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// clock is the time of day a line printed, for a reader watching a long step.
var clock = func() string { return time.Now().Format("15:04") }

// isTerminal reports whether there is somebody there to answer a question.
//
// It is the one effect the confirmation needs, and it lives at the edge so the
// asking itself stays pure. A variable because a test cannot hand a command a
// real terminal, and the paths that matter most here — the wrong name typed,
// the spend cancelled — only exist when there is one.
var isTerminal = func(r io.Reader) bool {
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// paySignals runs the paid step under the same signal handling a session gets:
// it must die with the binary, or an interrupt leaves agents running,
// unattended and still spending.
func paySignals(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return payCells(ctx, args, stdin, stdout, stderr)
}
