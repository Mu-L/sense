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
	"syscall"
	"time"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/position"
	"github.com/luuuc/sense/lab/internal/repo"
)

// defaultCheckouts is the lab's own clones root. Ownership is what makes a
// correction safe, and it is decided by where a clone lives: the lab fixes what
// it made, and reads what it was given.
const defaultCheckouts = "runs/checkouts"

// defaultRuns is where a repository's run tree lives, one directory per
// repository. It is a default rather than a flag anybody types, because which
// tree a repository's evidence belongs to is not a decision: it belongs to the
// repository.
const defaultRuns = "runs/repos"

// The exit codes `sense-lab repo` answers with. They are its API rather than a
// display, because this command ends up in a shell loop and what that loop stops
// on is this:
//
//	while sense-lab repo <id>; do :; done
//
// It terminates on a park, on a PAY and on either refusal, and `-show` never
// returns 0 so it cannot be dropped into that loop and spin.
//
// Two of them are worth telling apart above all: a park is the method failing on
// this repository, and a PAY is the method succeeding and waiting on a wallet.
// So are the two refusals: an agent that misbehaved, against an agent that died.
//
// exitShown is the binary's usage code, and that is not a collision in practice:
// the codes are read with the arguments in hand, and a caller that passed -show
// knows a position was printed while one that did not knows its flags were
// wrong.
const (
	exitShown    = exitUsage
	exitFinished = 3
	exitParked   = 4
	exitWaiting  = 5
	exitUnusable = 6
	exitMissing  = 7
)

// repoFlags is where a repository is admitted from and where the evidence of it
// lands.
type repoFlags struct {
	config    string
	runs      string
	checkouts string
	senseBin  string
	agent     string
	model     string
	show      bool
	until     bool
	name      string
}

func parseRepoFlags(args []string, stderr io.Writer) (repoFlags, error) {
	var f repoFlags
	fs := flag.NewFlagSet("repo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configFlag(fs, &f.config)
	fs.StringVar(&f.runs, "runs", defaultRuns, "the root the repositories' run trees live under")
	fs.StringVar(&f.checkouts, "checkouts", defaultCheckouts, "the lab's own clones root")
	fs.StringVar(&f.senseBin, "sense", "sense", "the Sense binary that indexes the repository")
	fs.StringVar(&f.agent, "agent", "", "catalog agent id a phase is run by")
	fs.StringVar(&f.model, "model", "", "catalog model id a phase is run by")
	fs.BoolVar(&f.show, "show", false, "print the position and stop, writing nothing")
	fs.BoolVar(&f.until, "until", false, "crank until it stops on its own, one phase at a time")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	// The name is positional and comes after the flags, which is how every Go
	// flag set reads its arguments: `sense-lab repo -sense ./bin/sense owner/name`.
	if fs.NArg() != 1 {
		return f, fmt.Errorf("name exactly one repository: an id already admitted, a path to a clone, `owner/name`, or a url")
	}
	f.name = fs.Arg(0)
	return f, nil
}

// admitRepo is the one command that admits a repository: resolve it, clone it,
// pin it at the tree it got, scan it, and record what the index holds.
//
// A repository the catalog already holds is not admitted again. It prints where
// that repository stands and stops, because the crank calls this on every
// invocation and admission has to be idempotent for that to be safe.
func admitRepo(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	f, err := parseRepoFlags(args, stderr)
	if err != nil {
		if err != flag.ErrHelp {
			_, _ = fmt.Fprintf(stderr, "sense-lab repo: %v\n", err)
		}
		return exitUsage
	}

	c, err := catalog.Load(f.config)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab repo: %v\n", err)
		return exitError
	}
	r, err := repo.Resolve(f.name, catalog.IDs(c.Repos), repo.OnDisk)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab repo: %v\n", err)
		return exitError
	}

	p, err := repo.Prepare(ctx, targetFor(c, r), f.checkouts, announce(stdout, f.show))
	switch {
	case errors.Is(err, errShown):
		// Stopped at the announcement, before anything was written. What a
		// caller asked to see is the position, so it is printed from what is on
		// disk rather than from anything this run did.
		return standing(f, r.ID, stdout, exitShown)
	case err != nil:
		_, _ = fmt.Fprintf(stderr, "sense-lab repo: %v\n", err)
		return exitError
	}
	if p.Admitted {
		if f.agent == "" && f.model == "" {
			// No driver named, so nothing can be dispatched. The position is
			// the whole answer, which is what this command did before it could
			// turn the loop at all.
			return standing(f, p.ID, stdout, exitOK)
		}
		return turn(ctx, f, c, p.ID, stdout, stderr)
	}
	if code := admitNew(ctx, f, p, stdout, stderr); code != exitOK {
		return code
	}
	return standing(f, p.ID, stdout, exitOK)
}

// targetFor is what admission acts on. A repository the catalog holds brings
// its own url, its checkout and its pin: re-admitting it must reach the same
// tree it reached last time, and the recorded pin is the only thing that says
// which that is.
func targetFor(c *catalog.Catalog, r repo.Resolution) repo.Target {
	if known, ok := c.Repos[r.ID]; ok {
		return repo.Target{ID: known.ID, URL: known.URL, Checkout: known.Checkout, Pin: known.Commit}
	}
	return repo.Target{ID: r.ID, URL: r.URL, Checkout: r.Path}
}

