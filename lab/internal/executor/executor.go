// Package executor is where a run happens, as an interface over the two ways
// that has turned out to mean.
//
// The interface was deferred from cycle 03 on purpose. An interface with one
// implementation is indirection rather than abstraction, and one written ahead
// of its second implementation is a guess about a shape nobody has seen. It is
// extracted here, from two things that both work, and it has three methods
// because that is what the two of them differ in — not because three is a good
// number.
//
// Where they disagreed was the design input. Both prepare a place and both give
// it back; what neither could share is how a command is spawned, because one
// runs it on this machine and the other runs it inside something else. So that
// is a method, and everything the two do identically stayed out of the
// interface.
package executor

import (
	"context"

	"github.com/luuuc/sense/lab/internal/isolate"
)

// Executor is where and how one arm runs.
type Executor interface {
	// ID is the catalog id this executor was declared under, so a run can
	// record where it happened.
	ID() string
	// Prepare builds the place one arm runs in and reports what a session
	// inside it sees. It spawns nothing.
	Prepare(ctx context.Context, s isolate.Spec) (isolate.Env, error)
	// Command turns the argv an arm would run into the argv that runs it HERE.
	//
	// This is the whole of the difference between the two implementations, and
	// it is why the interface exists at all: on this machine the argv is the
	// argv, and inside a container it has to be handed to something else first.
	Command(env isolate.Env, argv []string) []string
	// Release gives the place back, keeping whatever evidence the run left.
	//
	// Release rather than a bare removal, because a run that is about to be
	// diagnosed must survive the process that made it: what goes is the
	// disposable state, and what stays is the transcript, the record and the
	// capture.
	Release(ctx context.Context, env isolate.Env) error
}

// Of resolves an executor by the id a subject declares.
//
// A LOOKUP whose keys are catalog ids, which is the same rule the transcript
// formats follow: a subject names where it runs, and nothing here decides that
// by knowing what a subject is.
func Of(id string, c Container) (Executor, error) {
	switch id {
	case IsolatedHomeID:
		return IsolatedHome{}, nil
	case ContainerID:
		return c, nil
	default:
		return nil, &UnknownError{ID: id}
	}
}

// UnknownError is an executor id nothing implements. It is its own type because
// "you named an executor that does not exist" is a catalog problem and reads
// differently from a run that failed.
type UnknownError struct{ ID string }

func (e *UnknownError) Error() string {
	return "no executor called " + e.ID + "; the ones that exist are " + IsolatedHomeID + " and " + ContainerID
}
