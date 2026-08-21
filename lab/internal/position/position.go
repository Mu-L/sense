package position

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/luuuc/sense/lab/internal/phase"
)

// Standing is what a position amounts to for a caller that has to decide, and
// each one is an exit code. They are an API rather than a display: this command
// ends up in a shell loop, and what that loop stops on is this.
type Standing string

const (
	// Ready: there is a next phase and something to run it on.
	Ready Standing = "ready"
	// Finished: the repository reached the board.
	Finished Standing = "finished"
	// Parked: the authoring ceiling, re-entered only by a deliberate human act.
	Parked Standing = "parked"
	// Waiting: a PAY. The method worked and the next act spends money, which
	// the crank does not do. This and Parked are the two most worth telling
	// apart: one is the method failing on this repository, the other is the
	// method succeeding and waiting on a wallet.
	Waiting Standing = "waiting"
	// Unusable: the last attempt emitted something its phase cannot emit. An
	// agent misbehaved.
	Unusable Standing = "unusable"
	// Missing: the last attempt's verdict is usable and the artifact it owed is
	// not there. An agent died. Diagnosed differently from Unusable, which is
	// why it is not the same code.
	Missing Standing = "missing"
)

// Position is where one repository stands.
type Position struct {
	Repo string
	// Cycle is the authoring cycle on disk, which is the highest numbered
	// directory. Derived every time; there is no counter anywhere to drift.
	Cycle int
	// Indexed says the repository has been scanned. The index sits beside the
	// cycles rather than inside one, because a re-entry does not rescan.
	Indexed bool
	// Awaiting is the phase to run next: routed from the last verdict where
	// there is one, and otherwise the first phase of this cycle that owes an
	// artifact.
	Awaiting phase.Name
	// Reached is the furthest phase of this cycle whose artifact exists.
	Reached  phase.Name
	Standing Standing
	// Last is the last attempt recorded in this cycle, if any.
	Last Attempt
	// Answer is every rejection this repository has carried, oldest first. The
	// next attempt reads all of them and not only the last: six attempts
	// converging is what that makes possible, and six unrelated drafts is what
	// it prevents.
	Answer []Attempt
	// Banked is every cycle that reached the board.
	Banked []int
	// Because says why the standing is what it is, in a sentence, for whoever
	// is reading a stopped loop.
	Because string
}

// ToCeiling is how many authoring cycles are left before this repository parks.
func (p Position) ToCeiling() int { return max(phase.AuthoringCeiling-p.Cycle, 0) }

// Read reports where a repository stands, from its run tree and its attempt
// records.
//
// A repository directory that is not there is not an error: asking where a
// repository stands before it has been admitted is a fair question with an
// answer, and the answer is that it awaits its index.
func Read(runs, repo string) (Position, error) {
	dir := filepath.Join(runs, repo)
	p := Position{Repo: repo, Indexed: wrote(dir, phaseOf(phase.Index))}

	cycles, err := cycleDirs(dir)
	if err != nil {
		return Position{}, err
	}
	for _, c := range cycles {
		at := filepath.Join(dir, strconv.Itoa(c))
		if wrote(at, phaseOf(phase.Board)) {
			p.Banked = append(p.Banked, c)
		}
		if c > p.Cycle {
			p.Cycle = c
		}
	}
	if p.Cycle == 0 {
		// A repository that has opened no cycle directory is in its first one.
		// Reporting cycle 0 would be a number no rule is written in: the
		// ceiling counts from 1, and an attempt recorded for cycle 1 before its
		// directory exists is an attempt in the cycle this repository is in.
		p.Cycle = 1
	}
	attempts, err := Attempts(dir)
	if err != nil {
		return Position{}, err
	}
	p.Answer = rejections(attempts)
	p.settle(dir, attempts)
	return p, nil
}