// announce prints what was decided, before anything is written.
//
// The whole ordering exists for this line. Nothing has been cloned, moved or
// recorded when it prints, so a wrong repository is visible in the scrollback
// rather than three phases later — and it is a statement rather than a prompt,
// because an interactive question in the run path is what the crank cannot
// answer.
func announce(stdout io.Writer, show bool) func(repo.Plan) error {
	return func(p repo.Plan) error {
		_, _ = fmt.Fprintf(stdout, "id:       %s\nurl:      %s\nrevision: %s\ncheckout: %s (%s)\n\n",
			p.ID, p.URL, p.Revision, p.Checkout, sentence(p))
		if show {
			return errShown
		}
		return nil
	}
}

// errShown stops an admission at the announcement. -show is the same code path
// as an admission up to the point where one would write something, which is
// what makes it a preview of that admission rather than a second reading of the
// same inputs.
var errShown = errors.New("shown")

// standing prints where the repository stands and answers with the code that
// carries it.
//
// A position is a fact about the run tree, so it is read back off disk rather
// than assembled from what this invocation happened to do.
func standing(f repoFlags, id string, stdout io.Writer, ok int) int {
	at, err := position.Read(f.runs, id)
	if err != nil {
		_, _ = fmt.Fprintf(stdout, "position: unreadable: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprint(stdout, position.Render(at))
	if code, stop := stopOn[at.Standing]; stop {
		return code
	}
	return ok
}

// stopOn is every standing that ends the loop, and its code. A standing that is
// not in here is one with a next phase to run.
var stopOn = map[position.Standing]int{
	position.Finished: exitFinished,
	position.Parked:   exitParked,
	position.Waiting:  exitWaiting,
	position.Unusable: exitUnusable,
	position.Missing:  exitMissing,
}

// sentence says what the checkout is about to have done to it, in words.
func sentence(p repo.Plan) string {
	switch p.Action() {
	case repo.MakeIt:
		return "the lab will clone it"
	case repo.Adopt:
		return "already at this revision"
	case repo.Correct:
		return fmt.Sprintf("the lab's own clone, at %.12s, moving back to its pin", p.At)
	case repo.Read:
		return "yours, read and never written to"
	default:
		// Refused: a handed-in checkout at some other revision. The line says
		// where it is, and the refusal below says why it stays there.
		return fmt.Sprintf("yours, at %.12s", p.At)
	}
}

// admitNew indexes a repository that has never been admitted and records it.
func admitNew(ctx context.Context, f repoFlags, p repo.Plan, stdout, stderr io.Writer) int {
	if p.URL == "" {
		// A repository with no url is one nothing can re-clone, and every
		// command after this reads the catalog: recording it would leave a
		// config directory that fails to load until somebody edits it by hand.
		// It is refused here, before a scan is spent on it.
		_, _ = fmt.Fprintf(stderr, "sense-lab repo: %s has no origin, so there is no url to record and "+
			"nothing could clone it again. Give the clone an origin, or admit it by url\n", p.Checkout)
		return exitError
	}
	i, err := repo.Scan(ctx, f.senseBin, p, time.Now())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab repo: %v\n", err)
		return exitError
	}
	artifact := indexArtifact(f.runs, p.ID)
	if err := repo.Write(artifact, i); err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab repo: %v\n", err)
		return exitError
	}
	if !i.Indexed() {
		// The artifact says what happened and the repository is not admitted on
		// the strength of it. Writing the repository file here would advance a
		// repository whose index does not exist, and every phase after this one
		// would read it as a repository Sense has nothing to say about.
		_, _ = fmt.Fprintf(stdout, "index:    %s\n\n%s\n", artifact, i.Shortfall)
		return exitError
	}
	if err := writeRepoFile(f.config, p, i); err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab repo: %v\n", err)
		return exitError
	}
	_, _ = fmt.Fprintf(stdout, "index:    %d files, %d symbols, %d edges, %d embeddings, %v\nartifact: %s\nrepo:     %s\n",
		i.Files, i.Symbols, i.Edges, i.Embeddings, i.Names(), artifact, repoFile(f.config, p.ID))
	return exitOK
}

// writeRepoFile records the repository, and it happens last on purpose.
//
// The languages come from the scan rather than from a judgement about the
// repository, so the file cannot be written before there is an index to read
// them off. That ordering is also what makes a failed clone or a failed scan
// leave nothing behind: a repository file is the lab saying this repository is
// ready, and it is written when that is true.
func writeRepoFile(config string, p repo.Plan, i repo.Index) error {
	r := catalog.Repo{ID: p.ID, URL: p.URL, Commit: p.Revision, Languages: i.Names()}
	if !p.Owned {
		r.Checkout = p.Checkout
	}
	return repo.Write(repoFile(config, p.ID), r)
}

func repoFile(config, id string) string { return filepath.Join(config, "repos", id+".json") }

// indexArtifact is where the index phase's artifact lands: beside the cycles
// rather than inside one, because a repository is scanned once and a re-entry
// does not rescan it.
func indexArtifact(runs, id string) string {
	return filepath.Join(runs, id, "index", "index.json")
}

// admitSignals runs admission under the same signal handling a session gets. A
// clone is minutes of network and a scan is minutes of CPU, and an interrupt
// that did not reach them would leave both running behind a returned command.
func admitSignals(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return admitRepo(ctx, args, stdout, stderr)
}
