package cli

import (
	"context"
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
	"github.com/luuuc/sense/lab/internal/crank"
	"github.com/luuuc/sense/lab/internal/phase"
	"github.com/luuuc/sense/lab/internal/position"
	"github.com/luuuc/sense/lab/internal/repo"
	"github.com/luuuc/sense/lab/internal/stage"
)

// nextFlags is the whole flow in one verb. It takes a repository and, for the
// paths a test or a second checkout needs, the roots — all of which have
// defaults, so the ordinary invocation is `sense-lab next <repo>` and nothing
// else.
type nextFlags struct {
	config    string
	runs      string
	checkouts string
	senseBin  string
	agent     string
	model     string
	yes       bool
	name      string
}

func parseNextFlags(args []string, stderr io.Writer) (nextFlags, error) {
	var f nextFlags
	fs := flag.NewFlagSet("next", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configFlag(fs, &f.config)
	fs.StringVar(&f.runs, "runs", defaultRuns, "the root the repositories' run trees live under")
	fs.StringVar(&f.checkouts, "checkouts", defaultCheckouts, "the lab's own clones root")
	fs.StringVar(&f.senseBin, "sense", "sense", "the Sense binary the repository is indexed with")
	fs.StringVar(&f.agent, "agent", "", "override the agent tool the bench declares for phase work")
	fs.StringVar(&f.model, "model", "", "override the model the bench declares for phase work")
	fs.BoolVar(&f.yes, "yes", false, "run without being asked, for an unattended run")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	if fs.NArg() != 1 {
		return f, errors.New("name exactly one repository: an id already admitted, a path to a clone, " +
			"`owner/name`, or a url")
	}
	f.name = fs.Arg(0)
	return f, nil
}

// doNext does the next thing for one repository and says what comes after.
//
// One verb, because the operator's question is always the same one. An unknown
// repository is cloned, pinned and indexed; a known one is advanced until the
// loop stops on its own. What differs between those two is the first step, not
// the intent, which is why they are not two commands: `repo` used to switch
// between three behaviours on whether a flag happened to be set, and which one
// you were about to get was invisible until it had happened.
func doNext(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	f, err := parseNextFlags(args, stderr)
	if err != nil {
		if err != flag.ErrHelp {
			_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
		}
		return exitUsage
	}

	c, err := catalog.Load(f.config)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
		return exitError
	}
	r, err := repo.Resolve(f.name, catalog.IDs(c.Repos), repo.OnDisk)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
		return exitError
	}

	asker := ask.Asker{In: stdin, Out: stdout, Assumed: f.yes, Terminal: isTerminal(stdin)}
	if _, known := c.Repos[r.ID]; !known {
		return admitAndStop(ctx, f, c, r, asker, stdout, stderr)
	}
	return advance(ctx, f, c, r.ID, asker, stdout, stderr)
}

