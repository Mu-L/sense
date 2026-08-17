// Package record keeps the chain from an observation to a shipped change.
//
// The miner produces observations. Between one and a product change there are
// three steps that otherwise happen in someone's head and in prose: deciding an
// observation is a finding, forming a hypothesis about which surface is at
// fault, and proposing a change.
//
// Unrecorded, that chain has two failure modes and the retired tree shows both.
// One campaign's miner output sits in a log file, its findings directory is
// empty, and the findings that were written up live as three loose markdown
// files under a different campaign, connected to nothing. And "why was this
// change made to Sense" is answerable only by someone who remembers.
//
// # Three records, each pointing at the last
//
// A finding is what the miner saw. A candidate is a proposed change with its
// hypothesis. A validation is that candidate measured, with the decision and its
// reason.
//
// A finding may exist without a candidate, and most will. That is the honest
// state of most observations, and a schema that pressured every finding into a
// candidate would manufacture work.
//
// A candidate may not exist without a finding. A change with no observed problem
// behind it is taste, and taste does not earn its way in on this bench.
//
// # Evidence is a path, never a copy
//
// A finding names run directories. One that embedded a transcript excerpt would
// drift from the transcript the moment either was touched, and the run tree is
// already the source of truth.
//
// A finding also inherits the state of its evidence: invalidating a run marks
// every finding that cites it. A finding whose evidence is entirely invalidated
// is marked rather than deleted, because the fact that it was once observed is
// itself worth keeping.
//
// # It is a record, not a tracker
//
// No states beyond what is needed, no assignment, no triage, no priority. Every
// state added is a state someone has to maintain by hand. And no database:
// three record types with ids pointing at each other is not a schema, it is
// files with ids, and if querying them ever genuinely hurts, that is the moment
// SQLite was always waiting for.
package record

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// The three record kinds, which are also their directories.
const (
	kindFinding    = "findings"
	kindCandidate  = "candidates"
	kindValidation = "validations"
)

// stamp is how a time is written. Absolute and to the second, because a
// hypothesis is only "before" a validation if both can be compared later by a
// reader who was not there.
const stamp = time.RFC3339

// Finding is what the miner saw, and where the evidence for it is.
type Finding struct {
	ID string `json:"id"`
	// Surface is the Sense surface at fault. A finding without one is a number
	// again, and it is refused.
	Surface  string `json:"surface"`
	Detector string `json:"detector"`
	Subject  string `json:"subject"`
	Detail   string `json:"detail"`
	// Runs are paths into the run tree. Never a copy of what is in them.
	Runs []string `json:"runs"`
	// Invalidated names the runs that have since been invalidated. It is a
	// subset of Runs and nothing is removed from Runs: a finding that quietly
	// dropped its dead evidence would look like a finding with less support
	// rather than one whose support was withdrawn.
	Invalidated []string `json:"invalidated,omitempty"`
	// Dead says every run behind this finding has been invalidated. The finding
	// stays, because it was once observed.
	Dead          bool   `json:"dead,omitempty"`
	Discriminator bool   `json:"discriminator,omitempty"`
	RecordedAt    string `json:"recorded_at"`
}

// Standing reports whether this finding still has evidence.
func (f Finding) Standing() bool { return !f.Dead }

// Candidate is a proposed change to Sense.
type Candidate struct {
	ID string `json:"id"`
	// Finding is the observation behind it. Required.
	Finding    string `json:"finding"`
	Surface    string `json:"surface"`
	Hypothesis string `json:"hypothesis"`
	// HypothesisAt is when the hypothesis was written down. It is compared
	// against a validation's time, because a hypothesis written afterwards is a
	// description.
	HypothesisAt string `json:"hypothesis_at"`
	// TargetCells are the cells this change expects to move.
	TargetCells []string `json:"target_cells,omitempty"`
}

// Validation is a candidate measured.
type Validation struct {
	ID        string `json:"id"`
	Candidate string `json:"candidate"`
	// Before and After are the cells' margins, keyed by cell.
	Before map[string]float64 `json:"before,omitempty"`
	After  map[string]float64 `json:"after,omitempty"`
	// Regression is what the banked corpus said, and CorpusSize is how many
	// cells said it. The size travels with the result: a pass over four cells
	// and a pass over forty are different claims.
	Regression string `json:"regression"`
	CorpusSize int    `json:"corpus_size"`
	Decision   string `json:"decision"`
	Reason     string `json:"reason"`
	RanAt      string `json:"ran_at"`
}

