package executor

import (
	"context"

	"github.com/luuuc/sense/lab/internal/isolate"
)

// IsolatedHomeID is the catalog id of the default executor.
const IsolatedHomeID = "isolated-home"

// IsolatedHome runs an arm on this machine, in a disposable HOME with a
// shadowed PATH. It is the DEFAULT and it stays the default: process isolation
// is not free, and a subject in a container is a subject whose measured wall
// includes container overhead.
type IsolatedHome struct{}

func (IsolatedHome) ID() string { return IsolatedHomeID }

// Prepare is cycle 03's, unchanged. The interface was extracted around it
// rather than the other way round, which is the point: the working
// implementation did not move to fit an interface.
func (IsolatedHome) Prepare(_ context.Context, s isolate.Spec) (isolate.Env, error) {
	return isolate.Prepare(s)
}

// Command runs the argv as it is. There is nothing between the arm and the
// machine.
func (IsolatedHome) Command(_ isolate.Env, argv []string) []string { return argv }

func (IsolatedHome) Release(_ context.Context, env isolate.Env) error { return isolate.Release(env) }