// admitAndStop clones, pins and indexes a repository nothing has admitted, and
// stops there.
//
// It stops rather than running on into the first stage because what to measure
// is a decision, and it is the operator's. Everything after this point spends
// an agent's time against a question, and a question written before anybody
// said what this repository is being measured on is work nobody asked for.
func admitAndStop(ctx context.Context, f nextFlags, c *catalog.Catalog, r repo.Resolution,
	asker ask.Asker, stdout, stderr io.Writer) int {
	_, _ = fmt.Fprintf(stdout, "\n  Sense benchmark · %s\n\n  This walks one repository from a blank page to a "+
		"measured result. Five stages:\n\n%s\n  You are before stage 1. Nothing has been spent.\n\n",
		r.ID, indented(stage.Map("")))
	_, _ = fmt.Fprintf(stdout, "  First I clone %s and index it with Sense.\n"+
		"  A few minutes. Nothing is spent and nothing is decided.\n\n", r.ID)

	if err := asker.Continue("clone and index " + r.ID); err != nil {
		return cancelled(err, stdout, stderr)
	}

	// What is about to be done to the checkout, in words, printed after it is
	// done rather than before: the sentence is about the tree that was found,
	// and the revision is read back out of the tree rather than asked for.
	var did string
	p, err := repo.Prepare(ctx, targetFor(c, r), f.checkouts, func(plan repo.Plan) error {
		did = sentence(plan)
		return nil
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprintf(stdout, "  the checkout                %s\n  %-27s pinned at %.8s, %s\n",
		p.Checkout, "", p.Revision, did)

	got, code := admitNew(ctx, repoFlags{config: f.config, runs: f.runs, checkouts: f.checkouts,
		senseBin: f.senseBin}, p, prefixed(stderr, "sense-lab next: "))
	switch code {
	case exitNotIndexed:
		_, _ = fmt.Fprintf(stdout, "\n  Sense found nothing to index in %s.\n  %s\n\n  What it did find is written to %s.\n",
			r.ID, got.Index.Shortfall, got.Artifact)
		return exitError
	case exitOK:
		i := got.Index
		_, _ = fmt.Fprintf(stdout, "  Indexing…                   %d files · %d symbols · %d edges · %v\n",
			i.Files, i.Symbols, i.Edges, i.Names())
	default:
		return code
	}
	_, _ = fmt.Fprintf(stdout, "\n  Done. %s is admitted.\n\n", r.ID)

	// Read back, not reused. The catalog in hand was loaded before this
	// repository existed in it, and the matrix about to be proposed is resolved
	// against a catalog that has to hold the repository it names.
	c, err = catalog.Load(f.config)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
		return exitError
	}
	b, at, written, err := starter(f.config, r.ID, c)
	if err != nil {
		_, _ = fmt.Fprintf(stdout, "%s", benchDecision(at, r.ID, err))
		return exitBlocked
	}
	_, _ = fmt.Fprintf(stdout, "%s\n      sense-lab next %s\n", starterTable(b, at, written), r.ID)
	return exitOK
}

// benchDecision is what to say when no starter could be proposed. The decision
// is the operator's either way; this is the case where the instrument cannot
// even offer a draft, and the reason it cannot is the useful part.
func benchDecision(at, id string, why error) string {
	return fmt.Sprintf("  ──────────────────────────────────────────────────────────────────\n\n"+
		"  One decision before we start, and it is yours: what do we measure?\n\n"+
		"  I could not propose a starting point:\n  %s\n\n"+
		"  Write %s yourself — the models to compare, how many runs\n"+
		"  each, the judge, and which model runs the stages. Then:\n\n      sense-lab next %s\n",
		under(2, why.Error()), at, id)
}

// advance turns the crank until it stops on its own, narrating as it goes.
//
// The narration is printed HERE rather than inside the crank, and each stage's
// line is printed BEFORE the phase runs. A forty-minute phase that printed one
// line on completion reads as a hang for forty minutes, and an operator who
// cannot tell a working instrument from a wedged one stops leaving it running.
func advance(ctx context.Context, f nextFlags, c *catalog.Catalog, id string,
	asker ask.Asker, stdout, stderr io.Writer) int {
	// The position first, and the driver only when something is about to be
	// run. Where a repository stands is a fact about its run tree; asking for
	// it should not require the file that says what it is measured on, and a
	// repository stopped for any reason has an answer worth printing whether
	// or not a phase could be dispatched.
	at, err := position.Read(f.runs, id)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
		return exitError
	}

	_, _ = fmt.Fprint(stdout, header(id, at.Cycle, at.Awaiting))
	if code, stop := stopOn[at.Standing]; stop {
		_, _ = fmt.Fprint(stdout, standingBlock(at))
		return code
	}
	// The checkout is held at its pin on every invocation, not only the first.
	// Phases run against this tree, and a clone that drifted would produce
	// results recorded against a commit they did not come from — so the lab's
	// own clone is put back and a handed-in one that moved is refused, before
	// anything is dispatched.
	if err := holdAtPin(ctx, f, c, id); err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
		return exitError
	}
	rf, err := driver(f, c, id)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprint(stdout, intent(at))
	if err := asker.Continue(remaining(at)); err != nil {
		return cancelled(err, stdout, stderr)
	}

	k, err := crankFor(rf, c, id)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
		return exitError
	}
	return crankOn(ctx, k, f, id, stdout, stderr)
}

// holdAtPin re-prepares the checkout of a repository already admitted.
//
// Idempotent by construction: a clone at its pin is adopted and nothing is
// written. What it exists for is the two cases that are not that — the lab's
// own clone that drifted, which is moved back, and a handed-in one that moved,
// which is refused rather than moved because it is not the lab's to move.
func holdAtPin(ctx context.Context, f nextFlags, c *catalog.Catalog, id string) error {
	r, ok := c.Repos[id]
	if !ok {
		return fmt.Errorf("no repository %q in the catalog", id)
	}
	_, err := repo.Prepare(ctx, targetFor(c, repo.Resolution{ID: r.ID, URL: r.URL, Path: r.Checkout}),
		f.checkouts, func(repo.Plan) error { return nil })
	return err
}

