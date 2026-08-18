package transcript

import (
	"fmt"
	"sort"
)

// Formats are the stream shapes this package can read, keyed by the name an
// agent tool declares in `transcript_format`.
//
// A LOOKUP, and the values come from the catalog. Tool identity picks an
// implementation here and changes nothing inside one: a reader that asked which
// tool it was reading for would be the branch this whole layer exists to
// prevent, and it would be invisible in every number afterwards.
//
// Keyed by FORMAT rather than by tool so that two tools that emit the same
// events share one reader instead of one being copied.
var Formats = map[string]func(path string) (Transcript, error){
	// Named for the SHAPE of the stream rather than for the tool that
	// emits it, which is also what keeps a vendor's own spelling of its
	// output format out of this file: `stream-json` is a value in one
	// tool's arguments, and the identifier probe is right to refuse it
	// here.
	"assistant-events": ReadClaudeCode,
	"item-events":      ReadItemEvents,
	"message-parts":    ReadMessageParts,
}

// Read normalizes a capture written in the named format.
//
// An unknown format is an error rather than a fallback to whichever reader came
// first. A stream read by the wrong reader parses cleanly, finds none of its
// events and produces an empty transcript, which reads exactly like an arm that
// said nothing.
func Read(format, path string) (Transcript, error) {
	read, ok := Formats[format]
	if !ok {
		return Transcript{}, fmt.Errorf("no reader for transcript format %q; the formats that exist are %v",
			format, FormatNames())
	}
	return read(path)
}

// FormatNames are the formats that can be read, for a message that can name
// them.
func FormatNames() []string {
	names := make([]string, 0, len(Formats))
	for name := range Formats {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// LegacyFormat is what a capture with no recorded format is read as.
//
// It is not a default in the "pick something" sense. Every run recorded before
// the format was written down came from one agent tool, so a capture with no
// format is a Claude Code capture as a matter of history, and 238 of them exist.
// A reader that refused them would make the corpus unreadable to prove a point.
const LegacyFormat = "assistant-events"

// FormatOfRun is the format a recorded run says its capture is in.
func FormatOfRun(recorded string) string {
	if recorded == "" {
		return LegacyFormat
	}
	return recorded
}
