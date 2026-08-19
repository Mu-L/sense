package replay_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/replay"
)

// The fixture is the real banked corpus: five cells across two repositories, with
// each cell's margin and its own run-to-run spread derived from the runs it was
// banked on.
func banked(t *testing.T) []replay.Cell {
	t.Helper()
	b, err := os.ReadFile("testdata/banked.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Repo     string  `json:"repo"`
		Model    string  `json:"model"`
		Margin   float64 `json:"margin"`
		Spread   float64 `json:"spread"`
		Scenario string  `json:"scenario"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	out := make([]replay.Cell, len(rows))
	for i, r := range rows {
		out[i] = replay.Cell{Repo: r.Repo, Model: r.Model, Margin: r.Margin, Spread: r.Spread, Scenario: r.Scenario}
	}
	return out
}

const opus = "claude-opus-5"

// A cell that banked near the bar cannot tell a broken instrument from ordinary
// variance, so the corpus picks its own cell and it is not the highest margin.
func TestThePickIsTheCellWhoseWorstRunIsFurthestClearOfTheBar(t *testing.T) {
	got, skipped, err := replay.Pick(banked(t), []string{opus})
	if err != nil {
		t.Fatal(err)
	}
	if got.Repo != "mastodon" {
		t.Fatalf("picked %s, want mastodon: banked 0.775 with a spread of 0.050 is the only cell comfortably clear of the bar", got.Repo)
	}
	if len(skipped) != 4 {
		t.Fatalf("skipped %d cells, want the other 4 reported: %v", len(skipped), skipped)
	}

	// discourse banked HIGHER than the bar and wider than mastodon, so a
	// highest-margin pick would have taken it and could not have told a broken
	// instrument from noise.
	why := map[string]string{}
	for _, s := range skipped {
		why[s.Cell.Repo] = s.Why
	}
	if !strings.Contains(why["discourse"], "ordinary variance") {
		t.Errorf("discourse was skipped for the wrong reason: %q", why["discourse"])
	}
	if !strings.Contains(why["chatwoot"], "ordinary variance") {
		t.Errorf("chatwoot was skipped for the wrong reason: %q", why["chatwoot"])
	}
}

// A cell exactly at the bar is not comfortably clear of it. bitwarden-server
// banked 0.750 with a spread of 0.250, so its worst run lands exactly on 0.50.
func TestACellWhoseWorstRunLandsOnTheBarIsNotUnambiguous(t *testing.T) {
	for _, c := range banked(t) {
		if c.Repo != "bitwarden-server" {
			continue
		}
		if c.Unambiguous() {
			t.Errorf("%s is treated as unambiguous, and its worst run is exactly the bar", c)
		}
		return
	}
	t.Fatal("bitwarden-server is not in the corpus")
}

// The trap: a banked number is a number for a model. Replayed on the wrong one,
// a cell looks like a broken instrument while being a working one measuring a
// changed world.
func TestAnUnreachableModelPicksADifferentCellNeverADifferentModel(t *testing.T) {
	corpus := append(banked(t), replay.Cell{
		Repo: "aspnetcore", Model: "claude-opus-4-7", Margin: 0.900, Spread: 0.050,
	})

	// The retired model's cell is the best on paper. It must not be picked, and
	// no other model may be substituted for it.
	got, skipped, err := replay.Pick(corpus, []string{opus})
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != opus {
		t.Fatalf("picked a cell on %s, and only %s is reachable", got.Model, opus)
	}
	if got.Repo != "mastodon" {
		t.Errorf("picked %s; the best reachable cell is mastodon", got.Repo)
	}

	var reported bool
	for _, s := range skipped {
		if s.Cell.Repo == "aspnetcore" {
			reported = true
			if !strings.Contains(s.Why, "number for a model") {
				t.Errorf("the unreplayable cell does not say why: %q", s.Why)
			}
		}
	}
	if !reported {
		t.Error("a cell that can no longer be replayed was dropped rather than recorded")
	}
}

// A corpus with nothing replayable is an error rather than a pick nobody
// checked, because the alternative is picking a different model.
func TestACorpusWithNothingReplayableRefusesRatherThanSubstituting(t *testing.T) {
	_, skipped, err := replay.Pick(banked(t), []string{"gpt-5.6-sol"})
	if err == nil {
		t.Fatal("a cell was picked with no reachable model")
	}
	if len(skipped) != 5 {
		t.Errorf("reported %d unreplayable cells, want all 5", len(skipped))
	}
	if _, _, err := replay.Pick(nil, []string{opus}); err == nil {
		t.Fatal("an empty corpus produced a pick")
	}
}

// Agreement is a band. The measured same-cell spreads run from 0.077 to 0.250,
// so demanding the banked number exactly would fail on the instrument's own
// noise.
func TestAgreementIsTheCellsOwnSpreadInBothDirections(t *testing.T) {
	c := replay.Cell{Repo: "mastodon", Model: opus, Margin: 0.775, Spread: 0.050}
	for _, tc := range []struct {
		measured float64
		agrees   bool
		why      string
	}{
		{0.775, true, "the banked margin itself"},
		{0.725, true, "the bottom of the band"},
		{0.825, true, "the top of the band"},
		{0.700, false, "below the band"},
		{0.900, false, "above the band: a measurement that overshoots is not agreement"},
	} {
		if got := replay.Compare(c, tc.measured); got.Agrees != tc.agrees {
			t.Errorf("%.3f (%s): agrees=%v, want %v", tc.measured, tc.why, got.Agrees, tc.agrees)
		}
	}
}

// Landing outside the recorded band and clearing the bar are different facts,
// and a replay that reported only one of them would hide the other.
func TestDisagreeingAndStillWinningAreSeparateFacts(t *testing.T) {
	c := replay.Cell{Repo: "mastodon", Model: opus, Margin: 0.775, Spread: 0.050}

	high := replay.Compare(c, 0.950)
	if high.Agrees || !high.StillAWin {
		t.Errorf("0.950 against 0.775 ±0.050: agrees=%v win=%v, want a disagreement that is still a win",
			high.Agrees, high.StillAWin)
	}
	low := replay.Compare(c, 0.200)
	if low.Agrees || low.StillAWin {
		t.Errorf("0.200: agrees=%v win=%v, want neither", low.Agrees, low.StillAWin)
	}
	if !strings.Contains(low.String(), "DISAGREES") {
		t.Errorf("a disagreement does not read as one: %q", low.String())
	}
}

// The ordering is the whole discipline: the cell's answer is known, so a
// disagreement is a bug here until something else is demonstrated. And it needs
// a stopping point, or it is a licence to debug indefinitely.
func TestADisagreementWalksInstrumentThenEnvironmentThenASecondPairThenTheWorld(t *testing.T) {
	c := replay.Cell{Repo: "mastodon", Model: opus, Margin: 0.775, Spread: 0.050}
	v := replay.Compare(c, 0.200)

	walk := []struct {
		done replay.Checks
		want replay.Step
	}{
		{replay.Checks{}, replay.SuspectTheInstrument},
		{replay.Checks{InstrumentClean: true}, replay.SuspectTheEnvironment},
		{replay.Checks{InstrumentClean: true, EnvironmentClean: true, Pairs: 1}, replay.RunASecondPair},
		{replay.Checks{InstrumentClean: true, EnvironmentClean: true, Pairs: 2}, replay.TheWorldMoved},
	}
	for _, step := range walk {
		if got := replay.Investigate(v, step.done); got != step.want {
			t.Errorf("with %+v the next step is %q, want %q", step.done, got, step.want)
		}
	}
}

// The world is the last answer, never the first. Reaching it with the
// instrument unchecked is exactly the reverse ordering the discipline forbids.
func TestTheWorldCannotBeBlamedBeforeTheInstrumentAndTheEnvironment(t *testing.T) {
	c := replay.Cell{Repo: "mastodon", Model: opus, Margin: 0.775, Spread: 0.050}
	v := replay.Compare(c, 0.200)

	for _, done := range []replay.Checks{
		{Pairs: 5},
		{EnvironmentClean: true, Pairs: 5},
		{InstrumentClean: true, Pairs: 5},
	} {
		if got := replay.Investigate(v, done); got == replay.TheWorldMoved {
			t.Errorf("with %+v the world was blamed before the instrument and the environment were ruled out", done)
		}
	}
}

// One pair is n=1 against a spread that reaches 0.250, so a single disagreement
// is inside ordinary variance.
func TestOnePairIsNeverEnoughToConcludeAnything(t *testing.T) {
	c := replay.Cell{Repo: "mastodon", Model: opus, Margin: 0.775, Spread: 0.050}
	v := replay.Compare(c, 0.200)
	clean := replay.Checks{InstrumentClean: true, EnvironmentClean: true, Pairs: 1}
	if got := replay.Investigate(v, clean); got != replay.RunASecondPair {
		t.Errorf("after one clean pair the next step is %q, want a second pair", got)
	}
}

func TestAnAgreementHasNothingToInvestigate(t *testing.T) {
	c := replay.Cell{Repo: "mastodon", Model: opus, Margin: 0.775, Spread: 0.050}
	v := replay.Compare(c, 0.760)
	if got := replay.Investigate(v, replay.Checks{}); got != replay.Settled {
		t.Errorf("an agreement produced %q", got)
	}
}

// "Nothing was adjusted to make it land" is a claim, and a replay tuned until it
// agrees has proven that tuning works. This is how the claim is checked.
func TestEveryPinnedInputThatMovedIsNamed(t *testing.T) {
	was := replay.Inputs{
		Model: opus, Scenario: "sha256:27dfc6000a5e98f3", Wall: "1800s",
		Prompt: "sha256:aaa", Gold: "sha256:bbb", Subject: "sense-main",
	}
	if pinned, why := replay.Pinned(was, was); !pinned {
		t.Fatalf("an unchanged replay was reported as adjusted: %s", why)
	} else if !strings.Contains(why, "every pinned input matches") {
		t.Errorf("the clean case does not say so: %q", why)
	}

	live := was
	live.Wall = "2700s"
	live.Gold = "sha256:ccc"
	pinned, why := replay.Pinned(was, live)
	if pinned {
		t.Fatal("a replay with a longer wall and different gold was reported as pinned")
	}
	for _, want := range []string{"wall", "1800s", "2700s", "gold", "not a replay"} {
		if !strings.Contains(why, want) {
			t.Errorf("the drift report does not carry %q: %q", want, why)
		}
	}
	if got := replay.Drift(was, live); len(got) != 2 {
		t.Errorf("named %d drifted inputs, want both: %v", len(got), got)
	}
}

// Each pinned input is checked. One that was never compared is one a replay
// could quietly change.
func TestEveryPinnedInputIsActuallyCompared(t *testing.T) {
	base := replay.Inputs{
		Model: "a", Scenario: "a", Wall: "a", Prompt: "a", Gold: "a", Subject: "a",
	}
	changes := []struct {
		name  string
		apply func(*replay.Inputs)
	}{
		{"model", func(i *replay.Inputs) { i.Model = "b" }},
		{"scenario", func(i *replay.Inputs) { i.Scenario = "b" }},
		{"wall", func(i *replay.Inputs) { i.Wall = "b" }},
		{"prompt", func(i *replay.Inputs) { i.Prompt = "b" }},
		{"gold", func(i *replay.Inputs) { i.Gold = "b" }},
		{"subject", func(i *replay.Inputs) { i.Subject = "b" }},
	}
	for _, c := range changes {
		live := base
		c.apply(&live)
		drift := replay.Drift(base, live)
		if len(drift) != 1 || !strings.Contains(drift[0], c.name) {
			t.Errorf("changing the %s produced %v", c.name, drift)
		}
	}
}

// Every cell the pick did not take is reported readably, whether it lost on
// reach, on ambiguity, or simply to a better cell.
func TestASecondUnambiguousCellLosesToTheBetterOneAndSaysSo(t *testing.T) {
	corpus := []replay.Cell{
		{Repo: "mastodon", Model: opus, Margin: 0.775, Spread: 0.050, Scenario: "sha256:27dfc600"},
		{Repo: "aspnetcore", Model: opus, Margin: 0.700, Spread: 0.050, Scenario: "sha256:aaaa"},
		{Repo: "rails", Model: opus, Margin: 0.556, Spread: 0.222, Scenario: "sha256:3f210bcd"},
	}
	got, skipped, err := replay.Pick(corpus, []string{opus})
	if err != nil {
		t.Fatal(err)
	}
	if got.Repo != "mastodon" {
		t.Fatalf("picked %s, want the cell furthest clear of the bar", got.Repo)
	}
	if len(skipped) != 2 {
		t.Fatalf("skipped %d, want 2: %v", len(skipped), skipped)
	}

	why := map[string]string{}
	for _, s := range skipped {
		why[s.Cell.Repo] = s.String()
	}
	// aspnetcore is clear of the bar and simply lost.
	if !strings.Contains(why["aspnetcore"], "further clear of the bar") {
		t.Errorf("the runner-up was skipped for the wrong reason: %q", why["aspnetcore"])
	}
	// Every skip line names its cell, its margin and its model, so a reader can
	// act on it without opening the corpus.
	for repo, line := range why {
		for _, want := range []string{repo, opus, "banked", "spread"} {
			if !strings.Contains(line, want) {
				t.Errorf("the skip line for %s does not carry %q: %q", repo, want, line)
			}
		}
	}
}

// The picked cell reads as itself, and an agreement reads as an agreement.
func TestThePickAndAnAgreementBothReadPlainly(t *testing.T) {
	c := replay.Cell{Repo: "mastodon", Model: opus, Margin: 0.775, Spread: 0.050, Scenario: "sha256:27dfc600"}
	for _, want := range []string{"mastodon", opus, "0.775", "0.050", "sha256:27dfc600"} {
		if !strings.Contains(c.String(), want) {
			t.Errorf("the cell line does not carry %q: %q", want, c.String())
		}
	}
	if got := replay.Compare(c, 0.760).String(); !strings.Contains(got, "agrees with") {
		t.Errorf("an agreement does not read as one: %q", got)
	}
}
