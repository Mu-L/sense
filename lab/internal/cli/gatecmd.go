package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"

	"github.com/luuuc/sense/lab/internal/gate"
)

// `sense-lab gate` answers one question — may this cell be paid for — and
// answers it with an exit code.
//
// It exists because a gate that only builds a report string constrains nothing.
// The old tree has the receipt: a rule quoted as law lived inside a function
// that renders a sentence, one of the two files it came from was never executed
// at all, and nobody knew. The refusals are printed because a person has to
// know which gate fired; the exit code is what makes them a gate.
//
// There is no bypass flag, and there is not going to be one. When a gate is
// wrong mid-campaign the sanctioned path is to fix the gate, or to record a
// ruling that exempts that one cell with the reason attached to it.

func gateCmd(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		if err != flag.ErrHelp {
			_, _ = fmt.Fprintf(stderr, "sense-lab gate: %v\n", err)
		}
		return exitUsage
	}

	var d gate.Decision
	dec := json.NewDecoder(stdin)
	// An unknown field is a decision describing something these gates do not
	// check, and accepting it silently would report a pass on a question nobody
	// asked.
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab gate: read the decision: %v\n", err)
		return exitUsage
	}

	refused := gate.Refusals(d)
	if len(refused) == 0 {
		_, _ = fmt.Fprintln(stdout, "no gate refuses this cell")
		return exitOK
	}
	// Every gate that fired, not the first: an operator who fixes one thing
	// only to be refused by the next has learned nothing about the cell.
	_, _ = fmt.Fprintf(stderr, "%d gate(s) refuse this cell:\n", len(refused))
	for _, err := range refused {
		_, _ = fmt.Fprintf(stderr, "  %v\n", err)
	}
	return exitRefused
}
