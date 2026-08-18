package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/luuuc/sense/lab/internal/rescore"
	"github.com/luuuc/sense/lab/internal/scenario"
	"github.com/luuuc/sense/lab/internal/score"
	"github.com/luuuc/sense/lab/internal/transcript"
)

// recordedScore is the part of the old scorer's output this reads.
//
// It carries per-group counts AND a per-row list, which is what makes the
// comparison an accounting rather than a difference of two totals: without the
// rows, "credited two more" and "credited two different ones" look the same.
type recordedScore struct {
	GoldRecall struct {
		Groups map[string]struct {
			Total int `json:"total"`
			Cited int `json:"cited"`
		} `json:"groups"`
		Details []struct {
			ID    string `json:"id"`
			Cited bool   `json:"cited"`
		} `json:"details"`
	} `json:"gold_recall"`
}

type rescoreFlags struct {
	results   string
	scenarios string
	preFix    string
}

func parseRescoreFlags(args []string, stderr io.Writer) (rescoreFlags, error) {
	var f rescoreFlags
	fs := flag.NewFlagSet("rescore", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&f.results, "results", "", "recorded results tree (required)")
	fs.StringVar(&f.scenarios, "scenarios", "lab/scenarios", "directory of scenario sets")
	fs.StringVar(&f.preFix, "pre-fix", "", "file listing the runs whose scores predate the symbol-oracle fix")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	if f.results == "" {
		return f, fmt.Errorf("-results is required")
	}
	return f, nil
}

// rescoreRuns recomputes every recorded score and accounts for the differences.
//
// It is one command because cycle 06 reruns it whenever the scorer changes: a
// proof that has to be reassembled by hand is a proof nobody reruns.
func rescoreRuns(args []string, stdout, stderr io.Writer) int {
	f, err := parseRescoreFlags(args, stderr)
	if err != nil {
		if err != flag.ErrHelp {
			_, _ = fmt.Fprintf(stderr, "sense-lab rescore: %v\n", err)
		}
		return exitUsage
	}

	sets := loadScenarios(f.scenarios)
	if len(sets) == 0 {
		_, _ = fmt.Fprintf(stderr, "sense-lab rescore: no scenarios under %s\n", f.scenarios)
		return exitError
	}
	preFix := readPreFix(f.preFix)

	report, skipped := walkRecorded(f.results, sets, preFix)
	if len(report.Rows) == 0 {
		_, _ = fmt.Fprintf(stderr, "sense-lab rescore: nothing to compare under %s\n", f.results)
		return exitError
	}
	_, _ = fmt.Fprint(stdout, report.String())
	if skipped > 0 {
		_, _ = fmt.Fprintf(stdout, "\n%d recorded runs were not comparable (no gold recall, or no readable capture)\n", skipped)
	}
	if len(report.Unexplained()) > 0 {
		return exitBelowFloor
	}
	return exitOK
}

func loadScenarios(dir string) map[string]scenario.Set {
	sets := map[string]scenario.Set{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return sets
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if s, err := scenario.Load(filepath.Join(dir, e.Name()), e.Name()); err == nil {
			sets[e.Name()] = s
		}
	}
	return sets
}

// readPreFix reads the run keys whose recorded scores predate the fix.
//
// The list is written down BEFORE the comparison, from the artifacts' own
// dates, so `scorer version` is a prediction that can be wrong rather than a
// label assigned to whatever turns out to disagree.
func readPreFix(path string) map[string]bool {
	out := map[string]bool{}
	if path == "" {
		return out
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(line, "|")
		if len(cells) < 6 {
			continue
		}
		repo, model, arm, run := trim(cells[2]), trim(cells[3]), trim(cells[4]), trim(cells[5])
		// The header and the markdown separator are not runs.
		if repo == "" || repo == "repo" || strings.HasPrefix(repo, "-") {
			continue
		}
		out[strings.Join([]string{model, repo, arm, run}, "/")] = true
	}
	return out
}

func trim(s string) string { return strings.TrimSpace(s) }

func walkRecorded(root string, sets map[string]scenario.Set, preFix map[string]bool) (rescore.Report, int) {
	var report rescore.Report
	skipped := 0

	_ = filepath.Walk(root, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() || fi.Name() != "scored.json" {
			return nil
		}
		model, repo, arm, run, ok := runKey(p)
		if !ok {
			return nil
		}
		set, known := sets[repo]
		if !known {
			return nil
		}
		rows, ran := compareRun(p, set, model, repo, arm, run, preFix)
		if !ran {
			skipped++
			return nil
		}
		report.Rows = append(report.Rows, rows...)
		return nil
	})
	return report, skipped
}

// runKey pulls model, repo, arm and run out of a recorded run's path. The
// directories under a results tree ARE the identity of a cell.
func runKey(p string) (model, repo, arm, run string, ok bool) {
	parts := strings.Split(filepath.ToSlash(p), "/results/")
	if len(parts) < 2 {
		return "", "", "", "", false
	}
	seg := strings.Split(parts[1], "/")
	if len(seg) < 5 || (seg[2] != "sense" && seg[2] != "baseline") || strings.HasPrefix(seg[3], "_") {
		return "", "", "", "", false
	}
	return seg[0], seg[3], seg[2], seg[4], true
}

func compareRun(p string, set scenario.Set, model, repo, arm, run string, preFix map[string]bool) ([]rescore.Row, bool) {
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	var rec recordedScore
	if json.Unmarshal(b, &rec) != nil || len(rec.GoldRecall.Groups) == 0 {
		return nil, false
	}
	tr, err := transcript.Read(runFormat(filepath.Dir(p)), filepath.Join(filepath.Dir(p), "transcript.json"))
	if err != nil || strings.TrimSpace(tr.Answer()) == "" {
		return nil, false
	}
	oldCited := map[string]bool{}
	for _, d := range rec.GoldRecall.Details {
		oldCited[d.ID] = d.Cited
	}
	cites := score.Scan(tr.Answer())
	isPre := preFix[strings.Join([]string{model, repo, arm, run}, "/")]

	var out []rescore.Row
	for group, g := range rec.GoldRecall.Groups {
		gold, err := set.Gold.Group(group)
		// A group whose gold this cycle quarantined answers a different
		// question now, and that is a named cause rather than a skipped row.
		if err != nil || len(gold) == 0 {
			out = append(out, rescore.Row{
				Repo: repo, Model: model, Arm: arm, Run: run, Group: group,
				Recorded: g.Cited, Total: g.Total, PreFix: isPre, Quarantined: true,
			})
			continue
		}
		gr := make([]score.Row, 0, len(gold))
		for _, x := range gold {
			gr = append(gr, score.Row{ID: x.ID, Cite: x.Cite()})
		}
		r := score.GroupCites(group, gr, cites, "", 0.5)
		fileLevel, uncited := rescore.Split(cites, gr, oldCited)
		out = append(out, rescore.Row{
			Repo: repo, Model: model, Arm: arm, Run: run, Group: group,
			Recorded: g.Cited, Now: r.Cited, Total: g.Total,
			PreFix: isPre, FileLevel: fileLevel, Uncited: uncited,
		})
	}
	return out, true
}