// settle works out the next phase and what the position amounts to.
//
// The order of the questions is the answer to most of them: parked beats
// everything because a parked repository is not re-entered by any transition,
// and banked beats a stale attempt record because the board is on disk.
func (p *Position) settle(dir string, attempts []Attempt) {
	cycleDir := filepath.Join(dir, strconv.Itoa(p.Cycle))
	p.Reached, p.Awaiting = walk(cycleDir)
	// The last verdict is carried whatever the standing turns out to be.
	// Whoever finds a parked repository wants to read the verdict that parked
	// it, and a field only filled in on the paths that route from one would be
	// blank on exactly the positions somebody is looking at because it stopped.
	last, decided := lastOf(attempts, p.Cycle)
	p.Last = last

	switch {
	case wrote(cycleDir, phaseOf(phase.Handoff)):
		p.Standing, p.Awaiting = Parked, phase.Done
		p.Because = fmt.Sprintf("cycle %d wrote a handoff; re-entering a parked repository is a human act, not a transition", p.Cycle)
		return
	case slices.Contains(p.Banked, p.Cycle):
		p.Standing, p.Awaiting = Finished, phase.Done
		p.Because = fmt.Sprintf("cycle %d reached the board", p.Cycle)
		return
	}

	if !p.Indexed {
		// Nothing under a cycle can be believed before the repository is
		// scanned, so nothing under one is read as a position. This is checked
		// after the two endings and before any verdict: a scan is what the
		// repository is waiting for, whatever else is on disk.
		p.Standing, p.Awaiting = Ready, phase.Index
		p.Because = "the repository has not been scanned; the index is what everything after it reads"
		return
	}

	if !decided {
		// Nothing decided in this cycle. The tree walk is the whole answer, and
		// there is a phase owed.
		p.Standing = Ready
		p.Because = fmt.Sprintf("no verdict recorded in cycle %d; %s owes %s", p.Cycle, p.Awaiting, phaseOf(p.Awaiting).Writes)
		return
	}
	p.route(cycleDir, last)
}

// route reads the last verdict and says where it leads, or why it leads
// nowhere.
func (p *Position) route(cycleDir string, last Attempt) {
	switch last.Outcome {
	case Stalled:
		// An agent that died. It is Missing rather than Unusable because that
		// is how it is diagnosed: there is a log, and the phase never reached a
		// judgment.
		p.Standing = Missing
		p.Because = fmt.Sprintf("%s ran past its wall and recorded no verdict; its log is %s",
			last.Phase, orText(last.Log, "not recorded"))
		return
	case Refused:
		// An agent that misbehaved: it finished, and what it wrote is not a
		// verdict for this phase.
		p.Standing = Unusable
		p.Because = fmt.Sprintf("%s was refused: %s", last.Phase, last.Table)
		return
	}

	next, err := phase.Next(last.Phase, last.Verdict, p.Cycle)
	if err != nil {
		p.Standing, p.Because = Unusable, err.Error()
		return
	}
	if !wrote(cycleDir, phaseOf(last.Phase)) {
		// The verdict is one the phase may emit and the artifact it owed is not
		// there. An exit code is a claim; the artifact is the fact.
		p.Standing = Missing
		p.Because = fmt.Sprintf("%s emitted %s and did not write %s; the verdict is usable and the phase did not finish",
			last.Phase, last.Verdict, phaseOf(last.Phase).Writes)
		return
	}
	if phase.ReEntry(last.Phase, last.Verdict) && next == phase.Author {
		// A re-entry is the next authoring cycle, and the cycle is what the
		// ceiling is counted in. Without this the count never moves: every
		// re-entry re-opened the cycle it came from, so the sixth one routed
		// back to the author like the first, the ceiling could not bite, and
		// each attempt wrote its draft over the one it was sent back to
		// replace. Measured on mastodon 2026-08-20: four authoring passes, four
		// mini-bench pairs, all of them recorded in cycle 1.
		//
		// Still derived rather than counted. The number comes from the tree on
		// every read, and the new cycle's directory appears when its first
		// phase writes into it, which is the same moment the tree starts saying
		// so on its own.
		p.Cycle++
		p.Reached, p.Awaiting = walk(filepath.Join(filepath.Dir(cycleDir), strconv.Itoa(p.Cycle)))
		p.Standing = Ready
		p.Because = fmt.Sprintf("%s emitted %s, which routes to %s in cycle %d",
			last.Phase, last.Verdict, next, p.Cycle)
		return
	}
	// Nothing routes to Done here: the two phases that end the loop are the
	// board and the handoff, and both are recognised from their artifact above
	// before any verdict is read. A repository is finished because the board is
	// on disk, not because something said so.
	p.Awaiting = next
	switch {
	case last.Verdict == phase.Pay:
		p.Standing = Waiting
		p.Because = "the validation says PAY, and spending is a human act"
	case next == phase.Handoff:
		p.Standing = Parked
		p.Because = fmt.Sprintf("%s emitted %s at cycle %d, which is the authoring ceiling", last.Phase, last.Verdict, p.Cycle)
	default:
		p.Standing = Ready
		p.Because = fmt.Sprintf("%s emitted %s, which routes to %s", last.Phase, last.Verdict, next)
	}
}