// crankOn advances one phase at a time until the loop stops, printing what each
// one is and what came of it.
func crankOn(ctx context.Context, k crank.Crank, f nextFlags, id string, stdout, stderr io.Writer) int {
	// Which stage has already announced itself. A stage covers more than one
	// phase — the rehearsal is three — and announcing it once per phase reads
	// as the same stage starting over three times.
	shown := 0
	for {
		before, err := position.Read(f.runs, id)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
			return exitError
		}
		if code, stop := stopOn[before.Standing]; stop {
			_, _ = fmt.Fprint(stdout, standingBlock(before))
			return code
		}
		if s, ok := stage.Of(before.Awaiting); ok && s.Number != shown {
			shown = s.Number
			_, _ = fmt.Fprint(stdout, starting(s, wallOfStage(k, s)))
		}

		r, err := k.Advance(ctx, id)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
			return exitError
		}
		_, _ = fmt.Fprint(stdout, finished(r.After, before.Awaiting))
		// No guard on the position having moved. Every path through Advance
		// either records something the next read reports as a standing this
		// loop stops on — stalled, refused, or an artifact that is not there —
		// or routes forward to another phase. A loop that could spin would need
		// a path that dispatches and records nothing, and record() has none.
	}
}

// wallOfStage is how long a whole stage may take: every phase under it, at the
// wall each one's own plan declares. Summed rather than taken from the phase
// about to run, because a reader who is told twenty-five minutes and then sees
// two more phases has been told the wrong thing.
func wallOfStage(k crank.Crank, s stage.Stage) time.Duration {
	var total time.Duration
	for _, p := range k.Plans {
		for _, under := range s.Phases {
			if p.Phase == under {
				total += p.Wall
			}
		}
	}
	return total
}

// starting is the line printed as a stage begins: which stage, the time, and
// how long it may take.
func starting(s stage.Stage, wall time.Duration) string {
	line := fmt.Sprintf("\n  ─── %d. %s ", s.Number, s.Name)
	line += strings.Repeat("─", max(4, 40-len(line)))
	if wall > 0 {
		return fmt.Sprintf("%s started %s, up to %s\n", line, clock(), roughWall(wall))
	}
	return fmt.Sprintf("%s started %s\n", line, clock())
}

// finished is the line printed when a phase comes back: whether it went the way
// the loop wants, and what happened, in words.
func finished(at position.Position, ran phase.Name) string {
	mark, line, more := "✓", "", ""
	o, ok := stage.Says(ran, at.Last.Verdict)
	switch {
	case at.Last.Verdict == "":
		// The phase produced no verdict. What that means is on the position,
		// which read it off the tree rather than off an exit code.
		mark, line = "✗", at.Because
	case ok:
		line, more = o.Line, o.More
		if !o.Good {
			mark = "✗"
		}
	default:
		line = fmt.Sprintf("%s emitted %s", ran, at.Last.Verdict)
	}
	out := fmt.Sprintf("  %s %s   %s\n", mark, clock(), line)
	if more != "" {
		out += fmt.Sprintf("            %s\n", under(12, more))
	}
	return out
}

// intent is what this invocation is about to do, before it does any of it.
func intent(at position.Position) string {
	left := remaining(at)
	return fmt.Sprintf("  I will %s and stop before anything is spent.\n"+
		"  You can stop at any point and run this again — everything finished is kept.\n\n", left)
}

// remaining says what is left to do before the next stop, in stages rather than
// in phases.
func remaining(at position.Position) string {
	here, ok := stage.Of(at.Awaiting)
	if !ok {
		return "carry on"
	}
	if here.Number >= paidStage {
		return fmt.Sprintf("work through stage %d", here.Number)
	}
	return fmt.Sprintf("work through stages %d to %d", here.Number, paidStage-1)
}

// paidStage is the stage the money is spent in, which is where an unattended
// run stops.
const paidStage = 4

// standingBlock is the stop: what happened, what is next, why, and the one
// command that moves it.
//
// Four parts, because a stop that named only the next command would leave the
// reader to work out whether what just happened was good.
func standingBlock(at position.Position) string {
	out := "\n  ──────────────────────────────────────────────────────────────────\n\n"
	out += fmt.Sprintf("  %s\n\n", strings.TrimSpace(under(2, whatHappened(at))))
	nextUp, why, command := nextStep(at)
	out += fmt.Sprintf("  Next: %s\n  Why:  %s\n", under(8, nextUp), under(8, why))
	if command != "" {
		out += fmt.Sprintf("\n      %s\n", command)
	}
	return out
}

// whatHappened is the standing in the operator's words, falling back to the
// position's own sentence for the standings that are already plain.
func whatHappened(at position.Position) string {
	switch at.Standing {
	case position.Finished:
		return fmt.Sprintf("Finished. %s reached the board on attempt %d.", at.Repo, at.Cycle)
	case position.Waiting:
		return "The rehearsal says the question is worth paying for."
	case position.Parked:
		return fmt.Sprintf("Given up after %d attempts. Every question written here was beaten by "+
			"an ordinary text search, and that is recorded rather than dropped.", at.Cycle)
	case position.Blocked:
		return "Stopped before anything ran, because something outside the loop has to change first:\n" +
			at.Because
	default:
		return at.Because
	}
}

