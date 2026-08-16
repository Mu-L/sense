package plan

import (
	"strings"
	"testing"

	"github.com/luuuc/sense/lab/internal/catalog"
)

// cat builds a catalog in memory. This package answers questions ABOUT config,
// so the tests vary config directly; reading it off disk is the catalog
// package's job and is tested there.
func cat() *catalog.Catalog {
	return &catalog.Catalog{
		Subjects: map[string]catalog.Subject{
			"untreated": {ID: "untreated", Kind: catalog.Baseline,
				Executor: "isolated-home", Agents: []string{"tool"}},
			"sense": {ID: "sense", Kind: catalog.Sense, NeedsMCP: true, NeedsIsolatedConfig: true,
				Executor: "isolated-home", Agents: []string{"tool"}},
		},
		Agents: map[string]catalog.Agent{
			"tool":  {ID: "tool", Binary: "b", SupportsMCP: true, AuthModes: []string{"api_key"}},
			"other": {ID: "other", Binary: "o", AuthModes: []string{"subscription"}},
		},
		Models: map[string]catalog.Model{
			"m1":    {ID: "m1", Provider: "acme", AvailableUnder: []string{"api_key"}, Agents: []string{"tool"}},
			"judge": {ID: "judge", Provider: "acme", AvailableUnder: []string{"api_key"}, Agents: []string{"tool"}},
		},
		Repos: map[string]catalog.Repo{"r1": {ID: "r1"}, "r2": {ID: "r2"}},
		Executors: map[string]catalog.Executor{
			"isolated-home": {ID: "isolated-home",
				PreservesAuth: []string{"subscription", "api_key"}, IsolatesGlobalConfig: true},
			"local":     {ID: "local", PreservesAuth: []string{"subscription", "api_key"}},
			"container": {ID: "container", IsolatesGlobalConfig: true},
		},
	}
}

func campaign() Campaign {
	return Campaign{
		Key: "c", Judge: "judge",
		Subjects: []string{"untreated", "sense"},
		Repos:    []string{"r1"},
		Arms:     []Arm{{Role: Headline, Model: "m1", Runs: 2}},
	}
}

func expand(t *testing.T, c *catalog.Catalog, camp Campaign) Result {
	t.Helper()
	res, err := Expand(c, camp)
	if err != nil {
		t.Fatalf("Expand: %v", err)
	}
	return res
}

// Expansion is a matrix and it must produce BOTH arms of every cell. A plan
// missing one side is a guaranteed half-pair: the finished arm can never be
// paired, and the baseline's budget derives from its partner's wall.
func TestEveryRepoSubjectAndArmBecomesAJob(t *testing.T) {
	camp := campaign()
	camp.Repos = []string{"r1", "r2"}
	camp.Arms = append(camp.Arms, Arm{Role: Confirmation, Model: "judge", Runs: 3})

	res := expand(t, cat(), camp)

	// A cell is one repository and one arm across every subject, so this is
	// 2 repos x 2 arms — NOT the number of jobs, which is twice that.
	if res.Cells() != 4 {
		t.Fatalf("planned %d cells, want 4:\n%v", res.Cells(), res.Jobs)
	}
	if len(res.Jobs) != 8 {
		t.Errorf("planned %d jobs, want 8", len(res.Jobs))
	}
	if res.Runs() != 4*(2+3) { // per-arm run counts, not a flat multiplier
		t.Errorf("planned %d sessions, want %d", res.Runs(), 4*(2+3))
	}
	if len(res.Rejected) != 0 {
		t.Errorf("rejected %v", res.Rejected)
	}
}

// One repository at a time, both arms of a cell adjacent: the output reads in
// the order a campaign is run in.
func TestJobsComeOutInTheOrderACampaignIsRun(t *testing.T) {
	camp := campaign()
	camp.Repos = []string{"r1", "r2"}

	res := expand(t, cat(), camp)

	var got []string
	for _, j := range res.Jobs {
		got = append(got, j.Repo+"/"+j.Subject)
	}
	want := "r1/untreated r1/sense r2/untreated r2/sense"
	if strings.Join(got, " ") != want {
		t.Errorf("order = %v, want %s", got, want)
	}
}

