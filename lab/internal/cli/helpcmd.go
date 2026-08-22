package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
)

// concepts is the vocabulary this instrument thinks in, defined once.
//
// Every one of these words is exactly right inside the binary and wrong on a
// terminal, so the four commands a person walks the flow with print none of
// them. They are still the words in the plan files, the verdicts, the run tree
// and the artifacts — everything an operator reads the moment they want to know
// HOW something was decided rather than WHAT was decided.
//
// A glossary at the point of use rather than a rename: the internal names are
// good, and renaming them would make the code worse to read in order to make
// one screen better.
var concepts = []struct{ word, means string }{
	{"cell", "one comparison: two arms run against the same scenario, at the same budget"},
	{"arm", "one session. The sense arm has Sense, the baseline arm does not, and nothing else differs"},
	{"sound", "a checked pair: the two arms differed in Sense access and in nothing else. Only a sound pair may be scored"},
	{"burned", "a session that finished and can never be paired, because the cell was stopped before its other side ran"},
	{"anchor", "the symbol a question is built around — the record, type or class the scenario asks about"},
	{"gold row", "one file:line an answer is expected to cite. The gold is every row a question has"},
	{"discriminator", "the group of gold rows a win or a loss is decided on. The other groups are recorded and do not score"},
	{"recall", "how many of the gold rows an arm actually cited, as a fraction"},
	{"the gap", "one arm's recall minus the other's. The method asks for 50 points before a question is worth paying for"},
	{"attempt", "one pass at writing a question. A repository gets six, and the sixth parks it"},
	{"park", "six attempts spent without a question that works. Recorded rather than dropped, and re-entered only by a deliberate act"},
	{"the judge", "the model that grades how an answer was reached. Pinned per repository, and never one of the arms"},
	{"the driver", "the model the stages themselves are run by: it writes the question, reads the trial and writes up the result"},
}

// helpTopics is what `sense-lab help <topic>` answers to.
const helpTopics = "concepts"

// helpCmd prints the usage, or one topic.
func helpCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("help", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	switch {
	case fs.NArg() == 0:
		_, _ = fmt.Fprint(stdout, usage)
		return exitOK
	case fs.Arg(0) == "concepts":
		_, _ = fmt.Fprint(stdout, glossary())
		return exitOK
	default:
		_, _ = fmt.Fprintf(stderr, "sense-lab help: no topic %q; there is %s\n", fs.Arg(0), helpTopics)
		return exitUsage
	}
}

// glossary is the page. The words are sorted by nothing: they are in the order
// somebody meets them walking the flow, which is what a glossary read once is
// for.
func glossary() string {
	var b strings.Builder
	b.WriteString("\n  The words this instrument uses, and what each one means.\n\n" +
		"  None of these appear in what `next` and `pay` print — they are the vocabulary of\n" +
		"  the plan files, the verdicts and the run tree, for when you want to know how\n" +
		"  something was decided rather than what was decided.\n\n")
	width := 0
	for _, c := range concepts {
		if len(c.word) > width {
			width = len(c.word)
		}
	}
	for _, c := range concepts {
		fmt.Fprintf(&b, "      %-*s  %s\n", width, c.word, c.means)
	}
	b.WriteString("\n  The whole record behind one repository:  sense-lab why <repo>\n")
	return b.String()
}
