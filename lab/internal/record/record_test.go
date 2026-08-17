package record_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/mine"
	"github.com/luuuc/sense/lab/internal/record"
)

// The clock is passed in rather than read, so "the hypothesis was written before
// the measurement" is a fact a test can set up rather than one it has to wait
// for.
var (
	monday  = time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)
	tuesday = monday.Add(24 * time.Hour)
)

func aFinding() record.Finding {
	return record.Finding{
		Surface:       "blast",
		Detector:      "cited-not-returned",
		Subject:       "g:cloud-signup-validate",
		Detail:        "CloudOrganizationSignUpCommand.cs was cited and never returned",
		Runs:          []string{"campaigns/csharp/bitwarden/1/bench/cell-0/sense", "campaigns/csharp/bitwarden/1/bench/cell-1/sense"},
		Discriminator: true,
	}
}

func recordFinding(t *testing.T, dir string) record.Finding {
	t.Helper()
	f, err := record.RecordFinding(dir, aFinding(), monday)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func recordCandidate(t *testing.T, dir, finding string) record.Candidate {
	t.Helper()
	c, err := record.RecordCandidate(dir, record.Candidate{
		Finding:     finding,
		Hypothesis:  "the resolver drops a file when the dependency is established through a constructor argument",
		TargetCells: []string{"bitwarden-server/claude-opus-5"},
	}, monday)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The route from a number to a fix runs through a surface, so a finding that
// names none is a number again.
func TestAFindingCannotBeRecordedWithoutASurface(t *testing.T) {
	f := aFinding()
	f.Surface = ""
	if _, err := record.RecordFinding(t.TempDir(), f, monday); err == nil {
		t.Fatal("a finding with no surface was recorded")
	}
}

// Evidence is a path into the run tree. A finding citing nothing is an opinion.
func TestAFindingCannotBeRecordedWithoutEvidence(t *testing.T) {
	f := aFinding()
	f.Runs = nil
	if _, err := record.RecordFinding(t.TempDir(), f, monday); err == nil {
		t.Fatal("a finding citing no run was recorded")
	}
}

// A change with no observed problem behind it is taste, and a finding id that
// names nothing on disk is the same thing with a reference attached.
func TestACandidateCannotBeRecordedWithoutAFindingBehindIt(t *testing.T) {
	dir := t.TempDir()
	for _, finding := range []string{"", "f-deadbeefdead"} {
		_, err := record.RecordCandidate(dir, record.Candidate{
			Finding: finding, Hypothesis: "the resolver drops constructor-argument dependencies",
		}, monday)
		if err == nil {
			t.Errorf("a candidate behind finding %q was recorded", finding)
		}
	}
}

func TestACandidateCannotBeRecordedWithoutAHypothesis(t *testing.T) {
	dir := t.TempDir()
	f := recordFinding(t, dir)
	if _, err := record.RecordCandidate(dir, record.Candidate{Finding: f.ID}, monday); err == nil {
		t.Fatal("a candidate with no hypothesis was recorded")
	}
}

// A hypothesis written after the measurement is a description. The whole point
// of writing it first is that it can be wrong.
func TestAValidationCannotPredateItsCandidatesHypothesis(t *testing.T) {
	dir := t.TempDir()
	c := recordCandidate(t, dir, recordFinding(t, dir).ID)

	before := monday.Add(-time.Hour)
	_, err := record.RecordValidation(dir, record.Validation{
		Candidate: c.ID, Regression: "4 of 4 cells held", CorpusSize: 4,
		Decision: record.Accepted, Reason: "the target cell moved from 0.31 to 0.62",
	}, before)
	if err == nil {
		t.Fatal("a validation that ran before its hypothesis was written was recorded")
	}
	if !strings.Contains(err.Error(), "description") {
		t.Errorf("the refusal does not say why it matters: %v", err)
	}
}

// It accepts or it rejects. A third state is a validation that has not finished
// wearing the clothes of one that has.
func TestAValidationDecidesAndSaysWhy(t *testing.T) {
	dir := t.TempDir()
	c := recordCandidate(t, dir, recordFinding(t, dir).ID)

	for _, tc := range []struct{ name, decision, reason string }{
		{"no decision", "", "the cell moved"},
		{"a third state", "needs more work", "the cell moved"},
		{"no reason", record.Rejected, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := record.RecordValidation(dir, record.Validation{
				Candidate: c.ID, Decision: tc.decision, Reason: tc.reason, CorpusSize: 4,
			}, tuesday)
			if err == nil {
				t.Errorf("%s was recorded", tc.name)
			}
		})
	}
}

func TestAValidationCannotBeRecordedWithoutACandidate(t *testing.T) {
	dir := t.TempDir()
	for _, candidate := range []string{"", "c-deadbeefdead"} {
		_, err := record.RecordValidation(dir, record.Validation{
			Candidate: candidate, Decision: record.Accepted, Reason: "it moved",
		}, tuesday)
		if err == nil {
			t.Errorf("a validation behind candidate %q was recorded", candidate)
		}
	}
}

// Invalidating a run is checked through the findings rather than through the
// code that marks them: the property is that a finding pointing at a dead run
// says so, not that a function was called.
func TestInvalidatingARunMarksEveryFindingThatCitesIt(t *testing.T) {
	dir := t.TempDir()
	recordFinding(t, dir)

	other := aFinding()
	other.Subject = "w:price-increase-scheduler"
	other.Runs = []string{"campaigns/csharp/bitwarden/1/bench/cell-0/sense"}
	if _, err := record.RecordFinding(dir, other, monday); err != nil {
		t.Fatal(err)
	}

	elsewhere := aFinding()
	elsewhere.Subject = "g:invite-users-null"
	elsewhere.Runs = []string{"campaigns/rails/mastodon/1/bench/cell-0/sense"}
	if _, err := record.RecordFinding(dir, elsewhere, monday); err != nil {
		t.Fatal(err)
	}

	dead := "campaigns/csharp/bitwarden/1/bench/cell-0/sense"
	touched, err := record.InvalidateRun(dir, dead)
	if err != nil {
		t.Fatal(err)
	}
	if len(touched) != 2 {
		t.Fatalf("marked %d findings, want the 2 that cite the run", len(touched))
	}

	byID := map[string]record.Finding{}
	all, err := record.Findings(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range all {
		byID[f.Subject] = f
	}

	// One of the two still has a live run behind it, so it is marked and stands.
	partly := byID["g:cloud-signup-validate"]
	if len(partly.Invalidated) != 1 || partly.Invalidated[0] != dead {
		t.Errorf("the surviving finding does not name its dead run: %v", partly.Invalidated)
	}
	if !partly.Standing() {
		t.Error("a finding with one live run left was marked dead")
	}
	if len(partly.Runs) != 2 {
		t.Errorf("the dead run was dropped from the evidence: %v; withdrawn support is not less support",
			partly.Runs)
	}

	// The other's only evidence is gone. It is marked, not deleted: it was once
	// observed and that is worth keeping.
	gone := byID["w:price-increase-scheduler"]
	if gone.Standing() {
		t.Error("a finding whose only run was invalidated still stands")
	}
	if _, ok := byID["w:price-increase-scheduler"]; !ok {
		t.Error("a finding whose evidence died was deleted")
	}

	// A finding citing a different run is untouched.
	if len(byID["g:invite-users-null"].Invalidated) != 0 {
		t.Error("a finding citing another run was marked")
	}
}

// Invalidating the same run twice must not double-count it into a false death.
func TestInvalidatingTheSameRunTwiceChangesNothingTheSecondTime(t *testing.T) {
	dir := t.TempDir()
	f := recordFinding(t, dir)
	dead := f.Runs[0]

	if _, err := record.InvalidateRun(dir, dead); err != nil {
		t.Fatal(err)
	}
	again, err := record.InvalidateRun(dir, dead)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("the second invalidation marked %d findings", len(again))
	}
	all, err := record.Findings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(all[0].Invalidated) != 1 || all[0].Dead {
		t.Errorf("invalidating one run twice killed a finding with two: %+v", all[0])
	}
}

// "Why was this change made to Sense" is answerable from the change, through
// its candidate, to its finding, to the transcripts.
func TestTheChainIsQueryableFromEitherEnd(t *testing.T) {
	dir := t.TempDir()
	f := recordFinding(t, dir)
	c := recordCandidate(t, dir, f.ID)
	v, err := record.RecordValidation(dir, record.Validation{
		Candidate:  c.ID,
		Before:     map[string]float64{"bitwarden-server/claude-opus-5": 0.31},
		After:      map[string]float64{"bitwarden-server/claude-opus-5": 0.62},
		Regression: "4 of 4 banked cells held", CorpusSize: 4,
		Decision: record.Accepted, Reason: "the target cell moved +0.31 and no banked cell fell",
	}, tuesday)
	if err != nil {
		t.Fatal(err)
	}

	// Backward: from the change to the transcripts.
	fromChange, err := record.Trace(dir, v.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fromChange.Finding.ID != f.ID {
		t.Errorf("the validation traced to finding %s, want %s", fromChange.Finding.ID, f.ID)
	}
	if got := fromChange.Evidence(); len(got) != 2 {
		t.Errorf("the chain reaches %d run directories, want the finding's 2", len(got))
	}

	// Forward: from the finding to whether anything came of it.
	fromFinding, err := record.Trace(dir, f.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fromFinding.Candidates) != 1 || fromFinding.Candidates[0].ID != c.ID {
		t.Errorf("the finding does not reach its candidate: %+v", fromFinding.Candidates)
	}
	if len(fromFinding.Validations) != 1 || fromFinding.Validations[0].Decision != record.Accepted {
		t.Errorf("the finding does not reach its decision: %+v", fromFinding.Validations)
	}

	// And from the middle.
	fromCandidate, err := record.Trace(dir, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fromCandidate.Finding.ID != f.ID || len(fromCandidate.Validations) != 1 {
		t.Errorf("the candidate does not reach both ends: %+v", fromCandidate)
	}
}

// Most findings never become a candidate. That is the honest state of most
// observations, not a gap to be filled.
func TestAFindingWithNoCandidateTracesCleanly(t *testing.T) {
	dir := t.TempDir()
	f := recordFinding(t, dir)
	chain, err := record.Trace(dir, f.ID)
	if err != nil {
		t.Fatalf("a finding nobody acted on could not be traced: %v", err)
	}
	if len(chain.Candidates) != 0 || len(chain.Validations) != 0 {
		t.Errorf("a finding with no candidate reached %+v", chain)
	}
}

func TestAnIdThatIsNotARecordIsRefused(t *testing.T) {
	dir := t.TempDir()
	for _, bad := range []string{"", "mastodon", "x-1234"} {
		if _, err := record.Trace(dir, bad); err == nil {
			t.Errorf("%q was traced", bad)
		}
	}
}

// The miner's output lands without hand-editing. This is the gap the retired
// tree fell into: miner output in a log file and an empty findings directory.
func TestTheMinersOutputLandsAsFindings(t *testing.T) {
	dir := t.TempDir()
	runs := []string{"campaigns/csharp/bitwarden/1/bench/cell-0/sense", "campaigns/csharp/bitwarden/1/bench/cell-1/sense"}
	found := []mine.Finding{
		{Detector: "cited-not-returned", Surface: mine.Blast, Subject: "g:cloud-signup-validate",
			Detail: "cited and never returned", Runs: 3, Total: 6, Discriminator: true},
		{Detector: "empty-returns", Surface: mine.Graph, Subject: "sense_graph:ClaimsMap",
			Detail: "returned nothing", Runs: 1, Total: 6},
	}

	landed, err := record.FromMine(dir, found, runs, monday)
	if err != nil {
		t.Fatal(err)
	}
	if len(landed) != 2 {
		t.Fatalf("landed %d findings, want 2", len(landed))
	}
	on := map[string]record.Finding{}
	for _, f := range landed {
		on[f.Subject] = f
	}
	if got := on["g:cloud-signup-validate"]; got.Surface != "blast" || !got.Discriminator {
		t.Errorf("the discriminator miss lost its surface or its mark: %+v", got)
	}
	if !strings.Contains(on["g:cloud-signup-validate"].Detail, "3/6") {
		t.Errorf("the finding lost its counts: %q", on["g:cloud-signup-validate"].Detail)
	}
	if got := on["sense_graph:ClaimsMap"]; got.Discriminator {
		t.Error("an empty-return finding was marked as a discriminator miss")
	}
	for _, f := range landed {
		if len(f.Runs) != len(runs) {
			t.Errorf("%s cites %d runs, want the mined set of %d", f.Subject, len(f.Runs), len(runs))
		}
	}
}

// Running the miner again over the same corpus must update what it already
// wrote. A generated id would leave a second copy of every finding with nothing
// in the pile saying they are the same thing.
func TestMiningTwiceDoesNotDuplicateAFinding(t *testing.T) {
	dir := t.TempDir()
	runs := []string{"campaigns/csharp/bitwarden/1/bench/cell-0/sense"}
	found := []mine.Finding{{Detector: "empty-returns", Surface: mine.Graph,
		Subject: "sense_graph:ClaimsMap", Detail: "returned nothing", Runs: 1, Total: 6}}

	if _, err := record.FromMine(dir, found, runs, monday); err != nil {
		t.Fatal(err)
	}
	if _, err := record.FromMine(dir, found, runs, tuesday); err != nil {
		t.Fatal(err)
	}
	all, err := record.Findings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("mining the same corpus twice left %d findings", len(all))
	}
	if all[0].RecordedAt != tuesday.Format(time.RFC3339) {
		t.Errorf("the second run did not update the record: recorded at %s", all[0].RecordedAt)
	}
}

// A miner finding whose surface is empty would land as an unrecordable finding,
// and the failure must be reported rather than half-written.
func TestAMinerFindingWithNoSurfaceIsRefusedRatherThanLandedBlank(t *testing.T) {
	dir := t.TempDir()
	_, err := record.FromMine(dir, []mine.Finding{{Detector: "cited-not-returned", Subject: "g:one"}},
		[]string{"run-1"}, monday)
	if err == nil {
		t.Fatal("a finding with no surface landed")
	}
	all, err := record.Findings(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 0 {
		t.Errorf("a refused batch left %d findings behind", len(all))
	}
}

func TestAnUnreadableRecordIsAnErrorRatherThanAMissingRow(t *testing.T) {
	dir := t.TempDir()
	recordFinding(t, dir)
	if err := writeGarbage(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := record.Findings(dir); err == nil {
		t.Fatal("an unreadable record was silently skipped")
	}
}

// writeGarbage puts an unreadable record beside a good one.
func writeGarbage(dir string) error {
	return os.WriteFile(filepath.Join(dir, "findings", "f-000000000000.json"), []byte("{not json"), 0o644)
}

// A record that cannot be written is a chain link that silently does not exist,
// and the whole value of the chain is that it is complete.
func TestARecordThatCannotBeWrittenIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	if _, err := record.RecordFinding(dir, aFinding(), monday); err == nil {
		t.Fatal("a finding was reported as recorded into a directory that cannot be written")
	}
}

func TestAnUnreadableCandidateOrValidationBreaksTheTraceLoudly(t *testing.T) {
	dir := t.TempDir()
	f := recordFinding(t, dir)
	c := recordCandidate(t, dir, f.ID)
	v, err := record.RecordValidation(dir, record.Validation{
		Candidate: c.ID, Decision: record.Accepted, Reason: "the cell moved", CorpusSize: 4,
	}, tuesday)
	if err != nil {
		t.Fatal(err)
	}

	corrupt(t, dir, "validations", v.ID)
	if _, err := record.Trace(dir, v.ID); err == nil {
		t.Error("a trace through an unreadable validation reported a clean chain")
	}
	if _, err := record.Validations(dir); err == nil {
		t.Error("an unreadable validation was silently skipped")
	}

	corrupt(t, dir, "candidates", c.ID)
	if _, err := record.Trace(dir, c.ID); err == nil {
		t.Error("a trace through an unreadable candidate reported a clean chain")
	}
	if _, err := record.Candidates(dir); err == nil {
		t.Error("an unreadable candidate was silently skipped")
	}
}

// The hypothesis time is what "before the measurement" is compared against. An
// unreadable one is not a pass.
func TestACandidateWithAnUnreadableHypothesisTimeCannotBeValidated(t *testing.T) {
	dir := t.TempDir()
	c := recordCandidate(t, dir, recordFinding(t, dir).ID)
	c.HypothesisAt = "last tuesday"
	writeRaw(t, dir, "candidates", c.ID, c)

	_, err := record.RecordValidation(dir, record.Validation{
		Candidate: c.ID, Decision: record.Accepted, Reason: "the cell moved", CorpusSize: 4,
	}, tuesday)
	if err == nil {
		t.Fatal("a candidate whose hypothesis time cannot be read was validated")
	}
}

func TestInvalidatingARunOverUnreadableFindingsIsAnError(t *testing.T) {
	dir := t.TempDir()
	f := recordFinding(t, dir)
	corrupt(t, dir, "findings", f.ID)
	if _, err := record.InvalidateRun(dir, "campaigns/csharp/bitwarden/1/bench/cell-0/sense"); err == nil {
		t.Fatal("invalidating a run over unreadable findings reported success")
	}
}

func corrupt(t *testing.T, dir, kind, recordID string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, kind, recordID+".json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeRaw(t *testing.T, dir, kind, recordID string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, kind, recordID+".json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// A candidate whose finding has gone from disk is a broken chain, and the trace
// says so rather than returning a chain with a blank head.
func TestATraceThroughAMissingFindingIsAnError(t *testing.T) {
	dir := t.TempDir()
	f := recordFinding(t, dir)
	c := recordCandidate(t, dir, f.ID)
	if err := os.Remove(filepath.Join(dir, "findings", f.ID+".json")); err != nil {
		t.Fatal(err)
	}
	if _, err := record.Trace(dir, c.ID); err == nil {
		t.Fatal("a candidate whose finding is gone traced cleanly")
	}
}

func TestARecordFileThatCannotBeOpenedIsAnError(t *testing.T) {
	dir := t.TempDir()
	f := recordFinding(t, dir)
	at := filepath.Join(dir, "findings", f.ID+".json")
	if err := os.Chmod(at, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(at, 0o644) })

	if _, err := record.Findings(dir); err == nil {
		t.Fatal("a record that could not be opened was skipped")
	}
}
