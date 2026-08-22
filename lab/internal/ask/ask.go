// Package ask is the one place this instrument waits for a person.
//
// It exists at the COMMAND BOUNDARY and nowhere else. `NO HUMAN GATE IN THE
// PER-REPO PHASES` is a law, mechanised as no interactive prompt in the run
// path, and it is about the phases: they author, spend, diagnose and swap
// without anybody ticking a box between them. Nothing here is ever reached
// between two phases — it is reached once, before the first one spawns, when
// the operator is still holding the keyboard they typed the command with.
//
// Three properties make that distinction hold rather than merely be claimed:
//
//   - an unattended caller has a path that never asks, which is [Asker.Assumed]
//   - a caller with no terminal and no such flag is REFUSED rather than run, so
//     a cron job can neither hang on a question nobody will answer nor spend
//     because a question could not be put
//   - the question states what it is confirming, so an answer means something
//     more than a reflex
package ask

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrDeclined is what a caller checks for: the operator was asked and said no.
// It is not a failure and a caller that prints it as one is lying about what
// happened.
var ErrDeclined = errors.New("declined")

// Asker puts one question, at the boundary, before anything runs.
type Asker struct {
	In  io.Reader
	Out io.Writer
	// Assumed is the -yes flag: the operator has already answered, on the
	// command line, and nothing is printed or read.
	Assumed bool
	// Terminal reports whether there is somebody there to answer. It is a field
	// rather than a call because deciding it means reaching a file descriptor,
	// and that belongs at the edge with the other effects.
	Terminal bool
}

// Continue puts the yes-or-no, having been preceded by whatever the caller
// printed about consequence. Empty means yes, because the question is asked
// with a capital Y and a reader who presses return has agreed.
func (a Asker) Continue(subject string) error {
	if a.Assumed {
		return nil
	}
	if !a.Terminal {
		return a.unattended(subject)
	}
	_, _ = fmt.Fprint(a.Out, "  Continue? [Y/n] ")
	answer, err := a.line()
	if err != nil {
		return err
	}
	switch strings.ToLower(answer) {
	case "", "y", "yes":
		return nil
	default:
		return ErrDeclined
	}
}

// Name asks for the repository's name to be typed back, which is the
// confirmation the irreversible step gets.
//
// Typing a name is not ceremony here. The act being confirmed spends money that
// cannot be recovered and pairs arms that cannot be re-paired, and a keystroke
// answers it too easily: `y` is the same keystroke whether the reader took in
// the six lines above it or not. The repository's own name is not.
func (a Asker) Name(repo string) error {
	if a.Assumed {
		return nil
	}
	if !a.Terminal {
		return a.unattended("spend on " + repo)
	}
	_, _ = fmt.Fprint(a.Out, "     Type the repository name to confirm, or press Enter to cancel:  ")
	answer, err := a.line()
	if err != nil {
		return err
	}
	if answer != repo {
		return ErrDeclined
	}
	return nil
}

// unattended is the refusal a caller with nobody to ask gets.
//
// Refused rather than assumed, in both directions. Proceeding would let a
// scheduled run spend because a question could not be put, and waiting would
// hold a job forever on a prompt nothing will answer. The message names the
// flag, because the operator who hits this is running from a script and needs
// the one thing that fixes it.
func (a Asker) unattended(subject string) error {
	return fmt.Errorf("this would %s, and there is no terminal to confirm it on. "+
		"Pass -yes to run it unattended", subject)
}

// line is one answer, trimmed. End of input is a decline: a closed stdin has
// not agreed to anything.
func (a Asker) line() (string, error) {
	r := bufio.NewReader(a.In)
	text, err := r.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the answer: %w", err)
	}
	if errors.Is(err, io.EOF) && strings.TrimSpace(text) == "" {
		return "", ErrDeclined
	}
	return strings.TrimSpace(text), nil
}
