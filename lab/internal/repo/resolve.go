// Package repo admits a repository to the lab: it works out what was named,
// clones it, pins it at the tree it actually got, and records what an index
// over that tree holds.
//
// Admission used to be three manual acts that nothing checked against each
// other — a clone, a hand-written `repos/<id>.json` with a sha copied out of
// it, and a scan — and the failure that produced is silent. A sha in the json
// that is not the sha the clone sits at gives every later run a worktree at the
// wrong tree, and nothing about a result says so.
//
// The package is split by what it may do. [Resolve] decides, and it is pure: it
// reaches no network and no disk, so what a name resolves to can be table
// tested. Everything below it acts, and it acts only after the decision has
// been announced.
package repo

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// Kind is what an input turned out to name.
type Kind string

const (
	// Known is a repository the catalog already holds. Nothing is admitted:
	// the command prints where it stands.
	Known Kind = "known"
	// Local is a path that exists on disk, handed in by whoever ran the
	// command. The lab reads it and never writes to it.
	Local Kind = "local"
	// Handle is `owner/name`, which is a github repository.
	Handle Kind = "handle"
	// Remote is anything carrying a scheme, taken verbatim.
	Remote Kind = "remote"
)

// Resolution is what an input came to: which repository, by which reading.
type Resolution struct {
	Kind Kind
	ID   string
	// URL is where the repository comes from. Empty for a [Local] path, whose
	// origin can only be read out of the clone.
	URL string
	// Path is the handed-in directory, set for [Local] and nothing else.
	Path string
}

// handle is `owner/name` and nothing else: two segments of the characters
// github allows in a name, with no scheme, no host and no third segment. It is
// deliberately strict, because everything it does not match falls through to a
// refusal rather than to a guess at a url.
var handle = regexp.MustCompile(`^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$`)

// scheme is a url's leading `git@host:` or `xxx://`. The scp-like form is
// matched too, because that is what a private repository is usually handed in
// as and treating it as a path would refuse it for not existing.
var scheme = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9+.-]*://|[^/]+@[^/:]+:)`)

// Resolve works out which repository an input names.
//
// The order is the contract, and it is first match wins: a known id, then a
// path on disk, then `owner/name`, then a url. The ambiguous case is real — a
// local directory named `owner/name` is readable both ways — and it resolves as
// the path, because a directory that is there is evidence and a handle is a
// guess.
//
// It is pure. `known` is the catalog's ids and `onDisk` reports whether a path
// is a directory; both are the caller's to supply, so nothing here decides a
// repository by reaching for a network that may answer differently tomorrow.
func Resolve(in string, known []string, onDisk func(string) bool) (Resolution, error) {
	in = strings.TrimSpace(in)
	switch {
	case in == "":
		return Resolution{}, fmt.Errorf("name a repository: an id already admitted, a path to a clone, `owner/name`, or a url")
	case slices.Contains(known, in):
		return Resolution{Kind: Known, ID: in}, nil
	case onDisk(in):
		return Resolution{Kind: Local, ID: idOfPath(in), Path: in}, nil
	case handle.MatchString(in):
		// The suffix is trimmed before the url is built, not after: a handle
		// typed as `owner/thing.git` is the same repository as `owner/thing`,
		// and appending to the untrimmed name would resolve it to a url ending
		// `.git.git` — which clones nothing, at the end of a phase.
		owner, name, _ := strings.Cut(in, "/")
		name = trimGit(name)
		return Resolution{Kind: Handle, ID: name, URL: "https://github.com/" + owner + "/" + name + ".git"}, nil
	case scheme.MatchString(in):
		return Resolution{Kind: Remote, ID: idOfURL(in), URL: in}, nil
	}
	return Resolution{}, fmt.Errorf("cannot read %q as a repository: it is not an admitted id, "+
		"there is no such directory, and it is neither `owner/name` nor a url", in)
}

// idOfPath and idOfURL take the repository's own name as its id, which is the
// convention every repository file already follows: `discourse/discourse` is
// `discourse`, and so is a clone of it sitting in a directory of that name.
//
// Two repositories whose names collide would collide here. That is left alone
// rather than defended against with a disambiguating suffix nobody would
// recognise: the second admission finds the first's id already known, prints
// where that repository stands, and the collision is visible in the scrollback
// instead of buried in an id.
func idOfPath(path string) string {
	path = strings.TrimRight(path, "/")
	if i := strings.LastIndex(path, "/"); i >= 0 {
		path = path[i+1:]
	}
	return trimGit(path)
}

func idOfURL(url string) string {
	url = strings.TrimRight(url, "/")
	url = url[strings.LastIndexAny(url, "/:")+1:]
	return trimGit(url)
}

func trimGit(name string) string { return strings.TrimSuffix(name, ".git") }
