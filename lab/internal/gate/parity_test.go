package gate_test

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/luuuc/sense/lab/internal/gate"
)

// Gate parity: every recorded pay decision, replayed through these gates, comes
// out the same. A gate that would have paid where the old loop refused — or
// refused where it paid — is a bug in the new gate until proven otherwise.
//
// What the record can answer is what is replayed. `banked.jsonl` holds the
// cells that were paid for and published, with their per-group recalls and
// their replicate counts, so the three arithmetic gates can be replayed exactly.
// The mini-bench, validation and retry gates are process facts that the banked
// record does not carry; they are supplied as satisfied here, and that is said
// out loud rather than left to be inferred from a green test.

// bankedCell is one recorded, paid-for, published decision.
type bankedCell struct {
	Repo   string `json:"repo"`
	Model  string `json:"model"`
	Groups map[string]struct {
		// Baseline and Sense are per-run [reached, total] pairs.
		Baseline     [][2]int `json:"baseline"`
		Sense        [][2]int `json:"sense"`
		BaselineMean float64  `json:"baseline_mean"`
		Delta        float64  `json:"delta"`
	} `json:"groups"`
}

// discriminator is the group carrying the margin: the one with the largest
// delta, which is what `best_group_delta` records.
func (c bankedCell) discriminator() string {
	best, bestDelta := "", -1.0
	for name, g := range c.Groups {
		if g.Delta > bestDelta || (g.Delta == bestDelta && name < best) {
			best, bestDelta = name, g.Delta
		}
	}
	return best
}

func bankedCells(t *testing.T) []bankedCell {
	t.Helper()
	var cells []bankedCell
	for _, name := range []string{"banked-ruby-rails.jsonl", "banked-csharp-aspnet.jsonl"} {
		f, err := os.Open(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		s := bufio.NewScanner(f)
		s.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for s.Scan() {
			if strings.TrimSpace(s.Text()) == "" {
				continue
			}
			var c bankedCell
			if err := json.Unmarshal(s.Bytes(), &c); err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			cells = append(cells, c)
		}
		if err := s.Err(); err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		_ = f.Close()
	}
	if len(cells) == 0 {
		// An empty corpus makes parity pass by having nothing to disagree with,
		// which is the shape of a proof that proves nothing.
		t.Fatal("no banked cells were read; parity over an empty corpus proves nothing")
	}
	return cells
}

// decisionFor replays one banked cell as the gates would be asked about it.
func decisionFor(c bankedCell) gate.Decision {
	g := c.Groups[c.discriminator()]
	// The worst recorded baseline replicate, which is the one a gate must not
	// be fooled by: an arm that assembled the set once has assembled it.
	reached, total := 0, 0
	for _, run := range g.Baseline {
		if run[0] > reached {
			reached = run[0]
		}
		total = run[1]
	}
	return gate.Decision{
		BaselineReached: reached, GroupTotal: total,
		BaselineRecall: g.BaselineMean, BaselineRecorded: true,
		SenseRuns: len(g.Sense), BaselineRuns: len(g.Baseline),
		// Not on the banked record. Supplied as satisfied, because these cells
		// were paid for under a law that required all three.
		MiniBenchRan: true,
		Validation:   gate.Validation{Ran: true, Wall: 8 * time.Minute, RealWall: 8 * time.Minute},
		Retries:      0,
	}
}

func TestEveryRecordedPayDecisionIsStillPaidFor(t *testing.T) {
	cells := bankedCells(t)

	for _, c := range cells {
		if refused := gate.Refusals(decisionFor(c)); len(refused) != 0 {
			t.Errorf("%s/%s was banked and published, and the new gates refuse it: %v", c.Repo, c.Model, refused)
		}
	}
	t.Logf("gate parity: %d recorded pay decisions replayed, all still paid for", len(cells))
}

func TestTheGatesAreScopedToTheDiscriminatorGroupAndTheCorpusSaysWhy(t *testing.T) {
	// This is the parity finding, and it is the reason the scoping is not a
	// detail. Banked cells carry groups whose baseline sits at or above the
	// ceiling — one is at 1.00, the baseline having assembled that group
	// entirely — while the discriminator group is well below it. A gate applied
	// to any group would refuse cells that were paid for and published.
	cells := bankedCells(t)

	refusedByAnyGroup := 0
	for _, c := range cells {
		for name, g := range c.Groups {
			if name == c.discriminator() {
				continue
			}
			reached, total := 0, 0
			for _, run := range g.Baseline {
				if run[0] > reached {
					reached = run[0]
				}
				total = run[1]
			}
			if gate.BaselineAssemblesTheSet(reached, total) != nil ||
				gate.ArithmeticCeiling(g.BaselineMean, true) != nil {
				refusedByAnyGroup++
				break
			}
		}
	}

	if refusedByAnyGroup == 0 {
		t.Fatal("no banked cell has a non-discriminator group above the gates; the scoping is untested by this corpus")
	}
	t.Logf("gates applied to any group rather than the discriminator would refuse %d of %d banked cells",
		refusedByAnyGroup, len(cells))
}

func TestEveryBankedCellHasTheReplicatesAPublishedCellNeeds(t *testing.T) {
	// The single-run gate replayed on its own, so a parity pass cannot be
	// carried by the other five.
	for _, c := range bankedCells(t) {
		g := c.Groups[c.discriminator()]
		if err := gate.SingleRunCell(len(g.Sense), len(g.Baseline)); err != nil {
			t.Errorf("%s was published with %v", c.Repo, err)
		}
	}
}

func TestEveryBankedCellsBaselineLeftRoomForTheBar(t *testing.T) {
	// The arithmetic ceiling replayed on its own. A banked win whose recorded
	// baseline was above the ceiling would be a contradiction on the record,
	// and finding one would be a finding rather than a test failure to route
	// around.
	for _, c := range bankedCells(t) {
		g := c.Groups[c.discriminator()]
		if err := gate.ArithmeticCeiling(g.BaselineMean, true); err != nil {
			t.Errorf("%s was banked as a win although %v", c.Repo, err)
		}
	}
}
