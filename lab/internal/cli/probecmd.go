package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/probe"
	"github.com/luuuc/sense/lab/internal/scenario"
)

// probeFlags is one cell: the same three catalog ids `run` takes, plus where the
// pair lands and which Sense binary is under test.
type probeFlags struct {
	config   string
	scenario string
	repo     string
	checkout string
	out      string
	agent    string
	model    string
	senseBin string
	wall     time.Duration
}

func parseProbeFlags(args []string, stderr io.Writer) (probeFlags, error) {
	var f probeFlags
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configFlag(fs, &f.config)
	fs.StringVar(&f.scenario, "scenario", "", "path to the scenario file (required)")
	fs.StringVar(&f.repo, "repo", "", "catalog repo id to run against (required)")
	fs.StringVar(&f.checkout, "checkout", "", "repository the worktrees are taken from (required)")
	fs.StringVar(&f.out, "out", "", "cell directory to create, holding one run per arm (required)")
	fs.StringVar(&f.agent, "agent", "", "catalog agent id to drive (required)")
	fs.StringVar(&f.model, "model", "", "catalog model id (required)")
	fs.StringVar(&f.senseBin, "sense", "sense", "the Sense binary under test")
	fs.DurationVar(&f.wall, "wall", defaultWall, "the sense arm's budget; the baseline's derives from it")
	if err := fs.Parse(args); err != nil {
		return f, err
	}
	for _, required := range []struct{ name, value string }{
		{"-scenario", f.scenario},
		{"-repo", f.repo},
		{"-checkout", f.checkout},
		{"-out", f.out},
		{"-agent", f.agent},
		{"-model", f.model},
	} {
		if required.value == "" {
			return f, fmt.Errorf("%s is required", required.name)
		}
	}
	return f, nil
}

// probeCell runs both arms of one cell and reports whether the pair is a
// measurement.
//
// It exists because a one-arm run measures nothing. `sense-lab run` spawns a
// single unisolated session and takes no subject, so a run tree built with it
// carries arm names and no arms — which is not a theoretical failure: four paid
// runs were spent discovering it. This is the command that reaches
// lab/internal/probe, where both arms, the isolation and the capture already
// live.
func probeCell(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	f, err := parseProbeFlags(args, stderr)
	if err != nil {
		if err != flag.ErrHelp {
			_, _ = fmt.Fprintf(stderr, "sense-lab probe: %v\n", err)
		}
		return exitUsage
	}

	s, j, err := probeSpec(f)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab probe: %v\n", err)
		return exitError
	}

	_, _ = fmt.Fprintf(stdout, "scenario: %s\nrepo:     %s @ %.8s\nagent:    %s\nmodel:    %s\nwall:     %s / %s\ncell:     %s\n\n",
		j.scenarioName, j.repo.ID, j.repo.Commit, j.agent.ID, j.model.ID,
		s.Wall, probe.BaselineWall(s.Wall), s.Root)

	r, err := probe.Run(ctx, s)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab probe: %v\n", err)
		return exitError
	}

	_, _ = fmt.Fprint(stdout, renderReport(r))
	if !r.Sound() {
		// The pair ran and is not a measurement. That is a refusal rather than a
		// breakage, and it is its own exit code so a caller can tell "the arms
		// were not comparable" from "the binary broke" without parsing output.
		return exitRefused
	}
	return exitOK
}

// labBinary is this binary, which the capture shim is spawned through.
//
// It is a variable because under `go test` the running executable is the test
// binary, which has no tee: an arm handed one registers a capture shim that
// answers nothing, and both arms sit until their wall. That is not a test
// artifact — it is the same failure a probe run from a stale or wrong binary
// would have, and it costs a full wall per arm to discover.
var labBinary = os.Executable

// probeJob is what the catalog resolved, kept beside the spec so the header can
// print what was run without reading it back off the spec.
type probeJob struct {
	job
	scenarioName string
}

