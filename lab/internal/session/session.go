// Package session runs one arm end to end: a disposable environment, a worktree
// at the pinned commit, the subject prepared, the agent tool spawned under a
// wall, and everything it said captured.
//
// It captures and it does not interpret. Cycle 02 owns the canonical
// transcript and reads raw/; nothing here normalises anything.
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luuuc/sense/lab/internal/catalog"
	"github.com/luuuc/sense/lab/internal/isolate"
	"github.com/luuuc/sense/lab/internal/run"
	"github.com/luuuc/sense/lab/internal/subject"
	"github.com/luuuc/sense/lab/internal/tee"
)

// Spec is one arm to run.
type Spec struct {
	// Root is the run directory to create. It must not already exist.
	Root string
	// Arm decides both the PATH and whether the subject is configured at all.
	Arm isolate.Arm
	// Parent is the repository the worktree is taken from, and Commit is where
	// it is pinned.
	Parent string
	Commit string
	// Prompt is what the agent is asked, on stdin.
	Prompt string
	// Command and Args spawn the agent tool, and SetupTool is the catalog id
	// `sense setup` configures for. It comes from the catalog rather than a
	// constant here: a probe of one agent that configured the repository for
	// another would give the sense arm a treatment nobody asked for, and
	// nothing in a score would show it.
	Command   string
	Args      []string
	SetupTool string
	// TranscriptFormat is the shape this tool's capture arrives in, carried
	// from the catalog into the run record so a score reads it correctly.
	TranscriptFormat string
	// MCPRegistration is where and how this tool reads its MCP servers, so the
	// capture can be put in front of the Sense one. A tool that declares none
	// runs uncaptured.
	MCPRegistration catalog.MCPRegistration
	// AgentEnv is what the agent tool declares in agent.json, as KEY=VALUE.
	AgentEnv []string
	// Credential is what this arm is given to reach a model, and Route is how it
	// gets there. Both come from the catalog side, never from a constant here.
	Credential isolate.Credential
	Route      isolate.Route
	// SenseBin is the Sense binary, and LabBin is this binary, which the
	// capture shim is invoked through.
	SenseBin string
	LabBin   string
	// HostPath is the PATH both arms derive from.
	HostPath string
	// Wall is how long the session may take, and Grace how long it gets to stop
	// on its own afterwards.
	Wall  time.Duration
	Grace time.Duration
}

// Result is where the run landed and what it did.
type Result struct {
	Env  isolate.Env
	Meta run.Meta
}

// captureLog is the MCP capture, relative to the run's artifacts directory.
const captureLog = "sense-io.jsonl"

// Run prepares the environment and the repository, spawns the agent, and leaves
// the run directory behind. Cleaning it up is the caller's, because a run that
// is about to be diagnosed must survive the process that made it.
func Run(ctx context.Context, s Spec) (Result, error) {
	env, err := isolate.Prepare(isolate.Spec{
		Root:       s.Root,
		Arm:        s.Arm,
		SenseBin:   s.SenseBin,
		HostPath:   s.HostPath,
		AgentEnv:   s.AgentEnv,
		Credential: s.Credential,
		Route:      s.Route,
	})
	if err != nil {
		return Result{}, err
	}

	if err := isolate.AddWorktree(ctx, s.Parent, s.Commit, env.Repo); err != nil {
		return Result{}, err
	}

	wrote, err := prepareArm(ctx, s, env)
	if err != nil {
		// Give the checkout and its registration back before reporting, or an
		// attempt that never produced an arm still costs the parent repository a
		// stale entry — and git then refuses the next attempt at that path. Three
		// of those accumulated in one afternoon and were pruned by hand. The
		// removal's own error is not allowed to hide the real one.
		return Result{Env: env}, errors.Join(err, isolate.RemoveWorktree(detached(ctx), s.Parent, env.Repo))
	}

	// Past this point the run directory holds a transcript, so a failure leaves
	// evidence and the caller decides what to do with it through Finish.
	m, err := run.Session(ctx, filepath.Join(env.Root, "session"), run.Spec{
		Dir:              env.Repo,
		Name:             s.Command,
		Args:             s.Args,
		Stdin:            s.Prompt,
		Env:              env.Environ,
		Arm:              string(s.Arm),
		TranscriptFormat: s.TranscriptFormat,
		SenseSetup:       wrote,
		Wall:             s.Wall,
		Grace:            s.Grace,
	})
	if err != nil {
		// The environment is returned with the error, not swallowed with it: the
		// checkout and its registration exist and have to be given back, and the
		// caller cannot do that from a zero Result. An interrupted cell is exactly
		// the case nobody cleans up.
		return Result{Env: env}, err
	}
	return Result{Env: env, Meta: m}, nil
}

// prepareArm configures the repository for the sense arm and does nothing at
// all for the baseline arm. The asymmetry is the measurement.
func prepareArm(ctx context.Context, s Spec, env isolate.Env) (map[string]string, error) {
	if s.Arm != isolate.Sense {
		return nil, nil
	}
	wrote, err := subject.Sense(ctx, s.SenseBin, s.SetupTool, env.Repo)
	if err != nil {
		return nil, err
	}
	if err := wrapMCP(s.MCPRegistration, env.Repo, s.LabBin, filepath.Join(env.Artifacts, captureLog)); err != nil {
		return nil, err
	}
	return wrote, nil
}

