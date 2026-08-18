package executor

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/luuuc/sense/lab/internal/isolate"
)

// ContainerID is the catalog id of the escalation executor.
const ContainerID = "container"

// Container runs an arm inside a container image, for a subject that cannot be
// contained by a disposable home.
//
// ESCALATION, never the default, and the reason is in the measurement rather
// than in taste: a container's setup lands inside the measured wall, so a
// comparison in which one subject is containerised and another is not carries a
// difference that is about isolation and not about code intelligence. Whoever
// escalates a subject records why, and the comparison says which subjects paid
// the overhead.
//
// **No host credential and no host agent config is ever mounted in.** The
// authenticated agent runs on the host; the subject runs in here. That
// direction is the whole point of the executor, and the catalog says so
// already: this executor preserves no auth mode at all, which is why the
// planner refuses a subject that needs one before anything spawns.
type Container struct {
	// Runtime is the container command, e.g. `docker`. Config rather than a
	// constant so a machine with another runtime is a setting rather than a
	// fork.
	Runtime string
	// Image is what the arm runs inside.
	Image string
	// WorkDir is where the repository is mounted inside the container.
	WorkDir string
}

func (c Container) ID() string { return ContainerID }

// Prepare builds the same run directory tree as the default executor, because
// the run's evidence lives on this machine either way: the transcript, the
// record and the capture are written to the host and read months later.
//
// What differs is only where the COMMAND runs, which is Command's business.
func (c Container) Prepare(_ context.Context, s isolate.Spec) (isolate.Env, error) {
	return isolate.Prepare(s)
}

// Command wraps the argv so it runs inside the image, with the repository
// mounted and nothing else.
//
// One mount, and it is the repository. Not HOME, which is where a credential
// would be; not the config directory, which is where an agent's login would be.
// A container that mounted either would be a container that received host
// credentials, which is the one thing this executor exists to prevent.
func (c Container) Command(env isolate.Env, argv []string) []string {
	work := c.WorkDir
	if work == "" {
		work = "/repo"
	}
	wrapped := []string{
		c.Runtime, "run", "--rm", "-i",
		"--network", "none",
		"-v", env.Repo + ":" + work,
		"-w", work,
		c.Image,
	}
	return append(wrapped, argv...)
}

func (c Container) Release(_ context.Context, env isolate.Env) error { return isolate.Release(env) }

// Available reports whether this runtime can actually start a container here.
//
// Asked before a campaign rather than discovered by a burned arm: a runtime
// that is installed but not running fails at spawn, which costs the arm and
// reads in a score exactly like a model with nothing to say.
func (c Container) Available(ctx context.Context) error {
	if c.Runtime == "" {
		return fmt.Errorf("no container runtime declared")
	}
	out, err := exec.CommandContext(ctx, c.Runtime, "version", "--format", "{{.Server.Version}}").Output()
	if err != nil {
		return fmt.Errorf("%s cannot start a container here: %w", c.Runtime, err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("%s answered with no server version, so nothing is running to start a container", c.Runtime)
	}
	return nil
}