// probeSpec resolves the catalog and the scenario into one cell to run.
func probeSpec(f probeFlags) (probe.Spec, probeJob, error) {
	c, err := catalog.Load(f.config)
	if err != nil {
		return probe.Spec{}, probeJob{}, err
	}
	j, err := resolveJob(c, runFlags{repo: f.repo, agent: f.agent, model: f.model})
	if err != nil {
		return probe.Spec{}, probeJob{}, err
	}
	set, err := scenario.LoadPath(f.scenario)
	if err != nil {
		return probe.Spec{}, probeJob{}, err
	}

	checkout, _ := filepath.Abs(f.checkout)
	if _, err := os.Stat(checkout); err != nil {
		return probe.Spec{}, probeJob{}, fmt.Errorf("checkout: %w", err)
	}
	if err := checkoutIsAtPin(checkout, j.repo.Commit); err != nil {
		return probe.Spec{}, probeJob{}, err
	}

	// The lab binary is spawned again as the capture shim, from the .mcp.json
	// each arm is given. Resolving it here rather than assuming a name on PATH
	// is what lets a probe run from a build that is not installed.
	labBin, err := labBinary()
	if err != nil {
		return probe.Spec{}, probeJob{}, fmt.Errorf("cannot locate this binary, which the capture shim is spawned through: %w", err)
	}

	return probe.Spec{
		Root:       f.out,
		Parent:     checkout,
		Commit:     j.repo.Commit,
		Prompt:     set.Scenario.Prompt(),
		Command:    j.agent.Binary,
		Args:       append(slices.Clone(j.agent.HeadlessArgs), j.agent.ModelFlag, j.model.ID),
		AgentEnv:   j.agent.Env,
		SetupTool:  j.agent.ID,
		ConfigDirs: j.agent.ConfigDirs,
		SenseBin:   f.senseBin,
		LabBin:     labBin,
		HostPath:   os.Getenv("PATH"),
		Wall:       f.wall,
	}, probeJob{job: j, scenarioName: set.Scenario.Name}, nil
}

// renderReport prints what the pair proved.
//
// Every check that can invalidate a cell is printed whether or not it fired, so
// a clean pair reads as "these were checked and were clean" rather than as
// silence. A report that only listed problems would be indistinguishable from a
// report that ran no checks.
func renderReport(r probe.Report) string {
	var b strings.Builder
	for _, line := range []struct {
		what   string
		wrong  []string
		absent string
	}{
		{"sense arm routes", r.SenseMissing, "all present"},
		{"baseline arm routes", r.BaselineReached, "none reachable"},
		{"persisted memory", r.MemoryReached, "unreadable by both arms"},
		{"Sense in the baseline", r.BaselineUsed, "no sign of it"},
		{"arms differ in", r.Differences, "nothing but Sense access"},
	} {
		if len(line.wrong) == 0 {
			fmt.Fprintf(&b, "  %-22s %s\n", line.what, line.absent)
			continue
		}
		fmt.Fprintf(&b, "  %-22s %s\n", line.what, strings.Join(line.wrong, ", "))
	}
	fmt.Fprintf(&b, "  %-22s %v\n", "sense arm used Sense", r.SenseUsed)
	fmt.Fprintf(&b, "  %-22s %d\n", "MCP frames captured", r.Frames)
	fmt.Fprintf(&b, "  %-22s %v\n", "baseline left a capture", r.BaselineCaptured)

	if r.Sound() {
		b.WriteString("\nSOUND: the two arms differed in Sense access and nothing else.\n")
		return b.String()
	}
	b.WriteString("\nNOT A MEASUREMENT: this pair may not be scored, and its number may not be cited.\n")
	return b.String()
}

// probeSignals runs a cell under the same signal handling a session gets: a
// probe must die with the binary, or an interrupt leaves two agents running,
// unattended and still spending.
func probeSignals(args []string, stdout, stderr io.Writer) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return probeCell(ctx, args, stdout, stderr)
}
