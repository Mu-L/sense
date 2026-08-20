package record

import (
	"time"

	"github.com/luuuc/sense/lab/internal/mine"
)

// FromMine records the miner's output as findings, with the runs it was mined
// from as the evidence.
//
// It exists so the miner's output lands without hand-editing. The retired tree's
// failure was exactly the gap this closes: miner output in a log file, an empty
// findings directory, and three loose write-ups under a different tree,
// connected to nothing.
//
// The runs are the whole mined set rather than the subset each finding fired on.
// A finding says "3 of 6 runs" and the evidence is those six: knowing which
// three requires opening them, and a record that named only the three would make
// the denominator unverifiable.
func FromMine(dir string, found []mine.Finding, runs []string, now time.Time) ([]Finding, error) {
	out := make([]Finding, 0, len(found))
	for _, f := range found {
		recorded, err := RecordFinding(dir, Finding{
			Surface:       string(f.Surface),
			Detector:      f.Detector,
			Subject:       f.Subject,
			Detail:        f.String(),
			Runs:          runs,
			Discriminator: f.Discriminator,
		}, now)
		if err != nil {
			return nil, err
		}
		out = append(out, recorded)
	}
	return out, nil
}