// The five resolution questions, one rejection each. Every reason names what is
// wrong AND what would be right, because a rejection someone cannot act on is
// one they route around by guessing.
func TestEachResolutionQuestionRejectsWithItsOwnReason(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(c *catalog.Catalog, camp *Campaign)
		want   string
	}{
		{
			name: "one: the subject cannot be driven by that agent tool",
			change: func(c *catalog.Catalog, _ *Campaign) {
				s := c.Subjects["untreated"]
				s.Agents = []string{"other"}
				c.Subjects["untreated"] = s
			},
			want: "subject untreated cannot be driven by tool; it supports [other]",
		},
		{
			// Question two is answered by pickAgent: the arm names a tool the
			// model does not list.
			name: "two: the agent tool cannot drive that model",
			change: func(_ *catalog.Catalog, camp *Campaign) {
				camp.Arms[0].Agent = "other" // m1 names only "tool"
			},
			want: `the arm names agent "other", but model m1 can be driven by [tool]`,
		},
		{
			name: "three: nothing can authenticate to that model through that tool",
			change: func(c *catalog.Catalog, _ *Campaign) {
				m := c.Models["m1"]
				m.AvailableUnder = []string{"subscription"}
				c.Models["m1"] = m
			},
			want: "nothing could reach it",
		},
		{
			name: "four: the executor does not preserve the auth that would reach it",
			change: func(c *catalog.Catalog, _ *Campaign) {
				for id, s := range c.Subjects {
					s.Executor = "container"
					c.Subjects[id] = s
				}
			},
			want: "executor container preserves []",
		},
		{
			name: "five: the executor does not isolate config the subject writes",
			change: func(c *catalog.Catalog, _ *Campaign) {
				s := c.Subjects["sense"]
				s.Executor = "local"
				c.Subjects["sense"] = s
			},
			want: "would leak onto the host and into the next run",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, camp := cat(), campaign()
			tc.change(c, &camp)

			res := expand(t, c, camp)

			if len(res.Rejected) == 0 {
				t.Fatalf("nothing was rejected; planned %v", res.Jobs)
			}
			var found bool
			for _, r := range res.Rejected {
				if strings.Contains(r.Reason, tc.want) {
					found = true
				}
			}
			if !found {
				t.Errorf("no rejection said %q; got:\n%v", tc.want, res.Rejected)
			}
		})
	}
}

// A rejected job is not silently dropped: it comes back with the job it was,
// so a reader can see what would have run.
func TestARejectionCarriesTheJobItWas(t *testing.T) {
	c, camp := cat(), campaign()
	s := c.Subjects["sense"]
	s.Executor = "local"
	c.Subjects["sense"] = s

	res := expand(t, c, camp)

	var r Rejection
	for _, got := range res.Rejected {
		if got.Job.Subject == "sense" {
			r = got
		}
	}
	if r.Job.Repo != "r1" || r.Job.Model != "m1" || r.Job.Executor != "local" {
		t.Errorf("the rejection lost the job: %+v", r.Job)
	}
	if !strings.Contains(r.Reason, "would leak onto the host") {
		t.Errorf("the rejection lost its reason: %q", r.Reason)
	}
}

// A model several tools can drive is ambiguous, and picking one arbitrarily is
// how an arm ends up measured on a surface nobody intended.
func TestAModelSeveralToolsCanDriveNeedsTheArmToSayWhich(t *testing.T) {
	c, camp := cat(), campaign()
	m := c.Models["m1"]
	m.Agents = []string{"tool", "other"}
	c.Models["m1"] = m

	res := expand(t, c, camp)

	if len(res.Rejected) == 0 {
		t.Fatal("an ambiguous model was resolved anyway")
	}
	if !strings.Contains(res.Rejected[0].Reason, "does not say which") {
		t.Errorf("reason = %q, want it to name the ambiguity", res.Rejected[0].Reason)
	}

	// Naming the tool resolves it.
	camp.Arms[0].Agent = "tool"
	if res := expand(t, c, camp); res.Cells() != 1 || len(res.Jobs) != 2 {
		t.Errorf("naming the agent left %d cells / %d jobs, want 1 and 2:\n%v",
			res.Cells(), len(res.Jobs), res.Rejected)
	}
}

func TestAnArmNamingAToolItsModelDoesNotSupportIsRejected(t *testing.T) {
	c, camp := cat(), campaign()
	camp.Arms[0].Agent = "other" // m1 names only "tool"

	res := expand(t, c, camp)

	if len(res.Rejected) == 0 {
		t.Fatal("the arm's agent was not checked against its model")
	}
	if !strings.Contains(res.Rejected[0].Reason, `the arm names agent "other"`) {
		t.Errorf("reason = %q", res.Rejected[0].Reason)
	}
}

