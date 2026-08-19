package repo

import (
	"os"
	"path/filepath"
	"testing"
)

// The resolver decides which repository a name means, and getting that wrong
// quietly is worse than refusing: every phase after it reads a repository
// nobody chose. So the whole order is table tested, including the one case
// where two readings are both defensible.
func TestResolveReadsANameByTheDocumentedOrder(t *testing.T) {
	known := []string{"discourse"}
	nowhere := func(string) bool { return false }
	everywhere := func(string) bool { return true }
	thisOne := func(path string) bool {
		return path == "/clones/rails" || path == "/clones/rails/" || path == "owner/name"
	}

	for _, tc := range []struct {
		name string
		in   string
		// onDisk is per case, because the order is only testable where the
		// readings compete: what a name resolves to depends on what is there.
		onDisk func(string) bool
		want   Resolution
	}{
		{"an id the catalog already holds is not admitted again",
			"discourse", nowhere, Resolution{Kind: Known, ID: "discourse"}},
		{"a path that exists is a local clone",
			"/clones/rails", thisOne, Resolution{Kind: Local, ID: "rails", Path: "/clones/rails"}},
		{"owner/name is a github repository",
			"jellyfin/jellyfin", nowhere, Resolution{Kind: Handle, ID: "jellyfin",
				URL: "https://github.com/jellyfin/jellyfin.git"}},
		{"a name that is not the owner's is still the repository's",
			"rails/activerecord", nowhere, Resolution{Kind: Handle, ID: "activerecord",
				URL: "https://github.com/rails/activerecord.git"}},
		{"anything with a scheme is taken verbatim",
			"https://gitlab.test/team/thing.git", nowhere, Resolution{Kind: Remote, ID: "thing",
				URL: "https://gitlab.test/team/thing.git"}},
		{"an scp-like url is a url, not a path that is missing",
			"git@github.com:owner/private.git", nowhere, Resolution{Kind: Remote, ID: "private",
				URL: "git@github.com:owner/private.git"}},
		{"a trailing slash does not become the id",
			"/clones/rails/", thisOne, Resolution{Kind: Local, ID: "rails", Path: "/clones/rails/"}},
		{"surrounding whitespace is not part of a name",
			"  jellyfin/jellyfin  ", nowhere, Resolution{Kind: Handle, ID: "jellyfin",
				URL: "https://github.com/jellyfin/jellyfin.git"}},
		// The ambiguity is real rather than theoretical: `owner/name` is a
		// handle and a relative directory at the same time. The path wins,
		// because a directory that is there is evidence and a handle is a
		// guess — and a run that cloned github while the operator meant the
		// clone in front of them would be measuring somebody else's tree.
		{"a path that exists beats a handle that would read the same",
			"owner/name", thisOne, Resolution{Kind: Local, ID: "name", Path: "owner/name"}},
		// And an admitted id beats both. Admission has to be idempotent for
		// the crank to call it on every invocation, and a second reading of an
		// admitted id would re-admit it.
		{"an admitted id beats a directory of the same name",
			"discourse", everywhere, Resolution{Kind: Known, ID: "discourse"}},
		// A `.git` suffix is how a clone directory and a url both spell the
		// same repository, and an id carrying it would make one repository
		// admittable twice under two names.
		{"a .git suffix is not part of an id, in a handle",
			"owner/thing.git", nowhere, Resolution{Kind: Handle, ID: "thing",
				URL: "https://github.com/owner/thing.git"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tc.in, known, tc.onDisk)
			if err != nil {
				t.Fatalf("Resolve(%q) = %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("Resolve(%q) = %+v, want %+v", tc.in, got, tc.want)
			}
		})
	}
}

func TestResolveRefusesWhatItCannotRead(t *testing.T) {
	for _, tc := range []struct{ name, in string }{
		{"nothing at all", ""},
		{"only whitespace", "   "},
		{"a bare word that is neither an id nor a path", "jellyfin"},
		{"three segments, which is not a handle", "github.com/owner/name"},
		// The resolver is the door: every name that gets past it is handed to
		// git as an operand. A name that reads as an option would otherwise
		// reach `git clone`, where `--upload-pack` runs whatever it names.
		{"a name that would read as a git option", "--upload-pack=touch /tmp/pwned"},
		{"a name that would read as a git option, with a slash in it", "-c/protocol.ext.allow=always"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := Resolve(tc.in, nil, func(string) bool { return false }); err == nil {
				t.Errorf("Resolve(%q) resolved; a guess here is a repository nobody chose", tc.in)
			}
		})
	}
}

// OnDisk is what the command hands Resolve, and it is the same test the
// checkout side uses to find one. A file is not a checkout.
func TestOnDiskIsDirectoriesOnly(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "README")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !OnDisk(dir) {
		t.Error("OnDisk(a directory) = false")
	}
	if OnDisk(file) {
		t.Error("OnDisk(a file) = true; a file is not a clone")
	}
	if OnDisk(filepath.Join(dir, "absent")) {
		t.Error("OnDisk(nothing) = true")
	}
}