// wrapMCP points the registered Sense server at the capture shim, keeping the
// command the product wrote as the shim's argument.
//
// The registration is the product's, written by `sense setup` and recorded as
// such: the bench does not hand-roll it, because the moment it does it is
// measuring a configuration no user has. What changes is only where the server
// is spawned from, and the server spawned is still exactly the one the product
// named.
func wrapMCP(reg catalog.MCPRegistration, repo, labBin, logPath string) error {
	if reg.File == "" {
		// A tool whose registration this cannot rewrite runs against the real
		// server. The arm still reaches Sense and its own transcript still
		// names every call; what is missing is the frame-level capture, and the
		// pair report says so by counting zero frames.
		return nil
	}
	path := filepath.Join(repo, reg.File)
	b, err := os.ReadFile(path) // #nosec G304 -- the run's own worktree
	if err != nil {
		return fmt.Errorf("read the MCP registration: %w", err)
	}
	// Edited as a generic document rather than through a type of our own, so
	// every key the product wrote survives untouched. A round trip through a
	// type the bench declares would silently drop whatever the bench does not
	// know about yet.
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("read the MCP registration: %w", err)
	}
	servers, _ := raw[reg.ServersKey].(map[string]any)
	entry, ok := servers["sense"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s registers no sense server; the sense arm would run without Sense", path)
	}
	if err := captureThrough(entry, reg.CommandStyle, labBin, logPath); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	// Re-marshalling a document that was just unmarshalled into `any` cannot
	// fail, so there is no error branch to write here.
	out, _ := json.MarshalIndent(raw, "", "  ")
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("rewrite the MCP registration: %w", err)
	}
	return nil
}

// captureThrough rewrites one server entry to run behind the capture, in whichever of the
// two shapes this tool writes.
func captureThrough(entry map[string]any, style, labBin, logPath string) error {
	prefix := []any{labBin, "tee", "-log", logPath, "--"}
	switch style {
	case catalog.CommandArgv:
		argv, ok := entry["command"].([]any)
		if !ok || len(argv) == 0 {
			return errors.New("registers a sense server with no command")
		}
		entry["command"] = append(prefix, argv...)
	case catalog.CommandAndArgs:
		command, _ := entry["command"].(string)
		if command == "" {
			return errors.New("registers a sense server with no command")
		}
		args := []any{"tee", "-log", logPath, "--", command}
		if declared, ok := entry["args"].([]any); ok {
			args = append(args, declared...)
		}
		entry["command"] = labBin
		entry["args"] = args
	default:
		return fmt.Errorf("states its MCP command style as %q, which nothing here writes", style)
	}
	return nil
}

// Capture reports what the run's MCP capture managed, and whether there is one
// at all.
//
// The baseline arm has no MCP server to capture, so it has no capture file. Its
// absence is the signal rather than an error: an empty file would be
// indistinguishable from a capture that failed.
func Capture(env isolate.Env) (status tee.Status, present bool, err error) {
	b, readErr := os.ReadFile(filepath.Join(env.Artifacts, "capture.json"))
	if readErr != nil {
		return tee.Status{}, false, nil
	}
	if err := json.Unmarshal(b, &status); err != nil {
		return tee.Status{}, true, fmt.Errorf("read the capture status: %w", err)
	}
	return status, true, nil
}

// Finish gives back everything the run no longer needs, in whichever of the two
// shapes the directory calls for.
//
// A directory holding a capture, a transcript or a record is a measured arm: it
// gives back its checkout, its registration and its credential, and keeps the
// evidence. One holding none is a throwaway and goes entirely.
//
// Which shape applies is read off the directory rather than passed in. A boolean
// at the call site is a boolean somebody eventually gets backwards, silently, on
// the side that deletes an arm that was paid for and can never be recreated.
func Finish(ctx context.Context, parent string, env isolate.Env) error {
	ctx = detached(ctx)
	if isolate.HoldsEvidence(env.Root) {
		return Release(ctx, parent, env)
	}
	return Cleanup(ctx, parent, env)
}

// Release gives back the worktree, its registration and the credential, and
// keeps the run record. A finished arm is 230MB of checkout and about 19,700
// files that nothing will read again, beside a transcript that will be read for
// months.
func Release(ctx context.Context, parent string, env isolate.Env) error {
	if err := isolate.RemoveWorktree(detached(ctx), parent, env.Repo); err != nil {
		return err
	}
	return isolate.Release(env)
}

// Cleanup removes the worktree and then the whole environment, and proves both
// are gone. It is the shape for a throwaway directory; Finish is what a caller
// with a run directory should reach for.
func Cleanup(ctx context.Context, parent string, env isolate.Env) error {
	if err := isolate.RemoveWorktree(detached(ctx), parent, env.Repo); err != nil {
		return err
	}
	return isolate.Cleanup(env)
}

// detached drops the caller's cancellation, for the git commands cleanup shells
// out to.
//
// The case that most needs cleaning up is the interrupted one, and there the
// context is already cancelled — so every removal would be killed before it ran,
// precisely when a leaked registration costs most. Every entry point here does
// this for itself rather than trusting the caller to have done it: the removals
// are bounded work on local paths, and being correct only because of who called
// you is not being correct.
func detached(ctx context.Context) context.Context { return context.WithoutCancel(ctx) }

// LogPath is where a run's MCP capture lives.
func LogPath(env isolate.Env) string { return filepath.Join(env.Artifacts, captureLog) }
