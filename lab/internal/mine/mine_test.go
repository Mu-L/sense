package mine_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/mine"
)

// The fixtures are six real recorded runs against bitwarden-server, five models
// between them, taken from the run whose misses were found by hand. Nothing
// here is synthetic: a detector with no real example does not ship, and a
// fixture written to make a detector fire is the same failure one step earlier.
var recorded = []string{"opus-run-1", "opus-run-2", "glm-run-1", "glm-run-2", "glm-run-3", "gpt-run-2"}

// goldRow is the scenario's authored gold: the id, its group, and the path
// fragment that identifies its file.
type goldRow struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	Match string `json:"match"`
}

// citedRow is what the scorer credited in one run.
type citedRow struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	Cited bool   `json:"cited"`
}

// discriminators are the groups that carry the margin on this scenario. Gold in
// any other group is reached by both arms and does not decide the cell.
var discriminators = map[string]bool{"guards": true, "write-path": true, "dependents": true}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// corpus builds the completed runs the miner is handed, exactly the way the
// binary would: calls from the recorded capture, cited rows from the scorer.
func corpus(t *testing.T) []mine.Completed {
	t.Helper()
	var gold []goldRow
	if err := json.Unmarshal(readFixture(t, "bitwarden-server.gold.json"), &gold); err != nil {
		t.Fatal(err)
	}
	byID := map[string]goldRow{}
	for _, g := range gold {
		byID[g.ID] = g
	}

	var runs []mine.Completed
	for _, name := range recorded {
		var credited []citedRow
		if err := json.Unmarshal(readFixture(t, name+".cited.json"), &credited); err != nil {
			t.Fatal(err)
		}
		var cited []mine.Cited
		for _, c := range credited {
			g, ok := byID[c.ID]
			if !ok {
				continue
			}
			cited = append(cited, mine.Cited{
				ID: c.ID, Group: g.Group, Path: g.Match, Discriminator: discriminators[g.Group],
			})
		}
		run, err := mine.Complete(name, "completed", mine.Capture(readFixture(t, name+".sense-io.jsonl")), cited)
		if err != nil {
			t.Fatal(err)
		}
		runs = append(runs, run)
	}
	return runs
}

func subjects(findings []mine.Finding, detector string) map[string]mine.Finding {
	out := map[string]mine.Finding{}
	for _, f := range findings {
		if f.Detector == detector {
			out[f.Subject] = f
		}
	}
	return out
}

// The done-means of the whole pitch: run over the recorded corpus, the miner
// finds the misses that were found by hand, by name.
func TestTheMinerReproducesTheMissesFoundByHand(t *testing.T) {
	found := subjects(mine.Findings(corpus(t)), "cited-not-returned")
	for _, id := range []string{"g:cloud-signup-validate", "g:restart-subscription", "w:price-increase-scheduler"} {
		f, ok := found[id]
		if !ok {
			t.Errorf("the miner did not find %s, which was found by hand on this corpus", id)
			continue
		}
		if f.Runs < 1 || f.Runs > f.Total {
			t.Errorf("%s: %d of %d runs is not a count", id, f.Runs, f.Total)
		}
		if f.Total != len(recorded) {
			t.Errorf("%s reports a denominator of %d over %d runs", id, f.Total, len(recorded))
		}
	}
}

// A miss on the group that carries the margin is worth more than a miss
// elsewhere, and the output has to say which is which.
func TestDiscriminatorMissesAreMarkedApartFromTheRest(t *testing.T) {
	found := subjects(mine.Findings(corpus(t)), "cited-not-returned")
	guard, ok := found["g:cloud-signup-validate"]
	if !ok {
		t.Fatal("the guards miss is absent")
	}
	if !guard.Discriminator {
		t.Error("a miss in the guards group is not marked as a discriminator miss")
	}
	if !strings.HasPrefix(guard.String(), "*") {
		t.Errorf("the marked miss does not read as marked: %q", guard.String())
	}

	contract, ok := found["c:startup-wiring"]
	if !ok {
		t.Fatal("the contract-group miss is absent, so the marking cannot be told apart from marking everything")
	}
	if contract.Discriminator {
		t.Error("a miss in the contract group is marked as a discriminator miss")
	}
}

// The recorded output names this one: sense_blast:Permissions returned 46 files
// on one run and 50 on another. The detector's job is to see a symbol answered
// two different ways.
func TestTheSameSymbolAnsweredTwoDifferentWaysIsFound(t *testing.T) {
	found := mine.Nondeterministic(corpus(t))
	if len(found) == 0 {
		t.Fatal("no symbol was answered with different file counts across six recorded runs")
	}
	for _, f := range found {
		if !strings.Contains(f.Detail, "across runs") {
			t.Errorf("%s does not say the counts differ across runs: %q", f.Subject, f.Detail)
		}
		if f.Runs < 2 {
			t.Errorf("%s claims nondeterminism from %d distinct counts", f.Subject, f.Runs)
		}
	}
}