// A malformed campaign is a different answer from an unsatisfiable one. Naming
// a subject the catalog does not have is a typo; a model no tool can drive is a
// rejection with a reason. Confusing the two sends someone to fix the wrong
// file.
func TestAMalformedCampaignIsAnErrorNotARejection(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(camp *Campaign)
		want   string
	}{
		{"no headline arm", func(c *Campaign) { c.Arms[0].Role = Confirmation },
			"has 0 headline arms"},
		{"two headline arms", func(c *Campaign) {
			c.Arms = append(c.Arms, Arm{Role: Headline, Model: "judge", Runs: 2})
		}, "has 2 headline arms"},
		{"a role that is neither", func(c *Campaign) { c.Arms[0].Role = "control" },
			`role "control" is not headline or confirmation`},
		{"an arm that never runs", func(c *Campaign) { c.Arms[0].Runs = 0 },
			"an arm that never runs is not an arm"},
		{"no judge", func(c *Campaign) { c.Judge = "" },
			"makes every board incomparable"},
		{"a judge no model file declares", func(c *Campaign) { c.Judge = "ghost" },
			`pins judge "ghost"`},
		{"a subject no file declares", func(c *Campaign) { c.Subjects = []string{"ghost"} },
			`names subject "ghost"`},
		{"a repo no file declares", func(c *Campaign) { c.Repos = []string{"ghost"} },
			`names repo "ghost"`},
		{"a model no file declares", func(c *Campaign) { c.Arms[0].Model = "ghost" },
			`names model "ghost"`},
		{"nothing to run", func(c *Campaign) { c.Repos = nil },
			"plans nothing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			camp := campaign()
			tc.change(&camp)

			_, err := Expand(cat(), camp)

			if err == nil {
				t.Fatal("a malformed campaign was expanded")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want %q", err, tc.want)
			}
		})
	}
}

// The judge is not an arm. It produces no cell, and a plan that ran it as one
// would spend on a session that scores nothing.
func TestTheJudgeIsNotPlannedAsAnArm(t *testing.T) {
	res := expand(t, cat(), campaign())

	for _, j := range res.Jobs {
		if j.Model == "judge" {
			t.Errorf("the judge was planned as a job: %s", j)
		}
	}
}

func TestAJobAndARejectionReadAsThemselves(t *testing.T) {
	j := Job{Repo: "r1", Subject: "sense", Agent: "tool", Model: "m1",
		Executor: "local", Auth: "api_key", Runs: 2}

	// The auth mode is on the line because it was chosen, not merely possible.
	for _, want := range []string{"r1", "sense", "tool", "m1", "local", "api_key", "x2"} {
		if !strings.Contains(j.String(), want) {
			t.Errorf("job = %q, missing %q", j.String(), want)
		}
	}
	if got := (Rejection{Job: j, Reason: "because"}).String(); !strings.Contains(got, "because") {
		t.Errorf("rejection = %q, want it to carry the reason", got)
	}
}

// An arm may name a tool the model lists, and still be unreachable through it.
// Support and reachability are different questions and both have to be asked.
func TestAToolTheModelListsMayStillBeUnableToReachIt(t *testing.T) {
	c, camp := cat(), campaign()
	// The subject supports both tools, so question one passes and question two
	// is the one that has to fire.
	for id, s := range c.Subjects {
		s.Agents = []string{"tool", "other"}
		c.Subjects[id] = s
	}
	m := c.Models["m1"]
	m.Agents = []string{"tool", "other"}
	c.Models["m1"] = m
	camp.Arms[0].Agent = "other"

	res := expand(t, c, camp)

	// "other" authenticates by subscription; m1 is api_key only, so the pair
	// cannot authenticate and question three answers.
	if len(res.Rejected) == 0 {
		t.Fatal("an unreachable pairing was planned")
	}
	if !strings.Contains(res.Rejected[0].Reason, "nothing could reach it") {
		t.Errorf("reason = %q", res.Rejected[0].Reason)
	}
}

// A model naming no agent tool at all cannot be driven by anything, and the
// arm cannot rescue it by naming one.
func TestAModelNamingNoAgentToolIsRejected(t *testing.T) {
	c, camp := cat(), campaign()
	m := c.Models["m1"]
	m.Agents = nil
	c.Models["m1"] = m

	res := expand(t, c, camp)

	if len(res.Rejected) == 0 {
		t.Fatal("a model nothing can drive was planned")
	}
	if !strings.Contains(res.Rejected[0].Reason, "names no agent tool, so nothing can drive it") {
		t.Errorf("reason = %q", res.Rejected[0].Reason)
	}
}

