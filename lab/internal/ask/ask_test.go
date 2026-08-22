package ask

import (
	"errors"
	"strings"
	"testing"
)

func asker(t *testing.T, answer string, opts ...func(*Asker)) (*Asker, *strings.Builder) {
	t.Helper()
	out := &strings.Builder{}
	a := &Asker{In: strings.NewReader(answer), Out: out, Terminal: true}
	for _, o := range opts {
		o(a)
	}
	return a, out
}

func unattended(a *Asker) { a.Terminal = false }
func assumed(a *Asker)    { a.Assumed = true }

func TestReturnMeansYes(t *testing.T) {
	for _, answer := range []string{"\n", "y\n", "Y\n", "yes\n", "YES\n"} {
		a, _ := asker(t, answer)
		if err := a.Continue("run three stages"); err != nil {
			t.Errorf("answering %q = %v, want it to proceed", answer, err)
		}
	}
}

func TestAnythingElseDeclines(t *testing.T) {
	for _, answer := range []string{"n\n", "no\n", "N\n", "later\n", "q\n"} {
		a, _ := asker(t, answer)
		if err := a.Continue("run three stages"); !errors.Is(err, ErrDeclined) {
			t.Errorf("answering %q = %v, want ErrDeclined", answer, err)
		}
	}
}

// A closed stdin has not agreed to anything. The failure this prevents is a
// command in a pipeline reading end-of-input and reading it as consent.
func TestEndOfInputDeclines(t *testing.T) {
	a, _ := asker(t, "")
	if err := a.Continue("run three stages"); !errors.Is(err, ErrDeclined) {
		t.Errorf("end of input = %v, want ErrDeclined", err)
	}
}

// The unattended path never asks and never prints. An operator who has already
// answered on the command line is not asked again.
func TestTheFlagAnswersWithoutAsking(t *testing.T) {
	a, out := asker(t, "n\n", assumed)

	if err := a.Continue("run three stages"); err != nil {
		t.Errorf("with -yes = %v, want it to proceed", err)
	}
	if out.String() != "" {
		t.Errorf("with -yes it printed %q, want nothing", out)
	}
}

// No terminal and no flag is a refusal, in both directions: proceeding would
// let a scheduled run spend because a question could not be put, and waiting
// would hold it forever on a prompt nothing will answer.
func TestNoTerminalAndNoFlagIsRefused(t *testing.T) {
	a, out := asker(t, "y\n", unattended)

	err := a.Continue("spend on six sessions")
	if err == nil || errors.Is(err, ErrDeclined) {
		t.Fatalf("unattended = %v, want a refusal that is not a decline", err)
	}
	for _, want := range []string{"spend on six sessions", "-yes"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal %q does not name %q", err, want)
		}
	}
	if out.String() != "" {
		t.Errorf("it asked anyway: %q", out)
	}
}

func TestTheQuestionIsPutBeforeTheAnswerIsRead(t *testing.T) {
	a, out := asker(t, "y\n")

	if err := a.Continue("run three stages"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "[Y/n]") {
		t.Errorf("it read an answer without asking: %q", out)
	}
}

// The money step takes the repository's own name. A keystroke is the same
// keystroke whether the six lines above it were read or not.
func TestTheNameIsTypedBackToConfirmSpending(t *testing.T) {
	a, out := asker(t, "mastodon\n")

	if err := a.Name("mastodon"); err != nil {
		t.Errorf("typing the name = %v, want it to proceed", err)
	}
	if !strings.Contains(out.String(), "Type the repository name") {
		t.Errorf("it did not ask: %q", out)
	}
}

func TestAWrongNameOrAnEmptyLineCancelsTheSpending(t *testing.T) {
	for _, answer := range []string{"\n", "y\n", "yes\n", "mastodo\n", "MASTODON\n", " \n"} {
		a, _ := asker(t, answer)
		if err := a.Name("mastodon"); !errors.Is(err, ErrDeclined) {
			t.Errorf("answering %q = %v, want ErrDeclined", answer, err)
		}
	}
}

// A name typed with the whitespace a paste carries still counts. This is the
// one place a reader is asked to type rather than press a key, and refusing a
// trailing space would send them round again for nothing.
func TestASpacedNameStillCounts(t *testing.T) {
	a, _ := asker(t, "  mastodon  \n")
	if err := a.Name("mastodon"); err != nil {
		t.Errorf("a padded name = %v, want it to proceed", err)
	}
}

func TestTheNameIsNotAskedForUnattended(t *testing.T) {
	a, out := asker(t, "", assumed)
	if err := a.Name("mastodon"); err != nil {
		t.Errorf("with -yes = %v, want it to proceed", err)
	}
	if out.String() != "" {
		t.Errorf("with -yes it printed %q, want nothing", out)
	}

	b, _ := asker(t, "mastodon\n", unattended)
	err := b.Name("mastodon")
	if err == nil || errors.Is(err, ErrDeclined) {
		t.Fatalf("unattended = %v, want a refusal that is not a decline", err)
	}
	if !strings.Contains(err.Error(), "mastodon") {
		t.Errorf("the refusal %q does not name the repository", err)
	}
}

// A read that fails is not an answer. It is reported as itself rather than
// folded into a decline, because "the terminal broke" and "the operator said
// no" send whoever is reading to opposite places.
func TestAFailedReadIsNotADecline(t *testing.T) {
	a := &Asker{In: broken{}, Out: &strings.Builder{}, Terminal: true}

	err := a.Continue("run three stages")
	if err == nil || errors.Is(err, ErrDeclined) {
		t.Fatalf("a broken reader = %v, want the read error", err)
	}
	if !strings.Contains(err.Error(), "read the answer") {
		t.Errorf("the error %q does not say the read failed", err)
	}

	// And on the money question, where mistaking a broken terminal for a
	// decline would read as the operator having cancelled a spend they never
	// saw.
	b := &Asker{In: broken{}, Out: &strings.Builder{}, Terminal: true}
	if err := b.Name("mastodon"); err == nil || errors.Is(err, ErrDeclined) {
		t.Fatalf("a broken reader on the name = %v, want the read error", err)
	}
}

type broken struct{}

func (broken) Read([]byte) (int, error) { return 0, errors.New("the terminal went away") }
