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
	"github.com/luuuc/sense/lab/internal/isolate"
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

	s, j, err := probeSpec(ctx, f)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "sense-lab probe: %v\n", err)
		return exitError
	}

	// The baseline's wall is printed as a ceiling because it is not decided
	// yet: it derives from what the sense arm elapses, and a sense arm that
	// finishes early leaves the baseline less than this.
	_, _ = fmt.Fprintf(stdout, "scenario: %s\nrepo:     %s @ %.8s\nagent:    %s\nmodel:    %s\nwall:     %s sense, at most %s baseline (1.2x what sense spends)\ncell:     %s\n\n",
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

// hostCredential is the read of the operator's store, in the attended parent. It
// is a variable so a test can state what the host holds without needing a login.
var hostCredential = isolate.HostCredential

// cellCredential resolves and validates the credential both arms will be given,
// before anything spawns.
//
// It asks whether a credential is USABLE FOR THE WHOLE CELL, which is two
// corrections at once. It used to ask whether an environment variable was set:
// that missed the seat case entirely, because a seat lives in a store rather
// than in the environment, and it also passed a credential that expires part way
// through. A token that outlives the sense arm and dies during the baseline
// burns the finished arm — it can never be paired, because the baseline's budget
// derives from the sense arm it ran with. That is the half-pair hazard arriving
// through the credential instead of through an interrupt, so it is refused here,
// where every other unrunnable combination is refused.
//
// A key-based host needs none of this: the key is an allowlisted variable and it
// reaches the session that way, with no expiry anyone can read.
func cellCredential(ctx context.Context, a catalog.Agent, until time.Time) (isolate.Credential, error) {
	route := credentialRoute(a)
	// A tool that declares no credential route has no file a credential could be
	// provisioned into, so a seat cannot reach it however good the token is. It
	// authenticates by key or not at all.
	if route.File == "" {
		if keyInEnvironment() {
			return isolate.Credential{}, nil
		}
		return isolate.Credential{}, fmt.Errorf("%s declares no credential route, so it can only be "+
			"authenticated by key; set one of %v, or both arms reach no model and produce empty runs",
			a.ID, isolate.Credentials())
	}

	c, err := hostCredential(ctx, os.LookupEnv, route)
	if err != nil {
		if keyInEnvironment() {
			return isolate.Credential{}, nil
		}
		return isolate.Credential{}, fmt.Errorf("no credential for this cell: %w. Log in with `%s` and /login, "+
			"or set one of %v; without either, both arms reach no model and produce empty runs",
			err, a.Binary, isolate.Credentials())
	}
	if c.ExpiresBefore(until) {
		return isolate.Credential{}, fmt.Errorf("the credential expires at %s, before this cell can finish at %s: "+
			"the arm that ran first would be paid for and could never be paired. Log in again with `%s` "+
			"and re-run", c.Expiry().Format(time.RFC3339), until.Format(time.RFC3339), a.Binary)
	}
	return c, nil
}

// credentialRoute is the agent tool's credential route, as the catalog declares
// it. The first config directory is the one the credential lives in; the rest
// are other state directories the contamination checks look inside.
func credentialRoute(a catalog.Agent) isolate.Route {
	r := isolate.Route{
		ConfigDirVar: a.ConfigDirVar,
		Keychain:     a.KeychainService,
		File:         a.CredentialFile,
		EnvVar:       a.CredentialEnv,
		Fields:       a.CredentialFields,
		Expiry:       a.CredentialExpiry,
	}
	if len(a.ConfigDirs) > 0 {
		r.ConfigDir = a.ConfigDirs[0]
	}
	return r
}

// keyInEnvironment reports whether the host authenticates by key instead.
func keyInEnvironment() bool {
	for _, name := range isolate.Credentials() {
		if v, ok := os.LookupEnv(name); ok && v != "" {
			return true
		}
	}
	return false
}

// probeJob is what the catalog resolved, kept beside the spec so the header can
// print what was run without reading it back off the spec.
type probeJob struct {
	job
	scenarioName string
}

// probeSpec resolves the catalog and the scenario into one cell to run.
func probeSpec(ctx context.Context, f probeFlags) (probe.Spec, probeJob, error) {
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

	// A credential the session can actually reach, and one that outlives the
	// cell, resolved before anything spawns.
	//
	// The executor's preserves_auth says what a mode CAN survive isolation, not
	// that a credential is there today. Measured 2026-08-17: both arms of a cell
	// exited in about a second with "Not logged in", zero tokens, because the
	// disposable HOME carries no credential of its own and none was in the
	// environment to pass through. That is two arms produced and a cell wasted,
	// and it is exactly the kind of thing this command exists to refuse first.
	//
	// Both walls, not one. The arms run in sequence, so the cell ends after the
	// sense arm's wall and the baseline's, and a credential good for only the
	// first produces the burned arm this whole layer exists to prevent.
	until := time.Now().Add(f.wall + probe.BaselineWall(f.wall))
	cred, err := cellCredential(ctx, j.agent, until)
	if err != nil {
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
		Agent:      j.agent,
		SetupTool:  j.agent.SetupTool,
		ConfigDirs: j.agent.ConfigDirs,
		Credential: cred,
		Route:      credentialRoute(j.agent),
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
	// Reported for a human to read, never gated: an arm that had Sense and
	// barely touched it is a finding about the agent tool, and a completed run
	// is not evidence that the tool drove the model.
	fmt.Fprintf(&b, "  %-22s %s\n", "sense arm's calls", r.SenseCalls)
	fmt.Fprintf(&b, "  %-22s %s\n", "baseline arm's calls", r.BaselineCalls)
	fmt.Fprintf(&b, "  %-22s %.1fs sense, %.1fs baseline\n", "time to first output",
		r.Sense.Meta.FirstOutputSeconds, r.Baseline.Meta.FirstOutputSeconds)
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