func TestAResolverCallThatReturnedNothingIsFound(t *testing.T) {
	found := mine.EmptyReturns(corpus(t))
	if len(found) == 0 {
		t.Fatal("no empty resolver return in six recorded runs")
	}
	for _, f := range found {
		if !strings.Contains(f.Detail, "returned nothing") {
			t.Errorf("%s does not say what happened: %q", f.Subject, f.Detail)
		}
	}
}

// Every finding names a surface. A finding with no surface is a number again,
// and the route from a number to a fix runs through a surface.
func TestEveryFindingNamesASurface(t *testing.T) {
	known := map[mine.Surface]bool{}
	for _, s := range mine.AllSurfaces() {
		known[s] = true
	}
	findings := mine.Findings(corpus(t))
	if len(findings) == 0 {
		t.Fatal("the corpus produced no findings at all")
	}
	for _, f := range findings {
		if !known[f.Surface] {
			t.Errorf("%s names surface %q, which is not one Sense has", f.Subject, f.Surface)
		}
	}
}

// A finding with no surface is unrepresentable rather than discouraged: a call
// to a tool that is not a Sense surface cannot produce one.
func TestACallToSomethingThatIsNotASenseSurfaceProducesNoFinding(t *testing.T) {
	run, err := mine.Complete("r1", "completed",
		[]mine.Call{{Tool: "Bash", Key: "grep -rn Plan", Args: `{"cmd":"grep"}`}},
		[]mine.Cited{{ID: "g:one", Group: "guards", Path: "Guard.cs", Discriminator: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got := mine.Findings([]mine.Completed{run}); len(got) != 0 {
		t.Errorf("a non-Sense tool produced %v", got)
	}
}

// The agent choosing not to call a resolver is not a resolver defect. Counting
// that run would report a product finding where there is only a route.
func TestARunWhereNoResolverWasCalledIsNotAMiss(t *testing.T) {
	run, err := mine.Complete("r1", "completed",
		[]mine.Call{{Tool: "sense_search", Key: "plan pricing", Args: `{"query":"plan pricing"}`}},
		[]mine.Cited{{ID: "g:one", Group: "guards", Path: "Guard.cs", Discriminator: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got := mine.CitedNotReturned([]mine.Completed{run}); len(got) != 0 {
		t.Errorf("a run that never asked a resolver produced %v", got)
	}
}

func TestAFileTheResolverReturnedIsNotAMiss(t *testing.T) {
	run, err := mine.Complete("r1", "completed",
		[]mine.Call{{Tool: "sense_blast", Key: "Plan", Args: `{"symbol":"Plan"}`,
			Returned: []string{"src/Core/Guard.cs"}}},
		[]mine.Cited{{ID: "g:one", Group: "guards", Path: "Guard.cs", Discriminator: true}})
	if err != nil {
		t.Fatal(err)
	}
	if got := mine.CitedNotReturned([]mine.Completed{run}); len(got) != 0 {
		t.Errorf("a file the resolver returned was reported as a miss: %v", got)
	}
}

// Detection is post-run, and it is the type that says so. A run with no terminal
// state has no outcome to hand over, because reading such a directory refuses.
func TestARunWithNoTerminalStateCannotBeMined(t *testing.T) {
	if _, err := mine.Complete("r1", "", nil, nil); err == nil {
		t.Fatal("a run with no terminal state was accepted for mining")
	}
	if _, err := mine.Complete("", "completed", nil, nil); err == nil {
		t.Fatal("a run with no id was accepted")
	}

	// And the zero value carries nothing, so it cannot be smuggled past the
	// constructor by declaring one.
	var unbuilt mine.Completed
	if got := mine.Findings([]mine.Completed{unbuilt}); len(got) != 0 {
		t.Errorf("a run nobody completed produced %v", got)
	}
}

// The miner reports and does not interpret. There is nowhere on a finding to put
// a score, and nothing orders them by one.
func TestAFindingCarriesItsCountsAndNoScore(t *testing.T) {
	for _, f := range mine.Findings(corpus(t)) {
		if f.Total == 0 {
			t.Errorf("%s reports %d runs out of nothing; a numerator without its denominator is not a finding",
				f.Subject, f.Runs)
		}
		if !strings.Contains(f.String(), "runs") {
			t.Errorf("%s does not carry its run count: %q", f.Subject, f.String())
		}
	}
}