// The decisions a validation can reach. There is no third: a validation that
// neither accepts nor rejects is a validation that has not finished.
const (
	Accepted = "accepted"
	Rejected = "rejected"
)

// RecordFinding writes a finding.
//
// The id is derived from what the finding is about rather than generated, so
// running the miner again over the same corpus updates the finding it already
// wrote instead of creating a second one beside it.
func RecordFinding(dir string, f Finding, now time.Time) (Finding, error) {
	if f.Surface == "" {
		return Finding{}, fmt.Errorf("a finding with no surface: the route from a number to a fix runs through a surface")
	}
	if len(f.Runs) == 0 {
		return Finding{}, fmt.Errorf("finding %q cites no run; evidence is a path into the run tree", f.Subject)
	}
	f.ID = id("f", f.Surface, f.Detector, f.Subject)
	f.RecordedAt = now.Format(stamp)
	slices.Sort(f.Runs)
	return f, write(dir, kindFinding, f.ID, f)
}

// RecordCandidate writes a proposed change.
//
// It refuses a candidate whose finding is not on disk. A change with no observed
// problem behind it is taste, and a finding id that names nothing is the same
// thing with a reference attached.
func RecordCandidate(dir string, c Candidate, now time.Time) (Candidate, error) {
	if c.Hypothesis == "" {
		return Candidate{}, fmt.Errorf("a candidate with no hypothesis; there would be nothing for the measurement to be about")
	}
	f, err := readFinding(dir, c.Finding)
	if err != nil {
		return Candidate{}, err
	}
	if c.Surface == "" {
		c.Surface = f.Surface
	}
	c.ID = id("c", c.Finding, c.Hypothesis)
	c.HypothesisAt = now.Format(stamp)
	return c, write(dir, kindCandidate, c.ID, c)
}

// RecordValidation writes a candidate's measurement.
//
// It refuses a validation that ran before its hypothesis was written down. A
// hypothesis recorded afterwards is a description of what happened, and the
// whole point of writing it first is that it can be wrong.
func RecordValidation(dir string, v Validation, now time.Time) (Validation, error) {
	c, err := readCandidate(dir, v.Candidate)
	if err != nil {
		return Validation{}, err
	}
	if v.Decision != Accepted && v.Decision != Rejected {
		return Validation{}, fmt.Errorf("validation of %s decided %q; it accepts or it rejects", v.Candidate, v.Decision)
	}
	if v.Reason == "" {
		return Validation{}, fmt.Errorf("validation of %s gives no reason; a decision nobody can re-read is a decision nobody can revisit", v.Candidate)
	}
	written, err := time.Parse(stamp, c.HypothesisAt)
	if err != nil {
		return Validation{}, fmt.Errorf("candidate %s has an unreadable hypothesis time %q: %w", c.ID, c.HypothesisAt, err)
	}
	if now.Before(written) {
		return Validation{}, fmt.Errorf("this validation ran at %s and candidate %s wrote its hypothesis at %s; a hypothesis recorded after the measurement is a description",
			now.Format(stamp), c.ID, c.HypothesisAt)
	}
	v.ID = id("v", v.Candidate, now.Format(stamp))
	v.RanAt = now.Format(stamp)
	return v, write(dir, kindValidation, v.ID, v)
}

// InvalidateRun marks every finding that cites a run, and reports them.
//
// Nothing is deleted. A finding whose evidence is entirely withdrawn is marked
// dead and kept, because it was once observed and that is worth knowing.
func InvalidateRun(dir, run string) ([]Finding, error) {
	all, err := Findings(dir)
	if err != nil {
		return nil, err
	}
	var touched []Finding
	for _, f := range all {
		if !slices.Contains(f.Runs, run) || slices.Contains(f.Invalidated, run) {
			continue
		}
		f.Invalidated = append(f.Invalidated, run)
		slices.Sort(f.Invalidated)
		f.Dead = len(f.Invalidated) == len(f.Runs)
		if err := write(dir, kindFinding, f.ID, f); err != nil {
			return nil, err
		}
		touched = append(touched, f)
	}
	return touched, nil
}

// Chain is one observation and everything that came of it. It answers both
// directions: forward from a finding to whether anything was done, and backward
// from a change to the transcripts it came from.
type Chain struct {
	Finding     Finding
	Candidates  []Candidate
	Validations []Validation
}