// rejections is every attempt that left a table behind, oldest first.
//
// The table is the test rather than the verdict, and that is deliberate. A
// re-entry is recognisable from its routing, but a NO-ANCHOR at the authoring
// ceiling routes to the handoff instead of back to the author and is no less a
// rejection for it — and a diagnosis carries one too. What makes an attempt
// something the next one has to answer is that somebody wrote down why it was
// refused.
func rejections(attempts []Attempt) []Attempt {
	var out []Attempt
	for _, a := range attempts {
		if a.Table != "" {
			out = append(out, a)
		}
	}
	return out
}

// lastOf is the last attempt recorded in a cycle.
//
// Scoped to the cycle the DIRECTORIES say we are in, so a record naming a cycle
// that was never opened decides nothing. The tree is what says where a
// repository is; a record says what was judged there.
func lastOf(attempts []Attempt, cycle int) (Attempt, bool) {
	for i := len(attempts) - 1; i >= 0; i-- {
		if attempts[i].Cycle == cycle {
			return attempts[i], true
		}
	}
	return Attempt{}, false
}

// walk reports the furthest phase of a cycle whose artifact exists, and the
// first whose artifact does not. It decides nothing: an artifact is on disk or
// it is not.
func walk(cycleDir string) (reached, awaiting phase.Name) {
	for _, p := range phase.Graph {
		// The index is per repository rather than per cycle, and the handoff is
		// arrived at rather than walked to. Looking for either here would
		// report every cycle as owing one.
		if p.Name == phase.Index || p.Name == phase.Handoff {
			continue
		}
		if wrote(cycleDir, p) {
			reached = p.Name
			continue
		}
		if awaiting == "" {
			awaiting = p.Name
		}
	}
	if awaiting == "" {
		awaiting = phase.Done
	}
	return reached, awaiting
}

// wrote reports whether a phase's output artifact is under dir.
func wrote(dir string, p phase.Phase) bool {
	_, err := os.Stat(filepath.Join(dir, string(p.Name), p.Writes))
	return err == nil
}

func phaseOf(name phase.Name) phase.Phase {
	p, _ := phase.Lookup(name)
	return p
}

// cycleDirs is every numbered directory under a repository, which is every
// authoring cycle it has opened. Anything else in the tree is not a cycle and
// this walk does not rule on it.
func cycleDirs(dir string) ([]int, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read the run tree of %s: %w", dir, err)
	}
	var out []int
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if n, err := strconv.Atoi(e.Name()); err == nil {
			out = append(out, n)
		}
	}
	slices.Sort(out)
	return out, nil
}

// Render is the page. Nothing parses it: [Read] returns a [Position] and this
// turns one into a string, because parsing your own report is how a display
// format silently becomes a data contract.
func Render(p Position) string {
	var b strings.Builder
	fmt.Fprintf(&b, "repo:     %s\ncycle:    %d of %d\nindexed:  %s\nreached:  %s\nawaiting: %s\nstanding: %s — %s\n",
		p.Repo, p.Cycle, phase.AuthoringCeiling, yes(p.Indexed), or(p.Reached, "nothing yet"),
		or(p.Awaiting, "nothing"), p.Standing, p.Because)
	if len(p.Banked) > 0 {
		fmt.Fprintf(&b, "banked:   %v\n", p.Banked)
	}
	if len(p.Answer) == 0 {
		return b.String()
	}
	// Every rejection, not only the last. A page that showed the most recent
	// one would be a page that invites the next attempt to answer one thing.
	b.WriteString("answer:\n")
	for _, a := range p.Answer {
		table := a.Table
		if table == "" {
			table = "no table recorded"
		}
		fmt.Fprintf(&b, "  cycle %d %s emitted %s: %s\n", a.Cycle, a.Phase, a.Verdict, table)
	}
	return b.String()
}

func yes(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// orText is or, for a path. A blank where a file should be named reads as a
// file called nothing.
func orText(s, absent string) string {
	if s == "" {
		return absent
	}
	return s
}

// or names a phase, or says what its absence means. A phase that is empty is
// not "" on the page: an unset field and a real answer must not print alike.
func or(n phase.Name, absent string) string {
	if n == "" {
		return absent
	}
	return string(n)
}
