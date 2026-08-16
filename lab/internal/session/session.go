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
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	// Command and Args spawn the agent tool.
	Command string
	Args    []string
	// AgentEnv is what the agent tool declares in agent.json, as KEY=VALUE.
	AgentEnv []string
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
		Root:     s.Root,
		Arm:      s.Arm,
		SenseBin: s.SenseBin,
		HostPath: s.HostPath,
		AgentEnv: s.AgentEnv,
	})
	if err != nil {
		return Result{}, err
	}

	if err := isolate.AddWorktree(ctx, s.Parent, s.Commit, env.Repo); err != nil {
		return Result{}, err
	}

	wrote, err := prepareArm(ctx, s, env)
	if err != nil {
		return Result{}, err
	}

	m, err := run.Session(ctx, filepath.Join(env.Root, "session"), run.Spec{
		Dir:        env.Repo,
		Name:       s.Command,
		Args:       s.Args,
		Stdin:      s.Prompt,
		Env:        env.Environ,
		Arm:        string(s.Arm),
		SenseSetup: wrote,
		Wall:       s.Wall,
		Grace:      s.Grace,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{Env: env, Meta: m}, nil
}

// prepareArm configures the repository for the sense arm and does nothing at
// all for the baseline arm. The asymmetry is the measurement.
func prepareArm(ctx context.Context, s Spec, env isolate.Env) (map[string]string, error) {
	if s.Arm != isolate.Sense {
		return nil, nil
	}
	wrote, err := subject.Sense(ctx, s.SenseBin, env.Repo)
	if err != nil {
		return nil, err
	}
	if err := wrapMCP(env.Repo, s.LabBin, filepath.Join(env.Artifacts, captureLog)); err != nil {
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
func wrapMCP(repo, labBin, logPath string) error {
	path := filepath.Join(repo, ".mcp.json")
	b, err := os.ReadFile(path) // #nosec G304 -- the run's own worktree
	if err != nil {
		return fmt.Errorf("read the MCP registration: %w", err)
	}
	// Edited as a generic document rather than through the struct above, so
	// every key the product wrote survives untouched. A round trip through a
	// type the bench declares would silently drop whatever the bench does not
	// know about yet.
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("read the MCP registration: %w", err)
	}
	servers, _ := raw["mcpServers"].(map[string]any)
	entry, ok := servers["sense"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s registers no sense server; the sense arm would run without Sense", path)
	}
	command, _ := entry["command"].(string)
	if command == "" {
		return fmt.Errorf("%s registers a sense server with no command", path)
	}
	args := []string{"tee", "-log", logPath, "--", command}
	if declared, ok := entry["args"].([]any); ok {
		for _, a := range declared {
			args = append(args, fmt.Sprint(a))
		}
	}
	entry["command"] = labBin
	entry["args"] = args

	// Re-marshalling a document that was just unmarshalled into `any` cannot
	// fail, so there is no error branch to write here.
	out, _ := json.MarshalIndent(raw, "", "  ")
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return fmt.Errorf("rewrite the MCP registration: %w", err)
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

// Cleanup removes the worktree and then the whole environment, and proves both
// are gone.
func Cleanup(ctx context.Context, parent string, env isolate.Env) error {
	if err := isolate.RemoveWorktree(ctx, parent, env.Repo); err != nil {
		return err
	}
	return isolate.Cleanup(env)
}

// LogPath is where a run's MCP capture lives.
func LogPath(env isolate.Env) string { return filepath.Join(env.Artifacts, captureLog) }
