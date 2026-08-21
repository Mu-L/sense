package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/repo"
)

// What the page says it is about to do to a checkout, for each of the five
// things it can be. The distinction that matters is ownership: the lab fixes
// what it made and reads what it was given, and the sentence is where an
// operator sees which of the two is happening.
func TestTheCheckoutSentenceSaysWhoOwnsTheTree(t *testing.T) {
	for _, tc := range []struct {
		what string
		plan repo.Plan
		want string
	}{
		{"one the lab will make", repo.Plan{Owned: true, URL: "https://example.test/r.git"},
			"the lab will clone it"},
		{"the lab's own, at its pin", repo.Plan{Owned: true, Admitted: true, At: "abc123", Revision: "abc123"},
			"already at this revision"},
		{"the lab's own, drifted", repo.Plan{Owned: true, Admitted: true, At: "def456", Revision: "abc123"},
			"moving back to its pin"},
		{"one handed in", repo.Plan{Checkout: "/somewhere", At: "abc123", Revision: "abc123"},
			"read and never written to"},
		{"one handed in that moved", repo.Plan{Checkout: "/somewhere", Admitted: true, At: "def456", Revision: "abc123"},
			"yours, at def456"},
	} {
		if got := sentence(tc.plan); !strings.Contains(got, tc.want) {
			t.Errorf("%s: sentence = %q, want it to say %q", tc.what, got, tc.want)
		}
	}
}

// One function admits a repository for two commands that word it differently,
// so an error it reports has to arrive in the name of whichever asked.
func TestAnErrorArrivesInTheNameOfTheCommandThatAsked(t *testing.T) {
	var to strings.Builder

	n, err := prefixed(&to, "sense-lab next: ").Write([]byte("the clone could not be read\n"))

	if err != nil {
		t.Fatal(err)
	}
	if n != len("the clone could not be read\n") {
		t.Errorf("wrote %d, want the length of the message rather than of the message and its prefix", n)
	}
	if got := to.String(); got != "sense-lab next: the clone could not be read\n" {
		t.Errorf("wrote %q", got)
	}
}

// A writer that fails takes its error back rather than reporting a short write
// as a success.
func TestAPrefixThatCannotBeWrittenIsReported(t *testing.T) {
	if _, err := prefixed(brokenWriter{}, "sense-lab next: ").Write([]byte("anything")); err == nil {
		t.Error("a writer that fails reported success")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("the pipe went away") }