// Evidence is where to look: the run directories behind the finding.
func (c Chain) Evidence() []string { return c.Finding.Runs }

// Trace reports the chain around any record id, whichever end it is given.
func Trace(dir, recordID string) (Chain, error) {
	findingID, err := findingBehind(dir, recordID)
	if err != nil {
		return Chain{}, err
	}
	f, err := readFinding(dir, findingID)
	if err != nil {
		return Chain{}, err
	}
	chain := Chain{Finding: f}

	candidates, err := Candidates(dir)
	if err != nil {
		return Chain{}, err
	}
	validations, err := Validations(dir)
	if err != nil {
		return Chain{}, err
	}
	for _, c := range candidates {
		if c.Finding != f.ID {
			continue
		}
		chain.Candidates = append(chain.Candidates, c)
		for _, v := range validations {
			if v.Candidate == c.ID {
				chain.Validations = append(chain.Validations, v)
			}
		}
	}
	return chain, nil
}

// findingBehind resolves any id to the finding at the head of its chain.
func findingBehind(dir, recordID string) (string, error) {
	switch {
	case strings.HasPrefix(recordID, "f-"):
		return recordID, nil
	case strings.HasPrefix(recordID, "c-"):
		c, err := readCandidate(dir, recordID)
		return c.Finding, err
	case strings.HasPrefix(recordID, "v-"):
		v, err := read[Validation](dir, kindValidation, recordID)
		if err != nil {
			return "", err
		}
		c, err := readCandidate(dir, v.Candidate)
		return c.Finding, err
	}
	return "", fmt.Errorf("%q is not a record id", recordID)
}

// Findings, Candidates and Validations list what is on disk, by id.
func Findings(dir string) ([]Finding, error) { return list[Finding](dir, kindFinding) }

func Candidates(dir string) ([]Candidate, error) { return list[Candidate](dir, kindCandidate) }

func Validations(dir string) ([]Validation, error) { return list[Validation](dir, kindValidation) }

func readFinding(dir, recordID string) (Finding, error) {
	if recordID == "" {
		return Finding{}, fmt.Errorf("no finding behind this candidate; a change with no observed problem behind it is taste")
	}
	f, err := read[Finding](dir, kindFinding, recordID)
	if err != nil {
		return Finding{}, fmt.Errorf("finding %s: %w", recordID, err)
	}
	return f, nil
}

func readCandidate(dir, recordID string) (Candidate, error) {
	if recordID == "" {
		return Candidate{}, fmt.Errorf("no candidate behind this validation")
	}
	c, err := read[Candidate](dir, kindCandidate, recordID)
	if err != nil {
		return Candidate{}, fmt.Errorf("candidate %s: %w", recordID, err)
	}
	return c, nil
}

func list[T any](dir, kind string) ([]T, error) {
	names, _ := filepath.Glob(filepath.Join(dir, kind, "*.json"))
	slices.Sort(names)
	out := make([]T, 0, len(names))
	for _, name := range names {
		b, err := os.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		var v T
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, fmt.Errorf("%s is not a readable record: %w", name, err)
		}
		out = append(out, v)
	}
	return out, nil
}

func read[T any](dir, kind, recordID string) (T, error) {
	var v T
	b, err := os.ReadFile(filepath.Join(dir, kind, recordID+".json"))
	if err != nil {
		return v, err
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return v, fmt.Errorf("unreadable record: %w", err)
	}
	return v, nil
}

func write(dir, kind, recordID string, v any) error {
	at := filepath.Join(dir, kind)
	if err := os.MkdirAll(at, 0o755); err != nil {
		return fmt.Errorf("prepare %s: %w", at, err)
	}
	// The records are fixed structs of strings, numbers and maps of them, so
	// encoding cannot fail.
	b, _ := json.MarshalIndent(v, "", "  ")
	if err := os.WriteFile(filepath.Join(at, recordID+".json"), append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", recordID, err)
	}
	return nil
}

// id derives a record's name from what it is about.
//
// Derived rather than generated, so the same observation recorded twice is the
// same record. A generated id would turn a re-run of the miner into a second
// copy of every finding it already wrote, and nothing about the pile would say
// they were the same thing.
func id(prefix string, parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return fmt.Sprintf("%s-%x", prefix, sum[:6])
}