// THE hazard this pitch is named for, and the one the planner created before
// this was written. A cell is a PAIR of jobs. If one subject resolves and the
// other does not, planning only the survivor guarantees a burned arm: the
// finished side can never be paired, and the baseline's budget derives from its
// partner's wall, so there is nothing to derive it from.
func TestASurvivingArmOfAnIncompleteCellIsRejectedToo(t *testing.T) {
	c, camp := cat(), campaign()
	// The sense subject cannot run where it is pointed; untreated still can.
	s := c.Subjects["sense"]
	s.Executor = "local"
	c.Subjects["sense"] = s

	res := expand(t, c, camp)

	if len(res.Jobs) != 0 {
		t.Fatalf("planned %d jobs; a half-pair was planned:\n%v", len(res.Jobs), res.Jobs)
	}
	if len(res.Rejected) != 2 {
		t.Fatalf("rejected %d, want both sides", len(res.Rejected))
	}

	// The survivor's rejection names WHY its partner failed, not merely that it
	// did: a reader should not have to correlate two entries by hand.
	var survivor string
	for _, r := range res.Rejected {
		if r.Job.Subject == "untreated" {
			survivor = r.Reason
		}
	}
	if !strings.Contains(survivor, "its cell is incomplete, so running this would burn it") {
		t.Errorf("the survivor's reason = %q", survivor)
	}
	if !strings.Contains(survivor, "sense: subject sense writes agent configuration") {
		t.Errorf("the survivor's reason does not carry its partner's: %q", survivor)
	}
}

// Only the cell that lost a subject is dropped. One broken arm must not take
// the rest of the campaign with it.
func TestAnIncompleteCellDoesNotTakeTheWholeCampaignWithIt(t *testing.T) {
	c, camp := cat(), campaign()
	camp.Repos = []string{"r1", "r2"}
	camp.Arms = append(camp.Arms, Arm{Role: Confirmation, Model: "judge", Runs: 2})
	// Break exactly one job: the sense subject on the confirmation model.
	m := c.Models["judge"]
	m.Agents = []string{"tool", "other"}
	c.Models["judge"] = m

	res := expand(t, c, camp)

	// The judge-model arm is ambiguous for BOTH subjects, so its cells go. The
	// m1 arm is untouched: one cell per repository, two jobs each.
	if res.Cells() != 2 {
		t.Fatalf("planned %d cells, want the 2 m1 cells to survive:\n%v", res.Cells(), res.Jobs)
	}
	if len(res.Jobs) != 4 {
		t.Errorf("planned %d jobs, want 4 — a cell is a pair", len(res.Jobs))
	}
	for _, j := range res.Jobs {
		if j.Model != "m1" {
			t.Errorf("planned %s, which should have gone with its cell", j)
		}
	}
}

// A campaign with one subject has no pair to complete, and dropping its jobs
// would be the planner inventing a rule the campaign did not ask for. Whether
// one subject is enough to conclude anything is a spending law, and it belongs
// to the gates.
func TestASingleSubjectCampaignIsNotTreatedAsHalfOfSomething(t *testing.T) {
	c, camp := cat(), campaign()
	camp.Subjects = []string{"untreated"}

	res := expand(t, c, camp)

	if res.Cells() != 1 || len(res.Jobs) != 1 {
		t.Errorf("planned %d cells / %d jobs, want 1 and 1", res.Cells(), len(res.Jobs))
	}
	if len(res.Rejected) != 0 {
		t.Errorf("rejected %v", res.Rejected)
	}
}

// A cell short a subject with nothing rejected in it is not a resolution
// failure, it is a campaign that listed something twice. An earlier version
// reported it with a blank reason, which breaks this package's own rule that a
// rejection someone cannot act on is one they route around by guessing.
func TestACellShortASubjectForNoResolutionReasonStillSaysWhy(t *testing.T) {
	// decide is exercised directly: a campaign like this is refused at
	// validation now, and the branch is the last line of defence if it ever
	// is not.
	walked := []Rejection{
		{Job: Job{cell: "r1#0", Repo: "r1", Subject: "untreated", Model: "m1"}},
	}

	res := decide(walked, []string{"untreated", "sense"})

	if len(res.Jobs) != 0 {
		t.Fatalf("planned %d jobs from a cell with one side", len(res.Jobs))
	}
	if len(res.Rejected) != 1 {
		t.Fatalf("rejected %d, want 1", len(res.Rejected))
	}
	if !strings.Contains(res.Rejected[0].Reason, "does not name every subject for this cell") {
		t.Errorf("reason = %q, want it to say why rather than trail off", res.Rejected[0].Reason)
	}
}

