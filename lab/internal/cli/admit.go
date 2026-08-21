package cli

// Admitting a repository: resolving what somebody named, cloning it, holding it
// at its pin, indexing it, and recording it. It is not a command — the command
// that was called `repo` is gone, folded into `next`, because admission and
// advancing are the same question asked of a repository at two moments.

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
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

// admitted is what an admission produced: the index it read, and where the two
// files it wrote landed.
type admittedIndex struct {
	Index    repo.Index
	Artifact string
	RepoFile string
}

// admitNew indexes a repository that has never been admitted and records it.
//
// It reports what it found rather than printing it, because two commands admit
// a repository and they say so differently: `repo` prints a record, and `next`
// prints a page. What must NOT differ is which files were written and when, so
// that stays here and only the wording moves.
func admitNew(ctx context.Context, f repoFlags, p repo.Plan, stderr io.Writer) (admittedIndex, int) {
	var got admittedIndex
	if p.URL == "" {
		// A repository with no url is one nothing can re-clone, and every
		// command after this reads the catalog: recording it would leave a
		// config directory that fails to load until somebody edits it by hand.
		// It is refused here, before a scan is spent on it.
		_, _ = fmt.Fprintf(stderr, "%s has no origin, so there is no url to record and "+
			"nothing could clone it again. Give the clone an origin, or admit it by url\n", p.Checkout)
		return got, exitError
	}
	i, err := repo.Scan(ctx, f.senseBin, p, time.Now())
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return got, exitError
	}
	got = admittedIndex{Index: i, Artifact: indexArtifact(f.runs, p.ID), RepoFile: repoFile(f.config, p.ID)}
	if err := repo.Write(got.Artifact, i); err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return got, exitError
	}
	if !i.Indexed() {
		// The artifact says what happened and the repository is not admitted on
		// the strength of it. Writing the repository file here would advance a
		// repository whose index does not exist, and every phase after this one
		// would read it as a repository Sense has nothing to say about.
		return got, exitNotIndexed
	}
	if err := writeRepoFile(f.config, p, i); err != nil {
		_, _ = fmt.Fprintf(stderr, "%v\n", err)
		return got, exitError
	}
	return got, exitOK
}

// prefixed puts a command's own name in front of whatever is written to it, so
// one function can report an error for two commands without either of them
// answering in the other's name.
type prefixedWriter struct {
	to     io.Writer
	prefix string
}

func (w prefixedWriter) Write(b []byte) (int, error) {
	if _, err := io.WriteString(w.to, w.prefix); err != nil {
		return 0, err
	}
	return w.to.Write(b)
}

func prefixed(to io.Writer, prefix string) io.Writer { return prefixedWriter{to: to, prefix: prefix} }

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

// stopOn is every standing that ends the loop, and its code. A standing that is
// not in here is one with a next phase to run.
var stopOn = map[position.Standing]int{
	position.Finished: exitFinished,
	position.Parked:   exitParked,
	position.Waiting:  exitWaiting,
	position.Unusable: exitUnusable,
	position.Missing:  exitMissing,
	position.Blocked:  exitBlocked,
}