// nextStep is what to do, why, and the command that does it.
func nextStep(at position.Position) (nextUp, why, run string) {
	switch at.Standing {
	case position.Finished:
		return "nothing", "the result is written up and on the board", ""
	case position.Waiting:
		return "run the paid cells", "spending is yours, not the instrument's", command(at)
	case position.Parked:
		return "nothing here", "a repository that has spent its attempts is re-entered by a deliberate act", ""
	case position.Blocked:
		return "fix what it named above, then run this again",
			"nothing here is wrong with the question; the loop is missing something it does not own",
			command(at)
	default:
		return "read what the phase left behind", at.Because, command(at)
	}
}

// command is the verb that performs a position's next act, and it is the only
// place in this binary where a standing becomes a command line.
//
// The act is [position.Position.Next]'s, because there is one right answer per
// standing and it belongs beside the standings. Which verb performs it is here,
// because a package that decides positions has no business knowing what this
// binary's commands are called.
func command(at position.Position) string {
	switch at.Next() {
	case position.Spend:
		return "sense-lab pay " + at.Repo
	case position.Advance:
		return "sense-lab next " + at.Repo
	case position.Diagnose:
		return "sense-lab why " + at.Repo
	case position.Nothing:
		return ""
	default:
		return ""
	}
}

// cancelled reports a confirmation that was declined or could not be put.
func cancelled(err error, stdout, stderr io.Writer) int {
	if errors.Is(err, ask.ErrDeclined) {
		_, _ = fmt.Fprint(stdout, "\n  Cancelled. Nothing was run.\n")
		return exitRefused
	}
	_, _ = fmt.Fprintf(stderr, "sense-lab next: %v\n", err)
	return exitError
}

// driver is the agent tool and model the phase agents are run by.
//
// It comes from the repository's bench, which is where what a repository is
// measured on is declared, so the ordinary invocation names none of it. The
// flags stay as an override for a run against something the bench does not
// declare, which is what a test needs and what trying a second tool needs.
func driver(f nextFlags, c *catalog.Catalog, id string) (repoFlags, error) {
	rf := repoFlags{config: f.config, runs: f.runs, checkouts: f.checkouts, senseBin: f.senseBin,
		agent: f.agent, model: f.model, until: true, name: id}
	if rf.agent != "" && rf.model != "" {
		return rf, nil
	}
	b, err := loadBench(f.config, id)
	if err != nil {
		return rf, fmt.Errorf("%w. Nothing says what %s is measured on, so no phase can be run "+
			"against it", err, id)
	}
	if b.Driver.Model == "" {
		return rf, fmt.Errorf("%s declares no driver, so nothing says which model the phases "+
			"themselves are run by. Add a \"driver\" to %s", id,
			filepath.Join(f.config, "benches", id+".json"))
	}
	if rf.model == "" {
		rf.model = b.Driver.Model
	}
	if rf.agent == "" {
		rf.agent = b.Driver.Agent
	}
	if rf.agent == "" {
		// A bench that names no tool is the normal case: a model usually names
		// exactly one, and repeating it in the bench would be a second place
		// for it to disagree with the catalog. It is only a decision when the
		// model names several.
		tool, err := onlyAgentFor(c, rf.model)
		if err != nil {
			return rf, err
		}
		rf.agent = tool
	}
	return rf, nil
}

// onlyAgentFor is the tool a model names, when it names exactly one.
func onlyAgentFor(c *catalog.Catalog, model string) (string, error) {
	m, ok := c.Models[model]
	if !ok {
		return "", fmt.Errorf("the bench names the model %q, which the catalog does not hold", model)
	}
	if len(m.Agents) != 1 {
		return "", fmt.Errorf("%s can be driven by %v, so which one runs the stages is a decision: "+
			"name it as the driver's agent in this repository's bench", model, m.Agents)
	}
	return m.Agents[0], nil
}

// indented pushes a block in by four columns, for a map printed inside a
// sentence rather than as a page of its own.
func indented(s string) string {
	return "  " + strings.TrimRight(under(2, s), " ")
}

// roughWall is a phase's wall as a person reads it.
func roughWall(d time.Duration) string {
	if d < time.Hour {
		return count(int(d.Minutes()), "minute")
	}
	return fmt.Sprintf("%.1f hours", d.Hours())
}

// nextSignals runs the flow under the same signal handling a session gets: a
// clone is minutes of network, a scan is minutes of CPU and a phase is an agent
// spending, and an interrupt that did not reach them would leave all three
// running behind a returned command.
func nextSignals(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return doNext(ctx, args, stdin, stdout, stderr)
}