// A campaign that lists the same thing twice looks complete to anything
// counting: two untreated arms and no sense arm is two jobs in one cell.
func TestACampaignNamingSomethingTwiceIsRefused(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(camp *Campaign)
		want   string
	}{
		{"a subject twice", func(c *Campaign) { c.Subjects = []string{"untreated", "untreated"} },
			`names subject "untreated" twice; a duplicated arm is not a pair`},
		{"a repo twice", func(c *Campaign) { c.Repos = []string{"r1", "r1"} },
			`names repo "r1" twice`},
		{"an arm twice", func(c *Campaign) {
			c.Arms = append(c.Arms, Arm{Role: Confirmation, Model: "m1", Runs: 2})
		}, `names model "m1" twice; running one arm twice is not two arms`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			camp := campaign()
			tc.change(&camp)

			_, err := Expand(cat(), camp)

			if err == nil {
				t.Fatal("a campaign naming something twice was expanded")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %q, want %q", err, tc.want)
			}
		})
	}
}

// The planned job names the model's canonical id, so one plan shows one form
// of a name. Whether an ALIAS resolves is an integration property — a Catalog
// literal has no alias index — and it is tested against a real config directory
// in TestACampaignMayNameAModelByItsAlias.
func TestAPlannedJobNamesTheCanonicalModelID(t *testing.T) {
	c, camp := cat(), campaign()
	c.Models["full/m1:v2"] = catalog.Model{ID: "full/m1:v2", Provider: "acme",
		AvailableUnder: []string{"api_key"}, Agents: []string{"tool"}}
	camp.Arms[0].Model = "full/m1:v2"

	res := expand(t, c, camp)

	for _, j := range res.Jobs {
		if j.Model != "full/m1:v2" {
			t.Errorf("job names model %q, want the canonical id", j.Model)
		}
	}
}

// Role says which arm a win is claimed on, so it has to survive expansion. A
// plan that labelled every job headline would look right and mean something
// else entirely.
func TestEachJobKeepsItsArmsRole(t *testing.T) {
	camp := campaign()
	camp.Arms = append(camp.Arms, Arm{Role: Confirmation, Model: "judge", Runs: 2})

	res := expand(t, cat(), camp)

	got := map[Role]int{}
	for _, j := range res.Jobs {
		got[j.Role]++
	}
	// Two subjects on each of the two arms.
	if got[Headline] != 2 || got[Confirmation] != 2 {
		t.Errorf("roles = %v, want 2 headline and 2 confirmation", got)
	}
}

// The auth mode is CHOSEN, not merely confirmed to exist. A model available two
// ways, on an executor that preserves one of them, must resolve to that one —
// otherwise cycle 03's executor has to guess, and a session that guesses wrong
// dies empty at zero tokens.
func TestTheJobCarriesTheAuthModeThatWasChosen(t *testing.T) {
	c, camp := cat(), campaign()
	m := c.Models["m1"]
	m.AvailableUnder = []string{"subscription", "api_key"}
	c.Models["m1"] = m
	a := c.Agents["tool"]
	a.AuthModes = []string{"subscription", "api_key"}
	c.Agents["tool"] = a
	// The executor preserves only one of the two.
	e := c.Executors["isolated-home"]
	e.PreservesAuth = []string{"api_key"}
	c.Executors["isolated-home"] = e

	res := expand(t, c, camp)

	if len(res.Jobs) == 0 {
		t.Fatalf("nothing planned: %v", res.Rejected)
	}
	for _, j := range res.Jobs {
		if j.Auth != "api_key" {
			t.Errorf("job chose auth %q, want the one the executor preserves", j.Auth)
		}
	}
}

// Deterministic: the model's own order decides, so a plan does not change
// between runs of the same config.
func TestTheChosenAuthModeIsDeterministic(t *testing.T) {
	c, camp := cat(), campaign()
	for _, mode := range [][]string{{"subscription", "api_key"}, {"api_key", "subscription"}} {
		m := c.Models["m1"]
		m.AvailableUnder = mode
		c.Models["m1"] = m
		a := c.Agents["tool"]
		a.AuthModes = []string{"subscription", "api_key"}
		c.Agents["tool"] = a

		res := expand(t, c, camp)

		if len(res.Jobs) == 0 {
			t.Fatalf("nothing planned for %v: %v", mode, res.Rejected)
		}
		if res.Jobs[0].Auth != mode[0] {
			t.Errorf("with the model offering %v the plan chose %q, want the first", mode, res.Jobs[0].Auth)
		}
	}
}
