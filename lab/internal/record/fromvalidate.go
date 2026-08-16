package record

import (
	"time"

	"github.com/luuuc/sense/lab/internal/validate"
)

// FromValidate records a measured candidate, whichever way it went.
//
// A rejection is a result and is recorded with its numbers, not discarded. The
// next person to have the same idea should find the measurement rather than
// repeat it, and a path that only writes down its successes is a path that
// invites the same experiment twice.
func FromValidate(dir, candidate string, o validate.Outcome, now time.Time) (Validation, error) {
	v := Validation{
		Candidate:  candidate,
		Before:     map[string]float64{},
		After:      map[string]float64{},
		Regression: o.Reason(),
		CorpusSize: o.CorpusSize,
		Decision:   o.Decision,
		Reason:     o.Reason(),
	}
	for _, t := range o.Targets {
		key := t.Cell + "/" + t.Model
		v.Before[key] = t.Before
		v.After[key] = t.After
	}
	return RecordValidation(dir, v, now)
}
