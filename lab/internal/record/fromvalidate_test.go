package record_test

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/record"
	"github.com/luuuc/sense/lab/internal/validate"
)

func measured(targetAfter, bankedAfter float64) validate.Candidate {
	return validate.Candidate{
		Surface: "blast",
		Targets: []validate.Target{
			{Cell: "aspnetcore", Model: "claude-opus-5", Before: 0.310, After: targetAfter, Predict: validate.Up},
		},
		Regressions: []validate.Regression{
			{Banked: validate.Banked{Cell: "bitwarden-server", Model: "claude-opus-5",
				Margin: 0.625, Spread: 0.077, Surfaces: []string{"blast"}}, After: bankedAfter},
			{Banked: validate.Banked{Cell: "mastodon", Model: "claude-opus-5",
				Margin: 0.530, Spread: 0.125, Surfaces: []string{"blast"}}, After: 0.530},
		},
	}
}

// The whole chain, after the fact: a finding, the candidate it justified, the
// measurement, and the decision, reachable from either end.
func TestAValidationLandsOnTheChainAndIsTraceableBothWays(t *testing.T) {
	dir := t.TempDir()
	f := recordFinding(t, dir)
	c := recordCandidate(t, dir, f.ID)

	o, err := validate.Run(measured(0.620, 0.625))
	if err != nil {
		t.Fatal(err)
	}
	v, err := record.FromValidate(dir, c.ID, o, tuesday)
	if err != nil {
		t.Fatal(err)
	}
	if v.Decision != validate.Accepted {
		t.Fatalf("a clean candidate landed as %s", v.Decision)
	}
	if v.CorpusSize != 2 {
		t.Errorf("the record lost the corpus size: %d", v.CorpusSize)
	}
	if got := v.Before["aspnetcore/claude-opus-5"]; got != 0.310 {
		t.Errorf("the record lost the before figure: %v", v.Before)
	}
	if got := v.After["aspnetcore/claude-opus-5"]; got != 0.620 {
		t.Errorf("the record lost the after figure: %v", v.After)
	}

	chain, err := record.Trace(dir, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if chain.Finding.ID != f.ID {
		t.Errorf("the decision does not trace back to its finding")
	}
	if len(chain.Evidence()) == 0 {
		t.Error("the decision does not reach the transcripts behind it")
	}
}

// A rejection is a result. The next person with the same idea should find the
// measurement rather than repeat it, and a path that writes down only its
// successes invites the same experiment twice.
func TestARejectionIsRecordedWithItsNumbers(t *testing.T) {
	dir := t.TempDir()
	c := recordCandidate(t, dir, recordFinding(t, dir).ID)

	o, err := validate.Run(measured(0.620, 0.400))
	if err != nil {
		t.Fatal(err)
	}
	v, err := record.FromValidate(dir, c.ID, o, tuesday)
	if err != nil {
		t.Fatal(err)
	}
	if v.Decision != validate.Rejected {
		t.Fatalf("a candidate that broke a banked cell landed as %s", v.Decision)
	}
	for _, want := range []string{"bitwarden-server", "0.625", "0.400", "1 of 2 banked cells held"} {
		if !strings.Contains(v.Reason, want) {
			t.Errorf("the recorded rejection does not carry %q: %q", want, v.Reason)
		}
	}

	// And it is findable by someone coming to the same idea later.
	chain, err := record.Trace(dir, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chain.Validations) != 1 || chain.Validations[0].Decision != validate.Rejected {
		t.Errorf("the rejection is not on the candidate's chain: %+v", chain.Validations)
	}
}

// The single-model label and the blind spot are properties of the validation and
// have to survive being written down.
func TestTheSingleModelLabelAndTheBlindSpotSurviveOntoTheRecord(t *testing.T) {
	dir := t.TempDir()
	c := recordCandidate(t, dir, recordFinding(t, dir).ID)

	blind := measured(0.620, 0.625)
	blind.Surface = "conventions"
	o, err := validate.Run(blind)
	if err != nil {
		t.Fatal(err)
	}
	v, err := record.FromValidate(dir, c.ID, o, tuesday)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"single-model", "none of the changed surface"} {
		if !strings.Contains(v.Regression, want) {
			t.Errorf("the record lost %q: %q", want, v.Regression)
		}
	}
}
