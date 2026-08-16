package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/plan"
)

// planCmd prints what a campaign would run and every rejection with its reason.
//
// Printing is for humans. Later cycles call the planner as a library and take
// the job list as values; nothing anywhere parses this table.
func planCmd(args []string, stdout, stderr io.Writer) int {
	var dir, key string
	fs := flag.NewFlagSet("plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configFlag(fs, &dir)
	fs.StringVar(&key, "campaign", "", "campaign key under campaigns/ (required)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	if key == "" {
		_, _ = fmt.Fprintln(stderr, "sense-lab plan: -campaign is required")
		return exitUsage
	}

	c, err := catalog.Load(dir)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab plan: %v\n", err)
		return exitError
	}
	camp, err := loadCampaign(dir, key)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab plan: %v\n", err)
		return exitError
	}
	res, err := plan.Expand(c, camp)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab plan: %v\n", err)
		return exitError
	}

	printPlan(stdout, camp, res)

	// A plan that rejects something is not a failure — the whole point is to
	// find it here rather than in a session — but it exits non-zero so an
	// unattended caller cannot proceed as though the matrix were whole.
	if len(res.Rejected) > 0 {
		return exitIncomplete
	}
	return exitOK
}

func printPlan(w io.Writer, camp plan.Campaign, res plan.Result) {
	_, _ = fmt.Fprintf(w, "campaign %s\njudge    %s (pinned, not an arm)\n\n", camp.Key, camp.Judge)

	_, _ = fmt.Fprintf(w, "WILL RUN: %d cells, %d sessions\n", res.Cells(), res.Runs())
	for _, j := range res.Jobs {
		_, _ = fmt.Fprintf(w, "  %-12s %s\n", j.Role, j)
	}
	if len(res.Rejected) == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "\nWILL NOT RUN: %d\n", len(res.Rejected))
	for _, r := range res.Rejected {
		_, _ = fmt.Fprintf(w, "  %s\n", r)
	}
}

// loadCampaign reads campaigns/<key>/campaign.json. Unknown fields are refused
// for the same reason the catalog refuses them: a typo'd key that parses
// silently is a setting that looks applied and is not.
func loadCampaign(dir, key string) (plan.Campaign, error) {
	path := filepath.Join(dir, "campaigns", key, "campaign.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return plan.Campaign{}, fmt.Errorf("read campaign: %w", err)
	}
	var camp plan.Campaign
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&camp); err != nil {
		return plan.Campaign{}, fmt.Errorf("%s: %w", path, err)
	}
	if camp.Key != key {
		return plan.Campaign{}, fmt.Errorf("%s declares key %q but lives under %q", path, camp.Key, key)
	}
	return camp, nil
}
