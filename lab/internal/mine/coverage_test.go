package mine_test

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/mine"
)

func exercise(t *testing.T, runs []mine.Completed) map[mine.Surface]mine.Exercise {
	t.Helper()
	out := map[mine.Surface]mine.Exercise{}
	for _, e := range mine.Coverage(runs) {
		out[e.Surface] = e
	}
	return out
}

// An unexercised surface is an unimproved surface, and a row that is missing
// reads as a row that is fine. Every surface appears, zeros included.
func TestEverySurfaceAppearsIncludingTheOnesNobodyCalled(t *testing.T) {
	got := exercise(t, corpus(t))
	for _, s := range mine.AllSurfaces() {
		if _, ok := got[s]; !ok {
			t.Errorf("surface %s is missing from the coverage report", s)
		}
	}
	if got[mine.Blast].Calls == 0 || got[mine.Graph].Calls == 0 {
		t.Errorf("the resolvers show no calls over six recorded runs: blast %d, graph %d",
			got[mine.Blast].Calls, got[mine.Graph].Calls)
	}
	// The meta-surfaces are where misuse is born and no tool call reaches them,
	// so they are the report's honest zeros rather than an omission.
	if got[mine.Setup].Calls != 0 {
		t.Errorf("a tool call was attributed to the setup surface: %d", got[mine.Setup].Calls)
	}
}

// The report says what was exercised and with what, so an option nobody ever
// passed is visible rather than merely absent from a count.
func TestTheReportCarriesTheParametersASurfaceWasCalledWith(t *testing.T) {
	got := exercise(t, corpus(t))
	if len(got[mine.Blast].Params) == 0 {
		t.Fatal("blast was called and the report names no arguments")
	}
	var sawHops bool
	for _, p := range got[mine.Blast].Params {
		if strings.Contains(p, "max_hops") {
			sawHops = true
		}
	}
	if !sawHops {
		t.Error("no recorded blast call carries max_hops; the parameter list is not what was sent")
	}
}

// Runs and calls are different questions. One run calling blast twelve times is
// not twelve runs exercising it, and a report that conflated them would say a
// surface is well covered when one session leaned on it.
func TestCallsAndRunsAreCountedSeparately(t *testing.T) {
	one, err := mine.Complete("r1", "completed", []mine.Call{
		{Tool: "sense_blast", Key: "A", Args: `{"symbol":"A"}`},
		{Tool: "sense_blast", Key: "B", Args: `{"symbol":"B"}`},
		{Tool: "sense_blast", Key: "C", Args: `{"symbol":"C"}`},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := exercise(t, []mine.Completed{one})
	if got[mine.Blast].Calls != 3 {
		t.Errorf("%d calls, want 3", got[mine.Blast].Calls)
	}
	if got[mine.Blast].Runs != 1 {
		t.Errorf("%d runs, want 1: three calls in one session is one run", got[mine.Blast].Runs)
	}
}

// The coverage report and the detectors share their input and nothing else. A
// report derived from what the detectors inspected would answer "what did we
// look at" while appearing to answer "what was exercised", and the gap only
// shows up as a surface nobody improves.
func TestCoverageIsUnchangedByWhetherAnythingWasFound(t *testing.T) {
	// A run whose every cited row was returned produces no finding at all, and
	// its surfaces were exercised exactly as hard either way.
	clean, err := mine.Complete("clean", "completed",
		[]mine.Call{{Tool: "sense_blast", Key: "Plan", Args: `{"symbol":"Plan"}`,
			Returned: []string{"src/Core/Guard.cs"}}},
		[]mine.Cited{{ID: "g:one", Group: "guards", Path: "Guard.cs", Discriminator: true}})
	if err != nil {
		t.Fatal(err)
	}
	if found := mine.Findings([]mine.Completed{clean}); len(found) != 0 {
		t.Fatalf("the clean run produced findings, so this proves nothing: %v", found)
	}
	if got := exercise(t, []mine.Completed{clean}); got[mine.Blast].Calls != 1 || got[mine.Blast].Runs != 1 {
		t.Errorf("a run with no findings shows blast exercised %d times in %d runs, want 1 and 1",
			got[mine.Blast].Calls, got[mine.Blast].Runs)
	}
}

func TestACampaignWithNoRunsExercisedNothing(t *testing.T) {
	for _, e := range mine.Coverage(nil) {
		if e.Calls != 0 || e.Runs != 0 || len(e.Params) != 0 {
			t.Errorf("%s reports %+v with no runs", e.Surface, e)
		}
	}
}
